package eval

import (
	"fmt"
	"io"
	"strings"

	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/types"
)

// resolve is the main entry point for resolving a node's value.
func (t *Tree) resolve(n *Node) error {
	if n.state == stateResolved {
		return nil
	}
	if n.state == stateError {
		return n.err
	}
	if n.state == stateResolving {
		n.err = fmt.Errorf("cycle detected resolving %s", n.path)
		n.state = stateError
		return n.err
	}

	// Track this node on the resolving stack for dependency edge recording.
	t.pushResolving(n)
	defer t.popResolving()

	var err error
	if n.seqIndex >= 0 {
		err = t.resolveSeqField(n)
	} else if n.parent == nil {
		err = t.resolveRoot(n)
	} else {
		err = t.resolveInstance(n)
	}

	if err != nil {
		n.err = err
		n.state = stateError
	}
	return err
}

// resolveRoot resolves the root struct node. No children are eagerly resolved -
// they are pulled in by expression evaluation or explicit walks. Sibling
// positional reads still work because resolveSeqField asks its predecessor
// for span on demand.
func (t *Tree) resolveRoot(n *Node) error {
	n.state = stateResolving
	n.startPos = 0
	n.value = Value{Kind: KindStruct}

	// Resolve endian switch if needed
	if n.schema.Meta.Endian.Kind == types.SwitchEndian {
		endian, err := t.resolveEndianSwitch(n, n.schema.Meta.Endian)
		if err != nil {
			return fmt.Errorf("resolving endian switch: %w", err)
		}
		n.endian = endian
	}

	// The root has a known span only by way of its children resolving. For
	// now we leave the span at [0, 0); accessors that need the size can
	// pull the last child's end position lazily.
	n.span = Range{StartIndex: 0, EndIndex: 0}
	n.state = stateResolved
	return nil
}

// resolveSeqField resolves a sequential field node.
func (t *Tree) resolveSeqField(n *Node) error {
	n.state = stateResolving
	parent := n.parent

	// Step 1: determine start position from predecessor.
	// We only need the predecessor's span (byte range), not its full value,
	// so accept stateSpanResolved as well as stateResolved.
	if n.seqIndex == 0 {
		// First field starts at parent's start position
		if parent.state < stateSpanResolved {
			if err := t.resolve(parent); err != nil {
				return fmt.Errorf("resolving parent %s: %w", parent.path, err)
			}
		}
		n.startPos = parent.startPos
	} else {
		pred := parent.children[n.seqIndex-1]
		if pred.state < stateSpanResolved {
			if err := t.resolve(pred); err != nil {
				return fmt.Errorf("resolving predecessor %s: %w", pred.path, err)
			}
		}
		n.startPos = pred.endPos()
	}

	// Step 2: evaluate if: condition
	if n.attr.If != nil {
		result, err := t.evaluateExprBool(n.parent, n.attr.If)
		if err != nil {
			return fmt.Errorf("evaluating if condition for %s: %w", n.path, err)
		}
		if !result {
			// Condition false: skip this field
			n.value = Value{Kind: KindNone}
			n.span = Range{StartIndex: uint64(n.startPos), EndIndex: uint64(n.startPos)}
			n.state = stateResolved
			return nil
		}
	}

	// Step 3: handle bit alignment
	t.handleBitTransition(n)

	// Step 4: resolve the type
	typ := n.attr.Type.FoldEndian(n.endian)

	// Step 5: seek to position (expression evaluation in if/type-switch may
	// have moved the stream).
	if err := n.seekToStart(); err != nil {
		return fmt.Errorf("seeking to %d for %s: %w", n.startPos, n.path, err)
	}

	// Step 6: handle repeat + type-switch combo first - each element picks
	// its own type via the switch-on expression evaluated per-element.
	if n.attr.Repeat != nil && typ.TypeSwitch != nil {
		err := t.readRepeatedSwitch(n, typ.TypeSwitch)
		if err != nil {
			return err
		}
		n.state = stateResolved
		return nil
	}

	// Step 7: handle bare type switch (no repeat).
	if typ.TypeSwitch != nil {
		err := t.readTypeSwitch(n, typ.TypeSwitch)
		if err != nil {
			return err
		}
		n.state = stateResolved
		return nil
	}

	// Step 8: handle bare repeat.
	if n.attr.Repeat != nil {
		err := t.readRepeated(n, typ.TypeRef)
		if err != nil {
			return err
		}
		n.state = stateResolved
		return nil
	}

	// Step 9: read single value
	n.typeRef = typ.TypeRef
	err := t.readSingle(n, typ.TypeRef)
	if err != nil {
		return err
	}
	n.state = stateResolved
	return nil
}

