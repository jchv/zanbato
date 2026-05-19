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
	OpNegate // Unary minus: -x
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

// FStringNode is a formatted string (f-string) with interpolated expressions.
// Parts alternate between literal string segments and embedded expressions.
type FStringNode struct {
	Parts []FStringPart
}

// FStringPart is either a literal string segment or an expression in an f-string.
type FStringPart struct {
	Literal string // non-empty for literal segments
	Expr    Node   // non-nil for expression segments
}

func (FStringNode) isnode() {}

func (f FStringNode) String() string {
	b := strings.Builder{}
	b.WriteString("f\"")
	for _, p := range f.Parts {
		if p.Expr != nil {
			b.WriteString("{")
			b.WriteString(p.Expr.String())
			b.WriteString("}")
		} else {
			b.WriteString(p.Literal)
		}
	}
	b.WriteString("\"")
	return b.String()
}

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

// ArrayNode is an array literal.
type ArrayNode struct{ Items []Node }

func (ArrayNode) isnode() {}

func (a ArrayNode) String() string {
	b := strings.Builder{}
	b.WriteByte('[')
	for i := range a.Items {
		if i != 0 {
			b.WriteString(", ")
		}
		b.WriteString(a.Items[i].String())
	}
	b.WriteByte(']')
	return b.String()
}

// CallNode is a method/function call (e.g., x.method(args))
type CallNode struct {
	Object Node   // The object being called on (MemberNode or IdentNode)
	Args   []Node // Arguments
}

func (CallNode) isnode() {}

func (c CallNode) String() string {
	b := strings.Builder{}
	b.WriteString(c.Object.String())
	b.WriteByte('(')
	for i, arg := range c.Args {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(arg.String())
	}
	b.WriteByte(')')
	return b.String()
}

// CastNode is a type cast expression (e.g., x.as<type>)
type CastNode struct {
	Operand  Node   // The expression being cast
	TypeName string // The target type name
}

func (CastNode) isnode() {}

func (c CastNode) String() string {
	return c.Operand.String() + ".as<" + c.TypeName + ">"
}

// SizeofNode is a sizeof expression (e.g., sizeof<type>)
type SizeofNode struct {
	TypeName string
}

func (SizeofNode) isnode() {}

func (s SizeofNode) String() string {
	return "sizeof<" + s.TypeName + ">"
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
	case OpNegate:
		return "-(" + u.Operand.String() + ")"
	default:
		return u.Op.String() + " (" + u.Operand.String() + ")"
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
			} else {
				err = fmt.Errorf("error parsing expression at character %d: %v", p.pos+1, r)
			}
			result = nil
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
func isnumber(c rune) bool {
	return isdigit(c) || c == '_'
}
func isidentstart(c rune) bool { return isletter(c) || c == '_' }
func isident(c rune) bool      { return isidentstart(c) || isdigit(c) }
func iswhitespace(c rune) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

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

