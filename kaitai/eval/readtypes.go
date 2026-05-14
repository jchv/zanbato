package eval

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/types"

	kaitai_io "github.com/jchw-forks/kaitai_struct_go_runtime/kaitai"
)

// readSingle reads a single (non-repeated, non-switch) field from the stream.
func (t *Tree) readSingle(n *Node, ref *types.TypeRef) error {
	stream := n.stream
	startPos, err := stream.Pos()
	if err != nil {
		return err
	}
	n.startPos = startPos

	if ref == nil {
		// No explicit type - this might be a bytes field with default type
		n.value = Value{Kind: KindNone}
		endPos, _ := stream.Pos()
		n.span = Range{StartIndex: uint64(startPos), EndIndex: uint64(endPos)}
		n.state = stateResolved
		return nil
	}

	switch ref.Kind {
	case types.U1:
		v, err := stream.ReadU1()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindUint, Uint: uint64(v)}

	case types.U2le:
		v, err := stream.ReadU2le()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindUint, Uint: uint64(v)}

	case types.U2be:
		v, err := stream.ReadU2be()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindUint, Uint: uint64(v)}

	case types.U4le:
		v, err := stream.ReadU4le()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindUint, Uint: uint64(v)}

	case types.U4be:
		v, err := stream.ReadU4be()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindUint, Uint: uint64(v)}

	case types.U8le:
		v, err := stream.ReadU8le()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindUint, Uint: v}

	case types.U8be:
		v, err := stream.ReadU8be()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindUint, Uint: v}

	case types.S1:
		v, err := stream.ReadS1()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindInt, Int: int64(v)}

	case types.S2le:
		v, err := stream.ReadS2le()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindInt, Int: int64(v)}

	case types.S2be:
		v, err := stream.ReadS2be()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindInt, Int: int64(v)}

	case types.S4le:
		v, err := stream.ReadS4le()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindInt, Int: int64(v)}

	case types.S4be:
		v, err := stream.ReadS4be()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindInt, Int: int64(v)}

	case types.S8le:
		v, err := stream.ReadS8le()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindInt, Int: v}

	case types.S8be:
		v, err := stream.ReadS8be()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindInt, Int: v}

	case types.F4le:
		v, err := stream.ReadF4le()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindFloat, Float: float64(v)}

	case types.F4be:
		v, err := stream.ReadF4be()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindFloat, Float: float64(v)}

	case types.F8le:
		v, err := stream.ReadF8le()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindFloat, Float: v}

	case types.F8be:
		v, err := stream.ReadF8be()
		if err != nil {
			return err
		}
		n.value = Value{Kind: KindFloat, Float: v}

	case types.Bits:
		width := ref.Bits.Width
		var v uint64
		be := n.bitEndian
		if ref.Bits.Endian.Kind != types.UnspecifiedBitOrder {
			be = ref.Bits.Endian.Kind
		}
		if be == types.LittleBitEndian {
			v, err = stream.ReadBitsIntLe(int(width))
		} else {
			v, err = stream.ReadBitsIntBe(int(width))
		}
		if err != nil {
			return err
		}
		if width == 1 && n.attr.Enum == "" {
			n.value = Value{Kind: KindBool, Bool: v != 0}
		} else {
			n.value = Value{Kind: KindUint, Uint: v}
		}

	case types.Bytes:
		data, err := t.readBytes(n, ref)
		if err != nil {
			return err
		}
		// Apply process if specified
		if n.attr.Process != nil {
			data, err = t.applyProcess(n.attr.Process, data, func(e *expr.Expr) (int64, error) {
				return t.evaluateExprInt(n.parent, e)
			}, func(e *expr.Expr) (*engine.ExprValue, error) {
				return t.evaluateExpr(n.parent, e)
			})
			if err != nil {
				return fmt.Errorf("process %s: %w", n.path, err)
			}
		}
		n.value = Value{Kind: KindBytes, Bytes: data}

	case types.String:
		data, err := t.readStringBytes(n, ref)
		if err != nil {
			return err
		}
		// Apply process if specified
		if n.attr.Process != nil {
			data, err = t.applyProcess(n.attr.Process, data, func(e *expr.Expr) (int64, error) {
				return t.evaluateExprInt(n.parent, e)
			}, func(e *expr.Expr) (*engine.ExprValue, error) {
				return t.evaluateExpr(n.parent, e)
			})
			if err != nil {
				return fmt.Errorf("process %s: %w", n.path, err)
			}
		}
		// Decode encoding
		enc := ref.String.Encoding
		n.value = Value{Kind: KindStr, Str: decodeString(data, enc)}

	case types.User:
		err := t.readUserType(n, ref)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("unsupported type kind %s for %s", ref.Kind, n.path)
	}

	// Apply enum mapping (keep both Int and Uint for proper big.Int conversion)
	if n.attr.Enum != "" && (n.value.Kind == KindUint || n.value.Kind == KindInt) {
		n.value.EnumName = n.attr.Enum
		if n.value.Kind == KindUint {
			n.value.Int = int64(n.value.Uint) // may overflow for u8 max, but Uint is preserved
		}
		n.value.Kind = KindEnum
		n.value.EnumLabel = t.lookupEnumLabel(n, n.attr.Enum, n.value.Int, n.value.Uint)
	}

	// Record byte range
	endPos, _ := stream.Pos()
	n.span = Range{StartIndex: uint64(startPos), EndIndex: uint64(endPos)}
	n.state = stateResolved
	return nil
}

