package ksy

// ParamSpec represents a parameter.
// #/definitions/ParamSpec
type ParamSpec struct {
	ID     Identifier `yaml:"id,omitempty"`
	Type   string     `yaml:"type,omitempty"`
	Doc    string     `yaml:"doc,omitempty"`
	DocRef string     `yaml:"doc-ref,omitempty"`
	Enum   string     `yaml:"enum,omitempty"`
}

// ParamsSpec represents the parameter list.
// #/definitions/ParamsSpec
type ParamsSpec []ParamSpec
