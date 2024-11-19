package expr

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

//go:generate go run golang.org/x/tools/cmd/stringer -type=UnaryOp,BinaryOp -output expr_string.go

type UnaryOp int

const (
	InvalidUnaryOp UnaryOp = iota

	OpLogicalNot
)

type BinaryOp int

const (
	InvalidBinaryOp BinaryOp = iota

	OpAdd
	OpSub
	OpMult
	OpDiv
	OpMod
	OpLessThan
	OpLessThanEqual
	OpGreaterThan
	OpGreaterThanEqual
	OpEqual
	OpNotEqual
	OpShiftLeft
	OpShiftRight
	OpBitAnd
	OpBitOr
	OpBitXor
	OpLogicalAnd
	OpLogicalOr
)

// Expr contains a parsed expression.
type Expr struct {
	Root Node
}

// Node is a node in the AST.
type Node interface {
	fmt.Stringer
	isnode()
}

// IdentNode is an identifier.
type IdentNode struct{ Identifier string }

func (IdentNode) isnode() {}

func (i IdentNode) String() string { return i.Identifier }

// StringNode is a string literal.
type StringNode struct{ Str string }

func (StringNode) isnode() {}

func (s StringNode) String() string { return fmt.Sprintf("%q", s.Str) }

// IntNode is an integer literal.
type IntNode struct{ Integer *big.Int }

func (IntNode) isnode() {}

func (i IntNode) String() string { return i.Integer.String() }

// FloatNode is a floating point literal.
type FloatNode struct{ Float *big.Float }

func (FloatNode) isnode() {}

func (f FloatNode) String() string { return f.Float.String() }

// BoolNode is a boolean literal.
type BoolNode struct{ Bool bool }

func (BoolNode) isnode() {}

func (b BoolNode) String() string {
	if b.Bool {
		return "true"
	} else {
		return "false"
	}
}

// UnaryNode is a unary operation
type UnaryNode struct {
	Operand Node
	Op      UnaryOp
}

func (UnaryNode) isnode() {}

func (u UnaryNode) String() string {
	switch u.Op {
	case OpLogicalNot:
		return "not (" + u.Operand.String() + ")"
	default:
		return "!" + u.Op.String() + " (" + u.Operand.String() + ")"
	}
}

// BinaryNode is a binary operation
type BinaryNode struct {
	A, B Node
	Op   BinaryOp
}

func (BinaryNode) isnode() {}

func (b BinaryNode) String() string {
	switch b.Op {
	case OpAdd:
		return "(" + b.A.String() + ") + (" + b.B.String() + ")"
	case OpSub:
		return "(" + b.A.String() + ") - (" + b.B.String() + ")"
	case OpMult:
		return "(" + b.A.String() + ") * (" + b.B.String() + ")"
	case OpDiv:
		return "(" + b.A.String() + ") / (" + b.B.String() + ")"
	case OpMod:
		return "(" + b.A.String() + ") % (" + b.B.String() + ")"
	case OpLessThan:
		return "(" + b.A.String() + ") < (" + b.B.String() + ")"
	case OpLessThanEqual:
		return "(" + b.A.String() + ") <= (" + b.B.String() + ")"
	case OpGreaterThan:
		return "(" + b.A.String() + ") > (" + b.B.String() + ")"
	case OpGreaterThanEqual:
		return "(" + b.A.String() + ") >= (" + b.B.String() + ")"
	case OpEqual:
		return "(" + b.A.String() + ") == (" + b.B.String() + ")"
	case OpNotEqual:
		return "(" + b.A.String() + ") != (" + b.B.String() + ")"
	case OpShiftLeft:
		return "(" + b.A.String() + ") << (" + b.B.String() + ")"
	case OpShiftRight:
		return "(" + b.A.String() + ") >> (" + b.B.String() + ")"
	case OpBitAnd:
		return "(" + b.A.String() + ") & (" + b.B.String() + ")"
	case OpBitOr:
		return "(" + b.A.String() + ") | (" + b.B.String() + ")"
	case OpBitXor:
		return "(" + b.A.String() + ") ^ (" + b.B.String() + ")"
	case OpLogicalAnd:
		return "(" + b.A.String() + ") and (" + b.B.String() + ")"
	case OpLogicalOr:
		return "(" + b.A.String() + ") or (" + b.B.String() + ")"
	default:
		return "(" + b.A.String() + ") !" + b.Op.String() + " (" + b.B.String() + ")"
	}
}

