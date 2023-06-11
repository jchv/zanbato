package ksy

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func yamlMapForEach(node *yaml.Node, fn func(key *yaml.Node, value *yaml.Node) error) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node, got kind=%v", node.Kind)
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if err := fn(node.Content[i], node.Content[i+1]); err != nil {
			return err
		}
	}
	return nil
}
