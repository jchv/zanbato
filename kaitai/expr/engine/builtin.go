package engine

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
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
	// `.to_s(base)` formats the integer in the requested base; the no-arg
	// form (`.to_s`) defaults to base 10.
	base := 10
	if len(args) > 0 && args[0] != nil && args[0].Kind == IntegerKind && args[0].Integer != nil {
		b := int(args[0].Integer.Value.Int64())
		if b >= 2 && b <= 36 {
			base = b
		}
	}
	return NewStringLiteralValue(this.Integer.Value.Text(base)), nil
}

func builtinMethodFloatToInt(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	i, _ := this.Float.Value.Int(nil)
	return NewIntegerLiteralValue(i), nil
}

func builtinMethodByteArrayLength(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	return NewIntegerLiteralValue(big.NewInt(int64(len(this.ByteArray.Value)))), nil
}

func builtinMethodByteArrayToString(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	if this == nil {
		return nil, errors.New("to_s on nil value")
	}
	if this.ByteArray != nil {
		return NewStringLiteralValue(string(this.ByteArray.Value)), nil
	}
	// Handle ArrayKind of integers (byte arrays stored as int arrays)
	if this.Kind == ArrayKind && len(this.Items) > 0 {
		bs := make([]byte, len(this.Items))
		for i, item := range this.Items {
			if item.Kind == IntegerKind && item.Integer != nil {
				bs[i] = byte(item.Integer.Value.Int64())
			}
		}
		return NewStringLiteralValue(string(bs)), nil
	}
	return nil, errors.New("to_s on non-byte-array value")
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
	if len(args) < 2 {
		return nil, errors.New("substring requires 2 arguments")
	}
	s := this.String.Value
	from := int(args[0].Integer.Value.Int64())
	to := int(args[1].Integer.Value.Int64())
	if from < 0 {
		from = 0
	}
	if to > len(s) {
		to = len(s)
	}
	if from > to {
		from = to
	}
	return NewStringLiteralValue(s[from:to]), nil
}

func builtinMethodStringToInt(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	s := this.String.Value
	base := 10
	if len(args) > 0 && args[0] != nil && args[0].Integer != nil {
		base = int(args[0].Integer.Value.Int64())
	}
	// Strip leading whitespace. KS semantics raise ConversionError on
	// trailing non-digit characters, so we don't fall back to prefix parsing.
	str := strings.TrimSpace(s)
	val, err := strconv.ParseInt(str, base, 64)
	if err != nil {
		return nil, fmt.Errorf("ConversionError: %q is not a valid integer (base %d)", s, base)
	}
	return NewIntegerLiteralValue(big.NewInt(val)), nil
}

func builtinMethodEnumToInt(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	if this.EnumValue != nil {
		return NewIntegerLiteralValue(this.EnumValue.Value), nil
	}
	// For enum values stored as IntegerKind (from runtime evaluator)
	if this.Integer != nil {
		return NewIntegerLiteralValue(this.Integer.Value), nil
	}
	return nil, errors.New("enum value has no integer representation")
}

func builtinMethodBoolToInt(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	if this.Boolean.Value {
		return NewIntegerLiteralValue(big.NewInt(1)), nil
	} else {
		return NewIntegerLiteralValue(big.NewInt(0)), nil
	}
}

func builtinMethodStreamEOF(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	retVal, err := this.Stream.Stream.EOF()
	if err != nil {
		return nil, err
	}
	return NewBooleanLiteralValue(retVal), nil
}

func builtinMethodStreamSize(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	retVal, err := this.Stream.Stream.Size()
	if err != nil {
		return nil, err
	}
	return NewIntegerLiteralValue(big.NewInt(retVal)), nil
}

func builtinMethodStreamPos(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	retVal, err := this.Stream.Stream.Pos()
	if err != nil {
		return nil, err
	}
	return NewIntegerLiteralValue(big.NewInt(retVal)), nil
}

func builtinMethodArrayFirst(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	if this.ByteArray != nil && len(this.ByteArray.Value) > 0 {
		return NewIntegerLiteralValue(big.NewInt(int64(this.ByteArray.Value[0]))), nil
	}
	if len(this.Items) > 0 {
		return this.Items[0], nil
	}
	return nil, nil
}

func builtinMethodArrayLast(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	if this.ByteArray != nil && len(this.ByteArray.Value) > 0 {
		return NewIntegerLiteralValue(big.NewInt(int64(this.ByteArray.Value[len(this.ByteArray.Value)-1]))), nil
	}
	if len(this.Items) > 0 {
		return this.Items[len(this.Items)-1], nil
	}
	return nil, nil
}

func builtinMethodArraySize(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	if this.ByteArray != nil {
		return NewIntegerLiteralValue(big.NewInt(int64(len(this.ByteArray.Value)))), nil
	}
	if this.Runtime != nil {
		if n, ok := this.Runtime.Len(); ok {
			return NewIntegerLiteralValue(big.NewInt(int64(n))), nil
		}
	}
	return NewIntegerLiteralValue(big.NewInt(int64(len(this.Items)))), nil
}

func builtinMethodArrayMin(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	if this.ByteArray != nil {
		bs := this.ByteArray.Value
		if len(bs) == 0 {
			return nil, errors.New("min on empty byte array")
		}
		m := bs[0]
		for _, b := range bs[1:] {
			if b < m {
				m = b
			}
		}
		return NewIntegerLiteralValue(big.NewInt(int64(m))), nil
	}
	if len(this.Items) == 0 {
		return nil, errors.New("min on empty array")
	}
	min := this.Items[0]
	for _, item := range this.Items[1:] {
		less, err := Compare(item, min, CompareLessThan)
		if err == nil && less {
			min = item
		}
	}
	return min, nil
}

func builtinMethodArrayMax(this *ExprValue, args []*ExprValue) (*ExprValue, error) {
	if this.ByteArray != nil {
		bs := this.ByteArray.Value
		if len(bs) == 0 {
			return nil, errors.New("max on empty byte array")
		}
		m := bs[0]
		for _, b := range bs[1:] {
			if b > m {
				m = b
			}
		}
		return NewIntegerLiteralValue(big.NewInt(int64(m))), nil
	}
	if len(this.Items) == 0 {
		return nil, errors.New("max on empty array")
	}
	max := this.Items[0]
	for _, item := range this.Items[1:] {
		greater, err := Compare(item, max, CompareGreaterThan)
		if err == nil && greater {
			max = item
		}
	}
	return max, nil
}