// resolveInstance resolves an instance node.
func (t *Tree) resolveInstance(n *Node) error {
	n.state = stateResolving

	// Check if: condition
	if n.attr.If != nil {
		result, err := t.evaluateExprBool(n.parent, n.attr.If)
		if err != nil {
			return fmt.Errorf("evaluating if condition for instance %s: %w", n.path, err)
		}
		if !result {
			n.value = Value{Kind: KindNone}
			n.span = Range{}
			n.state = stateResolved
			return nil
		}
	}

	// Value-only instances (no IO)
	if n.attr.Value != nil {
		val, err := t.evaluateExpr(n.parent, n.attr.Value)
		if err != nil {
			return fmt.Errorf("evaluating value instance %s: %w", n.path, err)
		}
		n.value = exprValueToValue(val)
		// Apply enum mapping if the instance has an `enum:` clause and the
		// result is an integer.
		if n.attr.Enum != "" && (n.value.Kind == KindUint || n.value.Kind == KindInt) {
			n.value.EnumName = n.attr.Enum
			if n.value.Kind == KindUint {
				n.value.Int = int64(n.value.Uint)
			}
			n.value.Kind = KindEnum
			n.value.EnumLabel = t.lookupEnumLabel(n, n.attr.Enum, n.value.Int, n.value.Uint)
			n.exprVal = nil // force nodeToExprValue to use the KindEnum path
		} else {
			n.exprVal = val // cache the ExprValue for nodeToExprValue
		}
		n.span = Range{} // no byte range for computed values
		n.state = stateResolved
		return nil
	}

	// Determine stream. Default to whatever this node already pointed at
	// (inherited from the enclosing struct); `io:` can redirect to a
	// different stream, in which case we also pick up the new stream's
	// absolute origin so ByteRange translations work across IO hops.
	stream := n.stream
	streamOffset := n.streamOffset
	if n.attr.IO != nil {
		ioVal, err := t.evaluateExpr(n.parent, n.attr.IO)
		if err != nil {
			return fmt.Errorf("evaluating io for %s: %w", n.path, err)
		}
		if ioVal.Kind == engine.StreamKind && ioVal.Stream != nil {
			stream = ioVal.Stream.Stream
			streamOffset = int64(ioVal.Stream.AbsoluteOffset)
		}
	}

	// Determine position. For pos expressions that may reference `_io.pos`
	// (the parent's stream position at end-of-seq), the seq fields must
	// be resolved first AND the stream restored to end-of-seq so the
	// reference is well-defined regardless of which earlier instances
	// happened to be resolved.
	if n.attr.Pos != nil {
		if err := t.ensureSeqResolvedAndSeeked(n.parent); err != nil {
			return fmt.Errorf("preparing _io for instance %s: %w", n.path, err)
		}
		pos, err := t.evaluateExprInt(n.parent, n.attr.Pos)
		if err != nil {
			return fmt.Errorf("evaluating pos for %s: %w", n.path, err)
		}
		n.startPos = pos
	} else {
		// Non-positioned instances read at the current stream position;
		// ensureSeqResolvedAndSeeked makes that the post-seq position.
		if err := t.ensureSeqResolvedAndSeeked(n.parent); err != nil {
			return fmt.Errorf("preparing stream for instance %s: %w", n.path, err)
		}
		pos, err := stream.Pos()
		if err != nil {
			return fmt.Errorf("getting stream position for %s: %w", n.path, err)
		}
		n.startPos = pos
	}

	// Create sub-stream if size is specified (but not for repeated instances,
	// where size applies per-element, not to the whole). For user-type
	// instances, readUserType also handles `n.attr.Size`; doing it here as
	// well creates a doubly-nested SectionReader chain that misreads bytes
	// in some Go runtimes - so skip and let readUserType own the substream.
	typeRef := n.attr.Type.FoldEndian(n.endian).TypeRef
	isUserType := typeRef != nil && typeRef.Kind == types.User
	if n.attr.Size != nil && n.attr.Repeat == nil && !isUserType {
		size, err := t.evaluateExprInt(n.parent, n.attr.Size)
		if err != nil {
			return fmt.Errorf("evaluating size for %s: %w", n.path, err)
		}
		stream = NewSubStream(stream, n.startPos, size)
		streamOffset += n.startPos
		n.startPos = 0 // sub-stream is relative
	} else if n.attr.SizeEos && n.attr.Repeat == nil && !isUserType {
		streamSize, err := stream.Size()
		if err != nil {
			return fmt.Errorf("getting stream size for %s: %w", n.path, err)
		}
		remaining := streamSize - n.startPos
		stream = NewSubStream(stream, n.startPos, remaining)
		streamOffset += n.startPos
		n.startPos = 0
	}

	n.stream = stream
	n.streamOffset = streamOffset

	// Seek to position
	if err := n.seekToStart(); err != nil {
		return fmt.Errorf("seeking to %d for %s: %w", n.startPos, n.path, err)
	}

	// Resolve type
	typ := n.attr.Type.FoldEndian(n.endian)

	if typ.TypeSwitch != nil {
		return t.readTypeSwitch(n, typ.TypeSwitch)
	}

	if n.attr.Repeat != nil {
		return t.readRepeated(n, typ.TypeRef)
	}

	n.typeRef = typ.TypeRef
	return t.readSingle(n, typ.TypeRef)
}

