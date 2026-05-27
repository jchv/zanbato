package c

import (
	"fmt"
	"math/big"
	"slices"
	"sort"
	"strings"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/emitter"
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/resolve"
	"github.com/jchv/zanbato/kaitai/types"
)

type fileScope struct {
	header   *buf
	source   *buf
	rootName string

	includes map[string]struct{}

	arrayTypes        map[string]string
	arrayTypesOrder   []string
	arrayTypesEmitted map[string]bool

	parents engine.ParentTypes

	customProcessSigs  map[string]string
	customProcessOrder []string

	opaqueIncludes []string

	auxLitCounter int
}

func newFileScope() *fileScope {
	return &fileScope{
		header:            &buf{},
		source:            &buf{},
		includes:          map[string]struct{}{},
		arrayTypes:        map[string]string{},
		arrayTypesEmitted: map[string]bool{},
		customProcessSigs: map[string]string{},
	}
}

type exprMode struct {
	thisPointerName   string
	sizeOverride      string
	writingContext    bool
	writerPosOffset   string
	bytesShadowField  string
	bytesShadowExpr   string
	repeatElemIsBytes bool
}

// Emitter produces C source/header pairs for kaitai structs.
type Emitter struct {
	resolver  resolve.Resolver
	context   *engine.Context
	endian    types.EndianKind
	bitEndian types.BitEndianKind
	compat    kaitai.Compatibility

	artifacts []emitter.Artifact
	visited   map[*kaitai.Struct]struct{}

	file *fileScope
	mode exprMode

	currentStruct     *engine.ExprValue
	currentStructName string

	debugAlways bool
	debug       bool
}

func (e *Emitter) saveExprMode() func() {
	prev := e.mode
	return func() { e.mode = prev }
}

func (e *Emitter) enterStruct(val *engine.ExprValue, name string) func() {
	prevStruct, prevName := e.currentStruct, e.currentStructName
	e.currentStruct = val
	e.currentStructName = name
	return func() {
		e.currentStruct = prevStruct
		e.currentStructName = prevName
	}
}

func (e *Emitter) redirectSource(dst *buf) func() {
	prev := e.file.source
	e.file.source = dst
	return func() { e.file.source = prev }
}

func (e *Emitter) pushFileScope(rootName string) func() {
	prev := e.file
	e.file = newFileScope()
	e.file.rootName = rootName
	return func() { e.file = prev }
}

func (e *Emitter) enterLocal(val *engine.ExprValue) func() {
	prev := e.context
	e.context = e.context.WithLocalRoot(val)
	return func() { e.context = prev }
}

func (e *Emitter) enterModuleLocal(val *engine.ExprValue) func() {
	prev := e.context
	e.context = e.context.WithModuleRoot(val).WithLocalRoot(val)
	return func() { e.context = prev }
}

func (e *Emitter) pushMetaScope(ks *kaitai.Struct) func() {
	prevEndian, prevBit, prevDebug := e.endian, e.bitEndian, e.debug
	if ks.Meta.Endian.Kind != types.UnspecifiedOrder {
		e.endian = ks.Meta.Endian.Kind
	}
	if ks.Meta.BitEndian.Kind != types.UnspecifiedBitOrder {
		e.bitEndian = ks.Meta.BitEndian.Kind
	}
	e.debug = e.debug || ks.Meta.Debug || e.debugAlways
	return func() {
		e.endian = prevEndian
		e.bitEndian = prevBit
		e.debug = prevDebug
	}
}

// NewEmitter constructs a new C emitter.
func NewEmitter(resolver resolve.Resolver) *Emitter {
	return &Emitter{
		resolver: resolver,
		context:  engine.NewContext(),
		compat:   engine.DefaultCompat,
		visited:  map[*kaitai.Struct]struct{}{},
	}
}

// SetCompat sets the compatibility mode for the emitter.
func (e *Emitter) SetCompat(c kaitai.Compatibility) { e.compat = c }

// SetDebug controls whether debug features are unconditionally enabled for
// all generated code.
func (e *Emitter) SetDebug(enabled bool) { e.debugAlways = enabled }

// Emit emits C code for the given kaitai struct.
func (e *Emitter) Emit(inputname string, s *kaitai.Struct) []emitter.Artifact {
	e.endian = types.UnspecifiedOrder
	e.bitEndian = types.UnspecifiedBitOrder
	e.context = engine.NewContext()
	e.context.Compat = e.compat
	e.artifacts = nil
	e.visited = map[*kaitai.Struct]struct{}{}
	e.root(inputname, s)
	return e.artifacts
}

// EmitSafe attempts to emit but recovers a panic if a panic occurs.
func (e *Emitter) EmitSafe(inputname string, s *kaitai.Struct) (out []emitter.Artifact, panicMsg string) {
	defer func() {
		if r := recover(); r != nil {
			panicMsg = fmt.Sprintf("%v", r)
		}
	}()
	out = e.Emit(inputname, s)
	return
}

func (e *Emitter) RegisterKnownStructs(known map[string]*kaitai.Struct) {
	for id, s := range known {
		if s == nil {
			continue
		}
		if e.context.ResolveGlobalType(id) != nil {
			continue
		}
		rootType := engine.NewStructSymbol(s, nil)
		e.context.AddGlobalType(id, rootType)
		e.context.AddModuleType(id, rootType)
	}
}

func (e *Emitter) root(inputname string, s *kaitai.Struct) {
	if _, ok := e.visited[s]; ok {
		return
	}
	e.visited[s] = struct{}{}

	defer e.pushMetaScope(s)()

	rootType := engine.NewStructSymbol(s, nil)
	root := engine.NewStructValueSymbol(rootType, nil)
	e.context.AddGlobalType(string(s.ID), rootType)
	e.context.AddModuleType(string(s.ID), rootType)
	defer e.enterModuleLocal(root)()

	defer e.pushFileScope(e.typeName(s.ID))()
	e.file.parents = engine.BuildParentTypeMap(e.context, rootType)
	header, source := e.file.header, e.file.source

	if s.Meta.OpaqueTypes {
		e.resolveOpaqueRefs(inputname, s)
	}

	guard := strings.ToUpper(e.typeName(s.ID)) + "_H_"
	headerName := e.filename(s.ID) + ".h"
	sourceName := e.filename(s.ID) + ".c"

	header.pf("/* Generated by Zanbato. Do not edit! */")
	header.blank()
	header.pf("#ifndef %s", guard)
	header.pf("#define %s", guard)
	header.blank()
	header.pf("#include \"zanbato.h\"")
	header.blank()
	header.pf("#ifdef __cplusplus")
	header.pf("extern \"C\" {")
	header.pf("#endif")
	header.blank()

	source.pf("/* Generated by Zanbato. Do not edit! */")
	source.blank()
	source.pf("#include \"%s\"", headerName)
	source.pf("#include <string.h>")
	source.blank()

	collectAllStructs(root, func(v *engine.ExprValue) {
		if v.Struct == nil || v.Struct.Type == nil {
			return
		}
		name := e.prefix(v.DefParent) + e.typeName(v.Struct.Type.ID)
		header.pf("struct %s;", name)
		header.pf("typedef struct %s %s_t;", name, name)
	})
	header.blank()

	for _, n := range s.Meta.Imports {
		impName, _, err := e.resolver.Resolve(inputname, n)
		if err != nil {
			panic(fmt.Errorf("resolving import %q: %w", n, err))
		}
		header.pf("#include \"%s.h\"", strings.ToLower(filepathBase(impName)))
	}
	if len(e.file.opaqueIncludes) > 0 {
		sort.Strings(e.file.opaqueIncludes)
		for _, inc := range e.file.opaqueIncludes {
			header.pf("#include \"%s.h\"", inc)
		}
	}
	if len(s.Meta.Imports) > 0 || len(e.file.opaqueIncludes) > 0 {
		header.blank()
	}

	e.struc(inputname, root)

	if len(e.file.customProcessOrder) > 0 {
		for _, name := range e.file.customProcessOrder {
			header.pf("%s", e.file.customProcessSigs[name])
		}
		header.blank()
	}

	header.pf("#ifdef __cplusplus")
	header.pf("} /* extern \"C\" */")
	header.pf("#endif")
	header.pf("#endif /* %s */", guard)

	e.artifacts = append(e.artifacts, emitter.Artifact{
		Filename: headerName,
		Body:     []byte(header.String()),
	})
	e.artifacts = append(e.artifacts, emitter.Artifact{
		Filename: sourceName,
		Body:     []byte(source.String()),
	})
}

func (e *Emitter) structContextName(val *engine.ExprValue) string {
	return e.prefix(val.DefParent) + e.typeName(val.Struct.Type.ID)
}

func (e *Emitter) registerArrayType(tag, elem string) {
	if e.file.arrayTypes == nil {
		e.file.arrayTypes = map[string]string{}
	}
	if _, ok := e.file.arrayTypes[tag]; ok {
		return
	}
	e.file.arrayTypes[tag] = elem
	e.file.arrayTypesOrder = append(e.file.arrayTypesOrder, tag)
}

func arrayTagGuard(tag string) string {
	t := strings.TrimPrefix(tag, "struct ")
	out := strings.Builder{}
	out.WriteString("ZB_DEFINED_")
	for _, r := range t {
		if r >= 'a' && r <= 'z' {
			out.WriteRune(r - ('a' - 'A'))
		} else if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
		} else {
			out.WriteRune('_')
		}
	}
	return out.String()
}

func (e *Emitter) arrayTagFor(elem string) string {
	clean := elem
	clean = strings.ReplaceAll(clean, " ", "_")
	clean = strings.ReplaceAll(clean, "*", "p")
	tag := "struct zb_arr_" + clean
	e.registerArrayType(tag, elem)
	return tag
}

func (e *Emitter) struc(inputname string, val *engine.ExprValue) {
	ks := val.Struct.Type
	defer e.enterLocal(val)()
	defer e.pushMetaScope(ks)()

	for _, n := range ks.Meta.Imports {
		impName, impStruc, err := e.resolver.Resolve(inputname, n)
		if err != nil {
			panic(fmt.Errorf("resolving import %q: %w", n, err))
		}
		e.root(impName, impStruc)
	}

	for _, en := range val.Struct.Enums {
		if en.Enum == nil {
			continue
		}
		e.emitEnum(val, en.Enum)
	}

	for _, n := range val.Struct.Structs {
		if n.Struct == nil || n.Struct.Type == nil {
			continue
		}
		e.struc(inputname, engine.NewStructValueSymbol(n, val))
	}

	e.emitStructDecl(val)
	e.emitReadFunc(val)
	e.emitInstanceGetters(val)
	e.emitWriteFunc(val)
}

