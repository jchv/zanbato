package golang

import (
	"bytes"
	"fmt"
	"path"
	"strings"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/emitter"
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/resolve"
	"github.com/jchv/zanbato/kaitai/types"
)

const (
	// TODO: Switch back to the official Kaitai Go runtime after we resolve some issues:
	// - Missing Writer functionality (https://github.com/kaitai-io/kaitai_struct_go_runtime/pull/32)
	// - ReadBytesTerm EOF bug (https://github.com/kaitai-io/kaitai_struct_go_runtime/pull/31)
	// - ReadBytesPadTerm misbehaves when pad == term (PR TBD)
	// - ReadBytesPadTerm incorrectly interprets pad as UTF-8 (PR TBD)
	kaitaiRuntimePackagePath = "github.com/jchw-forks/kaitai_struct_go_runtime/kaitai"
	kaitaiRuntimePackageName = "kaitai"
	kaitaiStream             = kaitaiRuntimePackageName + ".Stream"
	kaitaiWriter             = kaitaiRuntimePackageName + ".Writer"
)

// fileScope holds state that is fresh for each Go source file (i.e. each
// top-level root() call). It is pushed/popped via pushFileScope so that
// import recursion does not bleed state between files.
type fileScope struct {
	unit         *goUnit
	rootTypeName string
	rootDepth    int
	opaqueTypes  bool
	parents      engine.ParentTypes

	// Deferred-import accumulators: set true the first time the
	// corresponding package is referenced, then flushed at end of root().
	needStrconv bool
	needStrings bool
	needFmt     bool
	needBytes   bool
}

// exprMode holds transient state that needs to be saved/restored across
// function-emit boundaries (emitReadFunc / emitWriteFunc /
// writeUserTypeSingle). It is swapped in/out via saveExprMode.
type exprMode struct {
	needRoot    bool
	needParent  bool
	inWriteExpr bool
}

// Emitter emits Go code for kaitai structs.
type Emitter struct {
	pkgname     string
	pkgpath     string
	resolver    resolve.Resolver
	endian      types.EndianKind
	bitEndian   types.BitEndianKind
	context     *engine.Context
	artifacts   []emitter.Artifact
	visited     map[*kaitai.Struct]struct{}
	debugAlways bool
	compat      kaitai.Compatibility
	debug       bool

	mode exprMode

	// file holds the currently-active per-file state. nil outside any
	// pushFileScope-bounded region.
	file *fileScope
}

// saveExprMode snapshots the current exprMode and returns a closure that
// restores it on call. Pair with `defer` at the top of a function that
// needs to mutate the mode locally.
func (e *Emitter) saveExprMode() func() {
	prev := e.mode
	return func() { e.mode = prev }
}

// pushFileScope sets up a fresh fileScope for a new output unit and returns
// a closure that restores the previous scope on call.
func (e *Emitter) pushFileScope(structID kaitai.Identifier) func() {
	prev := e.file
	newDepth := 0
	if prev != nil {
		newDepth = prev.rootDepth + 1
	}
	rootTypeName := ""
	// Only type Root_ for the top-level root (not imported files).
	// Imported files may be used from different root contexts at runtime,
	// so their Root_ must stay 'any' to accept different root types.
	if newDepth == 0 {
		rootTypeName = e.typeName(structID)
	}
	e.file = &fileScope{
		unit:         &goUnit{pkgname: e.pkgname, imports: map[string]string{}},
		rootTypeName: rootTypeName,
		rootDepth:    newDepth,
	}
	return func() { e.file = prev }
}

// pushMetaScope applies endian/bitEndian/debug overrides from a struct's meta
// block and returns a closure that restores the previous values on call.
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

// enterModuleLocal pivots the engine context to a new module/local root and
// returns a closure that restores the previous context on call.
func (e *Emitter) enterModuleLocal(root *engine.ExprValue) func() {
	prev := e.context
	e.context = e.context.WithModuleRoot(root).WithLocalRoot(root)
	return func() { e.context = prev }
}

// NewEmitter constructs a new emitter with the given parameters.
func NewEmitter(pkgpath string, resolver resolve.Resolver) *Emitter {
	return &Emitter{
		pkgname:  path.Base(pkgpath),
		pkgpath:  pkgpath,
		resolver: resolver,
		context:  engine.NewContext(),
		visited:  make(map[*kaitai.Struct]struct{}),
		compat:   engine.DefaultCompat,
	}
}

// SetDebug controls whether debug features are unconditionally enabled for
// all generated code.
func (e *Emitter) SetDebug(enabled bool) {
	e.debugAlways = enabled
}

// SetCompat sets the compatibility mode for generated code.
func (e *Emitter) SetCompat(c kaitai.Compatibility) {
	e.compat = c
}

// Emit emits Go code for the given kaitai struct.
func (e *Emitter) Emit(inputname string, s *kaitai.Struct) []emitter.Artifact {
	e.endian = types.UnspecifiedOrder
	e.bitEndian = types.UnspecifiedBitOrder
	e.context = engine.NewContext()
	e.context.Compat = e.compat
	e.artifacts = nil
	e.visited = make(map[*kaitai.Struct]struct{})
	e.mode = exprMode{}
	e.debug = e.debugAlways
	e.file = nil

	e.root(inputname, s)
	return e.artifacts
}

func (e *Emitter) filename(n kaitai.Identifier) string {
	return strings.ToLower(string(n)) + ".go"
}

func (e *Emitter) typeName(n kaitai.Identifier) string {
	return ksToGoName(string(n))
}

func (e *Emitter) typeSwitchName(n kaitai.Identifier) string {
	return e.typeName(n) + "_Cases"
}

func (e *Emitter) fieldName(n kaitai.Identifier) string {
	return e.typeName(n)
}

func (e *Emitter) setImport(unit *goUnit, pkg string, as string) {
	unit.imports[pkg] = as
}

func (e *Emitter) mustResolveType(ex string) *engine.ExprValue {
	// Handle Kaitai built-in parameter types
	switch ex {
	case "bool":
		return &engine.ExprValue{Kind: engine.BooleanKind}
	case "io":
		return &engine.ExprValue{Kind: engine.StreamKind}
	case "struct":
		// Generic struct type - used for params that accept any struct
		return &engine.ExprValue{Kind: engine.StructKind}
	}
	typ := engine.ResolveTypeOfExpr(e.context, expr.MustParseExpr(ex))
	if typ == nil {
		if e.file.opaqueTypes {
			opaque := engine.NewOpaqueStructSymbol(ex)
			e.context.AddGlobalType(ex, opaque)
			return opaque
		}
		panic(fmt.Errorf("unresolved type: %s", ex))
	}
	return typ
}

func (e *Emitter) isOpaqueType(val *engine.ExprValue) bool {
	return val.Kind == engine.StructKind && val.Struct != nil && val.Struct.Opaque
}

func (e *Emitter) declTypeRef(n *types.TypeRef, r types.RepeatType) string {
	if r != nil {
		return "[]" + e.declTypeRef(n, nil)
	}
	if n.IsArray {
		// Array type (e.g., u2[], str[]) - produce []elementType
		inner := *n
		inner.IsArray = false
		return "[]" + e.declTypeRef(&inner, nil)
	}
	switch n.Kind {
	case types.UntypedInt:
		return "int"
	case types.UntypedFloat:
		return "float64"
	case types.UntypedBool:
		return "bool"
	case types.U1:
		return "uint8"
	case types.U2, types.U2le, types.U2be:
		return "uint16"
	case types.U4, types.U4le, types.U4be:
		return "uint32"
	case types.U8, types.U8le, types.U8be:
		return "uint64"
	case types.S1:
		return "int8"
	case types.S2, types.S2le, types.S2be:
		return "int16"
	case types.S4, types.S4le, types.S4be:
		return "int32"
	case types.S8, types.S8le, types.S8be:
		return "int64"
	case types.Bits:
		if n.Bits != nil && n.Bits.Width == 1 {
			return "bool"
		} else {
			return "uint64"
		}
	case types.F4, types.F4le, types.F4be:
		return "float32"
	case types.F8, types.F8le, types.F8be:
		return "float64"
	case types.Bytes:
		return "[]byte"
	case types.String:
		return "string"
	case types.User:
		// Handle built-in type names that don't resolve through the context
		switch n.User.Name {
		case "bool":
			return "bool"
		case "io":
			return "*" + kaitaiStream
		case "struct":
			return "any"
		}
		typ := e.mustResolveType(n.User.Name)
		switch typ.Kind {
		case engine.StructKind:
			return "*" + e.prefix(typ.DefParent) + e.typeName(typ.Struct.Type.ID)
		case engine.EnumKind:
			return e.prefix(typ.DefParent) + e.typeName(typ.Enum.ID)
		case engine.BooleanKind:
			return "bool"
		case engine.StreamKind:
			return "*" + kaitaiStream
		default:
			panic(fmt.Errorf("expression %q yielded unexpected type %s", n.User.Name, typ.Kind))
		}
	}
	panic("unexpected typekind: " + n.Kind.String())
}

func (e *Emitter) declTypeSwitch(r types.RepeatType) string {
	if r != nil {
		return "[]any"
	}
	return "any"
}

func (e *Emitter) declType(typ *engine.ExprValue) string {
	if typ == nil {
		return ""
	}
	switch typ.Kind {
	case engine.StructKind:
		if typ.Struct == nil {
			return "any" // generic struct parameter
		}
		return e.prefix(typ.DefParent) + e.typeName(typ.Struct.Type.ID)
	case engine.EnumKind:
		return e.prefix(typ.DefParent) + e.typeName(typ.Enum.ID)
	case engine.BooleanKind:
		return "bool"
	case engine.StreamKind:
		return "*" + kaitaiStream
	case engine.ArrayKind:
		if typ.Array != nil && typ.Array.Elem != nil {
			elemType := e.declType(typ.Array.Elem)
			if elemType != "" {
				return "[]" + elemType
			}
		}
		return "[]any"
	default:
		vt, ok := typ.ValueType()
		if !ok {
			return ""
		}
		if vt.Type.TypeRef != nil {
			return e.declTypeRef(vt.Type.TypeRef, vt.Repeat)
		} else if vt.Type.TypeSwitch != nil {
			return e.declTypeSwitch(vt.Repeat)
		} else {
			panic(fmt.Errorf("invalid type for %s", typ.Kind))
		}
	}
}

func (e *Emitter) newTypeRef(n *types.TypeRef) string {
	t := e.declTypeRef(n, nil)
	if strings.HasPrefix(t, "*") {
		return "&" + t[1:]
	}
	return t
}

func (e *Emitter) readCallRef(n *types.TypeRef) string {
	return e.readCallRefOn("stream", n)
}

