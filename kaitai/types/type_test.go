package types

import (
	"math/big"
	"testing"

	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTypeRefUserParamsWithQuotedCommas(t *testing.T) {
	tests := []struct {
		name       string
		typeSpec   string
		wantName   string
		wantParams []expr.Node
	}{
		{
			name:       "empty parameter list",
			typeSpec:   `child()`,
			wantName:   "child",
			wantParams: nil,
		},
		{
			name:     "double quoted comma",
			typeSpec: `child("a,b", 3)`,
			wantName: "child",
			wantParams: []expr.Node{
				expr.StringNode{Str: "a,b"},
				expr.IntNode{Integer: big.NewInt(3)},
			},
		},
		{
			name:     "single quoted comma",
			typeSpec: `child('a,b', 3)`,
			wantName: "child",
			wantParams: []expr.Node{
				expr.StringNode{Str: "a,b"},
				expr.IntNode{Integer: big.NewInt(3)},
			},
		},
		{
			name:     "escaped quote before comma",
			typeSpec: `child("a\",b", 3)`,
			wantName: "child",
			wantParams: []expr.Node{
				expr.StringNode{Str: `a",b`},
				expr.IntNode{Integer: big.NewInt(3)},
			},
		},
		{
			name:     "commas in nested expression and string",
			typeSpec: `child([1, 2, 3], "x,y")`,
			wantName: "child",
			wantParams: []expr.Node{
				expr.ArrayNode{Items: []expr.Node{
					expr.IntNode{Integer: big.NewInt(1)},
					expr.IntNode{Integer: big.NewInt(2)},
					expr.IntNode{Integer: big.NewInt(3)},
				}},
				expr.StringNode{Str: "x,y"},
			},
		},
		{
			name:     "parentheses inside string",
			typeSpec: `child("a),b", 3)`,
			wantName: "child",
			wantParams: []expr.Node{
				expr.StringNode{Str: "a),b"},
				expr.IntNode{Integer: big.NewInt(3)},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseTypeRef(test.typeSpec)
			require.NoError(t, err)
			require.NotNil(t, got.User)
			assert.Equal(t, User, got.Kind)
			assert.Equal(t, test.wantName, got.User.Name)
			require.Len(t, got.User.Params, len(test.wantParams))

			for i, want := range test.wantParams {
				assert.Equal(t, want, got.User.Params[i].Root)
			}
		})
	}
}
