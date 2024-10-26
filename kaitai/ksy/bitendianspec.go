package ksy

// BitEndianSpec decodes Kaitai bit endianness specifications.
type BitEndianSpec struct {
	Value string `yaml:"-,omitempty"`
}

// UnmarshalText implements encoding.TextUnmarshaler
func (e *BitEndianSpec) UnmarshalText(text []byte) error {
	*e = BitEndianSpec{Value: string(text)}
	return nil
}

// MarshalYAML implements yaml.Marshaler
func (e *BitEndianSpec) MarshalYAML() (interface{}, error) {
	if e.Value != "" {
		return e.Value, nil
	}
	return e, nil
}
