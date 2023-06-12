package golang

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/emitter"
)

const (
	kaitaiRuntimePackagePath = "github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	kaitaiRuntimePackageName = "kaitai"
	kaitaiStream             = kaitaiRuntimePackageName + ".Stream"
	kaitaiWriter             = kaitaiRuntimePackageName + ".Writer"
)

// Emitter emits Go code for kaitai structs.
type Emitter struct {
	pkgname string
	endian  kaitai.Endianness
}

// NewEmitter constructs a new emitter with the given parameters.
func NewEmitter(pkgname string) *Emitter {
	return &Emitter{pkgname: pkgname}
}

// Emit emits Go code for the given kaitai struct.
func (e *Emitter) Emit(s *kaitai.Struct) []emitter.Artifact {
	return []emitter.Artifact{e.root(s)}
}

func (e *Emitter) filename(n kaitai.Identifier) string {
	return strings.ToLower(string(n)) + ".go"
}

func (e *Emitter) typename(n kaitai.Identifier) string {
	return strings.ReplaceAll(strings.Title(strings.ReplaceAll(string(n), "_", " ")), " ", "")
}

func (e *Emitter) fieldname(n kaitai.Identifier) string {
	return strings.ReplaceAll(strings.Title(strings.ReplaceAll(string(n), "_", " ")), " ", "")
}

func (e *Emitter) setimport(unit *gounit, pkg string, as string) {
	unit.imports[pkg] = as
}

func (e *Emitter) decltype(n kaitai.Type, r kaitai.RepeatType) string {
	if r != nil {
		return "[]" + e.decltype(n, nil)
	}
	switch n.Kind {
	case kaitai.U1:
		return "uint8"
	case kaitai.U2, kaitai.U2le, kaitai.U2be:
		return "uint16"
	case kaitai.U4, kaitai.U4le, kaitai.U4be:
		return "uint32"
	case kaitai.U8, kaitai.U8le, kaitai.U8be:
		return "uint64"
	case kaitai.S1:
		return "int8"
	case kaitai.S2, kaitai.S2le, kaitai.S2be:
		return "int16"
	case kaitai.S4, kaitai.S4le, kaitai.S4be:
		return "int32"
	case kaitai.S8, kaitai.S8le, kaitai.S8be:
		return "int64"
	case kaitai.Bits:
		return "uint64"
	case kaitai.F4, kaitai.F4le, kaitai.F4be:
		return "float32"
	case kaitai.F8, kaitai.F8le, kaitai.F8be:
		return "float64"
	case kaitai.Bytes:
		return "[]byte"
	case kaitai.String:
		return "string"
	case kaitai.User:
		return "interface{}"
	}
	panic("unexpected typekind: " + n.Kind.String())
}

func (e *Emitter) readcall(n kaitai.Type) string {
	switch n.Kind {
	case kaitai.U2, kaitai.U4, kaitai.U8,
		kaitai.S2, kaitai.S4, kaitai.S8,
		kaitai.F4, kaitai.F8:
		panic("undecided endianness")
	case kaitai.U1:
		return "ReadU1()"
	case kaitai.U2le:
		return "ReadU2le()"
	case kaitai.U2be:
		return "ReadU2be()"
	case kaitai.U4le:
		return "ReadU4le()"
	case kaitai.U4be:
		return "ReadU4be()"
	case kaitai.U8le:
		return "ReadU8le()"
	case kaitai.U8be:
		return "ReadU8be()"
	case kaitai.S1:
		return "ReadS1()"
	case kaitai.S2le:
		return "ReadS2le()"
	case kaitai.S2be:
		return "ReadS2be()"
	case kaitai.S4le:
		return "ReadS4le()"
	case kaitai.S4be:
		return "ReadS4be()"
	case kaitai.S8le:
		return "ReadS8le()"
	case kaitai.S8be:
		return "ReadS8be()"
	case kaitai.Bits:
		panic("not implemented yet: bits")
	case kaitai.F4le:
		return "ReadF4le()"
	case kaitai.F4be:
		return "ReadF4be()"
	case kaitai.F8le:
		return "ReadF8le()"
	case kaitai.F8be:
		return "ReadF8be()"
	case kaitai.Bytes:
		if n.Bytes.Size != nil {
			exprStr, err := e.expr(n.Bytes.Size)
			if err != nil {
				panic(err)
			}
			return fmt.Sprintf("ReadBytes(%s)", exprStr)
		}
		panic("not implemented yet: bytes")
	case kaitai.String:
		if n.String.SizeEOS {
			return fmt.Sprintf("ReadStrEOS(%q)", n.String.Encoding)
		}
		if n.String.Size != nil {
			sizeStr, err := e.expr(n.String.Size)
			if err != nil {
				panic(err)
			}
			if n.String.Terminator == -1 {
				return fmt.Sprintf("ReadBytes(%s)", sizeStr)
			} else {
				return fmt.Sprintf("ReadBytesPadTerm(%s, %q, %q, %v)", sizeStr, n.String.Terminator, n.String.Terminator, n.String.Include)
			}
		} else {
			if n.String.Terminator == -1 {
				panic("undecidable condition")
			}
			return fmt.Sprintf("ReadBytesPadTerm(%q, %v, %v, %v)", rune(n.String.Terminator), n.String.Include, n.String.Consume, n.String.EosError)
		}
	case kaitai.User:
		panic("not implemented yet: usertype")
	}
	panic("unexpected typekind: " + n.Kind.String())
}