func (e *Emitter) emitInstanceGetters(val *engine.ExprValue) {
	typeName := e.structContextName(val)
	defer e.enterStruct(val, typeName)()
	for _, inst := range val.Struct.Instances {
		a := inst.Instance
		if a == nil {
			continue
		}
		if a.Value != nil {
			e.emitValueInstanceGetter(val, typeName, a)
			continue
		}
		if a.Pos != nil {
			e.emitPosInstanceGetter(val, typeName, a)
			continue
		}
	}
}

func (e *Emitter) emitValueInstanceGetter(val *engine.ExprValue, typeName string, a *kaitai.Attr) {
	retType := e.inferValueType(a.Value)
	decl := fmt.Sprintf("%s %s_get_%s(struct %s *this_)", retType, typeName, e.fieldName(a.ID), typeName)
	e.file.header.pf("%s;", decl)
	e.file.source.pf("%s {", decl)
	e.file.source.indent()
	e.file.source.pf("if (!this_->_f_%s) {", e.fieldName(a.ID))
	e.file.source.indent()
	restoreCtx := e.enterLocal(val)
	if a.If != nil {
		ifExpr := e.expr(a.If)
		e.file.source.pf("if (!(%s)) {", ifExpr)
		e.file.source.indent()
		e.file.source.pf("this_->_n_%s = 1;", e.fieldName(a.ID))
		e.file.source.pf("this_->_f_%s = 1;", e.fieldName(a.ID))
		e.file.source.unindent()
		e.file.source.pf("} else {")
		e.file.source.indent()
	}
	valExpr := e.expr(a.Value)
	restoreCtx()
	if isScalarType(retType) {
		e.file.source.pf("this_->%s = (%s)(%s);", e.fieldName(a.ID), retType, valExpr)
	} else if retType == "zb_bytes_t" {
		e.file.source.pf("this_->%s = zb_bytes_dup(this_->_arena, (%s));", e.fieldName(a.ID), valExpr)
	} else {
		e.file.source.pf("this_->%s = (%s);", e.fieldName(a.ID), valExpr)
	}
	e.file.source.pf("this_->_f_%s = 1;", e.fieldName(a.ID))
	if a.If != nil {
		e.file.source.unindent()
		e.file.source.pf("}")
	}
	e.file.source.unindent()
	e.file.source.pf("}")
	e.file.source.pf("return this_->%s;", e.fieldName(a.ID))
	e.file.source.unindent()
	e.file.source.pf("}")
	e.file.source.blank()
}

func (e *Emitter) emitPosInstanceGetter(val *engine.ExprValue, typeName string, a *kaitai.Attr) {
	retType := e.declFieldType(a)
	if retType == "" {
		retType = "void *"
	}
	helperName := fmt.Sprintf("%s_load_%s", typeName, e.fieldName(a.ID))
	getterName := fmt.Sprintf("%s_get_%s", typeName, e.fieldName(a.ID))

	helperDecl := fmt.Sprintf("static int %s(struct %s *this_)", helperName, typeName)
	e.file.source.pf("%s {", helperDecl)
	e.file.source.indent()
	restoreCtx := e.enterLocal(val)
	posExpr := e.expr(a.Pos)
	var ioExpr string
	if a.IO != nil {
		ioExpr = e.expr(a.IO)
	}
	restoreCtx()
	streamVar := "this_->_io"
	if ioExpr != "" {
		streamVar = "(" + ioExpr + ")"
	}
	e.file.source.pf("int _err; (void)_err;")
	e.file.source.pf("int _endian = this_->_endian; (void)_endian;")
	e.file.source.pf("zb_stream_t *_save_io = this_->_io;")
	e.file.source.pf("zb_stream_t *stream = %s;", streamVar)
	e.file.source.pf("zb_arena_t *arena = this_->_arena;")
	e.file.source.pf("(void)arena;")
	rt := a.Type.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)
	prevSize := e.mode.sizeOverride
	if a.Size != nil {
		e.file.source.pf("size_t _pre_sub_n = (size_t)(%s);", e.expr(a.Size))
		e.mode.sizeOverride = "_pre_sub_n"
	}
	defer func() { e.mode.sizeOverride = prevSize }()
	e.file.source.pf("size_t _save_pos = zb_stream_pos(stream);")
	emitTry(e.file.source, "zb_stream_seek(stream, (size_t)(%s))", posExpr)
	e.file.source.pf("this_->_io = stream;")
	if a.Contents != nil {
		e.file.source.pf("{")
		e.file.source.indent()
		e.file.source.pf("zb_bytes_t _magic;")
		emitTry(e.file.source, "zb_read_bytes(stream, arena, %d, &_magic)", len(a.Contents))
		e.file.source.pf("static const uint8_t _expected[] = {%s};", byteArrayInitializer(a.Contents))
		e.file.source.pf("if (_magic.len != %d || memcmp(_magic.data, _expected, %d) != 0) return ZB_ERR_VALIDATION;", len(a.Contents), len(a.Contents))
		e.file.source.unindent()
		e.file.source.pf("}")
	} else if rt.TypeRef != nil {
		if a.Repeat != nil {
			e.emitRepeatRead(e.file.source, val, a, rt.TypeRef, e.fieldName(a.ID))
		} else {
			e.emitSingleRead(e.file.source, rt.TypeRef, e.fieldName(a.ID), a)
		}
	} else if rt.TypeSwitch != nil {
		if a.Repeat != nil {
			e.emitRepeatTypeSwitchRead(e.file.source, val, a, rt.TypeSwitch, e.fieldName(a.ID))
		} else {
			e.emitTypeSwitchRead(e.file.source, val, a, rt.TypeSwitch, e.fieldName(a.ID))
		}
	}

	if a.Valid != nil && rt.TypeRef != nil {
		if a.Repeat != nil {
			field := e.fieldName(a.ID)
			elem := e.declTypeRef(rt.TypeRef, false)
			e.file.source.pf("{")
			e.file.source.indent()
			e.file.source.pf("for (size_t _vi = 0; _vi < this_->%s.len; _vi++) {", field)
			e.file.source.indent()
			e.file.source.pf("%s _v_elem = this_->%s.data[_vi];", elem, field)
			e.file.source.pf("(void)_v_elem;")
			e.emitValidateOn(e.file.source, a, "_v_elem", rt.TypeRef)
			e.file.source.unindent()
			e.file.source.pf("}")
			e.file.source.unindent()
			e.file.source.pf("}")
		} else {
			e.emitValidate(e.file.source, a, e.fieldName(a.ID), rt.TypeRef)
		}
	}
	e.file.source.pf("(void)zb_stream_seek(stream, _save_pos);")
	e.file.source.pf("this_->_io = _save_io;")
	e.file.source.pf("return ZB_OK;")
	e.file.source.unindent()
	e.file.source.pf("}")
	e.file.source.blank()

	decl := fmt.Sprintf("%s %s(struct %s *this_)", retType, getterName, typeName)
	e.file.header.pf("%s;", decl)
	e.file.source.pf("%s {", decl)
	e.file.source.indent()
	e.file.source.pf("if (!this_->_f_%s) {", e.fieldName(a.ID))
	e.file.source.indent()
	e.file.source.pf("int _e = %s(this_);", helperName)
	e.file.source.pf("if (_e && !this_->_inst_err) this_->_inst_err = _e;")
	e.file.source.pf("this_->_f_%s = 1;", e.fieldName(a.ID))
	e.file.source.unindent()
	e.file.source.pf("}")
	e.file.source.pf("return this_->%s;", e.fieldName(a.ID))
	e.file.source.unindent()
	e.file.source.pf("}")
	e.file.source.blank()
}

func (e *Emitter) emitEnum(parent *engine.ExprValue, en *kaitai.Enum) {
	enumPrefix := e.prefix(parent) + e.typeName(en.ID) + "__"
	e.file.header.pf("enum {")
	e.file.header.indent()
	for _, v := range en.Values {
		e.file.header.pf("%s%s = %s,", enumPrefix, e.typeName(v.ID), v.Value.String())
	}
	e.file.header.unindent()
	e.file.header.pf("};")
	e.file.header.blank()
}

func (e *Emitter) emitStructDecl(val *engine.ExprValue) {
	ks := val.Struct.Type
	name := e.prefix(val.DefParent) + e.typeName(ks.ID)
	defer e.enterStruct(val, name)()

	gs := &cStruct{name: name, doc: ks.Doc}

	for _, p := range ks.Params {
		gs.fields = append(gs.fields, cField{
			typ:  e.declTypeRefForParam(&p.Type),
			name: e.fieldName(p.ID),
		})
	}

	for _, attr := range val.Struct.Attrs {
		e.appendSeqAttrFields(gs, attr.Attr)
	}

	for _, inst := range val.Struct.Instances {
		if inst.Instance == nil {
			continue
		}
		e.appendInstanceFields(gs, inst.Instance)
	}

	parentType := e.parentCType(ks)
	if parentType == "" {
		parentType = "void *"
	}
	gs.fields = append(gs.fields,
		cField{typ: "zb_stream_t *", name: "_io"},
		cField{typ: "struct " + e.file.rootName + " *", name: "_root"},
		cField{typ: parentType, name: "_parent"},
		cField{typ: "zb_arena_t *", name: "_arena"},
		cField{typ: "int", name: "_inst_err"},
		cField{typ: "int", name: "_endian"},
	)
	if e.debug {
		gs.fields = append(gs.fields, cField{typ: "zb_debug_info_t", name: "_debug"})
	}
	for _, slot := range engine.BitAlignSlots(ks) {
		gs.fields = append(gs.fields, cField{typ: "uint64_t", name: bitAlignFieldName(slot)})
	}

	for _, tag := range e.file.arrayTypesOrder {
		if e.file.arrayTypesEmitted[tag] {
			continue
		}
		elem := e.file.arrayTypes[tag]
		guard := arrayTagGuard(tag)
		e.file.header.pf("#ifndef %s", guard)
		e.file.header.pf("#define %s", guard)
		e.file.header.pf("%s { %s *data; size_t len; size_t cap; };", tag, elem)
		e.file.header.pf("#endif")
		e.file.arrayTypesEmitted[tag] = true
	}
	if len(e.file.arrayTypesOrder) > 0 {
		e.file.header.blank()
	}

	gs.emit(e.file.header)
}

func (e *Emitter) declFieldType(a *kaitai.Attr) string {
	rt := a.Type
	if a.Repeat != nil {
		var base string
		if rt.TypeSwitch != nil && rt.TypeRef == nil {
			base = "void *"
		} else {
			base = e.declTypeRef(rt.TypeRef, false)
		}
		if base == "" {
			return ""
		}
		return e.arrayTagFor(base)
	}
	if rt.TypeRef == nil && rt.TypeSwitch != nil {
		if uni := uniformSwitchCase(rt.TypeSwitch); uni != nil {
			return e.declTypeRef(uni, false)
		}
		if widened := integerWidenedSwitchCase(rt.TypeSwitch); widened != nil {
			return e.declTypeRef(widened, false)
		}
		return "void *"
	}
	if a.Value != nil {
		if t := e.inferValueType(a.Value); t != "" {
			return t
		}
	}
	if rt.TypeRef != nil {
		return e.declTypeRef(rt.TypeRef, false)
	}
	return ""
}

