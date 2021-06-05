package kaitai

import "github.com/jchv/zanbato/kaitai/ksy"

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
	CountExpr *Expr
}

// RepeatUntil repeatedly reads until an expression evaluates to true.
type RepeatUntil struct {
	UntilExpr *Expr
}

func (RepeatEOS) isRepeatType()   {}
func (RepeatExpr) isRepeatType()  {}
func (RepeatUntil) isRepeatType() {}

// ParseRepeat parses repeat specifications from attributes.
func ParseRepeat(spec ksy.AttributeSpec) RepeatType {
	switch spec.Repeat {
	case ksy.EosRepeatSpec:
		return RepeatEOS{}
	case ksy.ExprRepeatSpec:
		return RepeatExpr{MustParseExpr(spec.RepeatExpr)}
	case ksy.UntilRepeatSpec:
		return RepeatUntil{MustParseExpr(spec.RepeatUntil)}
	case "":
		return nil
	default:
		panic("invalid repeat spec")
	}
}
