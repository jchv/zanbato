package engine

import (
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/types"
)

type ExprResultType struct {
	typ *ExprType
	val *ExprValue
}

// Value returns the expression value type of the expression. If this function
// returns non-nil, it is valid to use this expression in contexts that need a
// value.
func (e ExprResultType) Value() *ExprValue {
	return e.val
}

func (e ExprResultType) Type() *ExprType {
	return e.typ
}

func ResultTypeOfExpr(context *Context, e *expr.Expr) ExprResultType {
	return ResultTypeOfNode(context, e.Root)
}

func ResultTypeOfNode(context *Context, node expr.Node) ExprResultType {
	switch node := node.(type) {
	case expr.IdentNode:
		val, _ := context.Resolve(node.Identifier)
		typ, _ := context.ResolveType(node.Identifier)
		return ExprResultType{val: val, typ: typ}

	case expr.StringNode:
		return ExprResultType{val: NewStringLiteralValue(node.Str)}

	case expr.IntNode:
		return ExprResultType{val: NewIntegerLiteralValue(node.Integer)}

	case expr.BoolNode:
		return ExprResultType{val: NewBooleanLiteralValue(node.Bool)}

	case expr.FloatNode:
		return ExprResultType{val: NewFloatLiteralValue(node.Float)}

	case expr.ScopeNode:
		op := ResultTypeOfNode(context, node.Operand)
		if op.typ == nil {
			return ExprResultType{}
		}
		typ := op.typ.Child(node.Type)
		if typ.Constant != nil {
			return ExprResultType{val: typ.Constant}
		}
		return ExprResultType{typ: typ}

	case expr.MemberNode:
		val := ResultTypeOfNode(context, node.Operand).Value()
		if val == nil {
			return ExprResultType{}
		}
		val = NewValueOf(context, val.Type)
		if val == nil {
			return ExprResultType{}
		}
		return ExprResultType{val: val.Child(node.Property)}

	case expr.UnaryNode:
		return ResultTypeOfNode(context, node.Operand)

	case expr.BinaryNode:
		if node.Op == expr.OpShiftLeft || node.Op == expr.OpShiftRight {
			return ResultTypeOfNode(context, node.A)
		}

		a := ResultTypeOfNode(context, node.A).Value()
		b := ResultTypeOfNode(context, node.B).Value()
		if a == nil || b == nil {
			// TODO: need better solution than passing first operand
			return ResultTypeOfNode(context, node.A)
		}

		vta, ok := a.Type.ValueType()
		if !ok || vta.Type.TypeRef == nil || vta.Repeat != nil {
			break
		}
		vtb, ok := b.Type.ValueType()
		if !ok || vtb.Type.TypeRef == nil || vtb.Repeat != nil {
			break
		}

		ka := vta.Type.TypeRef.Kind
		kb := vtb.Type.TypeRef.Kind
		if ka == kb {
			// TODO: need better solution than passing first operand
			return ResultTypeOfNode(context, node.A)
		}
		if ka > kb {
			ka, kb = kb, ka
		}
		k := ka.Promote(kb)

		// TODO: need better solution than passing first operand
		return ExprResultType{val: NewCastedValue(a, ValueType{Type: types.Type{TypeRef: &types.TypeRef{Kind: k}}})}

	case expr.TernaryNode:
		// TODO: need better solution than passing second operand
		return ResultTypeOfNode(context, node.B)
	}
	return ExprResultType{}
}
