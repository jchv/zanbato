package ksy

import (
	"fmt"
	"reflect"
	"strconv"

	"gopkg.in/yaml.v3"
)

// InstanceSpec represents a single instance.
type InstanceSpec AttributeSpec

// InstanceSpecItem represents a key/value pair in an instance list.
// #/definitions/InstancesSpec
type InstanceSpecItem struct {
	Key   string
	Value InstanceSpec
}

// InstancesSpec represents an instance list.
// #/definitions/InstancesSpec
type InstancesSpec struct{ Instances []InstanceSpecItem }

// MarshalYAML implements yaml.Marshaler
func (m InstancesSpec) MarshalYAML() (interface{}, error) {
	fields := []reflect.StructField{}
	for i, n := range m.Instances {
		fields = append(fields, reflect.StructField{
			Name: "Field" + strconv.Itoa(i),
			Type: reflect.TypeOf(InstanceSpec{}),
			Tag:  reflect.StructTag(fmt.Sprintf("yaml:%q", n.Key)),
		})
	}
	v := reflect.New(reflect.StructOf(fields)).Elem()
	for i, n := range m.Instances {
		v.Field(i).Set(reflect.ValueOf(n.Value))
	}
	return v.Interface(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler
func (m *InstancesSpec) UnmarshalYAML(node *yaml.Node) error {
	m.Instances = []InstanceSpecItem{}
	return yamlMapForEach(node, func(key *yaml.Node, value *yaml.Node) error {
		var item InstanceSpecItem
		if err := key.Decode(&item.Key); err != nil {
			return err
		}
		if err := value.Decode(&item.Value); err != nil {
			return err
		}
		m.Instances = append(m.Instances, item)
		return nil
	})
}
