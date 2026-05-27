package c

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/types"
)

func (e *Emitter) nextAuxLit(kind string) string {
	e.file.auxLitCounter++
	return fmt.Sprintf("_zb_%s_%d", kind, e.file.auxLitCounter)
}

func (e *Emitter) emitArrayLiteralAsArrayType(elemKsType string, arr expr.ArrayNode) string {
	ref, err := types.ParseTypeRef(elemKsType)
	if err != nil {
		panic(fmt.Errorf("bad array elem type %s", elemKsType))
	}
	elem := e.declTypeRef(&ref, false)
	if elem == "" {
		panic(fmt.Errorf("unknown array elem type"))
	}
	tag := e.arrayTagFor(elem)
	parts := make([]string, len(arr.Items))
	for i, item := range arr.Items {
		parts[i] = fmt.Sprintf("(%s)(%s)", elem, e.exprNode(item))
	}
	name := e.nextAuxLit("arr")
	e.file.source.pf("static const %s %s[] = {%s};", elem, name, strings.Join(parts, ", "))
	return fmt.Sprintf("(%s){.data=(%s *)%s, .len=%d, .cap=%d}",
		tag, elem, name, len(arr.Items), len(arr.Items))
}

func (e *Emitter) staticSizeOfNode(n expr.Node) (int64, bool) {
	r := engine.ResultTypeOfNode(e.context, n)
	if r == nil {
		return 0, false
	}
	switch r.Kind {
	case engine.AttrKind:
		if r.Attr != nil {
			if sz := engine.ComputeAttrSize(r.Attr); sz >= 0 {
				return sz, true
			}
			if r.Attr.Type.TypeRef != nil && r.Attr.Type.TypeRef.Kind == types.User {
				if rt := e.resolveQualifiedType(r.Attr.Type.TypeRef.User.Name); rt != nil && rt.Struct != nil {
					if sz := e.context.ComputeStructSize(rt.Struct.Type); sz >= 0 {
						return sz, true
					}
				}
			}
		}
	case engine.StructKind:
		if r.Struct != nil && r.Struct.Type != nil {
			if sz := e.context.ComputeStructSize(r.Struct.Type); sz >= 0 {
				return sz, true
			}
		}
	case engine.ParamKind:
		if r.Param != nil {
			if sz := engine.ComputeTypeRefSize(&r.Param.Type); sz >= 0 {
				return sz, true
			}
		}
	}
	return 0, false
}

func (e *Emitter) exprIs64Bit(n expr.Node) (out bool) {
	defer func() { _ = recover() }()
	r := engine.ResultTypeOfNode(e.context, n)
	if r == nil {
		return false
	}
	if r.Kind == engine.InstanceKind && r.Instance != nil && r.Instance.Value != nil {
		t := e.inferValueType(r.Instance.Value)
		return t == "int64_t" || t == "uint64_t"
	}
	if vt, ok := r.ValueType(); ok && vt.Type.TypeRef != nil {
		switch vt.Type.TypeRef.Kind {
		case types.U8, types.U8le, types.U8be, types.S8, types.S8le, types.S8be:
			return true
		}
	}
	return false
}

func (e *Emitter) exprIsFloat(n expr.Node) (out bool) {
	defer func() { _ = recover() }()
	switch t := n.(type) {
	case expr.FloatNode:
		return true
	case expr.CastNode:
		if t.TypeName == "f4" || t.TypeName == "f8" {
			return true
		}
	case expr.BinaryNode:
		return e.exprIsFloat(t.A) || e.exprIsFloat(t.B)
	case expr.UnaryNode:
		return e.exprIsFloat(t.Operand)
	case expr.TernaryNode:
		return e.exprIsFloat(t.B) || e.exprIsFloat(t.C)
	}
	r := engine.ResultTypeOfNode(e.context, n)
	if r == nil {
		return false
	}
	if r.Kind == engine.FloatKind {
		return true
	}
	if r.Kind == engine.InstanceKind && r.Instance != nil && r.Instance.Value != nil {
		t := e.inferValueType(r.Instance.Value)
		return t == "double" || t == "float"
	}
	if vt, ok := r.ValueType(); ok && vt.Type.TypeRef != nil {
		switch vt.Type.TypeRef.Kind {
		case types.F4, types.F4le, types.F4be, types.F8, types.F8le, types.F8be, types.UntypedFloat:
			return true
		}
	}
	return false
}