// readBytes reads a byte field based on the Bytes type spec.
func (t *Tree) readBytes(n *Node, ref *types.TypeRef) ([]byte, error) {
	stream := n.stream

	// Check type-level size first, then fall back to attr-level size
	sizeExpr := ref.Bytes.Size
	if sizeExpr == nil && n.attr != nil && n.attr.Size != nil {
		sizeExpr = n.attr.Size
	}
	sizeEOS := ref.Bytes.SizeEOS
	if !sizeEOS && n.attr != nil && n.attr.SizeEos {
		sizeEOS = true
	}

	if sizeExpr != nil {
		size, err := t.evaluateExprInt(n.parent, sizeExpr)
		if err != nil {
			return nil, err
		}
		if err := n.seekToStart(); err != nil {
			return nil, err
		}
		data, err := stream.ReadBytes(int(size))
		if err != nil {
			return nil, err
		}
		return stripBytes(data, ref.Bytes.Terminator, ref.Bytes.PadRight, ref.Bytes.Include), nil
	}

	if sizeEOS {
		if err := n.seekToStart(); err != nil {
			return nil, err
		}
		data, err := stream.ReadBytesFull()
		if err != nil {
			return nil, err
		}
		return stripBytes(data, ref.Bytes.Terminator, ref.Bytes.PadRight, ref.Bytes.Include), nil
	}

	if ref.Bytes.Terminator >= 0 {
		if err := n.seekToStart(); err != nil {
			return nil, err
		}
		data, err := stream.ReadBytesTerm(byte(ref.Bytes.Terminator),
			ref.Bytes.Include, ref.Bytes.Consume, ref.Bytes.EosError)
		if err != nil {
			return nil, err
		}
		return data, nil
	}

	return nil, fmt.Errorf("unsupported bytes type for %s", n.path)
}

// readStringBytes reads the raw bytes for a string field.
func (t *Tree) readStringBytes(n *Node, ref *types.TypeRef) ([]byte, error) {
	stream := n.stream

	// Check type-level size first, then fall back to attr-level size
	sizeExpr := ref.String.Size
	if sizeExpr == nil && n.attr != nil && n.attr.Size != nil {
		sizeExpr = n.attr.Size
	}
	sizeEOS := ref.String.SizeEOS
	if !sizeEOS && n.attr != nil && n.attr.SizeEos {
		sizeEOS = true
	}

	if sizeExpr != nil {
		size, err := t.evaluateExprInt(n.parent, sizeExpr)
		if err != nil {
			return nil, err
		}
		if err := n.seekToStart(); err != nil {
			return nil, err
		}
		data, err := stream.ReadBytes(int(size))
		if err != nil {
			return nil, err
		}
		term := ref.String.Terminator
		padRight := ref.String.PadRight
		enc := strings.ToUpper(strings.ReplaceAll(ref.String.Encoding, "-", ""))
		isUTF16 := enc == "UTF16LE" || enc == "UTF16BE"
		if isUTF16 {
			// KS semantics on a fixed-size UTF-16 string: pad-right is stripped
			// first (as a multi-byte aligned pad, matching the encoding's
			// code-unit width), then the terminator is looked up.
			if padRight >= 0 {
				data = stripPadRightMulti(data, byte(padRight))
			}
			if term >= 0 {
				data = stripBytesMulti(data, byte(term), ref.String.Include)
			}
		} else {
			data = stripBytes(data, term, padRight, ref.String.Include)
		}
		return data, nil
	}

	if sizeEOS {
		if err := n.seekToStart(); err != nil {
			return nil, err
		}
		data, err := stream.ReadBytesFull()
		if err != nil {
			return nil, err
		}
		term := ref.String.Terminator
		padRight := ref.String.PadRight
		enc := strings.ToUpper(strings.ReplaceAll(ref.String.Encoding, "-", ""))
		isUTF16 := enc == "UTF16LE" || enc == "UTF16BE"
		if isUTF16 && term >= 0 {
			data = stripBytesMulti(data, byte(term), ref.String.Include)
		} else {
			data = stripBytes(data, term, padRight, ref.String.Include)
		}
		return data, nil
	}

	if ref.String.Terminator != -1 {
		enc := strings.ToUpper(strings.ReplaceAll(ref.String.Encoding, "-", ""))
		isUTF16 := enc == "UTF16LE" || enc == "UTF16BE"
		if isUTF16 {
			// Multi-byte terminator for UTF-16: read 2 bytes at a time
			term := []byte{byte(ref.String.Terminator), byte(ref.String.Terminator)}
			data, err := readBytesTermMulti(stream, term, ref.String.Include, ref.String.Consume)
			if err != nil {
				return nil, err
			}
			return data, nil
		}
		data, err := stream.ReadBytesTerm(byte(ref.String.Terminator),
			ref.String.Include, ref.String.Consume, ref.String.EosError)
		if err != nil {
			return nil, err
		}
		return data, nil
	}

	return nil, fmt.Errorf("unsupported string type for %s", n.path)
}

