package kaitai

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	tests := []struct {
		Name   string
		Source string
		Struct *Struct
	}{
		{
			Name:   "EmptyStruct",
			Source: `{meta: {id: empty}}`,
			Struct: &Struct{ID: "empty"},
		},
		{
			Name:   "NestedEmpty",
			Source: `{meta: {id: nested_empty}, types: {subtype_a: {doc: Nested A}, subtype_b: {doc: Nested B}}}`,
			Struct: &Struct{
				ID: "nested_empty",
				Structs: []*Struct{
					{ID: "subtype_a", Doc: "Nested A"},
					{ID: "subtype_b", Doc: "Nested B"},
				},
			},
		},
		{
			Name:   "Enums",
			Source: `{meta: {id: enums}, enums: {enum_a: {0: value1, 1: value2}, enum_b: {0x00: b0, 0x01: b1}}}`,
			Struct: &Struct{
				ID: "enums",
				Enums: []*Enum{
					{ID: "enum_a", Values: []EnumValue{{big.NewInt(0), "value1"}, {big.NewInt(1), "value2"}}},
					{ID: "enum_b", Values: []EnumValue{{big.NewInt(0), "b0"}, {big.NewInt(1), "b1"}}},
				},
			},
		},
		{
			Name:   "Attrs",
			Source: `{meta: {id: attrs}, seq: [{id: magic, size: 4, contents: [0x7f, "ELF"]}]}`,
			Struct: &Struct{
				ID: "attrs",
				Seq: []*Attr{
					{ID: "magic", Type: Type{TypeRef: &TypeRef{Kind: Bytes, Bytes: &BytesType{Consume: true, EosError: true, Size: &Expr{IntNode{*big.NewInt(4)}}}}}, Contents: []byte{0x7f, 'E', 'L', 'F'}},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			resultStruct, err := ParseStruct(bytes.NewBufferString(test.Source))
			assert.NoError(t, err)
			assert.Equal(t, test.Struct, resultStruct)
		})
	}
}