func (e *Emitter) structOfNode(n expr.Node) string {
	defer func() { _ = recover() }()
	res := engine.ResultTypeOfNode(e.context, n)
	if res == nil {
		return ""
	}
	switch res.Kind {
	case engine.StructKind, engine.StructRootKind:
		if res.Struct != nil && res.Struct.Type != nil {
			return e.prefix(res.DefParent) + e.typeName(res.Struct.Type.ID)
		}
	case engine.AttrKind:
		if res.Attr != nil && res.Attr.Type.TypeRef != nil && res.Attr.Type.TypeRef.Kind == types.User {
			r := e.tryResolveTypeInScope(res.Attr.Type.TypeRef.User.Name, res.Parent)
			if r != nil && r.Struct != nil && r.Struct.Type != nil {
				return e.prefix(r.DefParent) + e.typeName(r.Struct.Type.ID)
			}
		}
	case engine.ParamKind:
		if res.Param != nil && res.Param.Type.Kind == types.User {
			r := e.tryResolveTypeInScope(res.Param.Type.User.Name, res.Parent)
			if r != nil && r.Struct != nil && r.Struct.Type != nil {
				return e.prefix(r.DefParent) + e.typeName(r.Struct.Type.ID)
			}
		}
	case engine.MethodKind:
		if res.Method != nil {
			ret := res.Method.ReturnType
			if ret.Type.TypeRef != nil && ret.Type.TypeRef.Kind == types.User {
				r := e.tryResolveType(ret.Type.TypeRef.User.Name)
				if r != nil && r.Struct != nil && r.Struct.Type != nil {
					return e.prefix(r.DefParent) + e.typeName(r.Struct.Type.ID)
				}
			}
		}
	case engine.InstanceKind:
		if res.Instance != nil && res.Instance.Value != nil {
			isDefaultBytes := res.Instance.Type.TypeRef != nil && res.Instance.Type.TypeRef.Kind == types.Bytes
			if res.Instance.Type.TypeRef == nil || isDefaultBytes {
				if inner := e.structOfNode(res.Instance.Value.Root); inner != "" {
					return inner
				}
			}
		}
		if res.Instance != nil && res.Instance.Type.TypeRef != nil && res.Instance.Type.TypeRef.Kind == types.User {
			r := e.tryResolveTypeInScope(res.Instance.Type.TypeRef.User.Name, res.Parent)
			if r != nil && r.Struct != nil && r.Struct.Type != nil {
				return e.prefix(r.DefParent) + e.typeName(r.Struct.Type.ID)
			}
		}
	}
	return ""
}

func (e *Emitter) thisExpr() string {
	if e.mode.thisPointerName != "" {
		return e.mode.thisPointerName
	}
	return "this_"
}

func (e *Emitter) expr(ex *expr.Expr) (out string) {
	if ex == nil {
		return "0"
	}
	e.withParentBinding(func() { out = e.exprNode(ex.Root) })
	return
}

func (e *Emitter) withParentBinding(fn func()) {
	if e.file.parents.Inferred == nil || e.currentStruct == nil || e.currentStruct.Struct == nil {
		fn()
		return
	}
	ks := e.currentStruct.Struct.Type
	parent, ok := e.file.parents.Inferred[ks]
	if !ok || parent == nil {
		fn()
		return
	}
	synth := &engine.ExprValue{
		Kind:      e.currentStruct.Kind,
		DefParent: e.currentStruct.DefParent,
		Struct:    e.currentStruct.Struct,
		Children:  e.currentStruct.Children,
		Types:     e.currentStruct.Types,
		Parent:    parent,
	}
	defer e.enterLocal(synth)()
	fn()
}

func (e *Emitter) exprNode(node expr.Node) string {
	switch t := node.(type) {
	case expr.IntNode:
		s := t.Integer.String()
		if t.Integer.Cmp(big.NewInt(0)) >= 0 && !t.Integer.IsInt64() {
			return "(uint64_t)" + s + "ull"
		}
		if t.Integer.Cmp(big.NewInt(-(1<<31))) < 0 || t.Integer.Cmp(big.NewInt(1<<31-1)) > 0 {
			return "INT64_C(" + s + ")"
		}
		return s
	case expr.FloatNode:
		return t.Float.Text('g', -1)
	case expr.BoolNode:
		if t.Bool {
			return "1"
		}
		return "0"
	case expr.StringNode:
		return e.stringLiteralBytes(t.Str)
	case expr.IdentNode:
		return e.exprIdent(t)
	case expr.UnaryNode:
		return e.exprUnary(t)
	case expr.BinaryNode:
		return e.exprBinary(t)
	case expr.TernaryNode:
		bC := e.exprNode(t.B)
		cC := e.exprNode(t.C)
		bIsPtr := e.exprIsPointer(t.B)
		cIsPtr := e.exprIsPointer(t.C)
		bStruct := e.structOfNode(t.B)
		cStruct := e.structOfNode(t.C)
		if bIsPtr && cIsPtr && bStruct != cStruct {
			return fmt.Sprintf("((%s) ? (void *)(%s) : (void *)(%s))",
				e.exprNode(t.A), bC, cC)
		}
		return fmt.Sprintf("((%s) ? (%s) : (%s))",
			e.exprNode(t.A), bC, cC)
	case expr.MemberNode:
		return e.exprMember(t)
	case expr.SubscriptNode:
		return e.exprSubscript(t)
	case expr.ArrayNode:
		return e.exprArray(t)
	case expr.ScopeNode:
		return e.exprScope(t)
	case expr.CastNode:
		return e.exprCast(t)
	case expr.SizeofNode:
		return e.exprSizeof(t)
	case expr.BitSizeofNode:
		return e.exprBitSizeof(t)
	case expr.CallNode:
		return e.exprCall(t)
	case expr.FStringNode:
		return e.exprFString(t)
	}
	panic(fmt.Errorf("unsupported expr: %T", node))
}

func (e *Emitter) exprIdent(t expr.IdentNode) string {
	switch t.Identifier {
	case "_index":
		return "_i"
	case "_":
		return "_repeat_elem"
	case "_sizeof":
		ks := e.context.Struct()
		if ks != nil {
			if sz := e.context.ComputeStructSize(ks); sz >= 0 {
				return strconv.FormatInt(sz, 10)
			}
		}
		// Unknown sizeof
		return "-1"
	case "_root":
		return e.thisExpr() + "->_root"
	case "_parent":
		return e.thisExpr() + "->_parent"
	case "_io":
		return e.thisExpr() + "->_io"
	}
	v := engine.ResultTypeOfNode(e.context, t)
	if v == nil {
		panic(fmt.Errorf("unresolved ident: %s", t.Identifier))
	}
	switch v {
	case e.context.StreamValue():
		return e.thisExpr() + "->_io"
	}
	switch v.Kind {
	case engine.ParamKind:
		return e.thisExpr() + "->" + e.fieldName(v.Param.ID)
	case engine.AttrKind:
		return e.thisExpr() + "->" + e.fieldName(v.Attr.ID)
	case engine.InstanceKind:
		if e.currentStructName != "" && (v.Instance.Value != nil || v.Instance.Pos != nil) {
			return fmt.Sprintf("%s_get_%s(%s)", e.currentStructName, e.fieldName(v.Instance.ID), e.thisExpr())
		}
		return e.thisExpr() + "->" + e.fieldName(v.Instance.ID)
	case engine.AliasKind:
		return v.Alias.Target
	}
	panic(fmt.Errorf("unresolved ident kind: %s", v.Kind))
}

