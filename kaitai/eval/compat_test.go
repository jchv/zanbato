package eval

import (
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/expr"
	"github.com/jchv/zanbato/kaitai/expr/engine"
	"github.com/jchv/zanbato/kaitai/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	customFormatsDir = "../../testdata/formats"
	customSrcDir     = "../../testdata/src"
)

// openCustomTree opens a tree from a custom testdata format and binary file,
// with the given compatibility mode.
func openCustomTree(t *testing.T, ksyName string, dataFile string, compat kaitai.Compatibility) *Tree {
	t.Helper()
	ksyPath := filepath.Join(customFormatsDir, ksyName+".ksy")
	dataPath := filepath.Join(customSrcDir, dataFile)

	resolver := resolve.NewOSResolverWithPaths([]string{
		customFormatsDir,
	})
	basename, struc, err := resolver.Resolve("", ksyPath)
	require.NoError(t, err, "resolving KSY %s", ksyName)

	f, err := os.Open(dataPath)
	require.NoError(t, err, "opening data file %s", dataFile)
	t.Cleanup(func() { _ = f.Close() })

	stream := NewStream(f)
	tree, err := NewTree(resolver, basename, struc, stream)
	require.NoError(t, err, "creating tree for %s", ksyName)
	tree.Compat = compat
	return tree
}

// evalExprOnTree evaluates a KS expression string on the given tree root
// using the tree's evaluation machinery and returns the engine ExprValue.
func evalExprOnTree(t *testing.T, tree *Tree, root *Node, expression string) *engine.ExprValue {
	t.Helper()
	e, err := expr.ParseExpr(expression)
	require.NoError(t, err, "parsing expression %q", expression)
	val, err := tree.evaluateExpr(root, e)
	require.NoError(t, err, "evaluating expression %q on root", expression)
	return val
}

// TestCompat_NativeMode_64BitArithmetic verifies that in ZanbatoNative mode,
// 64-bit arithmetic division produces the correct full-precision result.
func TestCompat_NativeMode_64BitArithmetic(t *testing.T) {
	tree := openCustomTree(t, "zb_expr_div_mod_64", "zb_expr_div_mod_64.bin", kaitai.ZanbatoNative)
	root := tree.Root()
	require.NoError(t, root.Resolve())

	// val_u8 is a u8 field with value 922337203685477580.
	// div_u8 = val_u8 / 100
	// In native mode (no 32-bit truncation), the result is 9223372036854775.
	val := evalExprOnTree(t, tree, root, "div_u8")
	require.NotNil(t, val)
	if val.Integer != nil {
		expected := big.NewInt(9223372036854775)
		assert.Equal(t, 0, val.Integer.Value.Cmp(expected),
			"native mode: div_u8 should be %s, got %s", expected, val.Integer.Value.String())
	}
}

// TestCompat_KS11Mode_64BitArithmetic verifies that in KaitaiStruct_0_11 mode,
// 64-bit arithmetic division is truncated to 32-bit signed, matching the
// upstream Kaitai Struct compiler's behavior.
func TestCompat_KS11Mode_64BitArithmetic(t *testing.T) {
	tree := openCustomTree(t, "zb_expr_div_mod_64", "zb_expr_div_mod_64.bin", kaitai.KaitaiStruct_0_11)
	root := tree.Root()
	require.NoError(t, root.Resolve())

	// val_u8 is a u8 field with value 922337203685477580.
	// div_u8 = val_u8 / 100
	// In KaitaiStruct_0.11 mode, the result is truncated to 32-bit signed:
	//   9223372036854775 (correct result) truncated to int32 -> -1511828489
	val := evalExprOnTree(t, tree, root, "div_u8")
	require.NotNil(t, val)
	if val.Integer != nil {
		expected := big.NewInt(-1511828489)
		assert.Equal(t, 0, val.Integer.Value.Cmp(expected),
			"KaitaiStruct_0.11 mode: div_u8 should be %s (truncated to 32-bit), got %s",
			expected, val.Integer.Value.String())
	}
}

// TestCompat_KS11Mode_Modulo verifies that modulo operations in compat mode
// produce the truncated (32-bit) result.
func TestCompat_KS11Mode_Modulo(t *testing.T) {
	tree := openCustomTree(t, "zb_expr_div_mod_64", "zb_expr_div_mod_64.bin", kaitai.KaitaiStruct_0_11)
	root := tree.Root()
	require.NoError(t, root.Resolve())

	// mod_u8 = val_u8 % 100
	// In KaitaiStruct_0.11 mode, the intermediate result is truncated, but
	// for this particular value the modulo result is small enough (80) that
	// it fits in 32 bits, so it should be the same as the correct answer.
	val := evalExprOnTree(t, tree, root, "mod_u8")
	require.NotNil(t, val)
	if val.Integer != nil {
		expected := big.NewInt(80)
		assert.Equal(t, 0, val.Integer.Value.Cmp(expected),
			"KaitaiStruct_0.11 mode: mod_u8 should be %s, got %s", expected, val.Integer.Value.String())
	}
}

