package c

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/types"
)

func (e *Emitter) emitWriteFunc(val *engine.ExprValue) {
	ks := val.Struct.Type
	name := e.prefix(val.DefParent) + e.typeName(ks.ID)
	defer e.enterStruct(val, name)()
	defer e.saveExprMode()()
	e.mode.writingContext = true

	params := fmt.Sprintf("const struct %s *this_, zb_writer_t *wstream", name)
	decl := fmt.Sprintf("int %s_write(%s)", name, params)
	e.file.header.pf("%s;", decl)

	src := buf{}
	restoreSrc := e.redirectSource(&src)
	src.indent()
	src.p("(void)this_; (void)wstream;")
	src.p("int _endian = this_->_endian; (void)_endian;")
	if ks.Meta.Endian.Kind == types.SwitchEndian && ks.Meta.Endian.SwitchOn != nil {
		sw := ks.Meta.Endian
		keys := make([]string, 0, len(sw.Cases))
		for k := range sw.Cases {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		defaultEnd, hasDefault := sw.Cases["_"]
		bytesDiscrim := e.exprIsByteKind(sw.SwitchOn.Root)
		discrim := e.expr(sw.SwitchOn)
		if bytesDiscrim {
			src.pf("zb_bytes_t _disc_e = (%s); (void)_disc_e;", discrim)
		} else {
			src.pf("uint64_t _disc_e = (uint64_t)(%s); (void)_disc_e;", discrim)
		}
		wrote := false
		for _, k := range keys {
			if k == "_" {
				continue
			}
			caseVal := e.typeSwitchCaseValue(k)
			var cmp string
			if bytesDiscrim {
				cmp = fmt.Sprintf("zb_bytes_equal(_disc_e, (%s))", caseVal)
			} else {
				cmp = fmt.Sprintf("_disc_e == (uint64_t)(%s)", caseVal)
			}
			endianBit := 0
			if sw.Cases[k] == types.BigEndian {
				endianBit = 1
			}
			if !wrote {
				src.pf("if (%s) _endian = %d;", cmp, endianBit)
			} else {
				src.pf("else if (%s) _endian = %d;", cmp, endianBit)
			}
			wrote = true
		}
		if hasDefault {
			endianBit := 0
			if defaultEnd == types.BigEndian {
				endianBit = 1
			}
			if wrote {
				src.pf("else _endian = %d;", endianBit)
			} else {
				src.pf("_endian = %d;", endianBit)
			}
		}
	}
	alignSlots := engine.BitAlignSlots(ks)
	for i, attr := range val.Struct.Attrs {
		for len(alignSlots) > 0 && alignSlots[0].BeforeAttr == i {
			e.emitAlignWrite(&src, alignSlots[0], ks)
			alignSlots = alignSlots[1:]
		}
		e.emitAttrWrite(&src, val, attr.Attr)
	}
	for len(alignSlots) > 0 {
		e.emitAlignWrite(&src, alignSlots[0], ks)
		alignSlots = alignSlots[1:]
	}
	src.p("return ZB_OK;")
	restoreSrc()
	e.file.source.raw(decl + " {\n")
	if bodyReferencesErr(src.String()) {
		e.file.source.indent().p("int _err; (void)_err;").unindent()
	}
	e.file.source.raw(src.String())
	e.file.source.p("}")
	e.file.source.blank()
}

func (e *Emitter) emitAlignWrite(src *buf, slot engine.BitAlignSlot, ks *kaitai.Struct) {
	endian := e.alignReadEndian(ks, slot.BeforeAttr)
	emitTry(src, "zb_write_bits_%s(wstream, %d, this_->%s)", endian, slot.Width, bitAlignFieldName(slot))
}

func (e *Emitter) emitAttrWrite(src *buf, parent *engine.ExprValue, a *kaitai.Attr) {
	if a.If != nil {
		src.pf("if (this_->_have_%s) {", e.fieldName(a.ID))
		src.indent()
		defer func() {
			src.unindent()
			src.pf("}")
		}()
	}

	if a.Contents != nil {
		emit := func() {
			src.pf("/* contents: %q */", string(a.Contents))
			src.pf("{")
			src.indent()
			src.pf("static const uint8_t _expected[] = {%s};", byteArrayInitializer(a.Contents))
			emitTry(src, "zb_write_bytes_raw(wstream, _expected, %d)", len(a.Contents))
			src.unindent()
			src.pf("}")
		}
		if a.Repeat == nil {
			emit()
			return
		}
		src.pf("for (size_t _i = 0; _i < this_->%s.len; _i++) {", e.fieldName(a.ID))
		src.indent()
		emit()
		src.unindent()
		src.pf("}")
		return
	}

	rt := a.Type.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)
	field := e.fieldName(a.ID)
	if rt.TypeRef == nil && rt.TypeSwitch != nil {
		if a.Repeat != nil {
			e.emitRepeatTypeSwitchWrite(src, parent, a, rt.TypeSwitch, field)
		} else {
			e.emitTypeSwitchWrite(src, parent, a, rt.TypeSwitch, field)
		}
		return
	}
	if rt.TypeRef == nil {
		panic(fmt.Errorf("unsupported attr: %s", a.ID))
	}
	if a.Repeat != nil {
		e.emitRepeatWrite(src, parent, a, rt.TypeRef, field)
		return
	}
	e.emitSingleWrite(src, rt.TypeRef, field, a, "this_->"+field)
}

