package eval

import (
	"strconv"
	"strings"
)

type Range struct {
	StartIndex, EndIndex uint64
}

type PathItem struct {
	Name  string
	Index *int `json:",omitempty"`
}

type Path []PathItem

func (p Path) String() string {
	s := strings.Builder{}
	for i, item := range p {
		if i > 0 {
			s.WriteByte('.')
		}
		s.WriteString(item.Name)
		if item.Index != nil {
			s.WriteByte('[')
			s.WriteString(strconv.Itoa(*item.Index))
			s.WriteByte(']')
		}
	}
	return s.String()
}

func (p Path) MarshalText() (text []byte, err error) {
	return []byte(p.String()), nil
}

func (p *Path) UnmarshalText(b []byte) error {
	*p = Path{}
	text := string(b)
	more := true
	for more {
		var element string
		element, text, more = strings.Cut(text, ".")
		name, subscript, hasSubscript := strings.Cut(element, "[")
		if hasSubscript && len(subscript) > 0 && subscript[len(subscript)-1] == ']' {
			if index, err := strconv.Atoi(string(subscript)); err == nil {
				*p = append(*p, PathItem{
					Name:  name,
					Index: &index,
				})
				continue
			}
		}
		*p = append(*p, PathItem{
			Name: name,
		})
	}
	return nil
}

type Label struct {
	Attr  Path
	Value any
}

type Annotation struct {
	Range Range
	Label Label
}
