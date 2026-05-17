package golang

import (
	"fmt"
	"slices"
	"strings"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/types"
)

// strucWrite generates the Write() method for a struct.
func (e *Emitter) strucWrite(unit *goUnit, gs *goStruct, val *engine.ExprValue, forceEndian types.EndianKind) {
	ks := val.Struct.Type

	endianSuffix := ""
	switch forceEndian {
	case types.LittleEndian:
		endianSuffix = "LE"
	case types.BigEndian:
		endianSuffix = "BE"
	}

	writeMethod := goFunc{
		recv: goVar{name: "this", typ: "*" + gs.name},
		name: "Write" + endianSuffix,
		in:   []goVar{{name: "wstream", typ: "*" + kaitaiWriter}},
		out:  []goVar{{name: "err", typ: "error"}},
	}

	// Save/restore expression engine state
	oldNeedRoot := e.needRoot
	oldNeedParent := e.needParent
	oldInWriteExpr := e.inWriteExpr
	e.needRoot = false
	e.needParent = false
	e.inWriteExpr = true

	inBitsMode := false
	bitsLE := false
	totalBits := 0
	alignIdx := 0
	for _, attr := range val.Struct.Attrs {
		if attr.Attr == nil {
			continue
		}
		a := attr.Attr
		rt := a.Type.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)
		if rt.TypeRef == nil && a.Type.TypeSwitch == nil {
			continue
		}
		switch forceEndian {
		case types.LittleEndian:
			rt = a.Type.FoldEndian(types.LittleEndian).FoldBitEndian(e.bitEndian)
		case types.BigEndian:
			rt = a.Type.FoldEndian(types.BigEndian).FoldBitEndian(e.bitEndian)
		}

		isBits := rt.TypeRef != nil && rt.TypeRef.Kind == types.Bits
		if inBitsMode && !isBits {
			padBits := (8 - (totalBits % 8)) % 8
			if padBits > 0 {
				endianSuffix2 := "Be"
				if bitsLE {
					endianSuffix2 = "Le"
				}
				writeMethod.printf("if err = wstream.WriteBitsInt%s(%d, this._align_%d); err != nil { return err }", endianSuffix2, padBits, alignIdx)
				alignIdx++
			} else {
				writeMethod.printf("if err = wstream.AlignToByte(); err != nil { return err }")
			}
			totalBits = 0
		}
		if isBits {
			totalBits += rt.TypeRef.Bits.Width
			bitsLE = rt.TypeRef.Bits.Endian.Kind == types.LittleBitEndian
		}
		inBitsMode = isBits
		e.writeAttr(unit, &writeMethod, attr, forceEndian)
	}
	if inBitsMode {
		padBits := (8 - (totalBits % 8)) % 8
		if padBits > 0 {
			endianSuffix2 := "Be"
			if bitsLE {
				endianSuffix2 = "Le"
			}
			writeMethod.printf("if err = wstream.WriteBitsInt%s(%d, this._align_%d); err != nil { return err }", endianSuffix2, padBits, alignIdx)
		} else {
			writeMethod.printf("if err = wstream.AlignToByte(); err != nil { return err }")
		}
	}

	writeMethod.printf("return nil")

	// Add stream/parent/root locals for expression compatibility
	// Some expressions reference 'stream', '_parent', or '_root' in write context.
	writeMethod.preprintf("_ = _root")
	writeMethod.preprintf("_root := this.Root_")
	writeMethod.preprintf("_ = _parent")
	writeMethod.preprintf("_parent := this.Parent_")
	writeMethod.preprintf("_ = stream")
	writeMethod.preprintf("stream := this.IO_")

	e.needRoot = oldNeedRoot
	e.needParent = oldNeedParent
	e.inWriteExpr = oldInWriteExpr

	_ = ks
	unit.methods = append(unit.methods, writeMethod)
}

// writeAttr generates the write code for a single attribute.
func (e *Emitter) writeAttr(unit *goUnit, fn *goFunc, typ *engine.ExprValue, forcedEndian types.EndianKind) {
	a := typ.Attr
	rt := a.Type.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)
	switch forcedEndian {
	case types.LittleEndian:
		rt = a.Type.FoldEndian(types.LittleEndian).FoldBitEndian(e.bitEndian)
	case types.BigEndian:
		rt = a.Type.FoldEndian(types.BigEndian).FoldBitEndian(e.bitEndian)
	}

	fieldName := "this." + e.fieldName(a.ID)

	// Conditional field - use cached condition for IO-dependent expressions
	if a.If != nil {
		if exprReferencesIO(a.If) {
			fn.printf("if this._if_%s {", string(a.ID))
		} else {
			fn.printf("if %s {", e.expr(a.If))
		}
		fn.indent()
	}

	if a.Type.TypeSwitch != nil {
		// Type switch - for sized fields, use stored raw bytes
		if a.Size != nil {
			rawField := fmt.Sprintf("this._raw_%s", string(a.ID))
			if a.Repeat != nil {
				// For repeated switch+size, each element's raw bytes are indexed
				fn.printf("for _si, _sv := range %s {", fieldName)
				fn.indent()
				fn.printf("_ = _sv")
				fn.printf("if %s != nil && _si < len(%s) {", rawField, rawField)
				fn.indent()
				fn.printf("if err = wstream.WriteBytes(%s[_si]); err != nil { return err }", rawField)
				fn.unindent()
				fn.printf("} else {")
				fn.indent()
				// Fallback: write via switch helper
				ts := a.Type.TypeSwitch
				helperName := "write" + e.typeSwitchName(a.ID)
				if exprContainsIndex(ts.SwitchOn) {
					fn.printf("if err = this.%s(wstream, _sv, _si); err != nil { return err }", helperName)
				} else {
					fn.printf("if err = this.%s(wstream, _sv); err != nil { return err }", helperName)
				}
				fn.unindent()
				fn.printf("}")
				fn.unindent()
				fn.printf("}")
			} else {
				fn.printf("if %s != nil {", rawField)
				fn.indent()
				fn.printf("if err = wstream.WriteBytes(%s); err != nil { return err }", rawField)
				fn.unindent()
				fn.printf("} else {")
				fn.indent()
				e.writeTypeSwitchCall(fn, typ)
				fn.unindent()
				fn.printf("}")
			}
		} else {
			e.writeTypeSwitchCall(fn, typ)
		}
	} else if rt.TypeRef != nil && rt.TypeRef.Kind == types.User {
		// User type
		e.writeUserType(unit, fn, typ, fieldName, forcedEndian)
	} else if rt.TypeRef != nil {
		// Primitive / bytes / string / bits
		e.writePrimitive(unit, fn, typ, fieldName, rt)
	}
	// If rt.TypeRef is nil (e.g. type not resolved), skip silently

	if a.If != nil {
		fn.unindent()
		fn.printf("}")
	}
}

