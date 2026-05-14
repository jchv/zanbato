package eval

import (
	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/resolve"
	"github.com/jchv/zanbato/kaitai/types"
)

// Tree is the top-level container for a lazy evaluation tree. It binds a KSY
// schema to binary data and provides lazy, on-demand parsing.
type Tree struct {
	root      *Node
	stream    *Stream
	resolver  resolve.Resolver
	typeCtx   *engine.Context
	inputName string
	schema    *kaitai.Struct
	evalDepth int // recursion depth guard for expression evaluation

	// resolvingStack tracks the chain of nodes currently being resolved.
	// Used to record dependency edges: when node X's resolution accesses
	// node Y, we record "X depends on Y" (and the reverse "Y is depended
	// on by X"). This is the substrate for invalidation and incremental
	// reparse when data changes.
	resolvingStack []*Node

	// indexStack tracks the active repeat-expr / repeat-eos / repeat-until
	// iteration indices. The top of the stack is the value of `_index` for
	// expressions evaluated while reading the current array element. We use a
	// stack (not a single value) so nested arrays nest correctly.
	indexStack []int

	// processes is the per-tree custom process registry, keyed by the
	// KSY-canonical name (bare identifier or dotted member chain). Set via
	// RegisterProcess. nil until first use.
	processes map[string]ProcessFunc
}

// pushIndex sets the current _index value for the duration of an array
// element's read.
func (t *Tree) pushIndex(i int) {
	t.indexStack = append(t.indexStack, i)
}

func (t *Tree) popIndex() {
	if len(t.indexStack) > 0 {
		t.indexStack = t.indexStack[:len(t.indexStack)-1]
	}
}

// currentIndex returns the innermost active _index, or -1 if none.
func (t *Tree) currentIndex() int {
	if len(t.indexStack) == 0 {
		return -1
	}
	return t.indexStack[len(t.indexStack)-1]
}

// pushResolving marks a node as currently being resolved.
func (t *Tree) pushResolving(n *Node) {
	t.resolvingStack = append(t.resolvingStack, n)
}

// popResolving unmarks the currently-resolving node.
func (t *Tree) popResolving() {
	if len(t.resolvingStack) > 0 {
		t.resolvingStack = t.resolvingStack[:len(t.resolvingStack)-1]
	}
}

// recordDep annotates that the node currently being resolved (top of the
// resolvingStack) depends on dep. No-op if dep is the same node or if there
// is no node currently being resolved.
func (t *Tree) recordDep(dep *Node) {
	if dep == nil || len(t.resolvingStack) == 0 {
		return
	}
	requester := t.resolvingStack[len(t.resolvingStack)-1]
	if requester == dep {
		return
	}
	if requester.deps == nil {
		requester.deps = make(map[*Node]struct{})
	}
	if dep.rdeps == nil {
		dep.rdeps = make(map[*Node]struct{})
	}
	requester.deps[dep] = struct{}{}
	dep.rdeps[requester] = struct{}{}
}

// NewTree creates a new lazy evaluation tree from a KSY schema and binary
// stream. No IO is performed; the tree is fully unresolved. Call Root() and
// then drill down into nodes to trigger lazy reads.
func NewTree(resolver resolve.Resolver, inputName string, schema *kaitai.Struct, stream *Stream) (*Tree, error) {
	t := &Tree{
		stream:    stream,
		resolver:  resolver,
		inputName: inputName,
		schema:    schema,
		typeCtx:   engine.NewContext(),
	}

	// Resolve imports into the type context
	t.resolveImports(inputName, schema)

	// Build the root type symbol and register it
	typeSym := engine.NewStructSymbol(schema, nil)
	t.typeCtx.AddGlobalType(string(schema.ID), typeSym)
	t.typeCtx.AddModuleType(string(schema.ID), typeSym)

	// Build the root node (children created but unresolved)
	t.root = t.newStructNode(nil, nil, schema, typeSym, stream, 0,
		schema.Meta.Endian.Kind, schema.Meta.BitEndian.Kind)

	return t, nil
}

// Root returns the root node of the tree.
func (t *Tree) Root() *Node { return t.root }

// Schema returns the KSY schema.
func (t *Tree) Schema() *kaitai.Struct { return t.schema }

// Invalidate clears all cached state, allowing re-evaluation.
func (t *Tree) Invalidate() {
	t.root.Invalidate()
}

// SetStream replaces the binary stream and invalidates the tree.
func (t *Tree) SetStream(stream *Stream) {
	t.stream = stream
	t.root.stream = stream
	t.root.Invalidate()
	// Propagate new stream to all nodes
	t.propagateStream(t.root, stream)
}

func (t *Tree) propagateStream(n *Node, stream *Stream) {
	n.stream = stream
	for _, child := range n.children {
		// Only propagate to children that use the same stream (not sub-streams)
		if child.stream == n.stream || child.stream == nil {
			t.propagateStream(child, stream)
		}
	}
	for _, inst := range n.instances {
		if inst.stream == n.stream || inst.stream == nil {
			t.propagateStream(inst, stream)
		}
	}
}

