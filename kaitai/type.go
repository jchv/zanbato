package kaitai

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/jchv/zanbato/kaitai/ksy"
)

//go:generate go run golang.org/x/tools/cmd/stringer -type=Kind

// Kind represents the lowest-level binary type.
type Kind int

// This is an enumeration of all valid kinds.
const (
	U1 Kind = iota + 1
	U2
	U2le
	U2be
	U4
	U4le
	U4be
	U8
	U8le
	U8be
	S1
	S2
	S2le
	S2be
	S4
	S4le
	S4be
	S8
	S8le
	S8be
	Bits
	F4
	F4le
	F4be
	F8
	F8le
	F8be
	Bytes
	String
	User
	UntypedNum
)

// BytesType contains data for bytes types.
type BytesType struct {
	Size       *Expr
	SizeEOS    bool
	Terminator int
	Consume    bool
	Include    bool
	EosError   bool
}

// StringType contains data for string types.
type StringType struct {
	Size       *Expr
	SizeEOS    bool
	Encoding   string
	Terminator int
	Consume    bool
	Include    bool
	EosError   bool
}

// UserType contains data for user types.
type UserType struct {
	Name   string
	Params []*Expr
}

// TypeSwitch contains a set of possible types.
type TypeSwitch struct {
	FieldName Identifier // Name of the field; this is for identity purposes.
	SwitchOn  *Expr
	Cases     map[string]TypeRef
}

type TypeRef struct {
	Kind   Kind
	Bytes  *BytesType
	String *StringType
	User   *UserType
}

// Type contains data for a type specification.
type Type struct {
	TypeRef    *TypeRef
	TypeSwitch *TypeSwitch
}

func (t TypeRef) FoldEndian(endian Endianness) TypeRef {
	switch endian {
	case LittleEndian:
		switch t.Kind {
		case U2:
			t.Kind = U2le
		case U4:
			t.Kind = U4le
		case U8:
			t.Kind = U8le
		case S2:
			t.Kind = S2le
		case S4:
			t.Kind = S4le
		case S8:
			t.Kind = S8le
		case F4:
			t.Kind = F4le
		case F8:
			t.Kind = F8le
		}
	case BigEndian:
		switch t.Kind {
		case U2:
			t.Kind = U2be
		case U4:
			t.Kind = U4be
		case U8:
			t.Kind = U8be
		case S2:
			t.Kind = S2be
		case S4:
			t.Kind = S4be
		case S8:
			t.Kind = S8be
		case F4:
			t.Kind = F4be
		case F8:
			t.Kind = F8be
		}
	}
	return t
}

func (t TypeSwitch) FoldEndian(endian Endianness) TypeSwitch {
	cases := make(map[string]TypeRef)
	for key, value := range t.Cases {
		cases[key] = value.FoldEndian(endian)
	}
	t.Cases = cases
	return t
}

func (t Type) FoldEndian(endian Endianness) Type {
	typeRef := t.TypeRef
	if typeRef != nil {
		newTypeRef := typeRef.FoldEndian(endian)
		typeRef = &newTypeRef
	}
	typeSwitch := t.TypeSwitch
	if typeSwitch != nil {
		newTypeSwitch := typeSwitch.FoldEndian(endian)
		typeSwitch = &newTypeSwitch
	}
	return Type{
		TypeRef:    typeRef,
		TypeSwitch: typeSwitch,
	}
}

// Identifier is used to distinguish Kaitai identifiers.
type Identifier string