func (e *Emitter) expr(expr *kaitai.Expr) (string, error) {
	return e.exprNode(expr.Root)
}

func (e *Emitter) exprNode(node kaitai.Node) (string, error) {
	switch t := node.(type) {
	case kaitai.IdentNode:
		// TODO: not really
		return "this." + e.fieldname(kaitai.Identifier(t.Value)), nil
	case kaitai.IntNode:
		return t.Value.String(), nil
	default:
		return "", fmt.Errorf("unsupported expression node %T", t)
	}
}

func (e *Emitter) root(s *kaitai.Struct) emitter.Artifact {
	if s.Meta.Endian != kaitai.UnspecifiedOrder {
		// TODO: endian switching
		e.endian = s.Meta.Endian
	}

	unit := gounit{pkgname: e.pkgname, imports: map[string]string{}}
	e.struc(&unit, "", s)

	out := bytes.Buffer{}
	unit.emit(&out)

	return emitter.Artifact{
		Filename: e.filename(s.ID),
		Body:     out.Bytes(),
	}
}

func (e *Emitter) enum(unit *gounit, pfx string, k *kaitai.Enum) {
	name := pfx + e.typename(k.ID)
	g := goenum{name: name, decltype: "int"}
	for _, v := range k.Values {
		g.values = append(g.values, goenumvalue{name: name + "__" + e.typename(v.ID), value: v.Value})
	}
	unit.enums = append(unit.enums, g)
}

func (e *Emitter) readattr(unit *gounit, method *gomethod, a *kaitai.Attr) bool {
	method.tmp++
	typ := a.Type.FoldEndian(e.endian)

	switch typ.Kind {
	case kaitai.U2, kaitai.U4, kaitai.U8,
		kaitai.S2, kaitai.S4, kaitai.S8,
		kaitai.F4, kaitai.F8:
		method.stmt = append(method.stmt, gostatement{
			source: "return io.UndecidedEndiannessError",
		})
		return false
	}

	readcall := fmt.Sprintf(`io.%s`, e.readcall(typ))
	fieldname := e.fieldname(a.ID)

	cast := ""
	if a.Type.Kind == kaitai.String {
		cast = "string"
	}

	switch a.Repeat.(type) {
	case kaitai.RepeatEOS:
		method.stmt = append(method.stmt, gostatement{source: fmt.Sprintf(`for {
		if eof, err := io.EOF(); err != nil {
			return err
		} else eof {
			break
		}
		tmp%d, err := %s
		if err != nil {
			return err
		}
		this.%s = append(this.%s, %s(tmp%d))
	}
`, method.tmp, readcall, fieldname, fieldname, cast, method.tmp)})
	case kaitai.RepeatExpr:
		panic("not implemented: repeat expr")
	case kaitai.RepeatUntil:
		panic("not implemented: repeat until")
	case nil:
		method.stmt = append(method.stmt, gostatement{source: fmt.Sprintf(`tmp%d, err := %s
	if err != nil {
		return err
	}
	this.%s = %s(tmp%d)
`, method.tmp, readcall, fieldname, cast, method.tmp)})
	}

	if a.Contents != nil {
		e.setimport(unit, "bytes", "bytes")
		method.stmt = append(method.stmt, gostatement{source: fmt.Sprintf(`if !bytes.Equal(tmp%d, %#v) {
		return io.NewValidationNotEqualError(%#v, tmp%d, io, "") // TODO: set srcPath
	}
`, method.tmp, a.Contents, a.Contents, method.tmp)})
	}
	return true
}