func (e *Emitter) readCallRefOn(sv string, n *types.TypeRef) string {
	switch n.Kind {
	case types.UntypedInt:
		panic("untyped number")
	case types.U2, types.U4, types.U8,
		types.S2, types.S4, types.S8,
		types.F4, types.F8:
		panic("undecided endianness")
	case types.U1:
		return sv + ".ReadU1()"
	case types.U2le:
		return sv + ".ReadU2le()"
	case types.U2be:
		return sv + ".ReadU2be()"
	case types.U4le:
		return sv + ".ReadU4le()"
	case types.U4be:
		return sv + ".ReadU4be()"
	case types.U8le:
		return sv + ".ReadU8le()"
	case types.U8be:
		return sv + ".ReadU8be()"
	case types.S1:
		return sv + ".ReadS1()"
	case types.S2le:
		return sv + ".ReadS2le()"
	case types.S2be:
		return sv + ".ReadS2be()"
	case types.S4le:
		return sv + ".ReadS4le()"
	case types.S4be:
		return sv + ".ReadS4be()"
	case types.S8le:
		return sv + ".ReadS8le()"
	case types.S8be:
		return sv + ".ReadS8be()"
	case types.Bits:
		endianSuffix := "Be"
		if n.Bits.Endian.Kind == types.LittleBitEndian {
			endianSuffix = "Le"
		}
		return fmt.Sprintf("%s.ReadBitsInt%s(%d)", sv, endianSuffix, n.Bits.Width)
	case types.F4le:
		return sv + ".ReadF4le()"
	case types.F4be:
		return sv + ".ReadF4be()"
	case types.F8le:
		return sv + ".ReadF8le()"
	case types.F8be:
		return sv + ".ReadF8be()"
	case types.Bytes:
		if n.Bytes.Size != nil {
			terminator := n.Bytes.Terminator
			padRight := n.Bytes.PadRight
			if terminator >= 0 || padRight >= 0 {
				if padRight >= 0 && terminator < 0 {
					// Only pad-right, no terminator: read fixed bytes then strip padding
					return fmt.Sprintf("func() ([]byte, error) { bs, err := %s.ReadBytes(int(%s)); if err != nil { return nil, err }; return kaitai.BytesStripRight(bs, %d), nil }()", sv, e.expr(n.Bytes.Size), padRight)
				}
				padByte := padRight
				if padByte < 0 {
					padByte = terminator
				}
				return fmt.Sprintf("%s.ReadBytesPadTerm(int(%s), %d, %d, %v)", sv, e.expr(n.Bytes.Size), terminator, padByte, n.Bytes.Include)
			}
			return fmt.Sprintf("%s.ReadBytes(int(%s))", sv, e.expr(n.Bytes.Size))
		}
		if n.Bytes.SizeEOS {
			terminator := n.Bytes.Terminator
			padRight := n.Bytes.PadRight
			if padRight >= 0 && terminator < 0 {
				// Only pad-right, no terminator: read all bytes then strip padding
				return fmt.Sprintf("func() ([]byte, error) { bs, err := %s.ReadBytesFull(); if err != nil { return nil, err }; return kaitai.BytesStripRight(bs, %d), nil }()", sv, padRight)
			}
			if terminator >= 0 {
				e.file.needBytes = true
				padByte := padRight
				if padByte < 0 {
					padByte = terminator
				}
				return fmt.Sprintf("func() ([]byte, error) { bs, err := %s.ReadBytesFull(); if err != nil { return nil, err }; if i := bytes.IndexByte(bs, %d); i != -1 { if %v { bs = bs[:i+1] } else { bs = bs[:i] } } else { bs = kaitai.BytesStripRight(bs, %d) }; return bs, nil }()", sv, terminator, n.Bytes.Include, padByte)
			}
			return sv + ".ReadBytesFull()"
		}
		if n.Bytes.Terminator >= 0 {
			return fmt.Sprintf("%s.ReadBytesTerm(%d, %v, %v, %v)", sv, n.Bytes.Terminator, n.Bytes.Include, n.Bytes.Consume, n.Bytes.EosError)
		}
		// No size, no terminator - read all remaining bytes
		return sv + ".ReadBytesFull()"
	case types.String:
		if n.String.SizeEOS {
			terminator := n.String.Terminator
			padRight := n.String.PadRight
			if padRight >= 0 && terminator == -1 {
				// Only pad-right, no terminator
				return fmt.Sprintf("func() ([]byte, error) { bs, err := %s.ReadBytesFull(); if err != nil { return nil, err }; return kaitai.BytesStripRight(bs, %d), nil }()", sv, padRight)
			}
			if terminator != -1 {
				e.file.needBytes = true
				padByte := padRight
				if padByte < 0 {
					padByte = terminator
				}
				return fmt.Sprintf("func() ([]byte, error) { bs, err := %s.ReadBytesFull(); if err != nil { return nil, err }; if i := bytes.IndexByte(bs, %d); i != -1 { if %v { bs = bs[:i+1] } else { bs = bs[:i] } } else { bs = kaitai.BytesStripRight(bs, %d) }; return bs, nil }()", sv, terminator, n.String.Include, padByte)
			}
			return sv + ".ReadBytesFull()"
		}
		multiByte := isMultiByteEncoding(n.String.Encoding)
		if n.String.Size != nil {
			terminator := n.String.Terminator
			padRight := n.String.PadRight
			if terminator != -1 || padRight >= 0 {
				termByte := terminator
				if termByte == -1 {
					termByte = 0
				}
				padByte := padRight
				if padByte < 0 {
					padByte = termByte
				}
				if multiByte {
					// Multi-byte encoding (UTF-16): read fixed size, then strip
					// multi-byte terminator and padding
					stripPad := ""
					if padRight >= 0 {
						stripPad = fmt.Sprintf("; _result = kaitai.BytesStripRight(_result, %d)", padByte)
					}
					return fmt.Sprintf("(func() ([]byte, error) { _raw, err := %s.ReadBytes(int(%s)); if err != nil { return nil, err }; _result := kaitai.BytesTerminateMulti(_raw, []byte{%d, %d}, %v)%s; return _result, nil }())",
						sv, e.expr(n.String.Size), termByte, termByte, n.String.Include, stripPad)
				}
				return fmt.Sprintf("%s.ReadBytesPadTerm(int(%s), %d, %d, %v)", sv, e.expr(n.String.Size), termByte, padByte, n.String.Include)
			}
			return fmt.Sprintf("%s.ReadBytes(int(%s))", sv, e.expr(n.String.Size))
		} else {
			if n.String.Terminator == -1 {
				// No size and no terminator - read all remaining bytes (EOS)
				return sv + ".ReadBytesFull()"
			}
			if multiByte {
				// Multi-byte encoding (UTF-16): use ReadBytesTermMulti with 2-byte null
				return fmt.Sprintf("%s.ReadBytesTermMulti([]byte{%d, %d}, %v, %v, %v)",
					sv, n.String.Terminator, n.String.Terminator, n.String.Include, n.String.Consume, n.String.EosError)
			}
			if !n.String.EosError {
				seekBack := ""
				if !n.String.Consume {
					seekBack = fmt.Sprintf("; %s.Seek(-1, 1)", sv)
				}
				return fmt.Sprintf("func() ([]byte, error) { var bs []byte; for { b, err := %s.ReadU1(); if err != nil { break }; if b == %d { if %v { bs = append(bs, b) }%s; break }; bs = append(bs, b) }; return bs, nil }()", sv, n.String.Terminator, n.String.Include, seekBack)
			}
			return fmt.Sprintf("%s.ReadBytesTerm(%d, %v, %v, %v)", sv, n.String.Terminator, n.String.Include, n.String.Consume, n.String.EosError)
		}
	case types.User:
		panic("called readCallRef on user type!")
	}
	panic("unexpected typekind: " + n.Kind.String())
}

func (e *Emitter) writeCallRef(n *types.TypeRef, valExpr string) string {
	return e.writeCallRefOn("wstream", n, valExpr)
}

func (e *Emitter) writeCallRefOn(sv string, n *types.TypeRef, valExpr string) string {
	switch n.Kind {
	case types.UntypedInt:
		panic("untyped number")
	case types.U2, types.U4, types.U8,
		types.S2, types.S4, types.S8,
		types.F4, types.F8:
		panic("undecided endianness")
	case types.U1:
		return fmt.Sprintf("%s.WriteU1(%s)", sv, valExpr)
	case types.U2le:
		return fmt.Sprintf("%s.WriteU2le(%s)", sv, valExpr)
	case types.U2be:
		return fmt.Sprintf("%s.WriteU2be(%s)", sv, valExpr)
	case types.U4le:
		return fmt.Sprintf("%s.WriteU4le(%s)", sv, valExpr)
	case types.U4be:
		return fmt.Sprintf("%s.WriteU4be(%s)", sv, valExpr)
	case types.U8le:
		return fmt.Sprintf("%s.WriteU8le(%s)", sv, valExpr)
	case types.U8be:
		return fmt.Sprintf("%s.WriteU8be(%s)", sv, valExpr)
	case types.S1:
		return fmt.Sprintf("%s.WriteS1(%s)", sv, valExpr)
	case types.S2le:
		return fmt.Sprintf("%s.WriteS2le(%s)", sv, valExpr)
	case types.S2be:
		return fmt.Sprintf("%s.WriteS2be(%s)", sv, valExpr)
	case types.S4le:
		return fmt.Sprintf("%s.WriteS4le(%s)", sv, valExpr)
	case types.S4be:
		return fmt.Sprintf("%s.WriteS4be(%s)", sv, valExpr)
	case types.S8le:
		return fmt.Sprintf("%s.WriteS8le(%s)", sv, valExpr)
	case types.S8be:
		return fmt.Sprintf("%s.WriteS8be(%s)", sv, valExpr)
	case types.Bits:
		endianSuffix := "Be"
		if n.Bits.Endian.Kind == types.LittleBitEndian {
			endianSuffix = "Le"
		}
		return fmt.Sprintf("%s.WriteBitsInt%s(%d, %s)", sv, endianSuffix, n.Bits.Width, valExpr)
	case types.F4le:
		return fmt.Sprintf("%s.WriteF4le(%s)", sv, valExpr)
	case types.F4be:
		return fmt.Sprintf("%s.WriteF4be(%s)", sv, valExpr)
	case types.F8le:
		return fmt.Sprintf("%s.WriteF8le(%s)", sv, valExpr)
	case types.F8be:
		return fmt.Sprintf("%s.WriteF8be(%s)", sv, valExpr)
	case types.Bytes:
		if n.Bytes != nil && n.Bytes.Size != nil {
			terminator := n.Bytes.Terminator
			padRight := n.Bytes.PadRight
			if terminator >= 0 || padRight >= 0 {
				// If include=true, terminator is already in the data - don't write it again
				writeTerm := terminator
				if n.Bytes.Include {
					writeTerm = -1
				}
				return fmt.Sprintf("%s.WriteBytesLimit(%s, int(%s), %d, %d)", sv, valExpr, e.expr(n.Bytes.Size), writeTerm, padRight)
			}
		}
		if n.Bytes != nil && n.Bytes.Terminator >= 0 && !n.Bytes.Include && n.Bytes.Consume {
			return fmt.Sprintf("func() error { if err := %s.WriteBytes(%s); err != nil { return err }; return %s.WriteU1(%d) }()", sv, valExpr, sv, n.Bytes.Terminator)
		}
		return fmt.Sprintf("%s.WriteBytes(%s)", sv, valExpr)
	case types.String:
		if n.String != nil && n.String.Size != nil {
			// Fixed-size string: write with padding
			terminator := n.String.Terminator
			padRight := n.String.PadRight
			if terminator >= 0 || padRight >= 0 {
				writeTerm := terminator
				if n.String.Include {
					writeTerm = -1
				}
				return fmt.Sprintf("%s.WriteBytesLimit([]byte(%s), int(%s), %d, %d)", sv, valExpr, e.expr(n.String.Size), writeTerm, padRight)
			}
			return fmt.Sprintf("%s.WriteBytes([]byte(%s))", sv, valExpr)
		}
		if n.String != nil && n.String.Terminator >= 0 && !n.String.Include && n.String.Consume {
			// Terminated string, consume=true: write string + terminator byte(s)
			if isMultiByteEncoding(n.String.Encoding) {
				// Multi-byte encoding (UTF-16): write 2-byte null terminator
				return fmt.Sprintf("func() error { if err := %s.WriteBytes([]byte(%s)); err != nil { return err }; return %s.WriteBytes([]byte{%d, %d}) }()", sv, valExpr, sv, n.String.Terminator, n.String.Terminator)
			}
			return fmt.Sprintf("func() error { if err := %s.WriteBytes([]byte(%s)); err != nil { return err }; return %s.WriteU1(%d) }()", sv, valExpr, sv, n.String.Terminator)
		}
		return fmt.Sprintf("%s.WriteBytes([]byte(%s))", sv, valExpr)
	case types.User:
		panic("called writeCallRefOn on user type!")
	}
	panic("unexpected typekind: " + n.Kind.String())
}

// emitRepeatRawTailRead emits read code for a repeated field that needs per-element raw tail storage.
func (e *Emitter) emitRepeatRawTailRead(fn *goFunc, unit *goUnit, a *kaitai.Attr, rt types.Type, fieldName, cast, assignSuffix string) {
	termByte := -1
	padRight := -1
	include := false
	isEOS := false
	sizeExpr := ""
	if rt.TypeRef.Kind == types.Bytes && rt.TypeRef.Bytes != nil {
		termByte = rt.TypeRef.Bytes.Terminator
		padRight = rt.TypeRef.Bytes.PadRight
		include = rt.TypeRef.Bytes.Include
		isEOS = rt.TypeRef.Bytes.SizeEOS
		if rt.TypeRef.Bytes.Size != nil {
			sizeExpr = e.expr(rt.TypeRef.Bytes.Size)
		}
	} else if rt.TypeRef.Kind == types.String && rt.TypeRef.String != nil {
		termByte = rt.TypeRef.String.Terminator
		padRight = rt.TypeRef.String.PadRight
		include = rt.TypeRef.String.Include
		isEOS = rt.TypeRef.String.SizeEOS
		if rt.TypeRef.String.Size != nil {
			sizeExpr = e.expr(rt.TypeRef.String.Size)
		}
	}

	if isEOS {
		fn.pf("_raw_%d, err := stream.ReadBytesFull()", fn.tmp)
	} else {
		fn.pf("_raw_%d, err := stream.ReadBytes(int(%s))", fn.tmp, sizeExpr)
	}
	fn.pf("if err != nil { return err }")
	fn.pf("var tmp%d []byte", fn.tmp)
	if termByte >= 0 {
		e.file.needBytes = true
		fn.pf("if _i_%d := bytes.IndexByte(_raw_%d, %d); _i_%d != -1 {", fn.tmp, fn.tmp, termByte, fn.tmp)
		fn.indent()
		if include {
			fn.pf("tmp%d = _raw_%d[:_i_%d+1]", fn.tmp, fn.tmp, fn.tmp)
		} else {
			fn.pf("tmp%d = _raw_%d[:_i_%d]", fn.tmp, fn.tmp, fn.tmp)
		}
		fn.pf("this._raw_tail_%s = append(this._raw_tail_%s, _raw_%d[_i_%d+1:])", string(a.ID), string(a.ID), fn.tmp, fn.tmp)
		fn.unindent()
		fn.pf("} else {")
		fn.indent()
		if padRight >= 0 {
			fn.pf("tmp%d = kaitai.BytesStripRight(_raw_%d, %d)", fn.tmp, fn.tmp, padRight)
			fn.pf("this._raw_tail_%s = append(this._raw_tail_%s, _raw_%d[len(tmp%d):])", string(a.ID), string(a.ID), fn.tmp, fn.tmp)
		} else {
			fn.pf("tmp%d = _raw_%d", fn.tmp, fn.tmp)
			fn.pf("this._raw_tail_%s = append(this._raw_tail_%s, nil)", string(a.ID), string(a.ID))
		}
		fn.unindent()
		fn.pf("}")
	} else if padRight >= 0 {
		fn.pf("tmp%d = kaitai.BytesStripRight(_raw_%d, %d)", fn.tmp, fn.tmp, padRight)
		fn.pf("this._raw_tail_%s = append(this._raw_tail_%s, _raw_%d[len(tmp%d):])", string(a.ID), string(a.ID), fn.tmp, fn.tmp)
	} else {
		fn.pf("tmp%d = _raw_%d", fn.tmp, fn.tmp)
		fn.pf("this._raw_tail_%s = append(this._raw_tail_%s, nil)", string(a.ID), string(a.ID))
	}
	if a.Process != nil {
		// Save full pre-strip bytes for process+pad/term roundtrip
		fn.pf("this._raw_%s = append(this._raw_%s, append([]byte(nil), _raw_%d...))", string(a.ID), string(a.ID), fn.tmp)
		e.emitProcess(fn, unit, a.Process, fmt.Sprintf("tmp%d", fn.tmp))
	}
	fn.pf("this.%s = append(this.%s, %s(tmp%d)%s)", fieldName, fieldName, cast, fn.tmp, assignSuffix)
}

