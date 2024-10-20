package eval

import "github.com/jchv/zanbato/kaitai/expr/engine"

type Range struct {
	StartIndex, EndIndex uint64
}

type Label struct {
	Attr  *engine.ExprValue
	Value *engine.ExprValue
	Index int
}

type Annotation struct {
	Range Range
	Label Label
}
