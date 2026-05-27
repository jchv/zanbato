package c

import (
	"fmt"
	"sort"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/types"
)

func uniformSwitchCase(sw *types.TypeSwitch) *types.TypeRef {
	var first *types.TypeRef
	for _, c := range sw.Cases {
		cc := c
		if first == nil {
			first = &cc
			continue
		}
		if !sameTypeRef(first, &cc) {
			return nil
		}
	}
	return first
}

func integerWidenedSwitchCase(sw *types.TypeSwitch) *types.TypeRef {
	if sw == nil || len(sw.Cases) == 0 {
		return nil
	}
	allUnsigned := true
	for _, c := range sw.Cases {
		switch c.Kind {
		case types.U1,
			types.U2, types.U2le, types.U2be,
			types.U4, types.U4le, types.U4be,
			types.U8, types.U8le, types.U8be:
		case types.S1,
			types.S2, types.S2le, types.S2be,
			types.S4, types.S4le, types.S4be,
			types.S8, types.S8le, types.S8be:
			allUnsigned = false
		default:
			return nil
		}
	}
	if allUnsigned {
		return &types.TypeRef{Kind: types.U8}
	}
	return &types.TypeRef{Kind: types.S8}
}

func sameTypeRef(a, b *types.TypeRef) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind != b.Kind {
		return false
	}
	if a.Kind == types.User {
		if a.User == nil || b.User == nil {
			return a.User == b.User
		}
		return a.User.Name == b.User.Name
	}
	return true
}

func (e *Emitter) emitRepeatTypeSwitchRead(src *buf, parent *engine.ExprValue, a *kaitai.Attr, sw *types.TypeSwitch, field string) {
	src.pf("{")
	src.indent()
	src.p("size_t _i;")
	src.p("(void)_i;")
	switch r := a.Repeat.(type) {
	case types.RepeatExpr:
		src.pf("size_t _count = (size_t)(%s);", e.expr(r.CountExpr))
		src.pf("if (_count > 0) {")
		src.indent()
		src.pf("if (zb_array_grow_impl(arena, (void **)&this_->%s.data, &this_->%s.cap, _count, sizeof(*this_->%s.data)) != ZB_OK) return ZB_ERR_ALLOC;",
			field, field, field)
		src.unindent()
		src.pf("}")
		src.pf("for (_i = 0; _i < _count; _i++) {")
	case types.RepeatEOS:
		src.pf("for (_i = 0; !zb_stream_eof(stream); _i++) {")
		src.indent().pf("if (zb_array_grow_impl(arena, (void **)&this_->%s.data, &this_->%s.cap, this_->%s.len+1, sizeof(*this_->%s.data)) != ZB_OK) return ZB_ERR_ALLOC;",
			field, field, field, field).unindent()
	case types.RepeatUntil:
		src.pf("_i = 0;")
		src.pf("for (;;) {")
		src.indent().pf("if (zb_array_grow_impl(arena, (void **)&this_->%s.data, &this_->%s.cap, this_->%s.len+1, sizeof(*this_->%s.data)) != ZB_OK) return ZB_ERR_ALLOC;",
			field, field, field, field).unindent()
		_ = r
	}
	src.indent()
	ksID := string(a.ID)
	e.debugArrElemStart(src, ksID)
	src.pf("void *_elem = NULL;")
	hasSize := a.Size != nil
	if hasSize {
		src.pf("{")
		src.indent()
		src.pf("size_t _sub_n = (size_t)(%s);", e.expr(a.Size))
		src.pf("zb_bytes_t _raw;")
		emitTry(src, "zb_read_bytes(stream, arena, _sub_n, &_raw)")
		emitSubstreamMemBytes(src, "_sub_e", "_raw")
		src.pf("zb_stream_t *stream = _sub_e; (void)stream;")
	}
	e.emitTypeSwitchReadInto(src, parent, a, sw, "_elem")
	if hasSize {
		src.pf("if (!_elem) {")
		src.indent()
		emitBytesBoxInto(src, "_elem", "_raw")
		src.unindent()
		src.pf("}")
		src.unindent()
		src.pf("}")
	}
	src.pf("this_->%s.data[this_->%s.len++] = _elem;", field, field)
	e.debugArrElemEnd(src, ksID)
	if r, ok := a.Repeat.(types.RepeatUntil); ok {
		src.pf("void *_repeat_elem = this_->%s.data[this_->%s.len-1];", field, field)
		src.pf("(void)_repeat_elem;")
		src.pf("_i++;")
		src.pf("if (%s) break;", e.repeatUntilExpr(r.UntilExpr, field))
	}
	src.unindent()
	src.pf("}")
	src.unindent()
	src.pf("}")
}