// TernaryNode is a ternary operation
type TernaryNode struct {
	A, B, C Node
}

func (TernaryNode) isnode() {}

func (t TernaryNode) String() string {
	return "(" + t.A.String() + ") ? (" + t.B.String() + ") : (" + t.C.String() + ")"
}

// ScopeNode is a scope access expression (a::b)
type ScopeNode struct {
	Operand Node
	Type    string
}

func (ScopeNode) isnode() {}

func (s ScopeNode) String() string {
	return s.Operand.String() + "::" + s.Type
}

// MemberNode is a member access expression (a.b)
type MemberNode struct {
	Operand  Node
	Property string
}

func (MemberNode) isnode() {}

func (m MemberNode) String() string {
	return m.Operand.String() + "." + m.Property
}

// SubscriptNode is a subscript expression (a[b])
type SubscriptNode struct {
	A, B Node
}

func (SubscriptNode) isnode() {}

func (m SubscriptNode) String() string {
	return m.A.String() + "[" + m.B.String() + "]"
}

// ParseExpr parses an expression into an AST.
func ParseExpr(src string) (result *Expr, err error) {
	p := parser{[]rune(src), 0}
	defer func() {
		if r := recover(); r != nil {
			if rErr, ok := r.(error); ok {
				err = fmt.Errorf("error parsing expression at character %d: %w", p.pos+1, rErr)
				result = nil
			} else {
				panic(r)
			}
		} else {
			if len(p.s) > 0 {
				err = fmt.Errorf("unparsed expression text: %q", string(p.s))
			}
		}
	}()
	if src == "" {
		return nil, nil
	}
	return &Expr{Root: p.expr(0)}, nil
}

// MustParseExpr parses an expression, and panics if an error occurs.
func MustParseExpr(src string) *Expr {
	expr, err := ParseExpr(src)
	if err != nil {
		panic(fmt.Errorf("error in expression %q: %w", src, err))
	}
	return expr
}

// # Lexing
func lower(ch rune) rune        { return ('a' - 'A') | ch }
func isdecimal(ch rune) bool    { return '0' <= ch && ch <= '9' }
func ishex(ch rune) bool        { return '0' <= ch && ch <= '9' || 'a' <= lower(ch) && lower(ch) <= 'f' }
func isasciiletter(c rune) bool { return 'a' <= lower(c) && lower(c) <= 'z' }
func isletter(c rune) bool      { return isasciiletter(c) || c >= utf8.RuneSelf && unicode.IsLetter(c) }
func isdigit(c rune) bool       { return isdecimal(c) || c >= utf8.RuneSelf && unicode.IsDigit(c) }
func isnumber(c rune) bool      { return isdigit(c) || ishex(c) || c == '.' || lower(c) == 'x' }
func isidentstart(c rune) bool  { return isletter(c) || c == '_' }
func isident(c rune) bool       { return isidentstart(c) || isdigit(c) }
func iswhitespace(c rune) bool  { return c == ' ' || c == '\t' }

type parser struct {
	s   []rune
	pos int
}

func (p *parser) peek() rune {
	if len(p.s) > 0 {
		return p.s[0]
	}
	return 0
}

func (p *parser) peek2() rune {
	if len(p.s) > 1 {
		return p.s[1]
	}
	return 0
}

func (p *parser) advance(n int) {
	p.s = p.s[n:]
	p.pos += n
}

func (p *parser) next() rune {
	if len(p.s) != 0 {
		v := p.s[0]
		p.advance(1)
		return v
	}
	return 0
}

func (p *parser) skipwhitespace() {
	for iswhitespace(p.peek()) {
		p.advance(1)
	}
}

func (p *parser) token(test func(rune) bool) string {
	var token string
	for len(p.s) != 0 && test(p.s[0]) {
		token += string(p.s[0])
		p.advance(1)
	}
	return token
}

