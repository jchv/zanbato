package golang

import (
	"fmt"
	"strings"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/expr"
)

// emitProcess generates code to apply a process transformation to a variable.
func (e *Emitter) emitProcess(fn *goFunc, unit *goUnit, process *expr.Expr, varName string) {
	if process == nil {
		return
	}
	e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
	// Parse the process expression
	root := process.Root
	switch n := root.(type) {
	case expr.CallNode:
		if mn, ok := n.Object.(expr.MemberNode); ok {
			// Method call like _.xor(key) - strip the member node
			switch mn.Property {
			case "xor":
				if len(n.Args) > 0 {
					argStr := e.exprNode(n.Args[0])
					if e.isNodeByteArray(n.Args[0]) {
						fn.pf("%s = kaitai.ProcessXOR(%s, %s)", varName, varName, argStr)
					} else {
						fn.pf("%s = kaitai.ProcessXOR(%s, []byte{byte(%s)})", varName, varName, argStr)
					}
				}
				return
			case "rol":
				if len(n.Args) > 0 {
					fn.pf("%s = kaitai.ProcessRotateLeft(%s, int(%s))", varName, varName, e.exprNode(n.Args[0]))
				}
				return
			case "ror":
				if len(n.Args) > 0 {
					fn.pf("%s = kaitai.ProcessRotateRight(%s, int(%s))", varName, varName, e.exprNode(n.Args[0]))
				}
				return
			case "zlib":
				fn.pf("%s, err = kaitai.ProcessZlib(%s)", varName, varName)
				fn.pf("if err != nil {").indent()
				fn.pf("return err")
				fn.unindent().pf("}")
				return
			}
		}
		if id, ok := n.Object.(expr.IdentNode); ok {
			switch id.Identifier {
			case "xor":
				if len(n.Args) > 0 {
					argStr := e.exprNode(n.Args[0])
					if _, ok := n.Args[0].(expr.ArrayNode); ok {
						// Array literal - already emitted as []byte by exprNode
						fn.pf("%s = kaitai.ProcessXOR(%s, %s)", varName, varName, argStr)
					} else if e.isNodeByteArray(n.Args[0]) {
						// Byte array variable: pass directly
						fn.pf("%s = kaitai.ProcessXOR(%s, %s)", varName, varName, argStr)
					} else {
						// Single value or variable: wrap in []byte
						fn.pf("%s = kaitai.ProcessXOR(%s, []byte{byte(%s)})", varName, varName, argStr)
					}
				}
				return
			case "rol":
				if len(n.Args) > 0 {
					fn.pf("%s = kaitai.ProcessRotateLeft(%s, int(%s))", varName, varName, e.exprNode(n.Args[0]))
				}
				return
			case "ror":
				if len(n.Args) > 0 {
					fn.pf("%s = kaitai.ProcessRotateRight(%s, int(%s))", varName, varName, e.exprNode(n.Args[0]))
				}
				return
			case "zlib":
				fn.pf("%s, err = kaitai.ProcessZlib(%s)", varName, varName)
				fn.pf("if err != nil {").indent()
				fn.pf("return err")
				fn.unindent().pf("}")
				return
			}
		}
	case expr.IdentNode:
		switch n.Identifier {
		case "zlib":
			fn.pf("%s, err = kaitai.ProcessZlib(%s)", varName, varName)
			fn.pf("if err != nil {").indent()
			fn.pf("return err")
			fn.unindent().pf("}")
			return
		default:
			// Custom process with no args: custom_fx_no_args
			procType := e.typeName(kaitai.Identifier(n.Identifier))
			fn.pf("%s = New%s().Decode(%s)", varName, procType, varName)
			return
		}
	}
	// Custom process with args: my_custom_fx(arg1, arg2, ...)
	if call, ok := root.(expr.CallNode); ok {
		var procName string
		switch obj := call.Object.(type) {
		case expr.IdentNode:
			procName = e.typeName(kaitai.Identifier(obj.Identifier))
		case expr.MemberNode:
			// nested.deeply.custom_fx -> use just the last part for the type name
			procName = e.typeName(kaitai.Identifier(obj.Property))
		}
		if procName != "" {
			args := make([]string, len(call.Args))
			for i, arg := range call.Args {
				argStr := e.exprNode(arg)
				// Cast field references to int for custom process constructors,
				// since our generated code uses sized types (uint8, uint16, etc.)
				// but custom process constructors typically use int.
				switch arg.(type) {
				case expr.IdentNode, expr.MemberNode:
					argStr = "int(" + argStr + ")"
				}
				args[i] = argStr
			}
			fn.pf("%s = New%s(%s).Decode(%s)", varName, procName, strings.Join(args, ", "), varName)
			return
		}
	}
	panic(fmt.Errorf("unsupported process expression: %s", process))
}

