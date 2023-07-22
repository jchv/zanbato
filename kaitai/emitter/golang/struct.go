package golang

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math/big"
	"path"
	"sort"
	"strings"
	"unicode"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/emitter"
)

const (
	kaitaiRuntimePackagePath = "github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
	kaitaiRuntimePackageName = "kaitai"
	kaitaiStream             = kaitaiRuntimePackageName + ".Stream"
	kaitaiWriter             = kaitaiRuntimePackageName + ".Writer"
)

type ResolveFunc func(from, to string) (string, *kaitai.Struct)

// Emitter emits Go code for kaitai structs.
type Emitter struct {
	pkgname  string
	pkgpath  string
	resolver ResolveFunc
	endian   kaitai.Endianness

	r       *kaitai.Struct
	stack   []*kaitai.Struct
	imports *kaitai.Struct

	artifacts []emitter.Artifact
}

// NewEmitter constructs a new emitter with the given parameters.
func NewEmitter(pkgpath string, resolver ResolveFunc) *Emitter {
	return &Emitter{
		pkgname:  path.Base(pkgpath),
		pkgpath:  pkgpath,
		resolver: resolver,
	}
}

// Emit emits Go code for the given kaitai struct.
func (e *Emitter) Emit(inputname string, s *kaitai.Struct) []emitter.Artifact {
	e.root(inputname, s)
	return e.artifacts
}

func (e *Emitter) filename(n kaitai.Identifier) string {
	return strings.ToLower(string(n)) + ".go"
}

func (e *Emitter) typeName(n kaitai.Identifier) string {
	return strings.ReplaceAll(strings.ReplaceAll(titleCase(strings.ReplaceAll(string(n), "_", " ")), " ", ""), "::", "__")
}

func (e *Emitter) typeSwitchName(n kaitai.Identifier) string {
	return e.typeName(n) + "_Cases"
}

func (e *Emitter) typeSwitchCaseTypeName(attr *kaitai.Attr, value string) string {
	typeSwitchName := e.localPrefix() + e.typeSwitchName(attr.Type.TypeSwitch.FieldName)
	return typeSwitchName + "_" + value
}

func (e *Emitter) fieldName(n kaitai.Identifier) string {
	return e.typeName(n)
}

func (e *Emitter) setImport(unit *goUnit, pkg string, as string) {
	unit.imports[pkg] = as
}

func (e *Emitter) resolveLocalStruct(id string) (*kaitai.Struct, *kaitai.Struct) {
	// Resolve from parent scope
	structParent, localStruct := e.parent().ResolveStruct(id)
	if localStruct != nil {
		return structParent, localStruct
	}

	// Resolve from root scope
	structParent, localStruct = e.r.ResolveStruct(id)
	if localStruct != nil {
		return structParent, localStruct
	}

	// Resolve from imports
	structParent, localStruct = e.imports.ResolveStruct(id)
	if localStruct != nil {
		return structParent, localStruct
	}

	return nil, nil
}

func (e *Emitter) resolveLocalEnum(id string) (*kaitai.Struct, *kaitai.Enum) {
	// Resolve from parent scope
	enumParent, localEnum := e.parent().ResolveEnum(id)
	if localEnum != nil {
		return enumParent, localEnum
	}

	// Resolve from root scope
	enumParent, localEnum = e.r.ResolveEnum(id)
	if localEnum != nil {
		return enumParent, localEnum
	}

	// Resolve from imports
	enumParent, localEnum = e.imports.ResolveEnum(id)
	if localEnum != nil {
		return enumParent, localEnum
	}

	return nil, nil
}

func (e *Emitter) resolveLocalEnumValue(id string) (*kaitai.Struct, *kaitai.Enum, kaitai.Identifier) {
	i := strings.LastIndex(id, "::")
	if i < 0 {
		return nil, nil, ""
	}
	parent, enum := e.resolveLocalEnum(id[:i])
	return parent, enum, kaitai.Identifier(id[i+2:])
}

