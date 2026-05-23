package kaitai

import (
	"fmt"
	"io"
	"math/big"
	"strings"

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
	result.Meta.Encoding = typ.Meta.Encoding
	result.Meta.OpaqueTypes = typ.Meta.KSOpaqueTypes
	result.Meta.Debug = typ.Meta.KSDebug

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
			switch value {
			case "le":
				result.Meta.Endian.Cases[key] = types.LittleEndian
			case "be":
				result.Meta.Endian.Cases[key] = types.BigEndian
			default:
				return nil, fmt.Errorf("unknown endian value %s", value)
			}
		}
	}

	if typ.Meta.BitEndian.Value != "" {
		switch typ.Meta.BitEndian.Value {
		case "le":
			result.Meta.BitEndian.Kind = types.LittleBitEndian
		case "be":
			result.Meta.BitEndian.Kind = types.BigBitEndian
		default:
			return nil, fmt.Errorf("unknown bit endian value %s", typ.Meta.BitEndian.Value)
		}
	}

	for _, spec := range typ.Params {
		param, err := translateParamSpec(spec)
		if err != nil {
			return nil, err
		}
		result.Params = append(result.Params, param)
	}
	anonIdx := 0
	for _, spec := range typ.Seq {
		attr, err := translateAttrSpec(spec, typ.Meta.Encoding)
		if err != nil {
			return nil, err
		}
		if attr.ID == "" {
			attr.ID = Identifier(fmt.Sprintf("_unnamed%d", anonIdx))
			anonIdx++
		}
		result.Seq = append(result.Seq, attr)
	}
	for _, spec := range typ.Types {
		// Inherit meta encoding from parent if child doesn't have its own
		childSpec := spec
		if childSpec.Meta.Encoding == "" && typ.Meta.Encoding != "" {
			childSpec.Meta.Encoding = typ.Meta.Encoding
		}
		child, err := translateTypeSpec(Identifier(childSpec.Meta.ID), childSpec)
		if err != nil {
			return nil, err
		}
		result.Structs = append(result.Structs, child)
	}
	for _, spec := range typ.Enums {
		enum, err := translateEnumSpec(Identifier(spec.ID), spec)
		if err != nil {
			return nil, err
		}
		result.Enums = append(result.Enums, enum)
	}
	for _, spec := range typ.Instances.Instances {
		instance, err := translateInstanceSpec(spec, typ.Meta.Encoding)
		if err != nil {
			return nil, err
		}
		result.Instances = append(result.Instances, instance)
	}
	result.ToString = typ.ToString
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

func translateAttrSpec(attr ksy.AttributeSpec, defaultEncoding string) (*Attr, error) {
	return buildAttr(Identifier(attr.ID), attr, defaultEncoding, false)
}

func translateEnumSpec(id Identifier, typ ksy.EnumSpec) (*Enum, error) {
	result := &Enum{}
	result.ID = id
	for _, val := range typ.Values {
		value := big.NewInt(0)
		if _, ok := value.SetString(val.Value, 0); !ok {
			return nil, fmt.Errorf("unable to parse %q as int in enum %q", val.Value, id)
		}
		result.Values = append(result.Values, EnumValue{value, Identifier(val.Spec.ID)})
	}
	return result, nil
}

func translateInstanceSpec(spec ksy.InstanceSpecItem, defaultEncoding string) (*Attr, error) {
	return buildAttr(Identifier(spec.Key), ksy.AttributeSpec(spec.Value), defaultEncoding, true)
}

func buildAttr(id Identifier, attr ksy.AttributeSpec, defaultEncoding string, instance bool) (*Attr, error) {
	// Propagate meta-level encoding to string attrs that don't have their own.
	if attr.Encoding == "" && defaultEncoding != "" {
		typVal := strings.TrimSpace(attr.Type.Value)
		if typVal == "str" || typVal == "strz" {
			attr.Encoding = defaultEncoding
		}
	}
	typ, err := types.ParseAttrType(attr, instance)
	if err != nil {
		return nil, err
	}

	repeat, err := types.ParseRepeat(attr)
	if err != nil {
		return nil, fmt.Errorf("parsing repeat expression for attr %q: %w", id, err)
	}

	parseOptionalExpr := func(name, src string) (*expr.Expr, error) {
		parsed, err := expr.ParseExpr(src)
		if err != nil {
			return nil, fmt.Errorf("parsing %s expression for attr %q: %w", name, id, err)
		}
		return parsed, nil
	}

	process, err := parseOptionalExpr("process", attr.Process)
	if err != nil {
		return nil, err
	}
	ifExpr, err := parseOptionalExpr("if", attr.If)
	if err != nil {
		return nil, err
	}
	pos, err := parseOptionalExpr("pos", attr.Pos)
	if err != nil {
		return nil, err
	}
	size, err := parseOptionalExpr("size", attr.Size)
	if err != nil {
		return nil, err
	}
	ioExpr, err := parseOptionalExpr("io", attr.IO)
	if err != nil {
		return nil, err
	}
	value, err := parseOptionalExpr("value", attr.Value)
	if err != nil {
		return nil, err
	}

	return &Attr{
		ID:         id,
		Doc:        attr.Doc,
		Contents:   attr.Contents,
		Type:       typ,
		Repeat:     repeat,
		Process:    process,
		If:         ifExpr,
		Valid:      attr.Valid,
		Enum:       attr.Enum,
		Pos:        pos,
		Size:       size,
		SizeEos:    attr.SizeEos,
		IO:         ioExpr,
		Value:      value,
		Parent:     attr.Parent,
		Terminator: attr.Terminator,
		PadRight:   attr.PadRight,
		Consume:    attr.Consume,
		Include:    attr.Include,
		EosError:   attr.EosError,
	}, nil
}