// readUserType reads a user-defined type (struct).
func (t *Tree) readUserType(n *Node, ref *types.TypeRef) error {
	// Resolve the struct type, searching local scope first
	typeSym := t.resolveTypeInScope(n, ref.User.Name)
	if typeSym == nil || typeSym.Struct == nil {
		// Opaque types (ks-opaque-types: true) are types declared without
		// a body - the runtime can't introspect them, but a hacky pattern
		// in the upstream test suite uses `.as<primitive>` to peek into
		// the stream anyway. Set up a minimal stub node that holds onto
		// the stream so the cast can read from it.
		if t.schema != nil && t.schema.Meta.OpaqueTypes {
			n.value = Value{Kind: KindStruct}
			n.opaque = true
			n.startPos, _ = n.stream.Pos()
			n.span = Range{StartIndex: uint64(n.startPos), EndIndex: uint64(n.startPos)}
			n.state = stateResolved
			return nil
		}
		return fmt.Errorf("unresolved user type: %s", ref.User.Name)
	}

	// Evaluate params (constructor arguments) in the parent's scope
	structSchema := typeSym.Struct.Type
	var paramValues map[string]*engine.ExprValue
	if len(ref.User.Params) > 0 && len(structSchema.Params) > 0 {
		paramValues = make(map[string]*engine.ExprValue)
		for i, paramExpr := range ref.User.Params {
			if i >= len(structSchema.Params) {
				break
			}
			paramDef := structSchema.Params[i]
			val, err := t.evaluateExpr(n.parent, paramExpr)
			if err != nil {
				// Best effort: skip params that can't be evaluated
				continue
			}
			paramValues[string(paramDef.ID)] = val
		}
	}

	// Param evaluation above may have moved the stream (e.g. by triggering
	// instance reads). Restore to n.startPos before continuing.
	if n.startPos >= 0 {
		if err := n.seekToStart(); err != nil {
			return fmt.Errorf("re-seeking after param eval for %s: %w", n.path, err)
		}
	}

	// Determine stream for the user type. `streamOffset` tracks the
	// absolute origin in the root buffer; if any of the branches below
	// produces a fresh sub-stream, it bumps streamOffset by the parent
	// startPos (i.e., where the sub-stream begins in the parent stream)
	// before resetting startPos to 0 of the new stream.
	stream := n.stream
	startPos := n.startPos
	streamOffset := n.streamOffset

	// Handle size-limited sub-streams
	if ref.User.Size != nil {
		size, err := t.evaluateExprInt(n.parent, ref.User.Size)
		if err != nil {
			return fmt.Errorf("evaluating size for user type %s: %w", n.path, err)
		}
		// Check if we need to read raw bytes for stripping or processing
		term := -1
		if n.attr != nil && n.attr.Terminator != nil {
			term = *n.attr.Terminator
		}
		padRight := -1
		if n.attr != nil && n.attr.PadRight != nil {
			padRight = *n.attr.PadRight
		}
		hasProcess := n.attr != nil && n.attr.Process != nil
		if term >= 0 || padRight >= 0 || hasProcess {
			// Read raw bytes, strip, process, create sub-stream from result
			if _, err := n.stream.Seek(startPos, io.SeekStart); err != nil {
				return err
			}
			rawData, err := n.stream.ReadBytes(int(size))
			if err != nil {
				return fmt.Errorf("reading sized bytes for %s: %w", n.path, err)
			}
			include := n.attr != nil && n.attr.Include != nil && *n.attr.Include
			data := stripBytes(rawData, term, padRight, include)
			// Apply process transform
			if hasProcess {
				data, err = t.applyProcess(n.attr.Process, data, func(e *expr.Expr) (int64, error) {
					return t.evaluateExprInt(n.parent, e)
				}, func(e *expr.Expr) (*engine.ExprValue, error) {
					return t.evaluateExpr(n.parent, e)
				})
				if err != nil {
					return fmt.Errorf("process for %s: %w", n.path, err)
				}
			}
			stream = kaitai_io.NewStream(bytes.NewReader(data))
		} else {
			stream = NewSubStream(n.stream, startPos, size)
		}
		// Advance parent stream past the sub-stream
		if _, err := n.stream.Seek(startPos+size, io.SeekStart); err != nil {
			return err
		}
		streamOffset += startPos
		startPos = 0
	}

	// Handle attr-level terminator (creates sub-stream from terminated bytes)
	if n.attr != nil && n.attr.Terminator != nil && ref.User.Size == nil {
		term := byte(*n.attr.Terminator)
		include := n.attr.Include != nil && *n.attr.Include
		consume := n.attr.Consume == nil || *n.attr.Consume
		eosError := n.attr.EosError == nil || *n.attr.EosError
		data, err := n.stream.ReadBytesTerm(term, include, consume, eosError)
		if err != nil {
			return fmt.Errorf("reading terminated bytes for %s: %w", n.path, err)
		}
		// Apply process to the terminated bytes before wrapping in a stream.
		if n.attr.Process != nil {
			data, err = t.applyProcess(n.attr.Process, data, func(e *expr.Expr) (int64, error) {
				return t.evaluateExprInt(n.parent, e)
			}, func(e *expr.Expr) (*engine.ExprValue, error) {
				return t.evaluateExpr(n.parent, e)
			})
			if err != nil {
				return fmt.Errorf("process for %s: %w", n.path, err)
			}
		}
		stream = kaitai_io.NewStream(bytes.NewReader(data))
		streamOffset += startPos
		startPos = 0
	}

	// Also check attr-level size/size-eos
	if n.attr != nil && n.attr.Size != nil && ref.User.Size == nil {
		size, err := t.evaluateExprInt(n.parent, n.attr.Size)
		if err != nil {
			return fmt.Errorf("evaluating attr size for %s: %w", n.path, err)
		}
		// Read raw sized bytes, apply attr-level pad/term stripping, then create sub-stream.
		// Re-seek to startPos because expression eval may have moved the stream.
		if _, err := n.stream.Seek(startPos, io.SeekStart); err != nil {
			return err
		}
		rawData, err := n.stream.ReadBytes(int(size))
		if err != nil {
			return fmt.Errorf("reading sized bytes for %s: %w", n.path, err)
		}
		// Apply attr-level terminator and pad-right
		term := -1
		if n.attr.Terminator != nil {
			term = *n.attr.Terminator
		}
		padRight := -1
		if n.attr.PadRight != nil {
			padRight = *n.attr.PadRight
		}
		include := n.attr.Include != nil && *n.attr.Include
		data := stripBytes(rawData, term, padRight, include)
		stream = kaitai_io.NewStream(bytes.NewReader(data))
		// Advance parent stream past the full size
		if _, err := n.stream.Seek(startPos+size, io.SeekStart); err != nil {
			return err
		}
		streamOffset += startPos
		startPos = 0
	} else if n.attr != nil && n.attr.SizeEos && ref.User.Size == nil {
		streamSize, err := n.stream.Size()
		if err != nil {
			return fmt.Errorf("getting stream size for %s: %w", n.path, err)
		}
		remaining := streamSize - startPos
		stream = NewSubStream(n.stream, startPos, remaining)
		streamOffset += startPos
		startPos = 0
	}

	// Build child struct node (structSchema already set above for params)
	childNode := t.newStructNode(n.parent, n.attr, structSchema, typeSym, stream, startPos,
		n.endian, n.bitEndian)
	childNode.path = n.path
	childNode.root = n.root
	childNode.typeSym = typeSym
	childNode.streamOffset = streamOffset

	// newStructNode derived its seq/instance children's paths from
	// (n.parent.path + n.name), which is wrong whenever `n` is an array
	// element: it loses the `[index]` part. Re-derive paths from the
	// corrected childNode.path. Grandchildren are not yet created;
	// they'll inherit the right prefix when their own fields are read.
	//
	// The same applies to streamOffset - newStructNode inherits it from
	// `parent` (= n.parent) which is the *old* offset before any
	// sub-stream we just created. Re-derive it on each child so spans
	// reported in this sub-stream's coordinates can be translated back
	// to the root buffer correctly.
	for _, child := range childNode.children {
		child.path = append(append(Path{}, childNode.path...),
			PathItem{Name: child.name})
		child.streamOffset = streamOffset
	}
	for _, inst := range childNode.instances {
		inst.path = append(append(Path{}, childNode.path...),
			PathItem{Name: inst.name})
		inst.streamOffset = streamOffset
	}

	// Handle endian switch on the nested struct
	if structSchema.Meta.Endian.Kind == types.SwitchEndian {
		endian, err := t.resolveEndianSwitch(childNode, structSchema.Meta.Endian)
		if err == nil {
			childNode.endian = endian
			// Propagate to seq children and instances.
			for _, child := range childNode.children {
				child.endian = endian
			}
			for _, inst := range childNode.instances {
				inst.endian = endian
			}
		}
	}

	// Copy children/instances into n
	n.schema = structSchema
	n.typeSym = typeSym
	n.children = childNode.children
	n.childMap = childNode.childMap
	n.instances = childNode.instances
	n.stream = stream
	n.startPos = startPos // relative to the (possibly sub-)stream
	n.streamOffset = streamOffset
	n.value = Value{Kind: KindStruct}
	n.params = paramValues

	// Fix parent references
	for _, child := range n.children {
		child.parent = n
	}
	for _, inst := range n.instances {
		inst.parent = n
	}

	// Handle `parent:` override. The default parent of n is whatever struct
	// it was declared in; expressions like `parent: _parent` reroute the
	// effective parent of n (used as scope for child expressions) to a
	// different node - typically the grandparent.
	if n.attr != nil && n.attr.Parent != nil && !n.attr.Parent.Disabled && n.attr.Parent.Expr != "" {
		if newParent := t.resolveParentOverride(n, n.attr.Parent.Expr); newParent != nil {
			n.parent = newParent
		}
	}

	// Span is now known (modulo full child resolution). Mark the node as
	// span-resolved so siblings can position relative to it.
	n.span = Range{StartIndex: uint64(startPos), EndIndex: uint64(startPos)}
	n.state = stateSpanResolved

	// If the user-type's byte extent is known without reading children - via
	// explicit size: / size-eos / terminator at either the attr or type-ref
	// level, or via a statically computable struct layout - we can skip the
	// eager child walk. Children resolve lazily through LookupChild, and the
	// parent stream is already positioned for the next sibling.
	if t.userTypeSpanIsKnown(n, ref, structSchema) {
		return nil
	}

	// Variable-extent user type: we must walk children eagerly to discover
	// where the type ends so the parent stream is correctly positioned.
	return t.fullyResolveUserType(n, stream, startPos)
}