// fieldNeedsRawTail returns true if a field needs a _raw_tail_ storage for roundtrip.
// This is the case for any sized/eos bytes/string where data is stripped (terminator or pad-right).
// For byte-identical roundtrip, we must preserve the exact bytes that were stripped.
func (e *Emitter) fieldNeedsRawTail(rt types.Type) bool {
	if rt.TypeRef == nil {
		return false
	}
	switch rt.TypeRef.Kind {
	case types.Bytes:
		if rt.TypeRef.Bytes == nil {
			return false
		}
		// Any sized or EOS field with stripping
		hasSizeOrEOS := rt.TypeRef.Bytes.Size != nil || rt.TypeRef.Bytes.SizeEOS
		hasStripping := rt.TypeRef.Bytes.Terminator >= 0 || rt.TypeRef.Bytes.PadRight >= 0
		return hasSizeOrEOS && hasStripping
	case types.String:
		if rt.TypeRef.String == nil {
			return false
		}
		hasSizeOrEOS := rt.TypeRef.String.Size != nil || rt.TypeRef.String.SizeEOS
		hasStripping := rt.TypeRef.String.Terminator >= 0 || rt.TypeRef.String.PadRight >= 0
		return hasSizeOrEOS && hasStripping
	}
	return false
}

// parentGoType returns the Go type string for a struct's Parent_ field.
// Returns "" if the parent should be 'any'.
func (e *Emitter) parentGoType(ks *kaitai.Struct) string {
	parentType, ok := e.file.parents.Inferred[ks]
	if !ok || parentType == nil {
		return ""
	}
	return "*" + e.prefix(parentType.Parent) + e.typeName(parentType.Struct.Type.ID)
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

	defer e.pushFileScope(s.ID)()
	e.file.parents = engine.BuildParentTypeMap(e.context, rootType)
	e.file.opaqueTypes = s.Meta.OpaqueTypes

	e.struc(inputname, e.file.unit, root)

	// Add deferred imports
	if e.file.needStrconv {
		e.setImport(e.file.unit, "strconv", "strconv")
	}
	if e.file.needStrings {
		e.setImport(e.file.unit, "strings", "strings")
	}
	if e.file.needFmt {
		e.setImport(e.file.unit, "fmt", "fmt")
	}
	if e.file.needBytes {
		e.setImport(e.file.unit, "bytes", "bytes")
	}

	out := bytes.Buffer{}
	e.file.unit.emit(&out)

	e.artifacts = append(e.artifacts, emitter.Artifact{
		Filename: e.filename(s.ID),
		Body:     out.Bytes(),
	})
}

func (e *Emitter) enterLocal(val *engine.ExprValue) func() {
	prev := e.context
	e.context = e.context.WithLocalRoot(val)
	return func() { e.context = prev }
}

func (e *Emitter) enumTypeName(parent *engine.ExprValue, enum *kaitai.Enum) string {
	return e.prefix(parent) + e.typeName(enum.ID)
}

func (e *Emitter) enumValueName(parent *engine.ExprValue, enum *kaitai.Enum, id kaitai.Identifier) string {
	return e.prefix(parent) + e.typeName(enum.ID) + "__" + e.typeName(id)
}

func (e *Emitter) enum(unit *goUnit, enum *engine.ExprValue) {
	g := goEnum{name: e.enumTypeName(enum.Parent, enum.Enum), decltype: "int"}
	for _, v := range enum.Enum.Values {
		g.values = append(g.values, goEnumValue{name: e.enumValueName(enum.Parent, enum.Enum, v.ID), value: int(v.Value.Int64())})
	}
	unit.enums = append(unit.enums, g)
}

// emitUserTypeRead generates code to read a user type, optionally with a substream.
// parentExpr is the expression to pass as __parent (normally "this", but "nil" for opaque/external types).
// rootExpr is the expression to pass as __root (normally "this.Root_", but "nil" for opaque/external types).
// When debugMode is true, the inner Read() call uses "err = ..." without error checking,
// allowing the caller to assign the result before checking for errors.
func (e *Emitter) emitUserTypeRead(fn *goFunc, unit *goUnit, rt types.TypeRef, endianSuffix string, a *kaitai.Attr, parentExpr string, rootExpr string, debugMode bool) {
	// Determine terminator/pad-right attributes from the attr
	terminator := -1
	padRight := -1
	include := false
	consume := true
	eosError := false
	if a.Terminator != nil {
		terminator = *a.Terminator
	}
	if a.PadRight != nil {
		padRight = *a.PadRight
	}
	if a.Include != nil {
		include = *a.Include
	}
	if a.Consume != nil {
		consume = *a.Consume
	}
	if a.EosError != nil {
		eosError = *a.EosError
	}
	hasTerm := terminator >= 0
	hasPad := padRight >= 0

	if rt.User != nil && rt.User.Size != nil {
		// Create substream from explicit size bytes - use ReadBytesPadTerm when needed
		e.setImport(unit, "bytes", "bytes")
		// Check if we need raw tail storage for user type fields
		needsUserRawTail := hasTerm || hasPad
		if hasTerm || hasPad {
			if hasPad && !hasTerm {
				// Only pad-right, no terminator: read fixed bytes, capture raw tail
				e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
				fn.pf("{")
				fn.indent()
				fn.pf("_full, err := stream.ReadBytes(int(%s))", e.expr(rt.User.Size))
				fn.pf("if err != nil { return err }")
				fn.pf("_stripped := kaitai.BytesStripRight(_full, %d)", padRight)
				fn.pf("this._raw_tail_%s = _full[len(_stripped):]", string(a.ID))
				// Store full raw bytes for substream roundtrip
				if a.Repeat != nil {
					fn.pf("this._raw_%s = append(this._raw_%s, _full)", string(a.ID), string(a.ID))
				} else {
					fn.pf("this._raw_%s = _full", string(a.ID))
				}
				if a.Process != nil {
					e.emitProcess(fn, unit, a.Process, "_stripped")
				}
				fn.pf("_sub := kaitai.NewStream(bytes.NewReader(_stripped))")
				fn.pf("if err := tmp%d.Read%s(_sub, %s, %s); err != nil { return err }", fn.tmp, endianSuffix, parentExpr, rootExpr)
				fn.unindent()
				fn.pf("}")
				return // early return - complete read handled inline
			} else if needsUserRawTail {
				// Terminator without pad-right: capture raw tail for roundtrip
				e.file.needBytes = true
				fn.pf("{")
				fn.indent()
				fn.pf("_full, err := stream.ReadBytes(int(%s))", e.expr(rt.User.Size))
				fn.pf("if err != nil { return err }")
				fn.pf("var _stripped []byte")
				if include {
					fn.pf("if _i := bytes.IndexByte(_full, %d); _i != -1 { _stripped = _full[:_i+1]; this._raw_tail_%s = _full[_i+1:] } else { _stripped = _full; this._raw_tail_%s = nil }", terminator, string(a.ID), string(a.ID))
				} else {
					fn.pf("if _i := bytes.IndexByte(_full, %d); _i != -1 { _stripped = _full[:_i]; this._raw_tail_%s = _full[_i+1:] } else { _stripped = _full; this._raw_tail_%s = nil }", terminator, string(a.ID), string(a.ID))
				}
				if a.Process != nil {
					// Store full pre-strip bytes for process+pad/term roundtrip
					if a.Repeat != nil {
						fn.pf("this._raw_%s = append(this._raw_%s, append([]byte(nil), _full...))", string(a.ID), string(a.ID))
					} else {
						fn.pf("this._raw_%s = append([]byte(nil), _full...)", string(a.ID))
					}
					e.emitProcess(fn, unit, a.Process, "_stripped")
				}
				fn.pf("_sub_tmp%d := kaitai.NewStream(bytes.NewReader(_stripped))", fn.tmp)
				fn.pf("if err := tmp%d.Read%s(_sub_tmp%d, %s, %s); err != nil { return err }", fn.tmp, endianSuffix, fn.tmp, parentExpr, rootExpr)
				fn.unindent()
				fn.pf("}")
				return // early return - we handled the complete read inline
			} else {
				// Both term and pad: read full, strip, capture raw tail
				e.file.needBytes = true
				fn.pf("{")
				fn.indent()
				fn.pf("_full, err := stream.ReadBytes(int(%s))", e.expr(rt.User.Size))
				fn.pf("if err != nil { return err }")
				fn.pf("var _stripped []byte")
				if include {
					fn.pf("if _i := bytes.IndexByte(_full, %d); _i != -1 { _stripped = _full[:_i+1]; this._raw_tail_%s = _full[_i+1:] } else { _stripped = kaitai.BytesStripRight(_full, %d); this._raw_tail_%s = _full[len(_stripped):] }", terminator, string(a.ID), padRight, string(a.ID))
				} else {
					fn.pf("if _i := bytes.IndexByte(_full, %d); _i != -1 { _stripped = _full[:_i]; this._raw_tail_%s = _full[_i+1:] } else { _stripped = kaitai.BytesStripRight(_full, %d); this._raw_tail_%s = _full[len(_stripped):] }", terminator, string(a.ID), padRight, string(a.ID))
				}
				// Store full raw for roundtrip
				if a.Repeat != nil {
					fn.pf("this._raw_%s = append(this._raw_%s, append([]byte(nil), _full...))", string(a.ID), string(a.ID))
				} else {
					fn.pf("this._raw_%s = append([]byte(nil), _full...)", string(a.ID))
				}
				if a.Process != nil {
					e.emitProcess(fn, unit, a.Process, "_stripped")
				}
				fn.pf("_sub := kaitai.NewStream(bytes.NewReader(_stripped))")
				fn.pf("if err := tmp%d.Read%s(_sub, %s, %s); err != nil { return err }", fn.tmp, endianSuffix, parentExpr, rootExpr)
				fn.unindent()
				fn.pf("}")
				return // early return
			}
		} else {
			fn.pf("_raw_tmp%d, err := stream.ReadBytes(int(%s))", fn.tmp, e.expr(rt.User.Size))
		}
		fn.pf("if err != nil {").indent()
		fn.pf("return err")
		fn.unindent().pf("}")
		// Store raw substream bytes for roundtrip
		if a.Repeat != nil {
			fn.pf("this._raw_%s = append(this._raw_%s, _raw_tmp%d)", string(a.ID), string(a.ID), fn.tmp)
		} else {
			fn.pf("this._raw_%s = _raw_tmp%d", string(a.ID), fn.tmp)
		}
		if a.Process != nil {
			e.emitProcess(fn, unit, a.Process, fmt.Sprintf("_raw_tmp%d", fn.tmp))
		}
		fn.pf("_sub_tmp%d := kaitai.NewStream(bytes.NewReader(_raw_tmp%d))", fn.tmp, fn.tmp)
		if debugMode {
			fn.pf("err = tmp%d.Read%s(_sub_tmp%d, %s, %s)", fn.tmp, endianSuffix, fn.tmp, parentExpr, rootExpr)
		} else {
			fn.pf("if err := tmp%d.Read%s(_sub_tmp%d, %s, %s); err != nil {", fn.tmp, endianSuffix, fn.tmp, parentExpr, rootExpr).indent()
			fn.pf("return err")
			fn.unindent().pf("}")
		}
	} else if a.SizeEos {
		// Create substream from all remaining bytes (size-eos: true)
		e.setImport(unit, "bytes", "bytes")
		fn.pf("_raw_tmp%d, err := stream.ReadBytesFull()", fn.tmp)
		fn.pf("if err != nil {").indent()
		fn.pf("return err")
		fn.unindent().pf("}")
		fn.pf("_sub_tmp%d := kaitai.NewStream(bytes.NewReader(_raw_tmp%d))", fn.tmp, fn.tmp)
		if debugMode {
			fn.pf("err = tmp%d.Read%s(_sub_tmp%d, %s, %s)", fn.tmp, endianSuffix, fn.tmp, parentExpr, rootExpr)
		} else {
			fn.pf("if err := tmp%d.Read%s(_sub_tmp%d, %s, %s); err != nil {", fn.tmp, endianSuffix, fn.tmp, parentExpr, rootExpr).indent()
			fn.pf("return err")
			fn.unindent().pf("}")
		}
	} else if hasTerm {
		// No explicit size, but have a terminator - read bytes until terminator,
		// then parse from a substream
		e.setImport(unit, "bytes", "bytes")
		if !eosError {
			// Work around upstream runtime bug in ReadBytesTerm with eos-error: false
			seekBack := ""
			if !consume {
				seekBack = "; stream.Seek(-1, 1)"
			}
			fn.pf("_raw_tmp%d, err := func() ([]byte, error) { var bs []byte; for { b, err := stream.ReadU1(); if err != nil { break }; if b == %d { if %v { bs = append(bs, b) }%s; break }; bs = append(bs, b) }; return bs, nil }()", fn.tmp, terminator, include, seekBack)
		} else {
			fn.pf("_raw_tmp%d, err := stream.ReadBytesTerm(%d, %v, %v, %v)", fn.tmp, terminator, include, consume, eosError)
		}
		fn.pf("if err != nil {").indent()
		fn.pf("return err")
		fn.unindent().pf("}")
		if a.Process != nil {
			// Store pre-process bytes for roundtrip
			if a.Repeat != nil {
				fn.pf("this._raw_%s = append(this._raw_%s, append([]byte(nil), _raw_tmp%d...))", string(a.ID), string(a.ID), fn.tmp)
			} else {
				fn.pf("this._raw_%s = append([]byte(nil), _raw_tmp%d...)", string(a.ID), fn.tmp)
			}
			e.emitProcess(fn, unit, a.Process, fmt.Sprintf("_raw_tmp%d", fn.tmp))
		}
		fn.pf("_sub_tmp%d := kaitai.NewStream(bytes.NewReader(_raw_tmp%d))", fn.tmp, fn.tmp)
		if debugMode {
			fn.pf("err = tmp%d.Read%s(_sub_tmp%d, %s, %s)", fn.tmp, endianSuffix, fn.tmp, parentExpr, rootExpr)
		} else {
			fn.pf("if err := tmp%d.Read%s(_sub_tmp%d, %s, %s); err != nil {", fn.tmp, endianSuffix, fn.tmp, parentExpr, rootExpr).indent()
			fn.pf("return err")
			fn.unindent().pf("}")
		}
	} else {
		if debugMode {
			fn.pf("err = tmp%d.Read%s(stream, %s, %s)", fn.tmp, endianSuffix, parentExpr, rootExpr)
		} else {
			fn.pf("if err := tmp%d.Read%s(stream, %s, %s); err != nil {", fn.tmp, endianSuffix, parentExpr, rootExpr).indent()
			fn.pf("return err")
			fn.unindent().pf("}")
		}
	}
}

