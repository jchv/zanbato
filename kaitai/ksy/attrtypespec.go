package ksy

// TypeCaseMapSpec maps cases for type switches.
type TypeCaseMapSpec map[string]string

// AttrTypeSpec decodes Kaitai attribute type specifications.
type AttrTypeSpec struct {
	Value    string          `yaml:"-,omitempty"`
	SwitchOn string          `yaml:"switch-on,omitempty"`
	Cases    TypeCaseMapSpec `yaml:"cases,omitempty"`
}

// UnmarshalText implements encoding.TextUnmarshaler
func (e *AttrTypeSpec) UnmarshalText(text []byte) error {
	*e = AttrTypeSpec{Value: string(text)}
	return nil
}

// MarshalYAML implements yaml.Marshaler
func (e *AttrTypeSpec) MarshalYAML() (interface{}, error) {
	if e.Value != "" {
		return e.Value, nil
	}
	return e, nil
}