func (e *Emitter) inferValueType(ex *expr.Expr) string {
	if ex == nil {
		return "int64_t"
	}
	if t := e.inferValueTypeFromNode(ex.Root); t != "" {
		return t
	}
	result := engine.ResultTypeOfExpr(e.context, ex)
	if result != nil {
		switch result.Kind {
		case engine.IntegerKind:
			return "int64_t"
		case engine.FloatKind:
			return "double"
		case engine.BooleanKind:
			return "int"
		case engine.StructKind:
			if result.Struct != nil && result.Struct.Type != nil {
				return e.prefix(result.DefParent) + e.typeName(result.Struct.Type.ID) + "_t *"
			}
		case engine.ArrayKind:
			if vt, ok := result.ValueType(); ok && vt.Type.TypeRef != nil {
				if c := e.declTypeRef(vt.Type.TypeRef, false); c != "" {
					return e.arrayTagFor(c)
				}
			}
		}
		if vt, ok := result.ValueType(); ok && vt.Type.TypeRef != nil {
			if vt.Type.TypeRef.Kind != types.Bytes {
				if c := e.declTypeRef(vt.Type.TypeRef, false); c != "" {
					if vt.Repeat != nil {
						return e.arrayTagFor(c)
					}
					return c
				}
			}
		}
	}
	return "int64_t"
}

func (e *Emitter) engineResolveType(n expr.Node) (out string) {
	defer func() { _ = recover() }()
	result := engine.ResultTypeOfNode(e.context, n)
	if result == nil {
		return ""
	}
	if result.Kind == engine.InstanceKind && result.Instance != nil && result.Instance.Value != nil {
		if t := e.inferValueType(result.Instance.Value); t != "" {
			return t
		}
	}
	if vt, ok := result.ValueType(); ok && vt.Type.TypeRef != nil {
		if c := e.declTypeRef(vt.Type.TypeRef, false); c != "" {
			if vt.Repeat != nil {
				return e.arrayTagFor(c)
			}
			return c
		}
	}
	switch result.Kind {
	case engine.IntegerKind:
		return "int64_t"
	case engine.FloatKind:
		return "double"
	case engine.BooleanKind:
		return "int"
	case engine.StringKind, engine.ByteArrayKind:
		return "zb_bytes_t"
	case engine.StructKind:
		if result.Struct != nil && result.Struct.Type != nil {
			return e.prefix(result.DefParent) + e.typeName(result.Struct.Type.ID) + "_t *"
		}
	case engine.ArrayKind:
		if vt, ok := result.ValueType(); ok && vt.Type.TypeRef != nil {
			if c := e.declTypeRef(vt.Type.TypeRef, false); c != "" {
				return e.arrayTagFor(c)
			}
		}
	}
	return ""
}

func (e *Emitter) inferValueTypeFromNode(n expr.Node) string {
	switch t := n.(type) {
	case expr.IntNode:
		return "int64_t"
	case expr.FloatNode:
		return "double"
	case expr.BoolNode:
		return "int"
	case expr.StringNode:
		return "zb_bytes_t"
	case expr.FStringNode:
		return "zb_bytes_t"
	case expr.UnaryNode:
		if t.Op == expr.OpLogicalNot {
			return "int"
		}
		return e.inferValueTypeFromNode(t.Operand)
	case expr.BinaryNode:
		switch t.Op {
		case expr.OpEqual, expr.OpNotEqual,
			expr.OpLessThan, expr.OpLessThanEqual,
			expr.OpGreaterThan, expr.OpGreaterThanEqual,
			expr.OpLogicalAnd, expr.OpLogicalOr:
			return "int"
		case expr.OpAdd:
			a := e.inferValueTypeFromNode(t.A)
			if a == "" {
				a = e.engineResolveType(t.A)
			}
			b := e.inferValueTypeFromNode(t.B)
			if b == "" {
				b = e.engineResolveType(t.B)
			}
			if a == "zb_bytes_t" || b == "zb_bytes_t" {
				return "zb_bytes_t"
			}
			if a == "double" || b == "double" || a == "float" || b == "float" {
				return "double"
			}
			return "int64_t"
		case expr.OpSub, expr.OpMult, expr.OpDiv, expr.OpMod,
			expr.OpBitAnd, expr.OpBitOr, expr.OpBitXor,
			expr.OpShiftLeft, expr.OpShiftRight:
			a := e.inferValueTypeFromNode(t.A)
			if a == "" {
				a = e.engineResolveType(t.A)
			}
			b := e.inferValueTypeFromNode(t.B)
			if b == "" {
				b = e.engineResolveType(t.B)
			}
			if a == "double" || b == "double" || a == "float" || b == "float" {
				return "double"
			}
			return "int64_t"
		}
	case expr.TernaryNode:
		b := e.inferValueTypeFromNode(t.B)
		c := e.inferValueTypeFromNode(t.C)
		if b == "" {
			b = e.engineResolveType(t.B)
		}
		if c == "" {
			c = e.engineResolveType(t.C)
		}
		if b == c && b != "" {
			return b
		}
		if b != "" {
			return b
		}
		if c != "" {
			return c
		}
	case expr.CastNode:
		if strings.HasSuffix(t.TypeName, "[]") {
			if ref, err := types.ParseTypeRef(t.TypeName[:len(t.TypeName)-2]); err == nil {
				if c := e.declTypeRef(&ref, false); c != "" {
					return e.arrayTagFor(c)
				}
			}
			return ""
		}
		switch t.TypeName {
		case "u1":
			return "uint8_t"
		case "u2":
			return "uint16_t"
		case "u4":
			return "uint32_t"
		case "u8":
			return "uint64_t"
		case "s1":
			return "int8_t"
		case "s2":
			return "int16_t"
		case "s4":
			return "int32_t"
		case "s8":
			return "int64_t"
		case "bytes":
			return "zb_bytes_t"
		case "str":
			return "zb_bytes_t"
		case "f4":
			return "float"
		case "f8":
			return "double"
		}
		if r := e.resolveQualifiedType(t.TypeName); r != nil && r.Struct != nil && r.Struct.Type != nil {
			return e.prefix(r.DefParent) + e.typeName(r.Struct.Type.ID) + "_t *"
		}
	case expr.SizeofNode, expr.BitSizeofNode:
		return "int64_t"
	case expr.SubscriptNode:
		if et := e.arrayElemCTypeOfNode(t.A); et != "" {
			return et
		}
	case expr.CallNode:
		if mn, ok := t.Object.(expr.MemberNode); ok {
			if mn.Property == "to_s" || mn.Property == "substring" {
				return "zb_bytes_t"
			}
		}
	case expr.MemberNode:
		switch t.Property {
		case "to_s", "substring", "reverse":
			return "zb_bytes_t"
		case "_io":
			return "zb_stream_t *"
		case "length", "size":
			return "int64_t"
		case "first", "last", "min", "max":
			if et := e.arrayElemCTypeOfNode(t.Operand); et != "" {
				return et
			}
			return ""
		}
	case expr.ArrayNode:
		allBytes := true
		allStrings := true
		allInts := true
		hasFloat := false
		for _, item := range t.Items {
			switch n := item.(type) {
			case expr.StringNode:
				allBytes = false
				allInts = false
			case expr.IntNode:
				allStrings = false
				if n.Integer.Sign() < 0 || n.Integer.Cmp(big.NewInt(255)) > 0 {
					allBytes = false
				}
			case expr.FloatNode:
				allBytes = false
				allStrings = false
				hasFloat = true
			default:
				allBytes = false
				allStrings = false
				allInts = false
			}
		}
		if allBytes {
			return "zb_bytes_t"
		}
		if allStrings {
			return e.arrayTagFor("zb_bytes_t")
		}
		if allInts {
			elem := "int64_t"
			if hasFloat {
				elem = "double"
			}
			return e.arrayTagFor(elem)
		}
		allStructs := true
		for _, item := range t.Items {
			if !e.exprIsPointer(item) {
				allStructs = false
				break
			}
		}
		if allStructs && len(t.Items) > 0 {
			return e.arrayTagFor("void *")
		}
		allStreams := true
		for _, item := range t.Items {
			if !e.exprIsStreamPointer(item) {
				allStreams = false
				break
			}
		}
		if allStreams && len(t.Items) > 0 {
			return e.arrayTagFor("zb_stream_t *")
		}
	}
	return ""
}

func (e *Emitter) declTypeRefForParam(t *types.TypeRef) string {
	if t != nil && t.Kind == types.User && t.User != nil {
		switch t.User.Name {
		case "io":
			if t.IsArray {
				return e.arrayTagFor("zb_stream_t *")
			}
			return "zb_stream_t *"
		case "struct":
			if t.IsArray {
				return e.arrayTagFor("void *")
			}
			return "void *"
		}
	}
	if s := e.declTypeRef(t, false); s != "" {
		return s
	}
	return "void *"
}

func (e *Emitter) declTypeRef(t *types.TypeRef, inExpr bool) string {
	_ = inExpr
	if t == nil {
		return ""
	}
	if t.IsArray {
		inner := *t
		inner.IsArray = false
		base := e.declTypeRef(&inner, inExpr)
		if base == "" {
			return ""
		}
		return e.arrayTagFor(base)
	}
	switch t.Kind {
	case types.U1:
		return "uint8_t"
	case types.U2, types.U2le, types.U2be:
		return "uint16_t"
	case types.U4, types.U4le, types.U4be:
		return "uint32_t"
	case types.U8, types.U8le, types.U8be:
		return "uint64_t"
	case types.S1:
		return "int8_t"
	case types.S2, types.S2le, types.S2be:
		return "int16_t"
	case types.S4, types.S4le, types.S4be:
		return "int32_t"
	case types.S8, types.S8le, types.S8be:
		return "int64_t"
	case types.F4, types.F4le, types.F4be:
		return "float"
	case types.F8, types.F8le, types.F8be:
		return "double"
	case types.UntypedBool:
		return "int"
	case types.Bits:
		if t.Bits != nil && t.Bits.Width == 1 {
			return "int"
		}
		return "uint64_t"
	case types.Bytes:
		return "zb_bytes_t"
	case types.String:
		return "zb_bytes_t"
	case types.User:
		if t.User == nil {
			return ""
		}
		typ := e.tryResolveType(t.User.Name)
		if typ == nil {
			return ""
		}
		switch typ.Kind {
		case engine.StructKind:
			return e.prefix(typ.DefParent) + e.typeName(typ.Struct.Type.ID) + "_t *"
		case engine.EnumKind:
			return "int /* enum " + e.typeName(typ.Enum.ID) + " */"
		}
	}
	return ""
}

