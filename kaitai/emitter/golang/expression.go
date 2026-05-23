package golang

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/types"
)

func (e *Emitter) expr(ex *expr.Expr) string {
	if ex == nil {
		panic("expr called with nil expression")
	}
	return e.exprNode(ex.Root)
}

func (e *Emitter) calcPromotionTypeKind(a types.Kind, b types.Kind) string {
	if a == b {
		return "" // Same types, no promotion needed
	}
	// Booleans can't be promoted to numeric types
	if a == types.UntypedBool || b == types.UntypedBool {
		return ""
	}
	if a == types.Bits || b == types.Bits {
		return ""
	}
	if a > b {
		a, b = b, a
	}

	return e.declTypeRef(&types.TypeRef{Kind: a.Promote(b)}, nil)
}

func (e *Emitter) calcPromotionTypeRef(a *types.TypeRef, b *types.TypeRef) string {
	if a == nil || b == nil {
		return ""
	}
	return e.calcPromotionTypeKind(a.Kind, b.Kind)
}

func (e *Emitter) calcPromotionExprType(a *engine.ExprValue, b *engine.ExprValue) string {
	if a == nil || b == nil {
		return ""
	}
	// For instances with default Bytes type, try to use inferred type instead
	aType := a
	bType := b
	if a.Kind == engine.InstanceKind {
		inferred := e.inferInstanceType(a)
		if mapped := e.goTypeToTypeRef(inferred); mapped != nil {
			aType = &engine.ExprValue{Kind: engine.AttrKind, Attr: &kaitai.Attr{Type: types.Type{TypeRef: mapped}}}
		}
	}
	if b.Kind == engine.InstanceKind {
		inferred := e.inferInstanceType(b)
		if mapped := e.goTypeToTypeRef(inferred); mapped != nil {
			bType = &engine.ExprValue{Kind: engine.AttrKind, Attr: &kaitai.Attr{Type: types.Type{TypeRef: mapped}}}
		}
	}
	vta, ok := aType.ValueType()
	if !ok {
		return ""
	}
	vtb, ok := bType.ValueType()
	if !ok {
		return ""
	}
	if vta.Type.TypeRef == nil || vtb.Type.TypeRef == nil {
		return ""
	}
	return e.calcPromotionTypeRef(vta.Type.TypeRef, vtb.Type.TypeRef)
}

// goTypeToTypeRef maps a Go type string back to a TypeRef for promotion calculations.
func (e *Emitter) goTypeToTypeRef(goType string) *types.TypeRef {
	switch goType {
	case "int":
		return &types.TypeRef{Kind: types.UntypedInt}
	case "uint8":
		return &types.TypeRef{Kind: types.U1}
	case "uint16":
		return &types.TypeRef{Kind: types.U2}
	case "uint32":
		return &types.TypeRef{Kind: types.U4}
	case "uint64":
		return &types.TypeRef{Kind: types.U8}
	case "int8":
		return &types.TypeRef{Kind: types.S1}
	case "int16":
		return &types.TypeRef{Kind: types.S2}
	case "int32":
		return &types.TypeRef{Kind: types.S4}
	case "int64":
		return &types.TypeRef{Kind: types.S8}
	case "float32":
		return &types.TypeRef{Kind: types.F4}
	case "float64":
		return &types.TypeRef{Kind: types.F8}
	case "bool":
		return &types.TypeRef{Kind: types.UntypedBool}
	case "string":
		return &types.TypeRef{Kind: types.String}
	case "[]byte":
		return &types.TypeRef{Kind: types.Bytes}
	}
	return nil
}

func (e *Emitter) calcPromotionExprValue(a *engine.ExprValue, b *engine.ExprValue) string {
	if a == nil || b == nil {
		return ""
	}
	return e.calcPromotionExprType(a, b)
}

func (e *Emitter) calcPromotionNode(a expr.Node, b expr.Node) string {
	av := engine.ResultTypeOfNode(e.context, a)
	if av == nil {
		return ""
	}
	bv := engine.ResultTypeOfNode(e.context, b)
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
		expr.OpBitAnd, expr.OpBitOr, expr.OpBitXor:
		return e.calcPromotionNode(n.A, n.B)

	default:
		// Should not need to cast.
		return ""
	}
}

// needsEncodingConversion returns whether the encoding needs explicit conversion
func (e *Emitter) normalizeEncoding(enc string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(enc, "-", ""), "_", ""))
}

func stringLiteralValue(node expr.Node) (string, bool) {
	switch n := node.(type) {
	case expr.StringNode:
		return n.Str, true
	default:
		return "", false
	}
}

func (e *Emitter) needsEncodingConversion(enc string) bool {
	enc = e.normalizeEncoding(enc)
	switch enc {
	case "", "UTF8", "ASCII":
		return false
	default:
		return true
	}
}

// encodingDecoder returns Go code to create a decoder for the given encoding
func (e *Emitter) encodingDecoder(unit *goUnit, enc string) string {
	enc = e.normalizeEncoding(enc)
	switch enc {
	case "UTF16LE":
		e.setImport(unit, "golang.org/x/text/encoding/unicode", "unicode")
		return "unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()"
	case "UTF16BE":
		e.setImport(unit, "golang.org/x/text/encoding/unicode", "unicode")
		return "unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewDecoder()"
	case "SHIFTJIS", "SJIS":
		e.setImport(unit, "golang.org/x/text/encoding/japanese", "japanese")
		return "japanese.ShiftJIS.NewDecoder()"
	case "IBM437", "CP437":
		e.setImport(unit, "golang.org/x/text/encoding/charmap", "charmap")
		return "charmap.CodePage437.NewDecoder()"
	case "ISO88591", "LATIN1":
		e.setImport(unit, "golang.org/x/text/encoding/charmap", "charmap")
		return "charmap.ISO8859_1.NewDecoder()"
	case "WINDOWS1252", "CP1252":
		e.setImport(unit, "golang.org/x/text/encoding/charmap", "charmap")
		return "charmap.Windows1252.NewDecoder()"
	default:
		return e.unsupportedEncodingDecoder(unit, enc)
	}
}

func (e *Emitter) unsupportedEncodingDecoder(unit *goUnit, enc string) string {
	e.setImport(unit, "fmt", "fmt")
	e.setImport(unit, "golang.org/x/text/encoding", "encoding")
	return fmt.Sprintf("(func() *encoding.Decoder { panic(fmt.Errorf(\"unsupported string encoding: %%s\", %s)) }())", strconv.Quote(enc))
}

func (e *Emitter) unsupportedEncodingEncoder(unit *goUnit, enc string) string {
	e.setImport(unit, "fmt", "fmt")
	e.setImport(unit, "golang.org/x/text/encoding", "encoding")
	return fmt.Sprintf("(func() *encoding.Encoder { panic(fmt.Errorf(\"unsupported string encoding: %%s\", %s)) }())", strconv.Quote(enc))
}