// resolveType looks up a type name. Searches local struct types first,
// then the global type context.
func (t *Tree) resolveType(name string) *engine.ExprValue {
	// Try global/module resolution first (handles imports and top-level types)
	typ := engine.ResolveTypeOfExpr(t.typeCtx, expr.MustParseExpr(name))
	if typ != nil {
		return typ
	}
	return nil
}

// ensureSeqResolvedAndSeeked makes sure all seq children of struct `s` are
// resolved and that `s.stream` is positioned right after the last seq
// field. This is what KS semantics expect when an instance references
// `_io.pos` - the natural read cursor is at end-of-seq, regardless of
// which earlier instances (which seek wildly) have been touched.
//
// Skips children that are mid-resolution to avoid cycling back through a
// seq field whose own resolution transitively asked for an instance.
func (t *Tree) ensureSeqResolvedAndSeeked(s *Node) error {
	if s == nil {
		return nil
	}
	for _, child := range s.children {
		if child.state == stateUnresolved {
			if err := t.resolve(child); err != nil {
				return fmt.Errorf("resolving seq sibling %s: %w", child.path, err)
			}
		}
	}
	// Seek to just past the last resolved seq field, so _io.pos reflects
	// end-of-seq regardless of any instance reads that happened in between.
	if len(s.children) > 0 && s.stream != nil {
		last := s.children[len(s.children)-1]
		if last.state == stateResolved || last.state == stateSpanResolved {
			endPos := int64(last.span.EndIndex)
			if _, err := s.stream.Seek(endPos, io.SeekStart); err != nil {
				return err
			}
		}
	}
	return nil
}

// resolveParentOverride evaluates the `parent:` expression for a user-type
// field and returns the Node the field's `_parent` should point to. Common
// cases: `parent: _parent` (grandparent), `parent: _root` (root). Returns
// nil if the expression doesn't resolve to a node we can map back.
func (t *Tree) resolveParentOverride(n *Node, exprStr string) *Node {
	// Handle the common identifiers directly to avoid an evaluator round-trip
	// that could fail before our seq is set up.
	switch exprStr {
	case "_parent":
		if n.parent != nil {
			return n.parent.parent
		}
		return nil
	case "_root":
		return n.root
	}
	// General case: evaluate and try to map the resulting struct value back
	// to a Node via its Runtime hook.
	e, err := expr.ParseExpr(exprStr)
	if err != nil {
		return nil
	}
	val, err := t.evaluateExpr(n.parent, e)
	if err != nil || val == nil || val.Runtime == nil {
		return nil
	}
	nr, ok := val.Runtime.(nodeRef)
	if !ok {
		return nil
	}
	return nr.n
}

