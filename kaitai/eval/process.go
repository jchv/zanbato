package eval

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"

	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
)

// ProcessFunc is a handler for a custom `process:` invocation. It is given a
// ProcessCall describing the data to transform and the (lazily-evaluated)
// arguments parsed from the KSY clause, and returns the transformed bytes.
//
// Handlers are registered on a *Tree via RegisterProcess. They are invoked
// only for names not handled by the built-in processes (xor, rol, ror, zlib).
type ProcessFunc func(call *ProcessCall) ([]byte, error)

// ProcessCall is the per-invocation context handed to a ProcessFunc.
type ProcessCall struct {
	// Name is the canonical process name. For a member-call chain like
	// `nested.deeply.custom_fx(...)`, this is the dotted form.
	Name string

	// Data is the bytes the handler must transform and return.
	Data []byte

	args []ProcessArg
}

// NumArgs returns the number of arguments passed to the process call.
func (c *ProcessCall) NumArgs() int { return len(c.args) }

// Arg returns the i-th argument. The caller decides how to interpret it via
// the ProcessArg accessors.
func (c *ProcessCall) Arg(i int) ProcessArg { return c.args[i] }

// ProcessArg wraps one unevaluated argument from a `process:` invocation.
// The expression is decoded on demand against the active evaluation scope -
// no work is done until the handler asks for a specific shape.
type ProcessArg struct {
	node     expr.Node
	evalInt  func(*expr.Expr) (int64, error)
	evalExpr func(*expr.Expr) (*engine.ExprValue, error)
}

// Int evaluates the argument as an integer.
func (a ProcessArg) Int() (int64, error) {
	return a.evalInt(&expr.Expr{Root: a.node})
}

// Bool evaluates the argument as a boolean.
func (a ProcessArg) Bool() (bool, error) {
	ev, err := a.evalExpr(&expr.Expr{Root: a.node})
	if err != nil {
		return false, err
	}
	if ev == nil || ev.Kind != engine.BooleanKind || ev.Boolean == nil {
		return false, fmt.Errorf("process arg: expected boolean, got %v", ev)
	}
	return ev.Boolean.Value, nil
}

// Bytes evaluates the argument as a byte array.
func (a ProcessArg) Bytes() ([]byte, error) {
	ev, err := a.evalExpr(&expr.Expr{Root: a.node})
	if err != nil {
		return nil, err
	}
	if ev == nil || ev.Kind != engine.ByteArrayKind || ev.ByteArray == nil {
		return nil, fmt.Errorf("process arg: expected bytes, got %v", ev)
	}
	return ev.ByteArray.Value, nil
}

// Value evaluates the argument and returns the raw ExprValue.
func (a ProcessArg) Value() (*engine.ExprValue, error) {
	return a.evalExpr(&expr.Expr{Root: a.node})
}

// RegisterProcess associates a custom process name with a handler. Names
// follow KSY convention: a bare identifier ("my_custom_fx") or a dotted
// qualified name ("nested.deeply.custom_fx"). Built-in xor/rol/ror/zlib
// names cannot be overridden - a registered handler for one of those is
// ignored.
func (t *Tree) RegisterProcess(name string, fn ProcessFunc) {
	if t.processes == nil {
		t.processes = make(map[string]ProcessFunc)
	}
	t.processes[name] = fn
}