func (e *Emitter) tryResolveType(name string) *engine.ExprValue {
	return e.tryResolveTypeInScope(name, nil)
}

func (e *Emitter) resolveOpaqueRefs(inputname string, ks *kaitai.Struct) {
	visit := func(name string) {
		if name == "" || strings.Contains(name, "::") {
			return
		}
		if t := e.tryResolveType(name); t != nil {
			return
		}
		impName, impStruc, err := e.resolver.Resolve(inputname, name)
		if err != nil || impStruc == nil {
			return
		}
		e.root(impName, impStruc)
		base := strings.ToLower(filepathBase(impName))
		if slices.Contains(e.file.opaqueIncludes, base) {
			return
		}
		e.file.opaqueIncludes = append(e.file.opaqueIncludes, base)
	}
	walkAttr := func(a *kaitai.Attr) {
		if a == nil {
			return
		}
		if a.Type.TypeRef != nil && a.Type.TypeRef.Kind == types.User && a.Type.TypeRef.User != nil {
			visit(a.Type.TypeRef.User.Name)
		}
		if a.Type.TypeSwitch != nil {
			for _, c := range a.Type.TypeSwitch.Cases {
				if c.Kind == types.User && c.User != nil {
					visit(c.User.Name)
				}
			}
		}
	}
	for _, a := range ks.Seq {
		walkAttr(a)
	}
	for _, a := range ks.Instances {
		walkAttr(a)
	}
	for _, child := range ks.Structs {
		e.resolveOpaqueRefs(inputname, child)
	}
}

func (e *Emitter) tryResolveTypeInScope(name string, scope *engine.ExprValue) *engine.ExprValue {
	defer func() { _ = recover() }()
	if strings.Contains(name, "::") {
		return e.resolveQualifiedType(name)
	}
	for s := scope; s != nil; s = s.DefParent {
		if t := s.TypeChild(name); t != nil {
			return t
		}
	}
	typ, _ := e.context.ResolveType(name)
	return typ
}

func (e *Emitter) emitReadFunc(val *engine.ExprValue) {
	ks := val.Struct.Type
	name := e.prefix(val.DefParent) + e.typeName(ks.ID)
	defer e.enterStruct(val, name)()

	params := e.buildReadParams(name, ks)
	decl := fmt.Sprintf("int %s_read(%s)", name, params)
	e.file.header.pf("%s;", decl)

	src := buf{}
	restoreSrc := e.redirectSource(&src)
	src.indent()
	src.p("memset(this_, 0, sizeof(*this_));")
	src.p("this_->_io = stream;")
	parentType := e.parentCType(ks)
	if parentType != "" {
		src.pf("this_->_parent = (%s)parent_;", parentType)
	} else {
		src.p("this_->_parent = parent_;")
	}
	src.p("this_->_arena = arena;")
	if parentType != "" && parentType != "void *" {
		src.p("int _endian = this_->_parent ? this_->_parent->_endian : 0; (void)_endian;")
	} else {
		src.p("int _endian = 0; (void)_endian;")
	}
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
		} else if wrote {
			src.pf("else return ZB_ERR_VALIDATION;")
		}
	} else if ks.Meta.Endian.Kind == types.LittleEndian {
		src.p("_endian = 0;")
	} else if ks.Meta.Endian.Kind == types.BigEndian {
		src.p("_endian = 1;")
	}
	src.p("this_->_endian = _endian;")
	src.pf("this_->_root = (struct %s *)root_;", e.file.rootName)
	src.p("(void)this_->_root;")
	for _, p := range ks.Params {
		src.pf("this_->%s = p_%s;", e.fieldName(p.ID), e.fieldName(p.ID))
	}

	alignSlots := engine.BitAlignSlots(ks)
	for i, attr := range val.Struct.Attrs {
		for len(alignSlots) > 0 && alignSlots[0].BeforeAttr == i {
			e.emitAlignRead(&src, alignSlots[0], ks)
			alignSlots = alignSlots[1:]
		}
		e.emitAttrRead(&src, val, attr.Attr)
	}
	for len(alignSlots) > 0 {
		e.emitAlignRead(&src, alignSlots[0], ks)
		alignSlots = alignSlots[1:]
	}

	if len(ks.Instances) > 0 {
		src.p("if (this_->_inst_err) return this_->_inst_err;")
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

func (e *Emitter) emitAlignRead(src *buf, slot engine.BitAlignSlot, ks *kaitai.Struct) {
	endian := e.alignReadEndian(ks, slot.BeforeAttr)
	src.pf("ZB_TRY(zb_read_bits_%s(stream, %d, &this_->%s));",
		endian, slot.Width, bitAlignFieldName(slot))
}

func (e *Emitter) alignReadEndian(ks *kaitai.Struct, beforeAttr int) string {
	for i := beforeAttr - 1; i >= 0; i-- {
		a := ks.Seq[i]
		if a == nil {
			continue
		}
		rt := a.Type.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)
		if rt.TypeRef == nil || rt.TypeRef.Kind != types.Bits || rt.TypeRef.Bits == nil {
			continue
		}
		if rt.TypeRef.Bits.Endian.Kind == types.LittleBitEndian {
			return "le"
		}
		return "be"
	}
	if e.bitEndian == types.LittleBitEndian {
		return "le"
	}
	return "be"
}

func (e *Emitter) buildReadParams(name string, ks *kaitai.Struct) string {
	var base strings.Builder
	fmt.Fprintf(&base, "struct %s *this_, zb_stream_t *stream, zb_arena_t *arena, void *parent_, void *root_", name)
	for _, p := range ks.Params {
		fmt.Fprintf(&base, ", %s p_%s", e.declTypeRefForParam(&p.Type), e.fieldName(p.ID))
	}
	return base.String()
}

func (e *Emitter) emitAttrRead(src *buf, parent *engine.ExprValue, a *kaitai.Attr) {
	if a.If != nil {
		src.pf("this_->_have_%s = (%s) ? 1 : 0;", e.fieldName(a.ID), e.expr(a.If))
		src.pf("if (this_->_have_%s) {", e.fieldName(a.ID))
		src.indent()
		defer func() {
			src.unindent()
			src.p("}")
		}()
	}

	ksID := string(a.ID)
	e.debugAttrStart(src, ksID)
	if a.Repeat != nil {
		e.debugArrInit(src, ksID)
	}
	defer e.debugAttrEnd(src, ksID)

	if a.Contents != nil {
		field := e.fieldName(a.ID)
		emitMagic := func() {
			src.pf("{")
			src.indent()
			src.pf("zb_bytes_t _magic;")
			emitTry(src, "zb_read_bytes(stream, arena, %d, &_magic)", len(a.Contents))
			src.pf("static const uint8_t _expected[] = {%s};", byteArrayInitializer(a.Contents))
			src.pf("if (_magic.len != %d || memcmp(_magic.data, _expected, %d) != 0) return ZB_ERR_VALIDATION;", len(a.Contents), len(a.Contents))
			if a.Repeat == nil {
				src.pf("this_->%s = _magic;", field)
			} else {
				src.pf("if (zb_array_grow_impl(arena, (void **)&this_->%s.data, &this_->%s.cap, this_->%s.len+1, sizeof(*this_->%s.data)) != ZB_OK) return ZB_ERR_ALLOC;",
					field, field, field, field)
				src.pf("this_->%s.data[this_->%s.len++] = _magic;", field, field)
			}
			src.unindent()
			src.pf("}")
		}
		if a.Repeat == nil {
			emitMagic()
			return
		}
		src.pf("{ size_t _i; (void)_i;")
		src.indent()
		switch r := a.Repeat.(type) {
		case types.RepeatExpr:
			src.pf("size_t _count = (size_t)(%s);", e.expr(r.CountExpr))
			src.pf("for (_i = 0; _i < _count; _i++) {")
		case types.RepeatEOS:
			src.pf("for (_i = 0; !zb_stream_eof(stream); _i++) {")
		case types.RepeatUntil:
			src.pf("for (_i = 0; ; _i++) {")
		}
		src.indent()
		emitMagic()
		if r, ok := a.Repeat.(types.RepeatUntil); ok {
			_ = r
			src.pf("break;")
		}
		src.unindent()
		src.pf("}")
		src.unindent()
		src.pf("}")
		return
	}

	rt := a.Type.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)
	field := e.fieldName(a.ID)
	if rt.TypeRef == nil && rt.TypeSwitch != nil {
		if a.Repeat != nil {
			e.emitRepeatTypeSwitchRead(src, parent, a, rt.TypeSwitch, field)
		} else {
			e.emitTypeSwitchRead(src, parent, a, rt.TypeSwitch, field)
		}
		return
	}
	if rt.TypeRef == nil {
		panic(fmt.Errorf("unsupported attr: %s", a.ID))
	}

	if a.Repeat != nil {
		e.emitRepeatRead(src, parent, a, rt.TypeRef, field)
		if a.Valid != nil {
			src.pf("{")
			src.indent()
			src.pf("for (size_t _vi = 0; _vi < this_->%s.len; _vi++) {", field)
			src.indent()
			elem := e.declTypeRef(rt.TypeRef, false)
			src.pf("%s _v_elem = this_->%s.data[_vi];", elem, field)
			src.pf("(void)_v_elem;")
			e.emitValidateOn(src, a, "_v_elem", rt.TypeRef)
			src.unindent()
			src.pf("}")
			src.unindent()
			src.pf("}")
		}
	} else {
		e.emitSingleRead(src, rt.TypeRef, field, a)
		if a.Valid != nil {
			e.emitValidate(src, a, field, rt.TypeRef)
		}
	}
}