// writePrimitive writes a primitive/bytes/string field.
func (e *Emitter) writePrimitive(unit *goUnit, fn *goFunc, typ *engine.ExprValue, fieldName string, rt types.Type) {
	a := typ.Attr

	// Determine the value expression with necessary casts
	valExpr := fieldName

	// If the field is conditional (has if:), it may be stored as 'any' and needs type assertion
	if a.If != nil && rt.TypeRef != nil {
		goType := e.declTypeRef(rt.TypeRef, nil)
		if needsPointerForNil(goType) {
			// This field is stored as 'any' - need type assertion
			valExpr = fmt.Sprintf("%s.(%s)", fieldName, goType)
		}
	}

	if a.Enum != "" {
		// Enum field: cast to underlying integer type
		baseType := e.declTypeRef(rt.TypeRef, nil)
		valExpr = fmt.Sprintf("(%s)(%s)", baseType, fieldName)
	}

	switch {
	case a.Repeat != nil:
		// Repeated field - write all elements (use index for process raw storage)
		fn.printf("for i, _item := range %s {", fieldName)
		fn.indent()
		fn.printf("_ = i")
		itemExpr := "_item"
		if a.Enum != "" {
			baseType := e.declTypeRef(rt.TypeRef, nil)
			itemExpr = fmt.Sprintf("(%s)(_item)", baseType)
		}
		// Handle bit fields in repeated context
		if rt.TypeRef != nil && rt.TypeRef.Kind == types.Bits {
			if rt.TypeRef.Bits.Width == 1 && a.Enum == "" {
				// b1 (bool) repeated: convert bool->uint64
				fn.printf("if err = wstream.WriteBitsIntBe(1, func() uint64 { if _item { return 1 }; return 0 }()); err != nil { return err }")
				fn.unindent()
				fn.printf("}")
				return
			}
			endianSuffix2 := "Be"
			if rt.TypeRef.Bits.Endian.Kind == types.LittleBitEndian {
				endianSuffix2 = "Le"
			}
			fn.printf("if err = wstream.WriteBitsInt%s(%d, uint64(_item)); err != nil { return err }", endianSuffix2, rt.TypeRef.Bits.Width)
			fn.unindent()
			fn.printf("}")
			return
		}
		// Handle repeated raw-tail fields (pad/term per element)
		if e.fieldNeedsRawTail(rt) {
			termByte := -1
			include := false
			if rt.TypeRef.Kind == types.Bytes && rt.TypeRef.Bytes != nil {
				termByte = rt.TypeRef.Bytes.Terminator
				include = rt.TypeRef.Bytes.Include
			} else if rt.TypeRef.Kind == types.String && rt.TypeRef.String != nil {
				termByte = rt.TypeRef.String.Terminator
				include = rt.TypeRef.String.Include
			}
			dataExpr := itemExpr
			if rt.TypeRef.Kind == types.String {
				dataExpr = fmt.Sprintf("[]byte(%s)", itemExpr)
			}
			fn.printf("if err = wstream.WriteBytes(%s); err != nil { return err }", dataExpr)
			if termByte >= 0 && !include {
				fn.printf("if i < len(this._raw_tail_%s) && (len(%s) > 0 || len(this._raw_tail_%s[i]) > 0) {", string(a.ID), dataExpr, string(a.ID))
				fn.indent()
				fn.printf("if err = wstream.WriteU1(%d); err != nil { return err }", termByte)
				fn.unindent()
				fn.printf("}")
			}
			fn.printf("if i < len(this._raw_tail_%s) && this._raw_tail_%s[i] != nil {", string(a.ID), string(a.ID))
			fn.indent()
			fn.printf("if err = wstream.WriteBytes(this._raw_tail_%s[i]); err != nil { return err }", string(a.ID))
			fn.unindent()
			fn.printf("}")
			fn.unindent()
			fn.printf("}")
			return
		}
		if a.Process != nil && (rt.TypeRef.Kind == types.Bytes || rt.TypeRef.Kind == types.String) {
			// Use stored raw pre-process bytes if available
			fn.printf("if this._raw_%s != nil && i < len(this._raw_%s) {", string(a.ID), string(a.ID))
			fn.indent()
			fn.printf("if err = wstream.WriteBytes(this._raw_%s[i]); err != nil { return err }", string(a.ID))
			fn.unindent()
			fn.printf("} else {")
			fn.indent()
			fn.printf("{")
			fn.indent()
			if rt.TypeRef.Kind == types.String {
				fn.printf("_raw := []byte(%s)", itemExpr)
			} else {
				fn.printf("_raw := append([]byte(nil), %s...)", itemExpr)
			}
			e.emitProcessReverse(fn, unit, a.Process, "_raw")
			fn.printf("if err = wstream.WriteBytes(_raw); err != nil { return err }")
			fn.unindent()
			fn.printf("}")
			fn.unindent()
			fn.printf("}")
		} else {
			writeCall := e.writeCallRef(rt.TypeRef, itemExpr)
			fn.printf("if err = %s; err != nil { return err }", writeCall)
		}
		fn.unindent()
		fn.printf("}")
	default:
		// When both process AND raw-tail apply, _raw_ has the full pre-strip/pre-process bytes.
		// Write them directly for perfect roundtrip.
		if a.Process != nil && e.fieldNeedsRawTail(rt) {
			fn.printf("if this._raw_%s != nil {", string(a.ID))
			fn.indent()
			fn.printf("if err = wstream.WriteBytes(this._raw_%s); err != nil { return err }", string(a.ID))
			fn.unindent()
			fn.printf("} else {")
			fn.indent()
			// Fallback: reverse process + reconstruct from raw tail
			fn.printf("{")
			fn.indent()
			if rt.TypeRef.Kind == types.String {
				fn.printf("_raw := []byte(%s)", valExpr)
			} else {
				fn.printf("_raw := append([]byte(nil), %s...)", valExpr)
			}
			e.emitProcessReverse(fn, unit, a.Process, "_raw")
			fn.printf("if err = wstream.WriteBytes(_raw); err != nil { return err }")
			fn.unindent()
			fn.printf("}")
			fn.unindent()
			fn.printf("}")
			return
		}
		// Single field - handle raw tail fields for roundtrip
		if e.fieldNeedsRawTail(rt) {
			// Write data + terminator (if applicable) + saved raw tail
			termByte := -1
			include := false
			if rt.TypeRef.Kind == types.Bytes && rt.TypeRef.Bytes != nil {
				termByte = rt.TypeRef.Bytes.Terminator
				include = rt.TypeRef.Bytes.Include
			} else if rt.TypeRef.Kind == types.String && rt.TypeRef.String != nil {
				termByte = rt.TypeRef.String.Terminator
				include = rt.TypeRef.String.Include
			}
			dataExpr := valExpr
			isMultiByte := false
			if rt.TypeRef.Kind == types.String && rt.TypeRef.String != nil {
				enc := rt.TypeRef.String.Encoding
				if e.needsEncodingConversion(enc) {
					isMultiByte = isMultiByteEncoding(enc)
					encoder := e.encodingEncoder(unit, enc)
					fn.printf("{")
					fn.indent()
					fn.printf("_data, err := %s.Bytes([]byte(%s))", encoder, valExpr)
					fn.printf("if err != nil { return err }")
				} else {
					fn.printf("{")
					fn.indent()
					fn.printf("_data := []byte(%s)", valExpr)
				}
			} else {
				fn.printf("{")
				fn.indent()
				if rt.TypeRef.Kind == types.String {
					fn.printf("_data := []byte(%s)", valExpr)
				} else {
					fn.printf("_data := %s", dataExpr)
				}
			}
			fn.printf("if err = wstream.WriteBytes(_data); err != nil { return err }")
			if termByte >= 0 && !include && !isMultiByte {
				// For single-byte terminators, write the terminator (raw_tail starts AFTER it)
				// For multi-byte terminators, raw_tail starts AT the terminator (skip explicit write)
				fn.printf("if len(_data) > 0 || len(this._raw_tail_%s) > 0 {", string(a.ID))
				fn.indent()
				fn.printf("if err = wstream.WriteU1(%d); err != nil { return err }", termByte)
				fn.unindent()
				fn.printf("}")
			}
			fn.printf("if this._raw_tail_%s != nil {", string(a.ID))
			fn.indent()
			fn.printf("if err = wstream.WriteBytes(this._raw_tail_%s); err != nil { return err }", string(a.ID))
			fn.unindent()
			fn.printf("}")
			fn.unindent()
			fn.printf("}")
			return
		}
		// Handle string encoding conversion (non-UTF8 strings need re-encoding)
		if rt.TypeRef != nil && rt.TypeRef.Kind == types.String && rt.TypeRef.String != nil {
			enc := rt.TypeRef.String.Encoding
			if e.needsEncodingConversion(enc) {
				encoder := e.encodingEncoder(unit, enc)
				fn.printf("{")
				fn.indent()
				fn.printf("_enc_bytes, err := %s.Bytes([]byte(%s))", encoder, valExpr)
				fn.printf("if err != nil { return err }")
				// Now write the encoded bytes using the appropriate method
				if a.Process != nil {
					fn.printf("_raw := _enc_bytes")
					e.emitProcessReverse(fn, unit, a.Process, "_raw")
					fn.printf("if err = wstream.WriteBytes(_raw); err != nil { return err }")
				} else if e.fieldNeedsRawTail(rt) {
					termByte := rt.TypeRef.String.Terminator
					include := rt.TypeRef.String.Include
					fn.printf("if err = wstream.WriteBytes(_enc_bytes); err != nil { return err }")
					if !include {
						fn.printf("if err = wstream.WriteU1(%d); err != nil { return err }", termByte)
					}
					fn.printf("if this._raw_tail_%s != nil {", string(a.ID))
					fn.indent()
					fn.printf("if err = wstream.WriteBytes(this._raw_tail_%s); err != nil { return err }", string(a.ID))
					fn.unindent()
					fn.printf("}")
				} else if rt.TypeRef.String.Size != nil {
					terminator := rt.TypeRef.String.Terminator
					padRight := rt.TypeRef.String.PadRight
					if terminator >= 0 || padRight >= 0 {
						fn.printf("if err = wstream.WriteBytesLimit(_enc_bytes, int(%s), %d, %d); err != nil { return err }", e.expr(rt.TypeRef.String.Size), terminator, padRight)
					} else {
						fn.printf("if err = wstream.WriteBytes(_enc_bytes); err != nil { return err }")
					}
				} else if rt.TypeRef.String.Terminator >= 0 && !rt.TypeRef.String.Include && rt.TypeRef.String.Consume {
					fn.printf("if err = wstream.WriteBytes(_enc_bytes); err != nil { return err }")
					if isMultiByteEncoding(enc) {
						fn.printf("if err = wstream.WriteBytes([]byte{%d, %d}); err != nil { return err }", rt.TypeRef.String.Terminator, rt.TypeRef.String.Terminator)
					} else {
						fn.printf("if err = wstream.WriteU1(%d); err != nil { return err }", rt.TypeRef.String.Terminator)
					}
				} else {
					fn.printf("if err = wstream.WriteBytes(_enc_bytes); err != nil { return err }")
				}
				fn.unindent()
				fn.printf("}")
				return
			}
		}
		// Handle process fields: write raw pre-process bytes directly
		if a.Process != nil && rt.TypeRef != nil && (rt.TypeRef.Kind == types.Bytes || rt.TypeRef.Kind == types.String) {
			fn.printf("if this._raw_%s != nil {", string(a.ID))
			fn.indent()
			fn.printf("if err = wstream.WriteBytes(this._raw_%s); err != nil { return err }", string(a.ID))
			fn.unindent()
			fn.printf("} else {")
			fn.indent()
			// Fallback: try to reverse the process
			fn.printf("{")
			fn.indent()
			if rt.TypeRef.Kind == types.String {
				fn.printf("_raw := []byte(%s)", valExpr)
			} else {
				fn.printf("_raw := append([]byte(nil), %s...)", valExpr)
			}
			e.emitProcessReverse(fn, unit, a.Process, "_raw")
			fn.printf("if err = wstream.WriteBytes(_raw); err != nil { return err }")
			fn.unindent()
			fn.printf("}")
			fn.unindent()
			fn.printf("}")
			return
		}
		if rt.TypeRef != nil && rt.TypeRef.Kind == types.Bits {
			// Bit fields: cast to uint64
			isAnyField := a.If != nil && needsPointerForNil("uint64")
			if rt.TypeRef.Bits.Width == 1 && a.Enum == "" {
				if isAnyField {
					// b1 without enum stored as any: assert to bool
					valExpr = fmt.Sprintf("func() uint64 { if %s.(bool) { return 1 }; return 0 }()", fieldName)
				} else {
					// b1 without enum: bool -> uint64
					valExpr = fmt.Sprintf("func() uint64 { if %s { return 1 }; return 0 }()", fieldName)
				}
			} else if a.Enum != "" {
				// bits with enum: enum -> int -> uint64
				valExpr = fmt.Sprintf("uint64(int(%s))", fieldName)
			} else if isAnyField {
				// bits stored as any: assert to uint64
				valExpr = fmt.Sprintf("%s.(uint64)", fieldName)
			} else {
				valExpr = fmt.Sprintf("uint64(%s)", fieldName)
			}
		}
		writeCall := e.writeCallRef(rt.TypeRef, valExpr)
		fn.printf("if err = %s; err != nil { return err }", writeCall)
	}
}

