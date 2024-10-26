package engine

import (
	"math/big"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/types"
)

//go:generate go run golang.org/x/tools/cmd/stringer -type=BuiltinMethod,ExprKind -output context_string.go

// BuiltinMethod indexes the built-in methods.
type BuiltinMethod int

const (
	InvalidMethod BuiltinMethod = iota

	// MethodIntToString converts an integer into a string using decimal
	// representation.
	MethodIntToString

	// MethodFloatToInt truncates a floating point number to an integer.
	MethodFloatToInt

	// MethodByteArrayLength gets the length of a byte array in bytes.
	MethodByteArrayLength

	// MethodByteArrayToString converts a byte array to a string. The first
	// parameter specifies what encoding the byte array should be decoded
	// using.
	MethodByteArrayToString

	// MethodStringLength gets the length of a string in characters.
	MethodStringLength

	// MethodStringReverse reverses a string character-by-character.
	MethodStringReverse

	// MethodStringSubstring extracts a substring using two indexes, one
	// specifying the (inclusive) start index, and the other the (exclusive)
	// end index.
	MethodStringSubstring

	// MethodStringToInt converts a string to an integer. A radix parameter may
	// optionally be provided; if it is omitted, 10 will be assumed.
	MethodStringToInt

	// MethodEnumToInt gets the corresponding integer value for a given enum
	// value.
	MethodEnumToInt

	// MethodBoolToInt returns 0 for false boolean values and 1 for true boolean
	// values.
	MethodBoolToInt

	// MethodArrayFirst gets the first element in an array.
	MethodArrayFirst

	// MethodArrayLast gets the last element in an array.
	MethodArrayLast

	// MethodArraySize gets the size of an array.
	MethodArraySize

	// MethodArrayMin gets the minimum element of the array.
	MethodArrayMin

	// MethodArrayMax gets the maximum element of the array.
	MethodArrayMax

	// MethodStreamEOF returns true if the stream is at EOF position.
	MethodStreamEOF

	// MethodStreamSize returns the total size of the stream, in bytes.
	MethodStreamSize

	// MethodStreamPos returns the current position in the stream, in bytes.
	MethodStreamPos
)

// ValueType represents a concrete type, e.g. one that resolves into generated
// code types.
type ValueType struct {
	Type   types.Type
	Repeat types.RepeatType
}

// IntegerValueType is the type used for integer values in symbols.
var IntegerValueType = ValueType{Type: types.Type{TypeRef: &types.TypeRef{Kind: types.UntypedInt}}}

// FloatValueType is the type used for float values in symbols.
var FloatValueType = ValueType{Type: types.Type{TypeRef: &types.TypeRef{Kind: types.UntypedFloat}}}

// ByteArrayValueType is the type used for byte array values in symbols.
var ByteArrayValueType = ValueType{Type: types.Type{TypeRef: &types.TypeRef{Kind: types.Bytes}}}

// StringValueType is the type used for integer values in symbols.
var StringValueType = ValueType{Type: types.Type{TypeRef: &types.TypeRef{Kind: types.String}}}

// BooleanValueType is the type used for boolean values in symbols.
var BooleanValueType = ValueType{Type: types.Type{TypeRef: &types.TypeRef{Kind: types.UntypedBool}}}

// IntegerSymbolTable is the static symbol table of integer values.
var IntegerSymbolTable = map[string]*ExprValue{
	"to_s": NewBuiltinMethodValue(MethodIntToString, []ValueType{}, StringValueType),
}

// FloatSymbolTable is the static symbol table of floating point values.
var FloatSymbolTable = map[string]*ExprValue{
	"to_i": NewBuiltinMethodValue(MethodFloatToInt, []ValueType{}, IntegerValueType),
}

// ByteArraySymbolTable is the static symbol table of byte buffers.
var ByteArraySymbolTable = map[string]*ExprValue{
	"length": NewBuiltinMethodValue(MethodByteArrayLength, []ValueType{}, IntegerValueType),
	"to_s":   NewBuiltinMethodValue(MethodByteArrayToString, []ValueType{StringValueType}, IntegerValueType),
}