func (e *Emitter) emitValidateOn(src *buf, a *kaitai.Attr, fieldExpr string, t *types.TypeRef) {
	v := a.Valid
	if v == nil {
		return
	}
	isBytes := t != nil && (t.Kind == types.Bytes || t.Kind == types.String)
	if v.Eq != "" {
		ex, err := expr.ParseExpr(v.Eq)
		if err == nil {
			rhs := e.expr(ex)
			if isBytes {
				src.pf("if (!zb_bytes_equal(%s, %s)) return ZB_ERR_VALIDATION;", fieldExpr, rhs)
			} else {
				src.pf("if (!((%s) == (%s))) return ZB_ERR_VALIDATION;", fieldExpr, rhs)
			}
		}
	}
	if v.Min != "" {
		ex, err := expr.ParseExpr(v.Min)
		if err == nil {
			rhs := e.expr(ex)
			if isBytes {
				src.pf("if (zb_bytes_compare(%s, %s) < 0) return ZB_ERR_VALIDATION;", fieldExpr, rhs)
			} else {
				src.pf("if ((%s) < (%s)) return ZB_ERR_VALIDATION;", fieldExpr, rhs)
			}
		}
	}
	if v.Max != "" {
		ex, err := expr.ParseExpr(v.Max)
		if err == nil {
			rhs := e.expr(ex)
			if isBytes {
				src.pf("if (zb_bytes_compare(%s, %s) > 0) return ZB_ERR_VALIDATION;", fieldExpr, rhs)
			} else {
				src.pf("if ((%s) > (%s)) return ZB_ERR_VALIDATION;", fieldExpr, rhs)
			}
		}
	}
	if len(v.AnyOf) > 0 {
		parts := make([]string, 0, len(v.AnyOf))
		for _, opt := range v.AnyOf {
			ex, err := expr.ParseExpr(opt)
			if err != nil {
				continue
			}
			rhs := e.expr(ex)
			if isBytes {
				parts = append(parts, fmt.Sprintf("zb_bytes_equal(%s, %s)", fieldExpr, rhs))
			} else {
				parts = append(parts, fmt.Sprintf("((%s) == (%s))", fieldExpr, rhs))
			}
		}
		if len(parts) > 0 {
			src.pf("if (!(%s)) return ZB_ERR_VALIDATION;", strings.Join(parts, " || "))
		}
	}
	if v.Expr != "" {
		ex, err := expr.ParseExpr(v.Expr)
		if err == nil {
			prev := e.mode.repeatElemIsBytes
			e.mode.repeatElemIsBytes = isBytes
			src.pf("{ %s _repeat_elem = %s; (void)_repeat_elem;", e.declTypeRef(t, true), fieldExpr)
			src.pf("if (!(%s)) return ZB_ERR_VALIDATION; }", e.expr(ex))
			e.mode.repeatElemIsBytes = prev
		}
	}
}

func (e *Emitter) emitSingleReadInto(src *buf, t *types.TypeRef, field string, a *kaitai.Attr, fieldIsVoidPtr bool) {
	if !fieldIsVoidPtr {
		e.emitSingleRead(src, t, field, a)
		return
	}
	switch t.Kind {
	case types.U1, types.S1,
		types.U2le, types.U2be, types.U4le, types.U4be, types.U8le, types.U8be,
		types.S2le, types.S2be, types.S4le, types.S4be, types.S8le, types.S8be,
		types.F4le, types.F4be, types.F8le, types.F8be:
		ctyp := e.declTypeRef(t, false)
		src.pf("%s *_tmp = (%s *)zb_arena_alloc(arena, sizeof(%s));", ctyp, ctyp, ctyp)
		src.pf("if (!_tmp) return ZB_ERR_ALLOC;")
		emitTry(src, "%s(stream, _tmp)", readCallForKind(t.Kind))
		src.pf("this_->%s = _tmp;", field)
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
			src.pf("this_->%s = _u;", field)
			src.unindent()
			src.pf("}")
			return
		}
		emitArenaNewLocal(src, "_u", typeName)
		emitTry(src, "%s_read(_u, stream, arena, %s, %s%s)", typeName, pExpr, rExpr, paramArgs)
		src.pf("this_->%s = _u;", field)
	case types.Bytes, types.String:
		emitArenaAllocLocal(src, "_b", "zb_bytes_t")
		e.emitBytesReadInto(src, t, "(*_b)")
		src.pf("this_->%s = _b;", field)
	default:
		panic(fmt.Errorf("unsupported switch case type %s", t.Kind.String()))
	}
}

func (e *Emitter) emitValidate(src *buf, a *kaitai.Attr, field string, t *types.TypeRef) {
	v := a.Valid
	if v == nil {
		return
	}
	fieldExpr := "this_->" + field
	isBytes := t != nil && (t.Kind == types.Bytes || t.Kind == types.String)
	if v.Eq != "" {
		ex, err := expr.ParseExpr(v.Eq)
		if err == nil {
			rhs := e.expr(ex)
			if isBytes {
				src.pf("if (!zb_bytes_equal(%s, %s)) return ZB_ERR_VALIDATION;", fieldExpr, rhs)
			} else {
				src.pf("if (!((%s) == (%s))) return ZB_ERR_VALIDATION;", fieldExpr, rhs)
			}
		}
	}
	if v.Min != "" {
		ex, err := expr.ParseExpr(v.Min)
		if err == nil {
			rhs := e.expr(ex)
			if isBytes {
				src.pf("if (zb_bytes_compare(%s, %s) < 0) return ZB_ERR_VALIDATION;", fieldExpr, rhs)
			} else {
				src.pf("if ((%s) < (%s)) return ZB_ERR_VALIDATION;", fieldExpr, rhs)
			}
		}
	}
	if v.Max != "" {
		ex, err := expr.ParseExpr(v.Max)
		if err == nil {
			rhs := e.expr(ex)
			if isBytes {
				src.pf("if (zb_bytes_compare(%s, %s) > 0) return ZB_ERR_VALIDATION;", fieldExpr, rhs)
			} else {
				src.pf("if ((%s) > (%s)) return ZB_ERR_VALIDATION;", fieldExpr, rhs)
			}
		}
	}
	if len(v.AnyOf) > 0 {
		parts := make([]string, 0, len(v.AnyOf))
		for _, opt := range v.AnyOf {
			ex, err := expr.ParseExpr(opt)
			if err != nil {
				continue
			}
			rhs := e.expr(ex)
			if isBytes {
				parts = append(parts, fmt.Sprintf("zb_bytes_equal(%s, %s)", fieldExpr, rhs))
			} else {
				parts = append(parts, fmt.Sprintf("((%s) == (%s))", fieldExpr, rhs))
			}
		}
		if len(parts) > 0 {
			src.pf("if (!(%s)) return ZB_ERR_VALIDATION;", strings.Join(parts, " || "))
		}
	}
	if v.Expr != "" {
		ex, err := expr.ParseExpr(v.Expr)
		if err == nil {
			prev := e.mode.repeatElemIsBytes
			e.mode.repeatElemIsBytes = isBytes
			src.pf("{ %s _repeat_elem = %s; (void)_repeat_elem;", e.declTypeRef(t, true), fieldExpr)
			src.pf("if (!(%s)) return ZB_ERR_VALIDATION; }", e.expr(ex))
			e.mode.repeatElemIsBytes = prev
		}
	}
	if v.InEnum && a.Enum != "" {
		en := e.lookupEnum(a.Enum)
		if en != nil {
			parts := make([]string, 0, len(en.Values))
			enumPrefix := ""
			if ev := e.tryResolveType(a.Enum); ev != nil && ev.Kind == engine.EnumKind {
				enumPrefix = e.prefix(ev.DefParent) + e.typeName(en.ID) + "__"
			} else {
				enumPrefix = e.typeName(en.ID) + "__"
			}
			for _, ev := range en.Values {
				parts = append(parts, fmt.Sprintf("(%s) == (%s%s)", fieldExpr, enumPrefix, e.typeName(ev.ID)))
			}
			if len(parts) > 0 {
				src.pf("if (!(%s)) return ZB_ERR_VALIDATION;", strings.Join(parts, " || "))
			}
		}
	}
}

func (e *Emitter) lookupEnum(name string) *kaitai.Enum {
	defer func() { _ = recover() }()
	ev := e.tryResolveType(name)
	if ev != nil && ev.Kind == engine.EnumKind {
		return ev.Enum
	}
	return nil
}

func (e *Emitter) emitSingleRead(src *buf, t *types.TypeRef, field string, a *kaitai.Attr) {
	switch t.Kind {
	case types.U2, types.U4, types.U8, types.S2, types.S4, types.S8, types.F4, types.F8:
		leKind, beKind := t.Kind.SplitEndian()
		emitTry(src, "_endian ? %s(stream, &this_->%s) : %s(stream, &this_->%s)",
			readCallForKind(beKind), field, readCallForKind(leKind), field)
	case types.U1, types.U2le, types.U2be, types.U4le, types.U4be, types.U8le, types.U8be,
		types.S1, types.S2le, types.S2be, types.S4le, types.S4be, types.S8le, types.S8be,
		types.F4le, types.F4be, types.F8le, types.F8be:
		emitTry(src, "%s(stream, &this_->%s)", readCallForKind(t.Kind), field)
	case types.Bits:
		if t.Bits != nil {
			endian := "be"
			if t.Bits.Endian.Kind == types.LittleBitEndian {
				endian = "le"
			}
			src.pf("{ uint64_t _bits; ZB_TRY(zb_read_bits_%s(stream, %d, &_bits)); this_->%s = %s; }",
				endian, t.Bits.Width, field, bitsAssignCast(t.Bits.Width))
		}
	case types.User:
		userT := t.User
		typ := e.tryResolveType(userT.Name)
		if typ == nil || typ.Struct == nil {
			panic(fmt.Errorf("unresolved user type %s", userT.Name))
		}
		typeName := e.prefix(typ.DefParent) + e.typeName(typ.Struct.Type.ID)
		paramArgs := e.userParamArgs(typ.Struct.Type, userT)
		pExpr, rExpr := e.userTypeParentRootArgs(typ, "this_")
		if a != nil && a.Terminator != nil && userT.Size == nil {
			term := *a.Terminator
			include := false
			if a.Include != nil {
				include = *a.Include
			}
			consume := true
			if a.Consume != nil {
				consume = *a.Consume
			}
			eosErr := false
			if a.EosError != nil {
				eosErr = *a.EosError
			}
			src.pf("{")
			src.indent()
			src.pf("zb_bytes_t _raw;")
			src.pf("ZB_TRY(zb_read_bytes_term(stream, arena, %d, %d, %d, %d, &_raw));",
				term, boolInt(include), boolInt(consume), boolInt(eosErr))
			if a.Process != nil {
				e.emitProcess(src, a.Process, "_raw")
			}
			emitSubstreamMemBytes(src, "_sub", "_raw")
			emitArenaNewInto(src, fmt.Sprintf("this_->%s", field), typeName)
			emitTry(src, "%s_read(this_->%s, _sub, arena, %s, %s%s)", typeName, field, pExpr, rExpr, paramArgs)
			src.unindent()
			src.pf("}")
			return
		}
		if a != nil && a.SizeEos {
			src.pf("{")
			src.indent()
			src.pf("zb_bytes_t _raw;")
			emitTry(src, "zb_read_bytes_full(stream, arena, &_raw)")
			emitSubstreamMemBytes(src, "_sub", "_raw")
			emitArenaNewInto(src, fmt.Sprintf("this_->%s", field), typeName)
			emitTry(src, "%s_read(this_->%s, _sub, arena, %s, %s%s)", typeName, field, pExpr, rExpr, paramArgs)
			src.unindent()
			src.pf("}")
			return
		}
		sizeExpr := userT.Size
		if sizeExpr == nil && a != nil && a.Size != nil {
			sizeExpr = a.Size
		}
		sizeOverride := ""
		if userT.Size == nil && a != nil && a.Size != nil && e.mode.sizeOverride != "" {
			sizeOverride = e.mode.sizeOverride
		}
		if sizeExpr != nil {
			padR, termR, includeR := -1, -1, false
			if a != nil {
				if a.PadRight != nil {
					padR = *a.PadRight
				}
				if a.Terminator != nil {
					termR = *a.Terminator
				}
				if a.Include != nil {
					includeR = *a.Include
				}
			}
			src.pf("{")
			src.indent()
			if sizeOverride != "" {
				src.pf("size_t _sub_n = %s;", sizeOverride)
			} else {
				src.pf("size_t _sub_n = (size_t)(%s);", e.expr(sizeExpr))
			}
			needRawCopy := (a != nil && a.Process != nil) || termR >= 0 || padR >= 0
			saveRaw := e.needsRawShadowField(a)
			if needRawCopy {
				src.pf("zb_bytes_t _raw;")
				if termR >= 0 || padR >= 0 {
					if saveRaw {
						emitTry(src, "zb_read_bytes(stream, arena, _sub_n, &this_->_raw_%s)", field)
						src.pf("_raw = this_->_raw_%s;", field)
						e.emitStripBytes(src, "_raw", termR, padR, includeR)
					} else {
						src.pf("ZB_TRY(zb_read_bytes_pad_term(stream, arena, _sub_n, %d, %d, %d, &_raw));",
							termR, padR, boolInt(includeR))
					}
				} else {
					emitTry(src, "zb_read_bytes(stream, arena, _sub_n, &_raw)")
				}
				if a != nil && a.Process != nil {
					e.emitProcess(src, a.Process, "_raw")
				}
				emitSubstreamMemBytes(src, "_sub", "_raw")
			} else if saveRaw {
				src.pf("zb_bytes_t _raw;")
				emitTry(src, "zb_read_bytes(stream, arena, _sub_n, &_raw)")
				src.pf("this_->_raw_%s = _raw;", field)
				emitSubstreamMemBytes(src, "_sub", "_raw")
			} else {
				emitSubstreamConsume(src, "_sub", "stream", "_sub_n")
			}
			emitArenaNewInto(src, fmt.Sprintf("this_->%s", field), typeName)
			emitTry(src, "%s_read(this_->%s, _sub, arena, %s, %s%s)", typeName, field, pExpr, rExpr, paramArgs)
			src.unindent()
			src.pf("}")
			return
		}
		emitArenaNewInto(src, fmt.Sprintf("this_->%s", field), typeName)
		parentArg := e.parentArgFor(a)
		pArg, rArg := e.userTypeParentRootArgs(typ, parentArg)
		emitTry(src, "%s_read(this_->%s, stream, arena, %s, %s%s)", typeName, field, pArg, rArg, paramArgs)
	case types.Bytes, types.String:
		e.emitBytesReadWithAttr(src, t, field, a)
		if a != nil && a.Process != nil {
			e.emitProcess(src, a.Process, fmt.Sprintf("this_->%s", field))
		}
	default:
		panic(fmt.Errorf("unsupported type %s for %s", t.Kind.String(), field))
	}
}