// writeUserType writes a user type field (potentially with substream).
func (e *Emitter) writeUserType(unit *goUnit, fn *goFunc, typ *engine.ExprValue, fieldName string, forcedEndian types.EndianKind) {
	a := typ.Attr

	switch {
	case a.Repeat != nil:
		// Repeated user type - use indexed loop for expressions that reference _index
		fn.printf("for i, _item := range %s {", fieldName)
		fn.indent()
		fn.printf("_ = i")
		e.writeUserTypeSingle(unit, fn, typ, "_item", forcedEndian)
		fn.unindent()
		fn.printf("}")
	default:
		e.writeUserTypeSingle(unit, fn, typ, fieldName, forcedEndian)
	}
}

// writeUserTypeSingle writes a single user type value.
func (e *Emitter) writeUserTypeSingle(unit *goUnit, fn *goFunc, typ *engine.ExprValue, valExpr string, forcedEndian types.EndianKind) {
	a := typ.Attr

	// Determine the Write method to call based on endianness
	writeMethodName := "Write"
	isSwitchEndian := e.endian == types.SwitchEndian
	if isSwitchEndian && forcedEndian == types.LittleEndian {
		writeMethodName = "WriteLE"
	} else if isSwitchEndian && forcedEndian == types.BigEndian {
		writeMethodName = "WriteBE"
	}

	if a.Size != nil {
		// User type with size: use stored raw bytes if available, else re-serialize
		fn.printf("{")
		fn.indent()
		fn.printf("var _raw []byte")
		// Determine the raw field access expression based on repeat
		rawFieldExpr := fmt.Sprintf("this._raw_%s", string(a.ID))
		rawAccessExpr := rawFieldExpr
		if a.Repeat != nil {
			rawAccessExpr = fmt.Sprintf("%s[i]", rawFieldExpr)
		}
		fn.printf("_useRaw := false")
		fn.printf("_ = _useRaw")
		fn.printf("if %s != nil && len(%s) > 0 {", rawFieldExpr, rawAccessExpr)
		fn.indent()
		if a.Process != nil {
			// Process field: _raw_ has the full pre-strip/pre-process bytes.
			// Write them directly for perfect roundtrip - skip process reverse.
			fn.printf("_raw = %s", rawAccessExpr)
			fn.printf("_useRaw = true")
		} else {
			// Re-serialize into buffer, then overlay with original raw bytes
			fn.printf("_buf := kaitai.NewSeekableBuffer(nil)")
			fn.printf("_sub := kaitai.NewWriter(_buf)")
			fn.printf("if err = %s.%s(_sub); err != nil { return err }", valExpr, writeMethodName)
			fn.printf("_written := _buf.Bytes()")
			fn.printf("_raw = make([]byte, len(%s))", rawAccessExpr)
			fn.printf("copy(_raw, %s)", rawAccessExpr)
			fn.printf("copy(_raw, _written)")
		}
		fn.unindent()
		fn.printf("} else {")
		fn.indent()
		fn.printf("_buf := kaitai.NewSeekableBuffer(nil)")
		fn.printf("_sub := kaitai.NewWriter(_buf)")
		fn.printf("if err = %s.%s(_sub); err != nil { return err }", valExpr, writeMethodName)
		fn.printf("_raw = _buf.Bytes()")
		fn.unindent()
		fn.printf("}")

		// Apply reverse process if applicable (skip when using stored raw bytes directly)
		if a.Process != nil {
			fn.printf("if !_useRaw {")
			fn.indent()
			e.emitProcessReverse(fn, unit, a.Process, "_raw")
			fn.unindent()
			fn.printf("}")
		}

		// Pad to declared size, inserting terminator and pad-right bytes from attr
		termByte := -1
		padByte := 0 // default: zero fill
		if a.Terminator != nil {
			termByte = *a.Terminator
		}
		if a.PadRight != nil && *a.PadRight >= 0 {
			padByte = *a.PadRight
		} else if termByte >= 0 {
			padByte = termByte
		}
		include := a.Include != nil && *a.Include
		needsTermOrPad := (termByte >= 0 && !include) || padByte != 0
		// Check if we have raw tail data for roundtrip
		hasRawTail := (a.Terminator != nil && *a.Terminator >= 0) || (a.PadRight != nil && *a.PadRight >= 0)
		fn.printf("_size := int(%s)", e.expr(a.Size))
		if hasRawTail {
			// Use raw tail for exact reconstruction
			fn.printf("if this._raw_tail_%s != nil {", string(a.ID))
			fn.indent()
			if a.Terminator != nil && *a.Terminator >= 0 && (a.Include == nil || !*a.Include) {
				fn.printf("_raw = append(_raw, %d)", *a.Terminator)
			}
			fn.printf("_raw = append(_raw, this._raw_tail_%s...)", string(a.ID))
			fn.unindent()
			fn.printf("}")
		}
		fn.printf("if len(_raw) < _size {")
		fn.indent()
		fn.printf("_padded := make([]byte, _size)")
		fn.printf("copy(_padded, _raw)")
		if needsTermOrPad && !hasRawTail {
			fn.printf("_fill := len(_raw)")
			if termByte >= 0 && !include {
				fn.printf("if _fill < _size { _padded[_fill] = %d; _fill++ }", termByte)
			}
			if padByte != 0 {
				fn.printf("for _j := _fill; _j < _size; _j++ { _padded[_j] = %d }", padByte)
			}
		}
		fn.printf("_raw = _padded")
		fn.unindent()
		fn.printf("} else {")
		fn.indent()
		fn.printf("_raw = _raw[:_size]")
		fn.unindent()
		fn.printf("}")
		fn.printf("if err = wstream.WriteBytes(_raw); err != nil { return err }")
		fn.unindent()
		fn.printf("}")
	} else if a.SizeEos {
		// User type with size-eos: write to substream
		fn.printf("{")
		fn.indent()
		fn.printf("_buf := kaitai.NewSeekableBuffer(nil)")
		fn.printf("_sub := kaitai.NewWriter(_buf)")
		fn.printf("if err = %s.%s(_sub); err != nil { return err }", valExpr, writeMethodName)
		fn.printf("_raw := _buf.Bytes()")
		if a.Process != nil {
			e.emitProcessReverse(fn, unit, a.Process, "_raw")
		}
		fn.printf("if err = wstream.WriteBytes(_raw); err != nil { return err }")
		fn.unindent()
		fn.printf("}")
	} else if a.Terminator != nil {
		// User type with terminator: write to substream, then write content + terminator
		fn.printf("{")
		fn.indent()
		if a.Process != nil {
			// When process + terminator, use stored raw bytes if available
			rawExpr := fmt.Sprintf("this._raw_%s", string(a.ID))
			fn.printf("if %s != nil {", rawExpr)
			fn.indent()
			fn.printf("if err = wstream.WriteBytes(%s); err != nil { return err }", rawExpr)
			fn.unindent()
			fn.printf("} else {")
			fn.indent()
			fn.printf("_buf := kaitai.NewSeekableBuffer(nil)")
			fn.printf("_sub := kaitai.NewWriter(_buf)")
			fn.printf("if err = %s.%s(_sub); err != nil { return err }", valExpr, writeMethodName)
			fn.printf("_raw := _buf.Bytes()")
			e.emitProcessReverse(fn, unit, a.Process, "_raw")
			fn.printf("if err = wstream.WriteBytes(_raw); err != nil { return err }")
			fn.unindent()
			fn.printf("}")
		} else {
			fn.printf("_buf := kaitai.NewSeekableBuffer(nil)")
			fn.printf("_sub := kaitai.NewWriter(_buf)")
			fn.printf("if err = %s.%s(_sub); err != nil { return err }", valExpr, writeMethodName)
			fn.printf("if err = wstream.WriteBytes(_buf.Bytes()); err != nil { return err }")
		}
		consume := a.Consume == nil || *a.Consume // default true
		include := a.Include != nil && *a.Include
		if consume && !include {
			fn.printf("if err = wstream.WriteU1(%d); err != nil { return err }", *a.Terminator)
		}
		fn.unindent()
		fn.printf("}")
	} else {
		// Direct write to stream
		fn.printf("if err = %s.%s(wstream); err != nil { return err }", valExpr, writeMethodName)
	}
}