func (e *Emitter) emitSingleWrite(src *buf, t *types.TypeRef, field string, a *kaitai.Attr, lvalue string) {
	switch t.Kind {
	case types.U2, types.U4, types.U8, types.S2, types.S4, types.S8, types.F4, types.F8:
		leKind, beKind := t.Kind.SplitEndian()
		emitTry(src, "_endian ? %s(wstream, %s) : %s(wstream, %s)",
			writeCallForKind(beKind), lvalue, writeCallForKind(leKind), lvalue)
	case types.U1, types.U2le, types.U2be, types.U4le, types.U4be, types.U8le, types.U8be,
		types.S1, types.S2le, types.S2be, types.S4le, types.S4be, types.S8le, types.S8be,
		types.F4le, types.F4be, types.F8le, types.F8be:
		emitTry(src, "%s(wstream, %s)", writeCallForKind(t.Kind), lvalue)
	case types.Bits:
		if t.Bits != nil {
			endian := "be"
			if t.Bits.Endian.Kind == types.LittleBitEndian {
				endian = "le"
			}
			emitTry(src, "zb_write_bits_%s(wstream, %d, (uint64_t)(%s))", endian, t.Bits.Width, lvalue)
		}
	case types.User:
		userT := t.User
		typ := e.tryResolveType(userT.Name)
		if typ == nil || typ.Struct == nil {
			panic(fmt.Errorf("unresolved user type %s", userT.Name))
		}
		typeName := e.prefix(typ.DefParent) + e.typeName(typ.Struct.Type.ID)
		if userT.Size != nil && a != nil && a.Process != nil {
			src.pf("if (%s) {", lvalue)
			src.indent()
			src.pf("zb_writer_t _w_inner; zb_writer_init(&_w_inner, this_->_arena);")
			emitTry(src, "%s_write(%s, &_w_inner)", typeName, lvalue)
			src.pf("zb_bytes_t _p = zb_writer_bytes(&_w_inner);")
			prevOffset := e.mode.writerPosOffset
			e.mode.writerPosOffset = fmt.Sprintf("(size_t)(%s)", e.expr(userT.Size))
			e.emitUnprocess(src, a.Process, "_p")
			e.mode.writerPosOffset = prevOffset
			term := -1
			pad := -1
			if a.Terminator != nil {
				term = *a.Terminator
			}
			if a.PadRight != nil {
				pad = *a.PadRight
			}
			if e.needsRawShadowField(a) && (term >= 0 || pad >= 0) {
				src.pf("size_t _expected = (size_t)(%s);", e.expr(userT.Size))
				src.pf("if (_p.len > _expected) _p.len = _expected;")
				src.pf("zb_bytes_t _trail = this_->_raw_%s;", field)
				src.pf("size_t _filled = 0;")
				src.pf("if (_p.len > 0) { ZB_TRY(zb_write_bytes_raw(wstream, _p.data, _p.len)); _filled = _p.len; }")
				includeOn := false
				if a.Include != nil {
					includeOn = *a.Include
				}
				if term >= 0 && !includeOn {
					src.pf("if (_filled < _expected) { ZB_TRY(zb_write_u1(wstream, %d)); _filled++; }", term)
				}
				src.pf("if (_trail.len > _filled && _filled < _expected) {")
				src.indent()
				src.pf("size_t _take = _trail.len - _filled;")
				src.pf("if (_take > _expected - _filled) _take = _expected - _filled;")
				emitTry(src, "zb_write_bytes_raw(wstream, _trail.data + _filled, _take)")
				src.pf("_filled += _take;")
				src.unindent()
				src.pf("}")
				padByte := max(pad, 0)
				src.pf("while (_filled < _expected) { ZB_TRY(zb_write_u1(wstream, %d)); _filled++; }", padByte)
			} else if term >= 0 || pad >= 0 {
				src.pf("ZB_TRY(zb_write_bytes_limit(wstream, _p, (size_t)(%s), %d, %d));",
					e.expr(userT.Size), term, pad)
			} else {
				emitTry(src, "zb_write_bytes(wstream, _p)")
			}
			src.unindent()
			src.pf("}")
			return
		}
		if userT.Size != nil {
			term := -1
			pad := -1
			if a != nil {
				if a.Terminator != nil {
					term = *a.Terminator
				}
				if a.PadRight != nil {
					pad = *a.PadRight
				}
			}
			src.pf("if (%s) {", lvalue)
			src.indent()
			src.pf("zb_writer_t _w_inner; zb_writer_init(&_w_inner, this_->_arena);")
			emitTry(src, "%s_write(%s, &_w_inner)", typeName, lvalue)
			src.pf("zb_bytes_t _p = zb_writer_bytes(&_w_inner);")
			if e.needsRawShadowField(a) {
				src.pf("size_t _expected = (size_t)(%s);", e.expr(userT.Size))
				src.pf("if (_p.len > _expected) _p.len = _expected;")
				src.pf("zb_bytes_t _trail = this_->_raw_%s;", field)
				src.pf("size_t _filled = 0;")
				src.pf("if (_p.len > 0) { ZB_TRY(zb_write_bytes_raw(wstream, _p.data, _p.len)); _filled = _p.len; }")
				includeOn := false
				if a != nil && a.Include != nil {
					includeOn = *a.Include
				}
				if term >= 0 && !includeOn {
					src.pf("if (_filled < _expected) { ZB_TRY(zb_write_u1(wstream, %d)); _filled++; }", term)
				}
				src.pf("if (_trail.len > _filled && _filled < _expected) {")
				src.indent()
				src.pf("size_t _take = _trail.len - _filled;")
				src.pf("if (_take > _expected - _filled) _take = _expected - _filled;")
				emitTry(src, "zb_write_bytes_raw(wstream, _trail.data + _filled, _take)")
				src.pf("_filled += _take;")
				src.unindent()
				src.pf("}")
				padByte := max(pad, 0)
				src.pf("while (_filled < _expected) { ZB_TRY(zb_write_u1(wstream, %d)); _filled++; }", padByte)
			} else {
				src.pf("ZB_TRY(zb_write_bytes_limit(wstream, _p, (size_t)(%s), %d, %d));",
					e.expr(userT.Size), term, pad)
			}
			src.unindent()
			src.pf("}")
			return
		}
		if userT.Size == nil && a != nil && a.Terminator != nil {
			term := *a.Terminator
			include := false
			if a.Include != nil {
				include = *a.Include
			}
			consume := true
			if a.Consume != nil {
				consume = *a.Consume
			}
			src.pf("if (%s) {", lvalue)
			src.indent()
			if a.Process != nil {
				src.pf("zb_writer_t _w_inner; zb_writer_init(&_w_inner, this_->_arena);")
				emitTry(src, "%s_write(%s, &_w_inner)", typeName, lvalue)
				src.pf("zb_bytes_t _p = zb_writer_bytes(&_w_inner);")
				e.emitUnprocess(src, a.Process, "_p")
				emitTry(src, "zb_write_bytes(wstream, _p)")
			} else {
				emitTry(src, "%s_write(%s, wstream)", typeName, lvalue)
			}
			if !include && consume {
				emitTry(src, "zb_write_u1(wstream, (uint8_t)(%d))", term)
			}
			src.unindent()
			src.pf("}")
			return
		}
		src.pf("if (%s) {", lvalue)
		src.indent()
		emitTry(src, "%s_write(%s, wstream)", typeName, lvalue)
		src.unindent()
		src.pf("}")
	case types.Bytes:
		if a != nil && a.Process != nil {
			src.pf("{")
			src.indent()
			src.pf("zb_bytes_t _p = %s;", lvalue)
			prevOffset := e.mode.writerPosOffset
			e.mode.writerPosOffset = fmt.Sprintf("(%s).len", lvalue)
			e.emitUnprocess(src, a.Process, "_p")
			e.mode.writerPosOffset = prevOffset
			prevField := e.mode.bytesShadowField
			e.mode.bytesShadowField = strings.TrimPrefix(lvalue, "this_->")
			e.emitBytesWrite(src, t, "_p", a)
			e.mode.bytesShadowField = prevField
			src.unindent()
			src.pf("}")
		} else {
			e.emitBytesWrite(src, t, lvalue, a)
		}
	case types.String:
		if a != nil && a.Process != nil {
			src.pf("{")
			src.indent()
			src.pf("zb_bytes_t _p = %s;", lvalue)
			prevOffset := e.mode.writerPosOffset
			e.mode.writerPosOffset = fmt.Sprintf("(%s).len", lvalue)
			e.emitUnprocess(src, a.Process, "_p")
			e.mode.writerPosOffset = prevOffset
			prevField := e.mode.bytesShadowField
			e.mode.bytesShadowField = strings.TrimPrefix(lvalue, "this_->")
			e.emitBytesWrite(src, t, "_p", a)
			e.mode.bytesShadowField = prevField
			src.unindent()
			src.pf("}")
		} else {
			e.emitBytesWrite(src, t, lvalue, a)
		}
	default:
		panic(fmt.Errorf("unsupported write type %s for %s", t.Kind.String(), field))
	}
}

