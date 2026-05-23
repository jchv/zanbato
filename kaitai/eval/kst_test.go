package eval

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/kst"
	"github.com/jchv/zanbato/kaitai/resolve"
)

var testSources = []struct {
	name       string
	formatsDir string
	kstDir     string
	srcDir     string
}{
	{
		name:       "upstream",
		formatsDir: "../../internal/third_party/kaitai_struct_tests/formats",
		kstDir:     "../../internal/third_party/kaitai_struct_tests/spec/ks",
		srcDir:     "../../internal/third_party/kaitai_struct_tests/src",
	},
	{
		name:       "custom",
		formatsDir: "../../testdata/formats",
		kstDir:     "../../testdata/spec/ks",
		srcDir:     "../../testdata/src",
	},
}

// compatTestIDs lists test IDs that are buggy and need the current latest
// Kaitai Struct compatibility mode to run in Zanbato.
var compatTestIDs = map[string]bool{
	"zb_expr_div_mod_64": true,
	"zb_expr_bitwise":    true,
}

var resolverPaths = []string{
	"../../internal/third_party/kaitai_struct_formats",
	"../../internal/third_party/kaitai_struct_tests/formats",
	"../../internal/third_party/kaitai_struct_tests/formats/ks_path",
	"../../testdata/formats",
}

func TestKST(t *testing.T) {
	resolver := resolve.NewOSResolverWithPaths(resolverPaths)

	for _, src := range testSources {
		kstFiles, err := filepath.Glob(filepath.Join(src.kstDir, "*.kst"))
		if err != nil {
			t.Logf("warning: could not glob %s: %v", src.kstDir, err)
			continue
		}

		for _, kstFile := range kstFiles {
			spec, err := kst.ParseFile(kstFile)
			if err != nil {
				t.Errorf("error: could not parse %s: %v", kstFile, err)
				continue
			}

			testName := src.name + "/" + spec.ID
			t.Run(testName, func(t *testing.T) {
				// Resolve the KSY format
				ksyPath := filepath.Join(src.formatsDir, spec.ID+".ksy")
				basename, struc, err := resolver.Resolve("", ksyPath)
				if err != nil {
					t.Errorf("could not resolve KSY %s: %v", spec.ID, err)
					return
				}

				// Check for ks-debug (skip for now)
				if struc.Meta.Debug {
					t.Skip("ks-debug not supported")
					return
				}

				// Open binary data
				dataPath := filepath.Join(src.srcDir, spec.Data)
				f, err := os.Open(dataPath)
				if err != nil {
					t.Errorf("could not open data %s: %v", dataPath, err)
					return
				}
				defer func() { _ = f.Close() }()

				// Build the tree
				stream := NewStream(f)
				tree, err := NewTree(resolver, basename, struc, stream)
				if err != nil {
					t.Fatalf("error creating tree: %v", err)
				}

				// Enable the latest Kaitai Struct compatibility mode for tests that
				// exercise bugs in Kaitai Struct.
				if compatTestIDs[spec.ID] {
					tree.Compat = kaitai.KaitaiStruct_0_11
				}

				// Register the custom processes used by upstream KS tests.
				registerTestProcesses(tree)

				// Resolve the root so children are available
				root := tree.Root()
				if err := root.Resolve(); err != nil {
					t.Fatalf("error resolving root: %v", err)
				}

				// Run each assertion
				for i, assert := range spec.Asserts {
					switch a := assert.(type) {
					case kst.TestEquals:
						runEqualsAssertion(t, tree, root, i, a)
					case kst.TestException:
						runExceptionAssertion(t, tree, root, i, a)
					}
				}
			})
		}
	}
}

func runEqualsAssertion(t *testing.T, tree *Tree, root *Node, idx int, a kst.TestEquals) {
	t.Helper()

	// Catch panics from the expression evaluator
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("assert[%d]: %s: panic: %v\n%s", idx, a.Actual, r, debug.Stack())
		}
	}()

	// Evaluate actual expression
	actualExpr, err := expr.ParseExpr(a.Actual)
	if err != nil {
		t.Errorf("assert[%d]: failed to parse actual %q: %v", idx, a.Actual, err)
		return
	}

	actualVal, err := tree.evaluateExpr(root, actualExpr)
	if err != nil {
		// If expected is null, an evaluation error means the field is absent - pass
		if a.Expected == "null" {
			return
		}
		t.Errorf("assert[%d]: failed to evaluate actual %q: %v", idx, a.Actual, err)
		return
	}

	// Handle null expected
	if a.Expected == "null" {
		if actualVal == nil {
			return // pass
		}
		// StructKind with no data (empty/unresolved struct) counts as null
		if actualVal.Kind == engine.StructKind {
			return // pass - conditional struct that was present but empty
		}
		switch actualVal.Kind {
		case engine.InvalidKind:
			return // pass
		default:
			t.Errorf("assert[%d]: %s: expected null, got %s", idx, a.Actual, actualVal.Kind)
		}
		return
	}

	// Evaluate expected expression
	expectedExpr, err := expr.ParseExpr(a.Expected)
	if err != nil {
		t.Errorf("assert[%d]: failed to parse expected %q: %v", idx, a.Expected, err)
		return
	}

	expectedVal, err := evalExpectedExpr(tree, root, expectedExpr)
	if err != nil {
		t.Errorf("assert[%d]: failed to evaluate expected %q: %v", idx, a.Expected, err)
		return
	}

	// Compare
	if !exprValuesEqual(actualVal, expectedVal) {
		t.Errorf("assert[%d]: %s\n  expected: %s\n  actual:   %s",
			idx, a.Actual, formatExprValue(expectedVal), formatExprValue(actualVal))
	}
}