func (e *Emitter) declTypeRef(n *kaitai.TypeRef, r kaitai.RepeatType) string {
	if r != nil {
		return "[]" + e.declTypeRef(n, nil)
	}
	switch n.Kind {
	case kaitai.UntypedNum:
		return ""
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
		enumParent, localEnum := e.resolveLocalStruct(n.User.Name)
		structParent, localStruct := e.resolveLocalEnum(n.User.Name)
		if localEnum != nil {
			return e.prefix(enumParent) + e.typeName(kaitai.Identifier(n.User.Name))
		} else if localStruct != nil {
			return e.prefix(structParent) + e.typeName(kaitai.Identifier(n.User.Name))
		} else {
			return e.typeName(kaitai.Identifier(n.User.Name))
		}
	}
	panic("unexpected typekind: " + n.Kind.String())
}

func (e *Emitter) declTypeSwitch(n *kaitai.TypeSwitch, r kaitai.RepeatType) string {
	if r != nil {
		return "[]" + e.declTypeSwitch(n, nil)
	}
	return e.localPrefix() + e.typeSwitchName(n.FieldName)
}

func (e *Emitter) declType(n kaitai.Type, r kaitai.RepeatType) string {
	if n.TypeRef != nil {
		return e.declTypeRef(n.TypeRef, r)
	} else if n.TypeSwitch != nil {
		return e.declTypeSwitch(n.TypeSwitch, r)
	} else {
		panic("invalid type")
	}
}

func (e *Emitter) readCallRef(n *kaitai.TypeRef) string {
	switch n.Kind {
	case kaitai.UntypedNum:
		panic("untyped number")
	case kaitai.U2, kaitai.U4, kaitai.U8,
		kaitai.S2, kaitai.S4, kaitai.S8,
		kaitai.F4, kaitai.F8:
		panic("undecided endianness")
	case kaitai.U1:
		return "io.ReadU1()"
	case kaitai.U2le:
		return "io.ReadU2le()"
	case kaitai.U2be:
		return "io.ReadU2be()"
	case kaitai.U4le:
		return "io.ReadU4le()"
	case kaitai.U4be:
		return "io.ReadU4be()"
	case kaitai.U8le:
		return "io.ReadU8le()"
	case kaitai.U8be:
		return "io.ReadU8be()"
	case kaitai.S1:
		return "io.ReadS1()"
	case kaitai.S2le:
		return "io.ReadS2le()"
	case kaitai.S2be:
		return "io.ReadS2be()"
	case kaitai.S4le:
		return "io.ReadS4le()"
	case kaitai.S4be:
		return "io.ReadS4be()"
	case kaitai.S8le:
		return "io.ReadS8le()"
	case kaitai.S8be:
		return "io.ReadS8be()"
	case kaitai.Bits:
		panic("not implemented yet: bits")
	case kaitai.F4le:
		return "io.ReadF4le()"
	case kaitai.F4be:
		return "io.ReadF4be()"
	case kaitai.F8le:
		return "io.ReadF8le()"
	case kaitai.F8be:
		return "io.ReadF8be()"
	case kaitai.Bytes:
		if n.Bytes.Size != nil {
			return fmt.Sprintf("io.ReadBytes(int(%s))", e.expr(n.Bytes.Size))
		}
		if n.Bytes.SizeEOS {
			return "io.ReadBytesFull()"
		}
		panic("not implemented yet: bytes")
	case kaitai.String:
		if n.String.SizeEOS {
			return fmt.Sprintf("io.ReadStrEOS(%q)", n.String.Encoding)
		}
		if n.String.Size != nil {
			if n.String.Terminator == -1 {
				return fmt.Sprintf("io.ReadBytes(int(%s))", e.expr(n.String.Size))
			} else {
				return fmt.Sprintf("io.ReadBytesPadTerm(%s, %q, %q, %v)", e.expr(n.String.Size), n.String.Terminator, n.String.Terminator, n.String.Include)
			}
		} else {
			if n.String.Terminator == -1 {
				panic("undecidable condition")
			}
			return fmt.Sprintf("io.ReadBytesPadTerm(%q, %v, %v, %v)", rune(n.String.Terminator), n.String.Include, n.String.Consume, n.String.EosError)
		}
	case kaitai.User:
		panic("called readCallRef on user type!")
	}
	panic("unexpected typekind: " + n.Kind.String())
}

