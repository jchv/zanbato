package c

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/types"
)

func bitAlignFieldName(slot engine.BitAlignSlot) string {
	return "_align_" + strconv.Itoa(slot.Index)
}

func readCallForKind(k types.Kind) string {
	switch k {
	case types.U1:
		return "zb_read_u1"
	case types.U2le:
		return "zb_read_u2le"
	case types.U2be:
		return "zb_read_u2be"
	case types.U4le:
		return "zb_read_u4le"
	case types.U4be:
		return "zb_read_u4be"
	case types.U8le:
		return "zb_read_u8le"
	case types.U8be:
		return "zb_read_u8be"
	case types.S1:
		return "zb_read_s1"
	case types.S2le:
		return "zb_read_s2le"
	case types.S2be:
		return "zb_read_s2be"
	case types.S4le:
		return "zb_read_s4le"
	case types.S4be:
		return "zb_read_s4be"
	case types.S8le:
		return "zb_read_s8le"
	case types.S8be:
		return "zb_read_s8be"
	case types.F4le:
		return "zb_read_f4le"
	case types.F4be:
		return "zb_read_f4be"
	case types.F8le:
		return "zb_read_f8le"
	case types.F8be:
		return "zb_read_f8be"
	}
	return ""
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func byteArrayInitializer(b []byte) string {
	parts := make([]string, len(b))
	for i, c := range b {
		parts[i] = fmt.Sprintf("0x%02x", c)
	}
	return strings.Join(parts, ", ")
}

func filepathBase(name string) string {
	i := strings.LastIndexByte(name, '/')
	if i < 0 {
		return name
	}
	return name[i+1:]
}

func collectAllStructs(v *engine.ExprValue, fn func(*engine.ExprValue)) {
	if v == nil || v.Struct == nil {
		return
	}
	fn(v)
	for _, ch := range v.Struct.Structs {
		collectAllStructs(ch, fn)
	}
}

func conditionalInclude(include bool) string {
	if include {
		return " + 1"
	}
	return ""
}

func bitsAssignCast(width int) string {
	if width == 1 {
		return "(int)(_bits != 0)"
	}
	return "(uint64_t)_bits"
}

func emitSubstreamMem(src *buf, name, dataExpr, lenExpr string) {
	src.pf("zb_stream_t *%s = zb_substream_mem(arena, %s, %s);", name, dataExpr, lenExpr)
	src.pf("if (!%s) return ZB_ERR_ALLOC;", name)
}

func emitSubstreamMemBytes(src *buf, name, bytesVar string) {
	emitSubstreamMem(src, name, bytesVar+".data", bytesVar+".len")
}

func emitSubstreamConsume(src *buf, name, parent, lenExpr string) {
	src.pf("zb_stream_t *%s = zb_substream_view(arena, %s, zb_stream_pos(%s), %s);", name, parent, parent, lenExpr)
	src.pf("if (!%s) return ZB_ERR_ALLOC;", name)
	src.pf("ZB_TRY(zb_stream_seek(%s, zb_stream_pos(%s) + %s));", parent, parent, lenExpr)
}

func emitArenaNewLocal(src *buf, name, typeName string) {
	src.pf("%s_t *%s; ZB_NEW(arena, %s_t, %s);", typeName, name, typeName, name)
}

func emitArenaAllocLocal(src *buf, name, ctype string) {
	src.pf("%s *%s = (%s *)zb_arena_alloc(arena, sizeof(%s));", ctype, name, ctype, ctype)
	src.pf("if (!%s) return ZB_ERR_ALLOC;", name)
}

func emitArenaNewInto(src *buf, lvalue, typeName string) {
	src.pf("ZB_NEW(arena, %s_t, %s);", typeName, lvalue)
}

func emitBytesBoxInto(src *buf, lvalue, bytesExpr string) {
	src.pf("%s = zb_bytes_box(arena, %s);", lvalue, bytesExpr)
	src.pf("if (!%s) return ZB_ERR_ALLOC;", lvalue)
}

func emitTry(src *buf, format string, args ...any) {
	src.pf("ZB_TRY("+format+");", args...)
}

func bodyReferencesErr(body string) bool {
	const tok = "_err"
	idx := 0
	for {
		off := strings.Index(body[idx:], tok)
		if off < 0 {
			return false
		}
		start := idx + off
		end := start + len(tok)
		prev := byte(' ')
		if start > 0 {
			prev = body[start-1]
		}
		next := byte(' ')
		if end < len(body) {
			next = body[end]
		}
		if !isIdentByte(prev) && !isIdentByte(next) {
			return true
		}
		idx = end
	}
}

func isIdentByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func (e *Emitter) isImportedType(typ *engine.ExprValue) bool {
	if typ == nil || typ.Struct == nil || typ.Struct.Type == nil {
		return false
	}
	name := e.prefix(typ.DefParent) + e.typeName(typ.Struct.Type.ID)
	if name == e.file.rootName {
		return false
	}
	return !strings.HasPrefix(name, e.file.rootName+"_")
}

func (e *Emitter) userTypeParentRootArgs(typ *engine.ExprValue, parentDefault string) (string, string) {
	if e.isImportedType(typ) {
		return "NULL", "NULL"
	}
	return parentDefault, "this_->_root"
}

func isScalarType(t string) bool {
	if strings.HasSuffix(t, "*") {
		return true
	}
	switch t {
	case "uint8_t", "uint16_t", "uint32_t", "uint64_t",
		"int8_t", "int16_t", "int32_t", "int64_t",
		"int", "long", "size_t", "ptrdiff_t",
		"float", "double",
		"char":
		return true
	}
	return false
}
