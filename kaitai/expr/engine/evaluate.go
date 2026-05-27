package engine

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/types"
)

type CompareMask int

const (
	CompareLessThan CompareMask = 1 << iota
	CompareEqual
	CompareGreaterThan
)

func Evaluate(context *EvalContext, expr *expr.Expr) (*ExprValue, error) {
	val, err := evalNode(context, expr.Root)
	if err != nil {
		return nil, err
	}
	return runtimeVal(context, val)
}

func runtimeVal(context *EvalContext, value *ExprValue) (*ExprValue, error) {
	switch value.Kind {
	case StructParentKind, StructRootKind, StructKind:
		// Struct values that already have populated data (from nodeToExprValue)
		// should be returned as-is. Only look up runtime values for type-level
		// struct symbols.
		if value.Struct != nil && value.Struct.Type != nil {
			rtVal := context.RuntimeValue(value)
			if rtVal != nil {
				return rtVal, nil
			}
		}
		// Already a value-level struct or no runtime value - return as-is
		return value, nil

	case EnumKind, EnumValueKind, ParamKind, AttrKind, InstanceKind, CastedValueKind:
		rtVal := context.RuntimeValue(value)
		if rtVal == nil {
			return nil, fmt.Errorf("no runtime value resolved for %s value", value.Kind)
		}
		return rtVal, nil

	case IntegerKind, FloatKind, BooleanKind, ArrayKind, ByteArrayKind, StringKind, StreamKind, MethodKind:
		return value, nil

	default:
		return nil, fmt.Errorf("unhandled runtime type %s", value.Kind)
	}
}

