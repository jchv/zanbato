package kaitai

import (
	"github.com/jchv/zanbato/kaitai/expr"
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

	// Integers
	Enum string

	// Instances
	Pos   *expr.Expr
	Size  *expr.Expr
	IO    *expr.Expr
	Value *expr.Expr
}
