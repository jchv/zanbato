package kaitai

import "github.com/jchv/zanbato/kaitai/types"

type Identifier = types.Identifier

// Param specifies a parameter to a struct.
type Param struct {
	Doc  string
	ID   Identifier
	Type types.TypeRef
	Enum string
}

// Meta contains the relevant metadata information.
type Meta struct {
	Endian    types.Endian
	BitEndian types.BitEndian
	Imports   []string
}

// Struct contains a Kaitai struct.
type Struct struct {
	Doc       string
	Meta      Meta
	ID        Identifier
	Params    []*Param
	Seq       []*Attr
	Instances []*Attr
	Structs   []*Struct
	Enums     []*Enum
}
