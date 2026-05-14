package eval

import (
	"fmt"
	"io"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/types"
)

type nodeState int

const (
	stateUnresolved   nodeState = iota
	stateResolving              // cycle detection sentinel
	stateSpanResolved           // byte range known; value not yet read (deferred user-type parse)
	stateResolved
	stateError
)

// Node represents a position in the parse tree. It combines schema information
// (from the KSY definition) with lazily-resolved runtime data (from the binary
// stream). Fields are only read when explicitly accessed.
type Node struct {
	// Schema (static, set at construction)
	name    string
	attr    *kaitai.Attr   // nil for root struct node
	schema  *kaitai.Struct // non-nil for struct-typed nodes
	typeRef *types.TypeRef // resolved (endian-folded) type; nil until resolution for type switches
	typeSym *engine.ExprValue
	path    Path

	// Tree structure
	parent    *Node
	root      *Node
	children  []*Node          // seq field children (in order)
	childMap  map[string]*Node // seq + instance children by name
	instances []*Node          // instance children

	// Resolution state
	state   nodeState
	value   Value
	exprVal *engine.ExprValue // cached ExprValue for value instances
	err     error
	span    Range // byte range [start, end)

	// Stream binding
	stream   *Stream
	startPos int64 // byte offset within `stream`; -1 if not yet determined
	// streamOffset is the absolute offset of `stream` position 0 in the
	// root buffer. Non-zero whenever this node (or one of its ancestors)
	// resolved into a sub-stream - e.g., a user type with explicit
	// `size:` / `size-eos:`. Used by ByteRange to translate stream-local
	// spans back to original-buffer coordinates so the hex editor lights
	// up the right bytes.
	streamOffset int64

	// Positioning
	seqIndex int // index in parent.children; -1 for instances and root

	// Endianness context
	endian    types.EndianKind
	bitEndian types.BitEndianKind

	// Repeat elements (for array-typed nodes)
	items []*Node

	// Params (evaluated constructor arguments for user types)
	params map[string]*engine.ExprValue

	// Dependency tracking (for incremental reparse / edit-and-reserialize).
	// deps: nodes whose values I consumed during my resolution.
	// rdeps: nodes that consumed my value during their resolution.
	deps  map[*Node]struct{}
	rdeps map[*Node]struct{}

	// Back-reference to tree for expression evaluation
	tree *Tree

	// opaque is true when this node represents an externally-defined opaque
	// user type (root meta `ks-opaque-types: true` + unresolvable type).
	// The runtime can't introspect such a type, but `.as<primitive>` casts
	// against it should still read from the underlying stream.
	opaque bool
}

// # Schema accessors (no IO)

// Name returns the field identifier.
func (n *Node) Name() string { return n.name }

// Path returns the full dotted path from root.
func (n *Node) Path() Path { return n.path }

// Attr returns the KSY attribute schema, or nil for the root struct node.
func (n *Node) Attr() *kaitai.Attr { return n.attr }

// StructSchema returns the KSY struct schema for struct-typed nodes, or nil.
func (n *Node) StructSchema() *kaitai.Struct { return n.schema }

// TypeRef returns the resolved type reference, or nil if not yet determined
// (e.g. type switch not yet resolved).
func (n *Node) TypeRef() *types.TypeRef { return n.typeRef }

// IsInstance returns true if this is an instance field (not a seq field).
func (n *Node) IsInstance() bool { return n.seqIndex < 0 && n.parent != nil }

// Parent returns the parent node, or nil for the root.
func (n *Node) Parent() *Node { return n.parent }

// Fields returns the schema-level children (seq fields + instances) without
// triggering any resolution. The returned nodes may be unresolved.
func (n *Node) Fields() []*Node {
	result := make([]*Node, 0, len(n.children)+len(n.instances))
	result = append(result, n.children...)
	result = append(result, n.instances...)
	return result
}

// # Lazy value accessors (trigger IO)

// Resolve ensures this node's value has been read. Returns any error
// encountered during resolution.
func (n *Node) Resolve() error {
	if n.state == stateResolved {
		return nil
	}
	if n.state == stateError {
		return n.err
	}
	return n.tree.resolve(n)
}

// Value returns the resolved value. Calls Resolve() if needed.
func (n *Node) Value() (Value, error) {
	if err := n.Resolve(); err != nil {
		return Value{}, err
	}
	return n.value, nil
}

// Child returns a child node by name. Works for both seq fields and instances.
// Triggers resolution of predecessor seq fields if needed for positioning.
func (n *Node) Child(name string) (*Node, error) {
	child, ok := n.childMap[name]
	if !ok {
		return nil, nil
	}
	return child, nil
}

// Items returns the array elements for a repeated field. Triggers resolution.
func (n *Node) Items() ([]*Node, error) {
	if err := n.Resolve(); err != nil {
		return nil, err
	}
	return n.items, nil
}