// ParseAttrType parses a type from an attr.
func ParseAttrType(attr ksy.AttributeSpec) (Type, error) {
	if attr.Type.Value != "" && attr.Type.SwitchOn != "" {
		return Type{}, errors.New("attr specifies both typeref and switch type")
	}

	if attr.Type.SwitchOn != "" {
		switchOn, err := ParseExpr(attr.Type.SwitchOn)
		if err != nil {
			return Type{}, fmt.Errorf("parsing attr switch-on statement: %w", err)
		}

		cases := make(map[string]TypeRef)
		for key, value := range attr.Type.Cases {
			cases[key], err = ParseTypeRef(value)
			if err != nil {
				return Type{}, fmt.Errorf("parsing case %q type: %w", key, err)
			}
		}

		return Type{TypeSwitch: &TypeSwitch{
			FieldName: Identifier(attr.ID),
			SwitchOn:  switchOn,
			Cases:     cases,
		}}, nil
	} else {
		// Default to bytes if not specified.
		typName := attr.Type.Value
		if typName == "" {
			typName = "bytes"
		}

		typ, err := ParseTypeRef(typName)
		if err != nil {
			return Type{}, err
		}

		if attr.Size != "" {
			sizeExpr, err := ParseExpr(attr.Size)
			if err != nil {
				return Type{}, err
			}
			switch typ.Kind {
			case Bytes:
				typ.Bytes.Size = sizeExpr
			case String:
				typ.String.Size = sizeExpr
			default:
				return Type{}, fmt.Errorf("size on type %s not supported", typ.Kind)
			}
		}

		if attr.Contents != nil {
			switch typ.Kind {
			case Bytes:
				typ.Bytes.Size = &Expr{Root: IntNode{Value: *big.NewInt(int64(len(attr.Contents)))}}
			case String:
				typ.String.Size = &Expr{Root: IntNode{Value: *big.NewInt(int64(len(attr.Contents)))}}
			default:
				return Type{}, fmt.Errorf("contents on type %s not supported", typ.Kind)
			}
		}

		if attr.SizeEos {
			switch typ.Kind {
			case Bytes:
				typ.Bytes.SizeEOS = attr.SizeEos
			case String:
				typ.String.SizeEOS = attr.SizeEos
			default:
				return Type{}, fmt.Errorf("size-eos on type %s not supported", typ.Kind)
			}
		}

		if attr.Encoding != "" {
			switch typ.Kind {
			case String:
				typ.String.Encoding = attr.Encoding
			default:
				return Type{}, fmt.Errorf("encoding on type %s not supported", typ.Kind)
			}
		}

		if attr.Terminator != nil {
			switch typ.Kind {
			case Bytes:
				typ.Bytes.Terminator = *attr.Terminator
			case String:
				typ.String.Terminator = *attr.Terminator
			default:
				return Type{}, fmt.Errorf("terminator on type %s not supported", typ.Kind)
			}
		}

		if attr.Consume != nil {
			switch typ.Kind {
			case Bytes:
				typ.Bytes.Consume = *attr.Consume
			case String:
				typ.String.Consume = *attr.Consume
			default:
				return Type{}, fmt.Errorf("consume on type %s not supported", typ.Kind)
			}
		}

		if attr.Include != nil {
			switch typ.Kind {
			case Bytes:
				typ.Bytes.Include = *attr.Include
			case String:
				typ.String.Include = *attr.Include
			default:
				return Type{}, fmt.Errorf("include on type %s not supported", typ.Kind)
			}
		}

		if attr.EosError != nil {
			switch typ.Kind {
			case Bytes:
				typ.Bytes.EosError = *attr.EosError
			case String:
				typ.String.EosError = *attr.EosError
			default:
				return Type{}, fmt.Errorf("eos-error on type %s not supported", typ.Kind)
			}
		}

		return Type{TypeRef: &typ}, nil
	}
}

// parseUserType parses a user type string.
func parseUserType(typestr string) (TypeRef, error) {
	result := UserType{}
	result.Name = typestr
	// This is kind of stinky, should probably lex this properly.
	if i := strings.IndexByte(typestr, '('); i >= 0 {
		j := strings.LastIndexByte(typestr, ')')
		if j < 0 {
			return TypeRef{}, errors.New("missing ) in type params")
		}
		result.Name = typestr[:i]
		params := strings.Split(typestr[i+1:j], ",")
		for i, src := range params {
			param, err := ParseExpr(src)
			if err != nil {
				return TypeRef{}, fmt.Errorf("in parameter %d of %s: %w", i+1, result.Name, err)
			}
			result.Params = append(result.Params, param)
		}
	}
	return TypeRef{Kind: User, User: &result}, nil
}

// ParseTypeRef parses a type from a type string.
func ParseTypeRef(typestr string) (TypeRef, error) {
	result := TypeRef{}
	if kind, ok := parseBasicDataType(typestr); ok {
		result.Kind = kind
		switch kind {
		case Bytes:
			result.Bytes = &BytesType{
				Consume:  true,
				EosError: true,
			}
		case String:
			result.String = &StringType{
				Consume:    true,
				EosError:   true,
				Terminator: -1,
			}
			if typestr == "strz" {
				result.String.Terminator = 0
			}
		}
		return result, nil
	}
	return parseUserType(typestr)
}

func parseBasicDataType(typestr string) (Kind, bool) {
	switch typestr {
	case "u1":
		return U1, true
	case "u2":
		return U2, true
	case "u4":
		return U4, true
	case "u8":
		return U8, true
	case "u2le":
		return U2le, true
	case "u2be":
		return U2be, true
	case "u4le":
		return U4le, true
	case "u4be":
		return U4be, true
	case "u8le":
		return U8le, true
	case "u8be":
		return U8be, true
	case "s1":
		return S1, true
	case "s2":
		return S2, true
	case "s4":
		return S4, true
	case "s8":
		return S8, true
	case "s2le":
		return S2le, true
	case "s2be":
		return S2be, true
	case "s4le":
		return S4le, true
	case "s4be":
		return S4be, true
	case "s8le":
		return S8le, true
	case "s8be":
		return S8be, true
	case "bits":
		return Bits, true
	case "f4":
		return F4, true
	case "f4le":
		return F4le, true
	case "f4be":
		return F4be, true
	case "f8":
		return F8, true
	case "f8le":
		return F8le, true
	case "f8be":
		return F8be, true
	case "bytes":
		return Bytes, true
	case "str", "strz":
		return String, true
	}
	return Kind(0), false
}