func evalNode(context *EvalContext, node expr.Node) (*ExprValue, error) {
	switch node := node.(type) {
	case expr.IdentNode:
		val, _ := context.Resolve(node.Identifier)
		if val == nil {
			return nil, fmt.Errorf("unresolved identifier: %s", node.Identifier)
		}
		rtVal := context.RuntimeValue(val)
		if rtVal != nil {
			return rtVal, nil
		}
		// No runtime value available - return the type-level symbol.
		// runtimeVal() at the top level will try to resolve it.
		return val, nil

	case expr.StringNode:
		return NewStringLiteralValue(node.Str), nil

	case expr.IntNode:
		return NewIntegerLiteralValue(node.Integer), nil

	case expr.BoolNode:
		return NewBooleanLiteralValue(node.Bool), nil

	case expr.FloatNode:
		return NewFloatLiteralValue(node.Float), nil

	case expr.ArrayNode:
		var elemKind ExprKind
		elements := []*ExprValue{}
		for _, item := range node.Items {
			element, err := evalNode(context, item)
			if err != nil {
				return nil, err
			}
			if elemKind == InvalidKind {
				elemKind = element.Kind
			} else if elemKind != element.Kind {
				return nil, fmt.Errorf("unexpected type mismatch in array element: %s", item.String())
			}
			elements = append(elements, element)
		}
		elemSym := &ExprValue{Kind: IntegerKind}
		if elemKind != InvalidKind {
			elemSym = &ExprValue{Kind: elemKind}
		}
		// NewArrayLiteralValue requires elem.ValueType() to succeed, which
		// fails for struct-typed arrays. Build the value directly when that
		// happens, populating the array method table.
		if result := NewArrayLiteralValue(NewArrayType(elemSym, nil), elements); result != nil {
			return result, nil
		}
		return &ExprValue{
			Kind:     ArrayKind,
			Array:    &ArrayTypeData{Elem: elemSym},
			Children: ArraySymbolTable(types.Type{TypeRef: &types.TypeRef{Kind: types.User}}),
			Items:    elements,
		}, nil

	case expr.ScopeNode:
		op := resolveTypeOfNode(context.Context, node.Operand)
		if op == nil {
			return nil, fmt.Errorf("unresolved scope: %s", node.Operand.String())
		}
		typ := op.TypeChild(node.Type)
		if typ != nil && typ.Constant != nil {
			return typ.Constant, nil
		}
		opVal := ResultTypeOfNode(context.Context, node.Operand)
		if opVal == nil {
			return nil, fmt.Errorf("unresolved scope value: %s", node.Operand.String())
		}
		val, err := runtimeVal(context, opVal)
		if err != nil {
			return nil, fmt.Errorf("resolving %s in scope %s: %w", node.Type, node.Operand.String(), err)
		}
		return val, nil

	case expr.MemberNode:
		// Evaluate the operand to get its runtime value
		opVal, err := evalNode(context, node.Operand)
		if err != nil {
			return nil, err
		}
		opVal, err = runtimeVal(context, opVal)
		if err != nil {
			// Fallback to type-level resolution if runtime fails
			op := ResultTypeOfNode(context.Context, node.Operand)
			if op == nil {
				return nil, fmt.Errorf("unresolved type: %s", node.Operand.String())
			}
			return op, nil
		}
		// Special intrinsic properties on struct values
		switch node.Property {
		case "_parent":
			if opVal.Parent != nil {
				return opVal.Parent, nil
			}
		case "_root":
			// Walk up the Parent chain to the root
			root := opVal
			for root.Parent != nil {
				root = root.Parent
			}
			if root != opVal {
				return root, nil
			}
		case "_io":
			// The struct's IO stream. Routed through Runtime so the
			// runtime can supply the actual *Stream pointer.
			if opVal.Runtime != nil {
				if rv, ok := opVal.Runtime.LookupChild("_io"); ok && rv != nil {
					return rv, nil
				}
			}
		}
		// Look up the member on the resolved value
		member := opVal.Child(node.Property)
		// If the cached child is a type-level symbol (AttrKind / InstanceKind /
		// ParamKind), the value-level lookup hasn't happened yet. Prefer the
		// Runtime hook to get the actual runtime ExprValue.
		needRuntimeLookup := member == nil ||
			(opVal.Runtime != nil && (member.Kind == AttrKind || member.Kind == InstanceKind))
		if needRuntimeLookup && opVal.Runtime != nil {
			if rv, ok := opVal.Runtime.LookupChild(node.Property); ok {
				if rv == nil {
					// Child exists but is null (e.g. if:false conditional).
					return nil, fmt.Errorf("field %q is null", node.Property)
				}
				// Cache for subsequent access within this evaluation.
				if opVal.Children == nil {
					opVal.Children = map[string]*ExprValue{}
				}
				opVal.Children[node.Property] = rv
				member = rv
			}
		}
		if member != nil {
			// If the member is a property-style method, invoke it immediately.
			// In KS, `.to_i`, `.to_s`, `.length`, etc. are called without parens.
			// Skip auto-invoke for methods that require 2+ arguments (like .substring(from, to)).
			if member.Kind == MethodKind && member.Method != nil && len(member.Method.Arguments) <= 1 {
				fn := getBuiltin(member.Method.Method)
				if fn != nil {
					result, err := fn(opVal, nil)
					if err != nil {
						return nil, fmt.Errorf("calling %s: %w", node.Property, err)
					}
					return result, nil
				}
			}
			return member, nil
		}
		// Fallback: type-level resolution (for methods, etc.)
		op := ResultTypeOfNode(context.Context, node.Operand)
		if op != nil {
			resolved := NewValueOf(context.Context, op)
			if resolved != nil {
				m := resolved.Child(node.Property)
				if m != nil {
					if m.Kind == MethodKind && m.Method != nil && len(m.Method.Arguments) <= 1 {
						fn := getBuiltin(m.Method.Method)
						if fn != nil {
							result, err := fn(opVal, nil)
							if err != nil {
								return nil, fmt.Errorf("calling %s: %w", node.Property, err)
							}
							return result, nil
						}
					}
					return m, nil
				}
			}
		}
		return nil, fmt.Errorf("no member %q on %s", node.Property, opVal.Kind)

	case expr.SubscriptNode:
		opVal, err := evalNode(context, node.A)
		if err != nil {
			return nil, err
		}
		opVal, err = runtimeVal(context, opVal)
		if err != nil {
			return nil, err
		}
		idxVal, err := evalNode(context, node.B)
		if err != nil {
			return nil, err
		}
		idxVal, err = runtimeVal(context, idxVal)
		if err != nil {
			return nil, err
		}
		if opVal.Kind == ArrayKind && idxVal.Kind == IntegerKind {
			idx := int(idxVal.Integer.Value.Int64())
			// Check cached Items first (fast path).
			if idx >= 0 && idx < len(opVal.Items) {
				return opVal.Items[idx], nil
			}
			// Fall back to Runtime hook for lazy arrays.
			if opVal.Runtime != nil {
				if item, ok := opVal.Runtime.LookupIndex(idx); ok && item != nil {
					return item, nil
				}
			}
			return nil, fmt.Errorf("array index %d out of bounds (len %d)", idx, len(opVal.Items))
		}
		if opVal.Kind == ByteArrayKind && idxVal.Kind == IntegerKind {
			idx := int(idxVal.Integer.Value.Int64())
			if idx < 0 || idx >= len(opVal.ByteArray.Value) {
				return nil, fmt.Errorf("byte array index %d out of bounds (len %d)", idx, len(opVal.ByteArray.Value))
			}
			return NewIntegerLiteralValue(big.NewInt(int64(opVal.ByteArray.Value[idx]))), nil
		}
		return nil, fmt.Errorf("subscript on %s not supported", opVal.Kind)

	case expr.CallNode:
		// For method calls (obj.method(args)), handle MemberNode specially
		// to avoid auto-invocation of the method as zero-arg.
		if member, ok := node.Object.(expr.MemberNode); ok {
			baseVal, err := evalNode(context, member.Operand)
			if err != nil {
				return nil, err
			}
			baseVal, err = runtimeVal(context, baseVal)
			if err != nil {
				return nil, err
			}
			// Look up method
			methodSym := baseVal.Child(member.Property)
			if methodSym == nil {
				resolved := NewValueOf(context.Context, ResultTypeOfNode(context.Context, member.Operand))
				if resolved != nil {
					methodSym = resolved.Child(member.Property)
				}
			}
			// For ArrayKind values, also check ByteArraySymbolTable (for .to_s etc.)
			if methodSym == nil && baseVal.Kind == ArrayKind {
				methodSym = ByteArraySymbolTable[member.Property]
			}
			if methodSym != nil && methodSym.Kind == MethodKind && methodSym.Method != nil {
				fn := getBuiltin(methodSym.Method.Method)
				if fn != nil {
					args := make([]*ExprValue, len(node.Args))
					for i, arg := range node.Args {
						args[i], err = evalNode(context, arg)
						if err != nil {
							return nil, err
						}
						args[i], err = runtimeVal(context, args[i])
						if err != nil {
							return nil, err
						}
					}
					return fn(baseVal, args)
				}
			}
		}
		// Fallback for non-MemberNode CallNode: evaluate the object directly
		opVal, err := evalNode(context, node.Object)
		if err != nil {
			return nil, err
		}
		return runtimeVal(context, opVal)

	case expr.CastNode:
		opVal, err := evalNode(context, node.Operand)
		if err != nil {
			return nil, err
		}
		// If the operand exposes a PrimitiveCaster (e.g. an opaque externally
		// defined struct), give it first crack at the cast - it can read
		// the target type directly from its backing stream.
		if opVal != nil && opVal.Runtime != nil {
			if caster, ok := opVal.Runtime.(PrimitiveCaster); ok {
				if rv, ok := caster.CastTo(node.TypeName); ok {
					return rv, nil
				}
			}
		}
		return runtimeVal(context, opVal)

	case expr.FStringNode:
		var result strings.Builder
		for _, part := range node.Parts {
			if part.Expr != nil {
				val, err := evalNode(context, part.Expr)
				if err != nil {
					return nil, err
				}
				val, err = runtimeVal(context, val)
				if err != nil {
					return nil, err
				}
				switch val.Kind {
				case IntegerKind:
					result.WriteString(val.Integer.Value.String())
				case FloatKind:
					result.WriteString(val.Float.Value.String())
				case StringKind:
					result.WriteString(val.String.Value)
				case BooleanKind:
					if val.Boolean.Value {
						result.WriteString("true")
					} else {
						result.WriteString("false")
					}
				default:
					fmt.Fprintf(&result, "%v", val)
				}
			} else {
				result.WriteString(part.Literal)
			}
		}
		return NewStringLiteralValue(result.String()), nil

	case expr.SizeofNode:
		// sizeof<type> returns the byte size of a type, rounded up from its
		// bit size for types that are not byte-aligned.
		if bits, ok := PrimitiveBitSize(node.TypeName); ok {
			return NewIntegerLiteralValue(big.NewInt((bits + 7) / 8)), nil
		}
		// Handle nested type paths like "block::subblock"
		parts := strings.Split(node.TypeName, "::")
		var typ *ExprValue
		for i, part := range parts {
			if i == 0 {
				typ, _ = context.ResolveType(part)
			} else if typ != nil {
				typ = typ.TypeChild(part)
			}
		}
		if typ != nil && typ.Kind == StructKind && typ.Struct != nil && typ.Struct.Type != nil {
			if bits := ComputeStructBitSize(typ.Struct.Type); bits >= 0 {
				return NewIntegerLiteralValue(big.NewInt((bits + 7) / 8)), nil
			}
		}
		return NewIntegerLiteralValue(big.NewInt(0)), nil

	case expr.BitSizeofNode:
		// bitsizeof<type> returns the size of the named type in bits.
		if bits, ok := PrimitiveBitSize(node.TypeName); ok {
			return NewIntegerLiteralValue(big.NewInt(bits)), nil
		}
		parts := strings.Split(node.TypeName, "::")
		var typ *ExprValue
		for i, part := range parts {
			if i == 0 {
				typ, _ = context.ResolveType(part)
			} else if typ != nil {
				typ = typ.TypeChild(part)
			}
		}
		if typ != nil && typ.Kind == StructKind && typ.Struct != nil && typ.Struct.Type != nil {
			if bits := ComputeStructBitSize(typ.Struct.Type); bits >= 0 {
				return NewIntegerLiteralValue(big.NewInt(bits)), nil
			}
		}
		return nil, fmt.Errorf("bitsizeof<%s>: unable to compute size", node.TypeName)

	case expr.UnaryNode:
		return evalUnary(context, node)

	case expr.BinaryNode:
		return evalBinary(context, node)

	case expr.TernaryNode:
		return evalTernary(context, node)
	}

	return nil, fmt.Errorf("unhandled node: %s", node.String())
}

