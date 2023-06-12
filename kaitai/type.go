package kaitai

import (
	"fmt"

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
	Name string
}

// Type contains data for a type specification.
type Type struct {
	Kind Kind

	Bytes  *BytesType
	String *StringType
	User   *UserType
}

func (t Type) FoldEndian(endian Endianness) Type {
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

// Identifier is used to distinguish Kaitai identifiers.
type Identifier string

// ParseAttrType parses a type from an attr.
func ParseAttrType(attr ksy.AttributeSpec) (Type, error) {
	typName := attr.Type.Value
	if attr.Type.Value == "" {
		typName = "bytes"
	}

	typ, err := ParseBasicType(typName)
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

	return typ, nil
}

// ParseBasicType parses a type from a type string.
func ParseBasicType(typestr string) (Type, error) {
	result := Type{}
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
	} else {
		result.Kind = User
		result.User = &UserType{
			Name: typestr,
		}
	}
	return result, nil
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
