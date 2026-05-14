package types

import (
	"errors"

	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/ksy"
)

// Assert that the different repeat types implement RepeatType.
var (
	_ = RepeatType(RepeatEOS{})
	_ = RepeatType(RepeatExpr{})
	_ = RepeatType(RepeatUntil{})
)

// RepeatType is a type implemented by the different modes of repeat.
type RepeatType interface{ isRepeatType() }

// RepeatEOS repeatedly reads until the end of the stream.
type RepeatEOS struct{}

// RepeatExpr evaluates an expression and uses it as a count.
type RepeatExpr struct {
	CountExpr *expr.Expr
}

// RepeatUntil repeatedly reads until an expression evaluates to true.
type RepeatUntil struct {
	UntilExpr *expr.Expr
}

func (RepeatEOS) isRepeatType()   {}
func (RepeatExpr) isRepeatType()  {}
func (RepeatUntil) isRepeatType() {}

// ParseRepeat parses repeat specifications from attributes.
func ParseRepeat(spec ksy.AttributeSpec) (RepeatType, error) {
	switch spec.Repeat {
	case ksy.EosRepeatSpec:
		return RepeatEOS{}, nil
	case ksy.ExprRepeatSpec:
		countExpr, err := expr.ParseExpr(spec.RepeatExpr)
		if err != nil {
			return nil, err
		}
		return RepeatExpr{countExpr}, nil
	case ksy.UntilRepeatSpec:
		untilExpr, err := expr.ParseExpr(spec.RepeatUntil)
		if err != nil {
			return nil, err
		}
		return RepeatUntil{untilExpr}, nil
	case "":
		return nil, nil
	default:
		return nil, errors.New("invalid repeat spec")
	}
}