// encodingEncoder returns Go code to create an encoder for the given encoding.
// This is the inverse of encodingDecoder - used for writing strings back.
func (e *Emitter) encodingEncoder(unit *goUnit, enc string) string {
	enc = e.normalizeEncoding(enc)
	switch enc {
	case "UTF16LE":
		e.setImport(unit, "golang.org/x/text/encoding/unicode", "unicode")
		return "unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()"
	case "UTF16BE":
		e.setImport(unit, "golang.org/x/text/encoding/unicode", "unicode")
		return "unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM).NewEncoder()"
	case "SHIFTJIS", "SJIS":
		e.setImport(unit, "golang.org/x/text/encoding/japanese", "japanese")
		return "japanese.ShiftJIS.NewEncoder()"
	case "IBM437", "CP437":
		e.setImport(unit, "golang.org/x/text/encoding/charmap", "charmap")
		return "charmap.CodePage437.NewEncoder()"
	case "ISO88591", "LATIN1":
		e.setImport(unit, "golang.org/x/text/encoding/charmap", "charmap")
		return "charmap.ISO8859_1.NewEncoder()"
	case "WINDOWS1252", "CP1252":
		e.setImport(unit, "golang.org/x/text/encoding/charmap", "charmap")
		return "charmap.Windows1252.NewEncoder()"
	default:
		return e.unsupportedEncodingEncoder(unit, enc)
	}
}

// inferInstanceType determines the Go type for an instance, using the same logic
// as the instance field declaration. This ensures instance references in expressions
// use the same type as the instance getter's return type.
func (e *Emitter) inferInstanceType(typ *engine.ExprValue) string {
	if typ == nil || typ.Kind != engine.InstanceKind || typ.Instance == nil {
		return e.declType(typ)
	}
	instType := e.declType(typ)
	inst := typ.Instance
	// If the instance has an enum annotation, use the enum type
	if inst.Enum != "" {
		enumType := engine.ResolveTypeOfExpr(e.context, expr.MustParseExpr(inst.Enum))
		if enumType != nil {
			return e.declType(enumType)
		}
	}
	isValueOnlyDefault := inst.Value != nil && instType == "[]byte"
	if inst.Value != nil && (instType == "[]byte" || instType == "") {
		// For ternary expressions, check both branches for type consistency
		if tern, ok := inst.Value.Root.(expr.TernaryNode); ok {
			bResult := engine.ResultTypeOfNode(e.context, tern.B)
			cResult := engine.ResultTypeOfNode(e.context, tern.C)
			var bType, cType string
			if bResult != nil {
				if bResult.Kind == engine.InstanceKind {
					bType = e.inferInstanceType(bResult)
				} else {
					bType = e.declType(bResult)
				}
			}
			if cResult != nil {
				if cResult.Kind == engine.InstanceKind {
					cType = e.inferInstanceType(cResult)
				} else {
					cType = e.declType(cResult)
				}
			}
			if bType != "" && bType != "[]byte" && cType != "" && cType != "[]byte" {
				if bType != cType {
					instType = "any"
				} else {
					instType = bType
				}
				goto applyModifiers
			} else if bType != "" && bType != "[]byte" {
				instType = bType
				goto applyModifiers
			} else if cType != "" && cType != "[]byte" {
				instType = cType
				goto applyModifiers
			}
		}
		// Infer from expression result
		result := engine.ResultTypeOfExpr(e.context, inst.Value)
		if result != nil {
			valType := result
			if valType.Kind == engine.MethodKind && valType.Method != nil {
				retVT := valType.Method.ReturnType
				if retVT.Type.TypeRef != nil {
					if inferred := e.declTypeRef(retVT.Type.TypeRef, retVT.Repeat); inferred != "" && inferred != "[]byte" {
						instType = inferred
						goto applyModifiers
					}
				}
			} else if valType.Kind == engine.InstanceKind {
				// Recursively infer the type of the referenced instance
				if inferred := e.inferInstanceType(valType); inferred != "" && inferred != "[]byte" {
					instType = inferred
					goto applyModifiers
				}
			} else {
				if inferred := e.declType(valType); inferred != "" && inferred != "[]byte" {
					// Check for uint64 overflow on int literals
					if inferred == "int" {
						if intNode, ok := inst.Value.Root.(expr.IntNode); ok {
							if intNode.Integer.Cmp(big.NewInt(0)) >= 0 && !intNode.Integer.IsInt64() {
								instType = "uint64"
								goto applyModifiers
							}
						}
					}
					instType = inferred
					goto applyModifiers
				}
			}
		}
		// Infer from ternary
		if tern, ok := inst.Value.Root.(expr.TernaryNode); ok {
			bResult := engine.ResultTypeOfNode(e.context, tern.B)
			if bResult != nil {
				if bt := e.declType(bResult); bt != "" && bt != "[]byte" {
					return bt
				}
			}
		}
		// Infer from AST
		switch root := inst.Value.Root.(type) {
		case expr.IntNode:
			if root.Integer.Cmp(big.NewInt(0)) >= 0 && !root.Integer.IsInt64() {
				return "uint64"
			}
			return "int"
		case expr.BoolNode:
			return "bool"
		case expr.FloatNode:
			return "float64"
		case expr.StringNode:
			return "string"
		case expr.BinaryNode:
			switch root.Op {
			case expr.OpEqual, expr.OpNotEqual, expr.OpLessThan, expr.OpLessThanEqual,
				expr.OpGreaterThan, expr.OpGreaterThanEqual, expr.OpLogicalAnd, expr.OpLogicalOr:
				return "bool"
			default:
				return "int"
			}
		case expr.UnaryNode:
			if root.Op == expr.OpLogicalNot {
				return "bool"
			}
			return "int"
		case expr.ArrayNode:
			// Check if all items fit in a byte
			allBytes := true
			hasFloat := false
			hasString := false
			hasStruct := false
			var firstStructType string
			allSameStruct := true
			for _, item := range root.Items {
				if intNode, ok := item.(expr.IntNode); ok {
					if intNode.Integer.Sign() < 0 || intNode.Integer.Cmp(big.NewInt(255)) > 0 {
						allBytes = false
					}
				} else if _, ok := item.(expr.FloatNode); ok {
					allBytes = false
					hasFloat = true
				} else if _, ok := item.(expr.StringNode); ok {
					allBytes = false
					hasString = true
				} else {
					allBytes = false
					// Check if element resolves to a struct type
					elemResult := engine.ResultTypeOfNode(e.context, item)
					if elemResult != nil && (elemResult.Kind == engine.AttrKind || elemResult.Kind == engine.InstanceKind || elemResult.Kind == engine.StructKind) {
						hasStruct = true
						dt := e.declType(elemResult)
						if firstStructType == "" {
							firstStructType = dt
						} else if dt != firstStructType {
							allSameStruct = false
						}
					}
				}
			}
			if allBytes {
				return "[]byte"
			}
			if hasFloat {
				return "[]float64"
			}
			if hasString {
				return "[]string"
			}
			if hasStruct {
				if allSameStruct && firstStructType != "" {
					return "[]" + firstStructType
				}
				return "[]any"
			}
			return "[]int"
		case expr.TernaryNode:
			// Infer from both branches - if they differ, use any
			bResult := engine.ResultTypeOfNode(e.context, root.B)
			cResult := engine.ResultTypeOfNode(e.context, root.C)
			var bType, cType string
			if bResult != nil {
				if bResult.Kind == engine.InstanceKind {
					bType = e.inferInstanceType(bResult)
				} else {
					bType = e.declType(bResult)
				}
			}
			if cResult != nil {
				if cResult.Kind == engine.InstanceKind {
					cType = e.inferInstanceType(cResult)
				} else {
					cType = e.declType(cResult)
				}
			}
			if bType != "" && bType != "[]byte" {
				if cType != "" && cType != "[]byte" && cType != bType {
					// Different types - use any
					instType = "any"
				} else {
					instType = bType
				}
				goto applyModifiers
			}
			if cType != "" && cType != "[]byte" {
				instType = cType
				goto applyModifiers
			}
			return instType
		case expr.SubscriptNode:
			// Infer element type from the array operand
			arrResult := engine.ResultTypeOfNode(e.context, root.A)
			if arrResult != nil {
				if arrResult.Kind == engine.InstanceKind {
					// Get the array instance type and strip the [] prefix
					arrInstType := e.inferInstanceType(arrResult)
					if strings.HasPrefix(arrInstType, "[]") {
						return arrInstType[2:] // element type
					}
				}
			}
			return "int"
		case expr.CallNode:
			// Method calls - try to infer from return type
			callResult := engine.ResultTypeOfNode(e.context, root)
			if callResult != nil {
				if inferred := e.declType(callResult); inferred != "" && inferred != "[]byte" {
					instType = inferred
					goto applyModifiers
				}
			}
			return instType
		case expr.MemberNode:
			// For cross-file instance access, try to resolve via the struct's type context
			if v := engine.ResultTypeOfNode(e.context, root); v != nil && v.Kind == engine.InstanceKind {
				if inferred := e.inferInstanceType(v); inferred != "" && inferred != "[]byte" {
					instType = inferred
					goto applyModifiers
				}
			}
			// Handle array intrinsics (first, last, min, max) - return element type
			switch root.Property {
			case "first", "last", "min", "max":
				arrResult := engine.ResultTypeOfNode(e.context, root.Operand)
				if arrResult != nil {
					if arrResult.Kind == engine.InstanceKind {
						arrInstType := e.inferInstanceType(arrResult)
						if strings.HasPrefix(arrInstType, "[]") {
							instType = arrInstType[2:]
							goto applyModifiers
						}
					}
				}
			}
			// Try to infer from member expression result
			memberResult := engine.ResultTypeOfNode(e.context, root)
			if memberResult != nil {
				if memberResult.Kind == engine.InstanceKind {
					// Recursively infer the instance type
					if inferred := e.inferInstanceType(memberResult); inferred != "" && inferred != "[]byte" {
						instType = inferred
						goto applyModifiers
					}
				}
			}
			if isValueOnlyDefault {
				return "int"
			}
			return instType
		case expr.IdentNode:
			if isValueOnlyDefault {
				return "int"
			}
			return instType
		case expr.CastNode:
			// .as<type> cast - extract the target type
			ref, err := types.ParseTypeRef(root.TypeName)
			if err == nil {
				if goType := e.declTypeRef(&ref, nil); goType != "" && goType != "[]byte" {
					instType = goType
					goto applyModifiers
				}
			}
			return instType
		}
		return "int"
	}
applyModifiers:
	// For struct types, ensure pointer to avoid recursive type definitions
	// But don't add pointer to interface types (switch types ending in _Cases)
	if len(instType) > 0 && instType[0] >= 'A' && instType[0] <= 'Z' && !strings.HasPrefix(instType, "*") {
		instType = "*" + instType
	}
	// For repeated instances, wrap in slice (but only if not already wrapped by declType)
	if inst.Repeat != nil && !strings.HasPrefix(instType, "[]") {
		instType = "[]" + instType
	}
	return instType
}

