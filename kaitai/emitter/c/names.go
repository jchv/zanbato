package c

import (
	"strings"
)

func ksToCName(name string) string {
	name = strings.ReplaceAll(name, "::", "__")
	return mangleKeyword(name)
}

var cReservedWords = map[string]struct{}{
	"auto": {}, "break": {}, "case": {}, "char": {}, "const": {},
	"continue": {}, "default": {}, "do": {}, "double": {}, "else": {},
	"enum": {}, "extern": {}, "float": {}, "for": {}, "goto": {},
	"if": {}, "inline": {}, "int": {}, "long": {}, "register": {},
	"restrict": {}, "return": {}, "short": {}, "signed": {}, "sizeof": {},
	"static": {}, "struct": {}, "switch": {}, "typedef": {}, "union": {},
	"unsigned": {}, "void": {}, "volatile": {}, "while": {},
	"_Bool": {}, "_Complex": {}, "_Imaginary": {}, "this": {},
}

func mangleKeyword(name string) string {
	if _, ok := cReservedWords[name]; ok {
		return name + "_"
	}
	return name
}
