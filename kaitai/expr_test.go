package kaitai

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func MustParseBigFloat(s string) big.Float {
	var fv big.Float

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
			Expr:   &Expr{Root: IdentNode{Value: "test"}},
		},
		{
			Source: "1",
			Expr:   &Expr{Root: IntNode{Value: *big.NewInt(1)}},
		},
		{
			Source: "1.0",
			Expr:   &Expr{Root: FloatNode{Value: MustParseBigFloat("1.0")}},
		},
	}

	for _, test := range tests {
		expr, err := ParseExpr(test.Source)
		assert.NoError(t, err)
		assert.Equal(t, test.Expr, expr)
	}
}
