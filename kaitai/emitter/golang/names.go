package golang

import (
	"strings"
	"unicode"
)

// ksToGoName converts a Kaitai Struct identifier (e.g. "my_type_name") to a
// Go exported name (e.g. "MyTypeName"). It also converts "::" scope separators
// to "__".
func ksToGoName(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(titleCase(strings.ReplaceAll(name, "_", " ")), " ", ""), "::", "__")
}

func titleCase(s string) string {
	prev := ' '
	return strings.Map(
		func(r rune) rune {
			if isSeparator(prev) {
				prev = r
				return unicode.ToTitle(r)
			}
			prev = r
			return r
		},
		s)
}

func isSeparator(r rune) bool {
	if r <= 0x7F {
		switch {
		case '0' <= r && r <= '9':
			return false
		case 'a' <= r && r <= 'z':
			return false
		case 'A' <= r && r <= 'Z':
			return false
		case r == '_':
			return false
		}
		return true
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return false
	}
	return unicode.IsSpace(r)
}
