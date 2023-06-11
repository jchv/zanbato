package kaitai

// Param specifies a parameter to a struct.
type Param struct {
	Doc  string
	ID   Identifier
	Type Type
	Enum string
}

// Struct contains a Kaitai struct.
type Struct struct {
	Doc     string
	ID      Identifier
	Params  []*Param
	Seq     []*Attr
	Structs []*Struct
	Enums   []*Enum
}
