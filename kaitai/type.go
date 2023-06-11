package kaitai

//go:generate go run golang.org/x/tools/cmd/stringer -type=Kind

// Kind represents the lowest-level binary type.
type Kind int

// This is an enumeration of all valid kinds.
const (
	U1 Kind = iota + 1
	U2le
	U2be
	U4le
	U4be
	U8le
	U8be
	S1
	S2le
	S2be
	S4le
	S4be
	S8le
	S8be
	Bits
	F4le
	F4be
	F8le
	F8be
	Bytes
	String
	User
)

// BytesType contains data for bytes types.
type BytesType struct {
	Size       int
	SizeEOS    bool
	Terminator byte
}

// StringType contains data for string types.
type StringType struct {
	Size       int
	SizeEOS    bool
	Terminated bool
	Terminator byte
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

// Identifier is used to distinguish Kaitai identifiers.
type Identifier string

// ParseType parses a type.
func ParseType(typestr string) (Type, error) {
	result := Type{}
	if kind, ok := parseBasicDataType(typestr); ok {
		result.Kind = kind
		switch kind {
		case Bytes:
			result.Bytes = &BytesType{}
		case String:
			result.String = &StringType{}
			if typestr == "strz" {
				result.String.Terminated = true
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
		// TODO: need to handle endianness
		return U2le, true
	case "u4":
		// TODO: need to handle endianness
		return U4le, true
	case "u8":
		// TODO: need to handle endianness
		return U8le, true
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
		// TODO: need to handle endianness
		return S2le, true
	case "s4":
		// TODO: need to handle endianness
		return S4le, true
	case "s8":
		// TODO: need to handle endianness
		return S8le, true
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
	case "f4le":
		return F4le, true
	case "f4be":
		return F4be, true
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
