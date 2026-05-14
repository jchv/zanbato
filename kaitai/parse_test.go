package kaitai

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/types"
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
			Source: `{meta: {id: enums}, enums: {enum_a: {1: value1, 2: value2}, enum_b: {0x01: b0, 0x02: b1}}}`,
			Struct: &Struct{
				ID: "enums",
				Enums: []*Enum{
					{ID: "enum_a", Values: []EnumValue{{big.NewInt(1), "value1"}, {big.NewInt(2), "value2"}}},
					{ID: "enum_b", Values: []EnumValue{{big.NewInt(1), "b0"}, {big.NewInt(2), "b1"}}},
				},
			},
		},
		{
			Name:   "Attrs",
			Source: `{meta: {id: attrs}, seq: [{id: magic, size: 4, contents: [0x7f, "ELF"]}]}`,
			Struct: &Struct{
				ID: "attrs",
				Seq: []*Attr{
					{
						ID: "magic",
						Type: types.Type{
							TypeRef: &types.TypeRef{
								Kind: types.Bytes,
								Bytes: &types.BytesType{
									Consume:    true,
									EosError:   true,
									PadRight:   -1,
									Terminator: -1,
									Size:       &expr.Expr{Root: expr.IntNode{Integer: big.NewInt(4)}},
								},
							},
						},
						Contents: []byte{0x7f, 'E', 'L', 'F'},
						Size:     &expr.Expr{Root: expr.IntNode{Integer: big.NewInt(4)}},
					},
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

func TestParseInvalidExpressions(t *testing.T) {
	tests := []struct {
		Name        string
		Source      string
		ErrContains string
	}{
		{
			Name:        "IfExpression",
			Source:      `{meta: {id: invalid_if}, seq: [{id: x, type: u1, if: "1 anx 2"}]}`,
			ErrContains: `parsing if expression for attr "x"`,
		},
		{
			Name:        "RepeatExpression",
			Source:      `{meta: {id: invalid_repeat}, seq: [{id: x, type: u1, repeat: expr, repeat-expr: "1 or2"}]}`,
			ErrContains: `parsing repeat expression for attr "x"`,
		},
		{
			Name:        "UnterminatedString",
			Source:      `{meta: {id: invalid_string}, seq: [{id: x, type: u1, if: "\"unterminated"}]}`,
			ErrContains: "unterminated string literal",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			_, err := ParseStruct(bytes.NewBufferString(test.Source))
			assert.ErrorContains(t, err, test.ErrContains)
		})
	}
}
