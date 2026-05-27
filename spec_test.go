package zanbato

import (
	"os"
	"testing"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/resolve"
)

type testSource struct {
	name       string
	formatsDir string
	kstDir     string
	srcDir     string
}

var testSources = []testSource{
	{
		name:       "upstream",
		formatsDir: "internal/third_party/kaitai_struct_tests/formats",
		kstDir:     "internal/third_party/kaitai_struct_tests/spec/ks",
		srcDir:     "internal/third_party/kaitai_struct_tests/src",
	},
	{
		name:       "custom",
		formatsDir: "testdata/formats",
		kstDir:     "testdata/spec/ks",
		srcDir:     "testdata/src",
	},
}

var testResolverPaths = []string{
	"internal/third_party/kaitai_struct_formats",
	"internal/third_party/kaitai_struct_tests/formats",
	"internal/third_party/kaitai_struct_tests/formats/ks_path",
	"testdata/formats",
}

func copyFileForTest(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func resolveImports(resolver resolve.Resolver, resolvedStructs map[string]*kaitai.Struct, inputname string, s *kaitai.Struct) {
	for _, imp := range s.Meta.Imports {
		impName, impStruc, err := resolver.Resolve(inputname, imp)
		if err == nil && impStruc != nil {
			if _, exists := resolvedStructs[string(impStruc.ID)]; !exists {
				resolvedStructs[string(impStruc.ID)] = impStruc
				resolveImports(resolver, resolvedStructs, impName, impStruc)
			}
		}
	}
}

// specResult aggregates KST-test outcomes for one emitter target so that
// TestSpecC and TestSpecGo can report directly comparable numbers.
type specResult struct {
	parsersOK  int // unique struct IDs whose parser was emitted
	total      int // total KST specs attempted
	emitErr    int // test-file emit failed (panic, parse error, missing struct)
	compileErr int // build/compile failed
	runErr     int // test ran but reported failure
	runOK      int // test ran and passed
	roundOK    int // roundtrip test passed
	roundFail  int // roundtrip test failed
}

func (r *specResult) log(t *testing.T) {
	t.Helper()
	t.Logf("Emitted parsers for %d KSY files", r.parsersOK)
	t.Logf("KST results: %d total / %d emit-err / %d compile-err / %d run-err / %d PASS",
		r.total, r.emitErr, r.compileErr, r.runErr, r.runOK)
	t.Logf("Roundtrip: %d OK / %d mismatch / %d not-attempted",
		r.roundOK, r.roundFail, r.runOK-r.roundOK-r.roundFail)
}
