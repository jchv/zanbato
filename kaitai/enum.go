package kaitai

// EnumValue contains a single enum value.
type EnumValue struct {
	Value int
	ID    Identifier
}

// Enum contains the definition of an enumeration.
type Enum struct {
	ID     Identifier
	Values []EnumValue
}