func (e *Emitter) emitRepeatRead(src *buf, _ *engine.ExprValue, a *kaitai.Attr, t *types.TypeRef, field string) {
	elemType := e.declTypeRef(t, false)
	if elemType == "" {
		panic(fmt.Errorf("repeat: unknown element type for %s", a.ID))
	}
	src.pf("{")
	src.indent()
	src.p("size_t _i;")
	src.p("(void)_i;")

	ksID := string(a.ID)
	switch r := a.Repeat.(type) {
	case types.RepeatExpr:
		countExpr := e.expr(r.CountExpr)
		src.pf("size_t _count = (size_t)(%s);", countExpr)
		src.pf("if (_count > 0) {")
		src.indent()
		src.pf("if (zb_array_grow_impl(arena, (void **)&this_->%s.data, &this_->%s.cap, _count, sizeof(*this_->%s.data)) != ZB_OK) return ZB_ERR_ALLOC;",
			field, field, field)
		src.unindent()
		src.pf("}")
		src.pf("for (_i = 0; _i < _count; _i++) {")
		src.indent()
		e.debugArrElemStart(src, ksID)
		e.emitRepeatBodyRead(src, t, field, a)
		e.debugArrElemEnd(src, ksID)
		src.unindent()
		src.pf("}")
	case types.RepeatEOS:
		src.pf("for (_i = 0; !zb_stream_eof(stream); _i++) {")
		src.indent()
		src.pf("if (zb_array_grow_impl(arena, (void **)&this_->%s.data, &this_->%s.cap, this_->%s.len+1, sizeof(*this_->%s.data)) != ZB_OK) return ZB_ERR_ALLOC;",
			field, field, field, field)
		e.debugArrElemStart(src, ksID)
		e.emitRepeatBodyRead(src, t, field, a)
		e.debugArrElemEnd(src, ksID)
		src.unindent()
		src.pf("}")
	case types.RepeatUntil:
		untilExpr := r.UntilExpr
		src.pf("_i = 0;")
		src.pf("for (;;) {")
		src.indent()
		src.pf("if (zb_array_grow_impl(arena, (void **)&this_->%s.data, &this_->%s.cap, this_->%s.len+1, sizeof(*this_->%s.data)) != ZB_OK) return ZB_ERR_ALLOC;",
			field, field, field, field)
		e.debugArrElemStart(src, ksID)
		e.emitRepeatBodyRead(src, t, field, a)
		e.debugArrElemEnd(src, ksID)
		elem := e.declTypeRef(t, false)
		src.pf("%s _repeat_elem = this_->%s.data[this_->%s.len-1];", elem, field, field)
		src.pf("(void)_repeat_elem;")
		src.pf("_i++;")
		prev := e.mode.repeatElemIsBytes
		e.mode.repeatElemIsBytes = (t.Kind == types.Bytes || t.Kind == types.String)
		oldCtx := e.context
		tmpSym := engine.NewValueOfType(e.context, *t)
		if tmpSym != nil {
			e.context = e.context.WithTemporary(tmpSym)
		}
		src.pf("if (%s) break;", e.repeatUntilExpr(untilExpr, field))
		e.context = oldCtx
		e.mode.repeatElemIsBytes = prev
		src.unindent()
		src.pf("}")
	default:
		panic(fmt.Errorf("unknown repeat kind for %s", a.ID))
	}

	src.unindent()
	src.pf("}")
}

func (e *Emitter) emitRepeatBodyRead(src *buf, t *types.TypeRef, field string, a *kaitai.Attr) {
	switch t.Kind {
	case types.User:
		userT := t.User
		typ := e.tryResolveType(userT.Name)
		if typ == nil || typ.Struct == nil {
			panic(fmt.Errorf("unresolved user type %s", userT.Name))
		}
		typeName := e.prefix(typ.DefParent) + e.typeName(typ.Struct.Type.ID)
		paramArgs := e.userParamArgs(typ.Struct.Type, userT)
		pExpr, rExpr := e.userTypeParentRootArgs(typ, "this_")
		emitArenaNewLocal(src, "_tmp", typeName)
		switch {
		case a != nil && a.Terminator != nil:
			term := *a.Terminator
			include := false
			if a.Include != nil {
				include = *a.Include
			}
			consume := true
			if a.Consume != nil {
				consume = *a.Consume
			}
			eosErr := false
			if a.EosError != nil {
				eosErr = *a.EosError
			}
			src.pf("{")
			src.indent()
			src.pf("zb_bytes_t _raw;")
			src.pf("ZB_TRY(zb_read_bytes_term(stream, arena, %d, %d, %d, %d, &_raw));",
				term, boolInt(include), boolInt(consume), boolInt(eosErr))
			emitSubstreamMemBytes(src, "_sub", "_raw")
			emitTry(src, "%s_read(_tmp, _sub, arena, %s, %s%s)", typeName, pExpr, rExpr, paramArgs)
			src.unindent()
			src.pf("}")
		case userT.Size != nil:
			src.pf("{")
			src.indent()
			src.pf("size_t _sub_n = (size_t)(%s);", e.expr(userT.Size))
			saveRaw := e.needsRawShadowField(a)
			if a != nil && a.Process != nil {
				if saveRaw {
					src.pf("zb_bytes_t _raw_e; ZB_TRY(zb_read_bytes(stream, arena, _sub_n, &_raw_e));")
					src.pf("if (zb_array_grow_impl(arena, (void **)&this_->_raw_%s.data, &this_->_raw_%s.cap, this_->_raw_%s.len+1, sizeof(*this_->_raw_%s.data)) != ZB_OK) return ZB_ERR_ALLOC;",
						field, field, field, field)
					src.pf("this_->_raw_%s.data[this_->_raw_%s.len++] = _raw_e;", field, field)
					src.pf("zb_bytes_t _raw = _raw_e;")
				} else {
					src.pf("zb_bytes_t _raw; ZB_TRY(zb_read_bytes(stream, arena, _sub_n, &_raw));")
				}
				e.emitProcess(src, a.Process, "_raw")
				emitSubstreamMemBytes(src, "_sub", "_raw")
			} else if saveRaw {
				src.pf("zb_bytes_t _raw_e; ZB_TRY(zb_read_bytes(stream, arena, _sub_n, &_raw_e));")
				src.pf("if (zb_array_grow_impl(arena, (void **)&this_->_raw_%s.data, &this_->_raw_%s.cap, this_->_raw_%s.len+1, sizeof(*this_->_raw_%s.data)) != ZB_OK) return ZB_ERR_ALLOC;",
					field, field, field, field)
				src.pf("this_->_raw_%s.data[this_->_raw_%s.len++] = _raw_e;", field, field)
				emitSubstreamMemBytes(src, "_sub", "_raw_e")
			} else {
				emitSubstreamConsume(src, "_sub", "stream", "_sub_n")
			}
			emitTry(src, "%s_read(_tmp, _sub, arena, %s, %s%s)", typeName, pExpr, rExpr, paramArgs)
			src.unindent()
			src.pf("}")
		default:
			src.pf("_err = %s_read(_tmp, stream, arena, %s, %s%s);", typeName, pExpr, rExpr, paramArgs)
			src.pf("if (_err) { this_->%s.data[this_->%s.len++] = _tmp; return _err; }", field, field)
		}
		src.pf("this_->%s.data[this_->%s.len++] = _tmp;", field, field)
	case types.Bytes, types.String:
		src.pf("zb_bytes_t _tmp;")
		if e.needsRawBytesShadowField(a) {
			rawField := "_raw_" + field
			src.pf("zb_bytes_t _raw_e;")
			sizeExpr := ""
			if t.Kind == types.Bytes && t.Bytes != nil && t.Bytes.Size != nil {
				sizeExpr = e.expr(t.Bytes.Size)
			} else if t.Kind == types.String && t.String != nil && t.String.Size != nil {
				sizeExpr = e.expr(t.String.Size)
			} else if a != nil && a.Size != nil {
				sizeExpr = e.expr(a.Size)
			}
			emitTry(src, "zb_read_bytes(stream, arena, (size_t)(%s), &_raw_e)", sizeExpr)
			src.pf("if (zb_array_grow_impl(arena, (void **)&this_->%s.data, &this_->%s.cap, this_->%s.len+1, sizeof(*this_->%s.data)) != ZB_OK) return ZB_ERR_ALLOC;",
				rawField, rawField, rawField, rawField)
			src.pf("this_->%s.data[this_->%s.len++] = _raw_e;", rawField, rawField)
			src.pf("_tmp = _raw_e;")
			term := -1
			pad := -1
			include := false
			if t.Kind == types.Bytes && t.Bytes != nil {
				term = t.Bytes.Terminator
				pad = t.Bytes.PadRight
				include = t.Bytes.Include
			} else if t.Kind == types.String && t.String != nil {
				term = t.String.Terminator
				pad = t.String.PadRight
				include = t.String.Include
			}
			e.emitStripBytes(src, "_tmp", term, pad, include)
		} else {
			e.emitBytesReadIntoWithAttr(src, t, "_tmp", a)
		}
		if a != nil && a.Process != nil {
			e.emitProcess(src, a.Process, "_tmp")
		}
		src.pf("this_->%s.data[this_->%s.len++] = _tmp;", field, field)
	case types.Bits:
		if t.Bits != nil {
			endian := "be"
			if t.Bits.Endian.Kind == types.LittleBitEndian {
				endian = "le"
			}
			src.pf("uint64_t _bits; ZB_TRY(zb_read_bits_%s(stream, %d, &_bits));", endian, t.Bits.Width)
			src.pf("this_->%s.data[this_->%s.len++] = %s;", field, field, bitsAssignCast(t.Bits.Width))
		}
	default:
		if call := readCallForKind(t.Kind); call != "" {
			src.pf("ZB_TRY(%s(stream, &this_->%s.data[this_->%s.len]));",
				call, field, field)
			src.pf("this_->%s.len++;", field)
		} else {
			panic(fmt.Errorf("unsupported repeat element type %s", t.Kind.String()))
		}
	}
}