func (e *Emitter) setParams(struc string, rt types.TypeRef, resolved *engine.ExprValue, fn *goFunc) {
	for i := range rt.User.Params {
		param := resolved.Struct.Type.Params[i]
		field := e.typeName(param.ID)
		paramType := e.declTypeRef(&param.Type, nil)
		fn.pf("%s.%s = (%s)(%s)", struc, field, paramType, e.expr(rt.User.Params[i]))
	}
}

func (e *Emitter) emitAttrRead(unit *goUnit, fn *goFunc, typ *engine.ExprValue, forcedEndian types.EndianKind) bool {
	a := typ.Attr

	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Errorf("attr: %s: %s", a.ID, r))
		}
	}()

	endianSuffix := ""
	switch forcedEndian {
	case types.LittleEndian:
		endianSuffix = "LE"
	case types.BigEndian:
		endianSuffix = "BE"
	}

	rt := a.Type.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)

	if rt.HasDependentEndian() {
		fn.pf("return kaitai.UndecidedEndiannessError{}")
		return false
	}

	fn.tmp++

	fieldName := e.fieldName(a.ID)

	if a.If != nil {
		if exprReferencesIO(a.If) {
			// Cache IO-dependent condition for Write roundtrip
			fn.pf("this._if_%s = %s", string(a.ID), e.expr(a.If))
			fn.pf("if this._if_%s {", string(a.ID)).indent()
		} else {
			fn.pf("if %s {", e.expr(a.If)).indent()
		}
	}

	if rt.TypeSwitch != nil {
		// Call type-switch helper, handling repeat and size
		switchName := e.prefix(typ.Parent) + e.typeSwitchName(rt.TypeSwitch.FieldName)
		needsIndex := exprContainsIndex(rt.TypeSwitch.SwitchOn)
		switchCall := func(streamVar string) {
			if needsIndex {
				fn.pf("if err := this.read%s%s(%s, i); err != nil {", switchName, endianSuffix, streamVar).indent()
			} else {
				fn.pf("if err := this.read%s%s(%s); err != nil {", switchName, endianSuffix, streamVar).indent()
			}
			fn.pf("return err")
			fn.unindent().pf("}")
		}
		switchCallWithSize := func() {
			if a.Size != nil {
				e.setImport(unit, "bytes", "bytes")
				fn.tmp++
				fn.pf("_raw_tmp%d, err := stream.ReadBytes(int(%s))", fn.tmp, e.expr(a.Size))
				fn.pf("if err != nil {").indent()
				fn.pf("return err")
				fn.unindent().pf("}")
				// Store raw substream bytes for roundtrip
				if a.Repeat != nil {
					fn.pf("this._raw_%s = append(this._raw_%s, _raw_tmp%d)", string(a.ID), string(a.ID), fn.tmp)
				} else {
					fn.pf("this._raw_%s = _raw_tmp%d", string(a.ID), fn.tmp)
				}
				fn.pf("_sub_tmp%d := kaitai.NewStream(bytes.NewReader(_raw_tmp%d))", fn.tmp, fn.tmp)
				switchCall(fmt.Sprintf("_sub_tmp%d", fn.tmp))
			} else {
				switchCall("stream")
			}
		}
		switch repeat := a.Repeat.(type) {
		case types.RepeatExpr:
			iterType := engine.ResultTypeOfExpr(e.context, repeat.CountExpr)
			iterCast := e.declType(iterType)
			fn.pf("for i := %s(0); i < %s; i++ {", iterCast, e.expr(repeat.CountExpr)).indent()
			switchCallWithSize()
			fn.unindent().pf("}")
		case types.RepeatEOS:
			fn.pf("for i := 0; ; i++ {").indent()
			fn.pf("_ = i")
			fn.pf("if eof, err := stream.EOF(); err != nil {").indent()
			fn.pf("return err")
			fn.unindent().pf("} else if eof {").indent()
			fn.pf("break")
			fn.unindent().pf("}")
			switchCallWithSize()
			fn.unindent().pf("}")
		case nil:
			switchCallWithSize()
		}
	} else {
		switch rt.TypeRef.Kind {
		case types.User:
			// ---------------------------------------------------------------------
			// User case: Need to call Read method of field
			// ---------------------------------------------------------------------
			resolved := e.mustResolveType(rt.TypeRef.User.Name)
			if resolved.Kind != engine.StructKind {
				panic(fmt.Errorf("expression %q yielded unexpected type %s (expected struct)", rt.TypeRef.User.Name, resolved.Kind))
			}
			isOpaque := e.isOpaqueType(resolved)
			// Check if the resolved type is from an imported file (different root).
			// Imported types have any-typed Root_ and should receive nil parent/root.
			isImported := !isOpaque && e.file.rootTypeName != "" && resolved.Struct != nil &&
				e.prefix(resolved.DefParent) != "" && !strings.HasPrefix(e.prefix(resolved.DefParent), e.file.rootTypeName)
			// Determine parent/root expressions for Read() call
			parentExpr := "this"
			rootExpr := "this.Root_"
			if a.Parent != nil && a.Parent.Disabled {
				parentExpr = "this.Parent_" // parent: false - pass through
			}
			if isOpaque || isImported {
				parentExpr = "nil"
				rootExpr = "nil"
			}
			debugRead := e.debug
			ksFieldName := string(a.ID) // original KSY field ID for debug maps

			// Helper: emit debug error check after field assignment
			debugErrCheck := func() {
				fn.pf("if err != nil {").indent()
				fn.pf("return err")
				fn.unindent().pf("}")
			}

			switch repeat := a.Repeat.(type) {
			case types.RepeatEOS:
				if debugRead {
					e.debugArrInit(fn, ksFieldName)
				}
				fn.pf("for i := 0; ; i++ {").indent()
				fn.pf("_ = i")

				// EOF return
				fn.pf("if eof, err := stream.EOF(); err != nil {").indent()
				fn.pf("return err")
				fn.unindent().pf("} else if eof {").indent()
				fn.pf("break")
				fn.unindent().pf("}")

				if debugRead {
					e.debugArrElemStart(fn, ksFieldName)
				}
				// Read
				fn.pf("tmp%d := %s{}", fn.tmp, e.newTypeRef(rt.TypeRef))
				if !isOpaque {
					e.setParams(fmt.Sprintf("tmp%d", fn.tmp), *rt.TypeRef, resolved, fn)
				}
				e.emitUserTypeRead(fn, unit, *rt.TypeRef, endianSuffix, a, parentExpr, rootExpr, debugRead)
				fn.pf("this.%s = append(this.%s, tmp%d)", fieldName, fieldName, fn.tmp)
				if debugRead {
					e.debugArrElemEnd(fn, ksFieldName)
					debugErrCheck()
				}

				fn.unindent().pf("}")

			case types.RepeatExpr:
				if debugRead {
					e.debugArrInit(fn, ksFieldName)
				}
				iterType := engine.ResultTypeOfExpr(e.context, repeat.CountExpr)
				iterCast := e.declType(iterType)
				fn.pf("for i := %s(0); i < %s; i++ {", iterCast, e.expr(repeat.CountExpr)).indent()
				if debugRead {
					e.debugArrElemStart(fn, ksFieldName)
				}
				fn.pf("tmp%d := %s{}", fn.tmp, e.newTypeRef(rt.TypeRef))
				if !isOpaque {
					e.setParams(fmt.Sprintf("tmp%d", fn.tmp), *rt.TypeRef, resolved, fn)
				}
				e.emitUserTypeRead(fn, unit, *rt.TypeRef, endianSuffix, a, parentExpr, rootExpr, debugRead)
				fn.pf("this.%s = append(this.%s, tmp%d)", fieldName, fieldName, fn.tmp)
				if debugRead {
					e.debugArrElemEnd(fn, ksFieldName)
					debugErrCheck()
				}
				fn.unindent().pf("}")

			case types.RepeatUntil:
				if debugRead {
					e.debugArrInit(fn, ksFieldName)
				}
				oldContext := e.context
				e.context = e.context.WithTemporary(engine.NewAliasSymbol(typ, fmt.Sprintf("tmp%d", fn.tmp)))
				fn.pf("{").indent()
				fn.pf("i := 0")
				fn.pf("for {").indent()
				fn.pf("_ = i")
				if debugRead {
					e.debugArrElemStart(fn, ksFieldName)
				}
				fn.pf("tmp%d := %s{}", fn.tmp, e.newTypeRef(rt.TypeRef))
				if !isOpaque {
					e.setParams(fmt.Sprintf("tmp%d", fn.tmp), *rt.TypeRef, resolved, fn)
				}
				e.emitUserTypeRead(fn, unit, *rt.TypeRef, endianSuffix, a, parentExpr, rootExpr, debugRead)
				fn.pf("this.%s = append(this.%s, tmp%d)", fieldName, fieldName, fn.tmp)
				if debugRead {
					e.debugArrElemEnd(fn, ksFieldName)
					debugErrCheck()
				}
				fn.pf("i++")
				fn.pf("if bool(%s) {", e.expr(repeat.UntilExpr)).indent()
				fn.pf("break")
				fn.unindent().pf("}")
				fn.unindent().pf("}")
				fn.unindent().pf("}")
				e.context = oldContext

			case nil:
				if debugRead {
					e.debugAttrStart(fn, ksFieldName)
				}
				fn.pf("tmp%d := %s{}", fn.tmp, e.newTypeRef(rt.TypeRef))
				if !isOpaque {
					e.setParams(fmt.Sprintf("tmp%d", fn.tmp), *rt.TypeRef, resolved, fn)
				}
				e.emitUserTypeRead(fn, unit, *rt.TypeRef, endianSuffix, a, parentExpr, rootExpr, debugRead)
				fn.pf("this.%s = tmp%d", fieldName, fn.tmp)
				if debugRead {
					e.debugAttrEnd(fn, ksFieldName)
					debugErrCheck()
				}
			}

		default:
			// ---------------------------------------------------------------------
			// General case: Need to assign field using readCall function
			// ---------------------------------------------------------------------
			// readCall is computed lazily to avoid side-effect (e.file.needBytes) when raw tail handles the read
			readCall := ""
			getReadCall := func() string {
				if readCall == "" {
					readCall = e.readCallRef(rt.TypeRef)
				}
				return readCall
			}
			_ = getReadCall // ensure used
			ksFieldName := string(a.ID)

			cast := ""
			needEncConv := false
			if a.Type.TypeRef != nil && a.Type.TypeRef.Kind == types.String {
				encoding := ""
				if a.Type.TypeRef.String != nil {
					encoding = a.Type.TypeRef.String.Encoding
				}
				if e.needsEncodingConversion(encoding) {
					needEncConv = true
				} else {
					cast = "string"
				}
			}
			if a.Enum != "" {
				enumType := e.mustResolveType(a.Enum)
				cast = e.declType(enumType)
			}

			assignSuffix := ""
			if rt.TypeRef.Bits != nil && rt.TypeRef.Bits.Width == 1 && a.Enum == "" {
				assignSuffix = " == 1"
			}

			needsRawTail := e.fieldNeedsRawTail(rt)

			switch repeat := a.Repeat.(type) {
			case types.RepeatEOS:
				if e.debug {
					e.debugArrInit(fn, ksFieldName)
				}
				fn.pf("for i := 0; ; i++ {").indent()
				fn.pf("_ = i")

				// EOF return
				fn.pf("if eof, err := stream.EOF(); err != nil {").indent()
				fn.pf("return err")
				fn.unindent().pf("} else if eof {").indent()
				fn.pf("break")
				fn.unindent().pf("}")

				if e.debug {
					e.debugArrElemStart(fn, ksFieldName)
				}
				// Read - with raw tail capture for repeated pad/term fields
				if needsRawTail {
					e.emitRepeatRawTailRead(fn, unit, a, rt, fieldName, cast, assignSuffix)
				} else {
					fn.pf("tmp%d, err := %s", fn.tmp, getReadCall())
					fn.pf("if err != nil {").indent()
					fn.pf("return err")
					fn.unindent().pf("}")
					if a.Process != nil {
						fn.pf("this._raw_%s = append(this._raw_%s, append([]byte(nil), tmp%d...))", string(a.ID), string(a.ID), fn.tmp)
						e.emitProcess(fn, unit, a.Process, fmt.Sprintf("tmp%d", fn.tmp))
					}
					fn.pf("this.%s = append(this.%s, %s(tmp%d)%s)", fieldName, fieldName, cast, fn.tmp, assignSuffix)
				}
				if e.debug {
					e.debugArrElemEnd(fn, ksFieldName)
				}

				fn.unindent().pf("}")

			case types.RepeatExpr:
				if e.debug {
					e.debugArrInit(fn, ksFieldName)
				}
				iterType := engine.ResultTypeOfExpr(e.context, repeat.CountExpr)
				iterCast := e.declType(iterType)
				fn.pf("for i := %s(0); i < %s; i++ {", iterCast, e.expr(repeat.CountExpr)).indent()
				if e.debug {
					e.debugArrElemStart(fn, ksFieldName)
				}
				if needsRawTail {
					e.emitRepeatRawTailRead(fn, unit, a, rt, fieldName, cast, assignSuffix)
				} else {
					fn.pf("tmp%d, err := %s", fn.tmp, getReadCall())
					fn.pf("if err != nil {").indent()
					fn.pf("return err")
					fn.unindent().pf("\t}")
					if a.Process != nil {
						e.emitProcess(fn, unit, a.Process, fmt.Sprintf("tmp%d", fn.tmp))
					}
					fn.pf("this.%s = append(this.%s, %s(tmp%d)%s)", fieldName, fieldName, cast, fn.tmp, assignSuffix)
				}
				if e.debug {
					e.debugArrElemEnd(fn, ksFieldName)
				}
				fn.unindent().pf("}")

			case types.RepeatUntil:
				if e.debug {
					e.debugArrInit(fn, ksFieldName)
				}
				oldContext := e.context
				e.context = e.context.WithTemporary(engine.NewAliasSymbol(typ, fmt.Sprintf("tmp%d", fn.tmp)))
				fn.pf("{").indent()
				fn.pf("i := 0")
				fn.pf("for {").indent()
				fn.pf("_ = i")
				if e.debug {
					e.debugArrElemStart(fn, ksFieldName)
				}
				if needsRawTail {
					e.emitRepeatRawTailRead(fn, unit, a, rt, fieldName, cast, assignSuffix)
				} else {
					fn.pf("tmp%d, err := %s", fn.tmp, getReadCall())
					fn.pf("if err != nil {").indent()
					fn.pf("return err")
					fn.unindent().pf("}")
					fn.pf("this.%s = append(this.%s, %s(tmp%d)%s)", fieldName, fieldName, cast, fn.tmp, assignSuffix)
				}
				if e.debug {
					e.debugArrElemEnd(fn, ksFieldName)
				}
				fn.pf("i++")
				fn.pf("if bool(%s) {", e.expr(repeat.UntilExpr)).indent()
				fn.pf("break")
				fn.unindent().pf("}")
				fn.unindent().pf("}")
				fn.unindent().pf("}")
				e.context = oldContext

			case nil:
				if e.debug {
					e.debugAttrStart(fn, ksFieldName)
				}
				// Check if this field needs raw tail storage for roundtrip (uses outer needsRawTail)
				if needsRawTail {
					// Read raw bytes first, then strip for field value, storing tail for roundtrip
					termByte := -1
					padRight := -1
					include := false
					isEOS := false
					sizeExpr := ""
					if rt.TypeRef.Kind == types.Bytes && rt.TypeRef.Bytes != nil {
						termByte = rt.TypeRef.Bytes.Terminator
						padRight = rt.TypeRef.Bytes.PadRight
						include = rt.TypeRef.Bytes.Include
						isEOS = rt.TypeRef.Bytes.SizeEOS
						if rt.TypeRef.Bytes.Size != nil {
							sizeExpr = e.expr(rt.TypeRef.Bytes.Size)
						}
					} else if rt.TypeRef.Kind == types.String && rt.TypeRef.String != nil {
						termByte = rt.TypeRef.String.Terminator
						padRight = rt.TypeRef.String.PadRight
						include = rt.TypeRef.String.Include
						isEOS = rt.TypeRef.String.SizeEOS
						if rt.TypeRef.String.Size != nil {
							sizeExpr = e.expr(rt.TypeRef.String.Size)
						}
					}
					if isEOS {
						fn.pf("_raw_%d, err := stream.ReadBytesFull()", fn.tmp)
					} else {
						fn.pf("_raw_%d, err := stream.ReadBytes(int(%s))", fn.tmp, sizeExpr)
					}
					fn.pf("if err != nil { return err }")
					fn.pf("var tmp%d []byte", fn.tmp)
					// Check for multi-byte encoding (UTF-16) for terminator search
					isMultiByte := false
					if rt.TypeRef.Kind == types.String && rt.TypeRef.String != nil {
						isMultiByte = isMultiByteEncoding(rt.TypeRef.String.Encoding)
					}
					if termByte >= 0 && isMultiByte {
						// Multi-byte terminator search + optional pad stripping
						e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
						fn.pf("tmp%d = kaitai.BytesTerminateMulti(_raw_%d, []byte{%d, %d}, %v)", fn.tmp, fn.tmp, termByte, termByte, include)
						if padRight >= 0 {
							// Strip pad from terminated result, capture everything after stripped data as tail
							fn.pf("tmp%d = kaitai.BytesStripRight(tmp%d, %d)", fn.tmp, fn.tmp, padRight)
						}
						fn.pf("this._raw_tail_%s = _raw_%d[len(tmp%d):]", string(a.ID), fn.tmp, fn.tmp)
					} else if termByte >= 0 {
						e.file.needBytes = true
						fn.pf("if _i_%d := bytes.IndexByte(_raw_%d, %d); _i_%d != -1 {", fn.tmp, fn.tmp, termByte, fn.tmp)
						fn.indent()
						if include {
							fn.pf("tmp%d = _raw_%d[:_i_%d+1]", fn.tmp, fn.tmp, fn.tmp)
						} else {
							fn.pf("tmp%d = _raw_%d[:_i_%d]", fn.tmp, fn.tmp, fn.tmp)
						}
						fn.pf("this._raw_tail_%s = _raw_%d[_i_%d+1:]", string(a.ID), fn.tmp, fn.tmp)
						fn.unindent()
						fn.pf("} else {")
						fn.indent()
						if padRight >= 0 {
							fn.pf("tmp%d = kaitai.BytesStripRight(_raw_%d, %d)", fn.tmp, fn.tmp, padRight)
							fn.pf("this._raw_tail_%s = _raw_%d[len(tmp%d):]", string(a.ID), fn.tmp, fn.tmp)
						} else {
							fn.pf("tmp%d = _raw_%d", fn.tmp, fn.tmp)
							fn.pf("this._raw_tail_%s = nil", string(a.ID))
						}
						fn.unindent()
						fn.pf("}")
					} else if padRight >= 0 {
						// Only pad-right, no terminator - strip right and save tail
						fn.pf("tmp%d = kaitai.BytesStripRight(_raw_%d, %d)", fn.tmp, fn.tmp, padRight)
						fn.pf("this._raw_tail_%s = _raw_%d[len(tmp%d):]", string(a.ID), fn.tmp, fn.tmp)
					} else {
						fn.pf("tmp%d = _raw_%d", fn.tmp, fn.tmp)
						fn.pf("this._raw_tail_%s = nil", string(a.ID))
					}
				} else {
					fn.pf("tmp%d, err := %s", fn.tmp, getReadCall())
					fn.pf("if err != nil {").indent()
					fn.pf("return err")
					fn.unindent().pf("}")
				}
				if a.Process != nil {
					// Save raw pre-process bytes for roundtrip
					// When raw tail is active, save the FULL pre-strip bytes (_raw_%d)
					// instead of the stripped bytes (tmp%d), since we need both
					// the unprocessed AND unstripped form for roundtrip
					if needsRawTail {
						if a.Repeat != nil {
							fn.pf("this._raw_%s = append(this._raw_%s, append([]byte(nil), _raw_%d...))", string(a.ID), string(a.ID), fn.tmp)
						} else {
							fn.pf("this._raw_%s = append([]byte(nil), _raw_%d...)", string(a.ID), fn.tmp)
						}
					} else {
						if a.Repeat != nil {
							fn.pf("this._raw_%s = append(this._raw_%s, append([]byte(nil), tmp%d...))", string(a.ID), string(a.ID), fn.tmp)
						} else {
							fn.pf("this._raw_%s = append([]byte(nil), tmp%d...)", string(a.ID), fn.tmp)
						}
					}
					e.emitProcess(fn, unit, a.Process, fmt.Sprintf("tmp%d", fn.tmp))
				}
				if needEncConv {
					enc := a.Type.TypeRef.String.Encoding
					decoder := e.encodingDecoder(unit, enc)
					e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
					fn.pf("tmp%d_str, err := kaitai.BytesToStr(tmp%d, %s)", fn.tmp, fn.tmp, decoder)
					fn.pf("if err != nil {").indent()
					fn.pf("return err")
					fn.unindent().pf("}")
					fn.pf("this.%s = tmp%d_str", fieldName, fn.tmp)
				} else {
					fn.pf("this.%s = %s(tmp%d)%s", fieldName, cast, fn.tmp, assignSuffix)
				}
				if e.debug {
					e.debugAttrEnd(fn, ksFieldName)
				}
			}

			if a.Contents != nil {
				e.setImport(unit, "bytes", "bytes")
				e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)

				contentsRef := fmt.Sprintf("tmp%d", fn.tmp)
				if a.Repeat != nil {
					contentsRef = fmt.Sprintf("this.%s[len(this.%s)-1]", fieldName, fieldName)
				}
				fn.pf("if !bytes.Equal(%s, %#v) {", contentsRef, a.Contents).indent()
				fn.pf("return kaitai.NewValidationNotEqualError(%#v, %s, stream, %q)", a.Contents, contentsRef, string(a.ID))
				fn.unindent().pf("}")
			}
		}
	}

	// Generate validation code
	if a.Valid != nil {
		e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
		valRef := fmt.Sprintf("this.%s", fieldName)
		// For repeated fields, validate each element (use last appended)
		if a.Repeat != nil {
			valRef = fmt.Sprintf("this.%s[len(this.%s)-1]", fieldName, fieldName)
		}
		if a.Valid.Eq != "" {
			eqExpr := expr.MustParseExpr(a.Valid.Eq)
			eqStr := e.expr(eqExpr)
			// Check if the field is a byte array - use bytes.Equal
			if a.Type.TypeRef != nil && a.Type.TypeRef.Kind == types.Bytes {
				e.setImport(unit, "bytes", "bytes")
				fn.pf("if !bytes.Equal(%s, %s) {", valRef, eqStr).indent()
			} else if a.Type.TypeSwitch != nil || (a.If != nil && a.Type.TypeRef != nil && needsPointerForNil(e.declTypeRef(a.Type.TypeRef, nil))) {
				// any-typed field (switch type or conditional primitive) -
				// use fmt.Sprintf for type-safe comparison
				e.file.needFmt = true
				fn.pf("if fmt.Sprintf(\"%%v\", %s) != fmt.Sprintf(\"%%v\", %s) {", valRef, eqStr).indent()
			} else {
				fn.pf("if %s != %s {", valRef, eqStr).indent()
			}
			fn.pf("return kaitai.NewValidationNotEqualError(%s, %s, stream, %q)", eqStr, valRef, string(a.ID))
			fn.unindent().pf("}")
		}
		if a.Valid.Min != "" {
			minExpr := expr.MustParseExpr(a.Valid.Min)
			minStr := e.expr(minExpr)
			if a.Type.TypeRef != nil && a.Type.TypeRef.Kind == types.Bytes {
				e.setImport(unit, "bytes", "bytes")
				fn.pf("if bytes.Compare(%s, %s) < 0 {", valRef, minStr).indent()
			} else {
				fn.pf("if %s < %s {", valRef, minStr).indent()
			}
			fn.pf("return kaitai.NewValidationLessThanError(%s, %s, stream, %q)", minStr, valRef, string(a.ID))
			fn.unindent().pf("}")
		}
		if a.Valid.Max != "" {
			maxExpr := expr.MustParseExpr(a.Valid.Max)
			maxStr := e.expr(maxExpr)
			if a.Type.TypeRef != nil && a.Type.TypeRef.Kind == types.Bytes {
				e.setImport(unit, "bytes", "bytes")
				fn.pf("if bytes.Compare(%s, %s) > 0 {", valRef, maxStr).indent()
			} else {
				fn.pf("if %s > %s {", valRef, maxStr).indent()
			}
			fn.pf("return kaitai.NewValidationGreaterThanError(%s, %s, stream, %q)", maxStr, valRef, string(a.ID))
			fn.unindent().pf("}")
		}
		if len(a.Valid.AnyOf) > 0 {
			fn.pf("{").indent()
			fn.pf("_valid := false")
			for _, item := range a.Valid.AnyOf {
				itemExpr := expr.MustParseExpr(item)
				fn.pf("if %s == %s { _valid = true }", valRef, e.expr(itemExpr))
			}
			fn.pf("if !_valid {").indent()
			fn.pf("return kaitai.NewValidationNotAnyOfError(%s, stream, %q)", valRef, string(a.ID))
			fn.unindent().pf("}")
			fn.unindent().pf("}")
		}
		if a.Valid.Expr != "" {
			// Use the engine's temporary value for _ references
			oldContext := e.context
			e.context = e.context.WithTemporary(engine.NewAliasSymbol(typ, valRef))
			validExpr := expr.MustParseExpr(a.Valid.Expr)
			fn.pf("if !(%s) {", e.expr(validExpr)).indent()
			fn.pf("return kaitai.NewValidationExprError(%s, stream, %q)", valRef, string(a.ID))
			fn.unindent().pf("}")
			e.context = oldContext
		}
		if a.Valid.InEnum && a.Enum != "" {
			// Check that the value is a valid enum member
			enumType := e.mustResolveType(a.Enum)
			if enumType.Kind == engine.EnumKind && enumType.Enum != nil {
				fn.pf("switch %s {", valRef).indent()
				for _, ev := range enumType.Enum.Values {
					fn.pf("case %s:", e.enumValueName(enumType.Parent, enumType.Enum, ev.ID))
				}
				fn.pf("\t// valid")
				fn.pf("default:").indent()
				fn.pf("return kaitai.NewValidationNotInEnumError(%s, stream, %q)", valRef, string(a.ID))
				fn.unindent()
				fn.unindent().pf("}")
			}
		}
	}

	if a.If != nil {
		fn.unindent().pf("}")
	}

	return true
}