func (e *Emitter) exprUnary(t expr.UnaryNode) string {
	inner := e.exprNode(t.Operand)
	switch t.Op {
	case expr.OpLogicalNot:
		return "!(" + inner + ")"
	case expr.OpNegate:
		return "-(" + inner + ")"
	case expr.OpInvert:
		if e.exprIs64Bit(t.Operand) {
			return "~((int64_t)(" + inner + "))"
		}
		if e.compat.HasCalcIntTypeTruncationBug() && !e.exprIsFloat(t.Operand) {
			return "((int64_t)(int32_t)(~(int64_t)(" + inner + ")))"
		}
		return "~((int64_t)(" + inner + "))"
	}
	panic(fmt.Errorf("unsupported unary %s", t.Op))
}

func (e *Emitter) exprBinary(t expr.BinaryNode) string {
	a := e.exprNode(t.A)
	b := e.exprNode(t.B)
	widen := e.needsInt64Widening(t.A, t.B)
	cast := func(s string) string {
		if widen {
			return "((int64_t)(" + s + "))"
		}
		return "(" + s + ")"
	}
	resultIsFloat := false
	if e.compat.HasCalcIntTypeTruncationBug() {
		if e.exprIsFloat(t.A) || e.exprIsFloat(t.B) {
			resultIsFloat = true
		}
	}
	trunc := func(s string) string {
		if e.compat.HasCalcIntTypeTruncationBug() && !resultIsFloat {
			return "((int64_t)(int32_t)(" + s + "))"
		}
		return s
	}
	switch t.Op {
	case expr.OpAdd:
		if e.isByteArrayBinary(t.A, t.B) {
			return "zb_bytes_concat(this_->_arena, " + a + ", " + b + ")"
		}
		return trunc(cast(a) + " + " + cast(b))
	case expr.OpSub:
		return trunc(cast(a) + " - " + cast(b))
	case expr.OpMult:
		return trunc(cast(a) + " * " + cast(b))
	case expr.OpDiv:
		return trunc("zb_div_floor((int64_t)(" + a + "), (int64_t)(" + b + "))")
	case expr.OpMod:
		return trunc("zb_mod_floor((int64_t)(" + a + "), (int64_t)(" + b + "))")
	case expr.OpLessThan:
		if e.isByteArrayBinary(t.A, t.B) {
			return "(zb_bytes_compare(" + a + ", " + b + ") < 0)"
		}
		return "(" + a + ") < (" + b + ")"
	case expr.OpLessThanEqual:
		if e.isByteArrayBinary(t.A, t.B) {
			return "(zb_bytes_compare(" + a + ", " + b + ") <= 0)"
		}
		return "(" + a + ") <= (" + b + ")"
	case expr.OpGreaterThan:
		if e.isByteArrayBinary(t.A, t.B) {
			return "(zb_bytes_compare(" + a + ", " + b + ") > 0)"
		}
		return "(" + a + ") > (" + b + ")"
	case expr.OpGreaterThanEqual:
		if e.isByteArrayBinary(t.A, t.B) {
			return "(zb_bytes_compare(" + a + ", " + b + ") >= 0)"
		}
		return "(" + a + ") >= (" + b + ")"
	case expr.OpEqual:
		if e.isByteArrayBinary(t.A, t.B) {
			return "zb_bytes_equal(" + a + ", " + b + ")"
		}
		return "(" + a + ") == (" + b + ")"
	case expr.OpNotEqual:
		if e.isByteArrayBinary(t.A, t.B) {
			return "!zb_bytes_equal(" + a + ", " + b + ")"
		}
		return "(" + a + ") != (" + b + ")"
	case expr.OpShiftLeft:
		return trunc("(" + a + ") << (" + b + ")")
	case expr.OpShiftRight:
		return trunc("(" + a + ") >> (" + b + ")")
	case expr.OpBitAnd:
		return trunc("(" + a + ") & (" + b + ")")
	case expr.OpBitOr:
		return trunc("(" + a + ") | (" + b + ")")
	case expr.OpBitXor:
		return trunc("(" + a + ") ^ (" + b + ")")
	case expr.OpLogicalAnd:
		return "(" + a + ") && (" + b + ")"
	case expr.OpLogicalOr:
		return "(" + a + ") || (" + b + ")"
	}
	panic(fmt.Errorf("unsupported binary op %s", t.Op))
}

