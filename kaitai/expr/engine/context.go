package engine

import (
	"math/big"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/types"

	kaitai_io "github.com/jchw-forks/kaitai_struct_go_runtime/kaitai"
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
	"length":  NewBuiltinMethodValue(MethodByteArrayLength, []ValueType{}, IntegerValueType),
	"size":    NewBuiltinMethodValue(MethodByteArrayLength, []ValueType{}, IntegerValueType),
	"to_s":    NewBuiltinMethodValue(MethodByteArrayToString, []ValueType{StringValueType}, StringValueType),
	"first":   NewBuiltinMethodValue(MethodArrayFirst, []ValueType{}, IntegerValueType),
	"last":    NewBuiltinMethodValue(MethodArrayLast, []ValueType{}, IntegerValueType),
	"min":     NewBuiltinMethodValue(MethodArrayMin, []ValueType{}, IntegerValueType),
	"max":     NewBuiltinMethodValue(MethodArrayMax, []ValueType{}, IntegerValueType),
	"reverse": NewBuiltinMethodValue(MethodStringReverse, []ValueType{}, ByteArrayValueType),
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

// MethodTypeData is a symbol used for methods, i.e. functions you can call in
// expressions.
type MethodTypeData struct {
	Method     BuiltinMethod
	Arguments  []ValueType
	ReturnType ValueType
}

// StructTypeData holds type-level information about a struct symbol.
type StructTypeData struct {
	Type      *kaitai.Struct
	Opaque    bool // true for externally-defined opaque types
	Enums     []*ExprValue
	Structs   []*ExprValue
	Params    []*ExprValue
	Attrs     []*ExprValue
	Instances []*ExprValue
}

// ArrayTypeData holds type-level information about an array symbol.
type ArrayTypeData struct {
	Elem *ExprValue
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

// AliasData is a symbol sub-type that refers to an alias for another symbol.
type AliasData struct {
	Ref    *ExprValue
	Target string
}

// StreamData is a symbol sub-type for stream values.
type StreamData struct {
	Stream *kaitai_io.Stream
	// AbsoluteOffset is the byte offset of this stream's position 0 in
	// the root buffer. 0 for the root stream; non-zero whenever the
	// stream is a sub-stream (e.g., created by a size-bound user type).
	// Used by the eval runtime to translate stream-local spans back to
	// root-buffer coordinates so the hex editor can highlight the right
	// bytes when instances reference a different stream via `io:`.
	AbsoluteOffset uint64
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

// ExprValue is the unified symbol type for the expression engine, combining
// type information and optional value data.
type ExprValue struct {
	Kind      ExprKind
	Parent    *ExprValue
	DefParent *ExprValue
	Children  map[string]*ExprValue
	Types     map[string]*ExprValue
	Constant  *ExprValue

	// Type metadata
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

	// Value data
	Integer   *IntegerData
	Float     *FloatData
	Boolean   *BooleanData
	Items     []*ExprValue // array element values
	ByteArray *ByteArrayData
	String    *StringData
	Stream    *StreamData

	// Runtime is an optional lazy lookup hook. When non-nil and a
	// Children/Items lookup misses, the expression evaluator consults Runtime
	// to resolve members/items on demand. Codegen path leaves this nil.
	Runtime RuntimeRef
}

// NewValueRoot creates a new value root.
func NewValueRoot() *ExprValue {
	return &ExprValue{
		Kind:     RootKind,
		Children: make(map[string]*ExprValue),
		Types:    make(map[string]*ExprValue),
	}
}

func (v *ExprValue) addMember(name string, value *ExprValue) {
	if v.Children == nil {
		v.Children = make(map[string]*ExprValue)
	}
	v.Children[name] = value
}

func (v *ExprValue) addType(name string, typ *ExprValue) {
	if v.Types == nil {
		v.Types = make(map[string]*ExprValue)
	}
	v.Types[name] = typ
}

// NewValueOf creates a resolved value for a given symbol. This resolves what
// kind of value a symbol produces (e.g. an attr of type u4 produces an integer
// with integer methods).
func NewValueOf(context *Context, sym *ExprValue) *ExprValue {
	if sym == nil {
		return nil
	}
	switch sym.Kind {
	case StructParentKind:
		return NewStructValueSymbol(sym, nil)
	case StructRootKind:
		return NewStructValueSymbol(sym, nil)
	case IntegerKind:
		return NewIntegerLiteralValue(big.NewInt(0))
	case FloatKind:
		return NewFloatLiteralValue(big.NewFloat(0))
	case BooleanKind:
		return NewBooleanLiteralValue(false)
	case ArrayKind:
		return NewArrayLiteralValue(sym, []*ExprValue{})
	case ByteArrayKind:
		return NewByteArrayLiteralValue([]byte{})
	case StringKind:
		return NewStringLiteralValue("")
	case StructKind:
		return NewStructValueSymbol(sym, nil)
	case EnumValueKind:
		if sym.EnumValue != nil {
			return NewIntegerLiteralValue(sym.EnumValue.Value)
		}
		return NewIntegerLiteralValue(big.NewInt(0))
	case ParamKind:
		return NewValueOfType(context, sym.Param.Type)
	case AttrKind:
		// If the attr has an enum, return an enum-typed value
		if sym.Attr.Enum != "" {
			return &ExprValue{
				Kind:     EnumValueKind,
				Children: EnumValueSymbolTable,
			}
		}
		// If the attr is repeated, return an array value with array methods
		if sym.Attr.Repeat != nil {
			return &ExprValue{
				Kind:     ArrayKind,
				Children: ArraySymbolTable(sym.Attr.Type),
			}
		}
		if sym.Attr.Type.TypeRef == nil {
			if sym.Attr.Type.TypeSwitch != nil {
				// Switch type - return a generic value that can be cast via .as<>
				return &ExprValue{Kind: StructKind}
			}
			return nil
		}
		return NewValueOfType(context, *sym.Attr.Type.TypeRef)
	case InstanceKind:
		// For repeated instances, return an array value with array methods
		if sym.Instance.Repeat != nil {
			return &ExprValue{
				Kind:     ArrayKind,
				Children: ArraySymbolTable(sym.Instance.Type),
			}
		}
		// For enum-typed instances, return an enum value with to_i method
		if sym.Instance.Enum != "" {
			return &ExprValue{
				Kind:     EnumValueKind,
				Children: EnumValueSymbolTable,
			}
		}
		// For value-only instances (no explicit type or default bytes), infer from expression
		if sym.Instance.Value != nil {
			isDefaultBytes := sym.Instance.Type.TypeRef != nil && sym.Instance.Type.TypeRef.Kind == types.Bytes
			if sym.Instance.Type.TypeRef == nil || isDefaultBytes {
				result := ResultTypeOfExpr(context, sym.Instance.Value)
				if result != nil {
					inferred := NewValueOf(context, result)
					if inferred != nil {
						return inferred
					}
				}
			}
		}
		if sym.Instance.Type.TypeRef == nil {
			return nil
		}
		return NewValueOfType(context, *sym.Instance.Type.TypeRef)
	case AliasKind:
		// For aliases (like _ in repeat-until), resolve to the element type, not the array
		aliasRef := sym.Alias.Ref
		if aliasRef.Kind == AttrKind && aliasRef.Attr.Repeat != nil {
			// Return element type, not array
			if aliasRef.Attr.Type.TypeRef != nil {
				return NewValueOfType(context, *aliasRef.Attr.Type.TypeRef)
			}
		}
		return NewValueOf(context, aliasRef)
	case StreamKind:
		return NewStreamValue()
	case CastedValueKind:
		if sym.Cast != nil && sym.Cast.ValueType.Type.TypeRef != nil {
			return NewValueOfType(context, *sym.Cast.ValueType.Type.TypeRef)
		}
		return nil
	case MethodKind:
		// Methods resolve to their return type for chained access (e.g., .to_i.to_s)
		if sym.Method != nil {
			ret := sym.Method.ReturnType
			if ret.Type.TypeRef != nil {
				return NewValueOfType(context, *ret.Type.TypeRef)
			}
		}
		return nil
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
		// Handle builtin type names
		switch typ.User.Name {
		case "io":
			return NewStreamValue()
		case "bool":
			return NewBooleanLiteralValue(false)
		}
		resolved, _ := context.ResolveType(typ.User.Name)
		if resolved == nil {
			return nil
		}
		return NewValueOf(context, resolved)
	case types.UntypedBool:
		return NewBooleanLiteralValue(false)
	default:
		return nil
	}
}

// NewStreamValue creates the stream intrinsic.
func NewStreamValue() *ExprValue {
	return &ExprValue{
		Kind:     StreamKind,
		Children: StreamSymbolTable,
	}
}

// NewRuntimeStreamValue creates a runtime stream intrinsic. `absoluteOffset`
// is the byte offset of `stream` position 0 in the root buffer (0 for the
// root stream itself; non-zero for sub-streams).
func NewRuntimeStreamValue(stream *kaitai_io.Stream, absoluteOffset uint64) *ExprValue {
	return &ExprValue{
		Kind:     StreamKind,
		Stream:   &StreamData{Stream: stream, AbsoluteOffset: absoluteOffset},
		Children: StreamSymbolTable,
	}
}

// NewBuiltinMethodValue creates an expression method symbol.
func NewBuiltinMethodValue(method BuiltinMethod, args []ValueType, ret ValueType) *ExprValue {
	return &ExprValue{
		Kind: MethodKind,
		Method: &MethodTypeData{
			Method:     method,
			Arguments:  args,
			ReturnType: ret,
		},
	}
}

// NewStructParentValue creates a new value referring to a struct's parent.
func NewStructParentValue(parent *ExprValue) *ExprValue {
	if parent.Kind != StructKind {
		return nil
	}
	return &ExprValue{
		Kind:   StructParentKind,
		Struct: parent.Struct,
	}
}

// NewStructRootValue creates a new value referring to a struct's root.
func NewStructRootValue(root *ExprValue) *ExprValue {
	if root.Kind != StructKind {
		return nil
	}
	return &ExprValue{
		Kind:   StructRootKind,
		Struct: root.Struct,
	}
}

// NewIntegerLiteralValue creates a new value referring to an integer literal.
func NewIntegerLiteralValue(value *big.Int) *ExprValue {
	return &ExprValue{
		Kind:     IntegerKind,
		Children: IntegerSymbolTable,
		Integer:  &IntegerData{Value: value},
	}
}

// NewFloatLiteralValue creates a new symbol referring to a float literal.
func NewFloatLiteralValue(value *big.Float) *ExprValue {
	return &ExprValue{
		Kind:     FloatKind,
		Children: FloatSymbolTable,
		Float:    &FloatData{Value: value},
	}
}

// NewBooleanLiteralValue creates a new value for a boolean literal.
func NewBooleanLiteralValue(value bool) *ExprValue {
	return &ExprValue{
		Kind:     BooleanKind,
		Children: BooleanSymbolTable,
		Boolean:  &BooleanData{Value: value},
	}
}

// NewArrayLiteralValue creates a new value for an array literal.
func NewArrayLiteralValue(typ *ExprValue, value []*ExprValue) *ExprValue {
	elemValueType, ok := typ.Array.Elem.ValueType()
	if !ok {
		return nil
	}
	return &ExprValue{
		Kind:     ArrayKind,
		Array:    typ.Array,
		Children: ArraySymbolTable(elemValueType.Type),
		Items:    value,
	}
}

// NewByteArrayLiteralValue creates a new value for a byte buffer.
func NewByteArrayLiteralValue(value []byte) *ExprValue {
	return &ExprValue{
		Kind:      ByteArrayKind,
		Children:  ByteArraySymbolTable,
		ByteArray: &ByteArrayData{Value: value},
	}
}

// NewStringLiteralValue creates a new value for a string literal.
func NewStringLiteralValue(value string) *ExprValue {
	return &ExprValue{
		Kind:     StringKind,
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

// NewArrayType creates a new array type symbol.
func NewArrayType(elem *ExprValue, parent *ExprValue) *ExprValue {
	return &ExprValue{
		Parent: parent,
		Kind:   ArrayKind,
		Array: &ArrayTypeData{
			Elem: elem,
		},
	}
}

func NewEnumValueSymbol(enumValue *kaitai.EnumValue) *ExprValue {
	sym := &ExprValue{
		Kind:      EnumValueKind,
		EnumValue: enumValue,
	}
	// Enum value constants need both integer methods (to_s) and enum methods (to_i)
	enumConstSymbols := map[string]*ExprValue{}
	for k, v := range IntegerSymbolTable {
		enumConstSymbols[k] = v
	}
	for k, v := range EnumValueSymbolTable {
		enumConstSymbols[k] = v
	}
	sym.Constant = &ExprValue{
		Kind:     IntegerKind,
		Parent:   sym,
		Children: enumConstSymbols,
		Integer:  &IntegerData{Value: enumValue.Value},
	}
	return sym
}

func NewEnumSymbol(enum *kaitai.Enum) *ExprValue {
	sym := &ExprValue{
		Types: make(map[string]*ExprValue),
		Kind:  EnumKind,
		Enum:  enum,
	}
	for _, value := range enum.Values {
		value := value
		vt := NewEnumValueSymbol(&value)
		vt.Parent = sym
		sym.addType(string(value.ID), vt)
	}
	return sym
}

// NewCastedValue creates a new symbol that is typecast from another symbol.
//
// The cast is intentionally permissive: any source with a valid ValueType is
// accepted, and no compatibility check is performed against `cast`. Callers
// rely on this leniency for cases the static type system cannot resolve -
// e.g. `.as<T>` on a switch-typed field where any case might be the actual
// runtime type. A stricter version would reject impossible casts at codegen
// time and, for type-switch sources, narrow to the subset of cases that fit;
// today both responsibilities sit with the caller.
func NewCastedValue(sym *ExprValue, cast ValueType) *ExprValue {
	_, ok := sym.ValueType()
	if !ok {
		return nil
	}
	value := &ExprValue{
		Parent: sym.Parent,
		Kind:   CastedValueKind,
		Cast: &CastData{
			Symbol:    sym,
			ValueType: cast,
		},
	}
	if cast.Type.TypeRef != nil {
		setMethodsForKind(value, cast.Type.TypeRef)
	}
	return value
}

// NewAliasSymbol creates a new symbol that is an alias of another symbol.
func NewAliasSymbol(ref *ExprValue, target string) *ExprValue {
	return &ExprValue{
		Kind: AliasKind,
		Alias: &AliasData{
			Ref:    ref,
			Target: target,
		},
	}
}

// NewStructSymbol creates a new symbol referring to a user-defined struct type.
// This creates a unified symbol with both type information (nested types in Types)
// and value information (fields in Children). parent should be nil for top-level structs.
func NewStructSymbol(struc *kaitai.Struct, parent *ExprValue) *ExprValue {
	structData := &StructTypeData{
		Type: struc,
	}
	result := &ExprValue{
		Kind:      StructKind,
		Parent:    parent,
		DefParent: parent,
		Children:  make(map[string]*ExprValue),
		Types:     make(map[string]*ExprValue),
		Struct:    structData,
	}
	for _, enum := range struc.Enums {
		enumSym := NewEnumSymbol(enum)
		enumSym.Parent = result
		enumSym.DefParent = result
		structData.Enums = append(structData.Enums, enumSym)
		result.addType(string(enum.ID), enumSym)
	}
	for _, child := range struc.Structs {
		childSym := NewStructSymbol(child, result)
		structData.Structs = append(structData.Structs, childSym)
		result.addType(string(child.ID), childSym)
	}
	for _, param := range struc.Params {
		paramSym := &ExprValue{Parent: result, Kind: ParamKind, Param: param}
		structData.Params = append(structData.Params, paramSym)
		result.addMember(string(param.ID), paramSym)
	}
	for _, attr := range struc.Seq {
		attrSym := &ExprValue{Parent: result, Kind: AttrKind, Attr: attr}
		structData.Attrs = append(structData.Attrs, attrSym)
		result.addMember(string(attr.ID), attrSym)
	}
	for _, instance := range struc.Instances {
		instSym := &ExprValue{Parent: result, Kind: InstanceKind, Instance: instance}
		structData.Instances = append(structData.Instances, instSym)
		result.addMember(string(instance.ID), instSym)
	}
	// Types can reference themselves, of course.
	result.addType(string(struc.ID), result)
	return result
}

// NewStructValueSymbol creates a new value-level symbol for a struct, useful
// when you need a copy with a different parent (e.g. for emitter traversal).
func NewStructValueSymbol(structSym *ExprValue, parent *ExprValue) *ExprValue {
	result := &ExprValue{
		Kind:      structSym.Kind,
		DefParent: structSym.DefParent,
		Struct:    structSym.Struct,
		Children:  make(map[string]*ExprValue),
		Types:     structSym.Types,
		Parent:    parent,
	}
	for _, param := range structSym.Struct.Params {
		paramVal := &ExprValue{Parent: result, Kind: ParamKind, Param: param.Param}
		result.addMember(string(param.Param.ID), paramVal)
	}
	for _, attr := range structSym.Struct.Attrs {
		attrVal := &ExprValue{Parent: result, Kind: AttrKind, Attr: attr.Attr}
		result.addMember(string(attr.Attr.ID), attrVal)
	}
	for _, instance := range structSym.Struct.Instances {
		instVal := &ExprValue{Parent: result, Kind: InstanceKind, Instance: instance.Instance}
		result.addMember(string(instance.Instance.ID), instVal)
	}
	return result
}

// NewOpaqueStructSymbol creates a minimal struct symbol for an opaque external type.
// Opaque types have no known params, attrs, instances, or children.
func NewOpaqueStructSymbol(name string) *ExprValue {
	return &ExprValue{
		Kind: StructKind,
		Struct: &StructTypeData{
			Type:   &kaitai.Struct{ID: kaitai.Identifier(name)},
			Opaque: true,
		},
		Children: make(map[string]*ExprValue),
		Types:    make(map[string]*ExprValue),
	}
}

// ValueType returns the value this symbol evaluates to, if it can be referred
// to as a value. Otherwise, it returns an empty value type and false.
func (s *ExprValue) ValueType() (ValueType, bool) {
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
		return s.Alias.Ref.ValueType()
	}
	return ValueType{}, false
}

func (s *ExprValue) Child(symbol string) *ExprValue {
	if s.Children == nil {
		return nil
	}
	return s.Children[symbol]
}

func (s *ExprValue) TypeChild(symbol string) *ExprValue {
	if s.Types == nil {
		return nil
	}
	return s.Types[symbol]
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
	index  *ExprValue // current `_index` value when evaluating inside an array element
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

func (context *Context) AddGlobalType(name string, typ *ExprValue) {
	context.global.addType(name, typ)
}

func (context *Context) AddGlobalSymbol(name string, value *ExprValue) {
	context.global.addMember(name, value)
}

func (context *Context) AddModuleType(name string, typ *ExprValue) {
	context.module.addType(name, typ)
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
		index:  context.index,
	}
}

func (context *Context) WithLocalRoot(symbol *ExprValue) *Context {
	return &Context{
		global: context.global,
		module: context.module,
		local:  symbol,
		stream: context.stream,
		tmp:    context.tmp,
		index:  context.index,
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

func (context *Context) WithStream(stream *ExprValue) *Context {
	return &Context{
		global: context.global,
		module: context.module,
		local:  context.local,
		stream: stream,
		tmp:    context.tmp,
		index:  context.index,
	}
}

// WithIndex returns a copy of context with `_index` bound to the given value.
// Used by repeat-* loops so size and other expressions can reference the
// current iteration index.
func (context *Context) WithIndex(index *ExprValue) *Context {
	return &Context{
		global: context.global,
		module: context.module,
		local:  context.local,
		stream: context.stream,
		tmp:    context.tmp,
		index:  index,
	}
}

func (context *Context) ResolveIntrinsic(name string) *ExprValue {
	switch name {
	case "_root":
		return context.module

	case "_parent":
		if context.local != nil && context.local.Parent != nil {
			return context.local.Parent
		}
		return context.local

	case "_io":
		return context.stream

	case "_":
		return context.tmp

	case "_index":
		// _index is the current iteration index in a repeat-* loop. Defaults
		// to 0 when no loop is active so type-resolution paths that don't
		// have a concrete index (e.g. codegen) still get a sensible value.
		if context.index != nil {
			return context.index
		}
		return NewIntegerLiteralValue(big.NewInt(0))

	case "_sizeof":
		// _sizeof returns the size of the current struct. First check for a
		// pre-populated value, then fall back to the Runtime hook (which can
		// compute the span lazily for runtime-backed struct values).
		if context.local != nil {
			if sizeVal := context.local.Child("_sizeof"); sizeVal != nil {
				return sizeVal
			}
			if context.local.Runtime != nil {
				if sizeVal, ok := context.local.Runtime.LookupChild("_sizeof"); ok && sizeVal != nil {
					return sizeVal
				}
			}
		}
		return NewIntegerLiteralValue(big.NewInt(0))
	}
	return nil
}

func (context *Context) RootValue() *ExprValue {
	return context.module
}

func (context *Context) ParentValue() *ExprValue {
	if context.local != nil && context.local.Parent != nil {
		return context.local.Parent
	}
	return context.local
}

func (context *Context) StreamValue() *ExprValue {
	return context.stream
}

func (context *Context) ResolveLocalType(name string) *ExprValue {
	typ := context.local.TypeChild(name)
	if typ != nil {
		return typ
	}
	if context.local.Parent != nil {
		return context.local.Parent.TypeChild(name)
	}
	return nil
}

func (context *Context) ResolveModuleType(name string) *ExprValue {
	return context.module.TypeChild(name)
}

func (context *Context) ResolveGlobalType(name string) *ExprValue {
	return context.global.TypeChild(name)
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

func (context *Context) ResolveType(name string) (*ExprValue, ResolutionScope) {
	if sym := context.ResolveIntrinsic(name); sym != nil {
		return sym, IntrinsicScope
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
	return context.local.Struct.Type
}
