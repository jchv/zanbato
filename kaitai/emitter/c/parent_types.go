package c

import (
	"github.com/jchv/zanbato/kaitai"
)

// parentFallbackCType returns the C type for `ks`'s first-recorded
// containing parent, regardless of whether other usage sites disagreed
// (in which case the strict parentCType lookup returns ""). Used when a
// typed fallback is needed even in the presence of ambiguity.
func (e *Emitter) parentFallbackCType(ks *kaitai.Struct) string {
	if e.file.parents.First == nil {
		return ""
	}
	p, ok := e.file.parents.First[ks]
	if !ok || p == nil || p.Struct == nil || p.Struct.Type == nil {
		return ""
	}
	return "struct " + e.prefix(p.DefParent) + e.typeName(p.Struct.Type.ID) + " *"
}

// parentCType returns the C type of `ks`'s inferred parent, or "" if the
// parent is ambiguous (multiple usage sites with different parents) or the
// build pass hasn't run.
func (e *Emitter) parentCType(ks *kaitai.Struct) string {
	if e.file.parents.Inferred == nil {
		return ""
	}
	parent, ok := e.file.parents.Inferred[ks]
	if !ok || parent == nil {
		return ""
	}
	return "struct " + e.prefix(parent.DefParent) + e.typeName(parent.Struct.Type.ID) + " *"
}
