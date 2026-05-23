package ksy

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ByteSpec specifies a literal of bytes.
type ByteSpec []byte

func (b *ByteSpec) decodeTopScalarYAML(node *yaml.Node) error {
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

func (b *ByteSpec) decodeArrayElementYAML(node *yaml.Node) error {
	switch node.Tag {
	case "!!str":
		var s string
		if err := node.Decode(&s); err != nil {
			return err
		}
		if v, ok := strToByte(s); ok {
			*b = append(*b, v)
			return nil
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

func strToByte(s string) (byte, bool) {
	if s == "" {
		return 0, false
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		v, err := strconv.ParseInt(s[2:], 16, 0)
		if err != nil {
			return 0, false
		}
		if v < -128 || v >= 256 {
			return 0, false
		}
		return byte(v), true
	}
	v, err := strconv.ParseInt(s, 10, 0)
	if err != nil {
		return 0, false
	}
	if v < -128 || v >= 256 {
		return 0, false
	}
	return byte(v), true
}

// UnmarshalYAML implements yaml.Unmarshaler
func (b *ByteSpec) UnmarshalYAML(node *yaml.Node) error {
	*b = []byte{}
	switch node.Kind {
	case yaml.SequenceNode:
		for _, c := range node.Content {
			if err := b.decodeArrayElementYAML(c); err != nil {
				return err
			}
		}
		return nil
	case yaml.ScalarNode:
		return b.decodeTopScalarYAML(node)
	default:
		return fmt.Errorf("unexpected bytespec node kind=%d", node.Kind)
	}
}

// MarshalYAML implements yaml.Marshaler
func (b ByteSpec) MarshalYAML() (any, error) {
	n := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, i := range b {
		n.Content = append(n.Content, &yaml.Node{
			Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprintf("0x%02x", i)})
	}
	return n, nil
}
