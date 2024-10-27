package engine

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/jchv/zanbato/kaitai/expr"
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
	switch value.Type.Kind {
	case StructParentKind, StructRootKind, StructKind, EnumKind, EnumValueKind, ParamKind, AttrKind, InstanceKind, CastedValueKind:
		rtVal := context.RuntimeValue(value)
		if rtVal == nil {
			return nil, fmt.Errorf("no runtime value resolved for %s value", value.Type.Kind)
		}
		return rtVal, nil

	case IntegerKind, FloatKind, BooleanKind, ArrayKind, ByteArrayKind, StringKind:
		return value, nil

	default:
		return nil, fmt.Errorf("unhandled runtime type %s", value.Type.Kind)
	}
}

func evalNode(context *EvalContext, node expr.Node) (*ExprValue, error) {
	switch node := node.(type) {
	case expr.IdentNode:
		val, _ := context.Resolve(node.Identifier)
		if val == nil {
			return nil, fmt.Errorf("unresolved identifier: %s", node.Identifier)
		}
		val = context.RuntimeValue(val)
		return val, nil

	case expr.StringNode:
		return NewStringLiteralValue(node.Str), nil

	case expr.IntNode:
		return NewIntegerLiteralValue(node.Integer), nil

	case expr.BoolNode:
		return NewBooleanLiteralValue(node.Bool), nil

	case expr.FloatNode:
		return NewFloatLiteralValue(node.Float), nil

	case expr.ScopeNode:
		op := ResultTypeOfNode(context.Context, node.Operand)
		if op.typ == nil {
			return nil, fmt.Errorf("unresolved scope: %s", node.Operand.String())
		}
		typ := op.typ.Child(node.Type)
		if typ.Constant != nil {
			return typ.Constant, nil
		}
		val, err := runtimeVal(context, op.val)
		if err != nil {
			return nil, fmt.Errorf("resolving %s in scope %s: %w", node.Type, node.Operand.String(), err)
		}
		return val, nil

	case expr.MemberNode:
		op := ResultTypeOfNode(context.Context, node.Operand)
		if op.val == nil {
			return nil, fmt.Errorf("unresolved type: %s", node.Operand.String())
		}
		return op.val, nil

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
		evalLogicalNot(operand)
	}
	return nil, fmt.Errorf("unhandled unary op: %s", node.Op.String())
}

func evalLogicalNot(operand *ExprValue) (*ExprValue, error) {
	switch operand.Type.Kind {
	case BooleanKind:
		return NewBooleanLiteralValue(!operand.Boolean.Value), nil
	}
	return nil, fmt.Errorf("unhandled unary logical not operand types: %s", operand.Type.Kind)
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
	b, err := evalNode(context, node.B)
	if err != nil {
		return nil, err
	}
	b, err = runtimeVal(context, b)
	if err != nil {
		return nil, err
	}
	switch node.Op {
	case expr.OpAdd:
		return evalAdd(a, b)
	case expr.OpSub:
		return evalSub(a, b)
	case expr.OpMult:
		return evalMul(a, b)
	case expr.OpDiv:
		return evalDiv(a, b)
	case expr.OpMod:
		return evalMod(a, b)
	case expr.OpLessThan:
		return evalCmp(a, b, CompareLessThan)
	case expr.OpLessThanEqual:
		return evalCmp(a, b, CompareLessThan|CompareEqual)
	case expr.OpGreaterThan:
		return evalCmp(a, b, CompareGreaterThan)
	case expr.OpGreaterThanEqual:
		return evalCmp(a, b, CompareGreaterThan|CompareEqual)
	case expr.OpEqual:
		return evalCmp(a, b, CompareEqual)
	case expr.OpNotEqual:
		return evalCmp(a, b, CompareLessThan|CompareGreaterThan)
	case expr.OpShiftLeft:
		return evalShl(a, b)
	case expr.OpShiftRight:
		return evalShr(a, b)
	case expr.OpBitAnd:
		return evalBitAnd(a, b)
	case expr.OpBitOr:
		return evalBitOr(a, b)
	case expr.OpBitXor:
		return evalBitXor(a, b)
	case expr.OpLogicalAnd:
		return evalAnd(a, b)
	case expr.OpLogicalOr:
		return evalOr(a, b)
	}
	return nil, fmt.Errorf("unhandled binary op: %s", node.Op.String())
}