func evalUnary(context *EvalContext, node expr.UnaryNode) (*ExprValue, error) {
	operand, err := evalNode(context, node.Operand)
	if err != nil {
		return nil, err
	}
	operand, err = runtimeVal(context, operand)
	if err != nil {
		return nil, err
	}
	switch node.Op {
	case expr.OpLogicalNot:
		return evalLogicalNot(operand)
	case expr.OpNegate:
		return evalNegate(operand)
	case expr.OpInvert:
		return evalInvert(operand)
	}
	return nil, fmt.Errorf("unhandled unary op: %s", node.Op.String())
}

func evalLogicalNot(operand *ExprValue) (*ExprValue, error) {
	switch operand.Kind {
	case BooleanKind:
		return NewBooleanLiteralValue(!operand.Boolean.Value), nil
	}
	return nil, fmt.Errorf("unhandled unary logical not operand types: %s", operand.Kind)
}

func evalNegate(operand *ExprValue) (*ExprValue, error) {
	switch operand.Kind {
	case IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.Neg(operand.Integer.Value)), nil
	case FloatKind:
		var result big.Float
		return NewFloatLiteralValue(result.Neg(operand.Float.Value)), nil
	}
	return nil, fmt.Errorf("unhandled unary negate operand types: %s", operand.Kind)
}

