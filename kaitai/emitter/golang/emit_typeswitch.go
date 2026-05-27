package golang

import (
	"fmt"
	"strings"

	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/types"
)

func (e *Emitter) typeSwitchCaseValue(value string) string {
	if value == "_" {
		return "_default_"
	}
	ex := expr.MustParseExpr(value)
	val := engine.ResultTypeOfExpr(e.context, ex)
	if val == nil {
		panic(fmt.Errorf("unresolved: %s", value))
	}
	if val.Parent != nil && val.Parent.Kind == engine.EnumValueKind {
		enumVal := val.NearestEnum()
		if enumVal == nil || enumVal.Enum == nil {
			panic(fmt.Errorf("enum value without enum ancestor: %s", value))
		}
		return e.enumValueName(val.NearestStruct(), enumVal.Enum, val.Parent.EnumValue.ID)
	} else {
		return e.expr(ex)
	}
}

func (e *Emitter) emitTypeSwitchStruct(unit *goUnit, typ *engine.ExprValue) {
	// No-op: switch types use 'any' fields with direct value storage.
	// No wrapper interfaces or case structs are needed.
}

func (e *Emitter) emitTypeSwitchRead(unit *goUnit, val *engine.ExprValue, forceEndian types.EndianKind) {
	e.emitTypeSwitchReadWithPrefix(unit, val, forceEndian, "")
}

