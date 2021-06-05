package kaitai

import (
	"io"

	"github.com/jchv/zanbato/kaitai/ksy"

	"gopkg.in/yaml.v3"
)

// ParseStruct parses a struct from YAML into a kaitai.Struct.
func ParseStruct(r io.Reader) (*Struct, error) {
	root := ksy.TypeSpec{}
	if err := yaml.NewDecoder(r).Decode(&root); err != nil {
		return nil, err
	}
	return translateTypeSpec("", root), nil
}

func translateTypeSpec(id Identifier, typ ksy.TypeSpec) *Struct {
	result := &Struct{}
	result.Doc = typ.Doc
	if id == "" {
		result.ID = Identifier(typ.Meta.ID)
	} else {
		result.ID = id
	}
	for _, spec := range typ.Seq {
		result.Attrs = append(result.Attrs, translateAttrSpec(spec))
	}
	for _, spec := range typ.Types {
		result.Structs = append(result.Structs, translateTypeSpec(Identifier(spec.Meta.ID), spec))
	}
	for _, spec := range typ.Enums {
		result.Enums = append(result.Enums, translateEnumSpec(Identifier(spec.ID), spec))
	}
	return result
}

func translateAttrSpec(typ ksy.AttributeSpec) *Attr {
	return &Attr{
		ID:         Identifier(typ.ID),
		Doc:        typ.Doc,
		Contents:   typ.Contents,
		Type:       ParseType(typ.Type.Value),
		Repeat:     ParseRepeat(typ),
		If:         MustParseExpr(typ.If),
		Size:       MustParseExpr(typ.Size),
		SizeEos:    typ.SizeEos,
		Process:    MustParseExpr(typ.Process),
		Enum:       typ.Enum,
		Encoding:   typ.Encoding,
		Terminator: typ.Terminator,
		Consume:    typ.Consume,
		Include:    typ.Include,
		EosError:   typ.EosError,
		Pos:        MustParseExpr(typ.Pos),
		IO:         MustParseExpr(typ.IO),
		Value:      MustParseExpr(typ.Value),
	}
}

func translateEnumSpec(id Identifier, typ ksy.EnumSpec) *Enum {
	result := &Enum{}
	result.ID = id
	for _, val := range typ.Values {
		result.Values = append(result.Values, EnumValue{val.Value, Identifier(val.ID)})
	}
	return result
}