func evalInvert(operand *ExprValue) (*ExprValue, error) {
	switch operand.Kind {
	case IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.Not(operand.Integer.Value)), nil
	}
	return nil, fmt.Errorf("unhandled unary invert operand types: %s", operand.Kind)
}

func evalBinary(context *EvalContext, node expr.BinaryNode) (*ExprValue, error) {
	a, err := evalNode(context, node.A)
	if err != nil {
		return nil, err
	}
	a, err = runtimeVal(context, a)
	if err != nil {
		return nil, err
	}
	// Short-circuit logical AND/OR - KS semantics evaluate the RHS only
	// when the LHS doesn't already determine the result, which lets
	// patterns like `inst._io.size != 0 and inst.content == 0x66` skip
	// the second clause when the first is false (and inst.content might
	// be null).
	if a.Kind == BooleanKind && a.Boolean != nil {
		if node.Op == expr.OpLogicalAnd && !a.Boolean.Value {
			return NewBooleanLiteralValue(false), nil
		}
		if node.Op == expr.OpLogicalOr && a.Boolean.Value {
			return NewBooleanLiteralValue(true), nil
		}
	}
	b, err := evalNode(context, node.B)
	if err != nil {
		return nil, err
	}
	b, err = runtimeVal(context, b)
	if err != nil {
		return nil, err
	}
	var result *ExprValue
	switch node.Op {
	case expr.OpAdd:
		result, err = evalAdd(a, b)
	case expr.OpSub:
		result, err = evalSub(a, b)
	case expr.OpMult:
		result, err = evalMul(a, b)
	case expr.OpDiv:
		result, err = evalDiv(a, b)
	case expr.OpMod:
		result, err = evalMod(a, b)
	case expr.OpLessThan:
		result, err = evalCmp(a, b, CompareLessThan)
	case expr.OpLessThanEqual:
		result, err = evalCmp(a, b, CompareLessThan|CompareEqual)
	case expr.OpGreaterThan:
		result, err = evalCmp(a, b, CompareGreaterThan)
	case expr.OpGreaterThanEqual:
		result, err = evalCmp(a, b, CompareGreaterThan|CompareEqual)
	case expr.OpEqual:
		result, err = evalCmp(a, b, CompareEqual)
	case expr.OpNotEqual:
		result, err = evalCmp(a, b, CompareLessThan|CompareGreaterThan)
	case expr.OpShiftLeft:
		result, err = evalShl(a, b)
	case expr.OpShiftRight:
		result, err = evalShr(a, b)
	case expr.OpBitAnd:
		result, err = evalBitAnd(a, b)
	case expr.OpBitOr:
		result, err = evalBitOr(a, b)
	case expr.OpBitXor:
		result, err = evalBitXor(a, b)
	case expr.OpLogicalAnd:
		result, err = evalAnd(a, b)
	case expr.OpLogicalOr:
		result, err = evalOr(a, b)
	default:
		return nil, fmt.Errorf("unhandled binary op: %s", node.Op.String())
	}
	if err != nil {
		return nil, err
	}
	if context.Compat.HasCalcIntTypeTruncationBug() && result.Kind == IntegerKind && result.Integer != nil {
		return newSignedInt32IntegerValue(result.Integer.Value), nil
	}
	return result, nil
}

