package kaitai

// Attr represents an attr inside of a Kaitai type.
type Attr struct {
	ID       Identifier
	Doc      string
	Contents []byte
	Type     Type
	Repeat   RepeatType
	If       *Expr

	// Byte arrays/strings
	Size    *Expr
	SizeEos bool

	// Byte arrays
	Process *Expr

	// Integers
	Enum string

	// Strings
	Encoding string

	// Strings with sentinals
	Terminator int
	Consume    bool `yaml:"consume"`
	Include    bool `yaml:"include"`
	EosError   bool `yaml:"eos-error"`

	// Instances
	Pos   *Expr `yaml:"pos"`
	IO    *Expr `yaml:"io"`
	Value *Expr `yaml:"value"`
}
