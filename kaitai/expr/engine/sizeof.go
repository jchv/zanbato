package engine

import (
	"strconv"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/types"
)

// ComputeStructSizeStatic computes the static byte size of a struct from its seq fields.
// Returns -1 if the size cannot be determined statically.
func ComputeStructSizeStatic(s *kaitai.Struct) int64 {
	return computeStructSizeStaticHelper(s, 0)
}

// PrimitiveBitSize returns the bit width of a built-in scalar type referenced
// by name (e.g. `u4`, `s2le`, `f8`, `b3`). Returns (0, false) for non-scalar
// or unrecognized names.
func PrimitiveBitSize(name string) (int64, bool) {
	switch name {
	case "u1", "s1":
		return 8, true
	case "u2", "u2le", "u2be", "s2", "s2le", "s2be":
		return 16, true
	case "u4", "u4le", "u4be", "s4", "s4le", "s4be", "f4", "f4le", "f4be":
		return 32, true
	case "u8", "u8le", "u8be", "s8", "s8le", "s8be", "f8", "f8le", "f8be":
		return 64, true
	}
	if len(name) > 1 && name[0] == 'b' {
		n, err := strconv.Atoi(name[1:])
		if err == nil && n > 0 {
			return int64(n), true
		}
	}
	return 0, false
}

// ComputeStructBitSize returns the static bit size of a struct or -1 if the
// size cannot be derived statically.
func ComputeStructBitSize(s *kaitai.Struct) int64 {
	return computeStructBitSizeHelper(s, 0)
}

func computeStructSizeStaticHelper(s *kaitai.Struct, depth int) int64 {
	if depth > 10 {
		return -1 // prevent infinite recursion
	}
	var total int64
	for _, attr := range s.Seq {
		if attr.Repeat != nil || attr.If != nil {
			return -1
		}
		ref := attr.Type.TypeRef
		if ref == nil {
			return -1
		}
		size := ComputeTypeRefSize(ref)
		if size < 0 && ref.Kind == types.User {
			// Try to resolve user type by searching nested types
			size = computeUserTypeSizeStatic(s, ref.User.Name, depth+1)
		}
		if size < 0 {
			return -1
		}
		total += size
	}
	return total
}

func computeUserTypeSizeStatic(parent *kaitai.Struct, name string, depth int) int64 {
	// Search nested types
	for _, child := range parent.Structs {
		if string(child.ID) == name {
			return computeStructSizeStaticHelper(child, depth)
		}
	}
	return -1
}

func computeStructBitSizeHelper(s *kaitai.Struct, depth int) int64 {
	if depth > 10 {
		return -1
	}
	var total int64
	for _, attr := range s.Seq {
		if attr.Repeat != nil || attr.If != nil {
			return -1
		}
		ref := attr.Type.TypeRef
		if ref == nil {
			return -1
		}
		bits := computeTypeRefBitSize(ref)
		if bits < 0 && ref.Kind == types.User {
			for _, child := range s.Structs {
				if string(child.ID) == ref.User.Name {
					bits = computeStructBitSizeHelper(child, depth+1)
					break
				}
			}
		}
		if bits < 0 {
			return -1
		}
		total += bits
	}
	return total
}

func computeTypeRefBitSize(ref *types.TypeRef) int64 {
	if ref.Kind == types.Bits && ref.Bits != nil {
		return int64(ref.Bits.Width)
	}
	if byteSize := ComputeTypeRefSize(ref); byteSize >= 0 {
		return byteSize * 8
	}
	return -1
}

func (context *Context) ComputeStructSize(s *kaitai.Struct) int64 {
	var total int64
	for _, attr := range s.Seq {
		sz := ComputeAttrSize(attr)
		if sz < 0 {
			if attr.Type.TypeRef != nil && attr.Type.TypeRef.Kind == types.User && attr.Repeat == nil {
				childName := attr.Type.TypeRef.User.Name
				for _, child := range s.Structs {
					if string(child.ID) == childName {
						sz = context.ComputeStructSize(child)
						break
					}
				}
				if sz < 0 {
					if r := context.ResolveQualifiedType(childName); r != nil && r.Struct != nil {
						sz = context.ComputeStructSize(r.Struct.Type)
					}
				}
			}
		}
		if sz < 0 {
			return -1
		}
		total += sz
	}
	return total
}

func (context *Context) ComputeStructBitSize(s *kaitai.Struct) int64 {
	var total int64
	for _, attr := range s.Seq {
		if attr.Repeat != nil {
			return -1
		}
		ref := attr.Type.TypeRef
		if ref == nil {
			return -1
		}
		bits := int64(-1)
		if ref.Kind == types.Bits && ref.Bits != nil {
			bits = int64(ref.Bits.Width)
		} else if sz := ComputeAttrSize(attr); sz >= 0 {
			bits = sz * 8
		} else if ref.Kind == types.User {
			childName := ref.User.Name
			for _, child := range s.Structs {
				if string(child.ID) == childName {
					bits = context.ComputeStructBitSize(child)
					break
				}
			}
			if bits < 0 {
				if r := context.ResolveQualifiedType(childName); r != nil && r.Struct != nil {
					bits = context.ComputeStructBitSize(r.Struct.Type)
				}
			}
		}
		if bits < 0 {
			return -1
		}
		total += bits
	}
	return total
}

func ComputeAttrSize(a *kaitai.Attr) int64 {
	if a.Repeat != nil {
		return -1
	}
	if a.Size != nil {
		if intNode, ok := a.Size.Root.(expr.IntNode); ok {
			return intNode.Integer.Int64()
		}
		return -1
	}
	if a.Type.TypeRef == nil {
		return -1
	}
	return ComputeTypeRefSize(a.Type.TypeRef)
}

func ComputeTypeRefSize(ref *types.TypeRef) int64 {
	switch ref.Kind {
	case types.U1, types.S1:
		return 1
	case types.U2, types.U2le, types.U2be, types.S2, types.S2le, types.S2be:
		return 2
	case types.U4, types.U4le, types.U4be, types.S4, types.S4le, types.S4be,
		types.F4, types.F4le, types.F4be:
		return 4
	case types.U8, types.U8le, types.U8be, types.S8, types.S8le, types.S8be,
		types.F8, types.F8le, types.F8be:
		return 8
	case types.Bytes:
		if ref.Bytes != nil && ref.Bytes.Size != nil {
			if intNode, ok := ref.Bytes.Size.Root.(expr.IntNode); ok {
				return intNode.Integer.Int64()
			}
		}
		return -1
	case types.String:
		if ref.String != nil && ref.String.Size != nil {
			if intNode, ok := ref.String.Size.Root.(expr.IntNode); ok {
				return intNode.Integer.Int64()
			}
		}
		return -1
	case types.User:
		return -1 // would need recursive resolution (handled in ComputeStructSize caller)
	default:
		return -1
	}
}
