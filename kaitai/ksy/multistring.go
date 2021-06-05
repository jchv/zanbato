package ksy

// MultiString can decode a scalar string or an array of strings.
type MultiString []string

// UnmarshalText implements encoding.TextUnmarshaler
func (m *MultiString) UnmarshalText(text []byte) error {
	*m = append(*m, string(text))
	return nil
}
