package ksy

import (
	"strconv"

	"gopkg.in/yaml.v3"
)

// EnumValueSpec represents a single enum value.
// #/definitions/EnumSpec
type EnumValueSpec struct {
	Value int
	ID    Identifier
}

// EnumValuesSpec represents multiple EnumValueSpec encoded as a map in YAML.
type EnumValuesSpec []EnumValueSpec

func enumValuesToYAML(e EnumValuesSpec) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	for _, i := range e {
		n.Content = append(n.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: strconv.Itoa(i.Value)},
			&yaml.Node{Kind: yaml.ScalarNode, Value: string(i.ID), Tag: "!!str"})
	}
	return n
}

// MarshalYAML implements yaml.Marshaler
func (e EnumValuesSpec) MarshalYAML() (interface{}, error) {
	return enumValuesToYAML(e), nil
}

// UnmarshalYAML implements yaml.Unmarshaler
func (e *EnumValuesSpec) UnmarshalYAML(node *yaml.Node) error {
	*e = []EnumValueSpec{}
	return yamlMapForEach(node, func(key *yaml.Node, value *yaml.Node) error {
		var item EnumValueSpec
		if err := key.Decode(&item.Value); err != nil {
			return err
		}
		if err := value.Decode(&item.ID); err != nil {
			return err
		}
		*e = append(*e, item)
		return nil
	})
}

// EnumSpec represents an enumeration.
// #/definitions/EnumSpec
type EnumSpec struct {
	ID     Identifier
	Values EnumValuesSpec
}

// EnumsSpec represents the enumeration list.
// #/definitions/EnumSspec
type EnumsSpec []EnumSpec

// MarshalYAML implements yaml.Marshaler
func (e EnumsSpec) MarshalYAML() (interface{}, error) {
	n := &yaml.Node{Kind: yaml.MappingNode}
	for _, i := range e {
		n.Content = append(n.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: string(i.ID)},
			enumValuesToYAML(i.Values))
	}
	return n, nil
}

// UnmarshalYAML implements yaml.Unmarshaler
func (e *EnumsSpec) UnmarshalYAML(node *yaml.Node) error {
	*e = []EnumSpec{}
	return yamlMapForEach(node, func(key *yaml.Node, value *yaml.Node) error {
		var item EnumSpec
		if err := key.Decode(&item.ID); err != nil {
			return err
		}
		if err := value.Decode(&item.Values); err != nil {
			return err
		}
		*e = append(*e, item)
		return nil
	})
}
