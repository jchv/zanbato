package wireshark

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/emitter"
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/resolve"
	"github.com/jchv/zanbato/kaitai/types"
)

// Emitter emits Wireshark dissectors for kaitai structs.
type Emitter struct {
	outpath   string
	resolver  resolve.Resolver
	endian    types.EndianKind
	bitEndian types.BitEndianKind
	context   *engine.Context
	artifacts []emitter.Artifact
}

// NewEmitter constructs a new emitter with the given parameters.
func NewEmitter(outpath string, resolver resolve.Resolver) *Emitter {
	return &Emitter{
		outpath:  path.Base(outpath),
		resolver: resolver,
		context:  engine.NewContext(),
	}
}

// Emit emits Go code for the given kaitai struct.
func (e *Emitter) Emit(inputname string, s *kaitai.Struct) []emitter.Artifact {
	e.root(inputname, s)
	return e.artifacts
}

func (e *Emitter) root(inputname string, s *kaitai.Struct) {
	oldEndian := e.endian
	oldBitEndian := e.bitEndian

	defer func() {
		e.endian = oldEndian
		e.bitEndian = oldBitEndian
	}()

	if s.Meta.Endian.Kind != types.UnspecifiedOrder {
		e.endian = s.Meta.Endian.Kind
	}
	if s.Meta.BitEndian.Kind != types.UnspecifiedBitOrder {
		e.bitEndian = s.Meta.BitEndian.Kind
	}

	mod := module{}

	rootType := engine.NewStructSymbol(s, nil)
	root := engine.NewStructValueSymbol(rootType, nil)
	e.context.AddGlobalType(string(s.ID), rootType)
	e.context.AddModuleType(string(s.ID), rootType)
	oldContext := e.context
	e.context = e.context.WithModuleRoot(root).WithLocalRoot(root)

	e.struc(inputname, &mod, root)

	// Pivot back to old root
	e.context = oldContext

	out := bytes.Buffer{}
	mod.emit(&out)

	e.artifacts = append(e.artifacts, emitter.Artifact{
		Filename: e.filename(s.ID),
		Body:     out.Bytes(),
	})
}

func (e *Emitter) struc(inputname string, mod *module, val *engine.ExprValue) {
	ks := val.Struct.Type

	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Errorf("struct %s: %s", ks.ID, r))
		}
	}()

	name := e.typeName(ks.ID)
	prefix := e.prefix(val.DefParent)

	ps := protocol{name: prefix + name}

	oldContext := e.context
	e.context = e.context.WithLocalRoot(val)
	defer func() { e.context = oldContext }()

	// Handle imports before anything else...
	for _, n := range ks.Meta.Imports {
		inputname, s, err := e.resolver.Resolve(inputname, n)
		if err != nil {
			panic(err)
		}
		e.root(inputname, s)
	}

	// Then handle nested structures
	for _, n := range val.Struct.Structs {
		e.struc(inputname, mod, engine.NewStructValueSymbol(n, val))
	}

	// Enumerations: not yet generated for Wireshark dissectors. The Lua
	// dissector API expects enum decoding to be inlined per-field via
	// VALS({...}) - wiring that up here is unimplemented.
	for _, n := range val.Struct.Enums {
		_ = n
	}

	// Attribute fields
	for _, attr := range val.Struct.Attrs {
		ps.fields = append(ps.fields, field{
			name: e.fieldName(attr.Attr.ID),
			typ:  e.fieldType(attr),
			base: "DEC",
			desc: attr.Attr.Doc,
		})
	}

	// Deserialization: the Wireshark dissector emitter currently builds the
	// field table (ps.fields above) but does NOT emit a runtime dissect
	// function. Each branch below is a stub for the read-side code we would
	// generate once dissection is wired up - endian switching, multi-endian
	// reads, and type-switch dispatch all need their Lua counterparts.
	if e.endian == types.SwitchEndian || (e.needMultipleEndian(ks) && e.endian == types.UnspecifiedOrder) {
		// e.endianSwitch / e.strucRead (multi-endian) - unimplemented.
		for _, attr := range val.Struct.Attrs {
			if attr.Attr.Type.TypeSwitch != nil {
				// e.typeSwitchStruct / e.typeSwitch (multi-endian) - unimplemented.
				_ = attr
			}
		}
	} else {
		// e.strucRead (single endian) + e.endianStubs - unimplemented.
		for _, attr := range val.Struct.Attrs {
			if attr.Attr.Type.TypeSwitch != nil {
				// e.typeSwitchStruct / e.typeSwitch (single endian) - unimplemented.
				_ = attr
			}
		}
	}

	mod.protocols = append(mod.protocols, ps)
}

