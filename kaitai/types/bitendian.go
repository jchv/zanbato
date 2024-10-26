package types

//go:generate go run golang.org/x/tools/cmd/stringer -type=BitEndianKind

// BitEndianKind refers to a specific bit ordering.
type BitEndianKind int

const (
	UnspecifiedBitOrder BitEndianKind = iota
	BigBitEndian
	LittleBitEndian
)

type BitEndian struct {
	Kind BitEndianKind
}