func (e *Emitter) exprMember(t expr.MemberNode) string {
	if ter, ok := t.Operand.(expr.TernaryNode); ok {
		bStruct := e.structOfNode(ter.B)
		cStruct := e.structOfNode(ter.C)
		bPtr := e.exprIsPointer(ter.B)
		cPtr := e.exprIsPointer(ter.C)
		if bPtr && cPtr && bStruct != cStruct &&
			(t.Property == "_io" || t.Property == "_root" || t.Property == "_parent") {
			lhs := expr.MemberNode{Operand: ter.B, Property: t.Property}
			rhs := expr.MemberNode{Operand: ter.C, Property: t.Property}
			return fmt.Sprintf("((%s) ? (%s) : (%s))",
				e.exprNode(ter.A), e.exprNode(lhs), e.exprNode(rhs))
		}
	}
	switch t.Property {
	case "_root":
		return e.exprNode(t.Operand) + "->_root"
	case "_parent":
		if id, ok := t.Operand.(expr.IdentNode); ok && id.Identifier == "_parent" {
			if cast := e.parentFallbackForCurrent(); cast != "" {
				return fmt.Sprintf("((%s)(%s))->_parent", cast, e.exprNode(t.Operand))
			}
		}
		return e.exprNode(t.Operand) + "->_parent"
	case "_io":
		return e.exprNode(t.Operand) + "->_io"
	case "_sizeof":
		if sz, ok := e.staticSizeOfNode(t.Operand); ok {
			return strconv.FormatInt(sz, 10)
		}
		panic(fmt.Errorf("unable to compute _sizeof for %s", t.Operand))
	}
	v := engine.ResultTypeOfNode(e.context, t)
	if v == nil {
		switch t.Property {
		case "length", "size":
			return "(" + e.exprNode(t.Operand) + ").len"
		case "first":
			return "(" + e.exprNode(t.Operand) + ").data[0]"
		case "last":
			operand := e.exprNode(t.Operand)
			return "(" + operand + ").data[(" + operand + ").len-1]"
		}
		panic(fmt.Errorf("unresolved member: %s.%s", t.Operand, t.Property))
	}
	switch v.Kind {
	case engine.ParamKind:
		return e.exprNode(t.Operand) + "->" + e.fieldName(v.Param.ID)
	case engine.AttrKind:
		return e.exprNode(t.Operand) + "->" + e.fieldName(v.Attr.ID)
	case engine.InstanceKind:
		opName := e.structOfNode(t.Operand)
		if opName != "" && (v.Instance.Value != nil || v.Instance.Pos != nil) {
			return fmt.Sprintf("%s_get_%s(%s)", opName, e.fieldName(v.Instance.ID), e.exprNode(t.Operand))
		}
		return e.exprNode(t.Operand) + "->" + e.fieldName(v.Instance.ID)
	case engine.MethodKind:
		return e.exprMethod(t, v)
	}
	panic(fmt.Errorf("unsupported member kind %s: %s", v.Kind, t.Property))
}

func (e *Emitter) exprMethod(t expr.MemberNode, v *engine.ExprValue) string {
	operand := e.exprNode(t.Operand)
	m := v.Method
	switch m.Method {
	case engine.MethodArraySize, engine.MethodByteArrayLength:
		return "(" + operand + ").len"
	case engine.MethodStringLength:
		return "zb_utf8_char_count(" + operand + ")"
	case engine.MethodStringReverse:
		return "zb_utf8_reverse(" + e.thisExpr() + "->_arena, " + operand + ")"
	case engine.MethodStringToInt:
		return fmt.Sprintf("zb_str_to_i_strict(%s->_arena, %s, 10, &%s->_inst_err)",
			e.thisExpr(), operand, e.thisExpr())
	case engine.MethodArrayFirst:
		return "(" + operand + ").data[0]"
	case engine.MethodArrayLast:
		return "(" + operand + ").data[(" + operand + ").len-1]"
	case engine.MethodArrayMin, engine.MethodArrayMax:
		op := "<"
		if m.Method == engine.MethodArrayMax {
			op = ">"
		}
		elem := e.arrayElemCTypeOfNode(t.Operand)
		operandIsBytes := e.isNodeByteArray(t.Operand)
		if elem == "" {
			if operandIsBytes {
				elem = "uint8_t"
			} else {
				elem = "int64_t"
			}
		}
		arrType := "zb_bytes_t"
		if !operandIsBytes {
			arrType = e.arrayTagFor(elem)
		}
		arrTmp := e.nextAuxLit("arr")
		resTmp := e.nextAuxLit("m")
		e.file.source.pf("%s %s = (%s);", arrType, arrTmp, operand)
		e.file.source.pf("%s %s = %s.data[0];", elem, resTmp, arrTmp)
		e.file.source.pf("for (size_t _k = 1; _k < %s.len; _k++) {", arrTmp)
		if elem == "zb_bytes_t" {
			cmpOp := "< 0"
			if m.Method == engine.MethodArrayMax {
				cmpOp = "> 0"
			}
			e.file.source.indent().pf("if (zb_bytes_compare(%s.data[_k], %s) %s) %s = %s.data[_k];",
				arrTmp, resTmp, cmpOp, resTmp, arrTmp).unindent()
		} else {
			e.file.source.indent().pf("if (%s.data[_k] %s %s) %s = %s.data[_k];",
				arrTmp, op, resTmp, resTmp, arrTmp).unindent()
		}
		e.file.source.pf("}")
		return resTmp
	case engine.MethodIntToString:
		return "zb_int_to_s(" + e.thisExpr() + "->_arena, (int64_t)(" + operand + "))"
	case engine.MethodFloatToInt:
		return "((int64_t)(" + operand + "))"
	case engine.MethodBoolToInt:
		return "((" + operand + ") ? 1 : 0)"
	case engine.MethodEnumToInt:
		return "((int64_t)(" + operand + "))"
	case engine.MethodStreamEOF:
		return "zb_stream_eof(" + operand + ")"
	case engine.MethodStreamSize:
		return "zb_stream_size(" + operand + ")"
	case engine.MethodStreamPos:
		if e.mode.writingContext && e.exprIsRootIO(t.Operand) {
			if e.mode.writerPosOffset != "" {
				return "(zb_writer_pos(wstream) + (" + e.mode.writerPosOffset + "))"
			}
			return "zb_writer_pos(wstream)"
		}
		return "zb_stream_pos(" + operand + ")"
	case engine.MethodByteArrayToString:
		if e.isNodeByteArray(t.Operand) {
			return operand
		}
		return "zb_int_to_s(" + e.thisExpr() + "->_arena, (int64_t)(" + operand + "))"
	}
	panic(fmt.Errorf("unsupported method: %s", m.Method))
}