func evalAdd(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Kind == IntegerKind && b.Kind == IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.Add(a.Integer.Value, b.Integer.Value)), nil
	case a.Kind == FloatKind && b.Kind == FloatKind:
		var result big.Float
		return NewFloatLiteralValue(result.Add(a.Float.Value, b.Float.Value)), nil
	case a.Kind == IntegerKind && b.Kind == FloatKind:
		var promotedA, result big.Float
		promotedA.SetInt(a.Integer.Value)
		return NewFloatLiteralValue(result.Add(&promotedA, b.Float.Value)), nil
	case a.Kind == FloatKind && b.Kind == IntegerKind:
		var promotedB, result big.Float
		promotedB.SetInt(b.Integer.Value)
		return NewFloatLiteralValue(result.Add(a.Float.Value, &promotedB)), nil
	case a.Kind == StringKind && b.Kind == StringKind:
		return NewStringLiteralValue(a.String.Value + b.String.Value), nil
	}
	return nil, fmt.Errorf("unhandled addition operand types: %s, %s", a.Kind, b.Kind)
}

func evalSub(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Kind == IntegerKind && b.Kind == IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.Sub(a.Integer.Value, b.Integer.Value)), nil
	case a.Kind == FloatKind && b.Kind == FloatKind:
		var result big.Float
		return NewFloatLiteralValue(result.Sub(a.Float.Value, b.Float.Value)), nil
	case a.Kind == IntegerKind && b.Kind == FloatKind:
		var promotedA, result big.Float
		promotedA.SetInt(a.Integer.Value)
		return NewFloatLiteralValue(result.Sub(&promotedA, b.Float.Value)), nil
	case a.Kind == FloatKind && b.Kind == IntegerKind:
		var promotedB, result big.Float
		promotedB.SetInt(b.Integer.Value)
		return NewFloatLiteralValue(result.Sub(a.Float.Value, &promotedB)), nil
	}
	return nil, fmt.Errorf("unhandled subtraction operand types: %s, %s", a.Kind, b.Kind)
}

