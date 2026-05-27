package eval

import (
	"math/big"

	"github.com/jchv/zanbato/kaitai/expr/engine"
)

// nodeRef wraps a Node to implement engine.RuntimeRef. It lets the expression
// engine look up children/items lazily, triggering Node resolution only when
// a value is actually demanded.
//
// Every successful LookupChild/LookupIndex records a dependency edge so that
// later invalidation can propagate correctly.
type nodeRef struct {
	n *Node
}

// LookupChild returns the runtime ExprValue for the named field/instance/param,
// resolving the corresponding Node on demand.
//
// Returns:
// - ev, true: child exists and resolved to ev (non-nil)
// - nil, true: child exists but is null (e.g.: `if: false` conditional)
// - nil, false: no such child
func (r nodeRef) LookupChild(name string) (*engine.ExprValue, bool) {
	n := r.n

	// Special intrinsic property: _io is the stream this node reads from.
	if name == "_io" {
		if n.stream != nil {
			return engine.NewRuntimeStreamValue(n.stream, uint64(n.streamOffset)), true
		}
		return nil, false
	}

	// Special intrinsic property: _sizeof is the byte-range size of this node.
	if name == "_sizeof" {
		// Span resolution suffices - we don't need to fully parse children.
		if n.state < stateSpanResolved {
			if err := n.tree.resolve(n); err != nil {
				return nil, false
			}
		}
		if n.span.EndIndex > n.span.StartIndex {
			size := int64(n.span.EndIndex - n.span.StartIndex)
			return engine.NewIntegerLiteralValue(big.NewInt(size)), true
		}
		// Span is still empty (e.g. a lazy root). Force the seq children to
		// resolve so the span widens to the actual end of the struct's data.
		// We prefer this over ComputeStructSize because the static layout
		// ignores per-usage `size:` overrides on user-type fields.
		for _, child := range n.children {
			if err := child.Resolve(); err != nil {
				break
			}
		}
		if len(n.children) > 0 {
			last := n.children[len(n.children)-1]
			start := n.span.StartIndex
			if n.startPos >= 0 {
				start = uint64(n.startPos)
			}
			end := last.span.EndIndex
			if end > start {
				return engine.NewIntegerLiteralValue(big.NewInt(int64(end - start))), true
			}
		}
		// Fall back to static computation if children couldn't be resolved.
		if n.schema != nil {
			if size := engine.ComputeStructSizeStatic(n.schema); size >= 0 {
				return engine.NewIntegerLiteralValue(big.NewInt(size)), true
			}
		}
		return nil, false
	}

	// Check params first (cheap, no IO).
	if n.params != nil {
		if val, ok := n.params[name]; ok {
			return val, true
		}
	}

	// Opaque externally-defined struct: any member access returns the same
	// opaque struct value, propagating the stream reference. This pattern
	// lets `field.x.y.as<u1>` peek into raw bytes via the cast.
	if n.opaque {
		ev, err := nodeToExprValue(n)
		if err == nil && ev != nil {
			return ev, true
		}
	}

	// Look up the named child Node.
	child, ok := n.childMap[name]
	if !ok {
		return nil, false
	}

	// Record dependency edge: whoever is currently being resolved is consuming
	// this child's value.
	n.tree.recordDep(child)

	// Resolve the child (lazy). If resolution fails, the child exists but has
	// no value.
	if err := child.Resolve(); err != nil {
		return nil, true
	}
	if child.state != stateResolved {
		return nil, true
	}

	ev, err := nodeToExprValue(child)
	if err != nil {
		return nil, true
	}
	if ev == nil {
		return nil, true
	}
	return ev, true
}

// LookupIndex returns the i-th element of an array node.
func (r nodeRef) LookupIndex(i int) (*engine.ExprValue, bool) {
	n := r.n

	// Ensure the array is resolved so n.items is populated.
	if n.state != stateResolved {
		if err := n.tree.resolve(n); err != nil {
			return nil, false
		}
	}
	if i < 0 || i >= len(n.items) {
		return nil, false
	}

	item := n.items[i]
	n.tree.recordDep(item)

	ev, err := nodeToExprValue(item)
	if err != nil {
		return nil, true
	}
	return ev, true
}

// CastTo reads a primitive type from this node's backing stream - used by
// `.as<TYPE>` against opaque externally-defined user types where the engine
// doesn't have a schema to walk through.
//
// Returns (nil, false) for non-opaque nodes or for type names we don't
// support; the engine falls back to its normal cast handling.
func (r nodeRef) CastTo(typeName string) (*engine.ExprValue, bool) {
	n := r.n
	if !n.opaque || n.stream == nil {
		return nil, false
	}
	// Read from the start of this opaque node's bytes.
	if _, err := n.stream.Seek(n.startPos, 0); err != nil {
		return nil, false
	}
	switch typeName {
	case "u1":
		v, err := n.stream.ReadU1()
		if err != nil {
			return nil, false
		}
		return engine.NewIntegerLiteralValue(big.NewInt(int64(v))), true
	case "u2", "u2le":
		v, err := n.stream.ReadU2le()
		if err != nil {
			return nil, false
		}
		return engine.NewIntegerLiteralValue(big.NewInt(int64(v))), true
	case "u2be":
		v, err := n.stream.ReadU2be()
		if err != nil {
			return nil, false
		}
		return engine.NewIntegerLiteralValue(big.NewInt(int64(v))), true
	case "u4", "u4le":
		v, err := n.stream.ReadU4le()
		if err != nil {
			return nil, false
		}
		return engine.NewIntegerLiteralValue(big.NewInt(int64(v))), true
	case "u4be":
		v, err := n.stream.ReadU4be()
		if err != nil {
			return nil, false
		}
		return engine.NewIntegerLiteralValue(big.NewInt(int64(v))), true
	case "s1":
		v, err := n.stream.ReadS1()
		if err != nil {
			return nil, false
		}
		return engine.NewIntegerLiteralValue(big.NewInt(int64(v))), true
	}
	return nil, false
}

// Len returns the array length if known.
func (r nodeRef) Len() (int, bool) {
	n := r.n
	if n.state != stateResolved {
		return 0, false
	}
	return len(n.items), true
}
