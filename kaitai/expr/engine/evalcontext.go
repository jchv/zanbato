package engine

type contextStack struct {
	values map[*ExprValue]*ExprValue
	parent *contextStack
}

type EvalContext struct {
	*Context
	globals map[*ExprValue]*ExprValue
	stack   *contextStack
}

// NewEvalContext creates a new runtime evaluation context from a type context.
func NewEvalContext(context *Context) *EvalContext {
	return &EvalContext{
		Context: context,
		globals: make(map[*ExprValue]*ExprValue),
	}
}

// SetContext sets the underlying context of the eval context.
func (e *EvalContext) SetContext(context *Context) {
	e.Context = context
}

// RuntimeValue returns a resolved runtime value for a type value if one exists.
func (e *EvalContext) RuntimeValue(typVal *ExprValue) *ExprValue {
	for frame := e.stack; frame != nil; frame = frame.parent {
		if val, ok := frame.values[typVal]; ok {
			return val
		}
	}
	if val, ok := e.globals[typVal]; ok {
		return val
	}
	return nil
}

// PutGlobal stores the provided global value for a given type value.
func (e *EvalContext) PutGlobal(typVal *ExprValue, rtVal *ExprValue) {
	e.globals[typVal] = rtVal
}

// PushStack pushes a new stack frame.
func (e *EvalContext) PushStack() {
	e.stack = &contextStack{
		values: make(map[*ExprValue]*ExprValue),
		parent: e.stack,
	}
}

// PutStack stores the provided value for a given type value in the current stack frame.
func (e *EvalContext) PutStack(typVal *ExprValue, rtVal *ExprValue) {
	e.stack.values[typVal] = rtVal
}

// PopStack pops a stack frame.
func (e *EvalContext) PopStack() {
	e.stack = e.stack.parent
}