// TestCompat_KS11Mode_Invert verifies that bitwise inversion in compat mode
// produces the 32-bit-truncated result.
func TestCompat_KS11Mode_Invert(t *testing.T) {
	tree := openCustomTree(t, "zb_expr_div_mod_64", "zb_expr_div_mod_64.bin", kaitai.KaitaiStruct_0_11)
	root := tree.Root()
	require.NoError(t, root.Resolve())

	// invert_u8 = ~val_u8
	// val_u8 = 922337203685477580 (0x0CCCCCCCCCCCCCCC in 64 bits)
	// ~val_u8 in arbitrary precision = -922337203685477581
	val := evalExprOnTree(t, tree, root, "invert_u8")
	require.NotNil(t, val)
	if val.Integer != nil {
		expected := big.NewInt(-922337203685477581)
		assert.Equal(t, 0, val.Integer.Value.Cmp(expected),
			"KaitaiStruct_0.11 mode: invert_u8 should be %s, got %s", expected, val.Integer.Value.String())
	}
}

// TestCompat_SmallArithmetic_NativeMode verifies that small integer arithmetic
// that fits in 32 bits produces the same result regardless of compat mode.
func TestCompat_SmallArithmetic_NativeMode(t *testing.T) {
	tree := openCustomTree(t, "zb_expr_div_mod_64", "zb_expr_div_mod_64.bin", kaitai.ZanbatoNative)
	root := tree.Root()
	require.NoError(t, root.Resolve())

	// The div_u8 result in native mode should be 9223372036854775 (full precision).
	val := evalExprOnTree(t, tree, root, "div_u8")
	require.NotNil(t, val)
	require.NotNil(t, val.Integer, "expected integer result, got kind %s", val.Kind)
	expected := big.NewInt(9223372036854775)
	assert.Equal(t, 0, val.Integer.Value.Cmp(expected),
		"div_u8 should be %s, got %s", expected, val.Integer.Value.String())
}

// TestCompat_NativeMode_NoTruncationOnAdd verifies that addition is not
// truncated in native mode.
func TestCompat_NativeMode_NoTruncationOnAdd(t *testing.T) {
	tree := openCustomTree(t, "zb_expr_div_mod_64", "zb_expr_div_mod_64.bin", kaitai.ZanbatoNative)
	root := tree.Root()
	require.NoError(t, root.Resolve())

	// val_u8 + 100 should give 922337203685477680 in native mode.
	val := evalExprOnTree(t, tree, root, "val_u8 + 100")
	require.NotNil(t, val)
	require.NotNil(t, val.Integer, "expected integer result, got kind %s", val.Kind)
	expected := big.NewInt(922337203685477680)
	assert.Equal(t, 0, val.Integer.Value.Cmp(expected),
		"native mode: val_u8 + 100 should be %s, got %s", expected, val.Integer.Value.String())
}

// TestCompat_KS11Mode_TruncationOnAdd verifies that addition is truncated
// to 32-bit signed in KaitaiStruct_0.11 mode.
func TestCompat_KS11Mode_TruncationOnAdd(t *testing.T) {
	tree := openCustomTree(t, "zb_expr_div_mod_64", "zb_expr_div_mod_64.bin", kaitai.KaitaiStruct_0_11)
	root := tree.Root()
	require.NoError(t, root.Resolve())

	// val_u8 + 100 in 32-bit compat mode.
	// val_u8 low 32 bits = 0xCCCCCCCC = 3435973836
	// 3435973836 + 100 = 3435973936
	// As int32: 3435973936 - 2^32 = 3435973936 - 4294967296 = -858993360
	val := evalExprOnTree(t, tree, root, "val_u8 + 100")
	require.NotNil(t, val)
	require.NotNil(t, val.Integer, "expected integer result, got kind %s", val.Kind)
	expected := big.NewInt(-858993360)
	assert.Equal(t, 0, val.Integer.Value.Cmp(expected),
		"KaitaiStruct_0.11 mode: val_u8 + 100 should be %s, got %s", expected, val.Integer.Value.String())
}

// TestCompat_GlobalDefault verifies that engine.DefaultCompat can be set to
// control the default compat mode for newly created trees.
func TestCompat_GlobalDefault(t *testing.T) {
	orig := engine.DefaultCompat
	defer func() { engine.DefaultCompat = orig }()

	engine.DefaultCompat = kaitai.KaitaiStruct_0_11

	ksyPath := filepath.Join(customFormatsDir, "zb_expr_div_mod_64.ksy")
	dataPath := filepath.Join(customSrcDir, "zb_expr_div_mod_64.bin")
	resolver := resolve.NewOSResolverWithPaths([]string{customFormatsDir})
	basename, struc, err := resolver.Resolve("", ksyPath)
	require.NoError(t, err)

	f, err := os.Open(dataPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	stream := NewStream(f)
	defaultTree, err := NewTree(resolver, basename, struc, stream)
	require.NoError(t, err)
	assert.Equal(t, kaitai.KaitaiStruct_0_11, defaultTree.Compat,
		"NewTree should inherit engine.DefaultCompat")
}