func (e *Emitter) struc(unit *gounit, pfx string, s *kaitai.Struct) {
	name := e.typename(s.ID)

	if len(s.Seq) > 0 {
		g := gostruct{name: pfx + name}
		for _, attr := range s.Seq {
			g.fields = append(g.fields, govar{name: e.fieldname(attr.ID), typ: e.decltype(attr.Type, attr.Repeat)})
		}
		unit.structs = append(unit.structs, g)

		e.setimport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
		readmethod := gomethod{
			recv: govar{name: "this", typ: "*" + g.name},
			name: "Read",
			in:   []govar{{name: "io", typ: "*" + kaitaiStream}},
			out:  []govar{{name: "err", typ: "error"}},
			stmt: []gostatement{},
		}
		for _, attr := range s.Seq {
			if !e.readattr(unit, &readmethod, attr) {
				// We may need to end the function early in some cases.
				break
			}
		}
		unit.methods = append(unit.methods, readmethod)
	}

	for _, n := range s.Structs {
		e.struc(unit, name+"_", n)
	}

	for _, n := range s.Enums {
		e.enum(unit, name+"_", n)
	}
}

type govar struct {
	name string
	typ  string
}

func (v govar) String() string {
	return fmt.Sprintf("%s %s", v.name, v.typ)
}

type govarlist []govar

func (v govarlist) String() string {
	vstrs := []string{}
	for _, n := range v {
		vstrs = append(vstrs, n.String())
	}
	return strings.Join(vstrs, ", ")
}

type gofields []govar

func (v gofields) String() string {
	vstrs := []string{}
	for _, n := range v {
		vstrs = append(vstrs, n.String())
	}
	return strings.Join(vstrs, "\n\t")
}

type gostruct struct {
	name   string
	fields gofields
}

type gostatement struct {
	source string
}

type gomethod struct {
	recv govar
	name string
	tmp  int
	in   govarlist
	out  govarlist
	stmt []gostatement
}

type goenumvalue struct {
	name  string
	value int
}

type goenum struct {
	name     string
	decltype string
	values   []goenumvalue
}

type gounit struct {
	pkgname string
	imports map[string]string
	enums   []goenum
	structs []gostruct
	methods []gomethod
}

func (g *gostruct) emit(buf io.Writer) {
	fmt.Fprintf(buf, "type %s struct {\n\t%s\n}\n\n", g.name, g.fields)
}

func (g *gomethod) emit(buf io.Writer) {
	fmt.Fprintf(buf, "func (%s) %s(%s) (%s) {\n", g.recv.String(), g.name, g.in.String(), g.out.String())
	for _, s := range g.stmt {
		fmt.Fprintf(buf, "\t%s", s.source)
	}
	fmt.Fprintf(buf, "}\n\n")
}

func (g *goenum) emit(buf io.Writer) {
	fmt.Fprintf(buf, "type %s %s\n", g.name, g.decltype)
	fmt.Fprintf(buf, "const (\n")
	for _, v := range g.values {
		fmt.Fprintf(buf, "\t%s %s = %d\n", v.name, g.name, v.value)
	}
	fmt.Fprintf(buf, ")\n\n")
}

func (g *gounit) emit(buf io.Writer) {
	fmt.Fprint(buf, "// Generated by Zanbato. Do not edit!\n\n")
	fmt.Fprintf(buf, "package %s\n\n", g.pkgname)
	if len(g.imports) > 0 {
		var keys []string
		for k := range g.imports {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintf(buf, "import (\n")
		for _, k := range keys {
			fmt.Fprintf(buf, "\t%s %q\n", g.imports[k], k)
		}
		fmt.Fprintf(buf, ")\n\n")
	}
	for _, e := range g.enums {
		e.emit(buf)
	}
	for _, s := range g.structs {
		s.emit(buf)
	}
	for _, m := range g.methods {
		m.emit(buf)
	}
}