func (e *Emitter) emitBytesWrite(src *buf, t *types.TypeRef, lvalue string, a *kaitai.Attr) {
	var size *expr.Expr
	term := -1
	pad := -1
	include := false
	consume := true
	encoding := ""
	if t.Kind == types.Bytes && t.Bytes != nil {
		size = t.Bytes.Size
		term = t.Bytes.Terminator
		pad = t.Bytes.PadRight
		include = t.Bytes.Include
		consume = t.Bytes.Consume
	} else if t.Kind == types.String && t.String != nil {
		size = t.String.Size
		term = t.String.Terminator
		pad = t.String.PadRight
		include = t.String.Include
		consume = t.String.Consume
		encoding = t.String.Encoding
	}
	if t.Kind == types.String && e.encodingNeedsConversion(encoding) {
		if e.mode.bytesShadowField == "" && strings.HasPrefix(lvalue, "this_->") {
			prev := e.mode.bytesShadowField
			e.mode.bytesShadowField = strings.TrimPrefix(lvalue, "this_->")
			defer func() { e.mode.bytesShadowField = prev }()
		}
		src.pf("{")
		src.indent()
		src.pf("zb_bytes_t _enc;")
		emitTry(src, "zb_bytes_encode(this_->_arena, %s, %q, &_enc)", lvalue, encoding)
		defer func() {
			src.unindent()
			src.pf("}")
		}()
		lvalue = "_enc"
	}
	if size != nil && (term >= 0 || pad >= 0) {
		field := ""
		if e.mode.bytesShadowField != "" {
			field = e.mode.bytesShadowField
		} else if lvalue != "_p" && lvalue != "_enc" {
			field = strings.TrimPrefix(lvalue, "this_->")
		}
		if e.needsRawBytesShadowField(a) && field != "" {
			src.pf("{")
			src.indent()
			src.pf("size_t _expected = (size_t)(%s);", e.expr(size))
			src.pf("zb_bytes_t _v = %s;", lvalue)
			shadowExpr := e.mode.bytesShadowExpr
			if shadowExpr == "" {
				shadowExpr = "this_->_raw_" + field
			}
			src.pf("zb_bytes_t _r = %s;", shadowExpr)
			src.pf("size_t _vlen = _v.len > _expected ? _expected : _v.len;")
			src.pf("if (_vlen) { ZB_TRY(zb_write_bytes_raw(wstream, _v.data, _vlen)); }")
			src.pf("size_t _filled = _vlen;")
			src.pf("if (_r.len >= _expected) {")
			src.indent()
			src.pf("if (_filled < _expected) {")
			src.indent()
			emitTry(src, "zb_write_bytes_raw(wstream, _r.data + _filled, _expected - _filled)")
			src.pf("_filled = _expected;")
			src.unindent()
			src.pf("}")
			src.unindent()
			src.pf("} else {")
			src.indent()
			if !include && term >= 0 {
				src.pf("if (_filled < _expected) { ZB_TRY(zb_write_u1(wstream, %d)); _filled++; }", term)
			}
			src.pf("if (_r.len > _filled && _filled < _expected) {")
			src.indent()
			src.pf("size_t _take = _r.len - _filled;")
			src.pf("if (_take > _expected - _filled) _take = _expected - _filled;")
			emitTry(src, "zb_write_bytes_raw(wstream, _r.data + _filled, _take)")
			src.pf("_filled += _take;")
			src.unindent()
			src.pf("}")
			padByte := max(pad, 0)
			src.pf("while (_filled < _expected) { ZB_TRY(zb_write_u1(wstream, %d)); _filled++; }", padByte)
			src.unindent()
			src.pf("}")
			src.unindent()
			src.pf("}")
			return
		}
		writeTerm := term
		if include {
			writeTerm = -1
		}
		src.pf("ZB_TRY(zb_write_bytes_limit(wstream, %s, (size_t)(%s), %d, %d));",
			lvalue, e.expr(size), writeTerm, pad)
		return
	}
	if size != nil {
		emitTry(src, "zb_write_bytes(wstream, %s)", lvalue)
		return
	}
	if term >= 0 {
		emitTry(src, "zb_write_bytes(wstream, %s)", lvalue)
		if consume && !include {
			unit := 1
			if t.Kind == types.String {
				unit = multiByteUnit(encoding)
			}
			if unit <= 1 {
				emitTry(src, "zb_write_u1(wstream, %d)", term)
			} else {
				emitTry(src, "zb_write_u1(wstream, %d)", term)
				for i := 1; i < unit; i++ {
					emitTry(src, "zb_write_u1(wstream, 0)")
				}
			}
		}
		return
	}
	emitTry(src, "zb_write_bytes(wstream, %s)", lvalue)
}