// StringSymbolTable is the static symbol table of string values.
var StringSymbolTable = map[string]*ExprValue{
	"length":    NewBuiltinMethodValue(MethodStringLength, []ValueType{}, IntegerValueType),
	"reverse":   NewBuiltinMethodValue(MethodStringReverse, []ValueType{}, StringValueType),
	"substring": NewBuiltinMethodValue(MethodStringSubstring, []ValueType{IntegerValueType, IntegerValueType}, StringValueType),
	"to_i":      NewBuiltinMethodValue(MethodStringToInt, []ValueType{IntegerValueType}, IntegerValueType),
}

// EnumValueSymbolTable is the static symbol table of enumeration values.
var EnumValueSymbolTable = map[string]*ExprValue{
	"to_i": NewBuiltinMethodValue(MethodEnumToInt, []ValueType{}, IntegerValueType),
}

// BooleanSymbolTable is the static symbol table of boolean values.
var BooleanSymbolTable = map[string]*ExprValue{
	"to_i": NewBuiltinMethodValue(MethodBoolToInt, []ValueType{}, IntegerValueType),
}

// StreamSymbolTable is the static symbol table of the stream object.
var StreamSymbolTable = map[string]*ExprValue{
	"eof":  NewBuiltinMethodValue(MethodStreamEOF, []ValueType{}, BooleanValueType),
	"size": NewBuiltinMethodValue(MethodStreamSize, []ValueType{}, IntegerValueType),
	"pos":  NewBuiltinMethodValue(MethodStreamPos, []ValueType{}, IntegerValueType),
}

func ArraySymbolTable(typ types.Type) map[string]*ExprValue {
	return map[string]*ExprValue{
		"first": NewBuiltinMethodValue(MethodArrayFirst, []ValueType{}, ValueType{Type: typ}),
		"last":  NewBuiltinMethodValue(MethodArrayLast, []ValueType{}, ValueType{Type: typ}),
		"size":  NewBuiltinMethodValue(MethodArraySize, []ValueType{}, IntegerValueType),
		"min":   NewBuiltinMethodValue(MethodArrayMin, []ValueType{}, ValueType{Type: typ}),
		"max":   NewBuiltinMethodValue(MethodArrayMax, []ValueType{}, ValueType{Type: typ}),
	}
}

// RootSymbol is a symbol used for symbol roots.
type RootSymbol struct {
}

// StreamSymbol is a symbol sub-type for the stream intrinsic type. It can be
// referred to using the _io intrinsic inside of expressions.
type StreamSymbol struct {
}

// ExprParentSymbol is a symbol used to point to a parent of a specific type.
type ExprParentSymbol struct {
	Struct *kaitai.Struct
}

// ExprRootSymbol is a symbol used to point to a root of a specific type.
type ExprRootSymbol struct {
	Struct *kaitai.Struct
}

// MethodTypeData is a symbol used for methods, i.e. functions you can call in
// expressions.
type MethodTypeData struct {
	Method     BuiltinMethod
	Arguments  []ValueType
	ReturnType ValueType
}

type StructTypeData struct {
	Type      *kaitai.Struct
	Enums     []*ExprType
	Structs   []*ExprType
	Params    []*ExprType
	Attrs     []*ExprType
	Instances []*ExprType
}

type ArrayTypeData struct {
	Elem *ExprType
}

// MethodFn is the runtime signature for methods.
type MethodFn func(this *ExprValue, args []*ExprValue) (*ExprValue, error)

// MethodData is a symbol sub-type for runtime methods.
type MethodData struct{ Value MethodFn }

// IntegerData is a symbol sub-type for integer literals.
type IntegerData struct{ Value *big.Int }

// FloatData is a symbol sub-type for floating point literals.
type FloatData struct{ Value *big.Float }

// BooleanData is a symbol sub-type for boolean literals.
type BooleanData struct{ Value bool }

// ArrayData is a symbol sub-type for array literals.
type ArrayData struct{ Value []*ExprValue }

// ByteArrayData is a symbol sub-type for byte array literals.
type ByteArrayData struct{ Value []byte }

// StringData is a symbol sub-type for string literals.
type StringData struct{ Value string }

// CastData is a symbol sub-type that refers to another symbol after it has
// been casted.
type CastData struct {
	Symbol    *ExprValue
	ValueType ValueType
}

// AliasData is a symbol sub type that refers to an alias for another symbol.
type AliasData struct {
	Type   *ExprType
	Value  *ExprValue
	Target string
}

type StructData struct {
	Params    []*ExprValue
	Attrs     []*ExprValue
	Instances []*ExprValue
}

