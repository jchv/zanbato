package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jchv/zanbato/kaitai/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	upstreamFormatsDir = "../../internal/third_party/kaitai_struct_tests/formats"
	upstreamSrcDir     = "../../internal/third_party/kaitai_struct_tests/src"
)

func openTree(t *testing.T, ksyName string, dataFile string) *Tree {
	t.Helper()
	ksyPath := filepath.Join(upstreamFormatsDir, ksyName+".ksy")
	dataPath := filepath.Join(upstreamSrcDir, dataFile)

	resolver := resolve.NewOSResolverWithPaths([]string{
		upstreamFormatsDir,
		filepath.Join(upstreamFormatsDir, "ks_path"),
	})
	basename, struc, err := resolver.Resolve("", ksyPath)
	require.NoError(t, err, "resolving KSY %s", ksyName)

	f, err := os.Open(dataPath)
	require.NoError(t, err, "opening data file %s", dataFile)
	t.Cleanup(func() { _ = f.Close() })

	stream := NewStream(f)
	tree, err := NewTree(resolver, basename, struc, stream)
	require.NoError(t, err, "creating tree for %s", ksyName)
	return tree
}

func TestRuntime_HelloWorld(t *testing.T) {
	tree := openTree(t, "hello_world", "fixed_struct.bin")
	root := tree.Root()
	require.NoError(t, root.Resolve())

	one, err := root.Child("one")
	require.NoError(t, err)
	require.NotNil(t, one)

	v, err := one.Value()
	require.NoError(t, err)
	assert.Equal(t, KindUint, v.Kind)
	assert.Equal(t, uint64(0x50), v.Uint)
}

func TestRuntime_FixedStruct(t *testing.T) {
	// fixed_struct.bin is used by hello_world.ksy
	// It has one u1 field "one" at offset 0
	tree := openTree(t, "hello_world", "fixed_struct.bin")
	root := tree.Root()

	one, err := root.Child("one")
	require.NoError(t, err)
	require.NotNil(t, one)

	// Check byte range
	r, err := one.ByteRange()
	require.NoError(t, err)
	assert.Equal(t, uint64(0), r.StartIndex)
	assert.Equal(t, uint64(1), r.EndIndex)
}

func TestRuntime_FloatToI(t *testing.T) {
	tree := openTree(t, "float_to_i", "floating_points.bin")
	root := tree.Root()

	node, err := root.Child("single_value")
	require.NoError(t, err)
	require.NotNil(t, node)
	v, err := node.Value()
	require.NoError(t, err)
	assert.Equal(t, KindFloat, v.Kind)
	// Just verify it parsed a float successfully
	assert.NotEqual(t, 0.0, v.Float)
}

func TestRuntime_RepeatNStruct(t *testing.T) {
	tree := openTree(t, "repeat_n_struct", "repeat_n_struct.bin")
	root := tree.Root()

	chunks, err := root.Child("chunks")
	require.NoError(t, err)
	require.NotNil(t, chunks)

	items, err := chunks.Items()
	require.NoError(t, err)
	assert.Len(t, items, 2, "expected 2 chunks")
}

func TestRuntime_StrPadTerm(t *testing.T) {
	tree := openTree(t, "str_pad_term", "str_pad_term.bin")
	root := tree.Root()

	node, err := root.Child("str_pad")
	require.NoError(t, err)
	require.NotNil(t, node)
	v, err := node.Value()
	require.NoError(t, err)
	assert.Equal(t, KindStr, v.Kind)
	assert.Equal(t, "str1", v.Str)
}

func TestRuntime_ProcessXorConst(t *testing.T) {
	tree := openTree(t, "process_xor_const", "process_xor_4.bin")
	root := tree.Root()

	// key is a u1 field (first byte)
	key, err := root.Child("key")
	require.NoError(t, err)
	require.NotNil(t, key)
	kv, err := key.Value()
	require.NoError(t, err)
	assert.Equal(t, KindUint, kv.Kind)

	// buf is size-eos with process: xor(0xff)
	buf, err := root.Child("buf")
	require.NoError(t, err)
	require.NotNil(t, buf)
	bv, err := buf.Value()
	require.NoError(t, err)
	assert.Equal(t, KindBytes, bv.Kind)
	// After XOR with 0xff, the result should be the decoded data
	assert.True(t, len(bv.Bytes) > 0, "buf should have data after XOR")
}

func TestRuntime_IfStruct(t *testing.T) {
	tree := openTree(t, "if_struct", "if_struct.bin")
	root := tree.Root()

	op1, err := root.Child("op1")
	require.NoError(t, err)
	require.NotNil(t, op1)
	err = op1.Resolve()
	require.NoError(t, err)

	opcode, err := op1.Child("opcode")
	require.NoError(t, err)
	require.NotNil(t, opcode)
	v, err := opcode.Value()
	require.NoError(t, err)
	assert.Equal(t, KindUint, v.Kind)
	assert.Equal(t, uint64(0x53), v.Uint)
}

