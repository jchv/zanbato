package kaitai

// Attr represents an attr inside of a Kaitai type.
type Attr struct {
	ID       Identifier
	Doc      string
	Contents []byte
	Type     Type
	Repeat   RepeatType
	Process  *Expr
	If       *Expr

	// Integers
	Enum string

	// Instances
	Pos   *Expr `yaml:"pos"`
	IO    *Expr `yaml:"io"`
	Value *Expr `yaml:"value"`
}