func evalAdd(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Type.Kind == IntegerKind && b.Type.Kind == IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.Add(a.Integer.Value, b.Integer.Value)), nil
	case a.Type.Kind == FloatKind && b.Type.Kind == FloatKind:
		var result big.Float
		return NewFloatLiteralValue(result.Add(a.Float.Value, b.Float.Value)), nil
	case a.Type.Kind == IntegerKind && b.Type.Kind == FloatKind:
		var promotedA, result big.Float
		promotedA.SetInt(a.Integer.Value)
		return NewFloatLiteralValue(result.Add(&promotedA, b.Float.Value)), nil
	case a.Type.Kind == FloatKind && b.Type.Kind == IntegerKind:
		var promotedB, result big.Float
		promotedB.SetInt(b.Integer.Value)
		return NewFloatLiteralValue(result.Add(a.Float.Value, &promotedB)), nil
	case a.Type.Kind == StringKind && b.Type.Kind == StringKind:
		return NewStringLiteralValue(a.String.Value + b.String.Value), nil
	}
	return nil, fmt.Errorf("unhandled addition operand types: %s, %s", a.Type.Kind, b.Type.Kind)
}

func evalSub(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Type.Kind == IntegerKind && b.Type.Kind == IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.Sub(a.Integer.Value, b.Integer.Value)), nil
	case a.Type.Kind == FloatKind && b.Type.Kind == FloatKind:
		var result big.Float
		return NewFloatLiteralValue(result.Sub(a.Float.Value, b.Float.Value)), nil
	case a.Type.Kind == IntegerKind && b.Type.Kind == FloatKind:
		var promotedA, result big.Float
		promotedA.SetInt(a.Integer.Value)
		return NewFloatLiteralValue(result.Sub(&promotedA, b.Float.Value)), nil
	case a.Type.Kind == FloatKind && b.Type.Kind == IntegerKind:
		var promotedB, result big.Float
		promotedB.SetInt(b.Integer.Value)
		return NewFloatLiteralValue(result.Sub(a.Float.Value, &promotedB)), nil
	}
	return nil, fmt.Errorf("unhandled subtraction operand types: %s, %s", a.Type.Kind, b.Type.Kind)
}

func evalMul(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Type.Kind == IntegerKind && b.Type.Kind == IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.Mul(a.Integer.Value, b.Integer.Value)), nil
	case a.Type.Kind == FloatKind && b.Type.Kind == FloatKind:
		var result big.Float
		return NewFloatLiteralValue(result.Mul(a.Float.Value, b.Float.Value)), nil
	case a.Type.Kind == IntegerKind && b.Type.Kind == FloatKind:
		var promotedA, result big.Float
		promotedA.SetInt(a.Integer.Value)
		return NewFloatLiteralValue(result.Mul(&promotedA, b.Float.Value)), nil
	case a.Type.Kind == FloatKind && b.Type.Kind == IntegerKind:
		var promotedB, result big.Float
		promotedB.SetInt(b.Integer.Value)
		return NewFloatLiteralValue(result.Mul(a.Float.Value, &promotedB)), nil
	}
	return nil, fmt.Errorf("unhandled multiplication operand types: %s, %s", a.Type.Kind, b.Type.Kind)
}

func evalDiv(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Type.Kind == IntegerKind && b.Type.Kind == IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.Div(a.Integer.Value, b.Integer.Value)), nil
	case a.Type.Kind == FloatKind && b.Type.Kind == FloatKind:
		var result big.Float
		return NewFloatLiteralValue(result.Quo(a.Float.Value, b.Float.Value)), nil
	case a.Type.Kind == IntegerKind && b.Type.Kind == FloatKind:
		var promotedA, result big.Float
		promotedA.SetInt(a.Integer.Value)
		return NewFloatLiteralValue(result.Quo(&promotedA, b.Float.Value)), nil
	case a.Type.Kind == FloatKind && b.Type.Kind == IntegerKind:
		var promotedB, result big.Float
		promotedB.SetInt(b.Integer.Value)
		return NewFloatLiteralValue(result.Quo(a.Float.Value, &promotedB)), nil
	}
	return nil, fmt.Errorf("unhandled division operand types: %s, %s", a.Type.Kind, b.Type.Kind)
}

func evalMod(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Type.Kind == IntegerKind && b.Type.Kind == IntegerKind:
		// TODO: need to fix divide-by-zero, wrong modulus implementation
		var result big.Int
		return NewIntegerLiteralValue(result.Mod(a.Integer.Value, b.Integer.Value)), nil
	}
	return nil, fmt.Errorf("unhandled modulus operand types: %s, %s", a.Type.Kind, b.Type.Kind)
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