func TestRuntime_UserType(t *testing.T) {
	// integers_min_max uses nested user types
	tree := openTree(t, "integers_min_max", "integers_min_max.bin")
	root := tree.Root()

	// Access unsigned_min (a nested struct)
	unsMin, err := root.Child("unsigned_min")
	require.NoError(t, err)
	require.NotNil(t, unsMin)
	err = unsMin.Resolve()
	require.NoError(t, err)
	assert.Equal(t, KindStruct, unsMin.value.Kind)

	// Access unsigned_min.u1 (should be 0)
	u1, err := unsMin.Child("u1")
	require.NoError(t, err)
	require.NotNil(t, u1)
	v, err := u1.Value()
	require.NoError(t, err)
	assert.Equal(t, KindUint, v.Kind)
	assert.Equal(t, uint64(0), v.Uint)
}

func TestRuntime_LazyResolution(t *testing.T) {
	tree := openTree(t, "hello_world", "fixed_struct.bin")
	root := tree.Root()

	fields := root.Fields()
	assert.True(t, len(fields) > 0, "should have fields from schema")

	// None should be resolved yet (root auto-resolves, but children don't)
	for _, f := range fields {
		assert.False(t, f.IsResolved(), "field %s should not be resolved yet", f.Name())
	}

	// Resolving a later field forces predecessors
	last := fields[len(fields)-1]
	err := last.Resolve()
	require.NoError(t, err)
	assert.True(t, last.IsResolved())

	// All predecessors should also be resolved
	for _, f := range fields {
		assert.True(t, f.IsResolved(), "field %s should be resolved after resolving last field", f.Name())
	}
}

func TestRuntime_Invalidate(t *testing.T) {
	tree := openTree(t, "hello_world", "fixed_struct.bin")
	root := tree.Root()

	// Resolve everything (best-effort - invalidation test doesn't care if
	// individual fields fail).
	for _, f := range root.Fields() {
		_ = f.Resolve()
	}

	// Invalidate
	root.Invalidate()

	// Nothing should be resolved
	for _, f := range root.Fields() {
		assert.False(t, f.IsResolved(), "field %s should not be resolved after invalidation", f.Name())
	}

	// Re-resolve should work
	for _, f := range root.Fields() {
		err := f.Resolve()
		assert.NoError(t, err, "re-resolving %s after invalidation", f.Name())
	}
}

// TestRuntime_MarkDirty verifies that dirtying a node propagates through the
// rdep DAG built up during expression evaluation: when a depended-on value is
// modified, every dependent ends up Unresolved again, while unrelated nodes
// stay intact.
func TestRuntime_MarkDirty(t *testing.T) {
	// nav_parent.ksy:
	//   index.entries count = _parent.header.qty_entries
	//   entry.filename size = _parent._parent.header.filename_len
	// So index.entries depends on header.qty_entries, and each entry's
	// filename depends on header.filename_len.
	tree := openTree(t, "nav_parent", "nav.bin")
	root := tree.Root()
	require.NoError(t, root.Resolve())

	header, err := root.Child("header")
	require.NoError(t, err)
	require.NotNil(t, header)
	require.NoError(t, header.Resolve()) // populates header.childMap
	qty, err := header.Child("qty_entries")
	require.NoError(t, err)
	require.NotNil(t, qty)

	index, err := root.Child("index")
	require.NoError(t, err)
	require.NotNil(t, index)
	require.NoError(t, index.Resolve())
	entries, err := index.Child("entries")
	require.NoError(t, err)
	require.NotNil(t, entries)

	// Touch entries.Items so the array is resolved and the dep on
	// header.qty_entries is recorded.
	items, err := entries.Items()
	require.NoError(t, err)
	require.True(t, len(items) > 0)

	// Touch the first item's filename so it pulls in header.filename_len too.
	filenameLen, err := header.Child("filename_len")
	require.NoError(t, err)
	require.NotNil(t, filenameLen)
	_, err = filenameLen.Value()
	require.NoError(t, err)
	filename, err := items[0].Child("filename")
	require.NoError(t, err)
	require.NotNil(t, filename)
	_, err = filename.Value()
	require.NoError(t, err)

	// Sanity: everything we care about is resolved.
	assert.True(t, qty.IsResolved())
	assert.True(t, filenameLen.IsResolved())
	assert.True(t, entries.IsResolved())
	assert.True(t, filename.IsResolved())

	// Dirty header.qty_entries. Anything that consulted it during
	// resolution must end up Unresolved.
	qty.MarkDirty()

	assert.False(t, qty.IsResolved(), "qty_entries should be dirty")
	assert.False(t, entries.IsResolved(),
		"entries depends on qty_entries - should be dirtied")
	// filename_len does NOT depend on qty_entries, so it stays clean.
	assert.True(t, filenameLen.IsResolved(),
		"filename_len is independent of qty_entries - should remain resolved")
}