func evalMul(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Kind == IntegerKind && b.Kind == IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.Mul(a.Integer.Value, b.Integer.Value)), nil
	case a.Kind == FloatKind && b.Kind == FloatKind:
		var result big.Float
		return NewFloatLiteralValue(result.Mul(a.Float.Value, b.Float.Value)), nil
	case a.Kind == IntegerKind && b.Kind == FloatKind:
		var promotedA, result big.Float
		promotedA.SetInt(a.Integer.Value)
		return NewFloatLiteralValue(result.Mul(&promotedA, b.Float.Value)), nil
	case a.Kind == FloatKind && b.Kind == IntegerKind:
		var promotedB, result big.Float
		promotedB.SetInt(b.Integer.Value)
		return NewFloatLiteralValue(result.Mul(a.Float.Value, &promotedB)), nil
	}
	return nil, fmt.Errorf("unhandled multiplication operand types: %s, %s", a.Kind, b.Kind)
}

func evalDiv(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Kind == IntegerKind && b.Kind == IntegerKind:
		if b.Integer.Value.Sign() == 0 {
			return nil, errors.New("division by zero")
		}
		var result big.Int
		return NewIntegerLiteralValue(result.Div(a.Integer.Value, b.Integer.Value)), nil
	case a.Kind == FloatKind && b.Kind == FloatKind:
		var result big.Float
		return NewFloatLiteralValue(result.Quo(a.Float.Value, b.Float.Value)), nil
	case a.Kind == IntegerKind && b.Kind == FloatKind:
		var promotedA, result big.Float
		promotedA.SetInt(a.Integer.Value)
		return NewFloatLiteralValue(result.Quo(&promotedA, b.Float.Value)), nil
	case a.Kind == FloatKind && b.Kind == IntegerKind:
		var promotedB, result big.Float
		promotedB.SetInt(b.Integer.Value)
		return NewFloatLiteralValue(result.Quo(a.Float.Value, &promotedB)), nil
	}
	return nil, fmt.Errorf("unhandled division operand types: %s, %s", a.Kind, b.Kind)
}

func evalMod(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Kind == IntegerKind && b.Kind == IntegerKind:
		if b.Integer.Value.Sign() == 0 {
			return nil, errors.New("modulus by zero")
		}
		var result big.Int
		// big.Int.Mod computes Euclidean modulus (non-negative), matching
		// Python/Kaitai semantics.
		return NewIntegerLiteralValue(result.Mod(a.Integer.Value, b.Integer.Value)), nil
	}
	return nil, fmt.Errorf("unhandled modulus operand types: %s, %s", a.Kind, b.Kind)
}