func (e *Emitter) emitRepeatWrite(src *buf, _ *engine.ExprValue, a *kaitai.Attr, t *types.TypeRef, field string) {
	src.pf("for (size_t _i = 0; _i < this_->%s.len; _i++) {", field)
	src.indent()
	switch t.Kind {
	case types.User:
		userT := t.User
		typ := e.tryResolveType(userT.Name)
		if typ != nil && typ.Struct != nil {
			typeName := e.prefix(typ.DefParent) + e.typeName(typ.Struct.Type.ID)
			src.pf("if (this_->%s.data[_i]) {", field)
			src.indent()
			switch {
			case userT.Size != nil && a != nil && a.Process != nil:
				src.pf("zb_writer_t _w_inner; zb_writer_init(&_w_inner, this_->_arena);")
				emitTry(src, "%s_write(this_->%s.data[_i], &_w_inner)", typeName, field)
				src.pf("zb_bytes_t _p = zb_writer_bytes(&_w_inner);")
				prevOffset := e.mode.writerPosOffset
				e.mode.writerPosOffset = fmt.Sprintf("(size_t)(%s)", e.expr(userT.Size))
				e.emitUnprocess(src, a.Process, "_p")
				e.mode.writerPosOffset = prevOffset
				term := -1
				pad := -1
				if a.Terminator != nil {
					term = *a.Terminator
				}
				if a.PadRight != nil {
					pad = *a.PadRight
				}
				if e.needsRawShadowField(a) {
					e.emitRepeatRawShadowWrite(src, field, "_p", e.expr(userT.Size), term, pad, false)
				} else if term >= 0 || pad >= 0 {
					src.pf("ZB_TRY(zb_write_bytes_limit(wstream, _p, (size_t)(%s), %d, %d));",
						e.expr(userT.Size), term, pad)
				} else {
					src.pf("ZB_TRY(zb_write_bytes_limit(wstream, _p, (size_t)(%s), -1, -1));",
						e.expr(userT.Size))
				}
			case userT.Size != nil:
				term := -1
				pad := -1
				includeOn := false
				if a != nil {
					if a.Terminator != nil {
						term = *a.Terminator
					}
					if a.PadRight != nil {
						pad = *a.PadRight
					}
					if a.Include != nil {
						includeOn = *a.Include
					}
				}
				src.pf("zb_writer_t _w_inner; zb_writer_init(&_w_inner, this_->_arena);")
				emitTry(src, "%s_write(this_->%s.data[_i], &_w_inner)", typeName, field)
				src.pf("zb_bytes_t _p = zb_writer_bytes(&_w_inner);")
				if e.needsRawShadowField(a) {
					e.emitRepeatRawShadowWrite(src, field, "_p", e.expr(userT.Size), term, pad, includeOn)
				} else {
					src.pf("ZB_TRY(zb_write_bytes_limit(wstream, _p, (size_t)(%s), %d, %d));",
						e.expr(userT.Size), term, pad)
				}
			default:
				emitTry(src, "%s_write(this_->%s.data[_i], wstream)", typeName, field)
				if a != nil && a.Terminator != nil {
					include := false
					if a.Include != nil {
						include = *a.Include
					}
					consume := true
					if a.Consume != nil {
						consume = *a.Consume
					}
					if consume && !include {
						emitTry(src, "zb_write_u1(wstream, %d)", *a.Terminator)
					}
				}
			}
			src.unindent()
			src.pf("}")
		}
	case types.Bits:
		if t.Bits != nil {
			endian := "be"
			if t.Bits.Endian.Kind == types.LittleBitEndian {
				endian = "le"
			}
			emitTry(src, "zb_write_bits_%s(wstream, %d, (uint64_t)(this_->%s.data[_i]))", endian, t.Bits.Width, field)
		}
	case types.Bytes, types.String:
		lv := fmt.Sprintf("this_->%s.data[_i]", field)
		prevField := e.mode.bytesShadowField
		prevExpr := e.mode.bytesShadowExpr
		if e.needsRawBytesShadowField(a) {
			e.mode.bytesShadowField = field
			e.mode.bytesShadowExpr = fmt.Sprintf("(_i < this_->_raw_%s.len ? this_->_raw_%s.data[_i] : (zb_bytes_t){0})", field, field)
		}
		if a != nil && a.Process != nil {
			src.pf("{")
			src.indent()
			src.pf("zb_bytes_t _p = %s;", lv)
			e.emitUnprocess(src, a.Process, "_p")
			e.emitBytesWrite(src, t, "_p", a)
			src.unindent()
			src.pf("}")
		} else {
			e.emitBytesWrite(src, t, lv, a)
		}
		e.mode.bytesShadowField = prevField
		e.mode.bytesShadowExpr = prevExpr
	default:
		if call := writeCallForKind(t.Kind); call != "" {
			emitTry(src, "%s(wstream, this_->%s.data[_i])", call, field)
		} else {
			panic(fmt.Errorf("unsupported repeat-write element type %s", t.Kind.String()))
		}
	}
	src.unindent()
	src.pf("}")
}

