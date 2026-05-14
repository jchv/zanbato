package engine

// RuntimeRef is an optional hook on a struct- or array-kind ExprValue that
// lets the expression evaluator look up children/items lazily, without the
// runtime having to pre-populate them all into ExprValue.Children/Items.
//
// LookupChild returns the runtime ExprValue for the named field, materialising
// it if needed. Returning (nil, false) means "no such name". Returning
// (nil, true) means "the field exists but is null" (e.g. an if:false seq field).
//
// LookupIndex is analogous for array indices.
//
// Len returns the array length, if known. Used for the .size builtin on lazy
// arrays where opVal.Items may not be populated.
//
// Implementations should be cheap when no resolution is needed, and should
// resolve lazily otherwise. The expression engine consults Runtime only after
// its in-memory Children/Items lookup misses, so this is a true escape hatch
// for lazy navigation.
type RuntimeRef interface {
	LookupChild(name string) (*ExprValue, bool)
	LookupIndex(i int) (*ExprValue, bool)
	Len() (int, bool)
}

// PrimitiveCaster is an optional capability for runtime references that can
// service `.as<TYPE>` casts directly from a stream (e.g. opaque external
// user types in upstream KS test fixtures). Returning (nil, false) means
// the runtime does not handle the cast and the engine should fall through
// to its normal CastNode logic.
type PrimitiveCaster interface {
	CastTo(typeName string) (*ExprValue, bool)
}