func (e *Emitter) expr(expr *kaitai.Expr) string {
	return e.exprNode(expr.Root)
}

func (e *Emitter) exprNode(node kaitai.Node) string {
	switch t := node.(type) {
	case kaitai.IdentNode:
		// TODO: not really
		return "this." + e.fieldName(kaitai.Identifier(t.Value))
	case kaitai.IntNode:
		return t.Value.String()
	default:
		panic(fmt.Errorf("unsupported expression node %T", t))
	}
}

func (e *Emitter) root(inputname string, s *kaitai.Struct) {
	if s.Meta.Endian != kaitai.UnspecifiedOrder {
		// TODO: endian switching
		e.endian = s.Meta.Endian
	}

	unit := goUnit{
		pkgname: e.pkgname,
		imports: map[string]string{},
	}

	// Pivot stack to new root
	oldRoot, oldStack, oldImports := e.r, e.stack, e.imports
	e.r, e.stack, e.imports = s, []*kaitai.Struct{}, &kaitai.Struct{}

	e.struc(inputname, &unit, s)

	// Pivot back to old root
	e.r, e.stack, e.imports = oldRoot, oldStack, oldImports

	out := bytes.Buffer{}
	unit.emit(&out)

	e.artifacts = append(e.artifacts, emitter.Artifact{
		Filename: e.filename(s.ID),
		Body:     out.Bytes(),
	})
}

func (e *Emitter) push(s *kaitai.Struct) {
	e.stack = append(e.stack, s)
}

func (e *Emitter) pop() {
	e.stack[len(e.stack)-1] = nil
	e.stack = e.stack[:len(e.stack)-1]
}

func (e *Emitter) parent() *kaitai.Struct {
	if len(e.stack) < 1 {
		return nil
	}
	return e.stack[len(e.stack)-1]
}

func (e *Emitter) grandparent() *kaitai.Struct {
	if len(e.stack) < 2 {
		return nil
	}
	return e.stack[len(e.stack)-2]
}

func (e *Emitter) enumTypeName(parent *kaitai.Struct, enum *kaitai.Enum) string {
	return e.prefix(parent) + e.typeName(enum.ID)
}

func (e *Emitter) enumValueName(parent *kaitai.Struct, enum *kaitai.Enum, id kaitai.Identifier) string {
	return e.prefix(parent) + e.typeName(enum.ID) + "__" + e.typeName(id)
}

func (e *Emitter) enum(unit *goUnit, enum *kaitai.Enum) {
	g := goEnum{name: e.enumTypeName(e.parent(), enum), decltype: "int"}
	for _, v := range enum.Values {
		g.values = append(g.values, goEnumValue{name: e.enumValueName(e.parent(), enum, v.ID), value: v.Value})
	}
	unit.enums = append(unit.enums, g)
}

func (e *Emitter) isValidEndianTypeRef(t *kaitai.TypeRef) bool {
	switch t.Kind {
	case kaitai.U2, kaitai.U4, kaitai.U8,
		kaitai.S2, kaitai.S4, kaitai.S8,
		kaitai.F4, kaitai.F8:
		return false
	default:
		return true
	}
}

func (e *Emitter) isValidEndianTypeSwitch(t *kaitai.TypeSwitch) bool {
	for _, value := range t.Cases {
		if !e.isValidEndianTypeRef(&value) {
			return false
		}
	}
	return true
}