func cmpResultToMode(result int) CompareMask {
	switch result {
	case -1:
		return CompareLessThan
	case 0:
		return CompareEqual
	case 1:
		return CompareGreaterThan
	}
	return 0
}

func evalCmp(a *ExprValue, b *ExprValue, maskCheck CompareMask) (*ExprValue, error) {
	cmp, err := Compare(a, b, maskCheck)
	if err != nil {
		return nil, err
	}
	return NewBooleanLiteralValue(cmp), nil
}

// arrayOfIntsToBytes converts an ArrayKind ExprValue whose elements are all
// IntegerKind in the 0..255 range to a byte slice. Returns ok=false if any
// element is non-integer or out of byte range. Lets the engine compare byte
// arrays (e.g. read from a stream) against `[0x4d, 0x4d]`-style literals.
func arrayOfIntsToBytes(arr *ExprValue) ([]byte, bool) {
	out := make([]byte, len(arr.Items))
	for i, item := range arr.Items {
		if item == nil || item.Kind != IntegerKind || item.Integer == nil {
			return nil, false
		}
		v := item.Integer.Value.Int64()
		if v < 0 || v > 255 {
			return nil, false
		}
		out[i] = byte(v)
	}
	return out, true
}

func Compare(a *ExprValue, b *ExprValue, maskCheck CompareMask) (bool, error) {
	var maskResult CompareMask
	switch {
	case a.Kind == IntegerKind && b.Kind == IntegerKind:
		maskResult = cmpResultToMode(a.Integer.Value.Cmp(b.Integer.Value))
	case a.Kind == FloatKind && b.Kind == FloatKind:
		maskResult = cmpResultToMode(a.Float.Value.Cmp(b.Float.Value))
	case a.Kind == IntegerKind && b.Kind == FloatKind:
		var promotedA big.Float
		promotedA.SetInt(a.Integer.Value)
		maskResult = cmpResultToMode(promotedA.Cmp(b.Float.Value))
	case a.Kind == FloatKind && b.Kind == IntegerKind:
		var promotedB big.Float
		promotedB.SetInt(b.Integer.Value)
		maskResult = cmpResultToMode(a.Float.Value.Cmp(&promotedB))
	case a.Kind == StringKind && b.Kind == StringKind:
		maskResult = cmpResultToMode(strings.Compare(a.String.Value, b.String.Value))
	case a.Kind == ByteArrayKind && b.Kind == ByteArrayKind:
		maskResult = cmpResultToMode(bytes.Compare(a.ByteArray.Value, b.ByteArray.Value))
	case a.Kind == StringKind && b.Kind == ByteArrayKind:
		maskResult = cmpResultToMode(bytes.Compare([]byte(a.String.Value), b.ByteArray.Value))
	case a.Kind == ByteArrayKind && b.Kind == StringKind:
		maskResult = cmpResultToMode(bytes.Compare(a.ByteArray.Value, []byte(b.String.Value)))
	case a.Kind == ByteArrayKind && b.Kind == ArrayKind:
		bb, ok := arrayOfIntsToBytes(b)
		if !ok {
			return false, fmt.Errorf("cannot compare ByteArrayKind with array of %s", b.Kind)
		}
		maskResult = cmpResultToMode(bytes.Compare(a.ByteArray.Value, bb))
	case a.Kind == ArrayKind && b.Kind == ByteArrayKind:
		ba, ok := arrayOfIntsToBytes(a)
		if !ok {
			return false, fmt.Errorf("cannot compare array of %s with ByteArrayKind", a.Kind)
		}
		maskResult = cmpResultToMode(bytes.Compare(ba, b.ByteArray.Value))
	case a.Kind == BooleanKind && b.Kind == BooleanKind:
		// Booleans only support == and != - relational comparison is undefined.
		if maskCheck == CompareEqual {
			return a.Boolean.Value == b.Boolean.Value, nil
		}
		if maskCheck == CompareLessThan|CompareGreaterThan {
			return a.Boolean.Value != b.Boolean.Value, nil
		}
		return false, errors.New("invalid comparison for boolean")
	// Enum values get unwrapped to IntegerKind by the bridge before reaching
	// Compare (see eval/value.go KindEnum -> engine.IntegerKind), so there is
	// no EnumKind case here.
	default:
		return false, fmt.Errorf("unhandled comparison types: %s, %s", a.Kind, b.Kind)
	}
	return maskResult&maskCheck != 0, nil
}