type ExprKind int

const (
	InvalidKind ExprKind = iota
	RootKind
	StreamKind
	MethodKind
	StructParentKind
	StructRootKind
	IntegerKind
	FloatKind
	BooleanKind
	ArrayKind
	ByteArrayKind
	StringKind
	StructKind
	EnumKind
	EnumValueKind
	ParamKind
	AttrKind
	InstanceKind
	CastedValueKind
	AliasKind
)

// ExprType represents a single expression type.
type ExprType struct {
	Kind     ExprKind
	Parent   *ExprType
	Children map[string]*ExprType
	Constant *ExprValue

	Method    *MethodTypeData
	Struct    *StructTypeData
	Array     *ArrayTypeData
	Enum      *kaitai.Enum
	EnumValue *kaitai.EnumValue
	Param     *kaitai.Param
	Attr      *kaitai.Attr
	Instance  *kaitai.Attr
	Elem      *types.TypeRef
	Cast      *CastData
	Alias     *AliasData
}

// ExprValue represents a single expression value.
type ExprValue struct {
	Parent   *ExprValue
	Children map[string]*ExprValue
	Type     *ExprType

	Integer   *IntegerData
	Float     *FloatData
	Boolean   *BooleanData
	Array     *ArrayData
	ByteArray *ByteArrayData
	String    *StringData
	Struct    *StructData
}

// NewTypeRoot creates a new type root.
func NewTypeRoot() *ExprType {
	return &ExprType{
		Kind:     RootKind,
		Children: make(map[string]*ExprType),
	}
}

func (t *ExprType) addMember(name string, typ *ExprType) {
	t.Children[name] = typ
}

// NewValueRoot creates a new value root.
func NewValueRoot() *ExprValue {
	return &ExprValue{
		Children: make(map[string]*ExprValue),
		Type:     NewTypeRoot(),
	}
}

// NewValueOf creates a new value of a given type.
func NewValueOf(context *Context, typ *ExprType) *ExprValue {
	switch typ.Kind {
	case StructParentKind:
		return NewStructValueSymbol(typ, nil)
	case StructRootKind:
		return NewStructValueSymbol(typ, nil)
	case IntegerKind:
		return NewIntegerLiteralValue(big.NewInt(0))
	case FloatKind:
		return NewFloatLiteralValue(big.NewFloat(0))
	case BooleanKind:
		return NewBooleanLiteralValue(false)
	case ArrayKind:
		return NewArrayLiteralValue(typ, []*ExprValue{})
	case ByteArrayKind:
		return NewByteArrayLiteralValue([]byte{})
	case StringKind:
		return NewStringLiteralValue("")
	case StructKind:
		return NewStructValueSymbol(typ, nil)
	case EnumValueKind:
		return NewIntegerLiteralValue(typ.EnumValue.Value)
	case ParamKind:
		return NewValueOfType(context, typ.Param.Type)
	case AttrKind:
		if typ.Attr.Type.TypeRef == nil {
			return nil
		}
		return NewValueOfType(context, *typ.Attr.Type.TypeRef)
	case InstanceKind:
		if typ.Instance.Type.TypeRef == nil {
			return nil
		}
		return NewValueOfType(context, *typ.Instance.Type.TypeRef)
	case AliasKind:
		return NewValueOf(context, typ.Alias.Type)
	default:
		return nil
	}
}

func NewValueOfType(context *Context, typ types.TypeRef) *ExprValue {
	switch typ.Kind {
	case types.U1, types.U2, types.U2le, types.U2be, types.U4,
		types.U4le, types.U4be, types.U8, types.U8le, types.U8be,
		types.S1, types.S2, types.S2le, types.S2be, types.S4,
		types.S4le, types.S4be, types.S8, types.S8le, types.S8be,
		types.UntypedInt:
		return NewIntegerLiteralValue(big.NewInt(0))
	case types.Bits:
		if typ.Bits.Width == 1 {
			return NewBooleanLiteralValue(false)
		} else {
			return NewIntegerLiteralValue(big.NewInt(0))
		}
	case types.F4, types.F4le, types.F4be,
		types.F8, types.F8le, types.F8be,
		types.UntypedFloat:
		return NewFloatLiteralValue(big.NewFloat(0))
	case types.Bytes:
		return NewByteArrayLiteralValue([]byte{})
	case types.String:
		return NewStringLiteralValue("")
	case types.User:
		resolved, _ := context.ResolveType(typ.User.Name)
		return NewValueOf(context, resolved)
	case types.UntypedBool:
		return NewBooleanLiteralValue(false)
	default:
		return nil
	}
}

