package ksy

import (
	"fmt"
	"strings"
)

// ParentSpec represents the parent: key on an attribute.
// It can be false (to disable parent tracking for this usage site),
// or a string expression (to override the parent value passed to the child).
type ParentSpec struct {
	Disabled bool   // true when parent: false
	Expr     string // expression string when parent: <expr>
}

func (p *ParentSpec) UnmarshalYAML(unmarshal func(any) error) error {
	var b bool
	if err := unmarshal(&b); err == nil {
		p.Disabled = !b // parent: false -> Disabled=true
		return nil
	}
	var s string
	if err := unmarshal(&s); err == nil {
		p.Expr = s
		return nil
	}
	return fmt.Errorf("parent must be false or a string expression")
}

// AttributeSpec represents a KaitaiStruct attribute.
// #/definitions/Attribute
type AttributeSpec struct {
	ID          Identifier   `yaml:"id,omitempty"`
	Doc         string       `yaml:"doc,omitempty"`
	DocRef      DocRefSpec   `yaml:"doc-ref,omitempty"`
	Contents    ByteSpec     `yaml:"contents,omitempty"`
	Type        AttrTypeSpec `yaml:"type,omitempty"`
	Repeat      RepeatSpec   `yaml:"repeat,omitempty"`
	RepeatExpr  string       `yaml:"repeat-expr,omitempty"`
	RepeatUntil string       `yaml:"repeat-until,omitempty"`
	If          string       `yaml:"if,omitempty"`
	Size        string       `yaml:"size,omitempty"`
	SizeEos     bool         `yaml:"size-eos,omitempty"`
	Process     string       `yaml:"process,omitempty"`
	Enum        string       `yaml:"enum,omitempty"`
	Encoding    string       `yaml:"encoding,omitempty"`
	PadRight    *int         `yaml:"pad-right,omitempty"`
	Terminator  *int         `yaml:"terminator,omitempty"`
	Consume     *bool        `yaml:"consume,omitempty"`
	Include     *bool        `yaml:"include,omitempty"`
	EosError    *bool        `yaml:"eos-error,omitempty"`
	Pos         string       `yaml:"pos,omitempty"`
	IO          string       `yaml:"io,omitempty"`
	Value       string       `yaml:"value,omitempty"`
	Valid       *ValidSpec   `yaml:"valid,omitempty"`
	Parent      *ParentSpec  `yaml:"parent,omitempty"`
}

// ValidSpec represents a validation constraint.
// It can be a simple value (for equality check) or a map with fields.
type ValidSpec struct {
	Eq     string
	Min    string
	Max    string
	AnyOf  []string
	Expr   string
	InEnum bool
}

func (v *ValidSpec) UnmarshalYAML(unmarshal func(any) error) error {
	// Try as bool first (YAML true/false)
	var simpleBool bool
	if err := unmarshal(&simpleBool); err == nil {
		if simpleBool {
			v.Eq = "true"
		} else {
			v.Eq = "false"
		}
		return nil
	}
	// Try as int (handles YAML integer syntax including hex with underscores)
	var simpleInt int64
	if err := unmarshal(&simpleInt); err == nil {
		v.Eq = fmt.Sprintf("%d", simpleInt)
		return nil
	}
	// Try as uint64 for values that overflow int64
	var simpleUint uint64
	if err := unmarshal(&simpleUint); err == nil {
		v.Eq = fmt.Sprintf("%d", simpleUint)
		return nil
	}
	var simpleFloat float64
	if err := unmarshal(&simpleFloat); err == nil {
		v.Eq = fmt.Sprintf("%v", simpleFloat)
		return nil
	}
	// Try as byte array (YAML sequence of integers)
	var simpleSeq []any
	if err := unmarshal(&simpleSeq); err == nil {
		parts := make([]string, len(simpleSeq))
		for i, item := range simpleSeq {
			parts[i] = fmt.Sprintf("%v", item)
		}
		v.Eq = "[" + strings.Join(parts, ", ") + "]"
		return nil
	}
	// Try as string
	var simpleStr string
	if err := unmarshal(&simpleStr); err == nil {
		v.Eq = simpleStr
		return nil
	}
	// Try as map
	var m map[string]any
	if err := unmarshal(&m); err == nil {
		if val, ok := m["eq"]; ok {
			v.Eq = fmt.Sprintf("%v", val)
		}
		if val, ok := m["min"]; ok {
			v.Min = fmt.Sprintf("%v", val)
		}
		if val, ok := m["max"]; ok {
			v.Max = fmt.Sprintf("%v", val)
		}
		if val, ok := m["expr"]; ok {
			v.Expr = fmt.Sprintf("%v", val)
		}
		if val, ok := m["any-of"]; ok {
			if arr, ok := val.([]any); ok {
				for _, item := range arr {
					v.AnyOf = append(v.AnyOf, fmt.Sprintf("%v", item))
				}
			}
		}
		if val, ok := m["in-enum"]; ok {
			if b, ok := val.(bool); ok {
				v.InEnum = b
			}
		}
		return nil
	}
	return fmt.Errorf("cannot parse valid spec")
}

// AttributesSpec represents an attribute list.
// #/definitions/Attributes
type AttributesSpec []AttributeSpec