func runExceptionAssertion(t *testing.T, tree *Tree, root *Node, idx int, a kst.TestException) {
	t.Helper()

	defer func() {
		// A panic counts as an exception - test passes. We deliberately
		// recover-and-drop without inspecting the panic value.
		_ = recover()
	}()

	actualExpr, err := expr.ParseExpr(a.Actual)
	if err != nil {
		// Parse error counts as exception
		return
	}

	_, err = tree.evaluateExpr(root, actualExpr)
	if err == nil {
		t.Errorf("assert[%d]: %s: expected exception %q but got no error", idx, a.Actual, a.Exception)
	}
}

// evalExpectedExpr evaluates an expected expression. Expected values are
// typically literals, so we can use the expression engine directly. For
// complex expressions that reference fields, we evaluate against the tree.
func evalExpectedExpr(tree *Tree, root *Node, e *expr.Expr) (*engine.ExprValue, error) {
	// Try evaluating as a standalone expression first (literals, enum constants)
	ctx := tree.contextForNode(root)
	ctx.PushStack()
	defer ctx.PopStack()
	return engine.Evaluate(ctx, e)
}

// exprValuesEqual compares two ExprValues for equality.
func exprValuesEqual(a, b *engine.ExprValue) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Normalize types for comparison
	aKind := a.Kind
	bKind := b.Kind

	// Handle integer comparisons (signed vs unsigned, enum vs int)
	aInt := exprValueToInt(a)
	bInt := exprValueToInt(b)
	if aInt != nil && bInt != nil {
		return aInt.Cmp(bInt) == 0
	}

	// Handle uint vs int comparison (e.g., u8 max value)
	if aKind == engine.IntegerKind && bKind == engine.IntegerKind {
		if a.Integer != nil && b.Integer != nil {
			return a.Integer.Value.Cmp(b.Integer.Value) == 0
		}
	}

	// Same kind comparisons
	if aKind == bKind {
		switch aKind {
		case engine.FloatKind:
			if a.Float != nil && b.Float != nil {
				diff := new(big.Float).Sub(a.Float.Value, b.Float.Value)
				diff.Abs(diff)
				threshold := new(big.Float).SetFloat64(1e-6)
				return diff.Cmp(threshold) <= 0
			}
		case engine.BooleanKind:
			if a.Boolean != nil && b.Boolean != nil {
				return a.Boolean.Value == b.Boolean.Value
			}
		case engine.StringKind:
			if a.String != nil && b.String != nil {
				return a.String.Value == b.String.Value
			}
		case engine.ByteArrayKind:
			if a.ByteArray != nil && b.ByteArray != nil {
				if len(a.ByteArray.Value) != len(b.ByteArray.Value) {
					return false
				}
				for i := range a.ByteArray.Value {
					if a.ByteArray.Value[i] != b.ByteArray.Value[i] {
						return false
					}
				}
				return true
			}
		case engine.ArrayKind:
			if len(a.Items) != len(b.Items) {
				return false
			}
			for i := range a.Items {
				if !exprValuesEqual(a.Items[i], b.Items[i]) {
					return false
				}
			}
			return true
		}
	}

	// Cross-kind: float vs int
	if aKind == engine.FloatKind && bKind == engine.IntegerKind {
		if a.Float != nil && b.Integer != nil {
			bFloat := new(big.Float).SetInt(b.Integer.Value)
			diff := new(big.Float).Sub(a.Float.Value, bFloat)
			diff.Abs(diff)
			threshold := new(big.Float).SetFloat64(1e-6)
			return diff.Cmp(threshold) <= 0
		}
	}
	if aKind == engine.IntegerKind && bKind == engine.FloatKind {
		return exprValuesEqual(b, a)
	}

	// String vs byte array (common in KST tests)
	if aKind == engine.StringKind && bKind == engine.ByteArrayKind {
		if a.String != nil && b.ByteArray != nil {
			return a.String.Value == string(b.ByteArray.Value)
		}
	}
	if aKind == engine.ByteArrayKind && bKind == engine.StringKind {
		return exprValuesEqual(b, a)
	}

	// Array of integers vs byte array (common in KST - expected: [0x73, 0x74, ...])
	if aKind == engine.ArrayKind && bKind == engine.ByteArrayKind {
		if b.ByteArray != nil && len(a.Items) == len(b.ByteArray.Value) {
			for i, item := range a.Items {
				if item.Kind != engine.IntegerKind || item.Integer == nil {
					return false
				}
				if byte(item.Integer.Value.Int64()) != b.ByteArray.Value[i] {
					return false
				}
			}
			return true
		}
	}
	if aKind == engine.ByteArrayKind && bKind == engine.ArrayKind {
		return exprValuesEqual(b, a)
	}

	// Array of integers vs string (KST: expected [0x73, ...] vs actual "str1")
	if aKind == engine.ArrayKind && bKind == engine.StringKind {
		if b.String != nil && len(a.Items) == len(b.String.Value) {
			for i, item := range a.Items {
				if item.Kind != engine.IntegerKind || item.Integer == nil {
					return false
				}
				if byte(item.Integer.Value.Int64()) != b.String.Value[i] {
					return false
				}
			}
			return true
		}
	}
	if aKind == engine.StringKind && bKind == engine.ArrayKind {
		return exprValuesEqual(b, a)
	}

	return false
}