func (v *ExprValue) addMember(name string, value *ExprValue) {
	v.Children[name] = value
}

// NewStreamValue creates the stream intrinsic.
func NewStreamValue() *ExprValue {
	return &ExprValue{
		Type: &ExprType{
			Kind: StreamKind,
		},
		Children: StreamSymbolTable,
	}
}

// NewBuiltinMethodValue creates an expression method symbol.
func NewBuiltinMethodValue(method BuiltinMethod, args []ValueType, ret ValueType) *ExprValue {
	return &ExprValue{
		Type: &ExprType{
			Kind: MethodKind,
			Method: &MethodTypeData{
				Method:     method,
				Arguments:  args,
				ReturnType: ret,
			},
		},
	}
}

// NewStructParentValue creates a new value referring to a struct's parent.
func NewStructParentValue(parent *ExprValue) *ExprValue {
	if parent.Type.Kind != StructKind {
		return nil
	}
	return &ExprValue{
		Type: &ExprType{
			Kind:   StructParentKind,
			Struct: parent.Type.Struct,
		},
	}
}

// NewStructRootValue creates a new value referring to a struct's root.
func NewStructRootValue(root *ExprValue) *ExprValue {
	if root.Type.Kind != StructKind {
		return nil
	}
	return &ExprValue{
		Type: &ExprType{
			Kind:   StructRootKind,
			Struct: root.Type.Struct,
		},
	}
}

var IntegerExprType = &ExprType{
	Kind: IntegerKind,
}

// NewIntegerLiteralValue creates a new value referring to an integer literal.
func NewIntegerLiteralValue(value *big.Int) *ExprValue {
	return &ExprValue{
		Type:     IntegerExprType,
		Children: IntegerSymbolTable,
		Integer:  &IntegerData{Value: value},
	}
}

var FloatExprType = &ExprType{
	Kind: FloatKind,
}

// NewFloatLiteralValue creates a new symbol referring to a float literal.
func NewFloatLiteralValue(value *big.Float) *ExprValue {
	return &ExprValue{
		Type:     FloatExprType,
		Children: FloatSymbolTable,
		Float:    &FloatData{Value: value},
	}
}

var BooleanExprType = &ExprType{
	Kind: BooleanKind,
}

// NewBooleanLiteralValue creates a new value for a boolean literal.
func NewBooleanLiteralValue(value bool) *ExprValue {
	return &ExprValue{
		Type:     BooleanExprType,
		Children: BooleanSymbolTable,
		Boolean:  &BooleanData{Value: value},
	}
}

// NewArrayLiteralValue creates a new value for an array literal.
func NewArrayLiteralValue(typ *ExprType, value []*ExprValue) *ExprValue {
	elemValueType, ok := typ.Array.Elem.ValueType()
	if !ok {
		return nil
	}
	return &ExprValue{
		Type:     typ,
		Children: ArraySymbolTable(elemValueType.Type),
		Array:    &ArrayData{Value: value},
	}
}

var ByteArrayExprType = &ExprType{
	Kind: ByteArrayKind,
}

// NewByteArrayLiteralValue creates a new value for a byte buffer.
func NewByteArrayLiteralValue(value []byte) *ExprValue {
	return &ExprValue{
		Type:      ByteArrayExprType,
		Children:  ByteArraySymbolTable,
		ByteArray: &ByteArrayData{Value: value},
	}
}

var StringExprType = &ExprType{
	Kind: StringKind,
}

// NewStringLiteralValue creates a new value for a string literal.
func NewStringLiteralValue(value string) *ExprValue {
	return &ExprValue{
		Type:     StringExprType,
		Children: StringSymbolTable,
		String:   &StringData{Value: value},
	}
}