func (e *Emitter) prefix(typ *engine.ExprValue) string {
	if typ == nil || typ.Struct == nil {
		return ""
	}
	// Build full prefix by walking up the definition-site parent chain (never re-parented)
	return e.prefix(typ.DefParent) + e.typeName(typ.Struct.Type.ID) + "_"
}

// Determines if endian switching may be necessary for a type.
func (e *Emitter) needMultipleEndian(s *kaitai.Struct) bool {
	if s.Meta.Endian.Kind == types.LittleEndian || s.Meta.Endian.Kind == types.BigEndian {
		return false
	}
	for _, attr := range s.Seq {
		if attr.Type.HasDependentEndian() {
			return true
		}
	}
	return false
}

func (e *Emitter) emitReadFunc(unit *goUnit, gs *goStruct, val *engine.ExprValue, forceEndian types.EndianKind) {
	oldEndian := e.endian
	endianSuffix := ""
	if forceEndian != types.UnspecifiedOrder {
		e.endian = forceEndian
		if forceEndian == types.LittleEndian {
			endianSuffix = "LE"
		} else {
			endianSuffix = "BE"
		}
	}
	defer func() {
		e.endian = oldEndian
	}()
	e.mode.needParent = false
	e.mode.needRoot = false
	e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
	readMethod := goFunc{
		recv: goVar{name: "this", typ: "*" + gs.name},
		name: "Read" + endianSuffix,
		in:   []goVar{{name: "stream", typ: "*" + kaitaiStream}, {name: "__parent", typ: "any"}, {name: "__root", typ: "any"}},
		out:  []goVar{{name: "err", typ: "error"}},
	}
	// Initialize debug maps (must come before field reads, after preprintf'd IO/Root/Parent)
	if e.debug {
		readMethod.pf("this.AttrStart_ = make(map[string]int64)")
		readMethod.pf("this.AttrEnd_ = make(map[string]int64)")
		readMethod.pf("this.ArrStart_ = make(map[string][]int64)")
		readMethod.pf("this.ArrEnd_ = make(map[string][]int64)")
	}
	errExit := false
	inBitsMode := false
	totalBits := 0
	alignIdx := 0
	for _, attr := range val.Struct.Attrs {
		a := attr.Attr
		rt := a.Type.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)
		isBits := rt.TypeRef != nil && rt.TypeRef.Kind == types.Bits
		if inBitsMode && !isBits {
			padBits := (8 - (totalBits % 8)) % 8
			if padBits > 0 {
				isLE := e.bitEndian == types.LittleBitEndian
				endianSuffix2 := "Be"
				if isLE {
					endianSuffix2 = "Le"
				}
				readMethod.pf("this._align_%d, _ = stream.ReadBitsInt%s(%d)", alignIdx, endianSuffix2, padBits)
				alignIdx++
			} else {
				readMethod.pf("stream.AlignToByte()")
			}
			totalBits = 0
		}
		if isBits {
			totalBits += rt.TypeRef.Bits.Width
		}
		inBitsMode = isBits
		if !e.emitAttrRead(unit, &readMethod, attr, forceEndian) {
			// We may need to end the function early in some cases.
			errExit = true
			break
		}
	}
	if inBitsMode {
		padBits := (8 - (totalBits % 8)) % 8
		if padBits > 0 {
			isLE := e.bitEndian == types.LittleBitEndian
			endianSuffix2 := "Be"
			if isLE {
				endianSuffix2 = "Le"
			}
			readMethod.pf("this._align_%d, _ = stream.ReadBitsInt%s(%d)", alignIdx, endianSuffix2, padBits)
		} else {
			readMethod.pf("stream.AlignToByte()")
		}
	}
	if !errExit {
		readMethod.pf("return nil")
	}
	e.ensureStructLinks(&readMethod, val)
	// For switch-endian structs, record which endian variant was called
	switch forceEndian {
	case types.LittleEndian:
		readMethod.ppf("this._isLE = true")
	case types.BigEndian:
		readMethod.ppf("this._isLE = false")
	}
	readMethod.ppf("this.IO_ = stream")
	// Recover panics from instance getter closures and return as errors.
	// Instance getters called in expression contexts use panic(err) since they
	// can't return errors from inline closures. This deferred recover converts
	// those panics (e.g., validation errors) back into returned errors.
	readMethod.ppf("defer func() { if r := recover(); r != nil { if e, ok := r.(error); ok { err = e } else { panic(r) } } }()")
	// Assign Parent_ with type assertion if typed.
	// Use safe assertion (non-panicking) since parent: false can cause
	// a different type to be passed at runtime.
	if pgt := e.parentGoType(val.Struct.Type); pgt != "" {
		readMethod.ppf("this.Parent_, _ = __parent.(%s)", pgt)
	} else {
		readMethod.ppf("this.Parent_ = __parent")
	}
	// Assign Root_ with type assertion
	if e.file.rootTypeName != "" {
		readMethod.ppf("this.Root_ = __root.(*%s)", e.file.rootTypeName)
	} else {
		readMethod.ppf("this.Root_ = __root")
	}
	unit.methods = append(unit.methods, readMethod)
}

