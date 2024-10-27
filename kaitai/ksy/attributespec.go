package ksy

// AttributeSpec represents a KaitaiStruct attribute.
// #/definitions/Attribute
type AttributeSpec struct {
	ID          Identifier   `yaml:"id,omitempty"`
	Doc         string       `yaml:"doc,omitempty"`
	DocRef      DocRefSpec   `yaml:"doc-ref,omitempty"`
	Contents    ByteSpec     `yaml:"contents,omitempty"`
	Type        AttrTypeSpec `yaml:"type,omitempty"`
	Repeat      RepeatSpec   `yaml:"repeat,omitempty"`
	RepeatExpr  string       `yaml:"repeat-expr,omitempty"`
	RepeatUntil string       `yaml:"repeat-until,omitempty"`
	If          string       `yaml:"if,omitempty"`
	Size        string       `yaml:"size,omitempty"`
	SizeEos     bool         `yaml:"size-eos,omitempty"`
	Process     string       `yaml:"process,omitempty"`
	Enum        string       `yaml:"enum,omitempty"`
	Encoding    string       `yaml:"encoding,omitempty"`
	Terminator  *int         `yaml:"terminator,omitempty"`
	Consume     *bool        `yaml:"consume,omitempty"`
	Include     *bool        `yaml:"include,omitempty"`
	EosError    *bool        `yaml:"eos-error,omitempty"`
	Pos         string       `yaml:"pos,omitempty"`
	IO          string       `yaml:"io,omitempty"`
	Value       string       `yaml:"value,omitempty"`
}

// AttributesSpec represents an attribute list.
// #/definitions/Attributes
type AttributesSpec []AttributeSpec