func (e *Emitter) fieldType(typ *engine.ExprValue) string {
	switch typ.Kind {
	case engine.StructKind:
		return "BYTES"
	case engine.EnumKind:
		return "UINT_STRING"
	default:
		vt, ok := typ.ValueType()
		if !ok {
			return "NONE"
		}
		if vt.Type.TypeRef != nil {
			return e.fieldTypeRef(vt.Type.TypeRef, vt.Repeat)
		} else if vt.Type.TypeSwitch != nil {
			// Type-switch fields don't map cleanly to a single ProtoField
			// type - we'd need to emit per-case sub-fields. For now we
			// surface a NONE placeholder so the rest of the dissector can
			// still be generated.
			return "NONE"
		} else {
			panic("invalid type")
		}
	}
}
func (e *Emitter) fieldTypeRef(n *types.TypeRef, r types.RepeatType) string {
	switch n.Kind {
	case types.UntypedInt:
		return "BYTES"
	case types.UntypedFloat:
		return "BYTES"
	case types.UntypedBool:
		return "BYTES"
	case types.U1:
		return "UINT8"
	case types.U2, types.U2le, types.U2be:
		return "UINT16"
	case types.U4, types.U4le, types.U4be:
		return "UINT32"
	case types.U8, types.U8le, types.U8be:
		return "UINT64"
	case types.S1:
		return "INT8"
	case types.S2, types.S2le, types.S2be:
		return "INT16"
	case types.S4, types.S4le, types.S4be:
		return "INT32"
	case types.S8, types.S8le, types.S8be:
		return "INT64"
	case types.Bits:
		// Wireshark ProtoFields support bit-packed integers via UINT8/16/32
		// with a mask, but emitting those requires width-aware allocation
		// across adjacent bit fields. Until that's implemented, fall over
		// loudly so the caller doesn't ship a silently-broken dissector.
		panic("wireshark: bit fields are not yet supported")
	case types.F4, types.F4le, types.F4be:
		return "FLOAT"
	case types.F8, types.F8le, types.F8be:
		return "DOUBLE"
	case types.Bytes:
		return "BYTES"
	case types.String:
		if n.String.Terminator == 0 {
			return "STRINGZ"
		}
		return "STRING"
	case types.User:
		typ := e.resolveType(n.User.Name)
		switch typ.Kind {
		case engine.StructKind:
			return e.prefix(typ.DefParent) + e.typeName(typ.Struct.Type.ID)
		case engine.EnumKind:
			return e.prefix(typ.DefParent) + e.typeName(typ.Enum.ID)
		default:
			panic(fmt.Errorf("expression %q yielded unexpected type %s", n.User.Name, typ.Kind))
		}
	}
	panic("unexpected typekind: " + n.Kind.String())
}

func (e *Emitter) resolveType(ex string) *engine.ExprValue {
	typ := engine.ResolveTypeOfExpr(e.context, expr.MustParseExpr(ex))
	if typ == nil {
		panic(fmt.Errorf("unresolved type: %s", ex))
	}
	return typ
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

func (e *Emitter) prefix(typ *engine.ExprValue) string {
	if typ == nil || typ.Struct == nil {
		return ""
	}
	return e.typeName(typ.Struct.Type.ID) + "_"
}

func (e *Emitter) fieldName(n kaitai.Identifier) string {
	return e.typeName(n)
}

func (e *Emitter) typeName(n kaitai.Identifier) string {
	return strings.ReplaceAll(string(n), "::", "__")
}

func (e *Emitter) filename(n kaitai.Identifier) string {
	return strings.ToLower(string(n)) + ".lua"
}

type module struct {
	protocols []protocol
}

func (m *module) emit(b io.Writer) {
	for _, protocol := range m.protocols {
		protocol.emitdef(b)
	}
}

type protocol struct {
	name   string
	fields []field
}

func (p *protocol) emitdef(w io.Writer) {
	if _, err := fmt.Fprintf(w, "-- region %s protocol definition\n", p.name); err != nil {
		panic(err)
	}
	if _, err := fmt.Fprintf(w, "\n"); err != nil {
		panic(err)
	}
	titleCase := cases.Title(language.English, cases.NoLower)
	if _, err := fmt.Fprintf(w, "local %[1]s_protocol = Proto(\"%[2]s Protocol\")\n", p.name, titleCase.String(p.name)); err != nil {
		panic(err)
	}
	for _, field := range p.fields {
		field.emitdef(w, p.name)
	}
	if _, err := fmt.Fprintf(w, "%[1]s_protocol.fields = {\n", p.name); err != nil {
		panic(err)
	}
	for i, field := range p.fields {
		eol := ","
		if i == len(p.fields)-1 {
			eol = ""
		}
		_, _ = fmt.Fprintf(w, "  %[1]s_%[2]s_field%s\n", p.name, field.name, eol)
	}
	if _, err := fmt.Fprintf(w, "}\n"); err != nil {
		panic(err)
	}
	if _, err := fmt.Fprintf(w, "\n"); err != nil {
		panic(err)
	}
	if _, err := fmt.Fprintf(w, "-- endregion %s protocol definition\n", p.name); err != nil {
		panic(err)
	}
	if _, err := fmt.Fprintf(w, "\n"); err != nil {
		panic(err)
	}
}

type field struct {
	name string
	typ  string
	base string
	desc string
}

func (f *field) emitdef(w io.Writer, prefix string) {
	_, _ = fmt.Fprintf(w, "local %[1]s_%[2]s_field = ProtoField.new(\"%[1]s.%[2]s\", \"%[2]s\", ftypes.%[3]s, nil, base.%[4]s, nil, %[5]q)\n", prefix, f.name, f.typ, f.base, strings.TrimSpace(f.desc))
}
