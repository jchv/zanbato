package golang

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"unicode"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/emitter"
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/types"
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
	pkgname   string
	pkgpath   string
	resolver  ResolveFunc
	endian    types.EndianKind
	context   *engine.Context
	artifacts []emitter.Artifact
}

// NewEmitter constructs a new emitter with the given parameters.
func NewEmitter(pkgpath string, resolver ResolveFunc) *Emitter {
	return &Emitter{
		pkgname:  path.Base(pkgpath),
		pkgpath:  pkgpath,
		resolver: resolver,
		context:  engine.NewContext(),
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

func (e *Emitter) typeSwitchCaseTypeName(typ *engine.ExprType, value string) string {
	typeSwitchName := e.prefix(typ.Parent) + e.typeSwitchName(typ.Attr.Type.TypeSwitch.FieldName)
	return typeSwitchName + "_" + value
}

func (e *Emitter) fieldName(n kaitai.Identifier) string {
	return e.typeName(n)
}

func (e *Emitter) setImport(unit *goUnit, pkg string, as string) {
	unit.imports[pkg] = as
}

func (e *Emitter) resolveType(ex string) *engine.ExprType {
	typ := engine.ResultTypeOfExpr(e.context, expr.MustParseExpr(ex)).Type()
	if typ == nil {
		panic(fmt.Errorf("unresolved type: %s", ex))
	}
	return typ
}

func (e *Emitter) declTypeRef(n *types.TypeRef, r types.RepeatType) string {
	if r != nil {
		return "[]" + e.declTypeRef(n, nil)
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
		return "uint64"
	case types.F4, types.F4le, types.F4be:
		return "float32"
	case types.F8, types.F8le, types.F8be:
		return "float64"
	case types.Bytes:
		return "[]byte"
	case types.String:
		return "string"
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

func (e *Emitter) declTypeSwitch(parent *engine.ExprType, n *types.TypeSwitch, r types.RepeatType) string {
	if r != nil {
		return "[]" + e.declTypeSwitch(parent, n, nil)
	}
	return e.prefix(parent) + e.typeSwitchName(n.FieldName)
}

func (e *Emitter) declType(typ *engine.ExprType) string {
	switch typ.Kind {
	case engine.StructKind:
		return e.prefix(typ.Parent) + e.typeName(typ.Struct.Type.ID)
	case engine.EnumKind:
		return e.prefix(typ.Parent) + e.typeName(typ.Enum.ID)
	default:
		vt, ok := typ.ValueType()
		if !ok {
			return ""
		}
		if vt.Type.TypeRef != nil {
			return e.declTypeRef(vt.Type.TypeRef, vt.Repeat)
		} else if vt.Type.TypeSwitch != nil {
			return e.declTypeSwitch(typ.Parent, vt.Type.TypeSwitch, vt.Repeat)
		} else {
			panic("invalid type")
		}
	}
}

func (e *Emitter) readCallRef(n *types.TypeRef) string {
	switch n.Kind {
	case types.UntypedInt:
		panic("untyped number")
	case types.U2, types.U4, types.U8,
		types.S2, types.S4, types.S8,
		types.F4, types.F8:
		panic("undecided endianness")
	case types.U1:
		return "io.ReadU1()"
	case types.U2le:
		return "io.ReadU2le()"
	case types.U2be:
		return "io.ReadU2be()"
	case types.U4le:
		return "io.ReadU4le()"
	case types.U4be:
		return "io.ReadU4be()"
	case types.U8le:
		return "io.ReadU8le()"
	case types.U8be:
		return "io.ReadU8be()"
	case types.S1:
		return "io.ReadS1()"
	case types.S2le:
		return "io.ReadS2le()"
	case types.S2be:
		return "io.ReadS2be()"
	case types.S4le:
		return "io.ReadS4le()"
	case types.S4be:
		return "io.ReadS4be()"
	case types.S8le:
		return "io.ReadS8le()"
	case types.S8be:
		return "io.ReadS8be()"
	case types.Bits:
		panic("not implemented yet: bits")
	case types.F4le:
		return "io.ReadF4le()"
	case types.F4be:
		return "io.ReadF4be()"
	case types.F8le:
		return "io.ReadF8le()"
	case types.F8be:
		return "io.ReadF8be()"
	case types.Bytes:
		if n.Bytes.Size != nil {
			return fmt.Sprintf("io.ReadBytes(int(%s))", e.expr(n.Bytes.Size))
		}
		if n.Bytes.SizeEOS {
			return "io.ReadBytesFull()"
		}
		panic("not implemented yet: bytes")
	case types.String:
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
			return fmt.Sprintf("io.ReadBytesTerm(%q, %v, %v, %v)", rune(n.String.Terminator), n.String.Include, n.String.Consume, n.String.EosError)
		}
	case types.User:
		panic("called readCallRef on user type!")
	}
	panic("unexpected typekind: " + n.Kind.String())
}

func (e *Emitter) expr(expr *expr.Expr) string {
	return e.exprNode(expr.Root)
}

func (e *Emitter) calcPromotionTypeKind(a types.Kind, b types.Kind) string {
	if a > b {
		a, b = b, a
	}

	return e.declTypeRef(&types.TypeRef{Kind: a.Promote(b)}, nil)
}

func (e *Emitter) calcPromotionTypeRef(a *types.TypeRef, b *types.TypeRef) string {
	return e.calcPromotionTypeKind(a.Kind, b.Kind)
}

func (e *Emitter) calcPromotionExprType(a *engine.ExprType, b *engine.ExprType) string {
	vta, ok := a.ValueType()
	if !ok {
		return ""
	}
	vtb, ok := b.ValueType()
	if !ok {
		return ""
	}
	return e.calcPromotionTypeRef(vta.Type.TypeRef, vtb.Type.TypeRef)
}

func (e *Emitter) calcPromotionExprValue(a *engine.ExprValue, b *engine.ExprValue) string {
	return e.calcPromotionExprType(a.Type, b.Type)
}

func (e *Emitter) calcPromotionNode(a expr.Node, b expr.Node) string {
	av := engine.ResultTypeOfNode(e.context, a).Value()
	if av == nil {
		return ""
	}
	bv := engine.ResultTypeOfNode(e.context, b).Value()
	if bv == nil {
		return ""
	}
	return e.calcPromotionExprValue(av, bv)
}

func (e *Emitter) exprPromotionTernaryNode(n expr.TernaryNode) string {
	return e.calcPromotionNode(n.B, n.C)
}

func (e *Emitter) exprPromotionBinaryNode(n expr.BinaryNode) string {
	switch n.Op {
	case expr.OpAdd, expr.OpSub, expr.OpMult, expr.OpDiv, expr.OpMod,
		expr.OpLessThan, expr.OpLessThanEqual,
		expr.OpGreaterThan, expr.OpGreaterThanEqual,
		expr.OpEqual, expr.OpNotEqual,
		expr.OpBitAnd, expr.OpBitOr, expr.OpBitXor,
		expr.OpLogicalAnd, expr.OpLogicalOr:
		return e.calcPromotionNode(n.A, n.B)

	default:
		// Should not need to cast.
		return ""
	}
}

func (e *Emitter) exprTernaryNode(t expr.TernaryNode) string {
	cast := e.exprPromotionTernaryNode(t)
	// Go does not have a conditional expression. You could emulate it using a
	// generic helper method, but then the sub-expressions would be evaluated
	// eagerly. However, Go DOES have a function expression that can be called
	// inline, and it will implicitly capture any locals. So, we can use this
	// to use any statement inside of an expression.
	return fmt.Sprintf("(func() (%s) { if (%s) { return (%s)(%s) } else { return (%s)(%s) } }())",
		cast, e.exprNode(t.A), cast, e.exprNode(t.B), cast, e.exprNode(t.C))
}

func (e *Emitter) exprBinaryNode(t expr.BinaryNode) string {
	cast := e.exprPromotionBinaryNode(t)
	switch t.Op {
	case expr.OpAdd:
		return fmt.Sprintf("(%s)(%s) + (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpSub:
		return fmt.Sprintf("(%s)(%s) - (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpMult:
		return fmt.Sprintf("(%s)(%s) * (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpDiv:
		return fmt.Sprintf("(%s)(%s) / (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpMod:
		return fmt.Sprintf("(%s)(%s) %% (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpLessThan:
		return fmt.Sprintf("(%s)(%s) < (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpLessThanEqual:
		return fmt.Sprintf("(%s)(%s) <= (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpGreaterThan:
		return fmt.Sprintf("(%s)(%s) > (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpGreaterThanEqual:
		return fmt.Sprintf("(%s)(%s) >= (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpEqual:
		return fmt.Sprintf("(%s)(%s) == (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpNotEqual:
		return fmt.Sprintf("(%s)(%s) != (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpShiftLeft:
		return fmt.Sprintf("(%s)(%s) << (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpShiftRight:
		return fmt.Sprintf("(%s)(%s) >> (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpBitAnd:
		return fmt.Sprintf("(%s)(%s) & (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpBitOr:
		return fmt.Sprintf("(%s)(%s) | (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpBitXor:
		return fmt.Sprintf("(%s)(%s) ^ (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpLogicalAnd:
		return fmt.Sprintf("(%s)(%s) && (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	case expr.OpLogicalOr:
		return fmt.Sprintf("(%s)(%s) || (%s)(%s)", cast, e.exprNode(t.A), cast, e.exprNode(t.B))
	default:
		panic(fmt.Errorf("unsupported binary op %s", t.Op))
	}
}

func (e *Emitter) exprNode(node expr.Node) string {
	switch t := node.(type) {
	case expr.IdentNode, expr.ScopeNode, expr.MemberNode:
		v := engine.ResultTypeOfNode(e.context, t).Value()
		if v == nil {
			panic(fmt.Errorf("unable to use subexpression %s as value", t))
		}
		switch v.Type.Kind {
		case engine.ParamKind:
			// TODO: need to handle struct/instance navigation...
			return fmt.Sprintf("this.%s", e.fieldName(v.Type.Param.ID))
		case engine.AttrKind:
			// TODO: need to handle struct/instance navigation...
			return fmt.Sprintf("this.%s", e.fieldName(v.Type.Attr.ID))
		case engine.IntegerKind:
			if v.Type.Parent != nil && v.Type.Parent.Kind == engine.EnumValueKind {
				return e.enumValueName(e.parentStruct(v), e.parentEnum(v), v.Type.Parent.EnumValue.ID)
			} else {
				panic(fmt.Errorf("unexpected constant in subexpression %s", t))
			}
		default:
			panic(fmt.Errorf("unsupported value type reference %s in subexpression %s", v.Type.Kind, t))
		}
	case expr.IntNode:
		return t.Integer.String()
	case expr.BoolNode:
		return t.String()
	case expr.BinaryNode:
		return e.exprBinaryNode(t)
	case expr.TernaryNode:
		return e.exprTernaryNode(t)
	default:
		panic(fmt.Errorf("unsupported expression node %T", t))
	}
}

func (e *Emitter) root(inputname string, s *kaitai.Struct) {
	if s.Meta.Endian.Kind != types.UnspecifiedOrder {
		e.endian = s.Meta.Endian.Kind
	}

	unit := goUnit{
		pkgname: e.pkgname,
		imports: map[string]string{},
	}

	// Pivot stack to new root
	root := engine.NewStructValueSymbol(engine.NewStructTypeSymbol(s, nil), nil)
	e.context.AddGlobalType(string(s.ID), root.Type)
	e.context.AddModuleType(string(s.ID), root.Type)
	oldContext := e.context
	e.context = e.context.WithModuleRoot(root).WithLocalRoot(root)

	e.struc(inputname, &unit, root)

	// Pivot back to old root
	e.context = oldContext

	out := bytes.Buffer{}
	unit.emit(&out)

	e.artifacts = append(e.artifacts, emitter.Artifact{
		Filename: e.filename(s.ID),
		Body:     out.Bytes(),
	})
}

func (e *Emitter) push(val *engine.ExprValue) {
	e.context = e.context.WithLocalRoot(val)
}

func (e *Emitter) pop() {
	e.context = e.context.Parent()
}

func (e *Emitter) enumTypeName(parent *engine.ExprType, enum *kaitai.Enum) string {
	return e.prefix(parent) + e.typeName(enum.ID)
}

func (e *Emitter) enumValueName(parent *engine.ExprType, enum *kaitai.Enum, id kaitai.Identifier) string {
	return e.prefix(parent) + e.typeName(enum.ID) + "__" + e.typeName(id)
}

func (e *Emitter) enum(unit *goUnit, enum *engine.ExprType) {
	g := goEnum{name: e.enumTypeName(enum.Parent, enum.Enum), decltype: "int"}
	for _, v := range enum.Enum.Values {
		g.values = append(g.values, goEnumValue{name: e.enumValueName(enum.Parent, enum.Enum, v.ID), value: int(v.Value.Int64())})
	}
	unit.enums = append(unit.enums, g)
}

func (e *Emitter) isValidEndianTypeRef(t *types.TypeRef) bool {
	switch t.Kind {
	case types.U2, types.U4, types.U8,
		types.S2, types.S4, types.S8,
		types.F4, types.F8:
		return false
	default:
		return true
	}
}

func (e *Emitter) isValidEndianTypeSwitch(t *types.TypeSwitch) bool {
	for _, value := range t.Cases {
		if !e.isValidEndianTypeRef(&value) {
			return false
		}
	}
	return true
}

func (e *Emitter) isValidEndianType(t types.Type) bool {
	if t.TypeRef != nil {
		return e.isValidEndianTypeRef(t.TypeRef)
	} else if t.TypeSwitch != nil {
		return e.isValidEndianTypeSwitch(t.TypeSwitch)
	} else {
		panic("invalid type")
	}
}

func (e *Emitter) setParams(struc string, tr *types.UserType, resolved *kaitai.Struct, fn *goFunc) {
	for i := range tr.Params {
		field := e.typeName(resolved.Params[i].ID)
		fn.printf("%s.%s = %s", struc, field, e.expr(tr.Params[i]))
	}
}

func (e *Emitter) readAttr(unit *goUnit, fn *goFunc, typ *engine.ExprType, forcedEndian types.EndianKind) bool {
	a := typ.Attr

	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Errorf("attr: %s: %s", a.ID, r))
		}
	}()

	endianSuffix := ""
	if forcedEndian == types.LittleEndian {
		endianSuffix = "LE"
	} else if forcedEndian == types.BigEndian {
		endianSuffix = "BE"
	}

	rt := a.Type.FoldEndian(e.endian)

	if !e.isValidEndianType(rt) {
		fn.printf("return kaitai.UndecidedEndiannessError{}")
		return false
	}

	fn.tmp++

	fieldName := e.fieldName(a.ID)

	if a.If != nil {
		fn.printf("if %s {", e.expr(a.If)).indent()
	}

	if rt.TypeSwitch != nil {
		// Call type-switch helper
		switchName := e.prefix(typ.Parent) + e.typeSwitchName(rt.TypeSwitch.FieldName)
		fn.printf("if err := this.read%s%s(io); err != nil {", switchName, endianSuffix).indent()
		fn.printf("return err")
		fn.unindent().printf("}")
	} else {
		switch rt.TypeRef.Kind {
		case types.User:
			// ---------------------------------------------------------------------
			// User case: Need to call Read method of field
			// ---------------------------------------------------------------------
			resolved := e.resolveType(rt.TypeRef.User.Name)
			if resolved.Kind != engine.StructKind {
				panic(fmt.Errorf("expression %q yielded unexpected type %s (expected struct)", rt.TypeRef.User.Name, resolved.Kind))
			}
			switch repeat := a.Repeat.(type) {
			case types.RepeatEOS:
				fn.printf("for {").indent()

				// EOF return
				fn.printf("if eof, err := io.EOF(); err != nil {").indent()
				fn.printf("return err")
				fn.unindent().printf("} else if eof {").indent()
				fn.printf("break")
				fn.unindent().printf("}")

				// Read
				declType := e.declTypeRef(rt.TypeRef, nil)
				fn.printf("tmp%d := %s{}", fn.tmp, declType)
				e.setParams(fmt.Sprintf("tmp%d", fn.tmp), rt.TypeRef.User, resolved.Struct.Type, fn)
				fn.printf("if err := tmp%d.Read%s(io); err != nil {", fn.tmp, endianSuffix).indent()
				fn.printf("return err")
				fn.unindent().printf("}")
				fn.printf("this.%s = append(this.%s, tmp%d)", fieldName, fieldName, fn.tmp)

				fn.unindent().printf("}")

			case types.RepeatExpr:
				iterType := engine.ResultTypeOfExpr(e.context, repeat.CountExpr)
				iterCast := e.declType(iterType.Value().Type)
				fn.printf("for i := %s(0); i < %s; i++ {", iterCast, e.expr(repeat.CountExpr)).indent()
				fn.printf("tmp%d := %s{}", fn.tmp, e.declTypeRef(rt.TypeRef, nil))
				e.setParams(fmt.Sprintf("tmp%d", fn.tmp), rt.TypeRef.User, resolved.Struct.Type, fn)
				fn.printf("if err := tmp%d.Read%s(io); err != nil {", fn.tmp, endianSuffix).indent()
				fn.printf("return err")
				fn.unindent().printf("}")
				fn.printf("this.%s = append(this.%s, tmp%d)", fieldName, fieldName, fn.tmp)
				fn.unindent().printf("}")

			case types.RepeatUntil:
				panic("not implemented: repeat until")

			case nil:
				fn.printf("tmp%d := %s{}", fn.tmp, e.declTypeRef(rt.TypeRef, nil))
				e.setParams(fmt.Sprintf("tmp%d", fn.tmp), rt.TypeRef.User, resolved.Struct.Type, fn)
				fn.printf("if err := tmp%d.Read%s(io); err != nil {", fn.tmp, endianSuffix).indent()
				fn.printf("return err")
				fn.unindent().printf("}")
				fn.printf("this.%s = tmp%d", fieldName, fn.tmp)
			}

		default:
			// ---------------------------------------------------------------------
			// General case: Need to assign field using readCall function
			// ---------------------------------------------------------------------
			readCall := e.readCallRef(rt.TypeRef)

			cast := ""
			if a.Type.TypeRef != nil && a.Type.TypeRef.Kind == types.String {
				cast = "string"
			}

			switch repeat := a.Repeat.(type) {
			case types.RepeatEOS:
				fn.printf("for {").indent()

				// EOF return
				fn.printf("if eof, err := io.EOF(); err != nil {").indent()
				fn.printf("return err")
				fn.unindent().printf("} else if eof {").indent()
				fn.printf("break")
				fn.unindent().printf("}")

				// Read
				fn.printf("tmp%d, err := %s", fn.tmp, readCall)
				fn.printf("if err != nil {").indent()
				fn.printf("return err")
				fn.unindent().printf("}")
				fn.printf("this.%s = append(this.%s, %s(tmp%d))", fieldName, fieldName, cast, fn.tmp)

				fn.unindent().printf("}")

			case types.RepeatExpr:
				iterType := engine.ResultTypeOfExpr(e.context, repeat.CountExpr)
				iterCast := e.declType(iterType.Value().Type)
				fn.printf("for i := %s(0); i < %s; i++ {", iterCast, e.expr(repeat.CountExpr)).indent()
				fn.printf("tmp%d, err := %s", fn.tmp, readCall)
				fn.printf("if err != nil {").indent()
				fn.printf("return err")
				fn.unindent().printf("\t}")
				fn.printf("this.%s = append(this.%s, %s(tmp%d))", fieldName, fieldName, cast, fn.tmp)
				fn.unindent().printf("}")

			case types.RepeatUntil:
				panic("not implemented: repeat until")

			case nil:
				fn.printf("tmp%d, err := %s", fn.tmp, readCall)
				fn.printf("if err != nil {").indent()
				fn.printf("return err")
				fn.unindent().printf("}")
				fn.printf("this.%s = %s(tmp%d)", fieldName, cast, fn.tmp)
			}

			if a.Contents != nil {
				e.setImport(unit, "bytes", "bytes")
				e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)

				fn.printf("if !bytes.Equal(tmp%d, %#v) {", fn.tmp, a.Contents).indent()
				fn.printf("return kaitai.NewValidationNotEqualError(%#v, tmp%d, io, \"\") // TODO: set srcPath", a.Contents, fn.tmp)
				fn.unindent().printf("}")
			}
		}
	}

	if a.If != nil {
		fn.unindent().printf("}")
	}

	return true
}

func (e *Emitter) parentStruct(val *engine.ExprValue) *engine.ExprType {
	typ := val.Type
	for typ != nil {
		if typ.Kind == engine.StructKind {
			return typ
		}
		typ = typ.Parent
	}
	return nil
}

func (e *Emitter) parentEnum(val *engine.ExprValue) *kaitai.Enum {
	typ := val.Type
	for typ != nil {
		if typ.Kind == engine.EnumKind {
			return typ.Enum
		}
		typ = typ.Parent
	}
	return nil
}

func (e *Emitter) typeSwitchCaseValue(value string) string {
	ex := expr.MustParseExpr(value)
	val := engine.ResultTypeOfExpr(e.context, ex).Value()
	if val == nil {
		panic(fmt.Errorf("unresolved: %s", value))
	}
	if val.Type != nil && val.Type.Parent != nil && val.Type.Parent.Kind == engine.EnumValueKind {
		return e.enumValueName(e.parentStruct(val), e.parentEnum(val), val.Type.Parent.EnumValue.ID)
	} else {
		return e.expr(ex)
	}
}

func (e *Emitter) typeSwitchStruct(unit *goUnit, typ *engine.ExprType) {
	ts := typ.Attr.Type.TypeSwitch
	typeSwitchName := e.prefix(typ.Parent) + e.typeSwitchName(ts.FieldName)
	unit.interfaces = append(unit.interfaces, goInterface{
		name: typeSwitchName,
		methods: goMethods{
			goMethod{
				name: "is" + typeSwitchName,
			},
		},
	})
	for value, caseType := range ts.Cases {
		goUnderlyingType := e.declTypeRef(&caseType, nil)
		goValue := e.typeSwitchCaseValue(value)
		caseStruct := e.typeSwitchCaseTypeName(typ, goValue)
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
}

func (e *Emitter) typeSwitch(unit *goUnit, val *engine.ExprValue, forceEndian types.EndianKind) {
	attr := val.Type.Attr
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

	ts := attr.Type.TypeSwitch
	typeSwitchName := e.prefix(val.Parent.Type) + e.typeSwitchName(ts.FieldName)
	readFn := goFunc{
		recv: goVar{name: "this", typ: "*" + e.prefix(val.Type.Parent.Parent) + e.typeName(val.Type.Parent.Struct.Type.ID)},
		name: "read" + typeSwitchName + endianSuffix,
		in:   []goVar{{name: "io", typ: "*" + kaitaiStream}},
		out:  []goVar{{name: "err", typ: "error"}},
	}
	readFn.printf("switch %s {", e.expr(ts.SwitchOn))
	for value, typ := range ts.Cases {
		switchOnType := engine.ResultTypeOfExpr(e.context, ts.SwitchOn)
		typeCast := e.declType(switchOnType.Value().Type)
		goValue := e.typeSwitchCaseValue(value)
		goUnderlyingType := e.declTypeRef(&typ, nil)
		caseStruct := e.typeSwitchCaseTypeName(val.Type, goValue)
		fieldName := e.fieldName(attr.ID)

		switch typ.Kind {
		case types.User:
			readFn.tmp++
			resolved := e.resolveType(typ.User.Name)
			if resolved.Kind != engine.StructKind {
				panic(fmt.Errorf("expression %q yielded unexpected type %s (expected struct)", typ.User.Name, resolved.Kind))
			}
			readFn.printf("case (%s)(%s):", typeCast, goValue).indent()
			readFn.printf("tmp%d := %s{}", readFn.tmp, goUnderlyingType)
			e.setParams(fmt.Sprintf("tmp%d", readFn.tmp), typ.User, resolved.Struct.Type, &readFn)
			readFn.printf("if err := tmp%d.Read(io); err != nil {", readFn.tmp).indent()
			readFn.printf("return err")
			readFn.unindent().printf("}")

			readFn.printf("this.%s = %s{Value: tmp%d}", fieldName, caseStruct, readFn.tmp)
			readFn.unindent()

		default:
			typ = typ.FoldEndian(e.endian)
			call := e.readCallRef(&typ)
			readFn.printf("case (%s)(%s):", typeCast, goValue).indent()
			readFn.printf("tmp%d, err := %s", readFn.tmp, call)
			readFn.printf("if err != nil {").indent()
			readFn.printf("\treturn err")
			readFn.unindent().printf("}")
			readFn.printf("this.%s = %s{Value: tmp%d}", fieldName, caseStruct, readFn.tmp)
			readFn.unindent()
		}
	}

	readFn.printf("}")
	readFn.printf("return nil")

	unit.methods = append(unit.methods, readFn)
}

func (e *Emitter) prefix(typ *engine.ExprType) string {
	if typ == nil || typ.Struct == nil {
		return ""
	}
	return e.typeName(typ.Struct.Type.ID) + "_"
}

func (e *Emitter) prefixVal(val *engine.ExprValue) string {
	if val == nil || val.Type == nil {
		return ""
	}
	return e.prefix(val.Type)
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

func (e *Emitter) strucRead(unit *goUnit, gs *goStruct, val *engine.ExprValue, forceEndian types.EndianKind) {
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

	e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
	readMethod := goFunc{
		recv: goVar{name: "this", typ: "*" + gs.name},
		name: "Read" + endianSuffix,
		in:   []goVar{{name: "io", typ: "*" + kaitaiStream}},
		out:  []goVar{{name: "err", typ: "error"}},
	}
	errExit := false
	for _, attr := range val.Struct.Attrs {
		if !e.readAttr(unit, &readMethod, attr.Type, forceEndian) {
			// We may need to end the function early in some cases.
			errExit = true
			break
		}
	}
	if !errExit {
		readMethod.printf("return nil")
	}
	unit.methods = append(unit.methods, readMethod)
}

func (e *Emitter) endianStubs(unit *goUnit, gs *goStruct, ks *kaitai.Struct) {
	e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
	unit.methods = append(unit.methods, goFunc{
		recv:   goVar{name: "this", typ: "*" + gs.name},
		name:   "ReadBE",
		in:     []goVar{{name: "io", typ: "*" + kaitaiStream}},
		out:    []goVar{{name: "err", typ: "error"}},
		source: "\treturn this.Read(io)\n",
	})
	unit.methods = append(unit.methods, goFunc{
		recv:   goVar{name: "this", typ: "*" + gs.name},
		name:   "ReadLE",
		in:     []goVar{{name: "io", typ: "*" + kaitaiStream}},
		out:    []goVar{{name: "err", typ: "error"}},
		source: "\treturn this.Read(io)\n",
	})
}

func (e *Emitter) endianSwitch(unit *goUnit, gs *goStruct, ks *kaitai.Struct) {
	e.setImport(unit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)

	fn := goFunc{
		recv: goVar{name: "this", typ: "*" + gs.name},
		name: "ReadBE",
		in:   []goVar{{name: "io", typ: "*" + kaitaiStream}},
		out:  []goVar{{name: "err", typ: "error"}},
	}

	fn.printf("switch %s {", e.expr(ks.Meta.Endian.SwitchOn))
	for value, endian := range ks.Meta.Endian.Cases {
		fn.printf("case %s:", e.typeSwitchCaseValue(value))
		if endian == types.LittleEndian {
			fn.printf("\treturn this.ReadLE(io)")
		} else {
			fn.printf("\treturn this.ReadBE(io)")
		}
	}
	fn.printf("default:")
	fn.printf("\treturn kaitai.UndecidedEndiannessError{}")
	fn.printf("}")

	unit.methods = append(unit.methods, fn)
}

func (e *Emitter) struc(inputname string, unit *goUnit, val *engine.ExprValue) {
	ks := val.Type.Struct.Type

	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Errorf("struct %s: %s", ks.ID, r))
		}
	}()

	name := e.typeName(ks.ID)
	prefix := e.prefix(val.Type.Parent)

	gs := goStruct{name: prefix + name}

	e.push(val)
	defer e.pop()

	// Handle imports before anything else...
	for _, n := range ks.Meta.Imports {
		e.root(e.resolver(inputname, n))
	}

	// Then handle nested structures
	for _, n := range val.Type.Struct.Structs {
		e.struc(inputname, unit, engine.NewStructValueSymbol(n, val))
	}

	// Enumerations
	for _, n := range val.Type.Struct.Enums {
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
		gs.fields = append(gs.fields, goVar{
			name: e.fieldName(attr.Type.Attr.ID),
			typ:  e.declType(attr.Type),
		})
	}

	unit.structs = append(unit.structs, gs)

	// Deserialization
	if e.endian == types.SwitchEndian || (e.needMultipleEndian(ks) && e.endian == types.UnspecifiedOrder) {
		if e.endian == types.SwitchEndian {
			e.endianSwitch(unit, &gs, ks)
		} else {
			// Generate unspecified endian even if it does always return an error.
			e.strucRead(unit, &gs, val, types.UnspecifiedOrder)
		}
		e.strucRead(unit, &gs, val, types.LittleEndian)
		e.strucRead(unit, &gs, val, types.BigEndian)

		for _, attr := range val.Struct.Attrs {
			if attr.Type.Attr.Type.TypeSwitch != nil {
				e.typeSwitchStruct(unit, attr.Type)
				e.typeSwitch(unit, attr, types.LittleEndian)
				e.typeSwitch(unit, attr, types.BigEndian)
			}
		}
	} else {
		// Struct is always consistent endianness: generate one read function and make two stubs to it.
		e.strucRead(unit, &gs, val, types.UnspecifiedOrder)
		e.endianStubs(unit, &gs, ks)

		for _, attr := range val.Struct.Attrs {
			if attr.Type.Attr.Type.TypeSwitch != nil {
				e.typeSwitchStruct(unit, attr.Type)
				e.typeSwitch(unit, attr, types.UnspecifiedOrder)
			}
		}
	}
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

type goFunc struct {
	recv   goVar
	name   string
	tmp    int
	in     goVarList
	out    goVarList
	source string
	pfx    string
}

func (g *goFunc) emit(buf io.Writer) {
	fmt.Fprintf(buf, "func (%s) %s(%s) (%s) {\n%s}\n\n", g.recv.String(), g.name, g.in.String(), g.out.String(), g.source)
}

func (g *goFunc) printf(format string, args ...any) *goFunc {
	g.source += "\t" + g.pfx + fmt.Sprintf(format, args...) + "\n"
	return g
}

func (g *goFunc) indent() *goFunc {
	g.pfx += "\t"
	return g
}

func (g *goFunc) unindent() *goFunc {
	g.pfx = g.pfx[:len(g.pfx)-1]
	return g
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