func (e *Emitter) emitRepeatRawShadowWrite(src *buf, field, pValue, sizeExpr string, term, pad int, include bool) {
	src.pf("{")
	src.indent()
	src.pf("size_t _expected = (size_t)(%s);", sizeExpr)
	src.pf("zb_bytes_t _v = %s;", pValue)
	src.pf("zb_bytes_t _r = (_i < this_->_raw_%s.len ? this_->_raw_%s.data[_i] : (zb_bytes_t){0});", field, field)
	src.pf("size_t _vlen = _v.len > _expected ? _expected : _v.len;")
	src.pf("if (_vlen) { ZB_TRY(zb_write_bytes_raw(wstream, _v.data, _vlen)); }")
	src.pf("size_t _filled = _vlen;")
	src.pf("if (_r.len >= _expected) {")
	src.indent()
	src.pf("if (_filled < _expected) { ZB_TRY(zb_write_bytes_raw(wstream, _r.data + _filled, _expected - _filled)); _filled = _expected; }")
	src.unindent()
	src.pf("} else {")
	src.indent()
	if term >= 0 && !include {
		src.pf("if (_filled < _expected) { ZB_TRY(zb_write_u1(wstream, %d)); _filled++; }", term)
	}
	src.pf("if (_r.len > _filled && _filled < _expected) {")
	src.indent()
	src.pf("size_t _take = _r.len - _filled;")
	src.pf("if (_take > _expected - _filled) _take = _expected - _filled;")
	emitTry(src, "zb_write_bytes_raw(wstream, _r.data + _filled, _take)")
	src.pf("_filled += _take;")
	src.unindent()
	src.pf("}")
	padByte := max(pad, 0)
	src.pf("while (_filled < _expected) { ZB_TRY(zb_write_u1(wstream, %d)); _filled++; }", padByte)
	src.unindent()
	src.pf("}")
	src.unindent()
	src.pf("}")
}