func (e *Emitter) exprSubscript(t expr.SubscriptNode) string {
	a := e.exprNode(t.A)
	b := e.exprNode(t.B)
	return "(" + a + ").data[(" + b + ")]"
}

func (e *Emitter) exprArray(t expr.ArrayNode) string {
	if len(t.Items) == 0 {
		return "((zb_bytes_t){.data=NULL, .len=0})"
	}
	allByteLit := true
	for _, item := range t.Items {
		if n, ok := item.(expr.IntNode); ok {
			if n.Integer.Sign() < 0 || n.Integer.Cmp(big.NewInt(255)) > 0 {
				allByteLit = false
				break
			}
		} else {
			allByteLit = false
			break
		}
	}
	parts := make([]string, len(t.Items))
	for i, item := range t.Items {
		parts[i] = e.exprNode(item)
	}
	if allByteLit {
		return fmt.Sprintf("((zb_bytes_t){.data=(const uint8_t[]){%s}, .len=%d})", strings.Join(parts, ", "), len(t.Items))
	}
	// Non-literal byte-sized items
	allByteVal := true
	for _, item := range t.Items {
		if n, ok := item.(expr.IntNode); ok {
			if n.Integer.Sign() < 0 || !n.Integer.IsInt64() || n.Integer.Int64() > 255 {
				allByteVal = false
				break
			}
			continue
		}
		k := engine.ResultTypeOfNode(e.context, item).TypeKind()
		if k != types.U1 && k != types.S1 {
			allByteVal = false
			break
		}
	}
	if allByteVal {
		castParts := make([]string, len(t.Items))
		for i, p := range parts {
			castParts[i] = "(uint8_t)(" + p + ")"
		}
		return fmt.Sprintf("((zb_bytes_t){.data=(const uint8_t[]){%s}, .len=%d})", strings.Join(castParts, ", "), len(t.Items))
	}
	// Numeric arrays
	allNumeric := true
	hasFloat := false
	for _, item := range t.Items {
		switch item.(type) {
		case expr.IntNode:
		case expr.FloatNode:
			hasFloat = true
		default:
			allNumeric = false
		}
	}
	if allNumeric {
		elem := "int64_t"
		if hasFloat {
			elem = "double"
		}
		tag := e.arrayTagFor(elem)
		castParts := make([]string, len(t.Items))
		for i, p := range parts {
			castParts[i] = "(" + elem + ")(" + p + ")"
		}
		name := e.nextAuxLit("arr")
		e.file.source.pf("static const %s %s[] = {%s};", elem, name, strings.Join(castParts, ", "))
		return fmt.Sprintf("(%s){.data=(%s *)%s, .len=%d, .cap=%d}",
			tag, elem, name, len(t.Items), len(t.Items))
	}
	allStrings := true
	for _, item := range t.Items {
		if _, ok := item.(expr.StringNode); !ok {
			allStrings = false
			break
		}
	}
	if allStrings {
		tag := e.arrayTagFor("zb_bytes_t")
		name := e.nextAuxLit("arr")
		e.file.source.pf("zb_bytes_t *%s = (zb_bytes_t *)zb_arena_alloc(%s->_arena, sizeof(zb_bytes_t) * %d);",
			name, e.thisExpr(), len(t.Items))
		for i, p := range parts {
			e.file.source.pf("%s[%d] = zb_bytes_dup(%s->_arena, %s);", name, i, e.thisExpr(), p)
		}
		return fmt.Sprintf("(%s){.data=%s, .len=%d, .cap=%d}", tag, name, len(t.Items), len(t.Items))
	}
	allStreams := true
	for _, item := range t.Items {
		if !e.exprIsStreamPointer(item) {
			allStreams = false
			break
		}
	}
	if allStreams {
		tag := e.arrayTagFor("zb_stream_t *")
		name := e.nextAuxLit("arr")
		e.file.source.pf("zb_stream_t **%s = (zb_stream_t **)zb_arena_alloc(%s->_arena, sizeof(zb_stream_t *) * %d);",
			name, e.thisExpr(), len(t.Items))
		for i, p := range parts {
			e.file.source.pf("%s[%d] = (zb_stream_t *)(%s);", name, i, p)
		}
		return fmt.Sprintf("(%s){.data=%s, .len=%d, .cap=%d}", tag, name, len(t.Items), len(t.Items))
	}
	allStructs := true
	for _, item := range t.Items {
		if !e.exprIsPointer(item) {
			allStructs = false
			break
		}
	}
	if allStructs {
		tag := e.arrayTagFor("void *")
		name := e.nextAuxLit("arr")
		e.file.source.pf("void **%s = (void **)zb_arena_alloc(%s->_arena, sizeof(void *) * %d);",
			name, e.thisExpr(), len(t.Items))
		for i, p := range parts {
			e.file.source.pf("%s[%d] = (void *)(%s);", name, i, p)
		}
		return fmt.Sprintf("(%s){.data=%s, .len=%d, .cap=%d}", tag, name, len(t.Items), len(t.Items))
	}
	return fmt.Sprintf("((void*){%s})", strings.Join(parts, ", "))
}

