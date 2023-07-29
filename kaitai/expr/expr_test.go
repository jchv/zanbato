package expr

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func MustParseBigFloat(s string) *big.Float {
	fv := big.NewFloat(0)
	_, _, err := fv.Parse(s, 10)
	if err != nil {
		panic(err)
	}
	return fv
}

func TestParseExpr(t *testing.T) {
	tests := []struct {
		Source string
		Expr   *Expr
	}{
		{
			Source: "",
			Expr:   nil,
		},
		{
			Source: "test",
			Expr:   &Expr{Root: IdentNode{Identifier: "test"}},
		},
		{
			Source: "1",
			Expr:   &Expr{Root: IntNode{Integer: big.NewInt(1)}},
		},
		{
			Source: "1.0",
			Expr:   &Expr{Root: FloatNode{Float: MustParseBigFloat("1.0")}},
		},
	}

	for _, test := range tests {
		expr, err := ParseExpr(test.Source)
		assert.NoError(t, err)
		assert.Equal(t, test.Expr, expr)
	}
}
