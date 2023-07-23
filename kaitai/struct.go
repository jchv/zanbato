package kaitai

//go:generate go run golang.org/x/tools/cmd/stringer -type=EndianKind

// Param specifies a parameter to a struct.
type Param struct {
	Doc  string
	ID   Identifier
	Type TypeRef
	Enum string
}

// EndianKind refers to a specific byte ordering.
type EndianKind int

const (
	UnspecifiedOrder EndianKind = iota
	BigEndian
	LittleEndian
	SwitchEndian
)

type Endian struct {
	Kind     EndianKind
	SwitchOn *Expr
	Cases    map[string]EndianKind
}

// Meta contains the relevant metadata information.
type Meta struct {
	Endian  Endian
	Imports []string
}

// Struct contains a Kaitai struct.
type Struct struct {
	Doc     string
	Meta    Meta
	ID      Identifier
	Params  []*Param
	Seq     []*Attr
	Structs []*Struct
	Enums   []*Enum
}
