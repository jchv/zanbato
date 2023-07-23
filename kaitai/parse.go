package kaitai

import (
	"fmt"
	"io"
	"math/big"

	"github.com/jchv/zanbato/kaitai/ksy"

	"gopkg.in/yaml.v3"
)

// ParseStruct parses a struct from YAML into a kaitai.Struct.
func ParseStruct(r io.Reader) (*Struct, error) {
	root := ksy.TypeSpec{}
	if err := yaml.NewDecoder(r).Decode(&root); err != nil {
		return nil, err
	}
	return translateTypeSpec("", root)
}

func translateTypeSpec(id Identifier, typ ksy.TypeSpec) (*Struct, error) {
	result := &Struct{}
	result.Doc = typ.Doc
	if id == "" {
		result.ID = Identifier(typ.Meta.ID)
	} else {
		result.ID = id
	}

	result.Meta.Imports = typ.Meta.Imports

	if typ.Meta.Endian.Value == "le" {
		result.Meta.Endian.Kind = LittleEndian
	} else if typ.Meta.Endian.Value == "be" {
		result.Meta.Endian.Kind = BigEndian
	} else if len(typ.Meta.Endian.Cases) > 0 || typ.Meta.Endian.SwitchOn != "" {
		switchOn, err := ParseExpr(typ.Meta.Endian.SwitchOn)
		if err != nil {
			return nil, err
		}
		result.Meta.Endian.Kind = SwitchEndian
		result.Meta.Endian.SwitchOn = switchOn
		result.Meta.Endian.Cases = make(map[string]EndianKind)
		for key, value := range typ.Meta.Endian.Cases {
			if value == "le" {
				result.Meta.Endian.Cases[key] = LittleEndian
			} else if value == "be" {
				result.Meta.Endian.Cases[key] = BigEndian
			} else {
				return nil, fmt.Errorf("unknown endian value %s", value)
			}
		}
	}

	for _, spec := range typ.Params {
		param, err := translateParamSpec(spec)
		if err != nil {
			return nil, err
		}
		result.Params = append(result.Params, param)
	}
	for _, spec := range typ.Seq {
		attr, err := translateAttrSpec(spec)
		if err != nil {
			return nil, err
		}
		result.Seq = append(result.Seq, attr)
	}
	for _, spec := range typ.Types {
		typ, err := translateTypeSpec(Identifier(spec.Meta.ID), spec)
		if err != nil {
			return nil, err
		}
		result.Structs = append(result.Structs, typ)
	}
	for _, spec := range typ.Enums {
		enum, err := translateEnumSpec(Identifier(spec.ID), spec)
		if err != nil {
			return nil, err
		}
		result.Enums = append(result.Enums, enum)
	}
	for _, spec := range typ.Instances.Instances {
		instance, err := translateInstanceSpec(spec)
		if err != nil {
			return nil, err
		}
		result.Instances = append(result.Instances, instance)
	}
	return result, nil
}

func translateParamSpec(param ksy.ParamSpec) (*Param, error) {
	typ, err := ParseTypeRef(param.Type)
	if err != nil {
		return nil, err
	}
	return &Param{
		ID:   Identifier(param.ID),
		Doc:  param.Doc,
		Type: typ,
		Enum: param.Enum,
	}, nil
}

func translateAttrSpec(attr ksy.AttributeSpec) (*Attr, error) {
	typ, err := ParseAttrType(attr)
	if err != nil {
		return nil, err
	}
	return &Attr{
		ID:       Identifier(attr.ID),
		Doc:      attr.Doc,
		Contents: attr.Contents,
		Type:     typ,
		Repeat:   ParseRepeat(attr),
		Process:  MustParseExpr(attr.Process),
		If:       MustParseExpr(attr.If),
		Enum:     attr.Enum,
		Pos:      MustParseExpr(attr.Pos),
		IO:       MustParseExpr(attr.IO),
		Value:    MustParseExpr(attr.Value),
	}, nil
}

func translateEnumSpec(id Identifier, typ ksy.EnumSpec) (*Enum, error) {
	result := &Enum{}
	result.ID = id
	for _, val := range typ.Values {
		value := big.NewInt(0)
		value.SetString(val.Value, 0)
		result.Values = append(result.Values, EnumValue{value, Identifier(val.ID)})
	}
	return result, nil
}

func translateInstanceSpec(spec ksy.InstanceSpecItem) (*Attr, error) {
	typ, err := ParseAttrType(ksy.AttributeSpec(spec.Value))
	if err != nil {
		return nil, err
	}
	return &Attr{
		ID:       Identifier(spec.Key),
		Doc:      spec.Value.Doc,
		Contents: spec.Value.Contents,
		Type:     typ,
		Repeat:   ParseRepeat(ksy.AttributeSpec(spec.Value)),
		Process:  MustParseExpr(spec.Value.Process),
		If:       MustParseExpr(spec.Value.If),
		Enum:     spec.Value.Enum,
		Pos:      MustParseExpr(spec.Value.Pos),
		IO:       MustParseExpr(spec.Value.IO),
		Value:    MustParseExpr(spec.Value.Value),
	}, nil
}
