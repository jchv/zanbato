package eval

import (
	"fmt"
	"maps"
	"math/big"

	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
)

const maxEvalDepth = 32

// evaluateExpr evaluates a KSY expression in the scope of a node.
// The scope node determines the local context (_parent, _root, _io, field names).
func (t *Tree) evaluateExpr(scope *Node, e *expr.Expr) (*engine.ExprValue, error) {
	t.evalDepth++
	if t.evalDepth > maxEvalDepth {
		t.evalDepth--
		return nil, fmt.Errorf("expression evaluation depth exceeded (possible cycle)")
	}
	defer func() { t.evalDepth-- }()

	ctx := t.contextForNode(scope)
	ctx.PushStack()
	defer ctx.PopStack()
	// Expose `_index` when we're reading an array element. The intrinsic
	// resolves to the index in the innermost active repeat loop.
	if idx := t.currentIndex(); idx >= 0 {
		ctx.SetContext(ctx.WithIndex(engine.NewIntegerLiteralValue(big.NewInt(int64(idx)))))
	}
	return engine.Evaluate(ctx, e)
}

// evaluateExprWithTemp evaluates an expression with a temporary value bound to "_".
// Used for repeat-until conditions where "_" refers to the current element.
func (t *Tree) evaluateExprWithTemp(scope *Node, e *expr.Expr, temp *Node, index int) (*engine.ExprValue, error) {
	ctx := t.contextForNode(scope)
	ctx.PushStack()
	defer ctx.PopStack()

	// Bind "_" to the temporary element and "_index" to the iteration index.
	newCtx := ctx.Context
	if temp != nil && temp.state == stateResolved {
		tmpVal, err := nodeToExprValue(temp)
		if err == nil && tmpVal != nil && scope.typeSym != nil {
			newCtx = newCtx.WithTemporary(tmpVal)
		}
	}
	newCtx = newCtx.WithIndex(engine.NewIntegerLiteralValue(big.NewInt(int64(index))))
	ctx.SetContext(newCtx)

	return engine.Evaluate(ctx, e)
}

// contextForNode creates an EvalContext configured for expression evaluation
// in the scope of the given node. The OnResolve callback lazily resolves
// sibling nodes when the expression engine needs their values.
func (t *Tree) contextForNode(scope *Node) *engine.EvalContext {
	// Find the struct node that is the scope for name resolution.
	// For a seq field, that's its parent struct.
	// For a struct node itself, that's itself.
	structNode := scope
	if scope.schema == nil && scope.parent != nil {
		structNode = scope.parent
	}

	// Build the type context with the correct local/module roots.
	// We create value-level symbols and pre-populate them with resolved
	// Node values so that expressions like _parent.header.qty_entries work.
	var typeCtx *engine.Context
	if structNode.typeSym != nil {
		// Build local root (current struct)
		valSym := nodeToExprValueStruct(structNode)
		if valSym == nil {
			valSym = engine.NewStructValueSymbol(structNode.typeSym, nil)
		}
		// _sizeof is now added by nodeToExprValueStruct automatically

		// Build parent chain (for _parent._parent... access)
		current := valSym
		parentNode := structNode.parent
		for parentNode != nil && parentNode.typeSym != nil {
			parentValSym := nodeToExprValueStruct(parentNode)
			if parentValSym != nil {
				current.Parent = parentValSym
				current = parentValSym
			} else {
				break
			}
			parentNode = parentNode.parent
		}

		typeCtx = t.typeCtx.WithLocalRoot(valSym)

		// Build module root (for _root access)
		if structNode.root != nil && structNode.root.typeSym != nil {
			rootValSym := nodeToExprValueStruct(structNode.root)
			if rootValSym == nil {
				rootValSym = engine.NewStructValueSymbol(structNode.root.typeSym, nil)
			}
			typeCtx = typeCtx.WithModuleRoot(rootValSym)
		}

		// Set stream
		typeCtx = typeCtx.WithStream(
			engine.NewRuntimeStreamValue(structNode.stream, uint64(structNode.streamOffset)),
		)
	} else {
		typeCtx = t.typeCtx
	}

	ctx := engine.NewEvalContext(typeCtx)

	ctx.OnResolve = func(sym *engine.ExprValue) *engine.ExprValue {
		// Prevent arbitrarily deep recursion
		if t.evalDepth > maxEvalDepth {
			return nil
		}
		// Check for param values stored on the struct node
		if sym.Kind == engine.ParamKind && sym.Param != nil && structNode.params != nil {
			if val, ok := structNode.params[string(sym.Param.ID)]; ok {
				return val
			}
		}
		node := nodeForSymbol(structNode, sym)
		if node == nil {
			return nil
		}
		// Try to avoid cycles
		if node.state == stateResolving {
			return nil
		}
		if err := node.Resolve(); err != nil {
			return nil
		}
		if node.state != stateResolved {
			return nil
		}
		ev, err := nodeToExprValue(node)
		if err != nil || ev == nil {
			return nil
		}
		// Cache it for future lookups in this evaluation. Nested member
		// access (`a.b.c`) is handled by ExprValue.Runtime.LookupChild,
		// so we don't need to register children here.
		ctx.PutStack(sym, ev)
		return ev
	}

	return ctx
}

// nodeForSymbol finds the Node that corresponds to a type-level ExprValue symbol
// in the immediate scope. Nested access (a.b.c) is handled by
// ExprValue.Runtime.LookupChild, not by recursive search here.
func nodeForSymbol(structNode *Node, sym *engine.ExprValue) *Node {
	var name string
	switch sym.Kind {
	case engine.AttrKind:
		if sym.Attr != nil {
			name = string(sym.Attr.ID)
		}
	case engine.InstanceKind:
		if sym.Instance != nil {
			name = string(sym.Instance.ID)
		}
	case engine.ParamKind:
		if sym.Param != nil {
			name = string(sym.Param.ID)
		}
	default:
		return nil
	}
	if name == "" {
		return nil
	}
	return structNode.childMap[name]
}

// nodeToExprValueStruct creates a struct ExprValue from a resolved Node,
// with all resolved children pre-populated as runtime values. Returns nil
// if the node has no type symbol.
func nodeToExprValueStruct(n *Node) *engine.ExprValue {
	if n == nil || n.typeSym == nil {
		return nil
	}
	valSym := engine.NewStructValueSymbol(n.typeSym, nil)
	valSym.Runtime = nodeRef{n: n}
	maps.Copy(valSym.Children, n.params)
	return valSym
}

// evaluateExprBool evaluates an expression and returns it as a boolean.
func (t *Tree) evaluateExprBool(scope *Node, e *expr.Expr) (bool, error) {
	val, err := t.evaluateExpr(scope, e)
	if err != nil {
		return false, err
	}
	if val == nil {
		return false, fmt.Errorf("expression evaluated to nil")
	}
	switch val.Kind {
	case engine.BooleanKind:
		return val.Boolean.Value, nil
	case engine.IntegerKind:
		return val.Integer.Value.Sign() != 0, nil
	default:
		return false, fmt.Errorf("expected boolean, got %s", val.Kind)
	}
}

// evaluateExprInt evaluates an expression and returns it as an int64.
func (t *Tree) evaluateExprInt(scope *Node, e *expr.Expr) (int64, error) {
	val, err := t.evaluateExpr(scope, e)
	if err != nil {
		return 0, err
	}
	if val == nil {
		return 0, fmt.Errorf("expression evaluated to nil")
	}
	switch val.Kind {
	case engine.IntegerKind:
		return val.Integer.Value.Int64(), nil
	default:
		return 0, fmt.Errorf("expected integer, got %s", val.Kind)
	}
}