// userTypeSpanIsKnown reports whether a user-type's byte range is already
// determined at the point readUserType returns, without walking the seq
// children. When true, child resolution can be deferred until demanded.
//
// Sizing mechanisms that pre-determine the span:
//   - ref.User.Size - explicit per-usage size expression (advances parent stream).
//   - n.attr.Size / n.attr.SizeEos - attr-level size (advances or consumes EOS).
//   - n.attr.Terminator - terminated bytes (consumed from parent stream).
//   - ComputeStructSize(schema) >= 0 - statically computable type layout.
//
// The first three cases are detected by readUserType already advancing the
// parent stream. The static-size case requires us to advance the parent
// stream ourselves so the next sibling lines up.
func (t *Tree) userTypeSpanIsKnown(n *Node, ref *types.TypeRef, schema *kaitai.Struct) bool {
	if ref.User.Size != nil {
		return true
	}
	if n.attr != nil {
		if n.attr.Size != nil || n.attr.SizeEos {
			return true
		}
		if n.attr.Terminator != nil {
			return true
		}
	}
	// Static-size: if every field in the schema has a fixed layout, the
	// total size is known. The parent stream has NOT been pre-advanced in
	// this case, so we seek past the static extent ourselves.
	if staticSize := engine.ComputeStructSize(schema); staticSize >= 0 {
		// n.stream is the (sub-)stream the children read from; for the
		// static-size case there is no sub-stream, so it's the parent
		// stream. n.startPos was reset to 0 for sub-stream cases - but
		// those are handled by the earlier returns above, so we're safe
		// to advance by staticSize from the original parent position.
		curPos, err := n.stream.Pos()
		if err != nil {
			return false
		}
		// Advance the parent stream so positional siblings read from the
		// correct offset.
		if _, err := n.stream.Seek(curPos+staticSize, 0); err != nil {
			return false
		}
		return true
	}
	return false
}