func (p *parser) consumeKeyword(keyword string) bool {
	runes := []rune(keyword)
	if len(p.s) < len(runes) {
		return false
	}
	for i, r := range runes {
		if p.s[i] != r {
			return false
		}
	}
	if len(p.s) > len(runes) && isident(p.s[len(runes)]) {
		return false
	}
	p.advance(len(runes))
	return true
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
	if len(p.s) == 0 {
		panic(fmt.Errorf("unterminated escape sequence"))
	}
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
	case 'e':
		return []byte{0x1b} // escape character
	case '\\':
		return []byte{'\\'}
	case rune(quote):
		return []byte(string(quote))
	case '"':
		return []byte{'"'}
	case '\'':
		return []byte{'\''}
	case '0', '1', '2', '3', '4', '5', '6', '7':
		// Octal escape: 1-3 digits
		octal := string(c)
		for i := 0; i < 2 && len(p.s) > 0 && p.s[0] >= '0' && p.s[0] <= '7'; i++ {
			octal += string(p.s[0])
			p.advance(1)
		}
		code, err := strconv.ParseUint(octal, 8, 8)
		if err != nil {
			panic(err)
		}
		return []byte{byte(code)}
	case 'x':
		if len(p.s) < 2 {
			panic(fmt.Errorf("short \\x escape: expected 2 hex digits"))
		}
		hex := string(p.s[0]) + string(p.s[1])
		p.advance(2)
		code, err := strconv.ParseUint(hex, 16, 8)
		if err != nil {
			panic(err)
		}
		return []byte{byte(code)}
	case 'u':
		if len(p.s) < 4 {
			panic(fmt.Errorf("short \\u escape: expected 4 hex digits"))
		}
		hex := string(p.s[0]) + string(p.s[1]) + string(p.s[2]) + string(p.s[3])
		p.advance(4)
		code, err := strconv.ParseUint(hex, 16, 16)
		if err != nil {
			panic(err)
		}
		return []byte(string(rune(code)))
	case 'U':
		if len(p.s) < 8 {
			panic(fmt.Errorf("short \\U escape: expected 8 hex digits"))
		}
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

	// Handle base prefixes: 0x (hex), 0o (octal), 0b (binary)
	if token == "0" {
		switch lower(p.peek()) {
		case 'x':
			token += string(p.next())
			token += p.token(func(c rune) bool { return ishex(c) || c == '_' })
			i := IntNode{Integer: big.NewInt(0)}
			_, ok := i.Integer.SetString(strings.ReplaceAll(token, "_", ""), 0)
			if !ok {
				panic(errors.New("invalid hex integer"))
			}
			return i
		case 'o':
			token += string(p.next())
			token += p.token(func(c rune) bool { return c >= '0' && c <= '7' || c == '_' })
			i := IntNode{Integer: big.NewInt(0)}
			_, ok := i.Integer.SetString(strings.ReplaceAll(token, "_", ""), 0)
			if !ok {
				panic(errors.New("invalid octal integer"))
			}
			return i
		case 'b':
			token += string(p.next())
			token += p.token(func(c rune) bool { return c == '0' || c == '1' || c == '_' })
			i := IntNode{Integer: big.NewInt(0)}
			_, ok := i.Integer.SetString(strings.ReplaceAll(token, "_", ""), 0)
			if !ok {
				panic(errors.New("invalid binary integer"))
			}
			return i
		}
	}

	// Handle decimal point for floats - but only if followed by a digit
	// (to avoid consuming `.as` in `0.5.as<f4>`)
	isFloat := false
	if p.peek() == '.' && isdigit(p.peek2()) {
		token += string(p.next()) // consume '.'
		token += p.token(isnumber)
		isFloat = true
	}

	// Handle exponent notation (e.g., 1e10, 2.5e-121)
	if lower(p.peek()) == 'e' {
		token += string(p.next()) // consume 'e'/'E'
		if p.peek() == '+' || p.peek() == '-' {
			token += string(p.next()) // consume sign
		}
		token += p.token(isnumber)
		isFloat = true
	}

	if isFloat {
		f := FloatNode{Float: big.NewFloat(0)}
		_, _, err := f.Float.Parse(strings.ReplaceAll(token, "_", ""), 0)
		if err != nil {
			panic(err)
		}
		return f
	}

	i := IntNode{Integer: big.NewInt(0)}
	_, ok := i.Integer.SetString(strings.ReplaceAll(token, "_", ""), 0)
	if !ok {
		panic(errors.New("invalid integer"))
	}
	return i
}

func (p *parser) fstrlit() Node {
	// Called after 'f' has been consumed; the next char is the opening quote.
	quote := p.s[0]
	p.advance(1)
	var parts []FStringPart
	lit := []byte{}

	for {
		if len(p.s) == 0 {
			panic(fmt.Errorf("unterminated f-string literal"))
		}
		c := p.s[0]
		p.advance(1)
		switch c {
		case rune(quote):
			// End of f-string
			if len(lit) > 0 {
				parts = append(parts, FStringPart{Literal: string(lit)})
			}
			return FStringNode{Parts: parts}
		case '\\':
			lit = append(lit, p.strescape(quote)...)
		case '{':
			// Start of expression interpolation
			if len(lit) > 0 {
				parts = append(parts, FStringPart{Literal: string(lit)})
				lit = nil
			}
			// Parse the expression until '}'
			exprNode := p.expr(0)
			if p.next() != '}' {
				panic(fmt.Errorf("expected '}' in f-string"))
			}
			parts = append(parts, FStringPart{Expr: exprNode})
		default:
			lit = append(lit, string(c)...)
		}
	}
}

func (p *parser) strlit() Node {
	quote := p.s[0]
	p.advance(1)
	str := []byte{}

	for {
		if len(p.s) == 0 {
			panic(fmt.Errorf("unterminated string literal"))
		}
		c := p.s[0]
		p.advance(1)
		switch c {
		case quote:
			return StringNode{string(str)}
		case '\\':
			if quote == '\'' {
				str = append(str, string(c)...)
			} else {
				str = append(str, p.strescape(quote)...)
			}
		default:
			str = append(str, string(c)...)
		}
	}
}

func (p *parser) arraylit() Node {
	p.advance(1)
	p.skipwhitespace()
	an := ArrayNode{}
	if p.peek() == ']' {
		p.advance(1)
		return an
	}
	an.Items = []Node{p.expr(0)}
	for {
		p.skipwhitespace()
		switch p.peek() {
		case ',':
			p.advance(1)
			an.Items = append(an.Items, p.expr(0))
		case ']':
			p.advance(1)
			return an
		default:
			panic(fmt.Errorf("expected ',' or ']'"))
		}
	}
}

