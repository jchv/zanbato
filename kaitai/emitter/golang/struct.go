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
}

// NewEmitter constructs a new emitter with the given parameters.
func NewEmitter(pkgname string) *Emitter {
	return &Emitter{pkgname}
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
	return strings.ToLower(string(n))
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
	case kaitai.U2le:
		return "uint16"
	case kaitai.U2be:
		return "uint16"
	case kaitai.U4le:
		return "uint32"
	case kaitai.U4be:
		return "uint32"
	case kaitai.U8le:
		return "uint64"
	case kaitai.U8be:
		return "uint64"
	case kaitai.S1:
		return "int8"
	case kaitai.S2le:
		return "int16"
	case kaitai.S2be:
		return "int16"
	case kaitai.S4le:
		return "int32"
	case kaitai.S4be:
		return "int32"
	case kaitai.S8le:
		return "int64"
	case kaitai.S8be:
		return "int64"
	case kaitai.Bits:
		return "uint64"
	case kaitai.F4le:
		return "float32"
	case kaitai.F4be:
		return "float32"
	case kaitai.F8le:
		return "float64"
	case kaitai.F8be:
		return "float64"
	case kaitai.Bytes:
		return "[]byte"
	case kaitai.String:
		return "string"
	case kaitai.User:
		return "interface{}"
	}
	panic("unexpected typekind")
}

func (e *Emitter) readfunc(n kaitai.Type) string {
	switch n.Kind {
	case kaitai.U1:
		return "ReadU1"
	case kaitai.U2le:
		return "ReadU2le"
	case kaitai.U2be:
		return "ReadU2be"
	case kaitai.U4le:
		return "ReadU4le"
	case kaitai.U4be:
		return "ReadU4be"
	case kaitai.U8le:
		return "ReadU8le"
	case kaitai.U8be:
		return "ReadU8be"
	case kaitai.S1:
		return "ReadS1"
	case kaitai.S2le:
		return "ReadS2le"
	case kaitai.S2be:
		return "ReadS2be"
	case kaitai.S4le:
		return "ReadS4le"
	case kaitai.S4be:
		return "ReadS4be"
	case kaitai.S8le:
		return "ReadS8le"
	case kaitai.S8be:
		return "ReadS8be"
	case kaitai.Bits:
		panic("not implemented yet")
	case kaitai.F4le:
		return "ReadF4le"
	case kaitai.F4be:
		return "ReadF4be"
	case kaitai.F8le:
		return "ReadF8le"
	case kaitai.F8be:
		return "ReadF8be"
	case kaitai.Bytes:
		panic("not implemented yet")
	case kaitai.String:
		panic("not implemented yet")
	case kaitai.User:
		panic("not implemented yet")
	}
	panic("unexpected typekind")
}

func (e *Emitter) root(s *kaitai.Struct) emitter.Artifact {
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

func (e *Emitter) readattr(method *gomethod, a *kaitai.Attr) {
	method.tmp++
	readcall := fmt.Sprintf(`io.%s()`, e.readfunc(a.Type))
	fieldname := e.fieldname(a.ID)

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
		this.%s = append(this.%s, tmp%d)
	}
`, method.tmp, e.readfunc(a.Type), fieldname, fieldname, method.tmp)})
	case kaitai.RepeatExpr:
		panic("not implemented")
	case kaitai.RepeatUntil:
		panic("not implemented")
	case nil:
		method.stmt = append(method.stmt, gostatement{source: fmt.Sprintf(`tmp%d, err := %s
	if err != nil {
		return err
	}
	this.%s = tmp%d
`, method.tmp, readcall, fieldname, method.tmp)})
	}
}

func (e *Emitter) struc(unit *gounit, pfx string, s *kaitai.Struct) {
	name := e.typename(s.ID)

	if len(s.Attrs) > 0 {
		g := gostruct{name: pfx + name}
		for _, attr := range s.Attrs {
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
		for _, attr := range s.Attrs {
			e.readattr(&readmethod, attr)
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
