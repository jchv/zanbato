package kaitai

//go:generate go run golang.org/x/tools/cmd/stringer -type=Endianness

// Param specifies a parameter to a struct.
type Param struct {
	Doc  string
	ID   Identifier
	Type TypeRef
	Enum string
}

// Endianness refers to a specific byte ordering.
type Endianness int

const (
	UnspecifiedOrder Endianness = iota
	BigEndian
	LittleEndian
)

// Meta contains the relevant metadata information.
type Meta struct {
	Endian  Endianness
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