const (
	depthTernaryExpr = iota
	depthOrExpr
	depthAndExpr
	depthCompareExpr
	depthBitOrExpr
	depthBitXorExpr
	depthBitAndExpr
	depthShiftExpr
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
		case "f":
			// f-string: f"..." or f'...'
			if p.peek() == '"' || p.peek() == '\'' {
				n = p.fstrlit()
				break
			}
			n = IdentNode{Identifier: tok}
		case "not":
			n = UnaryNode{Op: OpLogicalNot, Operand: p.expr(depthMemberExpr)}
		case "true":
			n = BoolNode{Bool: true}
		case "false":
			n = BoolNode{Bool: false}
		case "sizeof":
			// sizeof<type> syntax
			if p.peek() == '<' {
				p.advance(1)
				typeName := ""
				for p.peek() != '>' && p.peek() != 0 {
					typeName += string(p.next())
				}
				if p.next() != '>' {
					panic(fmt.Errorf("expected '>'"))
				}
				n = SizeofNode{TypeName: typeName}
			} else {
				n = IdentNode{Identifier: tok}
			}
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
	case c == '[':
		n = p.arraylit()
	case c == '-':
		// Unary negation
		p.advance(1)
		operand := p.expr(depthMemberExpr)
		n = UnaryNode{Op: OpNegate, Operand: operand}
	default:
		panic(fmt.Errorf("expected primary expression"))
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
			prop := p.token(isident)
			// Handle .as<type> cast syntax
			if prop == "as" && p.peek() == '<' {
				p.advance(1) // skip '<'
				// Read type name until '>'
				typeName := ""
				for p.peek() != '>' && p.peek() != 0 {
					typeName += string(p.next())
				}
				if p.next() != '>' {
					panic(fmt.Errorf("expected '>'"))
				}
				n = CastNode{Operand: n, TypeName: typeName}
				continue
			}
			n = MemberNode{Operand: n, Property: prop}
			// Function call check is handled by the '(' case in the next iteration
			continue
		case '(':
			// Function call: expr(args)
			p.advance(1)
			p.skipwhitespace()
			var args []Node
			if p.peek() != ')' {
				args = append(args, p.expr(0))
				for {
					p.skipwhitespace()
					if p.peek() != ',' {
						break
					}
					p.advance(1)
					p.skipwhitespace()
					args = append(args, p.expr(0))
				}
			}
			p.skipwhitespace()
			if p.next() != ')' {
				panic(fmt.Errorf("expected ')'"))
			}
			n = CallNode{Object: n, Args: args}
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
		}
		if depth >= depthAddExpr {
			break
		}
		switch p.peek() {
		case '<':
			if p.peek2() != '<' {
				break
			}
			p.next()
			p.next()
			n = BinaryNode{Op: OpShiftLeft, A: n, B: p.expr(depthAddExpr)}
			continue
		case '>':
			if p.peek2() != '>' {
				break
			}
			p.next()
			p.next()
			n = BinaryNode{Op: OpShiftRight, A: n, B: p.expr(depthAddExpr)}
			continue
		}
		if depth >= depthShiftExpr {
			break
		}
		if p.peek() == '&' {
			p.next()
			n = BinaryNode{Op: OpBitAnd, A: n, B: p.expr(depthShiftExpr)}
			continue
		}
		if depth >= depthBitAndExpr {
			break
		}
		if p.peek() == '^' {
			p.next()
			n = BinaryNode{Op: OpBitXor, A: n, B: p.expr(depthBitAndExpr)}
			continue
		}
		if depth >= depthBitXorExpr {
			break
		}
		if p.peek() == '|' {
			p.next()
			n = BinaryNode{Op: OpBitOr, A: n, B: p.expr(depthBitXorExpr)}
			continue
		}
		if depth >= depthBitOrExpr {
			break
		}
		switch p.peek() {
		case '=':
			if p.peek2() != '=' {
				panic(fmt.Errorf("expected '=', got %q", p.peek2()))
			}
			p.advance(2)
			n = BinaryNode{Op: OpEqual, A: n, B: p.expr(depthBitOrExpr)}
			continue
		case '!':
			if p.peek2() != '=' {
				panic(fmt.Errorf("expected '=', got %q", p.peek2()))
			}
			p.advance(2)
			n = BinaryNode{Op: OpNotEqual, A: n, B: p.expr(depthBitOrExpr)}
			continue
		case '<':
			p.next()
			if p.peek() == '=' {
				p.next()
				n = BinaryNode{Op: OpLessThanEqual, A: n, B: p.expr(depthBitOrExpr)}
				continue
			} else {
				n = BinaryNode{Op: OpLessThan, A: n, B: p.expr(depthBitOrExpr)}
				continue
			}
		case '>':
			p.next()
			if p.peek() == '=' {
				p.next()
				n = BinaryNode{Op: OpGreaterThanEqual, A: n, B: p.expr(depthBitOrExpr)}
				continue
			} else {
				n = BinaryNode{Op: OpGreaterThan, A: n, B: p.expr(depthBitOrExpr)}
				continue
			}
		}
		if depth >= depthCompareExpr {
			break
		}
		if p.consumeKeyword("and") {
			n = BinaryNode{Op: OpLogicalAnd, A: n, B: p.expr(depthCompareExpr)}
			continue
		}
		if depth >= depthAndExpr {
			break
		}
		if p.consumeKeyword("or") {
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
