package c

import (
	"fmt"
	"strings"

	"github.com/jchv/zanbato/kaitai/expr"
)

func (e *Emitter) emitProcess(src *buf, process *expr.Expr, varName string) {
	if process == nil {
		return
	}
	root := process.Root
	if call, ok := root.(expr.CallNode); ok {
		var name string
		var args []expr.Node
		switch obj := call.Object.(type) {
		case expr.IdentNode:
			name = obj.Identifier
		case expr.MemberNode:
			name = obj.Property
		}
		args = call.Args
		switch name {
		case "xor":
			if len(args) == 1 {
				keyExpr := e.xorKeyBytes(args[0])
				src.pf("%s = zb_process_xor(arena, %s, %s);", varName, varName, keyExpr)
				return
			}
		case "rol":
			if len(args) == 1 {
				src.pf("%s = zb_process_rotate_left(arena, %s, (int)(%s));", varName, varName, e.exprNode(args[0]))
				return
			}
		case "ror":
			if len(args) == 1 {
				src.pf("%s = zb_process_rotate_right(arena, %s, (int)(%s));", varName, varName, e.exprNode(args[0]))
				return
			}
		}
		if name != "" {
			e.recordCustomProcess(name + "_decode")
			callExpr := e.customProcessCall(name+"_decode", varName, args)
			src.pf("%s = %s;", varName, callExpr)
			return
		}
	}
	if id, ok := root.(expr.IdentNode); ok {
		switch id.Identifier {
		case "zlib":
			emitTry(src, "zb_process_zlib(arena, %s, &%s)", varName, varName)
			return
		}
		e.recordCustomProcess(id.Identifier + "_decode")
		src.pf("%s = %s_decode(arena, %s);", varName, id.Identifier, varName)
		return
	}
	panic(fmt.Errorf("unsupported process expression: %s", process))
}

func (e *Emitter) emitUnprocess(src *buf, process *expr.Expr, varName string) {
	if process == nil {
		return
	}
	defer e.saveExprMode()()
	e.mode.writingContext = true
	root := process.Root
	if call, ok := root.(expr.CallNode); ok {
		var name string
		var args []expr.Node
		switch obj := call.Object.(type) {
		case expr.IdentNode:
			name = obj.Identifier
		case expr.MemberNode:
			name = obj.Property
		}
		args = call.Args
		switch name {
		case "xor":
			if len(args) == 1 {
				keyExpr := e.xorKeyBytes(args[0])
				src.pf("%s = zb_process_xor(this_->_arena, %s, %s);", varName, varName, keyExpr)
				return
			}
		case "rol":
			if len(args) == 1 {
				src.pf("%s = zb_process_rotate_right(this_->_arena, %s, (int)(%s));", varName, varName, e.exprNode(args[0]))
				return
			}
		case "ror":
			if len(args) == 1 {
				src.pf("%s = zb_process_rotate_left(this_->_arena, %s, (int)(%s));", varName, varName, e.exprNode(args[0]))
				return
			}
		}
		if name != "" {
			e.recordCustomProcess(name + "_encode")
			callExpr := strings.Replace(e.customProcessCall(name+"_encode", varName, args),
				"arena,", "this_->_arena,", 1)
			src.pf("%s = %s;", varName, callExpr)
			return
		}
	}
	if id, ok := root.(expr.IdentNode); ok {
		switch id.Identifier {
		case "zlib":
			src.pf("{ zb_bytes_t _z; ZB_TRY(zb_unprocess_zlib(this_->_arena, %s, &_z)); %s = _z; }",
				varName, varName)
			return
		}
		e.recordCustomProcess(id.Identifier + "_encode")
		src.pf("%s = %s_encode(this_->_arena, %s);", varName, id.Identifier, varName)
		return
	}
	panic(fmt.Errorf("unsupported unprocess expression: %s", process))
}

func (e *Emitter) xorKeyBytes(n expr.Node) string {
	if e.isNodeByteArray(n) {
		return e.exprNode(n)
	}
	return fmt.Sprintf("((zb_bytes_t){.data=(const uint8_t[]){(uint8_t)(%s)}, .len=1})", e.exprNode(n))
}

func (e *Emitter) recordCustomProcess(name string) {
	if e.file.customProcessSigs == nil {
		e.file.customProcessSigs = map[string]string{}
	}
	if _, ok := e.file.customProcessSigs[name]; ok {
		return
	}
	e.file.customProcessSigs[name] = "zb_bytes_t " + name + "();"
	e.file.customProcessOrder = append(e.file.customProcessOrder, name)
}

func (e *Emitter) customProcessCall(fnName, inVar string, args []expr.Node) string {
	var sb strings.Builder
	sb.WriteString(fnName)
	sb.WriteString("(arena, ")
	sb.WriteString(inVar)
	for _, a := range args {
		sb.WriteString(", ")
		sb.WriteString(e.exprNode(a))
	}
	sb.WriteString(")")
	return sb.String()
}