func (e *Emitter) exprIsStreamPointer(n expr.Node) bool {
	if mn, ok := n.(expr.MemberNode); ok && mn.Property == "_io" {
		return true
	}
	result := engine.ResultTypeOfNode(e.context, n)
	if result == nil {
		return false
	}
	return result.Kind == engine.StreamKind
}

func (e *Emitter) exprScope(t expr.ScopeNode) string {
	v := engine.ResultTypeOfNode(e.context, t)
	if v == nil {
		panic(fmt.Errorf("unresolved scope: %s::%s", t.Operand, t.Type))
	}
	if v.Parent != nil && v.Parent.Kind == engine.EnumValueKind {
		structParent := v.NearestStruct()
		enumVal := v.NearestEnum()
		if enumVal != nil && enumVal.Enum != nil {
			return e.prefix(structParent) + e.typeName(enumVal.Enum.ID) + "__" + e.typeName(v.Parent.EnumValue.ID)
		}
	}
	panic(fmt.Errorf("unresolved scope: %s::%s", t.Operand, t.Type))
}

func (e *Emitter) exprCast(t expr.CastNode) string {
	if strings.HasSuffix(t.TypeName, "[]") {
		if arr, ok := t.Operand.(expr.ArrayNode); ok {
			return e.emitArrayLiteralAsArrayType(t.TypeName[:len(t.TypeName)-2], arr)
		}
	}
	operand := e.exprNode(t.Operand)
	isVoidPtr := false
	if sub, ok := t.Operand.(expr.SubscriptNode); ok {
		if arrRes := engine.ResultTypeOfNode(e.context, sub.A); arrRes != nil {
			switch arrRes.Kind {
			case engine.AttrKind:
				if arrRes.Attr != nil && arrRes.Attr.Repeat != nil && arrRes.Attr.Type.TypeSwitch != nil {
					isVoidPtr = true
				}
			case engine.InstanceKind:
				if arrRes.Instance != nil && arrRes.Instance.Repeat != nil && arrRes.Instance.Type.TypeSwitch != nil {
					isVoidPtr = true
				}
			}
		}
	}
	if op := engine.ResultTypeOfNode(e.context, t.Operand); op != nil {
		if op.Kind == engine.AttrKind && op.Attr != nil && op.Attr.Type.TypeSwitch != nil {
			isVoidPtr = true
		}
		if op.Kind == engine.InstanceKind && op.Instance != nil && op.Instance.Type.TypeSwitch != nil {
			isVoidPtr = true
		}
	}
	switch t.TypeName {
	case "bytes":
		if isVoidPtr {
			return "(*(zb_bytes_t *)(" + operand + "))"
		}
		return operand
	case "str":
		if isVoidPtr {
			return "(*(zb_bytes_t *)(" + operand + "))"
		}
		return operand
	case "u1":
		if isVoidPtr {
			return "(*(uint8_t *)(" + operand + "))"
		}
		return "((uint8_t)(" + operand + "))"
	case "u2":
		if isVoidPtr {
			return "(*(uint16_t *)(" + operand + "))"
		}
		return "((uint16_t)(" + operand + "))"
	case "u4":
		if isVoidPtr {
			return "(*(uint32_t *)(" + operand + "))"
		}
		return "((uint32_t)(" + operand + "))"
	case "u8":
		if isVoidPtr {
			return "(*(uint64_t *)(" + operand + "))"
		}
		return "((uint64_t)(" + operand + "))"
	case "s1":
		if isVoidPtr {
			return "(*(int8_t *)(" + operand + "))"
		}
		return "((int8_t)(" + operand + "))"
	case "s2":
		if isVoidPtr {
			return "(*(int16_t *)(" + operand + "))"
		}
		return "((int16_t)(" + operand + "))"
	case "s4":
		if isVoidPtr {
			return "(*(int32_t *)(" + operand + "))"
		}
		return "((int32_t)(" + operand + "))"
	case "s8":
		if isVoidPtr {
			return "(*(int64_t *)(" + operand + "))"
		}
		return "((int64_t)(" + operand + "))"
	case "f4":
		if isVoidPtr {
			return "(*(float *)(" + operand + "))"
		}
		return "((float)(" + operand + "))"
	case "f8":
		if isVoidPtr {
			return "(*(double *)(" + operand + "))"
		}
		return "((double)(" + operand + "))"
	}
	if strings.HasSuffix(t.TypeName, "[]") {
		return operand
	}
	tn := strings.ReplaceAll(t.TypeName, "::", "__")
	resolved := engine.ResultTypeOfNode(e.context, t)
	if resolved != nil && resolved.Kind == engine.StructKind {
		full := e.prefix(resolved.DefParent) + e.typeName(resolved.Struct.Type.ID)
		return "((" + full + "_t *)(" + operand + "))"
	}
	return "((" + tn + "_t *)(" + operand + "))"
}

func (e *Emitter) exprSizeof(t expr.SizeofNode) string {
	if bits, ok := engine.PrimitiveBitSize(t.TypeName); ok {
		return strconv.FormatInt((bits+7)/8, 10)
	}
	if resolved := e.resolveQualifiedType(t.TypeName); resolved != nil {
		if bits := e.context.ComputeStructBitSize(resolved.Struct.Type); bits >= 0 {
			return strconv.FormatInt((bits+7)/8, 10)
		}
	}
	// Unknown sizeof.
	return "-1"
}

func (e *Emitter) exprBitSizeof(t expr.BitSizeofNode) string {
	if bits, ok := engine.PrimitiveBitSize(t.TypeName); ok {
		return strconv.FormatInt(bits, 10)
	}
	if resolved := e.resolveQualifiedType(t.TypeName); resolved != nil {
		if bits := e.context.ComputeStructBitSize(resolved.Struct.Type); bits >= 0 {
			return strconv.FormatInt(bits, 10)
		}
	}
	// Unknwon bitszieof.
	return "-1"
}