func (e *Emitter) emitTypeSwitchReadWithPrefix(unit *goUnit, val *engine.ExprValue, forceEndian types.EndianKind, fieldPrefix string) {
	attr := val.Attr
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
	typeSwitchName := e.prefix(val.Parent) + e.typeSwitchName(ts.FieldName)
	inputs := []goVar{{name: "stream", typ: "*" + kaitaiStream}}
	if exprContainsIndex(ts.SwitchOn) {
		inputs = append(inputs, goVar{name: "i", typ: "int"})
	}
	readFn := goFunc{
		recv: goVar{name: "this", typ: "*" + e.prefix(val.Parent.Parent) + e.typeName(val.Parent.Struct.Type.ID)},
		name: "read" + typeSwitchName + endianSuffix,
		in:   inputs,
		out:  []goVar{{name: "err", typ: "error"}},
	}
	switchOnType := engine.ResultTypeOfExpr(e.context, ts.SwitchOn)
	typeCast := e.declType(switchOnType)
	// If the switch-on expression resolves to an enum, use the enum type for cases and switch expression
	isEnum := false
	if switchOnType != nil && switchOnType.Kind == engine.AttrKind {
		if switchOnType.Attr.Enum != "" {
			enumTyp := e.mustResolveType(switchOnType.Attr.Enum)
			typeCast = e.declType(enumTyp)
			isEnum = true
		}
	}
	// Check if any case values are byte arrays, or switch-on is byte-typed
	hasByteArrayCases := false
	if typeCast == "[]byte" {
		hasByteArrayCases = true
	} else {
		for value := range ts.Cases {
			if value != "_" && strings.HasPrefix(value, "[") {
				hasByteArrayCases = true
				break
			}
		}
	}
	switchOnExpr := e.expr(ts.SwitchOn)
	if hasByteArrayCases {
		// Only import bytes if there are non-default cases to compare
		for value := range ts.Cases {
			if value != "_" {
				e.file.needBytes = true
				break
			}
		}
	} else if isEnum {
		readFn.pf("switch %s {", switchOnExpr)
	} else {
		readFn.pf("switch (%s)(%s) {", typeCast, switchOnExpr)
	}
	firstByteCase := true
	for value, typ := range ts.Cases {
		fieldName := e.fieldName(attr.ID)
		if fieldPrefix != "" {
			fieldName = fieldPrefix + string(attr.ID)
		}

		// Generate case/if header
		var goValue string
		if !hasByteArrayCases {
			goValue = e.typeSwitchCaseValue(value)
		}

		emitCaseOpen := func() {
			if hasByteArrayCases {
				if value == "_" {
					if firstByteCase {
						readFn.pf("// default").indent()
					} else {
						readFn.unindent().pf("} else {").indent()
					}
				} else {
					caseExpr := expr.MustParseExpr(value)
					caseStr := e.exprNode(caseExpr.Root)
					// Wrap string literals in []byte() for bytes.Equal comparison
					if _, ok := caseExpr.Root.(expr.StringNode); ok {
						caseStr = fmt.Sprintf("[]byte(%s)", caseStr)
					}
					if firstByteCase {
						readFn.pf("if bytes.Equal(%s, %s) {", switchOnExpr, caseStr).indent()
					} else {
						readFn.unindent().pf("} else if bytes.Equal(%s, %s) {", switchOnExpr, caseStr).indent()
					}
					firstByteCase = false
				}
			} else {
				if goValue == "_default_" {
					readFn.pf("default:").indent()
				} else {
					readFn.pf("case (%s)(%s):", typeCast, goValue).indent()
				}
			}
		}
		emitCaseClose := func() {
			if hasByteArrayCases {
				// Close handled by next case or final close
			} else {
				readFn.unindent()
			}
		}

		switch typ.Kind {
		case types.User:
			readFn.tmp++
			resolved := e.mustResolveType(typ.User.Name)
			if resolved.Kind != engine.StructKind {
				panic(fmt.Errorf("expression %q yielded unexpected type %s (expected struct)", typ.User.Name, resolved.Kind))
			}
			isOpaque := e.isOpaqueType(resolved)
			emitCaseOpen()
			goUnderlyingType := e.declTypeRef(&typ, nil)
			newExpr := goUnderlyingType
			if strings.HasPrefix(newExpr, "*") {
				newExpr = "&" + newExpr[1:]
			}
			readFn.pf("tmp%d := %s{}", readFn.tmp, newExpr)
			if !isOpaque {
				e.setParams(fmt.Sprintf("tmp%d", readFn.tmp), typ, resolved, &readFn)
			}
			if isOpaque {
				readFn.pf("if err := tmp%d.Read(stream, nil, nil); err != nil {", readFn.tmp).indent()
			} else {
				readFn.pf("if err := tmp%d.Read(stream, this, this.Root_); err != nil {", readFn.tmp).indent()
			}
			readFn.pf("return err")
			readFn.unindent().pf("}")

			if attr.Repeat == nil {
				readFn.pf("this.%s = tmp%d", fieldName, readFn.tmp)
			} else {
				readFn.pf("this.%s = append(this.%s, tmp%d)", fieldName, fieldName, readFn.tmp)
			}
			emitCaseClose()

		default:
			typ = typ.FoldEndian(e.endian).FoldBitEndian(e.bitEndian)
			call := e.readCallRef(&typ)
			emitCaseOpen()
			readFn.tmp++
			readFn.pf("tmp%d, err := %s", readFn.tmp, call)
			readFn.pf("if err != nil {").indent()
			readFn.pf("\treturn err")
			readFn.unindent().pf("}")
			if attr.Repeat == nil {
				readFn.pf("this.%s = tmp%d", fieldName, readFn.tmp)
			} else {
				readFn.pf("this.%s = append(this.%s, tmp%d)", fieldName, fieldName, readFn.tmp)
			}
			emitCaseClose()
		}
	}

	// If no explicit default case was generated and the attr has a size
	// (meaning we're reading from a substream), add a default that reads raw bytes.
	hasDefault := false
	for value := range ts.Cases {
		if value == "_" {
			hasDefault = true
			break
		}
	}
	hasSizeConstraint := attr.Size != nil || attr.SizeEos
	if !hasDefault && hasSizeConstraint {
		fieldName := e.fieldName(attr.ID)
		if fieldPrefix != "" {
			fieldName = fieldPrefix + string(attr.ID)
		}
		if hasByteArrayCases {
			if firstByteCase {
				// No cases at all - just read bytes
				readFn.indent()
			} else {
				readFn.unindent().pf("} else {").indent()
			}
		} else {
			readFn.pf("default:").indent()
		}
		readFn.tmp++
		readFn.pf("tmp%d, err := stream.ReadBytesFull()", readFn.tmp)
		readFn.pf("if err != nil {").indent()
		readFn.pf("return err")
		readFn.unindent().pf("}")
		if attr.Repeat == nil {
			readFn.pf("this.%s = tmp%d", fieldName, readFn.tmp)
		} else {
			readFn.pf("this.%s = append(this.%s, tmp%d)", fieldName, fieldName, readFn.tmp)
		}
		if hasByteArrayCases {
			readFn.unindent()
		} else {
			readFn.unindent()
		}
	}
	if hasByteArrayCases {
		if !firstByteCase {
			readFn.unindent().pf("}")
		}
	} else {
		readFn.pf("}")
	}
	readFn.pf("return nil")

	e.ensureStructLinks(&readFn, val)
	unit.methods = append(unit.methods, readFn)
}

// isIntegerOnlySwitch checks if all cases in a type switch resolve to integer types.
func isIntegerOnlySwitch(ts *types.TypeSwitch) bool {
	for _, caseType := range ts.Cases {
		k := caseType.Kind
		switch {
		case k >= types.U1 && k <= types.S8be:
			continue
		case k == types.Bits:
			continue
		default:
			return false
		}
	}
	return true
}