func (p *parser) strescape(quote rune) []byte {
	c := p.s[0]
	p.advance(1)
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
		p.advance(2)
		code, err := strconv.ParseUint(octal, 8, 8)
		if err != nil {
			panic(err)
		}
		return []byte{byte(code)}
	case 'x':
		hex := string(p.s[0]) + string(p.s[1])
		p.advance(2)
		code, err := strconv.ParseUint(hex, 16, 8)
		if err != nil {
			panic(err)
		}
		return []byte{byte(code)}
	case 'u':
		hex := string(p.s[0]) + string(p.s[1]) + string(p.s[2]) + string(p.s[3])
		p.advance(4)
		code, err := strconv.ParseUint(hex, 16, 16)
		if err != nil {
			panic(err)
		}
		return []byte(string(rune(code)))
	case 'U':
		hex := string(p.s[0]) + string(p.s[1]) + string(p.s[2]) + string(p.s[3]) + string(p.s[4]) + string(p.s[5]) + string(p.s[6]) + string(p.s[7])
		p.advance(8)
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
func (p *parser) number() Node {
	token := p.token(isnumber)
	if strings.Contains(token, ".") {
		f := FloatNode{Float: big.NewFloat(0)}
		_, _, err := f.Float.Parse(token, 0)
		if err != nil {
			panic(err)
		}
		return f
	}
	i := IntNode{Integer: big.NewInt(0)}
	_, ok := i.Integer.SetString(token, 0)
	if !ok {
		panic(errors.New("invalid integer"))
	}
	return i
}

func (p *parser) strlit() Node {
	quote := p.s[0]
	p.advance(1)
	str := []byte{}

	for {
		c := p.s[0]
		p.advance(1)
		switch c {
		case quote:
			return StringNode{string(str)}
		case '\\':
			str = append(str, p.strescape(quote)...)
		default:
			str = append(str, string(c)...)
		}
	}
}

const (
	depthTernaryExpr = iota
	depthOrExpr
	depthAndExpr
	depthCompareExpr
	depthAddExpr
	depthMultExpr
	depthMemberExpr
	depthPrimaryExpr
)

func (p *parser) expr(depth int) Node {
	// This expression parser always starts at a primary expression, then loops
	// over possible operations until it fails to match one that is at a depth
	// of greater or equal depth than the depth passed in. Higher precedence
	// operators have higher depth, so expr(depthPrimaryExpr) will only ever
	// return a primary expression, whereas expr(0) will continue applying
	// operations until none match.
	// The value of depth checks should decrease over time as we go to lower
	// precedence operators.
	// This function calls itself recursively to parse sub-expressions with
	// potentially higher depth values.
	var n Node
	p.skipwhitespace()
	c := p.peek()
	switch {
	case isidentstart(c):
		tok := p.token(isident)
		switch tok {
		case "not":
			n = UnaryNode{Op: OpLogicalNot, Operand: p.expr(depthPrimaryExpr)}
		case "true":
			n = BoolNode{Bool: true}
		case "false":
			n = BoolNode{Bool: false}
		default:
			n = IdentNode{Identifier: tok}
		}
	case isnumber(c):
		n = p.number()
	case c == '"' || c == '\'':
		n = p.strlit()
	case c == '(':
		p.next()
		n = p.expr(0)
		if p.next() != ')' {
			panic(fmt.Errorf("expected ')'"))
		}
	}
	if depth >= depthPrimaryExpr {
		return n
	}
	for {
		p.skipwhitespace()
		switch p.peek() {
		case ':':
			// Could be ternary.
			if p.peek2() != ':' {
				break
			}
			p.advance(2)
			n = ScopeNode{Operand: n, Type: p.token(isident)}
			continue
		case '.':
			p.next()
			n = MemberNode{Operand: n, Property: p.token(isident)}
			continue
		case '[':
			p.next()
			n = SubscriptNode{A: n, B: p.expr(0)}
			if p.next() != ']' {
				panic(fmt.Errorf("expected ']'"))
			}
			continue
		case 0:
			return n
		}
		if depth >= depthMemberExpr {
			break
		}
		switch p.peek() {
		case '*':
			p.next()
			n = BinaryNode{Op: OpMult, A: n, B: p.expr(depthMemberExpr)}
			continue
		case '/':
			p.next()
			n = BinaryNode{Op: OpDiv, A: n, B: p.expr(depthMemberExpr)}
			continue
		case '%':
			p.next()
			n = BinaryNode{Op: OpMod, A: n, B: p.expr(depthMemberExpr)}
			continue
		case '>':
			if p.peek2() != '>' {
				break
			}
			p.next()
			p.next()
			n = BinaryNode{Op: OpShiftRight, A: n, B: p.expr(depthMemberExpr)}
			continue
		case '<':
			if p.peek2() != '<' {
				break
			}
			p.next()
			p.next()
			n = BinaryNode{Op: OpShiftLeft, A: n, B: p.expr(depthMemberExpr)}
			continue
		case '&':
			p.next()
			n = BinaryNode{Op: OpBitAnd, A: n, B: p.expr(depthMemberExpr)}
			continue
		}
		if depth >= depthMultExpr {
			break
		}
		switch p.peek() {
		case '+':
			p.next()
			n = BinaryNode{Op: OpAdd, A: n, B: p.expr(depthMultExpr)}
			continue
		case '-':
			p.next()
			n = BinaryNode{Op: OpSub, A: n, B: p.expr(depthMultExpr)}
			continue
		case '|':
			p.next()
			n = BinaryNode{Op: OpBitOr, A: n, B: p.expr(depthMultExpr)}
			continue
		case '^':
			p.next()
			n = BinaryNode{Op: OpBitXor, A: n, B: p.expr(depthMultExpr)}
			continue
		}
		if depth >= depthAddExpr {
			break
		}
		switch p.peek() {
		case '=':
			if p.peek2() != '=' {
				panic(fmt.Errorf("expected '=', got %q", p.peek2()))
			}
			p.advance(2)
			n = BinaryNode{Op: OpEqual, A: n, B: p.expr(depthAddExpr)}
			continue
		case '!':
			if p.peek2() != '=' {
				panic(fmt.Errorf("expected '=', got %q", p.peek2()))
			}
			p.advance(2)
			n = BinaryNode{Op: OpNotEqual, A: n, B: p.expr(depthAddExpr)}
			continue
		case '<':
			p.next()
			if p.peek() == '=' {
				p.next()
				n = BinaryNode{Op: OpLessThanEqual, A: n, B: p.expr(depthAddExpr)}
				continue
			} else {
				n = BinaryNode{Op: OpLessThan, A: n, B: p.expr(depthAddExpr)}
				continue
			}
		case '>':
			p.next()
			if p.peek() == '=' {
				p.next()
				n = BinaryNode{Op: OpGreaterThanEqual, A: n, B: p.expr(depthAddExpr)}
				continue
			} else {
				n = BinaryNode{Op: OpGreaterThan, A: n, B: p.expr(depthAddExpr)}
				continue
			}
		}
		if depth >= depthCompareExpr {
			break
		}
		switch p.peek() {
		case 'a':
			p.next()
			if p.peek() != 'n' && p.peek2() != 'd' {
				panic(fmt.Errorf("expected 'and'"))
			}
			p.advance(2)
			n = BinaryNode{Op: OpLogicalAnd, A: n, B: p.expr(depthCompareExpr)}
			continue
		}
		if depth >= depthAndExpr {
			break
		}
		switch p.peek() {
		case 'o':
			p.next()
			if p.next() != 'r' {
				panic(fmt.Errorf("expected 'or'"))
			}
			n = BinaryNode{Op: OpLogicalOr, A: n, B: p.expr(depthAndExpr)}
			continue
		}
		if depth >= depthOrExpr {
			break
		}
		switch p.peek() {
		case '?':
			p.next()
			a := n
			b := p.expr(0)
			if p.next() != ':' {
				panic(fmt.Errorf("expected ':'"))
			}
			c := p.expr(0)
			n = TernaryNode{a, b, c}
		}
		if depth >= depthTernaryExpr {
			break
		}
		break
	}
	return n
}
