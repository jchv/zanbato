package wireshark

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"path"
	"strings"

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

	root := engine.NewStructValueSymbol(engine.NewStructTypeSymbol(s, nil), nil)
	e.context.AddGlobalType(string(s.ID), root.Type)
	e.context.AddModuleType(string(s.ID), root.Type)
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
	ks := val.Type.Struct.Type

	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Errorf("struct %s: %s", ks.ID, r))
		}
	}()

	name := e.typeName(ks.ID)
	prefix := e.prefix(val.Type.Parent)

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
	for _, n := range val.Type.Struct.Structs {
		e.struc(inputname, mod, engine.NewStructValueSymbol(n, val))
	}

	// Enumerations
	for _, n := range val.Type.Struct.Enums {
		_ = n
		log.Printf("TODO: generate enumerations")
		// e.enum(mod, n)
	}

	// Attribute fields
	for _, attr := range val.Struct.Attrs {
		ps.fields = append(ps.fields, field{
			name: e.fieldName(attr.Type.Attr.ID),
			typ:  e.fieldType(attr.Type),
			base: "DEC",
			desc: attr.Type.Attr.Doc,
		})
	}

	// Deserialization
	if e.endian == types.SwitchEndian || (e.needMultipleEndian(ks) && e.endian == types.UnspecifiedOrder) {
		if e.endian == types.SwitchEndian {
			log.Printf("TODO: generate endian switch")
			// e.endianSwitch(mod, &ps, ks)
		} else {
			log.Printf("TODO: generate undecided endianness")
			// e.strucRead(mod, &ps, val, types.UnspecifiedOrder)
		}
		log.Printf("TODO: generate le/be")
		// e.strucRead(mod, &ps, val, types.LittleEndian)
		// e.strucRead(mod, &ps, val, types.BigEndian)

		for _, attr := range val.Struct.Attrs {
			if attr.Type.Attr.Type.TypeSwitch != nil {
				log.Printf("TODO: generate multi endian type switches")
				// e.typeSwitchStruct(mod, attr.Type)
				// e.typeSwitch(mod, attr, types.LittleEndian)
				// e.typeSwitch(mod, attr, types.BigEndian)
			}
		}
	} else {
		// Struct is always consistent endianness: generate one read function and make two stubs to it.
		log.Printf("TODO: generate single endian read")
		// e.strucRead(mod, &gs, val, types.UnspecifiedOrder)
		// e.endianStubs(mod, &gs, ks)

		for _, attr := range val.Struct.Attrs {
			if attr.Type.Attr.Type.TypeSwitch != nil {
				log.Printf("TODO: generate single endian type switches")
				//e.typeSwitchStruct(mod, attr.Type)
				//e.typeSwitch(mod, attr, types.UnspecifiedOrder)
			}
		}
	}

	mod.protocols = append(mod.protocols, ps)
}

func (e *Emitter) fieldType(typ *engine.ExprType) string {
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
			log.Printf("TODO: type switch decl?")
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
		panic("TODO: bitfields")
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
			return e.prefix(typ.Parent) + e.typeName(typ.Struct.Type.ID)
		case engine.EnumKind:
			return e.prefix(typ.Parent) + e.typeName(typ.Enum.ID)
		default:
			panic(fmt.Errorf("expression %q yielded unexpected type %s", n.User.Name, typ.Kind))
		}
	}
	panic("unexpected typekind: " + n.Kind.String())
}

func (e *Emitter) resolveType(ex string) *engine.ExprType {
	typ := engine.ResultTypeOfExpr(e.context, expr.MustParseExpr(ex)).Type()
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

func (e *Emitter) prefix(typ *engine.ExprType) string {
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
	fmt.Fprintf(w, "-- region %s protocol definition\n", p.name)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "local %[1]s_protocol = Proto(\"%[2]s Protocol\")\n", p.name, strings.Title(p.name))
	for _, field := range p.fields {
		field.emitdef(w, p.name)
	}
	fmt.Fprintf(w, "%[1]s_protocol.fields = {\n", p.name)
	for i, field := range p.fields {
		eol := ","
		if i == len(p.fields)-1 {
			eol = ""
		}
		fmt.Fprintf(w, "  %[1]s_%[2]s_field%s\n", p.name, field.name, eol)
	}
	fmt.Fprintf(w, "}\n")
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "-- endregion %s protocol definition\n", p.name)
	fmt.Fprintf(w, "\n")
}

type field struct {
	name string
	typ  string
	base string
	desc string
}

func (f *field) emitdef(w io.Writer, prefix string) {
	fmt.Fprintf(w, "local %[1]s_%[2]s_field = ProtoField.new(\"%[1]s.%[2]s\", \"%[2]s\", ftypes.%[3]s, nil, base.%[4]s, nil, %[5]q)\n", prefix, f.name, f.typ, f.base, strings.TrimSpace(f.desc))
}