func (e *Emitter) repeatUntilExpr(ex *expr.Expr, _ string) string {
	if ex == nil {
		return "0"
	}
	return e.expr(ex)
}

func (e *Emitter) maybeAppendDecode(src *buf, encoding, lhs string) {
	if !e.encodingNeedsConversion(encoding) {
		return
	}
	src.pf("{ zb_bytes_t _dec; ZB_TRY(zb_bytes_decode(arena, %s, %q, &_dec)); %s = _dec; }",
		lhs, encoding, lhs)
}

func (e *Emitter) encodingNeedsConversion(enc string) bool {
	norm := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(enc, "-", ""), "_", ""))
	switch norm {
	case "", "UTF8", "ASCII":
		return false
	}
	return true
}

func multiByteUnit(enc string) int {
	norm := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(enc, "-", ""), "_", ""))
	switch norm {
	case "UTF16", "UTF16LE", "UTF16BE":
		return 2
	case "UTF32", "UTF32LE", "UTF32BE":
		return 4
	}
	return 1
}

func (e *Emitter) emitBytesReadInto(src *buf, t *types.TypeRef, lhs string) {
	e.emitBytesReadIntoWithAttr(src, t, lhs, nil)
}

func (e *Emitter) emitBytesReadIntoWithAttr(src *buf, t *types.TypeRef, lhs string, a *kaitai.Attr) {
	if a != nil && a.Size != nil {
		if t.Kind == types.Bytes && t.Bytes != nil && t.Bytes.Size == nil {
			t.Bytes.Size = a.Size
			defer func() { t.Bytes.Size = nil }()
		} else if t.Kind == types.String && t.String != nil && t.String.Size == nil {
			t.String.Size = a.Size
			defer func() { t.String.Size = nil }()
		}
	}
	e.emitBytesReadIntoRaw(src, t, lhs)
}

func (e *Emitter) emitBytesReadIntoRaw(src *buf, t *types.TypeRef, lhs string) {
	switch t.Kind {
	case types.Bytes:
		if t.Bytes != nil {
			if t.Bytes.Size != nil {
				term := t.Bytes.Terminator
				pad := t.Bytes.PadRight
				if term >= 0 || pad >= 0 {
					src.pf("ZB_TRY(zb_read_bytes_pad_term(stream, arena, (size_t)(%s), %d, %d, %d, &%s));",
						e.expr(t.Bytes.Size), term, pad, boolInt(t.Bytes.Include), lhs)
					return
				}
				emitTry(src, "zb_read_bytes(stream, arena, (size_t)(%s), &%s)", e.expr(t.Bytes.Size), lhs)
				return
			}
			if t.Bytes.SizeEOS {
				emitTry(src, "zb_read_bytes_full(stream, arena, &%s)", lhs)
				return
			}
			if t.Bytes.Terminator >= 0 {
				src.pf("ZB_TRY(zb_read_bytes_term(stream, arena, %d, %d, %d, %d, &%s));",
					t.Bytes.Terminator, boolInt(t.Bytes.Include), boolInt(t.Bytes.Consume), boolInt(t.Bytes.EosError), lhs)
				return
			}
		}
	case types.String:
		if t.String != nil {
			if t.String.Size != nil {
				term := t.String.Terminator
				pad := t.String.PadRight
				if term >= 0 || pad >= 0 {
					src.pf("ZB_TRY(zb_read_bytes_pad_term(stream, arena, (size_t)(%s), %d, %d, %d, &%s));",
						e.expr(t.String.Size), term, pad, boolInt(t.String.Include), lhs)
					return
				}
				emitTry(src, "zb_read_bytes(stream, arena, (size_t)(%s), &%s)", e.expr(t.String.Size), lhs)
				return
			}
			if t.String.SizeEOS {
				emitTry(src, "zb_read_bytes_full(stream, arena, &%s)", lhs)
				return
			}
			if t.String.Terminator >= 0 {
				src.pf("ZB_TRY(zb_read_bytes_term(stream, arena, %d, %d, %d, %d, &%s));",
					t.String.Terminator, boolInt(t.String.Include), boolInt(t.String.Consume), boolInt(t.String.EosError), lhs)
				return
			}
		}
	}
	panic(fmt.Errorf("unsupported bytes/string into %s", lhs))
}

func (e *Emitter) arrayElemCTypeOfNode(n expr.Node) (out string) {
	defer func() { _ = recover() }()
	result := engine.ResultTypeOfNode(e.context, n)
	if result == nil {
		return ""
	}
	if vt, ok := result.ValueType(); ok {
		if vt.Type.TypeRef != nil {
			if vt.Type.TypeRef.IsArray || vt.Repeat != nil {
				inner := *vt.Type.TypeRef
				inner.IsArray = false
				if c := e.declTypeRef(&inner, false); c != "" {
					return c
				}
			}
		}
	}
	if result.Kind == engine.ArrayKind && result.Array != nil && result.Array.Elem != nil {
		return e.declType(result.Array.Elem)
	}
	switch result.Kind {
	case engine.AttrKind:
		if result.Attr != nil && result.Attr.Repeat != nil && result.Attr.Type.TypeRef != nil {
			ref := *result.Attr.Type.TypeRef
			ref.IsArray = false
			return e.declTypeRef(&ref, false)
		}
	case engine.InstanceKind:
		if result.Instance != nil && result.Instance.Repeat != nil && result.Instance.Type.TypeRef != nil {
			ref := *result.Instance.Type.TypeRef
			ref.IsArray = false
			return e.declTypeRef(&ref, false)
		}
		if result.Instance != nil && result.Instance.Value != nil {
			if arr, ok := result.Instance.Value.Root.(expr.ArrayNode); ok && len(arr.Items) > 0 {
				return e.arrayLiteralElemCType(arr)
			}
		}
	}
	return ""
}

func (e *Emitter) arrayLiteralElemCType(arr expr.ArrayNode) string {
	allString := true
	allByte := true
	anyFloat := false
	for _, item := range arr.Items {
		if _, ok := item.(expr.StringNode); !ok {
			allString = false
		}
		if n, ok := item.(expr.IntNode); ok {
			if n.Integer.Sign() < 0 || !n.Integer.IsInt64() || n.Integer.Int64() > 255 {
				allByte = false
			}
		} else {
			allByte = false
		}
		if _, ok := item.(expr.FloatNode); ok {
			anyFloat = true
		}
	}
	if allString {
		return "zb_bytes_t"
	}
	if allByte {
		return "uint8_t"
	}
	if anyFloat {
		return "double"
	}
	return "int64_t"
}

func (e *Emitter) declType(v *engine.ExprValue) string {
	if v == nil {
		return ""
	}
	if vt, ok := v.ValueType(); ok && vt.Type.TypeRef != nil {
		return e.declTypeRef(vt.Type.TypeRef, false)
	}
	switch v.Kind {
	case engine.IntegerKind:
		return "int64_t"
	case engine.FloatKind:
		return "double"
	case engine.BooleanKind:
		return "int"
	case engine.StringKind, engine.ByteArrayKind:
		return "zb_bytes_t"
	case engine.StructKind:
		if v.Struct != nil && v.Struct.Type != nil {
			return e.prefix(v.DefParent) + e.typeName(v.Struct.Type.ID) + "_t *"
		}
	}
	return ""
}

func (e *Emitter) exprIsByteKind(n expr.Node) (out bool) {
	defer func() { _ = recover() }()
	r := engine.ResultTypeOfNode(e.context, n)
	if r == nil {
		return false
	}
	if vt, ok := r.ValueType(); ok && vt.Type.TypeRef != nil {
		k := vt.Type.TypeRef.Kind
		return k == types.Bytes || k == types.String
	}
	switch r.Kind {
	case engine.ByteArrayKind, engine.StringKind:
		return true
	}
	return false
}

func (e *Emitter) parentArgFor(a *kaitai.Attr) string {
	if a == nil || a.Parent == nil {
		return "this_"
	}
	if a.Parent.Disabled {
		return "NULL"
	}
	if a.Parent.Expr != "" {
		if ex, err := expr.ParseExpr(a.Parent.Expr); err == nil {
			return e.exprNode(ex.Root)
		}
	}
	return "this_"
}

func (e *Emitter) userParamArgs(ks *kaitai.Struct, userT *types.UserType) string {
	if userT == nil || len(ks.Params) == 0 {
		return ""
	}
	out := strings.Builder{}
	for i, p := range ks.Params {
		out.WriteString(", ")
		if i < len(userT.Params) {
			cast := e.declTypeRefForParam(&p.Type)
			argExpr := e.expr(userT.Params[i])
			if isScalarType(cast) {
				out.WriteString("(" + cast + ")(")
				out.WriteString(argExpr)
				out.WriteString(")")
			} else {
				out.WriteString(argExpr)
			}
		} else {
			out.WriteString("0")
		}
	}
	return out.String()
}

