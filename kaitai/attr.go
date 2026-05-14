package kaitai

import (
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/ksy"
	"github.com/jchv/zanbato/kaitai/types"
)

// Attr represents an attr inside of a Kaitai type.
type Attr struct {
	ID       Identifier
	Doc      string
	Contents []byte
	Type     types.Type
	Repeat   types.RepeatType
	Process  *expr.Expr
	If       *expr.Expr
	Valid    *ksy.ValidSpec

	// Integers
	Enum string

	// Instances
	Pos     *expr.Expr
	Size    *expr.Expr
	SizeEos bool
	IO      *expr.Expr
	Value   *expr.Expr

	// Parent override (from parent: key)
	Parent *ksy.ParentSpec

	// Terminator/pad attributes (needed for user types where these
	// control stream-level reading rather than being stored on the type)
	Terminator *int
	PadRight   *int
	Consume    *bool
	Include    *bool
	EosError   *bool
}