// ByteRange returns the byte offset range [start, end) this field occupies
// in the root buffer's coordinate system. Nodes resolved inside sub-streams
// (e.g., size-bound user types) have their stream-local spans translated
// back via `streamOffset`. Triggers resolution.
func (n *Node) ByteRange() (Range, error) {
	if err := n.Resolve(); err != nil {
		return Range{}, err
	}
	off := uint64(n.streamOffset)
	return Range{
		StartIndex: n.span.StartIndex + off,
		EndIndex:   n.span.EndIndex + off,
	}, nil
}

// IsResolved returns true if this node has been successfully resolved.
func (n *Node) IsResolved() bool { return n.state == stateResolved }

// Err returns the cached error from resolution, or nil.
func (n *Node) Err() error { return n.err }

// # Mutation

// Invalidate clears this node's cached state and all descendants,
// allowing re-evaluation against (potentially changed) data.
func (n *Node) Invalidate() {
	n.state = stateUnresolved
	n.value = Value{}
	n.exprVal = nil
	n.err = nil
	n.span = Range{}
	n.startPos = -1
	n.items = nil
	n.params = nil
	n.deps = nil
	n.rdeps = nil
	for _, child := range n.children {
		child.Invalidate()
	}
	for _, inst := range n.instances {
		inst.Invalidate()
	}
}

// MarkDirty resets this node's cached value and transitively dirties every
// node that depended on its value during prior resolution. Unlike Invalidate
// (which clears the whole subtree), MarkDirty walks rdeps - the set of
// nodes whose previous resolution consulted this one - so dependents are
// re-resolved on next access while unrelated subtrees stay intact.
//
// This is the substrate for edit-and-reserialize: a caller modifies a leaf
// field's stored value and calls MarkDirty(field); downstream computed
// instances, sibling sizes, etc. that referenced it become invalid.
func (n *Node) MarkDirty() {
	n.markDirty(make(map[*Node]struct{}))
}

// SetValue overwrites this node's stored primitive value and dirties every
// node that depended on it during prior resolution. The node itself stays
// Resolved with the new value; only nodes that previously consulted it are
// marked Unresolved.
//
// Restricted to primitive Value kinds today: Int, Uint, Float, Bool, Bytes,
// Str, Enum. Edits on struct/array nodes are not yet supported.
func (n *Node) SetValue(v Value) error {
	switch v.Kind {
	case KindInt, KindUint, KindFloat, KindBool, KindBytes, KindStr, KindEnum:
	default:
		return fmt.Errorf("SetValue only supports primitive value kinds, got %s", v.Kind)
	}
	// Dirty dependents before we lose the rdep edges via re-resolution. The
	// node itself does not transition - we're writing its new authoritative
	// value, not invalidating it.
	dependents := make([]*Node, 0, len(n.rdeps))
	for d := range n.rdeps {
		dependents = append(dependents, d)
	}
	for _, d := range dependents {
		d.markDirty(make(map[*Node]struct{}))
	}
	n.value = v
	n.exprVal = nil
	n.state = stateResolved
	return nil
}

func (n *Node) markDirty(visited map[*Node]struct{}) {
	if _, seen := visited[n]; seen {
		return
	}
	visited[n] = struct{}{}

	n.state = stateUnresolved
	n.value = Value{}
	n.exprVal = nil
	n.err = nil
	n.span = Range{}
	n.items = nil
	n.params = nil

	// Snapshot rdeps before clearing forward edges - clearing deps below
	// removes our entry from each dep's rdeps, which is fine because the
	// next re-resolution will re-record them. But our own rdeps point AT
	// us; we want to dirty everyone who was watching us before they look
	// up our (now-stale) value.
	dependents := make([]*Node, 0, len(n.rdeps))
	for d := range n.rdeps {
		dependents = append(dependents, d)
	}

	// Drop forward edges: each `dep` no longer counts us as a dependent.
	for dep := range n.deps {
		delete(dep.rdeps, n)
	}
	n.deps = nil

	for _, dependent := range dependents {
		dependent.markDirty(visited)
	}
}

// endPos returns the end position (exclusive) of this node's byte range.
// Only valid after resolution.
func (n *Node) endPos() int64 {
	return int64(n.span.EndIndex)
}

// seekToStart repositions n.stream to n.startPos.
//
// Primitive reads call this defensively before consuming bytes: expression
// evaluation (size:, pos:, if:, repeat-until, etc.) can lazily resolve other
// nodes that share the stream and leave it at an arbitrary position. Seeking
// back to n.startPos restores the read cursor regardless.
func (n *Node) seekToStart() error {
	// TODO: The fact that this needs to bypass the bit flush is a really bad
	// sign. This is probably broken, and we probably need to get rid of
	// seekToStart entirely. Probably :)
	_, err := n.stream.ReadSeeker.Seek(n.startPos, io.SeekStart)
	return err
}
