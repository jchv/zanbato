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
		{
			Source: `'ASCII\\x'`,
			Expr:   &Expr{Root: StringNode{Str: `ASCII\\x`}},
		},
		{
			Source: "1 == 1 ? 2 : 3",
			Expr: &Expr{Root: TernaryNode{
				A: BinaryNode{
					Op: OpEqual,
					A:  IntNode{Integer: big.NewInt(1)},
					B:  IntNode{Integer: big.NewInt(1)},
				},
				B: IntNode{Integer: big.NewInt(2)},
				C: IntNode{Integer: big.NewInt(3)},
			}},
		},
	}

	for _, test := range tests {
		expr, err := ParseExpr(test.Source)
		assert.NoError(t, err)
		assert.Equal(t, test.Expr, expr)
	}
}

func TestParseExprPrecedence(t *testing.T) {
	// Since the AST String method returns parenthesized expressions, it is
	// possible to test precedence just using the String method.
	tests := []struct {
		Source string
		Want   string
	}{
		{"1 + 2 << 3", "((1) + (2)) << (3)"},
		{"1 << 2 + 3", "(1) << ((2) + (3))"},
		{"1 | 2 ^ 3", "(1) | ((2) ^ (3))"},
		{"1 ^ 2 | 3", "((1) ^ (2)) | (3)"},
		{"0xff & 0x80 | 0x10", "((255) & (128)) | (16)"},
		{"0x10 & 0xff << 4", "(16) & ((255) << (4))"},
		{"a == b | c", "(a) == ((b) | (c))"},
		{"a + b * c", "(a) + ((b) * (c))"},
		{"a - b - c", "((a) - (b)) - (c)"},
		{"a << b << c", "((a) << (b)) << (c)"},
		{"-a.foo", "-(a.foo)"},
		{"-a.b.c", "-(a.b.c)"},
		{"-a * b", "(-(a)) * (b)"},
		{"-a + b.c", "(-(a)) + (b.c)"},
	}
	for _, tc := range tests {
		t.Run(tc.Source, func(t *testing.T) {
			expr, err := ParseExpr(tc.Source)
			assert.NoError(t, err)
			assert.Equal(t, tc.Want, expr.Root.String())
		})
	}
}

func TestParseExprError(t *testing.T) {
	tests := []struct {
		Source      string
		ErrContains string
	}{
		{
			Source:      "(x + 1, 1.0)",
			ErrContains: "expected ')'",
		},
		{
			Source:      `"unterminated`,
			ErrContains: "unterminated string literal",
		},
		{
			Source:      `"\x"`,
			ErrContains: "short \\x escape: expected 2 hex digits",
		},
		{
			Source:      "1 anx 2",
			ErrContains: "unparsed expression text",
		},
		{
			Source:      "1 and2",
			ErrContains: "unparsed expression text",
		},
		{
			Source:      "1 or2",
			ErrContains: "unparsed expression text",
		},
	}

	for _, test := range tests {
		t.Run(test.Source, func(t *testing.T) {
			_, err := ParseExpr(test.Source)
			assert.ErrorContains(t, err, test.ErrContains)
		})
	}
}