func (e *Emitter) ensureStructLinks(fn *goFunc, val *engine.ExprValue) {
	// Create typed local variables _parent/_root for use in expressions.
	// When Root_/Parent_ are typed fields, use them directly.
	// Otherwise, type-assert from __root/__parent parameters.
	if e.mode.needRoot {
		if e.file.rootTypeName != "" {
			// Root_ is already typed - use it directly
			fn.ppf("_ = _root")
			fn.ppf("_root := this.Root_")
		} else {
			rootVal := val
			for rootVal.Parent != nil {
				rootVal = rootVal.Parent
			}
			rootType := e.declType(rootVal)
			fn.ppf("_ = _root")
			fn.ppf("_root, _ := __root.(*%s)", rootType)
		}
	}
	if e.mode.needParent {
		var ks *kaitai.Struct
		if val.Struct != nil {
			ks = val.Struct.Type
		}
		if ks != nil && e.parentGoType(ks) != "" {
			// Parent_ is typed - use it directly
			fn.ppf("_ = _parent")
			fn.ppf("_parent := this.Parent_")
		} else {
			// Parent_ is any - _parent is also any
			fn.ppf("_ = _parent")
			fn.ppf("_parent := this.Parent_")
		}
	}
}

func (e *Emitter) endianStubs(unit *goUnit, gs *goStruct) {
	e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
	unit.methods = append(unit.methods, goFunc{
		recv:   goVar{name: "this", typ: "*" + gs.name},
		name:   "ReadBE",
		in:     []goVar{{name: "stream", typ: "*" + kaitaiStream}, {name: "__parent", typ: "any"}, {name: "__root", typ: "any"}},
		out:    []goVar{{name: "err", typ: "error"}},
		source: "\treturn this.Read(stream, __parent, __root)\n",
	})
	unit.methods = append(unit.methods, goFunc{
		recv:   goVar{name: "this", typ: "*" + gs.name},
		name:   "ReadLE",
		in:     []goVar{{name: "stream", typ: "*" + kaitaiStream}, {name: "__parent", typ: "any"}, {name: "__root", typ: "any"}},
		out:    []goVar{{name: "err", typ: "error"}},
		source: "\treturn this.Read(stream, __parent, __root)\n",
	})
}

func (e *Emitter) endianSwitch(unit *goUnit, gs *goStruct, ks *kaitai.Struct, val *engine.ExprValue) {
	e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)

	e.mode.needRoot = false
	e.mode.needParent = false

	fn := goFunc{
		recv: goVar{name: "this", typ: "*" + gs.name},
		name: "Read",
		in:   []goVar{{name: "stream", typ: "*" + kaitaiStream}, {name: "__parent", typ: "any"}, {name: "__root", typ: "any"}},
		out:  []goVar{{name: "err", typ: "error"}},
	}

	// Note: IO/Root/Parent assignments and ensureStructLinks are added via
	// ppf AFTER the switch body is built (see below), ensuring correct ordering.
	// Check if any case values are byte arrays (need bytes.Equal instead of switch)
	hasByteArrayCases := false
	for value := range ks.Meta.Endian.Cases {
		if value != "_" && strings.HasPrefix(value, "[") {
			hasByteArrayCases = true
			break
		}
	}
	switchOnExpr := e.expr(ks.Meta.Endian.SwitchOn)
	if hasByteArrayCases {
		e.file.needBytes = true
		first := true
		for value, endian := range ks.Meta.Endian.Cases {
			if value == "_" {
				continue // handle default after
			}
			caseExpr := expr.MustParseExpr(value)
			caseStr := e.exprNode(caseExpr.Root)
			if first {
				fn.pf("if bytes.Equal(%s, %s) {", switchOnExpr, caseStr).indent()
				first = false
			} else {
				fn.unindent().pf("} else if bytes.Equal(%s, %s) {", switchOnExpr, caseStr).indent()
			}
			if endian == types.LittleEndian {
				fn.pf("return this.ReadLE(stream, __parent, __root)")
			} else {
				fn.pf("return this.ReadBE(stream, __parent, __root)")
			}
		}
		// Handle default case
		if defaultEndian, ok := ks.Meta.Endian.Cases["_"]; ok {
			fn.unindent().pf("} else {").indent()
			if defaultEndian == types.LittleEndian {
				fn.pf("return this.ReadLE(stream, __parent, __root)")
			} else {
				fn.pf("return this.ReadBE(stream, __parent, __root)")
			}
		} else {
			fn.unindent().pf("} else {").indent()
			fn.pf("return kaitai.UndecidedEndiannessError{}")
		}
		fn.unindent().pf("}")
	} else {
		fn.pf("switch %s {", switchOnExpr)
		for value, endian := range ks.Meta.Endian.Cases {
			fn.pf("case %s:", e.typeSwitchCaseValue(value))
			if endian == types.LittleEndian {
				fn.pf("\treturn this.ReadLE(stream, __parent, __root)")
			} else {
				fn.pf("\treturn this.ReadBE(stream, __parent, __root)")
			}
		}
		fn.pf("default:")
		fn.pf("\treturn kaitai.UndecidedEndiannessError{}")
		fn.pf("}")
	}

	e.ensureStructLinks(&fn, val)
	// Prepend IO/Root/Parent assignments (preprintf adds in reverse order)
	fn.ppf("this.IO_ = stream")
	if pgt := e.parentGoType(ks); pgt != "" {
		fn.ppf("this.Parent_, _ = __parent.(%s)", pgt)
	} else {
		fn.ppf("this.Parent_ = __parent")
	}
	if e.file.rootTypeName != "" {
		fn.ppf("this.Root_ = __root.(*%s)", e.file.rootTypeName)
	} else {
		fn.ppf("this.Root_ = __root")
	}
	unit.methods = append(unit.methods, fn)
}

