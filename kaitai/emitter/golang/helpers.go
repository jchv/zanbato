package golang

import (
	"slices"
	"strings"

	"github.com/jchv/zanbato/kaitai/expr"
)

// isMultiByteEncoding returns true if the encoding uses multi-byte code units
// (e.g., UTF-16), meaning terminators need to be multi-byte too.
func isMultiByteEncoding(enc string) bool {
	enc = strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(enc, "-", ""), "_", ""))
	switch enc {
	case "UTF16LE", "UTF16BE", "UTF16":
		return true
	default:
		return false
	}
}

// needsPointerForNil returns true if a Go type needs pointer wrapping to be nilable.
// Pointer types (*T), slices ([]T), interfaces, and 'any' are already nilable.
func needsPointerForNil(goType string) bool {
	if goType == "" || goType == "any" {
		return false
	}
	if strings.HasPrefix(goType, "*") || strings.HasPrefix(goType, "[]") {
		return false
	}
	return true
}

// exprReferencesIO checks if an expression references _io (pos, eof, size).
func exprReferencesIO(e *expr.Expr) bool {
	if e == nil {
		return false
	}
	return nodeReferencesIO(e.Root)
}

func nodeReferencesIO(n expr.Node) bool {
	if n == nil {
		return false
	}
	switch n := n.(type) {
	case expr.IdentNode:
		return n.Identifier == "_io"
	case expr.MemberNode:
		if id, ok := n.Operand.(expr.IdentNode); ok && id.Identifier == "_io" {
			return true
		}
		return nodeReferencesIO(n.Operand)
	case expr.BinaryNode:
		return nodeReferencesIO(n.A) || nodeReferencesIO(n.B)
	case expr.UnaryNode:
		return nodeReferencesIO(n.Operand)
	case expr.CallNode:
		if nodeReferencesIO(n.Object) {
			return true
		}
		if slices.ContainsFunc(n.Args, nodeReferencesIO) {
			return true
		}
	case expr.TernaryNode:
		return nodeReferencesIO(n.A) || nodeReferencesIO(n.B) || nodeReferencesIO(n.C)
	}
	return false
}

// exprContainsIndex checks if an expression tree contains the _index identifier.
func exprContainsIndex(e *expr.Expr) bool {
	if e == nil {
		return false
	}
	return nodeContainsIndex(e.Root)
}

func nodeContainsIndex(n expr.Node) bool {
	if n == nil {
		return false
	}
	switch n := n.(type) {
	case expr.IdentNode:
		return n.Identifier == "_index"
	case expr.MemberNode:
		return nodeContainsIndex(n.Operand)
	case expr.SubscriptNode:
		return nodeContainsIndex(n.A) || nodeContainsIndex(n.B)
	case expr.BinaryNode:
		return nodeContainsIndex(n.A) || nodeContainsIndex(n.B)
	case expr.UnaryNode:
		return nodeContainsIndex(n.Operand)
	case expr.TernaryNode:
		return nodeContainsIndex(n.A) || nodeContainsIndex(n.B) || nodeContainsIndex(n.C)
	case expr.CallNode:
		if slices.ContainsFunc(n.Args, nodeContainsIndex) {
			return true
		}
		return nodeContainsIndex(n.Object)
	case expr.CastNode:
		return nodeContainsIndex(n.Operand)
	}
	return false
}