func (e *Emitter) emitRepeatTypeSwitchWrite(src *buf, parent *engine.ExprValue, a *kaitai.Attr, sw *types.TypeSwitch, field string) {
	src.pf("for (size_t _i = 0; _i < this_->%s.len; _i++) {", field)
	src.indent()
	src.pf("void *_elem = this_->%s.data[_i];", field)
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
		e.emitSwitchCaseWriteFrom(src, &caseRef, "_elem", a)
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
		e.emitSwitchCaseWriteFrom(src, &defaultRef, "_elem", a)
		src.unindent()
		src.pf("}")
	} else if wroteAny {
		src.pf("} else if (_elem) {")
		src.indent()
		if a != nil && a.Size != nil {
			src.pf("ZB_TRY(zb_write_bytes_limit(wstream, *(zb_bytes_t *)_elem, (size_t)(%s), -1, -1));",
				e.expr(a.Size))
		} else {
			emitTry(src, "zb_write_bytes(wstream, *(zb_bytes_t *)_elem)")
		}
		src.unindent()
		src.pf("}")
	}
	src.unindent()
	src.pf("}")
	_ = parent
}

func (e *Emitter) emitSwitchCaseWriteFrom(src *buf, t *types.TypeRef, valExpr string, a *kaitai.Attr) {
	_ = a
	switch t.Kind {
	case types.U1, types.S1, types.U2le, types.U2be, types.U4le, types.U4be,
		types.U8le, types.U8be, types.S2le, types.S2be, types.S4le, types.S4be,
		types.S8le, types.S8be, types.F4le, types.F4be, types.F8le, types.F8be:
		ctyp := e.declTypeRef(t, false)
		src.pf("if (%s) { ZB_TRY(%s(wstream, *(%s *)%s)); }",
			valExpr, writeCallForKind(t.Kind), ctyp, valExpr)
	case types.User:
		userT := t.User
		typ := e.tryResolveType(userT.Name)
		if typ != nil && typ.Struct != nil {
			typeName := e.prefix(typ.DefParent) + e.typeName(typ.Struct.Type.ID)
			src.pf("if (%s) { ZB_TRY(%s_write((%s_t *)%s, wstream)); }",
				valExpr, typeName, typeName, valExpr)
		}
	case types.Bytes, types.String:
		src.pf("if (%s) { ZB_TRY(zb_write_bytes(wstream, *(zb_bytes_t *)%s)); }",
			valExpr, valExpr)
	default:
		panic(fmt.Errorf("unsupported type-switch case write %s", t.Kind.String()))
	}
}

