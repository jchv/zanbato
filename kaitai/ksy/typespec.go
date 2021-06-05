package ksy

import (
	"fmt"
	"reflect"
	"strconv"

	"gopkg.in/yaml.v3"
)

// TypeSpec represents a KaitaiStruct type.
type TypeSpec struct {
	Meta      MetaSpec       `yaml:"meta,omitempty"`
	Params    ParamsSpec     `yaml:"params,omitempty"`
	Seq       AttributesSpec `yaml:"seq,omitempty"`
	Types     TypesSpec      `yaml:"types,omitempty"`
	Enums     EnumsSpec      `yaml:"enums,omitempty"`
	Instances InstancesSpec  `yaml:"instances,omitempty"`
	Doc       string         `yaml:"doc,omitempty"`
	DocRef    string         `yaml:"doc-ref,omitempty"`
}

// TypesSpec represents a list of KaitaiStruct types.
type TypesSpec []TypeSpec

// MarshalYAML implements yaml.Marshaler
func (t TypesSpec) MarshalYAML() (interface{}, error) {
	fields := []reflect.StructField{}
	for i, n := range t {
		fields = append(fields, reflect.StructField{
			Name: "Field" + strconv.Itoa(i),
			Type: reflect.TypeOf(TypeSpec{}),
			Tag:  reflect.StructTag(fmt.Sprintf("yaml:%q", n.Meta.ID)),
		})
	}
	v := reflect.New(reflect.StructOf(fields)).Elem()
	for i, n := range t {
		v.Field(i).Set(reflect.ValueOf(n))
	}
	return v.Interface(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler
func (t *TypesSpec) UnmarshalYAML(node *yaml.Node) error {
	*t = []TypeSpec{}
	return yamlMapForEach(node, func(key *yaml.Node, value *yaml.Node) error {
		var item TypeSpec
		var id string
		if err := key.Decode(&id); err != nil {
			return err
		}
		if err := value.Decode(&item); err != nil {
			return err
		}
		item.Meta.ID = Identifier(id)
		*t = append(*t, item)
		return nil
	})
}
