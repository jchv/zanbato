package c

import (
	"fmt"
	"strings"
)

const indentStr = "  "

type buf struct {
	sb  strings.Builder
	pfx string
}

func (b *buf) indent() *buf { b.pfx += indentStr; return b }

func (b *buf) unindent() *buf { b.pfx = b.pfx[:len(b.pfx)-len(indentStr)]; return b }

func (b *buf) p(s string) *buf {
	b.sb.WriteString(b.pfx)
	b.sb.WriteString(s)
	b.sb.WriteByte('\n')
	return b
}

func (b *buf) pf(format string, args ...any) *buf {
	b.sb.WriteString(b.pfx)
	fmt.Fprintf(&b.sb, format, args...)
	b.sb.WriteByte('\n')
	return b
}

func (b *buf) blank() *buf { b.sb.WriteByte('\n'); return b }

func (b *buf) raw(s string) *buf { b.sb.WriteString(s); return b }

func (b *buf) String() string { return b.sb.String() }

type cField struct {
	typ     string
	name    string
	suffix  string
	comment string
}

type cStruct struct {
	name   string
	fields []cField
	doc    string
}

func (s *cStruct) emit(b *buf) {
	if s.doc != "" {
		for line := range strings.SplitSeq(strings.TrimSpace(s.doc), "\n") {
			b.pf("/* %s */", line)
		}
	}
	b.pf("typedef struct %s {", s.name)
	b.indent()
	for _, f := range s.fields {
		if f.comment != "" {
			b.pf("/* %s */", f.comment)
		}
		b.pf("%s %s%s;", f.typ, f.name, f.suffix)
	}
	b.unindent()
	b.pf("} %s_t;", s.name)
	b.blank()
}
