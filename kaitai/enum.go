package kaitai

import "math/big"

// EnumValue contains a single enum value.
type EnumValue struct {
	Value *big.Int
	ID    Identifier
}

// Enum contains the definition of an enumeration.
type Enum struct {
	ID     Identifier
	Values []EnumValue
}