func (e *Emitter) positionedInstanceWrite(unit *goUnit, gs *goStruct, inst *engine.ExprValue) {
	a := inst.Instance

	fn := goFunc{
		recv: goVar{name: "this", typ: "*" + gs.name},
		name: "write" + e.typeName(a.ID),
		in:   []goVar{{name: "wstream", typ: "*" + kaitaiWriter}},
		out:  []goVar{{name: "err", typ: "error"}},
	}

	oldNeedRoot := e.needRoot
	oldNeedParent := e.needParent
	oldInWriteExpr := e.inWriteExpr
	e.needRoot = false
	e.needParent = false
	e.inWriteExpr = true
	defer func() {
		e.needRoot = oldNeedRoot
		e.needParent = oldNeedParent
		e.inWriteExpr = oldInWriteExpr
	}()

	if a.IO != nil {
		streamExpr := e.expr(a.IO)
		if streamExpr != "this.IO_" && streamExpr != "stream" {
			fn.printf("return nil")
			unit.methods = append(unit.methods, fn)
			return
		}
	}

	if a.Pos == nil {
		fn.printf("return nil")
		unit.methods = append(unit.methods, fn)
		return
	}
	if positionedExprDependsOnInputExtent(a.Pos) {
		fn.printf("return nil")
		unit.methods = append(unit.methods, fn)
		return
	}

	fn.printf("if !this._f_computed_%s {", string(a.ID))
	fn.indent()
	fn.printf("return nil")
	fn.unindent()
	fn.printf("}")

	fn.printf("_v := this._inst_%s", string(a.ID))
	writeValExpr := "_v"
	if a.If != nil {
		fn.printf("if _v == nil {")
		fn.indent()
		fn.printf("return nil")
		fn.unindent()
		fn.printf("}")
		instType := e.inferInstanceType(inst)
		if instType != "any" && needsPointerForNil(instType) {
			e.setImport(unit, "fmt", "fmt")
			fn.printf("_vTyped, ok := _v.(%s)", instType)
			fn.printf("if !ok { return fmt.Errorf(\"write positioned instance %s: expected %s, got %%T\", _v) }", string(a.ID), instType)
			writeValExpr = "_vTyped"
		}
	}

	fn.printf("_pos, err := wstream.Pos()")
	fn.printf("if err != nil { return err }")
	fn.printf("_, err = wstream.Seek(int64(%s), 0)", e.expr(a.Pos))
	fn.printf("if err != nil { return err }")
	if e.endian == types.SwitchEndian {
		fn.printf("if this._isLE {")
		fn.indent()
		e.writePositionedInstanceValue(unit, &fn, inst, writeValExpr, types.LittleEndian)
		fn.unindent()
		fn.printf("} else {")
		fn.indent()
		e.writePositionedInstanceValue(unit, &fn, inst, writeValExpr, types.BigEndian)
		fn.unindent()
		fn.printf("}")
	} else {
		e.writePositionedInstanceValue(unit, &fn, inst, writeValExpr, types.UnspecifiedOrder)
	}
	fn.printf("_, err = wstream.Seek(_pos, 0)")
	fn.printf("if err != nil { return err }")
	fn.printf("return nil")

	fn.preprintf("_ = _root")
	fn.preprintf("_root := this.Root_")
	fn.preprintf("_ = _parent")
	fn.preprintf("_parent := this.Parent_")
	fn.preprintf("_ = stream")
	fn.preprintf("stream := wstream")

	unit.methods = append(unit.methods, fn)
}