func setMethodsForKind(value *ExprValue, typ *types.TypeRef) {
	switch typ.Kind {
	case types.U1, types.U2, types.U2le, types.U2be, types.U4, types.U4le, types.U4be, types.U8, types.U8le, types.U8be,
		types.S1, types.S2, types.S2le, types.S2be, types.S4, types.S4le, types.S4be, types.S8, types.S8le, types.S8be,
		types.UntypedInt:
		value.Children = IntegerSymbolTable
	case types.F4, types.F4le, types.F4be, types.F8, types.F8le, types.F8be, types.UntypedFloat:
		value.Children = FloatSymbolTable
	case types.Bytes:
		value.Children = ByteArraySymbolTable
	case types.String:
		value.Children = StringSymbolTable
	case types.UntypedBool:
		value.Children = BooleanSymbolTable
	}
}

func NewArrayType(elem *ExprType, parent *ExprType) *ExprType {
	return &ExprType{
		Parent: parent,
		Kind:   ArrayKind,
		Array: &ArrayTypeData{
			Elem: elem,
		},
	}
}

func NewParamType(param *kaitai.Param, parent *ExprType) *ExprType {
	return &ExprType{
		Parent: parent,
		Kind:   ParamKind,
		Param:  param,
	}
}

// NewAttrType creates a new type referring to a struct attribute.
func NewAttrType(attr *kaitai.Attr, parent *ExprType) *ExprType {
	return &ExprType{
		Parent: parent,
		Kind:   AttrKind,
		Attr:   attr,
	}
}

// NewInstanceType creates a new type referring to a struct instance.
func NewInstanceType(instance *kaitai.Attr, parent *ExprType) *ExprType {
	return &ExprType{
		Parent:   parent,
		Kind:     InstanceKind,
		Instance: instance,
	}
}

func NewEnumValueType(enumValue *kaitai.EnumValue) *ExprType {
	typ := &ExprType{
		Kind:      EnumValueKind,
		EnumValue: enumValue,
	}
	typ.Constant = &ExprValue{
		Type: &ExprType{
			Kind:   IntegerKind,
			Parent: typ,
		},
		Children: IntegerSymbolTable,
		Integer:  &IntegerData{Value: enumValue.Value},
	}
	return typ
}

func NewEnumType(enum *kaitai.Enum) *ExprType {
	typ := &ExprType{
		Children: make(map[string]*ExprType),
		Kind:     EnumKind,
		Enum:     enum,
	}
	for _, value := range enum.Values {
		value := value
		vt := NewEnumValueType(&value)
		vt.Parent = typ
		typ.addMember(string(value.ID), vt)
	}
	return typ
}

// NewAttrValue creates a new value referring to a struct parameter.
func NewParamValue(typ *ExprType, parent *ExprValue) *ExprValue {
	return &ExprValue{Parent: parent, Type: typ}
}

// NewAttrValue creates a new value referring to a struct attribute.
func NewAttrValue(typ *ExprType, parent *ExprValue) *ExprValue {
	return &ExprValue{Parent: parent, Type: typ}
}

// NewInstanceValue creates a new value referring to a struct instance.
func NewInstanceValue(typ *ExprType, parent *ExprValue) *ExprValue {
	return &ExprValue{Parent: parent, Type: typ}
}

// NewCastedValue creates a new symbol that is typecast from another symbol.
func NewCastedValue(sym *ExprValue, cast ValueType) *ExprValue {
	_, ok := sym.Type.ValueType()
	if !ok {
		return nil
	}
	// TODO:
	// - Should return nil if the cast is impossible
	// - Probably should find the set of typeswitch cases that fit
	value := &ExprValue{
		Parent: sym.Parent,
		Type: &ExprType{
			Kind: CastedValueKind,
			Cast: &CastData{
				Symbol:    sym,
				ValueType: cast,
			},
		},
	}
	if cast.Type.TypeRef != nil {
		setMethodsForKind(value, cast.Type.TypeRef)
	}
	return value
}

// NewAliasSymbol creates a new symbol that is an alias of another symbol.
func NewAliasSymbol(typ *ExprType, val *ExprValue, target string) *ExprValue {
	value := &ExprValue{
		Parent: nil,
		Type: &ExprType{
			Kind: AliasKind,
			Alias: &AliasData{
				Type:   typ,
				Value:  val,
				Target: target,
			},
		},
	}
	return value
}

