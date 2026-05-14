package eval

import (
	"fmt"
	"math/big"

	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/types"
)

// ValueKind discriminates the kind of value stored in a Value.
type ValueKind int

const (
	KindNone   ValueKind = iota // unresolved, skipped (if: false), or struct/array (use Children/Items)
	KindInt                     // signed integer (Value.Int)
	KindUint                    // unsigned integer (Value.Uint)
	KindFloat                   // floating point (Value.Float)
	KindBool                    // boolean (Value.Bool)
	KindBytes                   // byte array (Value.Bytes)
	KindStr                     // string (Value.Str)
	KindEnum                    // integer + enum metadata
	KindStruct                  // value is in node children
	KindArray                   // value is in node items
)

func (k ValueKind) String() string {
	switch k {
	case KindNone:
		return "none"
	case KindInt:
		return "int"
	case KindUint:
		return "uint"
	case KindFloat:
		return "float"
	case KindBool:
		return "bool"
	case KindBytes:
		return "bytes"
	case KindStr:
		return "str"
	case KindEnum:
		return "enum"
	case KindStruct:
		return "struct"
	case KindArray:
		return "array"
	default:
		return fmt.Sprintf("ValueKind(%d)", int(k))
	}
}

// Value holds a resolved runtime value for a Node.
type Value struct {
	Kind      ValueKind
	Int       int64
	Uint      uint64
	Float     float64
	Bool      bool
	Bytes     []byte
	Str       string
	EnumName  string // enum type name (e.g. "animal")
	EnumLabel string // symbolic label (e.g. "cat"), empty if unknown
}

// nodeToExprValue converts a resolved Node's Value to an engine.ExprValue
// for use in expression evaluation.
func nodeToExprValue(n *Node) (*engine.ExprValue, error) {
	if n.state != stateResolved {
		return nil, fmt.Errorf("node %s is not resolved", n.name)
	}
	// For value instances, return the cached ExprValue directly
	if n.exprVal != nil {
		return n.exprVal, nil
	}
	v := n.value
	// KindNone with resolved state means the field was skipped (if: false)
	// Return nil to signal "null" to the expression engine
	if v.Kind == KindNone {
		return nil, nil
	}

	// Helper: attach _sizeof to any ExprValue that has a byte range
	attachSizeof := func(ev *engine.ExprValue) *engine.ExprValue {
		if ev != nil && n.span.EndIndex > n.span.StartIndex {
			sizeVal := int64(n.span.EndIndex - n.span.StartIndex)
			// Make a copy with a new Children map so we don't mutate shared symbol tables
			newChildren := make(map[string]*engine.ExprValue)
			if ev.Children != nil {
				for k, v := range ev.Children {
					newChildren[k] = v
				}
			}
			newChildren["_sizeof"] = engine.NewIntegerLiteralValue(big.NewInt(sizeVal))
			ev = &engine.ExprValue{
				Kind:      ev.Kind,
				Parent:    ev.Parent,
				DefParent: ev.DefParent,
				Children:  newChildren,
				Types:     ev.Types,
				Constant:  ev.Constant,
				Struct:    ev.Struct,
				Array:     ev.Array,
				Integer:   ev.Integer,
				Float:     ev.Float,
				Boolean:   ev.Boolean,
				Items:     ev.Items,
				ByteArray: ev.ByteArray,
				String:    ev.String,
				Stream:    ev.Stream,
			}
		}
		return ev
	}
	switch v.Kind {
	case KindInt:
		return attachSizeof(engine.NewIntegerLiteralValue(big.NewInt(v.Int))), nil
	case KindUint:
		val := new(big.Int).SetUint64(v.Uint)
		return attachSizeof(engine.NewIntegerLiteralValue(val)), nil
	case KindFloat:
		return attachSizeof(engine.NewFloatLiteralValue(big.NewFloat(v.Float))), nil
	case KindBool:
		return attachSizeof(engine.NewBooleanLiteralValue(v.Bool)), nil
	case KindBytes:
		return attachSizeof(engine.NewByteArrayLiteralValue(v.Bytes)), nil
	case KindStr:
		return attachSizeof(engine.NewStringLiteralValue(v.Str)), nil
	case KindEnum:
		// Enum values need both integer methods (to_s) and enum methods (to_i)
		enumMethods := map[string]*engine.ExprValue{}
		for k, ev := range engine.IntegerSymbolTable {
			enumMethods[k] = ev
		}
		for k, ev := range engine.EnumValueSymbolTable {
			enumMethods[k] = ev
		}
		// Use Uint for proper unsigned representation (avoids overflow for u8 max)
		intVal := big.NewInt(v.Int)
		if v.Uint != 0 && v.Int < 0 {
			intVal = new(big.Int).SetUint64(v.Uint)
		}
		ev := &engine.ExprValue{
			Kind:     engine.IntegerKind,
			Children: enumMethods,
			Integer:  &engine.IntegerData{Value: intVal},
		}
		return attachSizeof(ev), nil
	case KindStruct:
		// Opaque externally-defined struct: no schema, but the node knows
		// where its data starts. Surface a bare StructKind with the Runtime
		// hook so member chains pass through and `.as<primitive>` reads
		// from the stream. Check this BEFORE the typeSym path because the
		// hw field has a non-Struct typeSym leftover from parent-typeSym
		// lookup that would crash NewStructValueSymbol.
		if n.opaque {
			return &engine.ExprValue{
				Kind:    engine.StructKind,
				Runtime: nodeRef{n: n},
			}, nil
		}
		// For structs, create a value-level symbol and pre-populate resolved
		// children directly so member access works without RuntimeValue callbacks.
		if n.typeSym != nil && n.typeSym.Struct != nil {
			valSym := engine.NewStructValueSymbol(n.typeSym, nil)
			valSym.Runtime = nodeRef{n: n}
			// Pre-populate param values (Runtime.LookupChild also covers
			// them but keeping in Children helps the PutStack caching path).
			for name, val := range n.params {
				valSym.Children[name] = val
			}
			return valSym, nil
		}
		return nil, fmt.Errorf("struct node %s has no type symbol", n.name)
	case KindArray:
		// Build array of ExprValues from items
		items := make([]*engine.ExprValue, len(n.items))
		for i, item := range n.items {
			ev, err := nodeToExprValue(item)
			if err != nil {
				return nil, fmt.Errorf("array item %d: %w", i, err)
			}
			items[i] = ev
		}
		// Build the array ExprValue directly - NewArrayLiteralValue requires
		// ValueType() to succeed on the element, which fails for structs.
		elemSym := &engine.ExprValue{Kind: engine.IntegerKind}
		if len(items) > 0 {
			elemSym = &engine.ExprValue{Kind: items[0].Kind}
		}
		arrType := engine.NewArrayType(elemSym, nil)
		result := engine.NewArrayLiteralValue(arrType, items)
		if result == nil {
			// Fallback: construct directly for struct-typed arrays etc.
			result = &engine.ExprValue{
				Kind:     engine.ArrayKind,
				Array:    &engine.ArrayTypeData{Elem: elemSym},
				Children: engine.ArraySymbolTable(types.Type{TypeRef: &types.TypeRef{Kind: types.User}}),
				Items:    items,
			}
		}
		result.Runtime = nodeRef{n: n}
		return result, nil
	case KindNone:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported value kind: %s", v.Kind)
	}
}