func positionedExprDependsOnInputExtent(ex *expr.Expr) bool {
	if ex == nil {
		return false
	}
	var walk func(expr.Node) bool
	walk = func(n expr.Node) bool {
		switch n := n.(type) {
		case expr.MemberNode:
			if id, ok := n.Operand.(expr.IdentNode); ok && id.Identifier == "_io" {
				return n.Property == "size" || n.Property == "eof"
			}
			return walk(n.Operand)
		case expr.CallNode:
			if member, ok := n.Object.(expr.MemberNode); ok {
				if id, ok := member.Operand.(expr.IdentNode); ok && id.Identifier == "_io" {
					return member.Property == "size" || member.Property == "eof"
				}
			}
			if walk(n.Object) {
				return true
			}
			return slices.ContainsFunc(n.Args, walk)
		case expr.UnaryNode:
			return walk(n.Operand)
		case expr.BinaryNode:
			return walk(n.A) || walk(n.B)
		case expr.TernaryNode:
			return walk(n.A) || walk(n.B) || walk(n.C)
		case expr.ScopeNode:
			return walk(n.Operand)
		case expr.SubscriptNode:
			return walk(n.A) || walk(n.B)
		case expr.CastNode:
			return walk(n.Operand)
		case expr.ArrayNode:
			if slices.ContainsFunc(n.Items, walk) {
				return true
			}
		}
		return false
	}
	return walk(ex.Root)
}