func (e *Emitter) struc(inputname string, unit *goUnit, val *engine.ExprValue) {
	ks := val.Struct.Type

	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Errorf("struct %s: %s", ks.ID, r))
		}
	}()

	defer e.pushMetaScope(ks)()

	name := e.typeName(ks.ID)
	prefix := e.prefix(val.DefParent)

	gs := goStruct{name: prefix + name}

	defer e.enterLocal(val)()

	// Handle imports before anything else...
	for _, n := range ks.Meta.Imports {
		inputname, s, err := e.resolver.Resolve(inputname, n)
		if err != nil {
			panic(err)
		}
		e.root(inputname, s)
	}

	// Then handle nested structures.
	// Pre-create ExprValues for all child structs so siblings can reference each other
	// as parents (e.g., entry's parent is index_obj, both defined under nav_parent).
	childValues := map[string]*engine.ExprValue{}
	for _, n := range val.Struct.Structs {
		childValues[string(n.Struct.Type.ID)] = engine.NewStructValueSymbol(n, val)
	}
	// Fix parent pointers based on usage-site parent type map
	for _, n := range val.Struct.Structs {
		childID := string(n.Struct.Type.ID)
		if parentType, ok := e.file.parents.Inferred[n.Struct.Type]; ok && parentType != nil {
			parentID := string(parentType.Struct.Type.ID)
			if parentVal, exists := childValues[parentID]; exists && parentID != childID {
				childValues[childID].Parent = parentVal
			}
		}
	}
	for _, n := range val.Struct.Structs {
		e.struc(inputname, unit, childValues[string(n.Struct.Type.ID)])
	}

	// Enumerations
	for _, n := range val.Struct.Enums {
		e.enum(unit, n)
	}

	// Parameter fields
	for _, param := range ks.Params {
		gs.fields = append(gs.fields, goVar{
			name: e.fieldName(param.ID),
			typ:  e.declTypeRef(&param.Type, nil),
		})
	}

	// Attribute fields
	for _, attr := range val.Struct.Attrs {
		fieldType := e.declType(attr)
		if attr.Attr.Enum != "" {
			enumType := e.mustResolveType(attr.Attr.Enum)
			fieldType = e.declType(enumType)
			if attr.Attr.Repeat != nil {
				fieldType = "[]" + fieldType
			}
		}
		// For conditional seq attrs (if: expr), use 'any' for primitives
		// so nil can be returned when condition is false
		if attr.Attr.If != nil && needsPointerForNil(fieldType) {
			fieldType = "any"
		}
		gs.fields = append(gs.fields, goVar{
			name: e.fieldName(attr.Attr.ID),
			typ:  fieldType,
		})
	}
	// Add alignment fields for bit->byte transitions
	{
		inBits := false
		totalBits := 0
		alignIdx := 0
		for _, attr := range val.Struct.Attrs {
			a := attr.Attr
			rt := a.Type.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)
			isBits := rt.TypeRef != nil && rt.TypeRef.Kind == types.Bits
			if inBits && !isBits {
				padBits := (8 - (totalBits % 8)) % 8
				if padBits > 0 {
					gs.fields = append(gs.fields, goVar{
						name: fmt.Sprintf("_align_%d", alignIdx),
						typ:  "uint64",
					})
					alignIdx++
				}
				totalBits = 0
			}
			if isBits {
				totalBits += rt.TypeRef.Bits.Width
			}
			inBits = isBits
		}
		if inBits {
			padBits := (8 - (totalBits % 8)) % 8
			if padBits > 0 {
				gs.fields = append(gs.fields, goVar{
					name: fmt.Sprintf("_align_%d", alignIdx),
					typ:  "uint64",
				})
			}
		}
	}
	// Add raw tail fields for fields that need roundtrip metadata
	for _, attr := range val.Struct.Attrs {
		a := attr.Attr
		rt := a.Type.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)
		needsRawTail := e.fieldNeedsRawTail(rt)
		// Also check user types with attr-level terminator or pad-right (any stripping)
		if !needsRawTail && rt.TypeRef != nil && rt.TypeRef.Kind == types.User {
			if (a.Terminator != nil && *a.Terminator >= 0) || (a.PadRight != nil && *a.PadRight >= 0) {
				needsRawTail = true
			}
		}
		if needsRawTail {
			fieldType := "[]byte"
			if a.Repeat != nil {
				fieldType = "[][]byte"
			}
			gs.fields = append(gs.fields, goVar{
				name: "_raw_tail_" + string(a.ID),
				typ:  fieldType,
			})
		}
	}
	// Add _if_ fields for IO-dependent conditional fields
	for _, attr := range val.Struct.Attrs {
		a := attr.Attr
		if a.If != nil && exprReferencesIO(a.If) {
			gs.fields = append(gs.fields, goVar{
				name: "_if_" + string(a.ID),
				typ:  "bool",
			})
		}
	}
	// Add _raw_ fields for sized user types and process fields (substream/process roundtrip)
	for _, attr := range val.Struct.Attrs {
		a := attr.Attr
		rt := a.Type.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)
		needsRaw := false
		if rt.TypeRef != nil && rt.TypeRef.Kind == types.User && a.Size != nil {
			needsRaw = true
		}
		// TypeSwitch fields with size need raw storage
		if a.Type.TypeSwitch != nil && a.Size != nil {
			needsRaw = true
		}
		// User types with process + terminator need raw storage
		if rt.TypeRef != nil && rt.TypeRef.Kind == types.User && a.Process != nil && a.Terminator != nil {
			needsRaw = true
		}
		// Process fields on primitives need raw storage for non-invertible processes
		if a.Process != nil && rt.TypeRef != nil && (rt.TypeRef.Kind == types.Bytes || rt.TypeRef.Kind == types.String) {
			needsRaw = true
		}
		if needsRaw {
			fieldType := "[]byte"
			if a.Repeat != nil {
				fieldType = "[][]byte"
			}
			gs.fields = append(gs.fields, goVar{
				name: "_raw_" + string(a.ID),
				typ:  fieldType,
			})
		}
	}
	gs.fields = append(gs.fields, goVar{
		name: "IO_",
		typ:  "*" + kaitaiStream,
	})
	// Type Root_ as the concrete root struct type
	rootFieldType := "any"
	if e.file.rootTypeName != "" {
		rootFieldType = "*" + e.file.rootTypeName
	}
	gs.fields = append(gs.fields, goVar{
		name: "Root_",
		typ:  rootFieldType,
	})
	// Type Parent_ based on parent type inference
	parentFieldType := "any"
	if pgt := e.parentGoType(ks); pgt != "" {
		parentFieldType = pgt
	}
	gs.fields = append(gs.fields, goVar{
		name: "Parent_",
		typ:  parentFieldType,
	})

	// Debug mode fields
	if e.debug {
		gs.fields = append(gs.fields,
			goVar{name: "AttrStart_", typ: "map[string]int64"},
			goVar{name: "AttrEnd_", typ: "map[string]int64"},
			goVar{name: "ArrStart_", typ: "map[string][]int64"},
			goVar{name: "ArrEnd_", typ: "map[string][]int64"},
		)
	}

	// Deserialization
	isSwitchEndian := e.endian == types.SwitchEndian || (e.needMultipleEndian(ks) && e.endian == types.UnspecifiedOrder)
	if isSwitchEndian {
		// Store endian choice at runtime so instance getters can use it
		gs.fields = append(gs.fields, goVar{name: "_isLE", typ: "bool"})
	}
	if isSwitchEndian {
		if ks.Meta.Endian.Kind == types.SwitchEndian && ks.Meta.Endian.SwitchOn != nil {
			// This struct has its own endian switch
			e.endianSwitch(unit, &gs, ks, val)
		} else if e.endian == types.SwitchEndian {
			// Inherited endian switch - no Read dispatch, just LE/BE variants.
			// The parent struct's dispatch will call ReadLE or ReadBE directly.
			// Generate a Read stub that returns UndecidedEndiannessError.
			e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
			unit.methods = append(unit.methods, goFunc{
				recv:   goVar{name: "this", typ: "*" + gs.name},
				name:   "Read",
				in:     []goVar{{name: "stream", typ: "*" + kaitaiStream}, {name: "__parent", typ: "any"}, {name: "__root", typ: "any"}},
				out:    []goVar{{name: "err", typ: "error"}},
				source: "\treturn kaitai.UndecidedEndiannessError{}\n",
			})
		} else {
			// Generate unspecified endian even if it does always return an error.
			e.emitReadFunc(unit, &gs, val, types.UnspecifiedOrder)
		}
		e.emitReadFunc(unit, &gs, val, types.LittleEndian)
		e.emitReadFunc(unit, &gs, val, types.BigEndian)

		for _, attr := range val.Struct.Attrs {
			if attr.Attr.Type.TypeSwitch != nil {
				e.emitTypeSwitchStruct(unit, attr)
				e.emitTypeSwitchRead(unit, attr, types.LittleEndian)
				e.emitTypeSwitchRead(unit, attr, types.BigEndian)
			}
		}
		for _, inst := range val.Struct.Instances {
			if inst.Instance != nil && inst.Instance.Type.TypeSwitch != nil {
				// Wrap instance as attr for typeSwitch functions
				attrSym := &engine.ExprValue{Kind: engine.AttrKind, Parent: inst.Parent, Attr: inst.Instance}
				e.emitTypeSwitchStruct(unit, attrSym)
				e.emitTypeSwitchReadWithPrefix(unit, attrSym, types.LittleEndian, "_inst_")
				e.emitTypeSwitchReadWithPrefix(unit, attrSym, types.BigEndian, "_inst_")
			}
		}

		// Write methods for switch-endian
		e.endianSwitchWrite(unit, &gs)
		e.emitWriteFunc(unit, &gs, val, types.LittleEndian)
		e.emitWriteFunc(unit, &gs, val, types.BigEndian)
		for _, attr := range val.Struct.Attrs {
			if attr.Attr.Type.TypeSwitch != nil {
				e.emitTypeSwitchWrite(unit, attr, types.LittleEndian)
				e.emitTypeSwitchWrite(unit, attr, types.BigEndian)
			}
		}
		for _, inst := range val.Struct.Instances {
			if inst.Instance != nil && inst.Instance.Type.TypeSwitch != nil {
				attrSym := &engine.ExprValue{Kind: engine.AttrKind, Parent: inst.Parent, Attr: inst.Instance}
				e.emitTypeSwitchWrite(unit, attrSym, types.LittleEndian)
				e.emitTypeSwitchWrite(unit, attrSym, types.BigEndian)
			}
		}
	} else {
		// Struct is always consistent endianness: generate one read function and make two stubs to it.
		e.emitReadFunc(unit, &gs, val, types.UnspecifiedOrder)
		e.endianStubs(unit, &gs)

		for _, attr := range val.Struct.Attrs {
			if attr.Attr.Type.TypeSwitch != nil {
				e.emitTypeSwitchStruct(unit, attr)
				e.emitTypeSwitchRead(unit, attr, types.UnspecifiedOrder)
			}
		}
		for _, inst := range val.Struct.Instances {
			if inst.Instance != nil && inst.Instance.Type.TypeSwitch != nil {
				attrSym := &engine.ExprValue{Kind: engine.AttrKind, Parent: inst.Parent, Attr: inst.Instance}
				e.emitTypeSwitchStruct(unit, attrSym)
				e.emitTypeSwitchReadWithPrefix(unit, attrSym, types.UnspecifiedOrder, "_inst_")
			}
		}

		// Write method for consistent endianness
		e.emitWriteFunc(unit, &gs, val, types.UnspecifiedOrder)
		e.endianStubsWrite(unit, &gs)
		for _, attr := range val.Struct.Attrs {
			if attr.Attr.Type.TypeSwitch != nil {
				e.emitTypeSwitchWrite(unit, attr, types.UnspecifiedOrder)
			}
		}
		for _, inst := range val.Struct.Instances {
			if inst.Instance != nil && inst.Instance.Type.TypeSwitch != nil {
				attrSym := &engine.ExprValue{Kind: engine.AttrKind, Parent: inst.Parent, Attr: inst.Instance}
				e.emitTypeSwitchWrite(unit, attrSym, types.UnspecifiedOrder)
			}
		}
	}

	// Positioned instance writer helpers
	for _, inst := range val.Struct.Instances {
		if inst.Instance != nil && (inst.Instance.Pos != nil || inst.Instance.IO != nil) && inst.Instance.Value == nil {
			e.positionedInstanceWrite(unit, &gs, inst)
		}
	}

	// Instance fields and methods
	for _, inst := range val.Struct.Instances {
		instAttr := inst.Instance
		instFieldName := e.fieldName(instAttr.ID)
		// inferInstanceType is the canonical resolution path - declType is
		// only a fallback for IO/Stream-typed values that aren't yet wired
		// through inferInstanceType, but it's currently overwritten in
		// every case below, so skip it entirely.
		instType := e.inferInstanceType(inst)

		// Override with enum type if instance has enum: key
		if instAttr.Enum != "" {
			enumType := e.mustResolveType(instAttr.Enum)
			instType = e.declType(enumType)
		}

		// For conditional instances (if: expr), use 'any' so nil can be
		// returned when the condition is false.
		if instAttr.If != nil && needsPointerForNil(instType) {
			instType = "any"
		}

		// Add flag and cached value fields
		gs.fields = append(gs.fields, goVar{
			name: "_f_computed_" + string(instAttr.ID),
			typ:  "bool",
		})
		gs.fields = append(gs.fields, goVar{
			name: "_inst_" + string(instAttr.ID),
			typ:  instType,
		})

		// Generate getter method
		e.instanceGetter(unit, &gs, val, inst, instFieldName, instType)
	}

	unit.structs = append(unit.structs, gs)

	// Generate SeqFields variable for debug mode
	if e.debug {
		var fieldNames []string
		for _, attr := range ks.Seq {
			fieldNames = append(fieldNames, fmt.Sprintf("%q", string(attr.ID)))
		}
		unit.vars = append(unit.vars, fmt.Sprintf("var %s_SeqFields = []string{%s}",
			gs.name, strings.Join(fieldNames, ", ")))
	}

	// Generate KS_Parent() and KS_Root() accessor methods.
	// These enable navigating parent/root chains through any-typed values
	// via interface literals like: val.(interface{ KS_Parent() any }).KS_Parent()
	unit.methods = append(unit.methods, goFunc{
		recv:   goVar{name: "this", typ: "*" + gs.name},
		name:   "KS_Parent",
		out:    []goVar{{typ: "any"}},
		source: "\treturn this.Parent_\n",
	})
	unit.methods = append(unit.methods, goFunc{
		recv:   goVar{name: "this", typ: "*" + gs.name},
		name:   "KS_Root",
		out:    []goVar{{typ: "any"}},
		source: "\treturn this.Root_\n",
	})

	unit.methods = append(unit.methods, goFunc{
		recv:   goVar{name: "this", typ: "*" + gs.name},
		name:   "KS_IO",
		out:    []goVar{{typ: "*" + kaitaiStream}},
		source: "\treturn this.IO_\n",
	})

	// Generate String() method for to-string: key
	if ks.ToString != "" {
		toStringExpr := expr.MustParseExpr(ks.ToString)
		if toStringExpr != nil {
			restore := e.saveExprMode()
			e.mode.needRoot = false
			e.mode.needParent = false
			toStringBody := e.expr(toStringExpr)
			fn := goFunc{
				recv: goVar{name: "this", typ: gs.name},
				name: "String",
				out:  []goVar{{typ: "string"}},
			}
			fn.pf("return %s", toStringBody)
			unit.methods = append(unit.methods, fn)
			restore()
		}
	}
}

