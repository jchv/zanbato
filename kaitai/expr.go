package kaitai

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Expr contains a parsed expression.
type Expr struct {
	Root Node
}

// Node is a node in the AST.
type Node interface {
	isnode()
	Type(ref *Struct) (Type, bool)
}

// IdentNode is an identifier.
type IdentNode struct{ Value string }

func (IdentNode) isnode() {}

func (i IdentNode) Type(ref *Struct) (Type, bool) {
	for _, param := range ref.Params {
		if param.ID == Identifier(i.Value) {
			typeref := param.Type
			return Type{TypeRef: &typeref}, true
		}
	}
	for _, attr := range ref.Seq {
		if attr.ID == Identifier(i.Value) {
			return attr.Type, true
		}
	}
	return Type{}, false
}

// StrNode is a string literal.
type StrNode struct{ Value string }

func (StrNode) isnode() {}

func (StrNode) Type(ref *Struct) (Type, bool) {
	return Type{
		TypeRef: &TypeRef{
			Kind:   String,
			String: &StringType{},
		},
	}, true
}

// IntNode is an integer literal.
type IntNode struct{ Value big.Int }

func (IntNode) isnode() {}

func (IntNode) Type(ref *Struct) (Type, bool) {
	return Type{
		TypeRef: &TypeRef{
			Kind: UntypedInt,
		},
	}, true
}

// FloatNode is a floating point literal.
type FloatNode struct{ Value big.Float }

func (FloatNode) isnode() {}

func (FloatNode) Type(ref *Struct) (Type, bool) {
	return Type{
		TypeRef: &TypeRef{
			Kind: UntypedInt,
		},
	}, true
}

// MemberNode is a member access expression
type MemberNode struct {
	Operand  Node
	Property string
}

func (MemberNode) isnode() {}

func (m MemberNode) Type(ref *Struct) (Type, bool) {
	// TODO: handle symbol resolution
	return Type{}, false
}

// ParseExpr parses an expression into an AST.
func ParseExpr(src string) (*Expr, error) {
	if src == "" {
		return nil, nil
	}
	p := parser{[]rune(src)}
	return &Expr{Root: p.expr(0)}, nil
}

// MustParseExpr parses an expression, and panics if an error occurs.
func MustParseExpr(src string) *Expr {
	expr, err := ParseExpr(src)
	if err != nil {
		panic(err)
	}
	return expr
}

func (e *Expr) Type(ref *Struct) (Type, bool) {
	return e.Root.Type(ref)
}

// # Lexing
func lower(ch rune) rune        { return ('a' - 'A') | ch }
func isdecimal(ch rune) bool    { return '0' <= ch && ch <= '9' }
func isoctal(ch rune) bool      { return '0' <= ch && ch <= '7' }
func ishex(ch rune) bool        { return '0' <= ch && ch <= '9' || 'a' <= lower(ch) && lower(ch) <= 'f' }
func isasciiletter(c rune) bool { return 'a' <= lower(c) && lower(c) <= 'z' }
func isletter(c rune) bool      { return isasciiletter(c) || c >= utf8.RuneSelf && unicode.IsLetter(c) }
func isdigit(c rune) bool       { return isdecimal(c) || c >= utf8.RuneSelf && unicode.IsDigit(c) }
func isnumber(c rune) bool      { return isdigit(c) || ishex(c) || c == '.' || lower(c) == 'x' }
func isidentstart(c rune) bool  { return isletter(c) || c == '_' }
func isident(c rune) bool       { return isidentstart(c) || isdigit(c) }
func iswhitespace(c rune) bool  { return c == ' ' || c == '\t' }

type parser struct {
	s []rune
}

func (p *parser) skipwhitespace() []rune {
	for iswhitespace(p.s[0]) {
		p.s = p.s[1:]
	}
	return p.s
}

func (p *parser) token(test func(rune) bool) string {
	var token string
	for len(p.s) != 0 && test(p.s[0]) {
		token += string(p.s[0])
		p.s = p.s[1:]
	}
	return token
}

func (p *parser) strescape(quote rune) []byte {
	c := p.s[0]
	p.s = p.s[1:]
	switch c {
	case 'a':
		return []byte{'\a'}
	case 'b':
		return []byte{'\b'}
	case 'f':
		return []byte{'\f'}
	case 'n':
		return []byte{'\n'}
	case 'r':
		return []byte{'\r'}
	case 't':
		return []byte{'\t'}
	case 'v':
		return []byte{'\v'}
	case '\\':
		return []byte{'\\'}
	case rune(quote):
		return []byte(string(quote))
	case '0', '1', '2', '3', '4', '5', '6', '7':
		octal := string(c) + string(p.s[0]) + string(p.s[1])
		p.s = p.s[2:]
		code, err := strconv.ParseUint(octal, 8, 8)
		if err != nil {
			panic(err)
		}
		return []byte{byte(code)}
	case 'x':
		hex := string(p.s[0]) + string(p.s[1])
		p.s = p.s[2:]
		code, err := strconv.ParseUint(hex, 16, 8)
		if err != nil {
			panic(err)
		}
		return []byte{byte(code)}
	case 'u':
		hex := string(p.s[0]) + string(p.s[1]) + string(p.s[2]) + string(p.s[3])
		p.s = p.s[4:]
		code, err := strconv.ParseUint(hex, 16, 16)
		if err != nil {
			panic(err)
		}
		return []byte(string(rune(code)))
	case 'U':
		hex := string(p.s[0]) + string(p.s[1]) + string(p.s[2]) + string(p.s[3]) + string(p.s[4]) + string(p.s[5]) + string(p.s[6]) + string(p.s[7])
		p.s = p.s[8:]
		code, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			panic(err)
		}
		return []byte(string(rune(code)))
	default:
		panic(fmt.Errorf("unexpected escape code %c", c))
	}
}

// # Parsing
func (p *parser) ident() Node {
	token := p.token(isident)
	return IdentNode{token}
}

func (p *parser) number() Node {
	token := p.token(isnumber)
	if strings.Contains(token, ".") {
		f := FloatNode{}
		_, _, err := f.Value.Parse(token, 0)
		if err != nil {
			panic(err)
		}
		return f
	}
	i := IntNode{}
	_, ok := i.Value.SetString(token, 0)
	if !ok {
		panic(errors.New("invalid integer"))
	}
	return i
}

func (p *parser) strlit() Node {
	quote := p.s[0]
	p.s = p.s[1:]
	str := []byte{}

	for {
		c := p.s[0]
		p.s = p.s[1:]
		switch c {
		case quote:
			return StrNode{string(str)}
		case '\\':
			str = append(str, p.strescape(quote)...)
		default:
			str = append(str, string(c)...)
		}
	}
}

const (
	depthPrimaryExpr = 0
	depthMemberExpr  = 1
)

func (p *parser) expr(depth int) Node {
	var n Node
	p.skipwhitespace()
	switch {
	case isidentstart(p.s[0]):
		n = p.ident()
	case isnumber(p.s[0]):
		n = p.number()
	case p.s[0] == '"':
		n = p.strlit()
	case p.s[0] == '(':
		n = p.expr(1)
	}
	if depth >= depthPrimaryExpr {
		return n
	}
	for {
		p.skipwhitespace()
		if p.s[0] == '.' {
			p.s = p.s[1:]
			n = MemberNode{Operand: n, Property: p.token(isident)}
			continue
		}
		if depth >= depthMemberExpr {
			break
		}
		// TODO: Implement operators here.
		break
	}
	return n
}