func (e *Emitter) writePositionedInstanceValue(unit *goUnit, fn *goFunc, inst *engine.ExprValue, valExpr string, forcedEndian types.EndianKind) {
	a := inst.Instance
	rt := a.Type.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)
	switch forcedEndian {
	case types.LittleEndian:
		rt = a.Type.FoldEndian(types.LittleEndian).FoldBitEndian(e.bitEndian)
	case types.BigEndian:
		rt = a.Type.FoldEndian(types.BigEndian).FoldBitEndian(e.bitEndian)
	}

	if a.Type.TypeSwitch != nil {
		ts := a.Type.TypeSwitch
		helperName := "write" + e.typeSwitchName(a.ID)
		needsIndex := exprContainsIndex(ts.SwitchOn)
		if a.Repeat != nil {
			fn.printf("for i, _item := range %s {", valExpr)
			fn.indent()
			fn.printf("_ = i")
			if needsIndex {
				fn.printf("if err = this.%s(wstream, _item, i); err != nil { return err }", helperName)
			} else {
				fn.printf("if err = this.%s(wstream, _item); err != nil { return err }", helperName)
			}
			fn.unindent()
			fn.printf("}")
		} else if needsIndex {
			fn.printf("if err = this.%s(wstream, %s, 0); err != nil { return err }", helperName, valExpr)
		} else {
			fn.printf("if err = this.%s(wstream, %s); err != nil { return err }", helperName, valExpr)
		}
		return
	}

	if rt.TypeRef == nil {
		return
	}

	if rt.TypeRef.Kind == types.User {
		e.writePositionedUserInstance(unit, fn, inst, valExpr, forcedEndian)
		return
	}

	if rt.TypeRef.Kind == types.String && rt.TypeRef.String != nil && e.needsEncodingConversion(rt.TypeRef.String.Encoding) {
		encoder := e.encodingEncoder(unit, rt.TypeRef.String.Encoding)
		fn.printf("{")
		fn.indent()
		fn.printf("_encBytes, err := %s.Bytes([]byte(%s))", encoder, valExpr)
		fn.printf("if err != nil { return err }")
		if a.Process != nil {
			fn.printf("_raw := _encBytes")
			e.emitProcessReverse(fn, unit, a.Process, "_raw")
			fn.printf("if err = wstream.WriteBytes(_raw); err != nil { return err }")
		} else {
			fn.printf("if err = %s; err != nil { return err }", e.writeCallRefOn("wstream", &types.TypeRef{Kind: types.Bytes}, "_encBytes"))
		}
		fn.unindent()
		fn.printf("}")
		return
	}

	writeVal := valExpr
	if a.Enum != "" {
		writeVal = fmt.Sprintf("(%s)(%s)", e.declTypeRef(rt.TypeRef, nil), valExpr)
	}
	if rt.TypeRef.Kind == types.Bits {
		if rt.TypeRef.Bits.Width == 1 && a.Enum == "" {
			writeVal = fmt.Sprintf("func() uint64 { if %s { return 1 }; return 0 }()", valExpr)
		} else if a.Enum != "" {
			writeVal = fmt.Sprintf("uint64(int(%s))", valExpr)
		} else {
			writeVal = fmt.Sprintf("uint64(%s)", valExpr)
		}
	}

	switch repeat := a.Repeat.(type) {
	case types.RepeatExpr:
		fn.printf("for i, _item := range %s {", valExpr)
		fn.indent()
		fn.printf("_ = i")
		fn.printf("if i >= int(%s) { break }", e.expr(repeat.CountExpr))
		itemExpr := "_item"
		if a.Enum != "" {
			itemExpr = fmt.Sprintf("(%s)(_item)", e.declTypeRef(rt.TypeRef, nil))
		}
		if rt.TypeRef.Kind == types.Bits {
			if rt.TypeRef.Bits.Width == 1 && a.Enum == "" {
				itemExpr = "func() uint64 { if _item { return 1 }; return 0 }()"
			} else if a.Enum != "" {
				itemExpr = "uint64(int(_item))"
			} else {
				itemExpr = "uint64(_item)"
			}
		}
		fn.printf("if err = %s; err != nil { return err }", e.writeCallRefOn("wstream", rt.TypeRef, itemExpr))
		fn.unindent()
		fn.printf("}")
	case types.RepeatEOS:
		fn.printf("for i, _item := range %s {", valExpr)
		fn.indent()
		fn.printf("_ = i")
		itemExpr := "_item"
		if a.Enum != "" {
			itemExpr = fmt.Sprintf("(%s)(_item)", e.declTypeRef(rt.TypeRef, nil))
		}
		fn.printf("if err = %s; err != nil { return err }", e.writeCallRefOn("wstream", rt.TypeRef, itemExpr))
		fn.unindent()
		fn.printf("}")
	default:
		fn.printf("if err = %s; err != nil { return err }", e.writeCallRefOn("wstream", rt.TypeRef, writeVal))
	}
}

func (e *Emitter) writePositionedUserInstance(unit *goUnit, fn *goFunc, inst *engine.ExprValue, valExpr string, forcedEndian types.EndianKind) {
	a := inst.Instance
	rt := a.Type.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)
	switch forcedEndian {
	case types.LittleEndian:
		rt = a.Type.FoldEndian(types.LittleEndian).FoldBitEndian(e.bitEndian)
	case types.BigEndian:
		rt = a.Type.FoldEndian(types.BigEndian).FoldBitEndian(e.bitEndian)
	}

	writeMethodName := "Write"
	if e.endian == types.SwitchEndian && forcedEndian == types.LittleEndian {
		writeMethodName = "WriteLE"
	} else if e.endian == types.SwitchEndian && forcedEndian == types.BigEndian {
		writeMethodName = "WriteBE"
	}

	writeOne := func(itemExpr string) {
		if a.Size != nil || a.SizeEos || a.Process != nil {
			fn.printf("{")
			fn.indent()
			fn.printf("_buf := kaitai.NewSeekableBuffer(nil)")
			fn.printf("_sub := kaitai.NewWriter(_buf)")
			fn.printf("if err = %s.%s(_sub); err != nil { return err }", itemExpr, writeMethodName)
			fn.printf("_raw := _buf.Bytes()")
			if a.Process != nil {
				e.emitProcessReverse(fn, unit, a.Process, "_raw")
			}
			if a.Size != nil {
				fn.printf("_size := int(%s)", e.expr(a.Size))
				fn.printf("if len(_raw) < _size {")
				fn.indent()
				fn.printf("_padded := make([]byte, _size)")
				fn.printf("copy(_padded, _raw)")
				fn.printf("_raw = _padded")
				fn.unindent()
				fn.printf("} else {")
				fn.indent()
				fn.printf("_raw = _raw[:_size]")
				fn.unindent()
				fn.printf("}")
			}
			fn.printf("if err = wstream.WriteBytes(_raw); err != nil { return err }")
			fn.unindent()
			fn.printf("}")
		} else {
			fn.printf("if err = %s.%s(wstream); err != nil { return err }", itemExpr, writeMethodName)
		}
	}

	switch repeat := a.Repeat.(type) {
	case types.RepeatExpr:
		fn.printf("for i, _item := range %s {", valExpr)
		fn.indent()
		fn.printf("_ = i")
		fn.printf("if i >= int(%s) { break }", e.expr(repeat.CountExpr))
		writeOne("_item")
		fn.unindent()
		fn.printf("}")
	case types.RepeatEOS:
		fn.printf("for i, _item := range %s {", valExpr)
		fn.indent()
		fn.printf("_ = i")
		writeOne("_item")
		fn.unindent()
		fn.printf("}")
	default:
		if rt.TypeRef != nil && rt.TypeRef.Kind == types.User {
			writeOne(valExpr)
		}
	}
}

