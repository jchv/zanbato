package kaitai

import "strings"

// ResolveStruct resolves a relative struct type.
func (root *Struct) ResolveStruct(name string) StructChain {
	part, rest, ok := strings.Cut(name, "::")
	if rest == "" && ok {
		return nil
	}
	for _, subType := range root.Structs {
		if part == string(subType.ID) {
			if rest != "" {
				return subType.ResolveStruct(rest).WithRoot(root)
			}
			return NewChain(subType).WithRoot(root)
		}
	}
	return nil
}

// ResolveEnum resolves a relative enum type.
func (root *Struct) ResolveEnum(name string) (StructChain, *Enum) {
	i := strings.LastIndex(name, "::")
	chain := NewChain(root)
	if i != -1 {
		chain = root.ResolveStruct(name[:i])
		name = name[i+2:]
	}
	if chain == nil {
		return nil, nil
	}
	for _, enum := range chain.Struct().Enums {
		if name == string(enum.ID) {
			return chain, enum
		}
	}
	return nil, nil
}

// HasDependentEndian returns true if the struct's endianness depends on its
// context. This returns false if the struct has an explicit endian or all of
// the types of its attributes have explicit endianness.
func (s *Struct) HasDependentEndian() bool {
	if s.Meta.Endian.Kind == LittleEndian || s.Meta.Endian.Kind == BigEndian {
		return false
	}
	for _, attr := range s.Seq {
		if attr.Type.HasDependentEndian() {
			return true
		}
	}
	return false
}

// StructChain represents a nested path of structs.
type StructChain []*Struct

func NewChain(node *Struct) StructChain {
	return StructChain{node}
}

// Root returns the root node in the chain.
func (s StructChain) Root() *Struct {
	if len(s) > 0 {
		return s[0]
	}
	return nil
}

// Parent returns the chain of the parent of the selected struct.
func (s StructChain) Parent() StructChain {
	if len(s) > 0 {
		return s[:len(s)-1]
	}
	return nil
}

// Struct returns the selected struct itself.
func (s StructChain) Struct() *Struct {
	if len(s) > 0 {
		return s[len(s)-1]
	}
	return nil
}

// WithRoot adds a new root node and nests the chain under it.
func (s StructChain) WithRoot(parent *Struct) StructChain {
	return append(StructChain{parent}, s...)
}