// emitUnprocess applies the inverse of a process transformation.
func (e *Emitter) emitUnprocess(fn *goFunc, unit *goUnit, process *expr.Expr, varName string) {
	if process == nil {
		return
	}
	e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
	root := process.Root
	switch n := root.(type) {
	case expr.CallNode:
		if mn, ok := n.Object.(expr.MemberNode); ok {
			switch mn.Property {
			case "xor":
				// XOR is self-inverse
				if len(n.Args) > 0 {
					argStr := e.exprNode(n.Args[0])
					if e.isNodeByteArray(n.Args[0]) {
						fn.pf("%s = kaitai.ProcessXOR(%s, %s)", varName, varName, argStr)
					} else {
						fn.pf("%s = kaitai.ProcessXOR(%s, []byte{byte(%s)})", varName, varName, argStr)
					}
				}
				return
			case "rol":
				// Inverse of ROL is ROR
				if len(n.Args) > 0 {
					fn.pf("%s = kaitai.ProcessRotateRight(%s, int(%s))", varName, varName, e.exprNode(n.Args[0]))
				}
				return
			case "ror":
				// Inverse of ROR is ROL
				if len(n.Args) > 0 {
					fn.pf("%s = kaitai.ProcessRotateLeft(%s, int(%s))", varName, varName, e.exprNode(n.Args[0]))
				}
				return
			case "zlib":
				fn.pf("%s, err = kaitai.UnprocessZlib(%s)", varName, varName)
				fn.pf("if err != nil { return err }")
				return
			}
		}
		if id, ok := n.Object.(expr.IdentNode); ok {
			switch id.Identifier {
			case "xor":
				if len(n.Args) > 0 {
					argStr := e.exprNode(n.Args[0])
					if e.isNodeByteArray(n.Args[0]) {
						fn.pf("%s = kaitai.ProcessXOR(%s, %s)", varName, varName, argStr)
					} else {
						fn.pf("%s = kaitai.ProcessXOR(%s, []byte{byte(%s)})", varName, varName, argStr)
					}
				}
				return
			case "rol":
				if len(n.Args) > 0 {
					fn.pf("%s = kaitai.ProcessRotateRight(%s, int(%s))", varName, varName, e.exprNode(n.Args[0]))
				}
				return
			case "ror":
				if len(n.Args) > 0 {
					fn.pf("%s = kaitai.ProcessRotateLeft(%s, int(%s))", varName, varName, e.exprNode(n.Args[0]))
				}
				return
			case "zlib":
				fn.pf("%s, err = kaitai.UnprocessZlib(%s)", varName, varName)
				fn.pf("if err != nil { return err }")
				return
			}
		}
	case expr.IdentNode:
		switch n.Identifier {
		case "zlib":
			fn.pf("%s, err = kaitai.UnprocessZlib(%s)", varName, varName)
			fn.pf("if err != nil { return err }")
			return
		default:
			procType := e.typeName(kaitai.Identifier(n.Identifier))
			fn.pf("%s = New%s().Encode(%s)", varName, procType, varName)
			return
		}
	}
	// Custom process with args: mirror `New<T>(args).Decode(x)` with `.Encode(x)`.
	if call, ok := root.(expr.CallNode); ok {
		var procName string
		switch obj := call.Object.(type) {
		case expr.IdentNode:
			procName = e.typeName(kaitai.Identifier(obj.Identifier))
		case expr.MemberNode:
			// nested.deeply.custom_fx -> use just the last part for the type name
			procName = e.typeName(kaitai.Identifier(obj.Property))
		}
		if procName != "" {
			args := make([]string, len(call.Args))
			for i, arg := range call.Args {
				argStr := e.exprNode(arg)
				switch arg.(type) {
				case expr.IdentNode, expr.MemberNode:
					argStr = "int(" + argStr + ")"
				}
				args[i] = argStr
			}
			fn.pf("%s = New%s(%s).Encode(%s)", varName, procName, strings.Join(args, ", "), varName)
			return
		}
	}
	panic(fmt.Errorf("unsupported unprocess expression: %s", process))
}