// writeTypeSwitchCall generates a call to the type switch write helper.
func (e *Emitter) writeTypeSwitchCall(fn *goFunc, typ *engine.ExprValue) {
	a := typ.Attr
	fieldName := "this." + e.fieldName(a.ID)
	ts := a.Type.TypeSwitch

	helperName := "write" + e.typeSwitchName(a.ID)
	needsIndex := exprContainsIndex(ts.SwitchOn)
	if a.Repeat != nil {
		fn.printf("for _i, _item := range %s {", fieldName)
		fn.indent()
		fn.printf("_ = _i")
		if needsIndex {
			fn.printf("if err = this.%s(wstream, _item, _i); err != nil { return err }", helperName)
		} else {
			fn.printf("if err = this.%s(wstream, _item); err != nil { return err }", helperName)
		}
		fn.unindent()
		fn.printf("}")
	} else {
		if needsIndex {
			fn.printf("if err = this.%s(wstream, %s, 0); err != nil { return err }", helperName, fieldName)
		} else {
			fn.printf("if err = this.%s(wstream, %s); err != nil { return err }", helperName, fieldName)
		}
	}
}

// typeSwitchWrite generates the type switch write helper method.
func (e *Emitter) typeSwitchWrite(unit *goUnit, typ *engine.ExprValue, forcedEndian types.EndianKind) {
	a := typ.Attr
	ts := a.Type.TypeSwitch

	gs := e.currentStruct(typ)
	helperName := "write" + e.typeSwitchName(a.ID)

	inputs := []goVar{{name: "wstream", typ: "*" + kaitaiWriter}, {name: "val", typ: "any"}}
	needsIndex := exprContainsIndex(ts.SwitchOn)
	if needsIndex {
		inputs = append(inputs, goVar{name: "i", typ: "int"})
	}

	fn := goFunc{
		recv: goVar{name: "this", typ: "*" + gs},
		name: helperName,
		in:   inputs,
		out:  []goVar{{name: "err", typ: "error"}},
	}

	// Add stream/parent/root/index references for expressions that need them
	fn.printf("stream := this.IO_")
	fn.printf("_ = stream")
	fn.printf("_parent := this.Parent_")
	fn.printf("_ = _parent")
	fn.printf("_root := this.Root_")
	fn.printf("_ = _root")
	if !needsIndex {
		fn.printf("i := 0")
		fn.printf("_ = i")
	}

	// Determine the write method based on endianness
	writeMethodName := "Write"
	isSwitchEndian := e.endian == types.SwitchEndian
	if isSwitchEndian && forcedEndian == types.LittleEndian {
		writeMethodName = "WriteLE"
	} else if isSwitchEndian && forcedEndian == types.BigEndian {
		writeMethodName = "WriteBE"
	}

	// Resolve switch-on expression type for casting
	switchOnType := engine.ResultTypeOfExpr(e.context, ts.SwitchOn)
	typeCast := ""
	if switchOnType != nil {
		typeCast = e.declType(switchOnType)
	}
	// If the switch-on resolves to an enum, use the enum type for cases and switch expression
	isEnum := false
	if switchOnType != nil && switchOnType.Kind == engine.AttrKind {
		if switchOnType.Attr.Enum != "" {
			enumTyp := e.resolveType(switchOnType.Attr.Enum)
			typeCast = e.declType(enumTyp)
			isEnum = true
		}
	}

	// Check if we have byte array cases
	hasByteArrayCases := typeCast == "[]byte"
	if !hasByteArrayCases {
		for caseKey := range ts.Cases {
			if caseKey != "_" && strings.HasPrefix(caseKey, "[") {
				hasByteArrayCases = true
				break
			}
		}
	}

	// Also check if any case resolves to an enum value (fallback detection)
	if !isEnum {
		for caseKey := range ts.Cases {
			if caseKey == "_" {
				continue
			}
			ex := expr.MustParseExpr(caseKey)
			val := engine.ResultTypeOfExpr(e.context, ex)
			if val != nil && val.Parent != nil && val.Parent.Kind == engine.EnumValueKind {
				isEnum = true
				// Get the enum type for the cast
				enumVal := val.Parent
				if enumVal.EnumValue != nil && enumVal.Parent != nil && enumVal.Parent.Enum != nil {
					typeCast = e.enumTypeName(enumVal.Parent.Parent, enumVal.Parent.Enum)
				}
				break
			}
		}
	}

	switchOnExpr := e.expr(ts.SwitchOn)
	if isEnum {
		// For enum switches, cast to enum type so case values match
		switchOnExpr = fmt.Sprintf("(%s)(%s)", typeCast, switchOnExpr)
	} else if typeCast != "" && typeCast != "[]byte" {
		switchOnExpr = fmt.Sprintf("(%s)(%s)", typeCast, switchOnExpr)
	}

	if hasByteArrayCases {
		// Byte array cases: use if/else if with bytes.Equal
		hasNonDefaultCases := false
		for caseKey := range ts.Cases {
			if caseKey != "_" {
				hasNonDefaultCases = true
				break
			}
		}
		if hasNonDefaultCases {
			e.needBytes = true
		}
		first := true
		for caseKey := range ts.Cases {
			if caseKey == "_" {
				continue
			}
			caseExpr := expr.MustParseExpr(caseKey)
			caseStr := e.exprNode(caseExpr.Root)
			if _, ok := caseExpr.Root.(expr.StringNode); ok {
				caseStr = fmt.Sprintf("[]byte(%s)", caseStr)
			}
			if first {
				fn.printf("if bytes.Equal(%s, %s) {", switchOnExpr, caseStr)
				first = false
			} else {
				fn.printf("} else if bytes.Equal(%s, %s) {", switchOnExpr, caseStr)
			}
			fn.indent()
			e.writeTypeSwitchCaseRef(&fn, ts.Cases[caseKey], writeMethodName)
			fn.unindent()
		}
		if _, ok := ts.Cases["_"]; ok {
			if !first {
				fn.printf("} else {")
				fn.indent()
				e.writeTypeSwitchCaseRef(&fn, ts.Cases["_"], writeMethodName)
				fn.unindent()
			} else {
				// Only default case - write directly without if/else
				e.writeTypeSwitchCaseRef(&fn, ts.Cases["_"], writeMethodName)
			}
		} else if !first {
			// Fallback: write raw bytes for unmatched cases
			fn.printf("} else {")
			fn.indent()
			fn.printf("if _raw, ok := val.([]byte); ok {")
			fn.indent()
			fn.printf("if err = wstream.WriteBytes(_raw); err != nil { return err }")
			fn.unindent()
			fn.printf("}")
			fn.unindent()
		}
		if !first {
			fn.printf("}")
		}
	} else {
		fn.printf("switch %s {", switchOnExpr)
		for caseKey := range ts.Cases {
			if caseKey == "_" {
				continue
			}
			goValue := e.typeSwitchCaseValue(caseKey)
			fn.printf("case %s:", goValue)
			fn.indent()
			e.writeTypeSwitchCaseRef(&fn, ts.Cases[caseKey], writeMethodName)
			fn.unindent()
		}
		if _, ok := ts.Cases["_"]; ok {
			fn.printf("default:")
			fn.indent()
			e.writeTypeSwitchCaseRef(&fn, ts.Cases["_"], writeMethodName)
			fn.unindent()
		} else {
			// Fallback: write raw bytes for unmatched switch cases
			fn.printf("default:")
			fn.indent()
			fn.printf("if _raw, ok := val.([]byte); ok {")
			fn.indent()
			fn.printf("if err = wstream.WriteBytes(_raw); err != nil { return err }")
			fn.unindent()
			fn.printf("}")
			fn.unindent()
		}
		fn.printf("}")
	}

	fn.printf("return nil")
	unit.methods = append(unit.methods, fn)
}

