package kaitai

// Kind represents the lowest-level binary type.
type Kind int

// This is an enumeration of all valid kinds.
const (
	U1 Kind = iota
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
func ParseType(typestr string) Type {
	// TODO: Implement
	return Type{}
}
