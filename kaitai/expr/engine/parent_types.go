package engine

import (
	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/types"
)

// ParentTypes is the result of the parent-type inference pass. It records,
// for each user type, which containing struct uses it.
//
// Inferred is the single inferred parent for each struct. A nil entry means
// the struct is referenced from multiple distinct parents (ambiguous) or via
// a custom `parent:` expression whose target cannot be determined statically;
// emitters should treat nil as "generic/any parent" in that case.
//
// First is the first containing parent recorded for each struct during the
// walk. Unlike Inferred, it is never downgraded to nil for ambiguity, so it
// can be used as a typed fallback when a concrete parent type is desired
// regardless of strict correctness.
//
// Absent entries (for either map) mean the struct was not reached during the
// walk.
type ParentTypes struct {
	Inferred map[*kaitai.Struct]*ExprValue
	First    map[*kaitai.Struct]*ExprValue
}

// BuildParentTypeMap implements the Kaitai parent-type inference pass. It
// walks the struct hierarchy rooted at `root` and, for each user type
// referenced from a seq/instance attribute, records the containing struct
// as that type's parent. When a struct is referenced from multiple distinct
// parents, its Inferred entry becomes nil (ambiguous).
//
//   - `parent: false` skips the attribute entirely.
//   - `parent: _parent` re-roots the recorded parent to the containing
//     struct's own parent.
//   - Any other `parent: <expr>` is treated as ambiguous, since the runtime
//     target cannot be determined statically; the referenced struct's
//     Inferred entry is forced to nil.
//
// ctx is used only to resolve type names when handling custom `parent:`
// expressions where there is no containing scope to consult.
func BuildParentTypeMap(ctx *Context, root *ExprValue) ParentTypes {
	pt := ParentTypes{
		Inferred: make(map[*kaitai.Struct]*ExprValue),
		First:    make(map[*kaitai.Struct]*ExprValue),
	}
	walkStructForParents(ctx, &pt, root)
	return pt
}

func walkStructForParents(ctx *Context, pt *ParentTypes, structType *ExprValue) {
	if structType == nil || structType.Struct == nil || structType.Struct.Type == nil {
		return
	}
	ks := structType.Struct.Type
	for _, attr := range ks.Seq {
		recordParentUsage(ctx, pt, structType, attr)
	}
	for _, inst := range ks.Instances {
		recordParentUsage(ctx, pt, structType, inst)
	}
	for _, childType := range structType.Struct.Structs {
		walkStructForParents(ctx, pt, childType)
	}
}

func recordParentUsage(ctx *Context, pt *ParentTypes, containing *ExprValue, attr *kaitai.Attr) {
	if attr.Parent != nil && attr.Parent.Disabled {
		return
	}
	effectiveContaining := containing
	if attr.Parent != nil && attr.Parent.Expr != "" {
		switch attr.Parent.Expr {
		case "_parent":
			if containing.Parent != nil {
				effectiveContaining = containing.Parent
			}
		default:
			// Custom expression: can't statically determine the target. Mark
			// any user types this attribute references as ambiguous.
			if attr.Type.TypeRef != nil && attr.Type.TypeRef.Kind == types.User {
				recordParentForType(ctx, pt, nil, attr.Type.TypeRef.User.Name)
			}
			if attr.Type.TypeSwitch != nil {
				for _, c := range attr.Type.TypeSwitch.Cases {
					if c.Kind == types.User {
						recordParentForType(ctx, pt, nil, c.User.Name)
					}
				}
			}
			return
		}
	}
	if attr.Type.TypeRef != nil && attr.Type.TypeRef.Kind == types.User {
		recordParentForType(ctx, pt, effectiveContaining, attr.Type.TypeRef.User.Name)
	}
	if attr.Type.TypeSwitch != nil {
		for _, c := range attr.Type.TypeSwitch.Cases {
			if c.Kind == types.User {
				recordParentForType(ctx, pt, effectiveContaining, c.User.Name)
			}
		}
	}
}

func recordParentForType(ctx *Context, pt *ParentTypes, containing *ExprValue, typeName string) {
	if containing == nil {
		resolved, _ := ctx.ResolveType(typeName)
		if resolved == nil || resolved.Kind != StructKind || resolved.Struct == nil || resolved.Struct.Type == nil {
			return
		}
		pt.Inferred[resolved.Struct.Type] = nil
		return
	}
	resolved := containing.TypeChild(typeName)
	if resolved == nil {
		for p := containing.Parent; p != nil; p = p.Parent {
			resolved = p.TypeChild(typeName)
			if resolved != nil {
				break
			}
		}
	}
	if resolved == nil || resolved.Kind != StructKind || resolved.Struct == nil || resolved.Struct.Type == nil {
		return
	}
	target := resolved.Struct.Type
	if _, ok := pt.First[target]; !ok {
		pt.First[target] = containing
	}
	existing, ok := pt.Inferred[target]
	if !ok {
		pt.Inferred[target] = containing
	} else if existing != nil && existing.Struct != nil && containing.Struct != nil && existing.Struct.Type != containing.Struct.Type {
		pt.Inferred[target] = nil
	}
}
