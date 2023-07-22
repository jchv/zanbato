package kaitai

import "strings"

func (s *Struct) ResolveStruct(name string) (*Struct, *Struct) {
	part, rest, ok := strings.Cut(name, "::")
	if rest == "" && ok {
		return nil, nil
	}
	for _, sub := range s.Structs {
		if part == string(sub.ID) {
			if rest != "" {
				return sub.ResolveStruct(rest)
			}
			return s, sub
		}
	}
	return nil, nil
}

func (s *Struct) ResolveEnum(name string) (*Struct, *Enum) {
	i := strings.LastIndex(name, "::")
	if i != -1 {
		_, s = s.ResolveStruct(name[:i])
		name = name[i+2:]
	}
	if s == nil {
		return nil, nil
	}
	for _, enum := range s.Enums {
		if name == string(enum.ID) {
			return s, enum
		}
	}
	return nil, nil
}