// NewStructTypeSymbol creates a new symbol referring to a user-defined struct
// type. parent should be nil in top-level structs.
func NewStructTypeSymbol(struc *kaitai.Struct, parent *ExprType) *ExprType {
	structData := &StructTypeData{
		Type: struc,
	}
	result := &ExprType{
		Kind:     StructKind,
		Parent:   parent,
		Children: make(map[string]*ExprType),
		Struct:   structData,
	}
	for _, enum := range struc.Enums {
		enumType := NewEnumType(enum)
		enumType.Parent = result
		structData.Enums = append(structData.Enums, enumType)
		result.addMember(string(enum.ID), enumType)
	}
	for _, struc := range struc.Structs {
		structType := NewStructTypeSymbol(struc, result)
		structData.Structs = append(structData.Structs, structType)
		result.addMember(string(struc.ID), structType)
	}
	for _, param := range struc.Params {
		paramTyp := NewParamType(param, result)
		structData.Params = append(structData.Params, paramTyp)
	}
	for _, attr := range struc.Seq {
		attrTyp := NewAttrType(attr, result)
		structData.Attrs = append(structData.Attrs, attrTyp)
	}
	for _, instance := range struc.Instances {
		instanceTyp := NewInstanceType(instance, result)
		structData.Instances = append(structData.Instances, instanceTyp)
	}
	// Types can reference themselves, of course.
	result.addMember(string(struc.ID), result)
	return result
}

// NewStructTypeSymbol creates a new symbol referring to a user-defined struct
// type. parent should be nil in top-level structs.
func NewStructValueSymbol(structType *ExprType, parent *ExprValue) *ExprValue {
	structData := &StructData{}
	result := &ExprValue{
		Type:     structType,
		Children: make(map[string]*ExprValue),
		Parent:   parent,
		Struct:   structData,
	}
	for _, param := range structType.Struct.Params {
		paramValue := NewParamValue(param, result)
		structData.Params = append(structData.Params, paramValue)
		result.addMember(string(param.Param.ID), paramValue)
	}
	for _, attr := range structType.Struct.Attrs {
		attrVal := NewAttrValue(attr, result)
		structData.Attrs = append(structData.Attrs, attrVal)
		result.addMember(string(attr.Attr.ID), attrVal)
	}
	for _, instance := range structType.Struct.Instances {
		instanceVal := NewInstanceValue(instance, result)
		structData.Instances = append(structData.Instances, instanceVal)
		result.addMember(string(instance.Instance.ID), instanceVal)
	}
	return result
}

// ValueType returns the value this symbol evaluates to, if it can be referred
// to as a value. Otherwise, it returns an empty value type and false.
func (s *ExprType) ValueType() (ValueType, bool) {
	if s == nil {
		return ValueType{}, false
	}
	switch s.Kind {
	case IntegerKind:
		return IntegerValueType, true
	case FloatKind:
		return FloatValueType, true
	case BooleanKind:
		return BooleanValueType, true
	case ByteArrayKind:
		return ByteArrayValueType, true
	case ArrayKind:
		return ValueType{
			Type:   types.Type{TypeRef: s.Elem},
			Repeat: types.RepeatEOS{},
		}, true
	case StringKind:
		return StringValueType, true
	case EnumValueKind:
		return IntegerValueType, true
	case ParamKind:
		typeRef := s.Param.Type
		return ValueType{
			Type: types.Type{TypeRef: &typeRef},
		}, true
	case AttrKind:
		return ValueType{
			Type:   s.Attr.Type,
			Repeat: s.Attr.Repeat,
		}, true
	case InstanceKind:
		return ValueType{
			Type:   s.Instance.Type,
			Repeat: s.Instance.Repeat,
		}, true
	case CastedValueKind:
		return s.Cast.ValueType, true
	case AliasKind:
		return s.Alias.Type.ValueType()
	}
	return ValueType{}, false
}

func (s *ExprType) Child(symbol string) *ExprType {
	return s.Children[symbol]
}

func (s *ExprValue) Child(symbol string) *ExprValue {
	return s.Children[symbol]
}

// ResolutionScope represents a specific scope of resolution. Values are in
// order of increasing precedence, e.g. value 0 is the lowest precedence.
type ResolutionScope int

const (
	// GlobalScope is the resolution scope of "global" symbols. This is provided
	// for compatibility with upstream Kaitai Struct, though you should prefer
	// to explicitly import symbols where possible. Any symbol that has been
	// processed should be in this scope.
	GlobalScope ResolutionScope = iota

	// ModuleScope is the resolution scope of symbols within a module: any root
	// symbols as well as imported symbols are present.
	ModuleScope

	// LocalScope is the resolution scope of symbols within a structure:
	// sub-types will be visible.
	LocalScope

	// IntrinsicScope is the resolution scope of intrinsic values like _io.
	IntrinsicScope
)