// writeTypeSwitchCaseRef generates the write code for a single case in a type switch.
func (e *Emitter) writeTypeSwitchCaseRef(fn *goFunc, caseTypeRef types.TypeRef, writeMethodName string) {
	folded := caseTypeRef
	// Fold endian on the case type ref
	caseType := types.Type{TypeRef: &caseTypeRef}
	foldedType := caseType.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)
	if foldedType.TypeRef != nil {
		folded = *foldedType.TypeRef
	}

	if folded.Kind == types.User && folded.User != nil {
		// User type case: type assert and write
		resolved := e.resolveType(folded.User.Name)
		typeName := e.declType(resolved)
		fn.printf("if _v, ok := val.(*%s); ok {", typeName)
		fn.indent()
		fn.printf("if err = _v.%s(wstream); err != nil { return err }", writeMethodName)
		fn.unindent()
		fn.printf("}")
	} else {
		// Primitive case: cast and write
		valExpr := fmt.Sprintf("val.(%s)", e.declTypeRef(&folded, nil))
		writeCall := e.writeCallRefOn("wstream", &folded, valExpr)
		fn.printf("if err = %s; err != nil { return err }", writeCall)
	}
}

// emitProcessReverse applies the inverse of a process transformation.
func (e *Emitter) emitProcessReverse(fn *goFunc, unit *goUnit, process *expr.Expr, varName string) {
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
						fn.printf("%s = kaitai.ProcessXOR(%s, %s)", varName, varName, argStr)
					} else {
						fn.printf("%s = kaitai.ProcessXOR(%s, []byte{byte(%s)})", varName, varName, argStr)
					}
				}
				return
			case "rol":
				// Inverse of ROL is ROR
				if len(n.Args) > 0 {
					fn.printf("%s = kaitai.ProcessRotateRight(%s, int(%s))", varName, varName, e.exprNode(n.Args[0]))
				}
				return
			case "ror":
				// Inverse of ROR is ROL
				if len(n.Args) > 0 {
					fn.printf("%s = kaitai.ProcessRotateLeft(%s, int(%s))", varName, varName, e.exprNode(n.Args[0]))
				}
				return
			case "zlib":
				fn.printf("%s, err = kaitai.ProcessZlibCompress(%s)", varName, varName)
				fn.printf("if err != nil { return err }")
				return
			}
		}
		if id, ok := n.Object.(expr.IdentNode); ok {
			switch id.Identifier {
			case "xor":
				if len(n.Args) > 0 {
					argStr := e.exprNode(n.Args[0])
					if e.isNodeByteArray(n.Args[0]) {
						fn.printf("%s = kaitai.ProcessXOR(%s, %s)", varName, varName, argStr)
					} else {
						fn.printf("%s = kaitai.ProcessXOR(%s, []byte{byte(%s)})", varName, varName, argStr)
					}
				}
				return
			case "rol":
				if len(n.Args) > 0 {
					fn.printf("%s = kaitai.ProcessRotateRight(%s, int(%s))", varName, varName, e.exprNode(n.Args[0]))
				}
				return
			case "ror":
				if len(n.Args) > 0 {
					fn.printf("%s = kaitai.ProcessRotateLeft(%s, int(%s))", varName, varName, e.exprNode(n.Args[0]))
				}
				return
			case "zlib":
				fn.printf("%s, err = kaitai.ProcessZlibCompress(%s)", varName, varName)
				fn.printf("if err != nil { return err }")
				return
			}
		}
	case expr.IdentNode:
		switch n.Identifier {
		case "zlib":
			fn.printf("%s, err = kaitai.ProcessZlibCompress(%s)", varName, varName)
			fn.printf("if err != nil { return err }")
			return
		default:
			procType := e.typeName(kaitai.Identifier(n.Identifier))
			fn.printf("%s = New%s().Encode(%s)", varName, procType, varName)
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
			fn.printf("%s = New%s(%s).Encode(%s)", varName, procName, strings.Join(args, ", "), varName)
			return
		}
	}
	panic(fmt.Errorf("unsupported process expression: %s", process))
}

// currentStruct returns the full Go struct name for the parent of an attribute.
func (e *Emitter) currentStruct(typ *engine.ExprValue) string {
	if typ.Parent != nil && typ.Parent.Struct != nil {
		return e.prefix(typ.Parent.DefParent) + e.typeName(typ.Parent.Struct.Type.ID)
	}
	return ""
}

// endianStubsWrite generates WriteBE/WriteLE stubs that delegate to Write.
func (e *Emitter) endianStubsWrite(unit *goUnit, gs *goStruct) {
	unit.methods = append(unit.methods, goFunc{
		recv:   goVar{name: "this", typ: "*" + gs.name},
		name:   "WriteBE",
		in:     []goVar{{name: "wstream", typ: "*" + kaitaiWriter}},
		out:    []goVar{{name: "err", typ: "error"}},
		source: "\treturn this.Write(wstream)\n",
	})
	unit.methods = append(unit.methods, goFunc{
		recv:   goVar{name: "this", typ: "*" + gs.name},
		name:   "WriteLE",
		in:     []goVar{{name: "wstream", typ: "*" + kaitaiWriter}},
		out:    []goVar{{name: "err", typ: "error"}},
		source: "\treturn this.Write(wstream)\n",
	})
}

// endianSwitchWrite generates a Write() method that dispatches to WriteLE/WriteBE.
func (e *Emitter) endianSwitchWrite(unit *goUnit, gs *goStruct) {
	unit.methods = append(unit.methods, goFunc{
		recv:   goVar{name: "this", typ: "*" + gs.name},
		name:   "Write",
		in:     []goVar{{name: "wstream", typ: "*" + kaitaiWriter}},
		out:    []goVar{{name: "err", typ: "error"}},
		source: "\tif this._isLE {\n\t\treturn this.WriteLE(wstream)\n\t}\n\treturn this.WriteBE(wstream)\n",
	})
}
