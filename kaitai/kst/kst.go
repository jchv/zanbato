// Package kst parses Kaitai Struct Test (.kst) specification files.
//
// KST files are YAML-based test specifications that describe assertions
// to verify after parsing binary files with a given Kaitai Struct format.
package kst

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// TestSpec represents a parsed .kst file.
type TestSpec struct {
	ID        string
	Data      string
	Asserts   []TestAssert
	Exception *ExpectedException
}

// TestAssert is an assertion in a KST spec.
type TestAssert interface {
	isTestAssert()
}

// TestEquals asserts that an expression evaluates to an expected value.
type TestEquals struct {
	Actual   string // raw KS expression for the actual value
	Expected string // raw KS expression for the expected value
}

func (TestEquals) isTestAssert() {}

// TestException asserts that evaluating an expression raises an exception.
type TestException struct {
	Actual    string // raw KS expression
	Exception string // expected exception type name
}

func (TestException) isTestAssert() {}

// ExpectedException represents a top-level exception expectation (parse fails).
type ExpectedException struct {
	Type    string
	Message string
}

// rawSpec is the internal YAML representation of a .kst file.
type rawSpec struct {
	ID        string      `yaml:"id"`
	Data      string      `yaml:"data"`
	Asserts   []rawAssert `yaml:"asserts"`
	Exception any         `yaml:"exception"`
}

type rawAssert struct {
	Actual    string  `yaml:"actual"`
	Expected  *string `yaml:"expected"`
	Exception *string `yaml:"exception"`
}

// ParseFile parses a .kst file from disk.
func ParseFile(path string) (*TestSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading KST file: %w", err)
	}
	return Parse(data)
}

// Parse parses a .kst spec from raw YAML bytes.
func Parse(data []byte) (*TestSpec, error) {
	var raw rawSpec
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing KST YAML: %w", err)
	}

	spec := &TestSpec{
		ID:   raw.ID,
		Data: raw.Data,
	}

	// Parse top-level exception
	if raw.Exception != nil {
		exc, err := parseException(raw.Exception)
		if err != nil {
			return nil, fmt.Errorf("parsing exception: %w", err)
		}
		spec.Exception = exc
	}

	// Parse assertions
	for i, ra := range raw.Asserts {
		assert, err := parseAssert(ra)
		if err != nil {
			return nil, fmt.Errorf("parsing assert %d: %w", i, err)
		}
		spec.Asserts = append(spec.Asserts, assert)
	}

	return spec, nil
}

func parseException(v any) (*ExpectedException, error) {
	switch v := v.(type) {
	case string:
		return &ExpectedException{Type: v}, nil
	case map[string]any:
		exc := &ExpectedException{}
		if t, ok := v["type"].(string); ok {
			exc.Type = t
		} else {
			return nil, fmt.Errorf("exception map missing 'type' key")
		}
		if m, ok := v["message"].(string); ok {
			exc.Message = m
		}
		return exc, nil
	default:
		return nil, fmt.Errorf("unexpected exception type: %T", v)
	}
}

func parseAssert(ra rawAssert) (TestAssert, error) {
	if ra.Actual == "" {
		return nil, fmt.Errorf("assert missing 'actual' field")
	}

	switch {
	case ra.Expected != nil && ra.Exception != nil:
		return nil, fmt.Errorf("assert cannot have both 'expected' and 'exception'")
	case ra.Expected != nil:
		return TestEquals{
			Actual:   ra.Actual,
			Expected: *ra.Expected,
		}, nil
	case ra.Exception != nil:
		return TestException{
			Actual:    ra.Actual,
			Exception: *ra.Exception,
		}, nil
	default:
		return nil, fmt.Errorf("assert must have either 'expected' or 'exception'")
	}
}