// fullyResolveUserType performs the eager seq-child resolution for a
// user-type node that has already had its span set up by readUserType. It
// updates n.span to cover the seq fields and marks n.state = stateResolved.
//
// Instances are NOT eagerly resolved here. They have arbitrary `pos:` and
// would corrupt stream.Pos() relative to where the seq fields ended; access
// is on-demand via Runtime.LookupChild.
func (t *Tree) fullyResolveUserType(n *Node, stream *Stream, startPos int64) error {
	// Mark resolved first so children can reference us.
	n.state = stateResolved

	// Eagerly resolve all seq children. Each child seeks to its own
	// startPos and reads from there; the parent stream advances naturally
	// to the end of the last seq field.
	for _, child := range n.children {
		if err := t.resolve(child); err != nil {
			return fmt.Errorf("resolving child %s: %w", child.path, err)
		}
	}

	// Capture end-of-seq position BEFORE any instance resolution (which
	// would re-position the stream).
	endPos, _ := stream.Pos()
	n.span = Range{StartIndex: uint64(startPos), EndIndex: uint64(endPos)}
	return nil
}

// readRepeated reads a repeated field (array).
func (t *Tree) readRepeated(n *Node, ref *types.TypeRef) error {
	n.value = Value{Kind: KindArray}
	n.items = nil

	// Re-seek to the correct position - expression evaluation for
	// repeat-expr/repeat-until may have triggered instance resolution
	// that changed the stream position.
	if err := n.seekToStart(); err != nil {
		return fmt.Errorf("seeking for repeat at %s: %w", n.path, err)
	}

	// Track the position the next element should read from. Expression
	// evaluation between elements (until-expr, instance access) may move the
	// stream, so we restore it before each readArrayElement.
	nextPos := n.startPos

	switch repeat := n.attr.Repeat.(type) {
	case types.RepeatEOS:
		i := 0
		for {
			// TODO: This bypasses bit alignment by calling seek directly on the
			// underlying ReadSeeker. This should not be necessary...
			if _, err := n.stream.ReadSeeker.Seek(nextPos, io.SeekStart); err != nil {
				return fmt.Errorf("seeking for repeat-eos element %d at %s: %w", i, n.path, err)
			}
			eof, err := n.stream.EOF()
			if err != nil {
				return fmt.Errorf("checking EOF for %s: %w", n.path, err)
			}
			if eof {
				break
			}
			t.pushIndex(i)
			elem, err := t.readArrayElement(n, ref, i)
			t.popIndex()
			if err != nil {
				return err
			}
			n.items = append(n.items, elem)
			nextPos = int64(elem.span.EndIndex)
			i++
		}

	case types.RepeatExpr:
		count, err := t.evaluateExprInt(n.parent, repeat.CountExpr)
		if err != nil {
			return fmt.Errorf("evaluating repeat-expr for %s: %w", n.path, err)
		}
		// Re-seek after count evaluation - it may have resolved instances
		// that moved the stream position.
		if _, err := n.stream.Seek(nextPos, io.SeekStart); err != nil {
			return fmt.Errorf("re-seeking for repeat-expr at %s: %w", n.path, err)
		}
		for i := 0; i < int(count); i++ {
			if _, err := n.stream.Seek(nextPos, io.SeekStart); err != nil {
				return fmt.Errorf("seeking for repeat-expr element %d at %s: %w", i, n.path, err)
			}
			t.pushIndex(i)
			elem, err := t.readArrayElement(n, ref, i)
			t.popIndex()
			if err != nil {
				return err
			}
			n.items = append(n.items, elem)
			nextPos = int64(elem.span.EndIndex)
		}

	case types.RepeatUntil:
		i := 0
		for {
			if _, err := n.stream.Seek(nextPos, io.SeekStart); err != nil {
				return fmt.Errorf("seeking for repeat-until element %d at %s: %w", i, n.path, err)
			}
			t.pushIndex(i)
			elem, err := t.readArrayElement(n, ref, i)
			if err != nil {
				t.popIndex()
				return err
			}
			n.items = append(n.items, elem)
			nextPos = int64(elem.span.EndIndex)

			done, err := t.evaluateExprWithTemp(n.parent, repeat.UntilExpr, elem, i)
			t.popIndex()
			if err != nil {
				return fmt.Errorf("evaluating repeat-until for %s: %w", n.path, err)
			}
			if done.Kind == engine.BooleanKind && done.Boolean.Value {
				break
			}
			i++
		}

	default:
		return fmt.Errorf("unsupported repeat type for %s", n.path)
	}
	// Leave the stream at the end of the array so the next sibling reads
	// from the right place.
	if _, err := n.stream.Seek(nextPos, io.SeekStart); err != nil {
		return fmt.Errorf("final seek for %s: %w", n.path, err)
	}

	// Record overall span
	if len(n.items) > 0 {
		n.span = Range{
			StartIndex: n.items[0].span.StartIndex,
			EndIndex:   n.items[len(n.items)-1].span.EndIndex,
		}
	} else {
		pos, _ := n.stream.Pos()
		n.span = Range{StartIndex: uint64(n.startPos), EndIndex: uint64(pos)}
	}
	n.state = stateResolved
	return nil
}

