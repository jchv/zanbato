package types

import "github.com/jchv/zanbato/kaitai/expr"

//go:generate go run golang.org/x/tools/cmd/stringer -type=EndianKind

// EndianKind refers to a specific byte ordering.
type EndianKind int

const (
	UnspecifiedOrder EndianKind = iota
	BigEndian
	LittleEndian
	SwitchEndian
)

type Endian struct {
	Kind     EndianKind
	SwitchOn *expr.Expr
	Cases    map[string]EndianKind
}