// exprValueToValue converts an engine.ExprValue to a Value.
// Used for value instances where the result comes from expression evaluation.
func exprValueToValue(ev *engine.ExprValue) Value {
	if ev == nil {
		return Value{Kind: KindNone}
	}
	switch ev.Kind {
	case engine.IntegerKind:
		if ev.Integer != nil {
			if ev.Integer.Value.IsInt64() {
				return Value{Kind: KindInt, Int: ev.Integer.Value.Int64()}
			}
			if ev.Integer.Value.IsUint64() {
				return Value{Kind: KindUint, Uint: ev.Integer.Value.Uint64()}
			}
			// Fallback to int64 (may overflow for very large values)
			return Value{Kind: KindInt, Int: ev.Integer.Value.Int64()}
		}
	case engine.FloatKind:
		if ev.Float != nil {
			f, _ := ev.Float.Value.Float64()
			return Value{Kind: KindFloat, Float: f}
		}
	case engine.BooleanKind:
		if ev.Boolean != nil {
			return Value{Kind: KindBool, Bool: ev.Boolean.Value}
		}
	case engine.ByteArrayKind:
		if ev.ByteArray != nil {
			return Value{Kind: KindBytes, Bytes: ev.ByteArray.Value}
		}
	case engine.StringKind:
		if ev.String != nil {
			return Value{Kind: KindStr, Str: ev.String.Value}
		}
	case engine.ArrayKind:
		// Check if it's a byte array (all integer elements 0-255)
		if len(ev.Items) > 0 {
			isByteArray := true
			for _, item := range ev.Items {
				if item.Kind != engine.IntegerKind || item.Integer == nil {
					isByteArray = false
					break
				}
				v := item.Integer.Value.Int64()
				if v < 0 || v > 255 {
					isByteArray = false
					break
				}
			}
			if isByteArray {
				bs := make([]byte, len(ev.Items))
				for i, item := range ev.Items {
					bs[i] = byte(item.Integer.Value.Int64())
				}
				return Value{Kind: KindBytes, Bytes: bs}
			}
		}
		// For non-byte integer arrays, we can't represent them in Value directly.
		// Store the ExprValue result - the KST test runner will compare ExprValues.
		if len(ev.Items) == 0 {
			return Value{Kind: KindBytes, Bytes: []byte{}}
		}
		return Value{Kind: KindNone}
	case engine.EnumValueKind:
		if ev.EnumValue != nil {
			return Value{Kind: KindEnum, Int: ev.EnumValue.Value.Int64()}
		}
		if ev.Integer != nil {
			return Value{Kind: KindInt, Int: ev.Integer.Value.Int64()}
		}
	}
	return Value{Kind: KindNone}
}