func (e *Emitter) exprMethod(t expr.MemberNode, v *engine.ExprValue) string {
	operand := e.exprNode(t.Operand)
	method := v.Method
	switch method.Method {
	case engine.MethodIntToString:
		e.needFmt = true
		return fmt.Sprintf("fmt.Sprintf(\"%%d\", %s)", operand)
	case engine.MethodFloatToInt:
		return fmt.Sprintf("int(%s)", operand)
	case engine.MethodByteArrayLength:
		return fmt.Sprintf("len(%s)", operand)
	case engine.MethodStringLength:
		return fmt.Sprintf("len([]rune(%s))", operand)
	case engine.MethodStringReverse:
		return fmt.Sprintf("(func() string { r := []rune(%s); for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 { r[i], r[j] = r[j], r[i] }; return string(r) }())", operand)
	case engine.MethodBoolToInt:
		return fmt.Sprintf("(func() int { if %s { return 1 }; return 0 }())", operand)
	case engine.MethodEnumToInt:
		return fmt.Sprintf("int(%s)", operand)
	case engine.MethodStreamEOF:
		return fmt.Sprintf("(func() bool { v, err := %s.EOF(); if err != nil { panic(err) }; return v }())", operand)
	case engine.MethodStreamSize:
		return fmt.Sprintf("(func() int { v, err := %s.Size(); if err != nil { panic(err) }; return int(v) }())", operand)
	case engine.MethodStreamPos:
		return fmt.Sprintf("(func() int { v, err := %s.Pos(); if err != nil { panic(err) }; return int(v) }())", operand)
	case engine.MethodArrayFirst:
		return fmt.Sprintf("(%s)[0]", operand)
	case engine.MethodArrayLast:
		return fmt.Sprintf("(%s)[len(%s)-1]", operand, operand)
	case engine.MethodArraySize:
		return fmt.Sprintf("len(%s)", operand)
	case engine.MethodByteArrayToString:
		// .to_s without args on byte array - assume UTF-8
		return fmt.Sprintf("string(%s)", operand)
	case engine.MethodStringToInt:
		// .to_i without args on string - parse as decimal
		e.needStrconv = true
		e.needStrings = true
		return fmt.Sprintf("(func() int { v, err := strconv.ParseInt(strings.TrimSpace(%s), 10, 0); if err != nil { panic(err) }; return int(v) }())", operand)
	case engine.MethodStringSubstring:
		// .substring without args doesn't make sense, return identity
		return operand
	case engine.MethodArrayMin:
		retType := "int"
		if method.ReturnType.Type.TypeRef != nil {
			if dt := e.declTypeRef(method.ReturnType.Type.TypeRef, nil); dt != "" {
				retType = dt
			}
		}
		return fmt.Sprintf("(func() %s { var m %s = %s((%s)[0]); for _, v := range (%s)[1:] { if %s(v) < m { m = %s(v) } }; return m }())", retType, retType, retType, operand, operand, retType, retType)
	case engine.MethodArrayMax:
		retType := "int"
		if method.ReturnType.Type.TypeRef != nil {
			if dt := e.declTypeRef(method.ReturnType.Type.TypeRef, nil); dt != "" {
				retType = dt
			}
		}
		return fmt.Sprintf("(func() %s { var m %s = %s((%s)[0]); for _, v := range (%s)[1:] { if %s(v) > m { m = %s(v) } }; return m }())", retType, retType, retType, operand, operand, retType, retType)
	default:
		panic(fmt.Errorf("unsupported method %s on %s", method.Method, t))
	}
}