func evalShl(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Kind == IntegerKind && b.Kind == IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.Lsh(a.Integer.Value, uint(b.Integer.Value.Uint64()))), nil
	}
	return nil, fmt.Errorf("unhandled left shift operand types: %s, %s", a.Kind, b.Kind)
}

func evalShr(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Kind == IntegerKind && b.Kind == IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.Rsh(a.Integer.Value, uint(b.Integer.Value.Uint64()))), nil
	}
	return nil, fmt.Errorf("unhandled right shift operand types: %s, %s", a.Kind, b.Kind)
}

func newSignedInt32IntegerValue(v *big.Int) *ExprValue {
	var low32 big.Int
	low32.And(v, big.NewInt(0xffffffff))
	if low32.Bit(31) == 1 {
		var mod32 big.Int
		mod32.Lsh(big.NewInt(1), 32)
		low32.Sub(&low32, &mod32)
	}
	return NewIntegerLiteralValue(&low32)
}

func evalBitAnd(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Kind == IntegerKind && b.Kind == IntegerKind:
		var result big.Int
		result.And(a.Integer.Value, b.Integer.Value)
		return NewIntegerLiteralValue(&result), nil
	}
	return nil, fmt.Errorf("unhandled bitwise and operand types: %s, %s", a.Kind, b.Kind)
}

func evalBitOr(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Kind == IntegerKind && b.Kind == IntegerKind:
		var result big.Int
		result.Or(a.Integer.Value, b.Integer.Value)
		return NewIntegerLiteralValue(&result), nil
	}
	return nil, fmt.Errorf("unhandled bitwise or operand types: %s, %s", a.Kind, b.Kind)
}

func evalBitXor(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Kind == IntegerKind && b.Kind == IntegerKind:
		var result big.Int
		result.Xor(a.Integer.Value, b.Integer.Value)
		return NewIntegerLiteralValue(&result), nil
	}
	return nil, fmt.Errorf("unhandled bitwise xor operand types: %s, %s", a.Kind, b.Kind)
}

func evalAnd(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Kind == BooleanKind && b.Kind == BooleanKind:
		return NewBooleanLiteralValue(a.Boolean.Value && b.Boolean.Value), nil
	}
	return nil, fmt.Errorf("unhandled logical and operand types: %s, %s", a.Kind, b.Kind)
}

func evalOr(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Kind == BooleanKind && b.Kind == BooleanKind:
		return NewBooleanLiteralValue(a.Boolean.Value || b.Boolean.Value), nil
	}
	return nil, fmt.Errorf("unhandled logical or operand types: %s, %s", a.Kind, b.Kind)
}

func evalTernary(context *EvalContext, node expr.TernaryNode) (*ExprValue, error) {
	condition, err := evalNode(context, node.A)
	if err != nil {
		return nil, err
	}
	condition, err = runtimeVal(context, condition)
	if err != nil {
		return nil, err
	}
	if condition.Kind != BooleanKind {
		return nil, fmt.Errorf("ternary condition did not evaluate to boolean, got: %s from expression %s", condition.Kind, node.A)
	}
	if condition.Boolean.Value {
		return evalNode(context, node.B)
	} else {
		return evalNode(context, node.C)
	}
}