func (e *Emitter) emitTypeSwitchWriteUniform(src *buf, t *types.TypeRef, field string, a *kaitai.Attr) {
	e.emitSingleWrite(src, t, field, a, "this_->"+field)
}

func (e *Emitter) emitTypeSwitchWrite(src *buf, parent *engine.ExprValue, a *kaitai.Attr, sw *types.TypeSwitch, field string) {
	_ = parent
	if uni := uniformSwitchCase(sw); uni != nil {
		e.emitTypeSwitchWriteUniform(src, uni, field, a)
		return
	}
	if widened := integerWidenedSwitchCase(sw); widened != nil {
		e.emitIntegerSwitchWrite(src, sw, field, widened)
		return
	}
	if e.needsSwitchRawShadowField(a) {
		src.pf("if (this_->_raw_%s.data != NULL) {", field)
		src.indent()
		emitTry(src, "zb_write_bytes(wstream, this_->_raw_%s)", field)
		src.unindent()
		src.pf("} else {")
		src.indent()
		defer func() {
			src.unindent()
			src.pf("}")
		}()
	}
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
		e.emitSwitchCaseWrite(src, &caseRef, field, a)
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
		e.emitSwitchCaseWrite(src, &defaultRef, field, a)
		src.unindent()
		src.pf("}")
	} else if wroteAny {
		src.pf("} else if (this_->%s) {", field)
		src.indent()
		if a != nil && a.Size != nil {
			src.pf("ZB_TRY(zb_write_bytes_limit(wstream, *(zb_bytes_t *)this_->%s, (size_t)(%s), -1, -1));",
				field, e.expr(a.Size))
		} else {
			emitTry(src, "zb_write_bytes(wstream, *(zb_bytes_t *)this_->%s)", field)
		}
		src.unindent()
		src.pf("}")
	}
	src.unindent()
	src.pf("}")
}