// resolveImports recursively imports types into the type context.
func (t *Tree) resolveImports(inputName string, s *kaitai.Struct) {
	id := string(s.ID)

	// Cycle detection: if this type is already registered, skip
	if t.typeCtx.ResolveGlobalType(id) != nil {
		return
	}

	typeSym := engine.NewStructSymbol(s, nil)
	t.typeCtx.AddGlobalType(id, typeSym)
	t.typeCtx.AddModuleType(id, typeSym)

	for _, importName := range s.Meta.Imports {
		resolvedName, resolvedStruct, err := t.resolver.Resolve(inputName, importName)
		if err != nil {
			continue // skip unresolvable imports
		}
		t.resolveImports(resolvedName, resolvedStruct)
	}
}

// newStructNode creates a Node for a struct type, populating child nodes for
// seq fields and instances but NOT resolving any of them.
func (t *Tree) newStructNode(parent *Node, attr *kaitai.Attr, schema *kaitai.Struct, typeSym *engine.ExprValue, stream *Stream, startPos int64, endian types.EndianKind, bitEndian types.BitEndianKind) *Node {
	root := parent
	if root != nil {
		root = parent.root
	}

	// Inherit the stream's absolute origin from the parent. Sub-streams
	// from `size:` etc. will override this in readUserType.
	var streamOffset int64
	if parent != nil {
		streamOffset = parent.streamOffset
	}

	node := &Node{
		name:         string(schema.ID),
		attr:         attr,
		schema:       schema,
		typeSym:      typeSym,
		parent:       parent,
		root:         root,
		stream:       stream,
		startPos:     startPos,
		streamOffset: streamOffset,
		seqIndex:     -1,
		endian:       endian,
		bitEndian:    bitEndian,
		childMap:     make(map[string]*Node),
		tree:         t,
	}

	// Root node points to itself
	if root == nil {
		node.root = node
	}

	// Build path
	if parent != nil {
		node.path = append(append(Path{}, parent.path...), PathItem{Name: node.name})
	}

	// Inherit/override endianness from schema
	if schema.Meta.Endian.Kind != types.UnspecifiedOrder {
		node.endian = schema.Meta.Endian.Kind
	}
	if schema.Meta.BitEndian.Kind != types.UnspecifiedBitOrder {
		node.bitEndian = schema.Meta.BitEndian.Kind
	}

	// Create child nodes for seq fields
	for i, a := range schema.Seq {
		child := t.newFieldNode(node, a, i, stream)
		node.children = append(node.children, child)
		if name := string(a.ID); name != "" {
			node.childMap[name] = child
		}
	}

	// Create child nodes for instances
	for _, inst := range schema.Instances {
		child := t.newInstanceNode(node, inst, stream)
		node.instances = append(node.instances, child)
		if name := string(inst.ID); name != "" {
			node.childMap[name] = child
		}
	}

	return node
}

// newFieldNode creates an unresolved Node for a seq field.
func (t *Tree) newFieldNode(parent *Node, a *kaitai.Attr, seqIndex int, stream *Stream) *Node {
	name := string(a.ID)

	// Find the type-level symbol for this attr in the parent's type symbol
	var typeSym *engine.ExprValue
	if parent.typeSym != nil {
		typeSym = parent.typeSym.Child(name)
	}

	node := &Node{
		name:         name,
		attr:         a,
		typeSym:      typeSym,
		parent:       parent,
		root:         parent.root,
		stream:       stream,
		startPos:     -1,
		streamOffset: parent.streamOffset,
		seqIndex:     seqIndex,
		endian:       parent.endian,
		bitEndian:    parent.bitEndian,
		childMap:     make(map[string]*Node),
		tree:         t,
		path:         append(append(Path{}, parent.path...), PathItem{Name: name}),
	}

	return node
}

// newInstanceNode creates an unresolved Node for an instance.
func (t *Tree) newInstanceNode(parent *Node, inst *kaitai.Attr, stream *Stream) *Node {
	name := string(inst.ID)

	var typeSym *engine.ExprValue
	if parent.typeSym != nil {
		typeSym = parent.typeSym.Child(name)
	}

	node := &Node{
		name:         name,
		attr:         inst,
		typeSym:      typeSym,
		parent:       parent,
		root:         parent.root,
		stream:       stream,
		startPos:     -1,
		streamOffset: parent.streamOffset,
		seqIndex:     -1, // instances are not positionally ordered
		endian:       parent.endian,
		bitEndian:    parent.bitEndian,
		childMap:     make(map[string]*Node),
		tree:         t,
		path:         append(append(Path{}, parent.path...), PathItem{Name: name}),
	}

	return node
}
