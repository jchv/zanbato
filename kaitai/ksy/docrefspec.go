package ksy

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// DocRefSpec specifies documentation references.
type DocRefSpec []string

func (b *DocRefSpec) decodeScalarYAML(node *yaml.Node) error {
	switch node.Tag {
	case "!!str":
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}
		*b = append(*b, s)
		return nil
	default:
		return fmt.Errorf("unexpected doc-ref tag %s", node.Tag)
	}
}

// UnmarshalYAML implements yaml.Unmarshaler
func (b *DocRefSpec) UnmarshalYAML(node *yaml.Node) error {
	*b = []string{}
	switch node.Kind {
	case yaml.SequenceNode:
		for _, c := range node.Content {
			if err := b.decodeScalarYAML(c); err != nil {
				return err
			}
		}
		return nil
	case yaml.ScalarNode:
		return b.decodeScalarYAML(node)
	default:
		return fmt.Errorf("unexpected bytespec node kind=%d", node.Kind)
	}
}

// MarshalYAML implements yaml.Marshaler
func (b DocRefSpec) MarshalYAML() (interface{}, error) {
	if len(b) == 0 {
		return nil, nil
	}
	if len(b) == 1 {
		return b[0], nil
	}
	return []string(b), nil
}