// isByteArrayComparison checks if a binary comparison involves byte arrays
// (which require bytes.Equal in Go instead of == or !=)
func (e *Emitter) isByteArrayComparison(a, b expr.Node) bool {
	aIsBytes := e.isNodeByteArray(a)
	bIsBytes := e.isNodeByteArray(b)
	// Only use bytes.Equal when both sides are byte-like
	return aIsBytes && bIsBytes
}

// hasEnumScopeOperand checks if either operand is an enum scope expression
// (like enum_0::animal::chicken) which produces a named-int type in Go.
func (e *Emitter) hasEnumScopeOperand(a, b expr.Node) bool {
	return isEnumScopeExpr(a) || isEnumScopeExpr(b)
}

func isEnumScopeExpr(n expr.Node) bool {
	switch n := n.(type) {
	case expr.ScopeNode:
		return true
	case expr.TernaryNode:
		return isEnumScopeExpr(n.B) || isEnumScopeExpr(n.C)
	}
	return false
}

func (e *Emitter) isNodeByteArray(n expr.Node) bool {
	if _, ok := n.(expr.ArrayNode); ok {
		return true
	}
	result := engine.ResultTypeOfNode(e.context, n)
	if result != nil {
		// For instances, use inferInstanceType which does expression-based inference
		if result.Kind == engine.InstanceKind {
			inferred := e.inferInstanceType(result)
			return inferred == "[]byte"
		}
		vt, ok := result.ValueType()
		if ok && vt.Type.TypeRef != nil && vt.Type.TypeRef.Kind == types.Bytes {
			return true
		}
		// Check method return type (e.g. records.last on [][]byte returns []byte)
		if result.Kind == engine.MethodKind && result.Method != nil {
			ret := result.Method.ReturnType
			if ret.Type.TypeRef != nil && ret.Type.TypeRef.Kind == types.Bytes {
				return true
			}
		}
	}
	return false
}

func (e *Emitter) exprTernaryNode(t expr.TernaryNode) string {
	cast := e.exprPromotionTernaryNode(t)
	if cast == "" || cast == "[]byte" {
		// Infer type from both branches; use any if they differ
		bResult := engine.ResultTypeOfNode(e.context, t.B)
		cResult := engine.ResultTypeOfNode(e.context, t.C)
		var bType, cType string
		if bResult != nil {
			if bResult.Kind == engine.InstanceKind {
				bType = e.inferInstanceType(bResult)
			} else {
				bType = e.declType(bResult)
			}
		}
		if cResult != nil {
			if cResult.Kind == engine.InstanceKind {
				cType = e.inferInstanceType(cResult)
			} else {
				cType = e.declType(cResult)
			}
		}
		if bType != "" && cType != "" && bType != cType {
			cast = "any"
		} else if bType != "" {
			cast = bType
		} else if cType != "" {
			cast = cType
		}
		if cast == "" {
			cast = "any"
		}
	}
	// If the cast is int but a branch contains a uint64-range literal,
	// promote to uint64 to avoid overflow
	if cast == "int" && (nodeHasUint64Literal(t.B) || nodeHasUint64Literal(t.C)) {
		cast = "uint64"
	}
	// Go does not have a conditional expression. We use an inline function.
	return fmt.Sprintf("(func() (%s) { if (%s) { return (%s)(%s) } else { return (%s)(%s) } }())",
		cast, e.exprNode(t.A), cast, e.exprNode(t.B), cast, e.exprNode(t.C))
}

func nodeHasUint64Literal(n expr.Node) bool {
	switch n := n.(type) {
	case expr.IntNode:
		return n.Integer.Cmp(big.NewInt(0)) >= 0 && !n.Integer.IsInt64()
	case expr.CastNode:
		return nodeHasUint64Literal(n.Operand)
	case expr.TernaryNode:
		return nodeHasUint64Literal(n.B) || nodeHasUint64Literal(n.C)
	}
	return false
}

// wrapAnyForArith wraps an expression in a runtime int conversion if it's any-typed
func (e *Emitter) wrapAnyForArith(node expr.Node, exprStr string) string {
	// Avoid double-wrapping if the expression was already converted
	if strings.HasPrefix(exprStr, "(func() int { switch v := (") {
		return exprStr
	}
	result := engine.ResultTypeOfNode(e.context, node)
	if result != nil {
		if result.Kind == engine.AttrKind && result.Attr != nil && result.Attr.Type.TypeSwitch != nil {
			// any-typed switch field - need runtime int extraction
			return fmt.Sprintf("(func() int { switch v := (%s).(type) { case int: return v; case int8: return int(v); case int16: return int(v); case int32: return int(v); case int64: return int(v); case uint8: return int(v); case uint16: return int(v); case uint32: return int(v); case uint64: return int(v); default: return 0 } }())", exprStr)
		}
	}
	return exprStr
}

func (e *Emitter) wrapCalcIntResult(node expr.BinaryNode, exprStr string) string {
	result := engine.ResultTypeOfNode(e.context, node)
	if result == nil || e.declType(result) != "int" {
		return exprStr
	}
	// 32-bit truncation is only applied in compatibility mode.
	if e.context.Compat.HasCalcIntTypeTruncationBug() {
		return fmt.Sprintf("int(int32(%s))", exprStr)
	}
	return exprStr
}