func (e *Emitter) instanceGetter(unit *goUnit, gs *goStruct, val *engine.ExprValue, inst *engine.ExprValue, fieldName string, retType string) {
	instAttr := inst.Instance
	instID := string(instAttr.ID)
	e.mode.needRoot = false
	e.mode.needParent = false

	fn := goFunc{
		recv: goVar{name: "this", typ: "*" + gs.name},
		name: fieldName,
		out:  []goVar{{name: "v", typ: retType}, {name: "err", typ: "error"}},
	}

	fn.pf("if this._f_computed_%s {", instID).indent()
	fn.pf("return this._inst_%s, nil", instID)
	fn.unindent().pf("}")

	if instAttr.If != nil {
		fn.pf("if %s {", e.expr(instAttr.If)).indent()
	}

	if instAttr.Value != nil {
		// Value instance: compute from expression
		valStr := e.expr(instAttr.Value)
		if retType == "any" {
			// any-typed field (e.g., conditional instance): assign value directly
			fn.pf("this._inst_%s = %s", instID, valStr)
		} else {
			cast := retType
			fn.pf("this._inst_%s = (%s)(%s)", instID, cast, valStr)
		}
	} else if instAttr.Type.TypeSwitch != nil && instAttr.Pos == nil && instAttr.IO == nil {
		// Type switch instance without positioning: call the generated read function directly
		ts := instAttr.Type.TypeSwitch
		typeSwitchName := e.prefix(inst.Parent) + e.typeSwitchName(ts.FieldName)
		fn.pf("if err = this.read%s(stream); err != nil {", typeSwitchName).indent()
		fn.pf("return v, err")
		fn.unindent().pf("}")
	} else if instAttr.Pos != nil || instAttr.IO != nil {
		// Position/IO instance: seek and read
		e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)

		// Determine which stream to use
		streamExpr := "this.IO_"
		if instAttr.IO != nil {
			streamExpr = e.expr(instAttr.IO)
		}

		fn.pf("_io := %s", streamExpr)
		fn.pf("_pos, err := _io.Pos()")
		fn.pf("if err != nil {").indent()
		fn.pf("return v, err")
		fn.unindent().pf("}")
		fn.pf("_, err = _io.Seek(int64(%s), 0)", e.expr(instAttr.Pos))
		fn.pf("if err != nil {").indent()
		fn.pf("return v, err")
		fn.unindent().pf("}")

		// For switch-endian structs, generate if/else for LE/BE reading
		if e.endian == types.SwitchEndian {
			rtLE := instAttr.Type.FoldEndian(types.LittleEndian).FoldBitEndian(e.bitEndian)
			rtBE := instAttr.Type.FoldEndian(types.BigEndian).FoldBitEndian(e.bitEndian)
			if rtLE.TypeRef != nil && rtLE.TypeRef.Kind == types.User {
				// User types - call ReadLE/ReadBE
				fn.tmp++
				resolved := e.mustResolveType(rtLE.TypeRef.User.Name)
				instParent, instRoot := "this", "this.Root_"
				if e.isOpaqueType(resolved) {
					instParent, instRoot = "nil", "nil"
				}
				fn.pf("tmp%d := %s{}", fn.tmp, e.newTypeRef(rtLE.TypeRef))
				fn.pf("if this._isLE {").indent()
				fn.pf("if err = tmp%d.ReadLE(_io, %s, %s); err != nil {", fn.tmp, instParent, instRoot).indent()
				fn.pf("return v, err")
				fn.unindent().pf("}")
				fn.unindent().pf("} else {").indent()
				fn.pf("if err = tmp%d.ReadBE(_io, %s, %s); err != nil {", fn.tmp, instParent, instRoot).indent()
				fn.pf("return v, err")
				fn.unindent().pf("}")
				fn.unindent().pf("}")
				fn.pf("this._inst_%s = tmp%d", instID, fn.tmp)
				_ = resolved
			} else if rtLE.TypeRef != nil {
				// Primitive types - use the right endian read call
				fn.tmp++
				fn.pf("if this._isLE {").indent()
				callLE := e.readCallRefOn("_io", rtLE.TypeRef)
				fn.pf("tmp%d, err := %s", fn.tmp, callLE)
				fn.pf("if err != nil {").indent()
				fn.pf("return v, err")
				fn.unindent().pf("}")
				fn.pf("this._inst_%s = tmp%d", instID, fn.tmp)
				fn.unindent().pf("} else {").indent()
				fn.tmp++
				callBE := e.readCallRefOn("_io", rtBE.TypeRef)
				fn.pf("tmp%d, err := %s", fn.tmp, callBE)
				fn.pf("if err != nil {").indent()
				fn.pf("return v, err")
				fn.unindent().pf("}")
				fn.pf("this._inst_%s = tmp%d", instID, fn.tmp)
				fn.unindent().pf("}")
			}
			fn.pf("_, err = _io.Seek(_pos, 0)")
			fn.pf("if err != nil {").indent()
			fn.pf("return v, err")
			fn.unindent().pf("}")
		} else {

			// Handle TypeSwitch instances with pos/io: call the switch read with the positioned stream
			if instAttr.Type.TypeSwitch != nil {
				ts := instAttr.Type.TypeSwitch
				typeSwitchName := e.prefix(inst.Parent) + e.typeSwitchName(ts.FieldName)
				fn.pf("if err = this.read%s(_io); err != nil {", typeSwitchName).indent()
				fn.pf("return v, err")
				fn.unindent().pf("}")
				fn.pf("_, err = _io.Seek(_pos, 0)")
				fn.pf("if err != nil {").indent()
				fn.pf("return v, err")
				fn.unindent().pf("}")
			} else {

				rt := instAttr.Type.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)

				// If instance has its own size and is NOT repeated, create a substream
				readStream := "_io"
				if instAttr.Size != nil && instAttr.Repeat == nil {
					e.setImport(unit, "bytes", "bytes")
					fn.pf("_raw, err := _io.ReadBytes(int(%s))", e.expr(instAttr.Size))
					fn.pf("if err != nil {").indent()
					fn.pf("return v, err")
					fn.unindent().pf("}")
					fn.pf("_io_sub := kaitai.NewStream(bytes.NewReader(_raw))")
					readStream = "_io_sub"
				}

				if rt.TypeRef != nil && rt.TypeRef.Kind == types.User {
					resolved := e.mustResolveType(rt.TypeRef.User.Name)
					isOpaque := e.isOpaqueType(resolved)
					instParent, instRoot := "this", "this.Root_"
					if isOpaque {
						instParent, instRoot = "nil", "nil"
					}
					if repeat, ok := instAttr.Repeat.(types.RepeatExpr); ok {
						// Repeated position instance
						if instAttr.Size != nil {
							e.setImport(unit, "bytes", "bytes")
						}
						fn.pf("for i := 0; i < int(%s); i++ {", e.expr(repeat.CountExpr)).indent()
						if instAttr.Size != nil {
							// Re-read size bytes for each iteration
							fn.pf("_raw_i, err := _io.ReadBytes(int(%s))", e.expr(instAttr.Size))
							fn.pf("if err != nil {").indent()
							fn.pf("return v, err")
							fn.unindent().pf("}")
							fn.pf("_io_i := kaitai.NewStream(bytes.NewReader(_raw_i))")
							fn.tmp++
							fn.pf("tmp%d := %s{}", fn.tmp, e.newTypeRef(rt.TypeRef))
							if !isOpaque {
								e.setParams(fmt.Sprintf("tmp%d", fn.tmp), *rt.TypeRef, resolved, &fn)
							}
							fn.pf("if err = tmp%d.Read(_io_i, %s, %s); err != nil {", fn.tmp, instParent, instRoot).indent()
							fn.pf("return v, err")
							fn.unindent().pf("}")
						} else {
							fn.tmp++
							fn.pf("tmp%d := %s{}", fn.tmp, e.newTypeRef(rt.TypeRef))
							if !isOpaque {
								e.setParams(fmt.Sprintf("tmp%d", fn.tmp), *rt.TypeRef, resolved, &fn)
							}
							fn.pf("if err = tmp%d.Read(%s, %s, %s); err != nil {", fn.tmp, readStream, instParent, instRoot).indent()
							fn.pf("return v, err")
							fn.unindent().pf("}")
						}
						fn.pf("this._inst_%s = append(this._inst_%s, tmp%d)", instID, instID, fn.tmp)
						fn.unindent().pf("}")
					} else {
						// Single position instance
						fn.tmp++
						fn.pf("tmp%d := %s{}", fn.tmp, e.newTypeRef(rt.TypeRef))
						if !isOpaque {
							e.setParams(fmt.Sprintf("tmp%d", fn.tmp), *rt.TypeRef, resolved, &fn)
						}
						fn.pf("if err = tmp%d.Read(%s, %s, %s); err != nil {", fn.tmp, readStream, instParent, instRoot).indent()
						fn.pf("return v, err")
						fn.unindent().pf("}")
						fn.pf("this._inst_%s = tmp%d", instID, fn.tmp)
					}
				} else if rt.TypeRef != nil {
					readCall := e.readCallRefOn(readStream, rt.TypeRef)
					cast := ""
					if instAttr.Type.TypeRef != nil && instAttr.Type.TypeRef.Kind == types.String {
						cast = "string"
					}
					switch repeat := instAttr.Repeat.(type) {
					case types.RepeatEOS:
						fn.pf("for {").indent()
						fn.pf("if eof, err := %s.EOF(); err != nil {", readStream).indent()
						fn.pf("return v, err")
						fn.unindent().pf("} else if eof {").indent()
						fn.pf("break")
						fn.unindent().pf("}")
						fn.tmp++
						fn.pf("tmp%d, err := %s", fn.tmp, readCall)
						fn.pf("if err != nil {").indent()
						fn.pf("return v, err")
						fn.unindent().pf("}")
						fn.pf("this._inst_%s = append(this._inst_%s, %s(tmp%d))", instID, instID, cast, fn.tmp)
						fn.unindent().pf("}")
					case types.RepeatExpr:
						fn.pf("for i := 0; i < int(%s); i++ {", e.expr(repeat.CountExpr)).indent()
						fn.tmp++
						// If the instance has its own size, read that many bytes per iteration
						if instAttr.Size != nil {
							fn.pf("tmp%d, err := %s.ReadBytes(int(%s))", fn.tmp, readStream, e.expr(instAttr.Size))
						} else {
							fn.pf("tmp%d, err := %s", fn.tmp, readCall)
						}
						fn.pf("if err != nil {").indent()
						fn.pf("return v, err")
						fn.unindent().pf("}")
						fn.pf("this._inst_%s = append(this._inst_%s, %s(tmp%d))", instID, instID, cast, fn.tmp)
						fn.unindent().pf("}")
					default:
						fn.tmp++
						fn.pf("tmp%d, err := %s", fn.tmp, readCall)
						fn.pf("if err != nil {").indent()
						fn.pf("return v, err")
						fn.unindent().pf("}")
						fn.pf("this._inst_%s = %s(tmp%d)", instID, cast, fn.tmp)
					}
				}

				fn.pf("_, err = _io.Seek(_pos, 0)")
				fn.pf("if err != nil {").indent()
				fn.pf("return v, err")
				fn.unindent().pf("}")
			} // close the else for non-TypeSwitch
		} // close the else from switch-endian check
	}

	// Validation for instances: contents check
	if instAttr.Contents != nil {
		e.setImport(unit, "bytes", "bytes")
		e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
		valRef := fmt.Sprintf("this._inst_%s", instID)
		fn.pf("if !bytes.Equal(%s, %#v) {", valRef, instAttr.Contents).indent()
		fn.pf("return v, kaitai.NewValidationNotEqualError(%#v, %s, stream, %q)", instAttr.Contents, valRef, string(instAttr.ID))
		fn.unindent().pf("}")
	}

	// Validation for instances: valid: spec
	if instAttr.Valid != nil {
		e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
		valRef := fmt.Sprintf("this._inst_%s", instID)
		if instAttr.Repeat != nil {
			valRef = fmt.Sprintf("this._inst_%s[len(this._inst_%s)-1]", instID, instID)
		}
		if instAttr.Valid.Eq != "" {
			eqExpr := expr.MustParseExpr(instAttr.Valid.Eq)
			eqStr := e.expr(eqExpr)
			if instAttr.Type.TypeRef != nil && instAttr.Type.TypeRef.Kind == types.Bytes {
				e.setImport(unit, "bytes", "bytes")
				fn.pf("if !bytes.Equal(%s, %s) {", valRef, eqStr).indent()
			} else {
				fn.pf("if %s != %s {", valRef, eqStr).indent()
			}
			fn.pf("return v, kaitai.NewValidationNotEqualError(%s, %s, stream, %q)", eqStr, valRef, string(instAttr.ID))
			fn.unindent().pf("}")
		}
	}

	if instAttr.If != nil {
		fn.unindent().pf("}")
	}

	fn.pf("this._f_computed_%s = true", instID)
	fn.pf("return this._inst_%s, nil", instID)

	// Instance getters don't have stream/__root/__parent as parameters,
	// so we define them as local variables from struct fields.
	// We use printf to add at the END of the function prologue (before body).
	// Instead of preprintf, let's build a prologue and set it directly.
	// The defer/recover converts panics from expression evaluation (e.g. .to_i
	// on invalid input) into error returns.
	prologue := "\tdefer func() { if r := recover(); r != nil { if e, ok := r.(error); ok { err = e } else { panic(r) } } }()\n"
	prologue += "\tstream := this.IO_\n\t_ = stream\n"
	if e.mode.needRoot || e.mode.needParent {
		prologue += "\t__root := this.Root_\n\t_ = __root\n"
		prologue += "\t__parent := this.Parent_\n\t_ = __parent\n"
		e.ensureStructLinks(&fn, val)
	}
	fn.source = prologue + fn.source
	unit.methods = append(unit.methods, fn)
}