func Compare(a *ExprValue, b *ExprValue, maskCheck CompareMask) (bool, error) {
	var maskResult CompareMask
	switch {
	case a.Type.Kind == IntegerKind && b.Type.Kind == IntegerKind:
		maskResult = cmpResultToMode(a.Integer.Value.Cmp(b.Integer.Value))
	case a.Type.Kind == FloatKind && b.Type.Kind == FloatKind:
		maskResult = cmpResultToMode(a.Float.Value.Cmp(b.Float.Value))
	case a.Type.Kind == IntegerKind && b.Type.Kind == FloatKind:
		var promotedA big.Float
		promotedA.SetInt(b.Integer.Value)
		maskResult = cmpResultToMode(promotedA.Cmp(a.Float.Value))
	case a.Type.Kind == FloatKind && b.Type.Kind == IntegerKind:
		var promotedB big.Float
		promotedB.SetInt(b.Integer.Value)
		maskResult = cmpResultToMode(a.Float.Value.Cmp(&promotedB))
	case a.Type.Kind == StringKind && b.Type.Kind == StringKind:
		maskResult = cmpResultToMode(strings.Compare(a.String.Value, b.String.Value))
	case a.Type.Kind == ByteArrayKind && b.Type.Kind == ByteArrayKind:
		maskResult = cmpResultToMode(bytes.Compare(a.ByteArray.Value, b.ByteArray.Value))
	case a.Type.Kind == StringKind && b.Type.Kind == ByteArrayKind:
		maskResult = cmpResultToMode(bytes.Compare([]byte(a.String.Value), b.ByteArray.Value))
	case a.Type.Kind == ByteArrayKind && b.Type.Kind == StringKind:
		maskResult = cmpResultToMode(bytes.Compare(a.ByteArray.Value, []byte(b.String.Value)))
	case a.Type.Kind == BooleanKind && b.Type.Kind == BooleanKind:
		// TODO: cast to bool?
		if maskCheck == CompareEqual {
			return a.Boolean.Value == b.Boolean.Value, nil
		}
		if maskCheck == CompareLessThan|CompareGreaterThan {
			return a.Boolean.Value != b.Boolean.Value, nil
		}
		return false, errors.New("invalid comparison for boolean")
		// TODO: enums? etc.
	default:
		return false, fmt.Errorf("todo: unhandled comparison types: %s, %s", a.Type.Kind, b.Type.Kind)
	}
	return maskResult&maskCheck != 0, nil
}

func evalShl(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Type.Kind == IntegerKind && b.Type.Kind == IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.Lsh(a.Integer.Value, uint(b.Integer.Value.Uint64()))), nil
	}
	return nil, fmt.Errorf("unhandled left shift operand types: %s, %s", a.Type.Kind, b.Type.Kind)
}

func evalShr(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Type.Kind == IntegerKind && b.Type.Kind == IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.Rsh(a.Integer.Value, uint(b.Integer.Value.Uint64()))), nil
	}
	return nil, fmt.Errorf("unhandled right shift operand types: %s, %s", a.Type.Kind, b.Type.Kind)
}

func evalBitAnd(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Type.Kind == IntegerKind && b.Type.Kind == IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.And(a.Integer.Value, b.Integer.Value)), nil
	}
	return nil, fmt.Errorf("unhandled bitwise and operand types: %s, %s", a.Type.Kind, b.Type.Kind)
}

func evalBitOr(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Type.Kind == IntegerKind && b.Type.Kind == IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.Or(a.Integer.Value, b.Integer.Value)), nil
	}
	return nil, fmt.Errorf("unhandled bitwise or operand types: %s, %s", a.Type.Kind, b.Type.Kind)
}

func evalBitXor(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Type.Kind == IntegerKind && b.Type.Kind == IntegerKind:
		var result big.Int
		return NewIntegerLiteralValue(result.Xor(a.Integer.Value, b.Integer.Value)), nil
	}
	return nil, fmt.Errorf("unhandled bitwise xor operand types: %s, %s", a.Type.Kind, b.Type.Kind)
}

func evalAnd(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Type.Kind == BooleanKind && b.Type.Kind == BooleanKind:
		return NewBooleanLiteralValue(a.Boolean.Value && b.Boolean.Value), nil
	}
	return nil, fmt.Errorf("unhandled logical and operand types: %s, %s", a.Type.Kind, b.Type.Kind)
}

func evalOr(a *ExprValue, b *ExprValue) (*ExprValue, error) {
	switch {
	case a.Type.Kind == BooleanKind && b.Type.Kind == BooleanKind:
		return NewBooleanLiteralValue(a.Boolean.Value || b.Boolean.Value), nil
	}
	return nil, fmt.Errorf("unhandled logical or operand types: %s, %s", a.Type.Kind, b.Type.Kind)
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
	if condition.Type.Kind != BooleanKind {
		return nil, fmt.Errorf("ternary condition did not evaluate to boolean, got: %s from expression %s", condition.Type.Kind, node.A)
	}
	if condition.Boolean.Value {
		return evalNode(context, node.B)
	} else {
		return evalNode(context, node.C)
	}
}