func (e *Emitter) exprBinaryNode(t expr.BinaryNode) string {
	cast := e.exprPromotionBinaryNode(t)
	if cast != "" {
		cast = "(" + cast + ")"
	}
	// For arithmetic, wrap any-typed operands in runtime int conversion
	aExpr := e.wrapAnyForArith(t.A, e.exprNode(t.A))
	bExpr := e.wrapAnyForArith(t.B, e.exprNode(t.B))
	switch t.Op {
	case expr.OpAdd:
		return e.wrapCalcIntResult(t, fmt.Sprintf("%s(%s) + %s(%s)", cast, aExpr, cast, bExpr))
	case expr.OpSub:
		return e.wrapCalcIntResult(t, fmt.Sprintf("%s(%s) - %s(%s)", cast, aExpr, cast, bExpr))
	case expr.OpMult:
		return e.wrapCalcIntResult(t, fmt.Sprintf("%s(%s) * %s(%s)", cast, aExpr, cast, bExpr))
	case expr.OpDiv:
		// Kaitai uses floor division (Python-style), not truncation division (Go-style)
		return e.wrapCalcIntResult(t, fmt.Sprintf("(func() int { a, b := int(%s(%s)), int(%s(%s)); d := a / b; if (a%%b != 0) && ((a^b) < 0) { d-- }; return d }())", cast, aExpr, cast, bExpr))
	case expr.OpMod:
		// Kaitai uses modulo with floor semantics (Python-style)
		return e.wrapCalcIntResult(t, fmt.Sprintf("(func() int { a, b := int(%s(%s)), int(%s(%s)); return a - b*(func() int { d := a / b; if (a%%b != 0) && ((a^b) < 0) { d-- }; return d }()) }())", cast, aExpr, cast, bExpr))
	case expr.OpLessThan:
		if e.isByteArrayComparison(t.A, t.B) {
			e.needBytes = true
			return fmt.Sprintf("bytes.Compare(%s, %s) < 0", aExpr, bExpr)
		}
		return fmt.Sprintf("%s(%s) < %s(%s)", cast, aExpr, cast, bExpr)
	case expr.OpLessThanEqual:
		if e.isByteArrayComparison(t.A, t.B) {
			e.needBytes = true
			return fmt.Sprintf("bytes.Compare(%s, %s) <= 0", aExpr, bExpr)
		}
		return fmt.Sprintf("%s(%s) <= %s(%s)", cast, aExpr, cast, bExpr)
	case expr.OpGreaterThan:
		if e.isByteArrayComparison(t.A, t.B) {
			e.needBytes = true
			return fmt.Sprintf("bytes.Compare(%s, %s) > 0", aExpr, bExpr)
		}
		return fmt.Sprintf("%s(%s) > %s(%s)", cast, aExpr, cast, bExpr)
	case expr.OpGreaterThanEqual:
		if e.isByteArrayComparison(t.A, t.B) {
			e.needBytes = true
			return fmt.Sprintf("bytes.Compare(%s, %s) >= 0", aExpr, bExpr)
		}
		return fmt.Sprintf("%s(%s) >= %s(%s)", cast, aExpr, cast, bExpr)
	case expr.OpEqual:
		if e.isByteArrayComparison(t.A, t.B) {
			e.needBytes = true
			return fmt.Sprintf("bytes.Equal(%s, %s)", aExpr, bExpr)
		}
		// When comparing values of potentially different types (e.g., int vs enum),
		// ensure both sides have a common type. Enum scope expressions (like
		// enum_0::animal::chicken) produce named-int types that don't match plain int.
		if cast == "" && e.hasEnumScopeOperand(t.A, t.B) {
			cast = "(int)"
		}
		return fmt.Sprintf("%s(%s) == %s(%s)", cast, aExpr, cast, bExpr)
	case expr.OpNotEqual:
		if e.isByteArrayComparison(t.A, t.B) {
			e.needBytes = true
			return fmt.Sprintf("!bytes.Equal(%s, %s)", aExpr, bExpr)
		}
		if cast == "" && e.hasEnumScopeOperand(t.A, t.B) {
			cast = "(int)"
		}
		return fmt.Sprintf("%s(%s) != %s(%s)", cast, aExpr, cast, bExpr)
	case expr.OpShiftLeft:
		return e.wrapCalcIntResult(t, fmt.Sprintf("%s(%s) << %s(%s)", cast, aExpr, cast, bExpr))
	case expr.OpShiftRight:
		return e.wrapCalcIntResult(t, fmt.Sprintf("%s(%s) >> %s(%s)", cast, aExpr, cast, bExpr))
	case expr.OpBitAnd:
		return e.wrapCalcIntResult(t, fmt.Sprintf("%s(%s) & %s(%s)", cast, aExpr, cast, bExpr))
	case expr.OpBitOr:
		return e.wrapCalcIntResult(t, fmt.Sprintf("%s(%s) | %s(%s)", cast, aExpr, cast, bExpr))
	case expr.OpBitXor:
		return e.wrapCalcIntResult(t, fmt.Sprintf("%s(%s) ^ %s(%s)", cast, aExpr, cast, bExpr))
	case expr.OpLogicalAnd:
		return fmt.Sprintf("%s(%s) && %s(%s)", cast, aExpr, cast, bExpr)
	case expr.OpLogicalOr:
		return fmt.Sprintf("%s(%s) || %s(%s)", cast, aExpr, cast, bExpr)
	default:
		panic(fmt.Errorf("unsupported binary op %s", t.Op))
	}
}

