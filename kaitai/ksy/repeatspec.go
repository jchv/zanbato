package ksy

// RepeatSpec represents the specification of how a field repeats.
type RepeatSpec string

// Enumeration of valid RepeatSpec values.
var (
	ExprRepeatSpec  RepeatSpec = "expr"
	EosRepeatSpec   RepeatSpec = "eos"
	UntilRepeatSpec RepeatSpec = "until"
)