func (e *Emitter) exprCall(t expr.CallNode) string {
	if mn, ok := t.Object.(expr.MemberNode); ok {
		v := engine.ResultTypeOfNode(e.context, t.Object)
		if v != nil && v.Kind == engine.MethodKind {
			operand := e.exprNode(mn.Operand)
			m := v.Method
			switch m.Method {
			case engine.MethodByteArrayToString:
				if len(t.Args) > 0 {
					return fmt.Sprintf("zb_bytes_decode_to(%s->_arena, %s, %s)",
						e.thisExpr(), operand, e.exprNode(t.Args[0]))
				}
				return operand
			case engine.MethodStringToInt:
				if len(t.Args) > 0 {
					return fmt.Sprintf("zb_str_to_i_strict(%s->_arena, %s, (int)(%s), &%s->_inst_err)",
						e.thisExpr(), operand, e.exprNode(t.Args[0]), e.thisExpr())
				}
				return fmt.Sprintf("zb_str_to_i_strict(%s->_arena, %s, 10, &%s->_inst_err)",
					e.thisExpr(), operand, e.thisExpr())
			case engine.MethodIntToString:
				if len(t.Args) > 0 {
					return "zb_int_to_s_base(" + e.thisExpr() + "->_arena, (int64_t)(" + operand + "), (int)(" + e.exprNode(t.Args[0]) + "))"
				}
				return "zb_int_to_s(" + e.thisExpr() + "->_arena, (int64_t)(" + operand + "))"
			case engine.MethodStringSubstring:
				if len(t.Args) >= 2 {
					return fmt.Sprintf("zb_utf8_substring(%s, (size_t)(%s), (size_t)(%s))",
						operand, e.exprNode(t.Args[0]), e.exprNode(t.Args[1]))
				}
				return operand
			}
		}
	}
	panic(fmt.Errorf("unsupported call: %s", t))
}

func (e *Emitter) exprFString(t expr.FStringNode) string {
	name := e.nextAuxLit("fb")
	e.file.source.pf("zb_buf_t %s; zb_buf_init(&%s, %s->_arena);", name, name, e.thisExpr())
	emitLiteral := func(p expr.FStringPart) {
		var sb strings.Builder
		sb.WriteString("zb_buf_append(&")
		sb.WriteString(name)
		sb.WriteString(", (const uint8_t *)\"")
		for _, b := range []byte(p.Literal) {
			switch b {
			case '"', '\\':
				sb.WriteByte('\\')
				sb.WriteByte(b)
			case '\n':
				sb.WriteString("\\n")
			case '\r':
				sb.WriteString("\\r")
			case '\t':
				sb.WriteString("\\t")
			default:
				if b < 0x20 || b >= 0x7f {
					fmt.Fprintf(&sb, "\\x%02x", b)
				} else {
					sb.WriteByte(b)
				}
			}
		}
		fmt.Fprintf(&sb, "\", %d);", len(p.Literal))
		e.file.source.pf("%s", sb.String())
	}
	for _, p := range t.Parts {
		if p.Expr == nil {
			if p.Literal == "" {
				continue
			}
			emitLiteral(p)
			continue
		}
		exprSrc := e.exprNode(p.Expr)
		switch e.exprToBytesKind(p.Expr) {
		case "bytes":
			tmp := e.nextAuxLit("fv")
			e.file.source.pf("zb_bytes_t %s = (%s);", tmp, exprSrc)
			e.file.source.pf("zb_buf_append(&%s, %s.data, %s.len);", name, tmp, tmp)
		case "bool":
			tmp := e.nextAuxLit("fv")
			e.file.source.pf("const char *%s = (%s) ? \"true\" : \"false\";", tmp, exprSrc)
			e.file.source.pf("zb_buf_append(&%s, (const uint8_t *)%s, strlen(%s));", name, tmp, tmp)
		case "float":
			tmpBuf := e.nextAuxLit("fbuf")
			tmpN := e.nextAuxLit("fn")
			e.file.source.pf("char %s[32];", tmpBuf)
			e.file.source.pf("int %s = snprintf(%s, sizeof(%s), \"%%g\", (double)(%s));", tmpN, tmpBuf, tmpBuf, exprSrc)
			e.file.source.pf("if (%s < 0) %s = 0;", tmpN, tmpN)
			e.file.source.pf("zb_buf_append(&%s, (const uint8_t *)%s, (size_t)%s);", name, tmpBuf, tmpN)
		default:
			tmp := e.nextAuxLit("fv")
			e.file.source.pf("zb_bytes_t %s = zb_int_to_s(%s->_arena, (int64_t)(%s));", tmp, e.thisExpr(), exprSrc)
			e.file.source.pf("zb_buf_append(&%s, %s.data, %s.len);", name, tmp, tmp)
		}
	}
	return fmt.Sprintf("zb_buf_to_bytes(&%s)", name)
}

func (e *Emitter) exprToBytesKind(n expr.Node) string {
	if e.isNodeByteArray(n) {
		return "bytes"
	}
	if e.exprIsFloat(n) {
		return "float"
	}
	switch n.(type) {
	case expr.BoolNode:
		return "bool"
	case expr.StringNode, expr.FStringNode:
		return "bytes"
	}
	res := engine.ResultTypeOfNode(e.context, n)
	if res != nil {
		switch res.Kind {
		case engine.BooleanKind:
			return "bool"
		case engine.FloatKind:
			return "float"
		case engine.StringKind, engine.ByteArrayKind:
			return "bytes"
		case engine.IntegerKind, engine.EnumValueKind:
			return "int"
		}
		if vt, ok := res.ValueType(); ok && vt.Type.TypeRef != nil {
			switch vt.Type.TypeRef.Kind {
			case types.Bytes, types.String:
				return "bytes"
			case types.F4, types.F8:
				return "float"
			}
		}
	}
	return "int"
}

