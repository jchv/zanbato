package kaitai

import "math/big"

// BuiltinMethod indexes the built-in methods.
type BuiltinMethod int

const (
	// MethodIntToString converts an integer into a string using decimal
	// representation.
	MethodIntToString BuiltinMethod = iota

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

// ValueType is a type used for values that can be referenced. Note that not
// all kinds of expressions yield values that can be referenced, as the
// expression language lacks first-class functions, for example.
type ValueType struct {
	Type   Type
	Repeat RepeatType
}

// IntegerValueType is the type used for integer values in symbols.
var IntegerValueType = ValueType{Type: Type{TypeRef: &TypeRef{Kind: UntypedInt}}}

// FloatValueType is the type used for float values in symbols.
var FloatValueType = ValueType{Type: Type{TypeRef: &TypeRef{Kind: UntypedFloat}}}

// ByteArrayValueType is the type used for byte array values in symbols.
var ByteArrayValueType = ValueType{Type: Type{TypeRef: &TypeRef{Kind: Bytes}}}

// StringValueType is the type used for integer values in symbols.
var StringValueType = ValueType{Type: Type{TypeRef: &TypeRef{Kind: String}}}

// BooleanValueType is the type used for boolean values in symbols.
var BooleanValueType = ValueType{Type: Type{TypeRef: &TypeRef{Kind: UntypedBool}}}

// IntegerSymbolTable is the static symbol table of integer values.
var IntegerSymbolTable = map[string]*Symbol{
	"to_s": NewMethodSymbol(MethodIntToString, []ValueType{}, StringValueType),
}

// FloatSymbolTable is the static symbol table of floating point values.
var FloatSymbolTable = map[string]*Symbol{
	"to_i": NewMethodSymbol(MethodIntToString, []ValueType{}, IntegerValueType),
}

// ByteArraySymbolTable is the static symbol table of byte buffers.
var ByteArraySymbolTable = map[string]*Symbol{
	"length": NewMethodSymbol(MethodByteArrayLength, []ValueType{}, IntegerValueType),
	"to_s":   NewMethodSymbol(MethodByteArrayToString, []ValueType{StringValueType}, IntegerValueType),
}

// StringSymbolTable is the static symbol table of string values.
var StringSymbolTable = map[string]*Symbol{
	"length":    NewMethodSymbol(MethodStringLength, []ValueType{}, IntegerValueType),
	"reverse":   NewMethodSymbol(MethodStringReverse, []ValueType{}, StringValueType),
	"substring": NewMethodSymbol(MethodStringSubstring, []ValueType{IntegerValueType, IntegerValueType}, StringValueType),
	"to_i":      NewMethodSymbol(MethodStringToInt, []ValueType{IntegerValueType}, IntegerValueType),
}

// EnumValueSymbolTable is the static symbol table of enumeration values.
var EnumValueSymbolTable = map[string]*Symbol{
	"to_i": NewMethodSymbol(MethodEnumToInt, []ValueType{}, IntegerValueType),
}

// BooleanSymbolTable is the static symbol table of boolean values.
var BooleanSymbolTable = map[string]*Symbol{
	"to_i": NewMethodSymbol(MethodBoolToInt, []ValueType{}, IntegerValueType),
}

// StreamSymbolTable is the static symbol table of the stream object.
var StreamSymbolTable = map[string]*Symbol{
	"eof":  NewMethodSymbol(MethodStreamEOF, []ValueType{}, BooleanValueType),
	"size": NewMethodSymbol(MethodStreamSize, []ValueType{}, IntegerValueType),
	"pos":  NewMethodSymbol(MethodStreamPos, []ValueType{}, IntegerValueType),
}

func ArraySymbolTable(typ Type) map[string]*Symbol {
	return map[string]*Symbol{
		"first": NewMethodSymbol(MethodArrayFirst, []ValueType{}, ValueType{Type: typ}),
		"last":  NewMethodSymbol(MethodArrayLast, []ValueType{}, ValueType{Type: typ}),
		"size":  NewMethodSymbol(MethodArraySize, []ValueType{}, IntegerValueType),
		"min":   NewMethodSymbol(MethodArrayMin, []ValueType{}, ValueType{Type: typ}),
		"max":   NewMethodSymbol(MethodArrayMax, []ValueType{}, ValueType{Type: typ}),
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
	Struct *Struct
}

// ExprRootSymbol is a symbol used to point to a root of a specific type.
type ExprRootSymbol struct {
	Struct *Struct
}

// MethodSymbol is a symbol used for methods, i.e. functions you can call in
// expressions.
type MethodSymbol struct {
	Method     BuiltinMethod
	Arguments  []ValueType
	ReturnType ValueType
}

// IntegerSymbol is a symbol sub-type for integer literals.
type IntegerSymbol struct{ Value *big.Int }

// FloatSymbol is a symbol sub-type for floating point literals.
type FloatSymbol struct{ Value *big.Float }

// BooleanSymbol is a symbol sub-type for boolean literals.
type BooleanSymbol struct{ Value bool }

// ByteArraySymbol is a symbol sub-type for byte array literals.
type ByteArraySymbol struct{ Value []byte }

// StringSymbol is a symbol sub-type for string literals.
type StringSymbol struct{ Value string }

// StructSymbol is a symbol sub-type for symbols that refer to user-defined
// struct types, i.e. the root of a ksy or any of its sub-types.
type StructSymbol struct {
	Struct *Struct
}

// EnumTypeSymbol is a symbol sub-type for symbols that refer to user-defined
// enumerations within struct definitions.
type EnumTypeSymbol struct {
	Parent *Struct
	Enum   *Enum
}

// EnumValueSymbol is a symbol sub-type for symbols that refer to user-defined
// enumeration values, i.e. an entry of an enum.
type EnumValueSymbol struct {
	Parent *Struct
	Enum   *Enum
	Value  *EnumValue
}

// AttrSymbol is a symbol sub-type for symbols that refer to attributes inside
// of user-defined structures.
type AttrSymbol struct {
	Parent *Struct
	Attr   *Attr
}

// InstanceSymbol is a symbol sub-type for symbols that refer to instances
// inside of user-defined structures.
type InstanceSymbol struct {
	Parent   *Struct
	Instance *Attr
}

// Symbol represents a single symbol, its location in its module, and all of its
// sub-symbols.
type Symbol struct {
	Parent   *Symbol
	Children map[string]*Symbol

	// Intrinsics
	Root   *RootSymbol
	Stream *StreamSymbol
	Method *MethodSymbol

	ExprParent *ExprParentSymbol
	ExprRoot   *ExprRootSymbol

	// Literals
	Integer   *IntegerSymbol
	Float     *FloatSymbol
	Boolean   *BooleanSymbol
	ByteArray *ByteArraySymbol
	String    *StringSymbol

	// User-defined types
	Struct    *StructSymbol
	EnumType  *EnumTypeSymbol
	EnumValue *EnumValueSymbol
	Attr      *AttrSymbol
	Instance  *InstanceSymbol
}

// NewRootSymbol creates a new symbol root.
func NewRootSymbol() *Symbol {
	return &Symbol{
		Children: make(map[string]*Symbol),
		Root:     &RootSymbol{},
	}
}

// NewStreamSymbol creates the stream symbol intrinsic.
func NewStreamSymbol() *Symbol {
	return &Symbol{
		Children: StreamSymbolTable,
		Stream:   &StreamSymbol{},
	}
}

// NewMethodSymbol creates an expression method symbol.
func NewMethodSymbol(method BuiltinMethod, args []ValueType, ret ValueType) *Symbol {
	return &Symbol{
		Method: &MethodSymbol{
			Method:     method,
			Arguments:  args,
			ReturnType: ret,
		},
	}
}

// NewExprParentSymbol creates a new symbol referring to an expression parent.
func NewExprParentSymbol(parent *Symbol) *Symbol {
	if parent.Struct == nil || parent.Struct.Struct == nil {
		return nil
	}
	return &Symbol{
		ExprParent: &ExprParentSymbol{
			Struct: parent.Struct.Struct,
		},
	}
}

// NewExprRootSymbol creates a new symbol referring to an expression root.
func NewExprRootSymbol(root *Symbol) *Symbol {
	if root.Struct == nil || root.Struct.Struct == nil {
		return nil
	}
	return &Symbol{
		ExprRoot: &ExprRootSymbol{
			Struct: root.Struct.Struct,
		},
	}
}

// NewIntegerLiteralSymbol creates a new symbol referring to an integer literal.
func NewIntegerLiteralSymbol(value *big.Int) *Symbol {
	return &Symbol{
		Children: IntegerSymbolTable,
		Integer:  &IntegerSymbol{Value: value},
	}
}

// NewFloatLiteralSymbol creates a new symbol referring to an integer literal.
func NewFloatLiteralSymbol(value *big.Float) *Symbol {
	return &Symbol{
		Children: FloatSymbolTable,
		Float:    &FloatSymbol{Value: value},
	}
}

// NewBooleanLiteralSymbol creates a new symbol referring to a boolean literal.
func NewBooleanLiteralSymbol(value bool) *Symbol {
	return &Symbol{
		Children: ByteArraySymbolTable,
		Boolean:  &BooleanSymbol{Value: value},
	}
}

// NewByteArrayLiteralSymbol creates a new symbol referring to a byte buffer.
func NewByteArrayLiteralSymbol(value []byte) *Symbol {
	return &Symbol{
		Children:  ByteArraySymbolTable,
		ByteArray: &ByteArraySymbol{Value: value},
	}
}

// NewStringLiteralSymbol creates a new symbol referring to a string literal.
func NewStringLiteralSymbol(value string) *Symbol {
	return &Symbol{
		Children: StringSymbolTable,
		String:   &StringSymbol{Value: value},
	}
}

// NewAttrSymbol creates a new symbol referring to a struct attribute. parent
// must be set to a struct symbol.
func NewAttrSymbol(context *Context, attr *Attr, parent *Symbol) *Symbol {
	sym := &Symbol{
		Parent: parent,
		Attr:   &AttrSymbol{Parent: parent.Struct.Struct, Attr: attr},
	}
	// Note: typeswitch values need to be casted, so we don't add any methods
	// to them.
	if attr.Type.TypeRef != nil {
		switch attr.Type.TypeRef.Kind {
		case U1, U2, U2le, U2be, U4, U4le, U4be, U8, U8le, U8be,
			S1, S2, S2le, S2be, S4, S4le, S4be, S8, S8le, S8be,
			UntypedInt:
			sym.Children = IntegerSymbolTable
		case F4, F4le, F4be, F8, F8le, F8be, UntypedFloat:
			sym.Children = FloatSymbolTable
		case Bytes:
			sym.Children = ByteArraySymbolTable
		case String:
			sym.Children = StringSymbolTable
		case UntypedBool:
			sym.Children = BooleanSymbolTable
		}
	}
	return sym
}

// NewStructSymbol creates a new symbol referring to a user-defined struct type.
// parent should be nil in top-level structs.
func NewStructSymbol(context *Context, struc *Struct, parent *Symbol) *Symbol {
	sym := &Symbol{
		Parent:   parent,
		Children: make(map[string]*Symbol),
		Struct:   &StructSymbol{Struct: struc},
	}
	for _, attr := range struc.Seq {
		sym.addDirectChild(string(attr.ID), NewAttrSymbol(context, attr, sym))
	}
	return sym
}

// addDirectChild adds a child as a direct descendent and sets its parent to us.
func (s *Symbol) addDirectChild(symbol string, child *Symbol) {
	s.Children[symbol] = child
	child.Parent = s
}

// ValueType returns the value this symbol evaluates to, if it can be referred
// to as a value. Otherwise, it returns an empty value type and false.
func (s *Symbol) ValueType() (ValueType, bool) {
	switch {
	case s.Root != nil:
		break
	case s.Stream != nil:
		break
	case s.Method != nil:
		// When not using the call syntax, referring to a method just calls it
		// with no arguments. Therefore, use the return value type.
		return s.Method.ReturnType, true
	case s.ExprParent != nil:
		break
	case s.ExprRoot != nil:
		break
	case s.Integer != nil:
		return IntegerValueType, true
	case s.Float != nil:
		return FloatValueType, true
	case s.Boolean != nil:
		return BooleanValueType, true
	case s.ByteArray != nil:
		return ByteArrayValueType, true
	case s.String != nil:
		return StringValueType, true
	case s.Struct != nil:
		break
	case s.EnumType != nil:
		break
	case s.EnumValue != nil:
		return IntegerValueType, true
	case s.Attr != nil:
		return ValueType{
			Type:   s.Attr.Attr.Type,
			Repeat: s.Attr.Attr.Repeat,
		}, true
	case s.Instance != nil:
		return ValueType{
			Type:   s.Instance.Instance.Type,
			Repeat: s.Instance.Instance.Repeat,
		}, true
	}
	return ValueType{}, false
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
	global *Symbol
	module *Symbol
	local  *Symbol

	stream *Symbol
}

// NewContext creates a new symbol context.
func NewContext() *Context {
	return &Context{
		global: NewRootSymbol(),
		module: NewRootSymbol(),
		local:  NewRootSymbol(),
		stream: NewStreamSymbol(),
	}
}

func (context *Context) ResolveIntrinsic(symbol string) *Symbol {
	switch symbol {
	case "_root":
		return context.module

	case "_parent":
		return context.local

	case "_io":
		return context.stream
	}
	return nil
}

func (context *Context) ResolveLocal(symbol string) *Symbol {
	return context.local.Children[symbol]
}

func (context *Context) ResolveModule(symbol string) *Symbol {
	return context.module.Children[symbol]
}

func (context *Context) ResolveGlobal(symbol string) *Symbol {
	return context.module.Children[symbol]
}

// Resolve resolves the provided symbol and returns it, as well as the scope it
// was resolved in.
func (context *Context) Resolve(symbol string) (*Symbol, ResolutionScope) {
	if sym := context.ResolveIntrinsic(symbol); sym != nil {
		return sym, IntrinsicScope
	}
	if sym := context.ResolveLocal(symbol); sym != nil {
		return sym, LocalScope
	}
	if sym := context.ResolveModule(symbol); sym != nil {
		return sym, ModuleScope
	}
	if sym := context.ResolveGlobal(symbol); sym != nil {
		return sym, GlobalScope
	}

	return nil, GlobalScope
}