func (e *Emitter) emitTypeSwitchReadInto(src *buf, parent *engine.ExprValue, a *kaitai.Attr, sw *types.TypeSwitch, lhs string) {
	_ = parent
	keys := make([]string, 0, len(sw.Cases))
	for k := range sw.Cases {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	defaultRef, hasDefault := sw.Cases["_"]
	bytesDiscrim := e.exprIsByteKind(sw.SwitchOn.Root)
	discrim := e.expr(sw.SwitchOn)
	src.pf("{")
	src.indent()
	if bytesDiscrim {
		src.pf("zb_bytes_t _disc = (%s);", discrim)
	} else {
		src.pf("uint64_t _disc = (uint64_t)(%s);", discrim)
	}
	wroteAny := false
	for _, k := range keys {
		if k == "_" {
			continue
		}
		caseVal := e.typeSwitchCaseValue(k)
		var cmp string
		if bytesDiscrim {
			cmp = fmt.Sprintf("zb_bytes_equal(_disc, (%s))", caseVal)
		} else {
			cmp = fmt.Sprintf("_disc == (uint64_t)(%s)", caseVal)
		}
		if !wroteAny {
			src.pf("if (%s) {", cmp)
		} else {
			src.pf("} else if (%s) {", cmp)
		}
		src.indent()
		caseRef := sw.Cases[k]
		e.emitTypeSwitchCaseInto(src, &caseRef, a, lhs)
		src.unindent()
		wroteAny = true
	}
	if hasDefault {
		if wroteAny {
			src.pf("} else {")
		} else {
			src.pf("{")
		}
		src.indent()
		e.emitTypeSwitchCaseInto(src, &defaultRef, a, lhs)
		src.unindent()
		src.pf("}")
	} else if wroteAny {
		src.pf("}")
	}
	src.unindent()
	src.pf("}")
}

func (e *Emitter) emitTypeSwitchCaseInto(src *buf, t *types.TypeRef, a *kaitai.Attr, lhs string) {
	switch t.Kind {
	case types.U1, types.S1,
		types.U2le, types.U2be, types.U4le, types.U4be, types.U8le, types.U8be,
		types.S2le, types.S2be, types.S4le, types.S4be, types.S8le, types.S8be,
		types.F4le, types.F4be, types.F8le, types.F8be:
		ctyp := e.declTypeRef(t, false)
		src.pf("%s *_tmp = (%s *)zb_arena_alloc(arena, sizeof(%s));", ctyp, ctyp, ctyp)
		src.pf("if (!_tmp) return ZB_ERR_ALLOC;")
		emitTry(src, "%s(stream, _tmp)", readCallForKind(t.Kind))
		src.pf("%s = _tmp;", lhs)
	case types.User:
		userT := t.User
		typ := e.tryResolveType(userT.Name)
		if typ == nil || typ.Struct == nil {
			panic(fmt.Errorf("unresolved user type %s", userT.Name))
		}
		typeName := e.prefix(typ.DefParent) + e.typeName(typ.Struct.Type.ID)
		paramArgs := e.userParamArgs(typ.Struct.Type, userT)
		pExpr, rExpr := e.userTypeParentRootArgs(typ, "this_")
		if userT.Size != nil {
			src.pf("{")
			src.indent()
			src.pf("size_t _sub_n = (size_t)(%s);", e.expr(userT.Size))
			emitSubstreamConsume(src, "_sub", "stream", "_sub_n")
			emitArenaNewLocal(src, "_u", typeName)
			emitTry(src, "%s_read(_u, _sub, arena, %s, %s%s)", typeName, pExpr, rExpr, paramArgs)
			src.pf("%s = _u;", lhs)
			src.unindent()
			src.pf("}")
			return
		}
		emitArenaNewLocal(src, "_u", typeName)
		emitTry(src, "%s_read(_u, stream, arena, %s, %s%s)", typeName, pExpr, rExpr, paramArgs)
		src.pf("%s = _u;", lhs)
	case types.Bytes, types.String:
		_ = a
		emitArenaAllocLocal(src, "_b", "zb_bytes_t")
		e.emitBytesReadInto(src, t, "(*_b)")
		src.pf("%s = _b;", lhs)
	default:
		panic(fmt.Errorf("unsupported switch case type %s", t.Kind.String()))
	}
}

func (e *Emitter) emitIntegerSwitchRead(src *buf, sw *types.TypeSwitch, field string, widened *types.TypeRef) {
	keys := make([]string, 0, len(sw.Cases))
	for k := range sw.Cases {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	_, hasDefault := sw.Cases["_"]
	bytesDiscrim := e.exprIsByteKind(sw.SwitchOn.Root)
	discrim := e.expr(sw.SwitchOn)
	storageType := e.declTypeRef(widened, false)
	src.pf("{")
	src.indent()
	if bytesDiscrim {
		src.pf("zb_bytes_t _disc = (%s);", discrim)
	} else {
		src.pf("uint64_t _disc = (uint64_t)(%s);", discrim)
	}
	wroteAny := false
	for _, k := range keys {
		if k == "_" {
			continue
		}
		caseRef := sw.Cases[k]
		caseVal := e.typeSwitchCaseValue(k)
		var cmp string
		if bytesDiscrim {
			cmp = fmt.Sprintf("zb_bytes_equal(_disc, (%s))", caseVal)
		} else {
			cmp = fmt.Sprintf("_disc == (uint64_t)(%s)", caseVal)
		}
		if !wroteAny {
			src.pf("if (%s) {", cmp)
		} else {
			src.pf("} else if (%s) {", cmp)
		}
		src.indent()
		caseC := e.declTypeRef(&caseRef, false)
		src.pf("%s _v;", caseC)
		emitTry(src, "%s(stream, &_v)", readCallForKind(caseRef.Kind))
		src.pf("this_->%s = (%s)_v;", field, storageType)
		src.unindent()
		wroteAny = true
	}
	if hasDefault {
		defRef := sw.Cases["_"]
		if wroteAny {
			src.pf("} else {")
		} else {
			src.pf("{")
		}
		src.indent()
		caseC := e.declTypeRef(&defRef, false)
		src.pf("%s _v;", caseC)
		emitTry(src, "%s(stream, &_v)", readCallForKind(defRef.Kind))
		src.pf("this_->%s = (%s)_v;", field, storageType)
		src.unindent()
		src.pf("}")
	} else if wroteAny {
		src.pf("}")
	}
	src.unindent()
	src.pf("}")
}

func (e *Emitter) emitTypeSwitchRead(src *buf, parent *engine.ExprValue, a *kaitai.Attr, sw *types.TypeSwitch, field string) {
	_ = parent
	if uni := uniformSwitchCase(sw); uni != nil {
		e.emitSingleRead(src, uni, field, a)
		return
	}
	if widened := integerWidenedSwitchCase(sw); widened != nil {
		e.emitIntegerSwitchRead(src, sw, field, widened)
		return
	}
	if a != nil && a.SizeEos {
		src.pf("{")
		src.indent()
		src.pf("zb_bytes_t _raw;")
		emitTry(src, "zb_read_bytes_full(stream, arena, &_raw)")
		if e.needsSwitchRawShadowField(a) {
			src.pf("this_->_raw_%s = _raw;", field)
		}
		emitSubstreamMemBytes(src, "_sub", "_raw")
		src.pf("{")
		src.indent()
		src.pf("zb_stream_t *stream = _sub; (void)stream;")
		e.emitTypeSwitchReadInto(src, parent, a, sw, fmt.Sprintf("this_->%s", field))
		src.unindent()
		src.pf("}")
		src.pf("if (!this_->%s) {", field)
		src.indent()
		emitBytesBoxInto(src, fmt.Sprintf("this_->%s", field), "_raw")
		src.unindent()
		src.pf("}")
		src.unindent()
		src.pf("}")
		return
	}
	if a != nil && a.Size != nil {
		src.pf("{")
		src.indent()
		src.pf("size_t _sub_n = (size_t)(%s);", e.expr(a.Size))
		src.pf("zb_bytes_t _raw;")
		emitTry(src, "zb_read_bytes(stream, arena, _sub_n, &_raw)")
		if e.needsSwitchRawShadowField(a) {
			src.pf("this_->_raw_%s = _raw;", field)
		}
		emitSubstreamMemBytes(src, "_sub", "_raw")
		src.pf("{")
		src.indent()
		src.pf("zb_stream_t *stream = _sub; (void)stream;")
		e.emitTypeSwitchReadInto(src, parent, a, sw, fmt.Sprintf("this_->%s", field))
		src.unindent()
		src.pf("}")
		src.pf("if (!this_->%s) {", field)
		src.indent()
		emitBytesBoxInto(src, fmt.Sprintf("this_->%s", field), "_raw")
		src.unindent()
		src.pf("}")
		src.unindent()
		src.pf("}")
		return
	}
	src.pf("{")
	src.indent()
	keys := make([]string, 0, len(sw.Cases))
	for k := range sw.Cases {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	defaultRef, hasDefault := sw.Cases["_"]

	bytesDiscrim := e.exprIsByteKind(sw.SwitchOn.Root)
	discrim := e.expr(sw.SwitchOn)

	if bytesDiscrim {
		src.pf("zb_bytes_t _disc = (%s);", discrim)
		wroteAny := false
		for _, k := range keys {
			if k == "_" {
				continue
			}
			caseVal := e.typeSwitchCaseValue(k)
			if !wroteAny {
				src.pf("if (zb_bytes_equal(_disc, (%s))) {", caseVal)
			} else {
				src.pf("} else if (zb_bytes_equal(_disc, (%s))) {", caseVal)
			}
			src.indent()
			caseRef := sw.Cases[k]
			e.emitSingleReadInto(src, &caseRef, field, a, true)
			src.unindent()
			wroteAny = true
		}
		if hasDefault {
			if wroteAny {
				src.pf("} else {")
			} else {
				src.pf("{")
			}
			src.indent()
			e.emitSingleReadInto(src, &defaultRef, field, a, true)
			src.unindent()
			src.pf("}")
		} else if wroteAny {
			src.pf("}")
		}
		src.unindent()
		src.pf("}")
		return
	}

	src.pf("uint64_t _disc = (uint64_t)(%s);", discrim)
	wroteAny := false
	for _, k := range keys {
		if k == "_" {
			continue
		}
		caseVal := e.typeSwitchCaseValue(k)
		if !wroteAny {
			src.pf("if (_disc == (uint64_t)(%s)) {", caseVal)
		} else {
			src.pf("} else if (_disc == (uint64_t)(%s)) {", caseVal)
		}
		src.indent()
		caseRef := sw.Cases[k]
		e.emitSingleReadInto(src, &caseRef, field, a, true)
		src.unindent()
		wroteAny = true
	}
	if hasDefault {
		if wroteAny {
			src.pf("} else {")
		} else {
			src.pf("{")
		}
		src.indent()
		e.emitSingleReadInto(src, &defaultRef, field, a, true)
		src.unindent()
		src.pf("}")
	} else if wroteAny {
		src.pf("}")
	}
	src.unindent()
	src.pf("}")
}

func (e *Emitter) typeSwitchCaseValue(s string) string {
	if s == "_" {
		return "0"
	}
	ex, err := expr.ParseExpr(s)
	if err != nil {
		panic(err)
	}
	return e.expr(ex)
}
