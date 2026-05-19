package engine

import (
	"math/big"
	"strings"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/types"
)

func ResultTypeOfExpr(context *Context, e *expr.Expr) *ExprValue {
	return ResultTypeOfNode(context, e.Root)
}

// ResolveTypeOfExpr resolves the type of an expression (for scope resolution
// like enum::value and simple type name lookups).
func ResolveTypeOfExpr(context *Context, e *expr.Expr) *ExprValue {
	return resolveTypeOfNode(context, e.Root)
}

// resolveTypeOfNode resolves the type namespace for an expression node.
// This is used for scope resolution (enum::value) and type lookups.
func resolveTypeOfNode(context *Context, node expr.Node) *ExprValue {
	switch node := node.(type) {
	case expr.IdentNode:
		typ, _ := context.ResolveType(node.Identifier)
		return typ
	case expr.ScopeNode:
		op := resolveTypeOfNode(context, node.Operand)
		if op == nil {
			return nil
		}
		return op.TypeChild(node.Type)
	}
	return nil
}

func ResultTypeOfNode(context *Context, node expr.Node) *ExprValue {
	switch node := node.(type) {
	case expr.IdentNode:
		val, _ := context.Resolve(node.Identifier)
		return val

	case expr.StringNode:
		return NewStringLiteralValue(node.Str)

	case expr.IntNode:
		return NewIntegerLiteralValue(node.Integer)

	case expr.BoolNode:
		return NewBooleanLiteralValue(node.Bool)

	case expr.FloatNode:
		return NewFloatLiteralValue(node.Float)

	case expr.ArrayNode:
		// Determine element type from items
		if len(node.Items) > 0 {
			first := ResultTypeOfNode(context, node.Items[0])
			if first != nil && first.Kind != IntegerKind && first.Kind != ByteArrayKind {
				// Check if all items have the same type
				allSame := true
				for _, item := range node.Items[1:] {
					other := ResultTypeOfNode(context, item)
					if other == nil || other.Kind != first.Kind {
						allSame = false
						break
					}
					// For struct/attr types, check if they refer to the same struct
					if first.Kind == AttrKind || first.Kind == StructKind {
						ft := first
						if ft.Kind == AttrKind && ft.Attr != nil && ft.Attr.Type.TypeRef != nil {
							ot := other
							if ot.Kind == AttrKind && ot.Attr != nil && ot.Attr.Type.TypeRef != nil {
								if ft.Attr.Type.TypeRef.User != nil && ot.Attr.Type.TypeRef.User != nil {
									if ft.Attr.Type.TypeRef.User.Name != ot.Attr.Type.TypeRef.User.Name {
										allSame = false
										break
									}
								}
							}
						}
					}
				}
				if allSame {
					return &ExprValue{Kind: ArrayKind, Array: &ArrayTypeData{Elem: first}}
				}
				// Heterogeneous - return generic array (no specific element type)
				return &ExprValue{Kind: ArrayKind, Array: &ArrayTypeData{}}
			}
		}
		// Default: byte array for integer literals
		return NewByteArrayLiteralValue(nil)

	case expr.ScopeNode:
		op := resolveTypeOfNode(context, node.Operand)
		if op == nil {
			return nil
		}
		typ := op.TypeChild(node.Type)
		if typ == nil {
			return nil
		}
		if typ.Constant != nil {
			return typ.Constant
		}
		return typ

	case expr.MemberNode:
		val := ResultTypeOfNode(context, node.Operand)
		if val == nil {
			return nil
		}
		// Handle special intrinsic properties
		switch node.Property {
		case "_parent":
			// Prefer the ExprValue parent chain (using usage-site analysis)
			if val.Parent != nil {
				return val.Parent
			}
			if val.Kind == StructKind || val.Kind == StructParentKind {
				return NewStructParentValue(val)
			}
		case "_root":
			if val.Kind == StructKind || val.Kind == StructParentKind {
				return NewStructRootValue(val)
			}
		case "_io":
			return NewStreamValue()
		case "_sizeof":
			return NewIntegerLiteralValue(big.NewInt(0))
		}
		// Try the original value's children first (preserves custom symbol tables)
		if child := val.Child(node.Property); child != nil {
			return child
		}
		// Fall back to creating a new value from the type
		val = NewValueOf(context, val)
		if val == nil {
			return nil
		}
		return val.Child(node.Property)

	case expr.UnaryNode:
		if node.Op == expr.OpInvert {
			return NewIntegerLiteralValue(big.NewInt(0))
		}
		return ResultTypeOfNode(context, node.Operand)

	case expr.BinaryNode:
		// Comparison operators always return boolean
		switch node.Op {
		case expr.OpEqual, expr.OpNotEqual,
			expr.OpLessThan, expr.OpLessThanEqual,
			expr.OpGreaterThan, expr.OpGreaterThanEqual:
			return NewBooleanLiteralValue(false)
		case expr.OpLogicalAnd, expr.OpLogicalOr:
			return NewBooleanLiteralValue(false)
		}

		if node.Op == expr.OpShiftLeft || node.Op == expr.OpShiftRight {
			return ResultTypeOfNode(context, node.A)
		}

		a := ResultTypeOfNode(context, node.A)
		b := ResultTypeOfNode(context, node.B)
		if a == nil || b == nil {
			// If we can't determine types, assume arithmetic produces an integer
			return NewIntegerLiteralValue(big.NewInt(0))
		}

		// Resolve method return types for value type inference
		aResolved := a
		if a.Kind == MethodKind && a.Method != nil {
			ret := a.Method.ReturnType
			if ret.Type.TypeRef != nil {
				aResolved = NewValueOfType(context, *ret.Type.TypeRef)
				if aResolved == nil {
					aResolved = a
				}
			}
		}
		bResolved := b
		if b.Kind == MethodKind && b.Method != nil {
			ret := b.Method.ReturnType
			if ret.Type.TypeRef != nil {
				bResolved = NewValueOfType(context, *ret.Type.TypeRef)
				if bResolved == nil {
					bResolved = b
				}
			}
		}

		vta, ok := aResolved.ValueType()
		if !ok || vta.Type.TypeRef == nil || vta.Repeat != nil {
			break
		}
		vtb, ok := bResolved.ValueType()
		if !ok || vtb.Type.TypeRef == nil || vtb.Repeat != nil {
			break
		}

		ka := vta.Type.TypeRef.Kind
		kb := vtb.Type.TypeRef.Kind
		if ka == kb {
			// Same primitive kind on both sides: either operand's type works
			// as the result type. We pick A arbitrarily.
			return ResultTypeOfNode(context, node.A)
		}
		if ka > kb {
			ka, kb = kb, ka
		}
		k := ka.Promote(kb)

		// Mixed-kind arithmetic: promote to the wider type and cast through
		// operand A. The cast is type-only - the actual value comes from the
		// runtime evaluator - so the choice of source operand only affects
		// where Parent/Constant chains attach for downstream method lookups.
		return NewCastedValue(a, ValueType{Type: types.Type{TypeRef: &types.TypeRef{Kind: k}}})

	case expr.TernaryNode:
		// `cond ? B : C` - well-formed KSY requires B and C to share a type,
		// so either branch's type is a valid result type. We pick B.
		return ResultTypeOfNode(context, node.B)

	case expr.CallNode:
		// For method calls, the result type comes from the method's return type
		objResult := ResultTypeOfNode(context, node.Object)
		if objResult != nil && objResult.Kind == MethodKind {
			ret := objResult.Method.ReturnType
			if ret.Type.TypeRef != nil {
				// Construct directly from the return type instead of using NewCastedValue
				// (which fails when source is MethodKind since it has no ValueType)
				return NewValueOfType(context, *ret.Type.TypeRef)
			}
		}
		return objResult

	case expr.CastNode:
		// .as<type> cast - resolve the target type and return a value of that type
		// Handle :: scope separator (e.g., "opcode::strval")
		typeName := node.TypeName
		if strings.Contains(typeName, "::") {
			parts := strings.Split(typeName, "::")
			var resolved *ExprValue
			for i, part := range parts {
				if i == 0 {
					resolved, _ = context.ResolveType(part)
				} else if resolved != nil {
					resolved = resolved.TypeChild(part)
				}
			}
			if resolved != nil {
				return NewValueOf(context, resolved)
			}
		} else {
			val, _ := context.Resolve(typeName)
			typ, _ := context.ResolveType(typeName)
			if val != nil {
				return val
			}
			if typ != nil {
				return NewValueOf(context, typ)
			}
		}
		// Handle array type casts like .as<u2[]>
		if strings.HasSuffix(typeName, "[]") {
			baseType := typeName[:len(typeName)-2]
			ref, err := types.ParseTypeRef(baseType)
			if err == nil {
				elemVal := &ExprValue{
					Kind: AttrKind,
					Attr: &kaitai.Attr{Type: types.Type{TypeRef: &ref}},
				}
				return &ExprValue{Kind: ArrayKind, Array: &ArrayTypeData{Elem: elemVal}}
			}
		}
		// Handle primitive type casts like .as<u8>, .as<s4>, .as<f4>
		ref, err := types.ParseTypeRef(typeName)
		if err == nil && ref.Kind != types.User {
			operand := ResultTypeOfNode(context, node.Operand)
			if operand != nil {
				cast := NewCastedValue(operand, ValueType{Type: types.Type{TypeRef: &ref}})
				if cast != nil {
					return cast
				}
			}
			// Operand couldn't be resolved - return a typed attr value
			// that preserves the exact type (e.g., U8 -> uint64, not just int)
			return &ExprValue{
				Kind: AttrKind,
				Attr: &kaitai.Attr{Type: types.Type{TypeRef: &ref}},
			}
		}
		// Fallback: return the operand's type
		return ResultTypeOfNode(context, node.Operand)

	case expr.SubscriptNode:
		// Subscript on an array returns the element type
		arrResult := ResultTypeOfNode(context, node.A)
		if arrResult != nil {
			switch arrResult.Kind {
			case AttrKind:
				// Array attr - get the element type (without repeat)
				if arrResult.Attr != nil && arrResult.Attr.Type.TypeRef != nil {
					return NewValueOfType(context, *arrResult.Attr.Type.TypeRef)
				}
			case InstanceKind:
				if arrResult.Instance != nil && arrResult.Instance.Type.TypeRef != nil {
					return NewValueOfType(context, *arrResult.Instance.Type.TypeRef)
				}
			case ByteArrayKind:
				return NewIntegerLiteralValue(big.NewInt(0))
			case ParamKind:
				if arrResult.Param != nil && arrResult.Param.Type.IsArray {
					elemRef := arrResult.Param.Type
					elemRef.IsArray = false
					return NewValueOfType(context, elemRef)
				}
			}
		}
		return nil

	case expr.FStringNode:
		// f-strings produce a string value
		return NewStringLiteralValue("")

	case expr.SizeofNode:
		// sizeof<type> returns an integer
		return NewIntegerLiteralValue(big.NewInt(0))
	}
	return nil
}