// resolveTypeInScope looks up a type name relative to a node's scope. Names
// may use the `::` syntax to walk through nested type scopes (e.g.
// `subtype_a::subtype_cc`).
//
// Resolution order:
//  1. Walk the node's tree parent chain - handles `_parent`-style type access.
//  2. Walk the *definition* parent chain of each typeSym we encounter - this
//     finds sibling types defined in the same KSY file, including across
//     imports (where the tree parent and type parent diverge).
//  3. Fall back to global type registration.
func (t *Tree) resolveTypeInScope(n *Node, name string) *engine.ExprValue {
	parts := strings.Split(name, "::")

	tryAt := func(sym *engine.ExprValue) *engine.ExprValue {
		for s := sym; s != nil; s = s.Parent {
			if typ := resolveTypeChain(s, parts); typ != nil {
				return typ
			}
		}
		return nil
	}

	for node := n; node != nil; node = node.parent {
		if node.typeSym == nil {
			continue
		}
		if typ := tryAt(node.typeSym); typ != nil {
			return typ
		}
	}
	// Fall back to global resolution: walk parts through the global type tree.
	if root := t.resolveType(parts[0]); root != nil {
		if len(parts) == 1 {
			return root
		}
		if typ := resolveTypeChain(root, parts[1:]); typ != nil {
			return typ
		}
	}
	return nil
}

// lookupEnumLabel finds the symbolic identifier matching `intVal` (or
// `uintVal` for unsigned overflow cases) within the enum named `name` in
// `n`'s scope. Returns "" if the enum isn't found or the value isn't a
// declared member.
func (t *Tree) lookupEnumLabel(n *Node, name string, intVal int64, uintVal uint64) string {
	enumSym := t.resolveTypeInScope(n, name)
	if enumSym == nil || enumSym.Kind != engine.EnumKind || enumSym.Enum == nil {
		return ""
	}
	for _, ev := range enumSym.Enum.Values {
		if ev.Value == nil {
			continue
		}
		if ev.Value.IsInt64() && ev.Value.Int64() == intVal {
			return string(ev.ID)
		}
		// Fall back to uint comparison for high-bit values that don't fit
		// in int64.
		if ev.Value.Sign() >= 0 && ev.Value.IsUint64() && ev.Value.Uint64() == uintVal {
			return string(ev.ID)
		}
	}
	return ""
}

// resolveTypeChain walks `start.TypeChild(parts[0]).TypeChild(parts[1])...`,
// returning nil if any step misses. With an empty `parts`, returns start.
func resolveTypeChain(start *engine.ExprValue, parts []string) *engine.ExprValue {
	current := start.TypeChild(parts[0])
	if current == nil {
		return nil
	}
	for _, part := range parts[1:] {
		current = current.TypeChild(part)
		if current == nil {
			return nil
		}
	}
	return current
}

// resolveEndianSwitch evaluates an endian switch expression. Cases are tried
// in arbitrary order; the "_" wildcard is the default and is consulted only
// when no concrete case matches.
func (t *Tree) resolveEndianSwitch(n *Node, endian types.Endian) (types.EndianKind, error) {
	switchVal, err := t.evaluateExpr(n, endian.SwitchOn)
	if err != nil {
		return types.UnspecifiedOrder, err
	}
	defaultKind := types.UnspecifiedOrder
	hasDefault := false
	for caseStr, endianKind := range endian.Cases {
		if caseStr == "_" {
			defaultKind = endianKind
			hasDefault = true
			continue
		}
		caseExpr := expr.MustParseExpr(caseStr)
		caseVal, err := t.evaluateExpr(n, caseExpr)
		if err != nil {
			continue
		}
		match, err := engine.Compare(switchVal, caseVal, engine.CompareEqual)
		if err != nil {
			continue
		}
		if match {
			return endianKind, nil
		}
	}
	if hasDefault {
		return defaultKind, nil
	}
	return types.UnspecifiedOrder, fmt.Errorf("no matching endian case")
}