// readArrayElement reads one element of a repeated field. Callers must wrap
// the invocation with pushIndex/popIndex so `_index` is bound for any
// expressions evaluated during the element's read.
func (t *Tree) readArrayElement(arrayNode *Node, ref *types.TypeRef, index int) (*Node, error) {
	elem := &Node{
		name:         arrayNode.name,
		attr:         arrayNode.attr,
		typeRef:      ref,
		parent:       arrayNode.parent,
		root:         arrayNode.root,
		stream:       arrayNode.stream,
		startPos:     -1,
		streamOffset: arrayNode.streamOffset,
		seqIndex:     -1,
		endian:       arrayNode.endian,
		bitEndian:    arrayNode.bitEndian,
		childMap:     make(map[string]*Node),
		tree:         t,
		path: append(append(Path{}, arrayNode.path[:len(arrayNode.path)-1]...),
			PathItem{Name: arrayNode.name, Index: &index}),
	}

	// Set start position from current stream position
	pos, err := arrayNode.stream.Pos()
	if err != nil {
		return nil, err
	}
	elem.startPos = pos

	if err := t.readSingle(elem, ref); err != nil {
		return nil, fmt.Errorf("reading element %d of %s: %w", index, arrayNode.path, err)
	}

	return elem, nil
}

// readRepeatedSwitch reads a repeated field whose element type is a switch.
// Each element evaluates the switch-on expression fresh - `_index` (and `_` in
// repeat-until) is bound to the current iteration.
func (t *Tree) readRepeatedSwitch(n *Node, ts *types.TypeSwitch) error {
	n.value = Value{Kind: KindArray}
	n.items = nil

	if err := n.seekToStart(); err != nil {
		return fmt.Errorf("seeking for repeat-switch at %s: %w", n.path, err)
	}

	nextPos := n.startPos
	readOne := func(i int) (*Node, error) {
		if _, err := n.stream.Seek(nextPos, io.SeekStart); err != nil {
			return nil, fmt.Errorf("seeking for repeat-switch element %d at %s: %w", i, n.path, err)
		}
		t.pushIndex(i)
		defer t.popIndex()

		// Evaluate the switch-on expression in n.parent's scope. We have
		// to do this per-element because `codes[_index]`-style expressions
		// depend on the current iteration.
		switchVal, err := t.evaluateExpr(n.parent, ts.SwitchOn)
		if err != nil {
			return nil, fmt.Errorf("evaluating switch-on for %s[%d]: %w", n.path, i, err)
		}
		// Re-seek after expression evaluation (expression eval may have
		// resolved instances that moved the stream).
		if _, err := n.stream.Seek(nextPos, io.SeekStart); err != nil {
			return nil, err
		}

		var caseRef *types.TypeRef
		for caseStr, ref := range ts.Cases {
			if caseStr == "_" {
				continue
			}
			caseExpr := expr.MustParseExpr(caseStr)
			caseVal, err := t.evaluateExpr(n.parent, caseExpr)
			if err != nil {
				continue
			}
			match, err := engine.Compare(switchVal, caseVal, engine.CompareEqual)
			if err != nil || !match {
				continue
			}
			folded := ref.FoldEndian(n.endian)
			caseRef = &folded
			break
		}
		if caseRef == nil {
			if defRef, ok := ts.Cases["_"]; ok {
				folded := defRef.FoldEndian(n.endian)
				caseRef = &folded
			}
		}
		// Re-seek before reading (case-expr evaluation may have moved the stream).
		if _, err := n.stream.Seek(nextPos, io.SeekStart); err != nil {
			return nil, err
		}
		// No case matched: if the array attr has an element-level `size:`,
		// read that many raw bytes so the element is still consumed and
		// accessible via `.as<bytes>`. Terminator/pad default to -1 so no
		// trimming is applied - these are unparsed bytes.
		if caseRef == nil {
			if n.attr.Size != nil {
				bytesRef := &types.TypeRef{
					Kind: types.Bytes,
					Bytes: &types.BytesType{
						Size:       n.attr.Size,
						Terminator: -1,
						PadRight:   -1,
					},
				}
				return t.readArrayElement(n, bytesRef, i)
			}
			return nil, fmt.Errorf("no matching case in switch for %s[%d]", n.path, i)
		}
		return t.readArrayElement(n, caseRef, i)
	}

	switch repeat := n.attr.Repeat.(type) {
	case types.RepeatEOS:
		i := 0
		for {
			if _, err := n.stream.Seek(nextPos, io.SeekStart); err != nil {
				return err
			}
			eof, err := n.stream.EOF()
			if err != nil || eof {
				break
			}
			elem, err := readOne(i)
			if err != nil {
				return err
			}
			n.items = append(n.items, elem)
			nextPos = int64(elem.span.EndIndex)
			i++
		}
	case types.RepeatExpr:
		count, err := t.evaluateExprInt(n.parent, repeat.CountExpr)
		if err != nil {
			return fmt.Errorf("evaluating repeat-expr for %s: %w", n.path, err)
		}
		for i := 0; i < int(count); i++ {
			elem, err := readOne(i)
			if err != nil {
				return err
			}
			n.items = append(n.items, elem)
			nextPos = int64(elem.span.EndIndex)
		}
	case types.RepeatUntil:
		i := 0
		for {
			elem, err := readOne(i)
			if err != nil {
				return err
			}
			n.items = append(n.items, elem)
			nextPos = int64(elem.span.EndIndex)
			done, err := t.evaluateExprWithTemp(n.parent, repeat.UntilExpr, elem, i)
			if err != nil {
				return fmt.Errorf("evaluating repeat-until for %s: %w", n.path, err)
			}
			if done.Kind == engine.BooleanKind && done.Boolean.Value {
				break
			}
			i++
		}
	default:
		return fmt.Errorf("unsupported repeat type for switch %s", n.path)
	}
	if _, err := n.stream.Seek(nextPos, io.SeekStart); err != nil {
		return err
	}

	if len(n.items) > 0 {
		n.span = Range{
			StartIndex: n.items[0].span.StartIndex,
			EndIndex:   n.items[len(n.items)-1].span.EndIndex,
		}
	} else {
		n.span = Range{StartIndex: uint64(n.startPos), EndIndex: uint64(n.startPos)}
	}
	return nil
}

