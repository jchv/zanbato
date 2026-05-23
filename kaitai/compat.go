package kaitai

import "fmt"

// Compatibility is an enumeration that provides options for configuring
// compatibility with different implementations of Kaitai Struct.
//
// By default, Zanbato should behave in a way that is compatible with any
// intentional  Kaitai Struct behavior, but may not always emulate all Kaitai
// Struct bugs. Using compatibility options will enable further bug
// compatibility.
//
// Generally, you should not compare against this value directly unless to
// see if it equals ZanbatoNative. Instead, prefer using helper methods like
// HasCalcIntTypeTruncationBug(). If one doesn't exist for a specific behavior,
// it should be added.
type Compatibility int

const (
	// ZanbatoNative does not enable any bug compatibility options. This mode
	// is the default mode and should have the highest spec-accuracy.
	ZanbatoNative Compatibility = iota

	// KaitaiStruct_0_11 emulates Kaitai Struct 0.11. This will cause all
	// integer arithmetic operations in expressions to be truncated to 32-bits.
	KaitaiStruct_0_11
)

// String returns the name of the Compatibility value. Implements flag.Value.
func (c Compatibility) String() string {
	switch c {
	case ZanbatoNative:
		return "native"
	case KaitaiStruct_0_11:
		return "0.11"
	default:
		return fmt.Sprintf("Compatibility(%d)", int(c))
	}
}

// Set sets the Compatibility value from a string. Implements flag.Value.
func (c *Compatibility) Set(s string) error {
	switch s {
	case "native":
		*c = ZanbatoNative
	case "0.11":
		*c = KaitaiStruct_0_11
	default:
		return fmt.Errorf("unknown compatibility mode %q; valid values are native, 0.11", s)
	}
	return nil
}

// HasCalcIntTypeTruncationBug returns whether or not the compatibility version
// has the behavior of truncating all binary integer operator results to 32-bit
// signed integers.
func (c Compatibility) HasCalcIntTypeTruncationBug() bool {
	switch c {
	case ZanbatoNative:
		return false
	case KaitaiStruct_0_11:
		return true
	default:
		return false
	}
}