// TestRuntime_SetValueRecomputesDependents demonstrates the edit half of
// the rdep-tracked invalidation pipeline: editing a primitive value causes
// dependent computations to re-evaluate against the new value on next
// access, without touching unrelated nodes.
func TestRuntime_SetValueRecomputesDependents(t *testing.T) {
	tree := openTree(t, "nav_parent", "nav.bin")
	root := tree.Root()
	require.NoError(t, root.Resolve())

	header, err := root.Child("header")
	require.NoError(t, err)
	require.NoError(t, header.Resolve())
	qty, err := header.Child("qty_entries")
	require.NoError(t, err)
	require.NotNil(t, qty)

	index, err := root.Child("index")
	require.NoError(t, err)
	require.NoError(t, index.Resolve())
	entries, err := index.Child("entries")
	require.NoError(t, err)
	require.NotNil(t, entries)

	// Initial: read entries (uses qty_entries to determine count).
	items, err := entries.Items()
	require.NoError(t, err)
	initialLen := len(items)
	require.True(t, initialLen > 0)

	// Confirm the qty value matches.
	qtyVal, err := qty.Value()
	require.NoError(t, err)
	require.Equal(t, KindUint, qtyVal.Kind)
	require.Equal(t, uint64(initialLen), qtyVal.Uint)

	// Edit qty_entries to a smaller number. Dependents (entries) should be
	// invalidated. Note: entries' subsequent re-resolution will read the
	// same underlying bytes but interpret a different count.
	require.NoError(t, qty.SetValue(Value{Kind: KindUint, Uint: uint64(initialLen - 1)}))

	// qty stays resolved with the new value.
	assert.True(t, qty.IsResolved())
	newVal, err := qty.Value()
	require.NoError(t, err)
	assert.Equal(t, uint64(initialLen-1), newVal.Uint)

	// entries was dirtied. Re-reading triggers re-resolution with the new
	// count and yields fewer items.
	assert.False(t, entries.IsResolved(), "entries should have been dirtied by SetValue")
	newItems, err := entries.Items()
	require.NoError(t, err)
	assert.Len(t, newItems, initialLen-1,
		"entries.Items should reflect the edited qty_entries on re-resolution")
}

// TestRuntime_LazyUserType verifies that user-type fields whose size is known
// (static layout) do not eagerly resolve their seq children. Demanding one
// child's value resolves that child and its positional predecessors, but
// leaves successor siblings and unrelated array elements untouched.
func TestRuntime_LazyUserType(t *testing.T) {
	// repeat_n_struct.ksy: qty:u4 then chunks: chunk[qty], where chunk has
	// a fixed 8-byte layout (offset:u4 + len:u4). Each chunk should therefore
	// be span-resolved without eagerly walking its seq children.
	tree := openTree(t, "repeat_n_struct", "repeat_n_struct.bin")
	root := tree.Root()
	require.NoError(t, root.Resolve())

	chunks, err := root.Child("chunks")
	require.NoError(t, err)
	require.NotNil(t, chunks)

	items, err := chunks.Items()
	require.NoError(t, err)
	require.True(t, len(items) >= 2, "need at least 2 chunks for this probe")

	// At this point chunks[*] are span-resolved (the array materialized
	// them), but their children should be untouched.
	for i, ch := range items {
		for _, f := range ch.Fields() {
			assert.Falsef(t, f.IsResolved(),
				"chunks[%d].%s should be lazy before access", i, f.Name())
		}
	}

	// Demand chunks[0].offset: only that field should resolve.
	c0 := items[0]
	off, err := c0.Child("offset")
	require.NoError(t, err)
	_, err = off.Value()
	require.NoError(t, err)
	assert.True(t, off.IsResolved())

	// chunks[0].len is positionally after offset, so it should still be lazy.
	lenF, err := c0.Child("len")
	require.NoError(t, err)
	assert.False(t, lenF.IsResolved(),
		"chunks[0].len should remain lazy when only offset was demanded")

	// chunks[1] children should be entirely untouched.
	c1 := items[1]
	for _, f := range c1.Fields() {
		assert.Falsef(t, f.IsResolved(),
			"chunks[1].%s should remain lazy", f.Name())
	}
}