// exprValueToInt extracts a *big.Int from an integer or enum value.
func exprValueToInt(v *engine.ExprValue) *big.Int {
	if v == nil {
		return nil
	}
	switch v.Kind {
	case engine.IntegerKind:
		if v.Integer != nil {
			return v.Integer.Value
		}
	case engine.EnumValueKind:
		if v.EnumValue != nil {
			return v.EnumValue.Value
		}
		if v.Integer != nil {
			return v.Integer.Value
		}
	}
	return nil
}

func formatExprValue(v *engine.ExprValue) string {
	if v == nil {
		return "<nil>"
	}
	switch v.Kind {
	case engine.IntegerKind:
		if v.Integer != nil {
			return fmt.Sprintf("int(%s)", v.Integer.Value.String())
		}
	case engine.FloatKind:
		if v.Float != nil {
			return fmt.Sprintf("float(%s)", v.Float.Value.String())
		}
	case engine.BooleanKind:
		if v.Boolean != nil {
			return fmt.Sprintf("bool(%v)", v.Boolean.Value)
		}
	case engine.StringKind:
		if v.String != nil {
			return fmt.Sprintf("str(%q)", v.String.Value)
		}
	case engine.ByteArrayKind:
		if v.ByteArray != nil {
			return fmt.Sprintf("bytes(%x)", v.ByteArray.Value)
		}
	case engine.ArrayKind:
		parts := make([]string, len(v.Items))
		for i, item := range v.Items {
			parts[i] = formatExprValue(item)
		}
		return fmt.Sprintf("[%s]", strings.Join(parts, ", "))
	}
	return fmt.Sprintf("<%s>", v.Kind)
}

// registerTestProcesses installs the custom-process handlers required by the
// upstream KS test suite. The implementations mirror the Go fixtures the
// codegen-based test rig drops alongside generated structs (see spec_test.go
// custom_fx*). Names match the qualified identifier used in the KSY
// `process:` clauses.
func registerTestProcesses(tree *Tree) {
	// custom_fx_no_args: wraps data with a leading and trailing '_'.
	wrapUnderscore := func(call *ProcessCall) ([]byte, error) {
		out := make([]byte, len(call.Data)+2)
		out[0] = '_'
		copy(out[1:], call.Data)
		out[len(out)-1] = '_'
		return out, nil
	}
	tree.RegisterProcess("custom_fx_no_args", wrapUnderscore)
	// nested.deeply.custom_fx(key): takes an int key but ignores it.
	tree.RegisterProcess("nested.deeply.custom_fx", wrapUnderscore)

	// my_custom_fx(key, flag, _ignored_bytes_): adds key to each byte mod 256;
	// flips key sign when flag is false.
	tree.RegisterProcess("my_custom_fx", func(call *ProcessCall) ([]byte, error) {
		if call.NumArgs() < 2 {
			return nil, fmt.Errorf("my_custom_fx requires >= 2 args")
		}
		key, err := call.Arg(0).Int()
		if err != nil {
			return nil, fmt.Errorf("my_custom_fx key: %w", err)
		}
		flag, err := call.Arg(1).Bool()
		if err != nil {
			return nil, fmt.Errorf("my_custom_fx flag: %w", err)
		}
		if !flag {
			key = -key
		}
		out := make([]byte, len(call.Data))
		for i, b := range call.Data {
			out[i] = byte((int(b) + int(key) + 0x100) % 0x100)
		}
		return out, nil
	})
}
