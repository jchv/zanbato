package kaitai

import (
	"fmt"
	"io"
	"math/big"

	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/ksy"
	"github.com/jchv/zanbato/kaitai/types"

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
		result.Meta.Endian.Kind = types.LittleEndian
	} else if typ.Meta.Endian.Value == "be" {
		result.Meta.Endian.Kind = types.BigEndian
	} else if len(typ.Meta.Endian.Cases) > 0 || typ.Meta.Endian.SwitchOn != "" {
		switchOn, err := expr.ParseExpr(typ.Meta.Endian.SwitchOn)
		if err != nil {
			return nil, err
		}
		result.Meta.Endian.Kind = types.SwitchEndian
		result.Meta.Endian.SwitchOn = switchOn
		result.Meta.Endian.Cases = make(map[string]types.EndianKind)
		for key, value := range typ.Meta.Endian.Cases {
			if value == "le" {
				result.Meta.Endian.Cases[key] = types.LittleEndian
			} else if value == "be" {
				result.Meta.Endian.Cases[key] = types.BigEndian
			} else {
				return nil, fmt.Errorf("unknown endian value %s", value)
			}
		}
	}

	if typ.Meta.BitEndian.Value == "le" {
		result.Meta.BitEndian.Kind = types.LittleBitEndian
	} else if typ.Meta.BitEndian.Value == "be" {
		result.Meta.BitEndian.Kind = types.BigBitEndian
	} else if typ.Meta.BitEndian.Value != "" {
		return nil, fmt.Errorf("unknown bit endian value %s", typ.Meta.BitEndian.Value)
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
	typ, err := types.ParseTypeRef(param.Type)
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
	typ, err := types.ParseAttrType(attr, false)
	if err != nil {
		return nil, err
	}
	return &Attr{
		ID:       Identifier(attr.ID),
		Doc:      attr.Doc,
		Contents: attr.Contents,
		Type:     typ,
		Repeat:   types.ParseRepeat(attr),
		Process:  expr.MustParseExpr(attr.Process),
		If:       expr.MustParseExpr(attr.If),
		Enum:     attr.Enum,
		Pos:      expr.MustParseExpr(attr.Pos),
		IO:       expr.MustParseExpr(attr.IO),
		Value:    expr.MustParseExpr(attr.Value),
	}, nil
}

func translateEnumSpec(id Identifier, typ ksy.EnumSpec) (*Enum, error) {
	result := &Enum{}
	result.ID = id
	for _, val := range typ.Values {
		value := big.NewInt(0)
		value.SetString(val.Value, 0)
		result.Values = append(result.Values, EnumValue{value, Identifier(val.Spec.ID)})
	}
	return result, nil
}

func translateInstanceSpec(spec ksy.InstanceSpecItem) (*Attr, error) {
	typ, err := types.ParseAttrType(ksy.AttributeSpec(spec.Value), true)
	if err != nil {
		return nil, err
	}
	return &Attr{
		ID:       Identifier(spec.Key),
		Doc:      spec.Value.Doc,
		Contents: spec.Value.Contents,
		Type:     typ,
		Repeat:   types.ParseRepeat(ksy.AttributeSpec(spec.Value)),
		Process:  expr.MustParseExpr(spec.Value.Process),
		If:       expr.MustParseExpr(spec.Value.If),
		Enum:     spec.Value.Enum,
		Pos:      expr.MustParseExpr(spec.Value.Pos),
		Size:     expr.MustParseExpr(spec.Value.Size),
		IO:       expr.MustParseExpr(spec.Value.IO),
		Value:    expr.MustParseExpr(spec.Value.Value),
	}, nil
}