func (e *Emitter) emitIntegerSwitchWrite(src *buf, sw *types.TypeSwitch, field string, widened *types.TypeRef) {
	_ = widened
	keys := make([]string, 0, len(sw.Cases))
	for k := range sw.Cases {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	_, hasDefault := sw.Cases["_"]
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
		src.pf("ZB_TRY(%s(wstream, (%s)(this_->%s)));",
			writeCallForKind(caseRef.Kind), caseC, field)
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
		src.pf("ZB_TRY(%s(wstream, (%s)(this_->%s)));",
			writeCallForKind(defRef.Kind), caseC, field)
		src.unindent()
		src.pf("}")
	} else if wroteAny {
		src.pf("}")
	}
	src.unindent()
	src.pf("}")
}

func (e *Emitter) emitSwitchCaseWrite(src *buf, t *types.TypeRef, field string, a *kaitai.Attr) {
	hasSize := a != nil && a.Size != nil
	switch t.Kind {
	case types.U1, types.S1, types.U2le, types.U2be, types.U4le, types.U4be,
		types.U8le, types.U8be, types.S2le, types.S2be, types.S4le, types.S4be,
		types.S8le, types.S8be, types.F4le, types.F4be, types.F8le, types.F8be:
		ctyp := e.declTypeRef(t, false)
		src.pf("if (this_->%s) { ZB_TRY(%s(wstream, *(%s *)this_->%s)); }",
			field, writeCallForKind(t.Kind), ctyp, field)
	case types.User:
		userT := t.User
		typ := e.tryResolveType(userT.Name)
		if typ != nil && typ.Struct != nil {
			typeName := e.prefix(typ.DefParent) + e.typeName(typ.Struct.Type.ID)
			if hasSize {
				src.pf("if (this_->%s) {", field)
				src.indent()
				src.pf("zb_writer_t _w_inner; zb_writer_init(&_w_inner, this_->_arena);")
				emitTry(src, "%s_write((%s_t *)this_->%s, &_w_inner)", typeName, typeName, field)
				src.pf("zb_bytes_t _p = zb_writer_bytes(&_w_inner);")
				emitTry(src, "zb_write_bytes_limit(wstream, _p, (size_t)(%s), -1, -1)", e.expr(a.Size))
				src.unindent()
				src.pf("}")
			} else {
				src.pf("if (this_->%s) { ZB_TRY(%s_write((%s_t *)this_->%s, wstream)); }",
					field, typeName, typeName, field)
			}
		}
	case types.Bytes, types.String:
		if hasSize {
			src.pf("if (this_->%s) { ZB_TRY(zb_write_bytes_limit(wstream, *(zb_bytes_t *)this_->%s, (size_t)(%s), -1, -1)); }",
				field, field, e.expr(a.Size))
		} else {
			src.pf("if (this_->%s) { ZB_TRY(zb_write_bytes(wstream, *(zb_bytes_t *)this_->%s)); }",
				field, field)
		}
	default:
		panic(fmt.Errorf("unsupported type-switch case write %s", t.Kind.String()))
	}
}

func writeCallForKind(k types.Kind) string {
	switch k {
	case types.U1:
		return "zb_write_u1"
	case types.U2le:
		return "zb_write_u2le"
	case types.U2be:
		return "zb_write_u2be"
	case types.U4le:
		return "zb_write_u4le"
	case types.U4be:
		return "zb_write_u4be"
	case types.U8le:
		return "zb_write_u8le"
	case types.U8be:
		return "zb_write_u8be"
	case types.S1:
		return "zb_write_s1"
	case types.S2le:
		return "zb_write_s2le"
	case types.S2be:
		return "zb_write_s2be"
	case types.S4le:
		return "zb_write_s4le"
	case types.S4be:
		return "zb_write_s4be"
	case types.S8le:
		return "zb_write_s8le"
	case types.S8be:
		return "zb_write_s8be"
	case types.F4le:
		return "zb_write_f4le"
	case types.F4be:
		return "zb_write_f4be"
	case types.F8le:
		return "zb_write_f8le"
	case types.F8be:
		return "zb_write_f8be"
	}
	return ""
}
