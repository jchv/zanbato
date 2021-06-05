package ksy

// EndianCaseMapSpec maps cases for endianness switches.
type EndianCaseMapSpec map[string]string

// EndianSpec decodes Kaitai endianness specifications.
type EndianSpec struct {
	Value    string            `yaml:"-,omitempty"`
	SwitchOn string            `yaml:"switch-on,omitempty"`
	Cases    EndianCaseMapSpec `yaml:"cases,omitempty"`
}

// UnmarshalText implements encoding.TextUnmarshaler
func (e *EndianSpec) UnmarshalText(text []byte) error {
	*e = EndianSpec{Value: string(text)}
	return nil
}

// MarshalYAML implements yaml.Marshaler
func (e *EndianSpec) MarshalYAML() (interface{}, error) {
	if e.Value != "" {
		return e.Value, nil
	}
	return e, nil
}