// applyProcess applies a `process:` expression to data.
func (t *Tree) applyProcess(
	proc *expr.Expr,
	data []byte,
	evalInt func(*expr.Expr) (int64, error),
	evalExpr func(*expr.Expr) (*engine.ExprValue, error),
) ([]byte, error) {
	root := proc.Root
	switch node := root.(type) {
	case expr.IdentNode:
		switch node.Identifier {
		case "zlib":
			return processZlib(data)
		}
		// Bare identifier: custom process with no args.
		return t.dispatchCustom(node.Identifier, nil, data, evalInt, evalExpr)
	case expr.CallNode:
		if ident, ok := node.Object.(expr.IdentNode); ok {
			switch ident.Identifier {
			case "xor":
				if len(node.Args) >= 1 {
					// Check if arg is an array (multi-byte XOR key)
					if arrNode, ok := node.Args[0].(expr.ArrayNode); ok {
						key := make([]byte, len(arrNode.Items))
						for i, item := range arrNode.Items {
							itemExpr := &expr.Expr{Root: item}
							v, err := evalInt(itemExpr)
							if err != nil {
								return nil, fmt.Errorf("xor key[%d]: %w", i, err)
							}
							key[i] = byte(v)
						}
						return processXorMulti(data, key), nil
					}
					keyExpr := &expr.Expr{Root: node.Args[0]}
					key, err := evalInt(keyExpr)
					if err != nil {
						// Try evaluating as bytes (multi-byte XOR from field reference)
						if evalExpr != nil {
							ev, err2 := evalExpr(keyExpr)
							if err2 == nil && ev != nil && ev.Kind == engine.ByteArrayKind && ev.ByteArray != nil {
								return processXorMulti(data, ev.ByteArray.Value), nil
							}
						}
						return nil, fmt.Errorf("xor key: %w", err)
					}
					return processXor(data, byte(key)), nil
				}
			case "rol":
				if len(node.Args) >= 1 {
					keyExpr := &expr.Expr{Root: node.Args[0]}
					count, err := evalInt(keyExpr)
					if err != nil {
						return nil, fmt.Errorf("rol count: %w", err)
					}
					return processRotateLeft(data, int(count)), nil
				}
			case "ror":
				if len(node.Args) >= 1 {
					keyExpr := &expr.Expr{Root: node.Args[0]}
					count, err := evalInt(keyExpr)
					if err != nil {
						return nil, fmt.Errorf("ror count: %w", err)
					}
					return processRotateRight(data, int(count)), nil
				}
			}
			// Custom process invoked as a bare ident call (no namespace).
			return t.dispatchCustom(ident.Identifier, node.Args, data, evalInt, evalExpr)
		}
		// Handle member.call (e.g. nested.deeply.custom_fx(key)) - flatten
		// the member chain to a "namespace.fn" name for registry lookup.
		if name, ok := flattenMemberCallName(node.Object); ok {
			return t.dispatchCustom(name, node.Args, data, evalInt, evalExpr)
		}
	}
	return data, fmt.Errorf("unsupported process: %s", proc.Root.String())
}

// flattenMemberCallName collapses a member chain like `nested.deeply.custom_fx`
// into "nested.deeply.custom_fx". Returns false if the chain isn't pure
// identifier members.
func flattenMemberCallName(n expr.Node) (string, bool) {
	switch v := n.(type) {
	case expr.IdentNode:
		return v.Identifier, true
	case expr.MemberNode:
		base, ok := flattenMemberCallName(v.Operand)
		if !ok {
			return "", false
		}
		return base + "." + v.Property, true
	}
	return "", false
}

// dispatchCustom looks up a registered ProcessFunc for `name` and invokes it.
// Returns an "unsupported process" error if no handler is registered.
func (t *Tree) dispatchCustom(name string, args []expr.Node, data []byte, evalInt func(*expr.Expr) (int64, error), evalExpr func(*expr.Expr) (*engine.ExprValue, error)) ([]byte, error) {
	fn, ok := t.processes[name]
	if !ok {
		return data, fmt.Errorf("unsupported process: %s", name)
	}
	call := &ProcessCall{
		Name: name,
		Data: data,
		args: make([]ProcessArg, len(args)),
	}
	for i, a := range args {
		call.args[i] = ProcessArg{node: a, evalInt: evalInt, evalExpr: evalExpr}
	}
	return fn(call)
}

func processXorMulti(data []byte, key []byte) []byte {
	if len(key) == 0 {
		return data
	}
	result := make([]byte, len(data))
	for i, b := range data {
		result[i] = b ^ key[i%len(key)]
	}
	return result
}

func processXor(data []byte, key byte) []byte {
	result := make([]byte, len(data))
	for i, b := range data {
		result[i] = b ^ key
	}
	return result
}

func processRotateLeft(data []byte, count int) []byte {
	count = ((count % 8) + 8) % 8
	result := make([]byte, len(data))
	for i, b := range data {
		result[i] = (b << uint(count)) | (b >> uint(8-count))
	}
	return result
}

func processRotateRight(data []byte, count int) []byte {
	count = ((count % 8) + 8) % 8
	result := make([]byte, len(data))
	for i, b := range data {
		result[i] = (b >> uint(count)) | (b << uint(8-count))
	}
	return result
}

func processZlib(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	return io.ReadAll(r)
}
