package ksy

import (
	"gopkg.in/yaml.v3"
)

// EnumValueSpec represents a single enum value spec.
// #/definitions/EnumValueSpec
type EnumValueSpec struct {
	ID     Identifier
	Doc    string
	DocRef DocRefSpec
}

// EnumValuePairSpec represents a single enum value pair.
// #/definitions/EnumSpec
type EnumValuePairSpec struct {
	Value string
	Spec  EnumValueSpec
}

// EnumValuePairsSpec represents multiple EnumValueSpec encoded as a map in YAML.
type EnumValuePairsSpec []EnumValuePairSpec

func enumValuesToYAML(e EnumValuePairsSpec) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	for _, i := range e {
		n.Content = append(n.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: i.Value},
			&yaml.Node{Kind: yaml.ScalarNode, Value: string(i.Spec.ID), Tag: "!!str"})
	}
	return n
}

// MarshalYAML implements yaml.Marshaler
func (e EnumValuePairsSpec) MarshalYAML() (interface{}, error) {
	return enumValuesToYAML(e), nil
}

// UnmarshalYAML implements yaml.Unmarshaler
func (e *EnumValuePairsSpec) UnmarshalYAML(node *yaml.Node) error {
	*e = []EnumValuePairSpec{}
	return yamlMapForEach(node, func(key *yaml.Node, value *yaml.Node) error {
		var item EnumValuePairSpec
		if err := key.Decode(&item.Value); err != nil {
			return err
		}
		switch value.Tag {
		case "!!map":
			if err := value.Decode(&item.Spec); err != nil {

			}
		default:
			if err := value.Decode(&item.Spec.ID); err != nil {
				return err
			}
		}
		*e = append(*e, item)
		return nil
	})
}

// EnumSpec represents an enumeration.
// #/definitions/EnumSpec
type EnumSpec struct {
	ID     Identifier
	Values EnumValuePairsSpec
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
