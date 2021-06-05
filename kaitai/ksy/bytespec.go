package ksy

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ByteSpec specifies a literal of bytes.
type ByteSpec []byte

func (b *ByteSpec) decodeScalarYAML(node *yaml.Node) error {
	switch node.Tag {
	case "!!str":
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}
		*b = append(*b, s...)
		return nil
	case "!!int":
		var i byte
		if err := node.Decode(&i); err != nil {
			return err
		}
		*b = append(*b, i)
		return nil
	default:
		return fmt.Errorf("unexpected bytespec tag %s", node.Tag)
	}
}

// UnmarshalYAML implements yaml.Unmarshaler
func (b *ByteSpec) UnmarshalYAML(node *yaml.Node) error {
	*b = []byte{}
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
func (b ByteSpec) MarshalYAML() (interface{}, error) {
	n := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, i := range b {
		n.Content = append(n.Content, &yaml.Node{
			Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprintf("0x%02x", i)})
	}
	return n, nil
}