func (e *Emitter) exprNode(node expr.Node) string {
	switch t := node.(type) {
	case expr.UnaryNode:
		switch t.Op {
		case expr.OpLogicalNot:
			return "!(" + e.exprNode(t.Operand) + ")"
		case expr.OpNegate:
			return "-(" + e.exprNode(t.Operand) + ")"
		case expr.OpInvert:
			return "^(int(" + e.exprNode(t.Operand) + "))"
		default:
			panic(fmt.Errorf("unsupported unary op node %s", t.Op))
		}
	case expr.IdentNode:
		// Handle _sizeof intrinsic - sizeof current struct
		if t.Identifier == "_sizeof" {
			ks := e.context.Struct()
			if ks != nil {
				sz := e.computeStructSize(ks)
				if sz >= 0 {
					return fmt.Sprintf("%d", sz)
				}
			}
			panic(fmt.Errorf("unable to compute _sizeof for current struct"))
		}
		v := engine.ResultTypeOfNode(e.context, t)
		switch v {
		case nil:
			panic(fmt.Errorf("unable to use nested ident subexpression %s as value", t))
		case e.context.RootValue():
			e.needRoot = true
			return "_root"
		case e.context.ParentValue():
			e.needParent = true
			return "_parent"
		case e.context.StreamValue():
			return "stream"
		}
		// Handle _index special variable
		if t.Identifier == "_index" {
			return "i"
		}
		switch v.Kind {
		case engine.ParamKind:
			return fmt.Sprintf("this.%s", e.fieldName(v.Param.ID))
		case engine.AttrKind:
			fieldExpr := fmt.Sprintf("this.%s", e.fieldName(v.Attr.ID))
			// If the attr is conditional (has if:) and is typed as any,
			// insert a type assertion to the underlying type
			if v.Attr.If != nil && v.Attr.Type.TypeRef != nil {
				declType := e.declTypeRef(v.Attr.Type.TypeRef, nil)
				if needsPointerForNil(declType) {
					return fmt.Sprintf("%s.(%s)", fieldExpr, declType)
				}
			}
			// If the attr is a switch type (any) with only integer cases,
			// wrap with runtime int extraction so it can be used in size
			// expressions, comparisons, etc.
			if v.Attr.Type.TypeSwitch != nil && isIntegerOnlySwitch(v.Attr.Type.TypeSwitch) {
				return e.wrapAnyForArith(t, fieldExpr)
			}
			return fieldExpr
		case engine.InstanceKind:
			// Instance access: call the getter and use the value. In write
			// expressions, positioned parse instances must also be materialized
			// back to their target offsets as soon as the expression observes
			// them (for example, repeat-expr can deliberately invoke a
			// positioned instance before sequential fields are written).
			retType := e.inferInstanceType(v)
			writeSuffix := ""
			if e.inWriteExpr && v.Instance.Value == nil && (v.Instance.Pos != nil || v.Instance.IO != nil) {
				writeSuffix = fmt.Sprintf("; if err := this.write%s(wstream); err != nil { panic(err) }", e.typeName(v.Instance.ID))
			}
			// If the instance is conditional, the getter returns 'any'.
			// Cast back to the base type for use in expressions.
			if v.Instance.If != nil && needsPointerForNil(retType) {
				return fmt.Sprintf("(func() %s { v, err := this.%s(); if err != nil { panic(err) }%s; return v.(%s) }())",
					retType, e.fieldName(v.Instance.ID), writeSuffix, retType)
			}
			// Cast return value to handle cases where the getter returns a
			// named type (e.g., enum) but the inferred type is its underlying type.
			return fmt.Sprintf("(func() %s { v, err := this.%s(); if err != nil { panic(err) }%s; return (%s)(v) }())",
				retType, e.fieldName(v.Instance.ID), writeSuffix, retType)
		case engine.AliasKind:
			return v.Alias.Target
		default:
			panic(fmt.Errorf("unsupported value type reference %s in ident subexpression %s", v.Kind, t))
		}
	case expr.ScopeNode:
		v := engine.ResultTypeOfNode(e.context, t)
		if v == nil {
			panic(fmt.Errorf("unable to use primary scope expression %s as value", t))
		}
		switch v.Kind {
		case engine.IntegerKind:
			if v.Parent != nil && v.Parent.Kind == engine.EnumValueKind {
				return e.enumValueName(e.parentStruct(v), e.parentEnum(v), v.Parent.EnumValue.ID)
			} else {
				panic(fmt.Errorf("unexpected constant in scope subexpression %s", t))
			}
		default:
			panic(fmt.Errorf("unsupported value type reference %s in scope subexpression %s", v.Kind, t))
		}
	case expr.MemberNode:
		switch t.Property {
		case "_root":
			if e.isOperandAnyTyped(t.Operand) {
				return fmt.Sprintf("(%s).(interface{ KS_Root() any }).KS_Root()", e.exprNode(t.Operand))
			}
			return fmt.Sprintf("%s.Root_", e.exprNode(t.Operand))
		case "_parent":
			if e.isOperandAnyTyped(t.Operand) {
				return fmt.Sprintf("(%s).(interface{ KS_Parent() any }).KS_Parent()", e.exprNode(t.Operand))
			}
			return fmt.Sprintf("%s.Parent_", e.exprNode(t.Operand))
		case "_io":
			if e.isOperandAnyTyped(t.Operand) {
				return fmt.Sprintf("(%s).(interface{ KS_IO() *%s }).KS_IO()", e.exprNode(t.Operand), kaitaiStream)
			}
			return fmt.Sprintf("%s.IO_", e.exprNode(t.Operand))
		case "_sizeof":
			// field._sizeof - compute sizeof the specific field
			operandResult := engine.ResultTypeOfNode(e.context, t.Operand)
			if operandResult != nil {
				switch operandResult.Kind {
				case engine.AttrKind:
					if operandResult.Attr != nil {
						sz := computeAttrSize(operandResult.Attr)
						if sz < 0 && operandResult.Attr.Type.TypeRef != nil && operandResult.Attr.Type.TypeRef.Kind == types.User {
							resolved := e.resolveType(operandResult.Attr.Type.TypeRef.User.Name)
							if resolved.Kind == engine.StructKind && resolved.Struct != nil {
								sz = e.computeStructSize(resolved.Struct.Type)
							}
						}
						if sz >= 0 {
							return fmt.Sprintf("%d", sz)
						}
					}
				case engine.StructKind:
					if operandResult.Struct != nil {
						sz := e.computeStructSize(operandResult.Struct.Type)
						if sz >= 0 {
							return fmt.Sprintf("%d", sz)
						}
					}
				}
			}
			panic(fmt.Errorf("unable to compute _sizeof for %s", t.Operand))
		}
		v := engine.ResultTypeOfNode(e.context, t)
		if v == nil {
			panic(fmt.Errorf("unable to use nested member subexpression %s as value", t))
		}
		switch v.Kind {
		case engine.ParamKind:
			return fmt.Sprintf("%s.%s", e.exprNode(t.Operand), e.fieldName(v.Param.ID))
		case engine.AttrKind:
			fieldExpr := fmt.Sprintf("%s.%s", e.exprNode(t.Operand), e.fieldName(v.Attr.ID))
			// If the attr is conditional (if: expr), its Go field type is 'any'.
			// Add a type assertion to recover the underlying type for use in expressions.
			if v.Attr.If != nil {
				goType := e.declType(v)
				if needsPointerForNil(goType) && goType != "" {
					return fmt.Sprintf("%s.(%s)", fieldExpr, goType)
				}
			}
			return fieldExpr
		case engine.InstanceKind:
			retType := e.inferInstanceType(v)
			operand := e.exprNode(t.Operand)
			writeSuffix := ""
			if e.inWriteExpr && v.Instance.Value == nil && (v.Instance.Pos != nil || v.Instance.IO != nil) {
				writeSuffix = fmt.Sprintf("; if err := %s.write%s(wstream); err != nil { panic(err) }", operand, e.typeName(v.Instance.ID))
			}
			if v.Instance.If != nil && needsPointerForNil(retType) {
				return fmt.Sprintf("(func() %s { v, err := %s.%s(); if err != nil { panic(err) }%s; return v.(%s) }())",
					retType, operand, e.fieldName(v.Instance.ID), writeSuffix, retType)
			}
			return fmt.Sprintf("(func() %s { v, err := %s.%s(); if err != nil { panic(err) }%s; return (%s)(v) }())",
				retType, operand, e.fieldName(v.Instance.ID), writeSuffix, retType)
		case engine.MethodKind:
			return e.exprMethod(t, v)
		default:
			panic(fmt.Errorf("unsupported value type reference %s in nested member subexpression %s", v.Kind, t))
		}
	case expr.IntNode:
		s := t.Integer.String()
		// Check if value overflows int64 - needs uint64 cast
		if t.Integer.Cmp(big.NewInt(0)) >= 0 && !t.Integer.IsInt64() {
			return fmt.Sprintf("uint64(%s)", s)
		}
		return s
	case expr.FloatNode:
		return t.Float.Text('f', -1)
	case expr.BoolNode:
		return t.String()
	case expr.BinaryNode:
		return e.exprBinaryNode(t)
	case expr.TernaryNode:
		return e.exprTernaryNode(t)
	case expr.StringNode:
		return strconv.Quote(t.Str)
	case expr.ArrayNode:
		// Determine if this is a byte array (all items are small integers)
		// or a general int array
		allBytes := true
		for _, item := range t.Items {
			if intNode, ok := item.(expr.IntNode); ok {
				if intNode.Integer.Sign() < 0 || intNode.Integer.Cmp(big.NewInt(255)) > 0 {
					allBytes = false
					break
				}
			} else {
				allBytes = false
				break
			}
		}
		typ := "byte"
		if !allBytes {
			typ = "int"
			// Determine element type from all items - if heterogeneous, use any
			if len(t.Items) > 0 {
				var firstType string
				allSame := true
				for _, item := range t.Items {
					result := engine.ResultTypeOfNode(e.context, item)
					if result != nil {
						dt := e.declType(result)
						if dt == "" {
							dt = "int"
						}
						if firstType == "" {
							firstType = dt
						} else if dt != firstType {
							allSame = false
						}
					}
				}
				if firstType != "" {
					if allSame {
						typ = firstType
					} else {
						typ = "any"
					}
				}
			}
		}
		b := strings.Builder{}
		b.WriteString("[]")
		b.WriteString(typ)
		b.WriteString("{")
		for i, item := range t.Items {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(e.exprNode(item))
		}
		b.WriteString("}")
		return b.String()
	case expr.SubscriptNode:
		return fmt.Sprintf("(%s)[%s]", e.exprNode(t.A), e.exprNode(t.B))
	case expr.CallNode:
		// Method call: the object is typically a MemberNode
		// Check if the object is a MemberNode for method dispatch
		if mn, ok := t.Object.(expr.MemberNode); ok {
			v := engine.ResultTypeOfNode(e.context, t.Object)
			if v != nil && v.Kind == engine.MethodKind {
				operand := e.exprNode(mn.Operand)
				method := v.Method
				switch method.Method {
				case engine.MethodByteArrayToString:
					if len(t.Args) > 0 {
						enc, ok := stringLiteralValue(t.Args[0])
						if !ok {
							panic(fmt.Errorf(".to_s encoding argument must be a string literal, got %s", t.Args[0]))
						}
						if e.needsEncodingConversion(enc) {
							decoder := e.encodingDecoder(e.currentUnit, enc)
							e.setImport(e.currentUnit, kaitaiRuntimePackagePath, kaitaiRuntimePackageName)
							return fmt.Sprintf("(func() string { s, err := kaitai.BytesToStr(%s, %s); if err != nil { panic(err) }; return s }())", operand, decoder)
						}
						return fmt.Sprintf("string(%s)", operand)
					}
					return fmt.Sprintf("string(%s)", operand)
				case engine.MethodStringSubstring:
					if len(t.Args) >= 2 {
						return fmt.Sprintf("string([]rune(%s)[%s:%s])", operand, e.exprNode(t.Args[0]), e.exprNode(t.Args[1]))
					}
				case engine.MethodStringToInt:
					e.needStrconv = true
					e.needStrings = true
					if len(t.Args) > 0 {
						return fmt.Sprintf("(func() int { v, err := strconv.ParseInt(strings.TrimSpace(%s), %s, 0); if err != nil { panic(err) }; return int(v) }())", operand, e.exprNode(t.Args[0]))
					}
					return fmt.Sprintf("(func() int { v, err := strconv.ParseInt(strings.TrimSpace(%s), 10, 0); if err != nil { panic(err) }; return int(v) }())", operand)
				case engine.MethodIntToString:
					e.needFmt = true
					if len(t.Args) > 0 {
						e.needStrconv = true
						return fmt.Sprintf("strconv.FormatInt(int64(%s), %s)", operand, e.exprNode(t.Args[0]))
					}
					return fmt.Sprintf("fmt.Sprintf(\"%%d\", %s)", operand)
				}
			}
		}
		// Fallback: just generate the call
		panic(fmt.Errorf("unsupported call expression: %s", t))
	case expr.CastNode:
		operand := e.exprNode(t.Operand)
		// Handle built-in type casts
		switch t.TypeName {
		case "bytes":
			// Cast array to []byte - if operand is already []byte, just return it
			// For array nodes, generate element-wise byte conversion
			if arrNode, ok := t.Operand.(expr.ArrayNode); ok {
				b := strings.Builder{}
				b.WriteString("[]byte{")
				for i, item := range arrNode.Items {
					if i > 0 {
						b.WriteString(", ")
					}
					fmt.Fprintf(&b, "byte(%s)", e.exprNode(item))
				}
				b.WriteString("}")
				return b.String()
			}
			return fmt.Sprintf("(func() []byte { src := %s; dst := make([]byte, len(src)); for i, v := range src { dst[i] = byte(v) }; return dst }())", operand)
		case "str":
			return fmt.Sprintf("string(%s)", operand)
		case "u1", "u2", "u4", "u8", "s1", "s2", "s4", "s8":
			ref, err := types.ParseTypeRef(t.TypeName)
			if err == nil {
				goType := e.declTypeRef(&ref, nil)
				return fmt.Sprintf("(%s)(%s)", goType, operand)
			}
			return fmt.Sprintf("(%s)", operand)
		}
		// Handle array type casts like .as<u2[]>
		if strings.HasSuffix(t.TypeName, "[]") {
			baseType := t.TypeName[:len(t.TypeName)-2]
			ref, err := types.ParseTypeRef(baseType)
			if err == nil {
				goElemType := e.declTypeRef(&ref, nil)
				// Convert array to the target slice type
				if arrNode, ok := t.Operand.(expr.ArrayNode); ok {
					b := strings.Builder{}
					b.WriteString("[]" + goElemType + "{")
					for i, item := range arrNode.Items {
						if i > 0 {
							b.WriteString(", ")
						}
						fmt.Fprintf(&b, "%s(%s)", goElemType, e.exprNode(item))
					}
					b.WriteString("}")
					return b.String()
				}
				// Generic conversion from any slice
				return fmt.Sprintf("(func() []%s { src := %s; dst := make([]%s, len(src)); for i, v := range src { dst[i] = %s(v) }; return dst }())",
					goElemType, operand, goElemType, goElemType)
			}
		}
		// .as<type> cast: in Go, this is a type assertion
		typeName := strings.ReplaceAll(t.TypeName, "::", "__")
		goType := e.typeName(kaitai.Identifier(typeName))
		// Try to resolve the type to get the full prefixed name
		resolved := engine.ResultTypeOfNode(e.context, t)
		if resolved != nil && resolved.Kind == engine.StructKind {
			goType = e.prefix(resolved.DefParent) + e.typeName(resolved.Struct.Type.ID)
		}
		// Check if the operand is already a concrete type (not an interface)
		// If so, skip the type assertion (it's invalid on non-interface types).
		// But if the operand goes through any-typed parent/root chains,
		// the Go code produces 'any' even if the type engine knows the concrete type.
		operandResult := engine.ResultTypeOfNode(e.context, t.Operand)
		if operandResult != nil && !e.isOperandAnyTyped(t.Operand) {
			switch operandResult.Kind {
			case engine.StructKind, engine.StructRootKind:
				// Already a concrete struct type in generated code
				return operand
			case engine.InstanceKind:
				// Check if the instance resolves to a concrete struct type
				inferred := e.inferInstanceType(operandResult)
				if inferred != "" && inferred != "[]byte" && inferred != "any" {
					return operand
				}
			}
		}
		return fmt.Sprintf("(%s).(*%s)", operand, goType)
	case expr.SizeofNode:
		// sizeof<type> - compute the fixed byte size of a type at compile time,
		// rounded up from its bit size for types that are not byte-aligned.
		if bits, ok := engine.PrimitiveBitSize(t.TypeName); ok {
			return fmt.Sprintf("%d", (bits+7)/8)
		}
		if resolved := e.resolveQualifiedType(t.TypeName); resolved != nil {
			if bits := e.computeStructBitSize(resolved.Struct.Type); bits >= 0 {
				return fmt.Sprintf("%d", (bits+7)/8)
			}
		}
		panic(fmt.Errorf("sizeof<%s>: unable to compute size", t.TypeName))
	case expr.BitSizeofNode:
		// bitsizeof<type> - compute the fixed bit size of a type at compile time
		if bits, ok := engine.PrimitiveBitSize(t.TypeName); ok {
			return fmt.Sprintf("%d", bits)
		}
		if resolved := e.resolveQualifiedType(t.TypeName); resolved != nil {
			if bits := e.computeStructBitSize(resolved.Struct.Type); bits >= 0 {
				return fmt.Sprintf("%d", bits)
			}
		}
		panic(fmt.Errorf("bitsizeof<%s>: unable to compute size", t.TypeName))
	case expr.FStringNode:
		// f-string: concatenate literal parts with fmt.Sprintf for interpolated expressions
		if len(t.Parts) == 0 {
			return `""`
		}
		e.needFmt = true
		// Build fmt.Sprintf format string and args
		fmtStr := strings.Builder{}
		var args []string
		for _, part := range t.Parts {
			if part.Expr != nil {
				fmtStr.WriteString("%v")
				args = append(args, e.exprNode(part.Expr))
			} else {
				// Escape % signs in literal text
				fmtStr.WriteString(strings.ReplaceAll(part.Literal, "%", "%%"))
			}
		}
		if len(args) == 0 {
			// Pure literal f-string, no interpolation
			return strconv.Quote(fmtStr.String())
		}
		return fmt.Sprintf("fmt.Sprintf(%s, %s)", strconv.Quote(fmtStr.String()), strings.Join(args, ", "))
	default:
		panic(fmt.Errorf("unsupported expression node %T", t))
	}
}