func (e *Emitter) stringLiteralBytes(s string) string {
	if len(s) == 0 {
		return "((zb_bytes_t){.data=NULL, .len=0})"
	}
	parts := make([]string, 0, len(s))
	for _, c := range []byte(s) {
		parts = append(parts, fmt.Sprintf("0x%02x", c))
	}
	return fmt.Sprintf("((zb_bytes_t){.data=(const uint8_t[]){%s}, .len=%d})", strings.Join(parts, ", "), len(s))
}

func (e *Emitter) isByteArrayBinary(a, b expr.Node) bool {
	return e.isNodeByteArray(a) && e.isNodeByteArray(b)
}

func (e *Emitter) needsInt64Widening(a, b expr.Node) bool {
	ka := engine.ResultTypeOfNode(e.context, a).TypeKind()
	kb := engine.ResultTypeOfNode(e.context, b).TypeKind()
	hasSigned := isSignedKind(ka) || isSignedKind(kb)
	hasUnsigned := isUnsignedKind(ka) || isUnsignedKind(kb)
	return hasSigned && hasUnsigned
}

func isSignedKind(k types.Kind) bool {
	switch k {
	case types.S1, types.S2, types.S2le, types.S2be,
		types.S4, types.S4le, types.S4be,
		types.S8, types.S8le, types.S8be,
		types.UntypedInt:
		return true
	}
	return false
}

func isUnsignedKind(k types.Kind) bool {
	switch k {
	case types.U1, types.U2, types.U2le, types.U2be,
		types.U4, types.U4le, types.U4be,
		types.U8, types.U8le, types.U8be:
		return true
	}
	return false
}

func (e *Emitter) exprIsRootIO(n expr.Node) bool {
	if id, ok := n.(expr.IdentNode); ok {
		return id.Identifier == "_io"
	}
	return false
}

func (e *Emitter) parentFallbackForCurrent() string {
	if e.currentStruct == nil || e.currentStruct.Struct == nil || e.currentStruct.Struct.Type == nil {
		return ""
	}
	return e.parentFallbackCType(e.currentStruct.Struct.Type)
}

func (e *Emitter) exprIsPointer(n expr.Node) (out bool) {
	defer func() { _ = recover() }()
	result := engine.ResultTypeOfNode(e.context, n)
	if result == nil {
		return false
	}
	if result.Kind == engine.StructKind || result.Kind == engine.StructRootKind {
		return true
	}
	if vt, ok := result.ValueType(); ok && vt.Type.TypeRef != nil && vt.Repeat == nil {
		return vt.Type.TypeRef.Kind == types.User
	}
	return false
}

func (e *Emitter) isNodeByteArray(n expr.Node) bool {
	if a, ok := n.(expr.ArrayNode); ok {
		for _, item := range a.Items {
			if in, ok := item.(expr.IntNode); ok {
				if in.Integer.Sign() < 0 || !in.Integer.IsInt64() || in.Integer.Int64() > 255 {
					return false
				}
			} else {
				return false
			}
		}
		return true
	}
	if _, ok := n.(expr.StringNode); ok {
		return true
	}
	if id, ok := n.(expr.IdentNode); ok && id.Identifier == "_" {
		return e.mode.repeatElemIsBytes
	}
	result := engine.ResultTypeOfNode(e.context, n)
	if result == nil {
		return false
	}
	if result.Kind == engine.ByteArrayKind || result.Kind == engine.StringKind {
		return true
	}
	if result.Kind == engine.InstanceKind && result.Instance != nil && result.Instance.Value != nil {
		owner := result.Parent
		var restore func()
		if owner != nil && (owner.Kind == engine.StructKind || owner.Kind == engine.StructRootKind) {
			restore = e.enterLocal(owner)
		}
		t := e.inferValueType(result.Instance.Value)
		if restore != nil {
			restore()
		}
		return t == "zb_bytes_t"
	}
	if bn, ok := n.(expr.BinaryNode); ok {
		switch bn.Op {
		case expr.OpSub, expr.OpMult, expr.OpDiv, expr.OpMod,
			expr.OpBitAnd, expr.OpBitOr, expr.OpBitXor,
			expr.OpShiftLeft, expr.OpShiftRight:
			return false
		case expr.OpAdd:
			return e.isNodeByteArray(bn.A) && e.isNodeByteArray(bn.B)
		}
	}
	if vt, ok := result.ValueType(); ok && vt.Type.TypeRef != nil && vt.Repeat == nil {
		k := vt.Type.TypeRef.Kind
		if k == types.Bytes || k == types.String {
			return true
		}
	}
	if result.Kind == engine.MethodKind && result.Method != nil {
		ret := result.Method.ReturnType
		if ret.Type.TypeRef != nil {
			k := ret.Type.TypeRef.Kind
			if k == types.Bytes || k == types.String {
				return true
			}
		}
	}
	return false
}

func (e *Emitter) resolveQualifiedType(name string) *engine.ExprValue {
	defer func() { _ = recover() }()
	resolved := e.context.ResolveQualifiedType(name)
	if resolved == nil || resolved.Kind != engine.StructKind || resolved.Struct == nil {
		return nil
	}
	return resolved
}