func (e *Emitter) emitBytesReadWithAttr(src *buf, t *types.TypeRef, field string, a *kaitai.Attr) {
	lhs := "this_->" + field
	if a != nil && a.Size != nil {
		if t.Kind == types.Bytes && t.Bytes != nil && t.Bytes.Size == nil {
			t.Bytes.Size = a.Size
			defer func() { t.Bytes.Size = nil }()
		} else if t.Kind == types.String && t.String != nil && t.String.Size == nil {
			t.String.Size = a.Size
			defer func() { t.String.Size = nil }()
		}
	}
	if t.Kind == types.Bytes && t.Bytes != nil {
		if t.Bytes.Size != nil {
			term := t.Bytes.Terminator
			pad := t.Bytes.PadRight
			if term >= 0 || pad >= 0 {
				if e.needsRawBytesShadowField(a) {
					src.pf("ZB_TRY(zb_read_bytes(stream, arena, (size_t)(%s), &this_->_raw_%s));",
						e.expr(t.Bytes.Size), field)
					src.pf("%s = this_->_raw_%s;", lhs, field)
					e.emitStripBytes(src, lhs, term, pad, t.Bytes.Include)
					return
				}
				emitTry(src, "zb_read_bytes_pad_term(stream, arena, (size_t)(%s), %d, %d, %d, &%s)",
					e.expr(t.Bytes.Size), term, pad, boolInt(t.Bytes.Include), lhs)
				return
			}
			emitTry(src, "zb_read_bytes(stream, arena, (size_t)(%s), &%s)", e.expr(t.Bytes.Size), lhs)
			return
		}
		if t.Bytes.SizeEOS {
			term := t.Bytes.Terminator
			pad := t.Bytes.PadRight
			if term >= 0 || pad >= 0 {
				emitTry(src, "zb_read_bytes_full(stream, arena, &%s)", lhs)
				e.emitStripBytes(src, lhs, term, pad, t.Bytes.Include)
				return
			}
			emitTry(src, "zb_read_bytes_full(stream, arena, &%s)", lhs)
			return
		}
		if t.Bytes.Terminator >= 0 {
			emitTry(src, "zb_read_bytes_term(stream, arena, %d, %d, %d, %d, &%s)",
				t.Bytes.Terminator, boolInt(t.Bytes.Include), boolInt(t.Bytes.Consume), boolInt(t.Bytes.EosError), lhs)
			return
		}
		emitTry(src, "zb_read_bytes_full(stream, arena, &%s)", lhs)
		return
	}
	if t.Kind == types.String && t.String != nil {
		defer e.maybeAppendDecode(src, t.String.Encoding, lhs)
		if t.String.Terminator >= 0 && multiByteUnit(t.String.Encoding) > 1 {
			unit := multiByteUnit(t.String.Encoding)
			b := make([]byte, unit)
			b[0] = byte(t.String.Terminator)
			termBytes := byteArrayInitializer(b)
			if t.String.Size != nil {
				if e.needsRawBytesShadowField(a) {
					src.pf("ZB_TRY(zb_read_bytes(stream, arena, (size_t)(%s), &this_->_raw_%s));",
						e.expr(t.String.Size), field)
					src.pf("%s = this_->_raw_%s;", lhs, field)
				} else {
					emitTry(src, "zb_read_bytes(stream, arena, (size_t)(%s), &%s)", e.expr(t.String.Size), lhs)
				}
				if t.String.PadRight >= 0 {
					src.pf("%s = zb_bytes_strip_right(%s, %d);", lhs, lhs, t.String.PadRight)
				}
				src.pf("{ static const uint8_t _term[] = {%s};", termBytes)
				src.pf("  %s = zb_bytes_terminate_multi(%s, _term, %d, %d); }", lhs, lhs, unit, boolInt(t.String.Include))
				return
			}
			src.pf("{ static const uint8_t _term[] = {%s};", termBytes)
			src.pf("  ZB_TRY(zb_read_bytes_term_multi(stream, arena, _term, %d, %d, %d, %d, &%s)); }",
				unit, boolInt(t.String.Include), boolInt(t.String.Consume), boolInt(t.String.EosError), lhs)
			return
		}
		if t.String.Size != nil {
			term := t.String.Terminator
			pad := t.String.PadRight
			if term >= 0 || pad >= 0 {
				if e.needsRawBytesShadowField(a) {
					src.pf("ZB_TRY(zb_read_bytes(stream, arena, (size_t)(%s), &this_->_raw_%s));",
						e.expr(t.String.Size), field)
					src.pf("%s = this_->_raw_%s;", lhs, field)
					e.emitStripBytes(src, lhs, term, pad, t.String.Include)
					return
				}
				emitTry(src, "zb_read_bytes_pad_term(stream, arena, (size_t)(%s), %d, %d, %d, &%s)",
					e.expr(t.String.Size), term, pad, boolInt(t.String.Include), lhs)
				return
			}
			emitTry(src, "zb_read_bytes(stream, arena, (size_t)(%s), &%s)", e.expr(t.String.Size), lhs)
			return
		}
		if t.String.Terminator >= 0 {
			emitTry(src, "zb_read_bytes_term(stream, arena, %d, %d, %d, %d, &%s)",
				t.String.Terminator, boolInt(t.String.Include), boolInt(t.String.Consume), boolInt(t.String.EosError), lhs)
			return
		}
		if t.String.SizeEOS {
			term := t.String.Terminator
			pad := t.String.PadRight
			if term >= 0 || pad >= 0 {
				emitTry(src, "zb_read_bytes_full(stream, arena, &%s)", lhs)
				e.emitStripBytes(src, lhs, term, pad, t.String.Include)
				return
			}
			emitTry(src, "zb_read_bytes_full(stream, arena, &%s)", lhs)
			return
		}
		panic(fmt.Errorf("unsupported: string without size or terminator for %s", field))
	}
	panic(fmt.Errorf("unsupported bytes/string shape for %s", field))
}

func (e *Emitter) emitStripBytes(src *buf, lhs string, term, pad int, include bool) {
	if term >= 0 {
		src.pf("{")
		src.indent()
		src.pf("ptrdiff_t _i = zb_bytes_index(%s, %d);", lhs, term)
		src.pf("if (_i >= 0) {")
		src.indent()
		src.pf("%s.len = (size_t)_i%s;", lhs, conditionalInclude(include))
		src.unindent()
		src.pf("}")
		if pad >= 0 {
			src.pf("else {")
			src.indent()
			src.pf("%s = zb_bytes_strip_right(%s, %d);", lhs, lhs, pad)
			src.unindent()
			src.pf("}")
		}
		src.unindent()
		src.pf("}")
		return
	}
	if pad >= 0 {
		src.pf("%s = zb_bytes_strip_right(%s, %d);", lhs, lhs, pad)
	}
}

func (e *Emitter) typeName(id kaitai.Identifier) string {
	return ksToCName(string(id))
}

func (e *Emitter) fieldName(id kaitai.Identifier) string {
	return ksToCName(string(id))
}

func (e *Emitter) filename(id kaitai.Identifier) string {
	return strings.ToLower(string(id))
}

func (e *Emitter) prefix(typ *engine.ExprValue) string {
	if typ == nil || typ.Struct == nil {
		return ""
	}
	return e.prefix(typ.DefParent) + e.typeName(typ.Struct.Type.ID) + "_"
}

func (e *Emitter) appendSeqAttrFields(gs *cStruct, a *kaitai.Attr) {
	typ := e.declFieldType(a)
	if typ == "" {
		gs.fields = append(gs.fields, cField{
			typ:  "void *",
			name: e.fieldName(a.ID),
		})
		return
	}
	gs.fields = append(gs.fields, cField{
		typ:  typ,
		name: e.fieldName(a.ID),
	})
	if a.If != nil {
		gs.fields = append(gs.fields, cField{
			typ:  "int",
			name: "_have_" + e.fieldName(a.ID),
		})
	}
	if e.needsRawShadowField(a) || e.needsRawBytesShadowField(a) || e.needsSwitchRawShadowField(a) {
		shadowType := "zb_bytes_t"
		if a.Repeat != nil {
			shadowType = e.arrayTagFor("zb_bytes_t")
		}
		gs.fields = append(gs.fields, cField{
			typ:  shadowType,
			name: "_raw_" + e.fieldName(a.ID),
		})
	}
}

func (e *Emitter) appendInstanceFields(gs *cStruct, a *kaitai.Attr) {
	typ := e.declFieldType(a)
	if typ == "" {
		typ = "void *"
	}
	gs.fields = append(gs.fields, cField{
		typ:  typ,
		name: e.fieldName(a.ID),
	})
	gs.fields = append(gs.fields, cField{
		typ:  "int",
		name: "_f_" + e.fieldName(a.ID),
	})
	if a.Value != nil && a.If != nil {
		gs.fields = append(gs.fields, cField{
			typ:  "int",
			name: "_n_" + e.fieldName(a.ID),
		})
	}
	if e.needsRawShadowField(a) || e.needsRawBytesShadowField(a) {
		gs.fields = append(gs.fields, cField{
			typ:  "zb_bytes_t",
			name: "_raw_" + e.fieldName(a.ID),
		})
	}
}

func (e *Emitter) needsSwitchRawShadowField(a *kaitai.Attr) bool {
	if a == nil {
		return false
	}
	if a.Type.TypeSwitch == nil {
		return false
	}
	if uniformSwitchCase(a.Type.TypeSwitch) != nil {
		return false
	}
	if integerWidenedSwitchCase(a.Type.TypeSwitch) != nil {
		return false
	}
	return a.Size != nil || a.SizeEos
}

func (e *Emitter) needsRawShadowField(a *kaitai.Attr) bool {
	if a == nil {
		return false
	}
	if a.Type.TypeRef == nil || a.Type.TypeRef.Kind != types.User {
		return false
	}
	if a.Type.TypeRef.User == nil {
		return false
	}
	return a.Type.TypeRef.User.Size != nil || a.Size != nil
}

func (e *Emitter) needsRawBytesShadowField(a *kaitai.Attr) bool {
	if a == nil {
		return false
	}
	if a.Type.TypeRef == nil {
		return false
	}
	t := a.Type.TypeRef
	switch t.Kind {
	case types.Bytes:
		if t.Bytes == nil {
			return false
		}
		hasSize := t.Bytes.Size != nil || a.Size != nil
		if !hasSize {
			return false
		}
		return t.Bytes.Terminator >= 0 || t.Bytes.PadRight >= 0
	case types.String:
		if t.String == nil {
			return false
		}
		hasSize := t.String.Size != nil || a.Size != nil
		if !hasSize {
			return false
		}
		return t.String.Terminator >= 0 || t.String.PadRight >= 0
	}
	return false
}