// isOperandAnyTyped checks whether the operand expression would produce an
// any-typed value in generated Go code. When true, field access on the result
// requires an interface assertion (e.g., for ._parent or ._root).
// This checks the GENERATED code's type, not the type engine's view -
// the type engine may know more than what the Go code can express.
func (e *Emitter) isOperandAnyTyped(operand expr.Node) bool {
	result := engine.ResultTypeOfNode(e.context, operand)
	if result == nil {
		return true // unknown -> treat as any
	}

	// Check if this is a ternary expression - if both branches produce
	// different struct types, the generated code uses 'any'.
	if tern, ok := operand.(expr.TernaryNode); ok {
		bResult := engine.ResultTypeOfNode(e.context, tern.B)
		cResult := engine.ResultTypeOfNode(e.context, tern.C)
		if bResult != nil && cResult != nil {
			bType := e.declType(bResult)
			cType := e.declType(cResult)
			if bType != cType {
				return true
			}
		}
	}

	// Check if this is the _parent intrinsic - its Go type depends on Parent_ field type
	if ident, ok := operand.(expr.IdentNode); ok && ident.Identifier == "_parent" {
		// _parent is typed in Go only if the current struct has a typed Parent_ field
		if e.context.Struct() != nil {
			return e.parentGoType(e.context.Struct()) == ""
		}
		return true
	}

	// Check if this is a member access that goes through an any-typed chain
	if mn, ok := operand.(expr.MemberNode); ok {
		if mn.Property == "_parent" || mn.Property == "_root" {
			// ._parent or ._root on a value - the result type depends on
			// whether the operand struct has typed Parent_/Root_ fields.
			// If the operand itself is any-typed, the result is definitely any.
			if e.isOperandAnyTyped(mn.Operand) {
				return true
			}
			// The operand is concrete. Check if its Parent_/Root_ field is typed.
			if mn.Property == "_parent" {
				opResult := engine.ResultTypeOfNode(e.context, mn.Operand)
				if opResult != nil {
					st := opResult.Struct
					if st != nil && st.Type != nil {
						return e.parentGoType(st.Type) == ""
					}
				}
				return true
			}
			// _root is typed when rootTypeName is set
			return e.rootTypeName == ""
		}
	}

	switch result.Kind {
	case engine.StructKind, engine.StructRootKind:
		return false // concrete struct type -> can access fields directly
	case engine.AttrKind:
		// Attr access on a concrete struct - check if the attr is a concrete type
		if result.Attr != nil && result.Attr.Type.TypeRef != nil && result.Attr.Type.TypeRef.Kind == types.User {
			return false
		}
		if result.Attr != nil && result.Attr.If != nil {
			return true // conditional attrs are any-typed
		}
		return false
	case engine.ParamKind:
		if result.Param != nil && result.Param.Type.Kind == types.User {
			return false
		}
		return false
	case engine.InstanceKind:
		// Instance access generates a typed closure. Check if the instance type
		// resolves to a concrete struct.
		inferred := e.inferInstanceType(result)
		if inferred != "" && inferred != "any" && inferred != "[]byte" {
			return false
		}
		return true
	case engine.StructParentKind:
		return true // parent kind values come from navigating chains -> may be any
	default:
		return true
	}
}