func (e *Emitter) isValidEndianType(t kaitai.Type) bool {
	if t.TypeRef != nil {
		return e.isValidEndianTypeRef(t.TypeRef)
	} else if t.TypeSwitch != nil {
		return e.isValidEndianTypeSwitch(t.TypeSwitch)
	} else {
		panic("invalid type")
	}
}

func (e *Emitter) setParams(pfx, name string, tr *kaitai.UserType, resolved *kaitai.Struct) string {
	setParams := ""
	for i := range tr.Params {
		setParams += "\n" + pfx + name + "." + e.typeName(resolved.Params[i].ID) + " = " + e.expr(tr.Params[i])
	}
	return setParams
}

func (e *Emitter) readAttr(unit *goUnit, method *goFunc, a *kaitai.Attr) bool {
	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Errorf("attr: %s: %s", a.ID, r))
		}
	}()

	typ := a.Type.FoldEndian(e.endian)

	if !e.isValidEndianType(typ) {
		method.stmt = append(method.stmt, goStatement{
			source: "return io.UndecidedEndiannessError",
		})
		return false
	}

	method.tmp++

	fieldname := e.fieldName(a.ID)

	if typ.TypeSwitch != nil {
		// Call typeswitch helper
		method.stmt = append(method.stmt, goStatement{source: fmt.Sprintf(`if err := this.read%s(io); err != nil {
		return err
	}
`, e.localPrefix()+e.typeSwitchName(typ.TypeSwitch.FieldName))})
		return true
	}

	switch typ.TypeRef.Kind {
	case kaitai.User:
		// Call read method
		setParams := ""
		_, resolved := e.resolveLocalStruct(typ.TypeRef.User.Name)
		if len(typ.TypeRef.User.Params) > 0 {
			if resolved == nil {
				panic(fmt.Errorf("unresolved type: %s", typ.TypeRef.User.Name))
			}
			setParams = e.setParams("\t", fmt.Sprintf("tmp%d", method.tmp), typ.TypeRef.User, resolved)
		} else {
			if resolved == nil {
				log.Printf("WARNING: unresolved type %s in %s.%s; missing import?", typ.TypeRef.User.Name, e.parent().ID, a.ID)
			}
		}
		switch repeat := a.Repeat.(type) {
		case kaitai.RepeatEOS:
			method.stmt = append(method.stmt, goStatement{source: fmt.Sprintf(`for {
		if eof, err := io.EOF(); err != nil {
			return err
		} else eof {
			break
		}
		tmp%d := %s{}%s
		if err := tmp%d.Read(io); err != nil {
			return err
		}
		this.%s = append(this.%s, tmp%d)
	}
`, method.tmp, e.declTypeRef(typ.TypeRef, nil), setParams, method.tmp, fieldname, fieldname, method.tmp)})
		case kaitai.RepeatExpr:
			iterType, ok := repeat.CountExpr.Type(e.parent())
			iterCast := ""
			if ok {
				// todo: type resolution?
				iterCast = e.declType(iterType, nil)
			}
			method.stmt = append(method.stmt, goStatement{source: fmt.Sprintf(`for i := %s(0); i < %s; i++ {
		tmp%d := %s{}%s
		if err := tmp%d.Read(io); err != nil {
			return err
		}
		this.%s = append(this.%s, tmp%d)
	}
`, iterCast, e.expr(repeat.CountExpr), method.tmp, e.declTypeRef(typ.TypeRef, nil), setParams, method.tmp, fieldname, fieldname, method.tmp)})
		case kaitai.RepeatUntil:
			panic("not implemented: repeat until")
		case nil:
			method.stmt = append(method.stmt, goStatement{source: fmt.Sprintf(`
	tmp%d := %s{}%s
	if err := tmp%d.Read(io); err != nil {
		return err
	}
	this.%s = tmp%d
`, method.tmp, e.declTypeRef(typ.TypeRef, nil), setParams, method.tmp, fieldname, method.tmp)})
			method.stmt = append(method.stmt, goStatement{source: fmt.Sprintf(`if err := this.%s.Read(io); err != nil {
		return err
	}
`, fieldname)})
		}
		return true
	default:
		// Assign values directly
		readcall := e.readCallRef(typ.TypeRef)

		cast := ""
		if a.Type.TypeRef != nil && a.Type.TypeRef.Kind == kaitai.String {
			cast = "string"
		}

		switch repeat := a.Repeat.(type) {
		case kaitai.RepeatEOS:
			method.stmt = append(method.stmt, goStatement{source: fmt.Sprintf(`for {
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
			iterType, ok := repeat.CountExpr.Type(e.parent())
			iterCast := ""
			if ok {
				// todo: type resolution?
				iterCast = e.declType(iterType, nil)
			}
			method.stmt = append(method.stmt, goStatement{source: fmt.Sprintf(`for i := %s(0); i < %s; i++ {
		tmp%d, err := %s
		if err != nil {
			return err
		}
		this.%s = append(this.%s, %s(tmp%d))
	}
`, iterCast, e.expr(repeat.CountExpr), method.tmp, readcall, fieldname, fieldname, cast, method.tmp)})
		case kaitai.RepeatUntil:
			panic("not implemented: repeat until")
		case nil:
			method.stmt = append(method.stmt, goStatement{source: fmt.Sprintf(`tmp%d, err := %s
	if err != nil {
		return err
	}
	this.%s = %s(tmp%d)
`, method.tmp, readcall, fieldname, cast, method.tmp)})
		}

		if a.Contents != nil {
			e.setImport(unit, "bytes", "bytes")
			e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
			method.stmt = append(method.stmt, goStatement{source: fmt.Sprintf(`if !bytes.Equal(tmp%d, %#v) {
		return kaitai.NewValidationNotEqualError(%#v, tmp%d, io, "") // TODO: set srcPath
	}
`, method.tmp, a.Contents, a.Contents, method.tmp)})
		}
		return true
	}
}

func (e *Emitter) typeSwitchCaseValue(value string) string {
	i := big.NewInt(0)
	numeric, ok := i.SetString(value, 0)
	if !ok {
		// TODO: type resolution? current prefix is wrong
		parent, enum, id := e.resolveLocalEnumValue(value)
		if enum == nil {
			log.Fatalf("couldn't resolve %s in %+v", value, e.parent())
		}
		return e.enumValueName(parent, enum, id)
	}
	return numeric.String()
}

func (e *Emitter) typeSwitch(unit *goUnit, attr *kaitai.Attr) {
	ts := attr.Type.TypeSwitch
	typeSwitchName := e.localPrefix() + e.typeSwitchName(ts.FieldName)
	unit.interfaces = append(unit.interfaces, goInterface{
		name: typeSwitchName,
		methods: goMethods{
			goMethod{
				name: "is" + typeSwitchName,
			},
		},
	})
	readSwitchMethod := goFunc{
		recv: goVar{name: "this", typ: "*" + e.parentPrefix() + e.typeName(e.parent().ID)},
		name: "read" + typeSwitchName,
		in:   []goVar{{name: "io", typ: "*" + kaitaiStream}},
		out:  []goVar{{name: "err", typ: "error"}},
		stmt: []goStatement{},
	}
	readSwitchMethod.stmt = append(readSwitchMethod.stmt, goStatement{
		source: "switch " + e.expr(ts.SwitchOn) + " {\n",
	})
	for value, typ := range ts.Cases {
		declTyp, ok := ts.SwitchOn.Type(e.parent())
		typeCast := ""
		if ok {
			typeCast = e.declType(declTyp, nil)
		}
		goValue := e.typeSwitchCaseValue(value)
		goUnderlyingType := e.declType(kaitai.Type{TypeRef: &typ}, nil)
		caseStruct := e.typeSwitchCaseTypeName(attr, goValue)
		if typ.Kind == kaitai.User {
			readSwitchMethod.tmp++
			setParams := ""
			if len(typ.User.Params) > 0 {
				_, resolved := e.resolveLocalStruct(typ.User.Name)
				if resolved == nil {
					panic(fmt.Errorf("unresolved type: %s", typ.User.Name))
				}
				setParams = e.setParams("\t\t", fmt.Sprintf("tmp%d", readSwitchMethod.tmp), typ.User, resolved)
			}
			readSwitchMethod.stmt = append(readSwitchMethod.stmt, goStatement{
				source: fmt.Sprintf("case %s(%s):\n\t\ttmp%d := %s{}\n\t\ttmp%d.Read(io)%s\n\t\tthis.%s = %s{Value: tmp%d}\n",
					typeCast,
					goValue,
					readSwitchMethod.tmp, goUnderlyingType,
					readSwitchMethod.tmp, setParams,
					e.fieldName(attr.ID), caseStruct, readSwitchMethod.tmp),
			})
		} else {
			readSwitchMethod.stmt = append(readSwitchMethod.stmt, goStatement{
				source: fmt.Sprintf("case %s(%s):\n\t\tthis.%s = %s{Value: %s}\n", typeCast, goValue, e.fieldName(attr.ID), caseStruct, e.readCallRef(&typ)),
			})
		}
		unit.structs = append(unit.structs, goStruct{
			name: caseStruct,
			fields: []goVar{
				{
					name: "Value",
					typ:  goUnderlyingType,
				},
			},
		})
		unit.methods = append(unit.methods,
			goFunc{
				recv: goVar{name: "", typ: caseStruct},
				name: "is" + typeSwitchName,
			},
		)
	}
	readSwitchMethod.stmt = append(readSwitchMethod.stmt, goStatement{
		source: "}\n",
	})
	readSwitchMethod.stmt = append(readSwitchMethod.stmt, goStatement{
		source: "return nil\n",
	})
	unit.methods = append(unit.methods, readSwitchMethod)
}

func (e *Emitter) prefix(parent *kaitai.Struct) string {
	if parent == nil || parent.ID == "" {
		return ""
	}
	return e.typeName(parent.ID) + "_"
}

func (e *Emitter) localPrefix() string {
	return e.prefix(e.parent())
}

func (e *Emitter) parentPrefix() string {
	return e.prefix(e.grandparent())
}

func (e *Emitter) struc(inputname string, unit *goUnit, s *kaitai.Struct) {
	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Errorf("struct %s: %s", s.ID, r))
		}
	}()

	name := e.typeName(s.ID)
	prefix := e.localPrefix()

	g := goStruct{name: prefix + name}

	e.push(s)
	defer e.pop()

	for _, param := range s.Params {
		g.fields = append(g.fields, goVar{
			name: e.fieldName(param.ID),
			typ:  e.declTypeRef(&param.Type, nil),
		})
	}
	for _, attr := range s.Seq {
		g.fields = append(g.fields, goVar{
			name: e.fieldName(attr.ID),
			typ:  e.declType(attr.Type, attr.Repeat),
		})
	}
	unit.structs = append(unit.structs, g)

	// Do sub-structures first.
	for _, n := range s.Meta.Imports {
		inputname, s := e.resolver(inputname, n)
		e.imports.Structs = append(e.imports.Structs, s)
		e.root(inputname, s)
	}
	for _, n := range s.Structs {
		e.struc(inputname, unit, n)
	}
	for _, n := range s.Enums {
		e.enum(unit, n)
	}
	e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
	readMethod := goFunc{
		recv: goVar{name: "this", typ: "*" + g.name},
		name: "Read",
		in:   []goVar{{name: "io", typ: "*" + kaitaiStream}},
		out:  []goVar{{name: "err", typ: "error"}},
		stmt: []goStatement{},
	}
	for _, attr := range s.Seq {
		if !e.readAttr(unit, &readMethod, attr) {
			// We may need to end the function early in some cases.
			break
		}
		if attr.Type.TypeSwitch != nil {
			e.typeSwitch(unit, attr)
		}
	}
	readMethod.stmt = append(readMethod.stmt, goStatement{source: "return nil\n"})
	unit.methods = append(unit.methods, readMethod)
}

type goVar struct {
	name string
	typ  string
}

func (v goVar) String() string {
	return fmt.Sprintf("%s %s", v.name, v.typ)
}

type goVarList []goVar

func (v goVarList) String() string {
	vstrs := []string{}
	for _, n := range v {
		vstrs = append(vstrs, n.String())
	}
	return strings.Join(vstrs, ", ")
}

type goFields []goVar

func (v goFields) String() string {
	vstrs := []string{}
	for _, n := range v {
		vstrs = append(vstrs, n.String())
	}
	return strings.Join(vstrs, "\n\t")
}

type goStruct struct {
	name   string
	fields goFields
}

func (g *goStruct) emit(buf io.Writer) {
	fmt.Fprintf(buf, "type %s struct {\n\t%s\n}\n\n", g.name, g.fields)
}

type goMethod struct {
	name string
	in   goVarList
	out  goVarList
}

func (g *goMethod) String() string {
	return fmt.Sprintf("%s(%s) (%s)\n", g.name, g.in, g.out)
}

type goMethods []goMethod

func (m goMethods) String() string {
	mstrs := []string{}
	for _, n := range m {
		mstrs = append(mstrs, n.String())
	}
	return strings.Join(mstrs, "\n\t")
}

type goInterface struct {
	name    string
	methods goMethods
}

func (g *goInterface) emit(buf io.Writer) {
	fmt.Fprintf(buf, "type %s interface {\n\t%s\n}\n\n", g.name, g.methods)
}

type goStatement struct {
	source string
}

type goFunc struct {
	recv goVar
	name string
	tmp  int
	in   goVarList
	out  goVarList
	stmt []goStatement
}

func (g *goFunc) emit(buf io.Writer) {
	fmt.Fprintf(buf, "func (%s) %s(%s) (%s) {\n", g.recv.String(), g.name, g.in.String(), g.out.String())
	for _, s := range g.stmt {
		fmt.Fprintf(buf, "\t%s", s.source)
	}
	fmt.Fprintf(buf, "}\n\n")
}

type goEnumValue struct {
	name  string
	value int
}

type goEnum struct {
	name     string
	decltype string
	values   []goEnumValue
}

func (g *goEnum) emit(buf io.Writer) {
	fmt.Fprintf(buf, "type %s %s\n", g.name, g.decltype)
	fmt.Fprintf(buf, "const (\n")
	for _, v := range g.values {
		fmt.Fprintf(buf, "\t%s %s = %d\n", v.name, g.name, v.value)
	}
	fmt.Fprintf(buf, ")\n\n")
}

type goUnit struct {
	pkgname    string
	imports    map[string]string
	enums      []goEnum
	interfaces []goInterface
	structs    []goStruct
	methods    []goFunc
}

func (g *goUnit) emit(buf io.Writer) {
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
	for _, e := range g.interfaces {
		e.emit(buf)
	}
	for _, s := range g.structs {
		s.emit(buf)
	}
	for _, m := range g.methods {
		m.emit(buf)
	}
}

func isSeparator(r rune) bool {
	if r <= 0x7F {
		switch {
		case '0' <= r && r <= '9':
			return false
		case 'a' <= r && r <= 'z':
			return false
		case 'A' <= r && r <= 'Z':
			return false
		case r == '_':
			return false
		}
		return true
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return false
	}
	return unicode.IsSpace(r)
}

func titleCase(s string) string {
	prev := ' '
	return strings.Map(
		func(r rune) rune {
			if isSeparator(prev) {
				prev = r
				return unicode.ToTitle(r)
			}
			prev = r
			return r
		},
		s)
}