// Context contains a symbol context for Kaitai Struct.
type Context struct {
	global *ExprValue
	module *ExprValue
	local  *ExprValue

	stream *ExprValue
	tmp    *ExprValue
}

// NewContext creates a new symbol context.
func NewContext() *Context {
	return &Context{
		global: NewValueRoot(),
		module: NewValueRoot(),
		local:  NewValueRoot(),
		stream: NewStreamValue(),
	}
}

func (context *Context) AddGlobalType(name string, typ *ExprType) {
	context.global.Type.addMember(name, typ)
}

func (context *Context) AddGlobalSymbol(name string, value *ExprValue) {
	context.global.addMember(name, value)
}

func (context *Context) AddModuleType(name string, typ *ExprType) {
	context.module.Type.addMember(name, typ)
}

func (context *Context) AddModuleSymbol(name string, value *ExprValue) {
	context.module.addMember(name, value)
}

func (context *Context) AddLocalSymbol(name string, value *ExprValue) {
	context.local.addMember(name, value)
}

func (context *Context) WithModuleRoot(symbol *ExprValue) *Context {
	return &Context{
		global: context.global,
		module: symbol,
		local:  context.local,
		stream: context.stream,
		tmp:    context.tmp,
	}
}

func (context *Context) WithLocalRoot(symbol *ExprValue) *Context {
	return &Context{
		global: context.global,
		module: context.module,
		local:  symbol,
		stream: context.stream,
		tmp:    context.tmp,
	}
}

func (context *Context) WithTemporary(symbol *ExprValue) *Context {
	return &Context{
		global: context.global,
		module: context.module,
		local:  context.local,
		stream: context.stream,
		tmp:    symbol,
	}
}

func (context *Context) ResolveIntrinsic(name string) *ExprValue {
	switch name {
	case "_root":
		return context.module

	case "_parent":
		return context.local

	case "_io":
		return context.stream

	case "_":
		return context.tmp
	}
	return nil
}

func (context *Context) ResolveLocalType(name string) *ExprType {
	typ := context.local.Type.Child(name)
	if typ != nil {
		return typ
	}
	if context.local.Type.Parent != nil {
		return context.local.Type.Parent.Child(name)
	}
	return nil
}

func (context *Context) ResolveModuleType(name string) *ExprType {
	return context.module.Type.Child(name)
}

func (context *Context) ResolveGlobalType(name string) *ExprType {
	return context.global.Type.Child(name)
}

func (context *Context) ResolveLocal(name string) *ExprValue {
	return context.local.Child(name)
}

func (context *Context) ResolveModule(name string) *ExprValue {
	return context.module.Child(name)
}

func (context *Context) ResolveGlobal(name string) *ExprValue {
	return context.global.Child(name)
}

// Resolve resolves the provided symbol and returns it, as well as the scope it
// was resolved in.
func (context *Context) Resolve(name string) (*ExprValue, ResolutionScope) {
	if sym := context.ResolveIntrinsic(name); sym != nil {
		return sym, IntrinsicScope
	}
	if sym := context.ResolveLocal(name); sym != nil {
		return sym, LocalScope
	}
	if sym := context.ResolveModule(name); sym != nil {
		return sym, ModuleScope
	}
	if sym := context.ResolveGlobal(name); sym != nil {
		return sym, GlobalScope
	}

	return nil, GlobalScope
}

func (context *Context) ResolveType(name string) (*ExprType, ResolutionScope) {
	if sym := context.ResolveIntrinsic(name); sym != nil {
		return sym.Type, IntrinsicScope
	}
	if sym := context.ResolveLocalType(name); sym != nil {
		return sym, LocalScope
	}
	if sym := context.ResolveModuleType(name); sym != nil {
		return sym, ModuleScope
	}
	if sym := context.ResolveGlobalType(name); sym != nil {
		return sym, GlobalScope
	}

	return nil, GlobalScope
}

func (context *Context) Parent() *Context {
	if context == nil || context.local.Parent == nil {
		return nil
	}
	return context.WithLocalRoot(context.local.Parent)
}

func (context *Context) Struct() *kaitai.Struct {
	if context == nil {
		return nil
	}
	return context.local.Type.Struct.Type
}
