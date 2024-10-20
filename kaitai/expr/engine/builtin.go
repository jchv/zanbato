package engine

import (
	"errors"
	"math/big"
	"unicode/utf8"
)

func getBuiltin(builtin BuiltinMethod) MethodFn {
	switch builtin {
	case MethodIntToString:
		return builtinMethodIntToString
	case MethodFloatToInt:
		return builtinMethodFloatToInt
	case MethodByteArrayLength:
		return builtinMethodByteArrayLength
	case MethodByteArrayToString:
		return builtinMethodByteArrayToString
	case MethodStringLength:
		return builtinMethodStringLength
	case MethodStringReverse:
		return builtinMethodStringReverse
	case MethodStringSubstring:
		return builtinMethodStringSubstring
	case MethodStringToInt:
		return builtinMethodStringToInt
	case MethodEnumToInt:
		return builtinMethodEnumToInt
	case MethodBoolToInt:
		return builtinMethodBoolToInt
	case MethodStreamEOF:
		return builtinMethodStreamEOF
	case MethodStreamSize:
		return builtinMethodStreamSize
	case MethodStreamPos:
		return builtinMethodStreamPos
	case MethodArrayFirst:
		return builtinMethodArrayFirst
	case MethodArrayLast:
		return builtinMethodArrayLast
	case MethodArraySize:
		return builtinMethodArraySize
	case MethodArrayMin:
		return builtinMethodArrayMin
	case MethodArrayMax:
		return builtinMethodArrayMax
	}
	return nil
}

func builtinMethodIntToString(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	return NewStringLiteralValue(this.Integer.Value.String()), nil
}

func builtinMethodFloatToInt(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	i, _ := this.Float.Value.Int(nil)
	return NewIntegerLiteralValue(i), nil
}

func builtinMethodByteArrayLength(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	return NewIntegerLiteralValue(big.NewInt(int64(len(this.ByteArray.Value)))), nil
}

func builtinMethodByteArrayToString(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	return nil, errors.New("not implemented")
}

func builtinMethodStringLength(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	return NewIntegerLiteralValue(big.NewInt(int64(len(this.String.Value)))), nil
}

func builtinMethodStringReverse(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	val := this.String.Value
	len := len(val)
	buf := make([]byte, len)
	for i := 0; i < len; {
		r, size := utf8.DecodeRuneInString(val[i:])
		i += size
		utf8.EncodeRune(buf[len-i:], r)
	}
	return NewStringLiteralValue(string(buf)), nil
}

func builtinMethodStringSubstring(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	return nil, errors.New("not implemented")
}

func builtinMethodStringToInt(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	return nil, errors.New("not implemented")
}

func builtinMethodEnumToInt(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	return NewIntegerLiteralValue(this.Type.EnumValue.Value), nil
}

func builtinMethodBoolToInt(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	if this.Boolean.Value {
		return NewIntegerLiteralValue(big.NewInt(1)), nil
	} else {
		return NewIntegerLiteralValue(big.NewInt(0)), nil
	}
}

func builtinMethodStreamEOF(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	return nil, errors.New("not implemented")
}

func builtinMethodStreamSize(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	return nil, errors.New("not implemented")
}

func builtinMethodStreamPos(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	return nil, errors.New("not implemented")
}

func builtinMethodArrayFirst(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	if len(this.Array.Value) > 0 {
		return this.Array.Value[0], nil
	}
	return nil, nil
}

func builtinMethodArrayLast(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	if len(this.Array.Value) > 0 {
		return this.Array.Value[len(this.Array.Value)-1], nil
	}
	return nil, nil
}

func builtinMethodArraySize(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	return NewIntegerLiteralValue(big.NewInt(int64(len(this.Array.Value)))), nil
}

func builtinMethodArrayMin(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	return nil, errors.New("not implemented")
}

func builtinMethodArrayMax(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	return nil, errors.New("not implemented")
}