// readTypeSwitch resolves a type switch and reads the matching type.
func (t *Tree) readTypeSwitch(n *Node, ts *types.TypeSwitch) error {
	switchVal, err := t.evaluateExpr(n.parent, ts.SwitchOn)
	if err != nil {
		return fmt.Errorf("evaluating switch-on for %s: %w", n.path, err)
	}

	// Re-seek after switch-on evaluation - it may have moved the stream
	if err := n.seekToStart(); err != nil {
		return fmt.Errorf("re-seeking for type switch at %s: %w", n.path, err)
	}

	for caseStr, caseTypeRef := range ts.Cases {
		if caseStr == "_" {
			continue // handle default last
		}
		caseExpr := expr.MustParseExpr(caseStr)
		caseVal, err := t.evaluateExpr(n.parent, caseExpr)
		if err != nil {
			continue
		}
		match, err := engine.Compare(switchVal, caseVal, engine.CompareEqual)
		if err != nil {
			continue
		}
		if match {
			folded := caseTypeRef.FoldEndian(n.endian)
			n.typeRef = &folded
			return t.readSingle(n, &folded)
		}
	}

	// Check default case
	if defaultRef, ok := ts.Cases["_"]; ok {
		folded := defaultRef.FoldEndian(n.endian)
		n.typeRef = &folded
		return t.readSingle(n, &folded)
	}

	// No match - read as raw bytes if we know the extent.
	if n.attr != nil && (n.attr.Size != nil || n.attr.SizeEos) {
		var data []byte
		var err error
		if n.attr.Size != nil {
			size, sizeErr := t.evaluateExprInt(n.parent, n.attr.Size)
			if sizeErr == nil {
				data, err = n.stream.ReadBytes(int(size))
			} else {
				err = sizeErr
			}
		} else {
			data, err = n.stream.ReadBytesFull()
		}
		if err == nil {
			n.value = Value{Kind: KindBytes, Bytes: data}
			endPos, _ := n.stream.Pos()
			n.span = Range{StartIndex: uint64(n.startPos), EndIndex: uint64(endPos)}
			n.state = stateResolved
			return nil
		}
	}

	// No match and no known extent - Kaitai semantics treat this as the field
	// having no value (e.g. an invalid enum opcode where body has no parsable
	// type). The field exists but is null; siblings continue from the
	// current stream position.
	n.value = Value{Kind: KindNone}
	n.span = Range{StartIndex: uint64(n.startPos), EndIndex: uint64(n.startPos)}
	n.state = stateResolved
	return nil
}

// handleBitTransition handles the transition between bit fields and byte fields.
func (t *Tree) handleBitTransition(n *Node) {
	if n.seqIndex == 0 {
		return
	}

	pred := n.parent.children[n.seqIndex-1]
	predIsBit := pred.typeRef != nil && pred.typeRef.Kind == types.Bits

	typ := n.attr.Type.FoldEndian(n.endian)
	currentIsBit := typ.TypeRef != nil && typ.TypeRef.Kind == types.Bits

	// If transitioning from bit to non-bit, align to byte
	if predIsBit && !currentIsBit {
		n.stream.AlignToByte()
		// Update start position after alignment
		pos, _ := n.stream.Pos()
		n.startPos = pos
	}
}
