package zanbato

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/emitter/golang"
	"github.com/jchv/zanbato/kaitai/kst"
	"github.com/jchv/zanbato/kaitai/resolve"
	"github.com/stretchr/testify/require"
)

const (
	TestSpecPath = ".test/spec"
)

// testSource describes a set of KSY formats, KST specs, and binary data.
type testSource struct {
	name       string
	formatsDir string
	kstDir     string
	srcDir     string
}

func TestSpec(t *testing.T) {
	// Get absolute paths
	wd, err := os.Getwd()
	require.NoError(t, err)

	// Create output directory
	outDir := filepath.Join(wd, TestSpecPath)
	require.NoError(t, os.RemoveAll(outDir))
	formatsOutDir := filepath.Join(outDir, "test_formats")
	specOutDir := filepath.Join(outDir, "spec")
	require.NoError(t, os.MkdirAll(formatsOutDir, 0o755))
	require.NoError(t, os.MkdirAll(specOutDir, 0o755))

	// Define test sources: upstream + custom
	sources := []testSource{
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

	// Shared resolver includes all format directories
	resolverPaths := []string{
		"internal/third_party/kaitai_struct_formats",
		"internal/third_party/kaitai_struct_tests/formats",
		"internal/third_party/kaitai_struct_tests/formats/ks_path",
		"testdata/formats",
	}

	generated := map[string]struct{}{}
	resolvedStructs := map[string]*kaitai.Struct{}
	var codegenErrors []string

	for _, src := range sources {
		matches, err := filepath.Glob(filepath.Join(src.formatsDir, "*.ksy"))
		if err != nil {
			t.Logf("warning: could not glob %s: %v", src.formatsDir, err)
			continue
		}

		for _, match := range matches {
			func() {
				defer func() {
					if r := recover(); r != nil {
						codegenErrors = append(codegenErrors, fmt.Sprintf("%s: %v", filepath.Base(match), r))
					}
				}()
				resolver := resolve.NewOSResolverWithPaths(resolverPaths)
				emitter := golang.NewEmitter("test_formats", resolver)
				basename, struc, err := resolver.Resolve("", match)
				if err != nil {
					panic(err)
				}
				resolvedStructs[string(struc.ID)] = struc
				// Also resolve all imported structs for cross-file KST test generation
				var resolveImports func(inputname string, s *kaitai.Struct)
				resolveImports = func(inputname string, s *kaitai.Struct) {
					for _, imp := range s.Meta.Imports {
						impName, impStruc, err := resolver.Resolve(inputname, imp)
						if err == nil && impStruc != nil {
							if _, exists := resolvedStructs[string(impStruc.ID)]; !exists {
								resolvedStructs[string(impStruc.ID)] = impStruc
								resolveImports(impName, impStruc)
							}
						}
					}
				}
				resolveImports(basename, struc)
				artifacts := emitter.Emit(basename, struc)
				for _, artifact := range artifacts {
					outname := filepath.Join(formatsOutDir, artifact.Filename)
					if _, exists := generated[artifact.Filename]; exists {
						// Overwrite only if the new version uses any-typed Root_
						// (import context), since imported types must accept any root.
						if !bytes.Contains(artifact.Body, []byte("Root_ any")) {
							continue
						}
					}
					if err := os.WriteFile(outname, artifact.Body, 0o644); err != nil {
						panic(err)
					}
					generated[artifact.Filename] = struct{}{}
				}
			}()
		}
	}

	t.Logf("Generated %d files, %d codegen errors", len(generated), len(codegenErrors))
	for _, e := range codegenErrors {
		t.Errorf("  codegen error: %s", e)
	}

	var generatedTests []string
	var kstErrors []string

	for _, src := range sources {
		kstFiles, err := filepath.Glob(filepath.Join(src.kstDir, "*.kst"))
		if err != nil {
			t.Logf("warning: could not glob %s: %v", src.kstDir, err)
			continue
		}

		absSrcDir := filepath.Join(wd, src.srcDir)
		kstEmitter := golang.NewKSTEmitter("spec", "test_formats", absSrcDir+"/")
		kstEmitter.ResolvedStructs = resolvedStructs

		for _, kstFile := range kstFiles {
			func() {
				defer func() {
					if r := recover(); r != nil {
						kstErrors = append(kstErrors, fmt.Sprintf("%s: %v", filepath.Base(kstFile), r))
					}
				}()
				spec, err := kst.ParseFile(kstFile)
				if err != nil {
					kstErrors = append(kstErrors, fmt.Sprintf("%s: parse error: %v", filepath.Base(kstFile), err))
					return
				}
				// Look up the resolved struct for this test
				ks := resolvedStructs[spec.ID]
				if ks == nil {
					kstErrors = append(kstErrors, fmt.Sprintf("%s: no resolved struct for %q", filepath.Base(kstFile), spec.ID))
					return
				}
				filename, body := kstEmitter.Emit(spec, ks)
				outPath := filepath.Join(specOutDir, filename)
				if err := os.WriteFile(outPath, body, 0o644); err != nil {
					kstErrors = append(kstErrors, fmt.Sprintf("%s: write error: %v", filepath.Base(kstFile), err))
					return
				}
				generatedTests = append(generatedTests, filename)
			}()
		}
	}
	t.Logf("Generated %d test files from KST, %d KST errors", len(generatedTests), len(kstErrors))
	for _, e := range kstErrors {
		t.Errorf("  KST error: %s", e)
	}

	customProcessFiles := map[string]string{
		"custom_fx_no_args.go": `package test_formats

// CustomFxNoArgs is a custom process that wraps data in underscores.
type CustomFxNoArgs struct{}

func NewCustomFxNoArgs() *CustomFxNoArgs {
	return &CustomFxNoArgs{}
}

func (p *CustomFxNoArgs) Decode(data []byte) []byte {
	result := make([]byte, len(data)+2)
	result[0] = '_'
	copy(result[1:], data)
	result[len(data)+1] = '_'
	return result
}

// Encode is the inverse of Decode: strips one leading and one trailing byte.
func (p *CustomFxNoArgs) Encode(data []byte) []byte {
	if len(data) < 2 {
		return nil
	}
	out := make([]byte, len(data)-2)
	copy(out, data[1:len(data)-1])
	return out
}
`,
		"my_custom_fx.go": `package test_formats

// MyCustomFx is a custom process that shifts bytes by a key.
type MyCustomFx struct {
	key int
}

func NewMyCustomFx(key int, flag bool, someBytes []byte) *MyCustomFx {
	k := key
	if !flag {
		k = -key
	}
	return &MyCustomFx{key: k}
}

func (p *MyCustomFx) Decode(data []byte) []byte {
	r := make([]byte, len(data))
	copy(r, data)
	for i, b := range r {
		r[i] = byte((int(b) + p.key) % 0x100)
	}
	return r
}

// Encode is the inverse shift used by Decode.
func (p *MyCustomFx) Encode(data []byte) []byte {
	r := make([]byte, len(data))
	for i, b := range data {
		r[i] = byte((int(b) - p.key + 0x100) % 0x100)
	}
	return r
}
`,
		"custom_fx.go": `package test_formats

// CustomFx is a custom process (nested.deeply.custom_fx).
// Wraps data with underscore bytes, ignoring the key (matches the official test definition).
type CustomFx struct{}

func NewCustomFx(key int) *CustomFx {
	return &CustomFx{}
}

func (p *CustomFx) Decode(data []byte) []byte {
	result := make([]byte, len(data)+2)
	result[0] = '_'
	copy(result[1:], data)
	result[len(data)+1] = '_'
	return result
}

// Encode is the inverse of Decode: strips one leading and one trailing byte.
func (p *CustomFx) Encode(data []byte) []byte {
	if len(data) < 2 {
		return nil
	}
	out := make([]byte, len(data)-2)
	copy(out, data[1:len(data)-1])
	return out
}
`,
	}
	for name, content := range customProcessFiles {
		require.NoError(t, os.WriteFile(filepath.Join(formatsOutDir, name), []byte(content), 0o644))
	}

	goMod := `module spec_test

go 1.24.0

require (
	test_formats v0.0.0
	github.com/jchw-forks/kaitai_struct_go_runtime v0.0.0-20260517224813-0f63727a30a6
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace test_formats => ./test_formats
`
	require.NoError(t, os.WriteFile(filepath.Join(outDir, "go.mod"), []byte(goMod), 0o644))

	tfGoMod := `module test_formats

go 1.24.0

require (
	github.com/jchw-forks/kaitai_struct_go_runtime v0.0.0-20260517224813-0f63727a30a6
)

require (
	golang.org/x/text v0.34.0 // indirect
)
`
	require.NoError(t, os.WriteFile(filepath.Join(formatsOutDir, "go.mod"), []byte(tfGoMod), 0o644))

	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = formatsOutDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Errorf("go mod tidy (test_formats) error: %v", err)
	}

	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = outDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Errorf("go mod tidy (spec) error: %v", err)
	}

	type dropped struct {
		Phase string
		File  string
		Error string
	}
	var droppedFiles []dropped

	dropFiles := func(phase string, buildOut []byte, prefixes []string) map[string]bool {
		removed := map[string]bool{}
		for line := range strings.SplitSeq(string(buildOut), "\n") {
			// Compiler error lines look like: <path>:<line>:<col>: <msg>
			var badFile string
			for _, prefix := range prefixes {
				if strings.HasPrefix(line, prefix) {
					if i := strings.Index(line, ":"); i > 0 {
						badFile = line[:i]
					}
					break
				}
			}
			if badFile == "" || removed[badFile] {
				continue
			}
			fullPath := filepath.Join(outDir, badFile)
			_ = os.Remove(fullPath)
			removed[badFile] = true
			droppedFiles = append(droppedFiles, dropped{Phase: phase, File: badFile, Error: line})
		}
		return removed
	}

	for range 20 {
		cmd = exec.Command("go", "build", "test_formats")
		cmd.Dir = outDir
		out, err := cmd.CombinedOutput()
		if err == nil {
			break
		}
		removed := dropFiles("test_formats build", out, []string{"test_formats/"})
		if len(removed) == 0 {
			t.Fatalf("test_formats build failed with no identifiable files to remove:\n%s", string(out))
		}
	}

	for range 50 {
		cmd = exec.Command("go", "test", "-c", "-o", "/dev/null", "./spec/")
		cmd.Dir = outDir
		buildOut, buildErr := cmd.CombinedOutput()
		if buildErr == nil {
			break
		}
		removed := dropFiles("spec build", buildOut, []string{"test_formats/", "spec/"})
		if len(removed) == 0 {
			t.Fatalf("spec build failed with no identifiable files to remove:\n%s", string(buildOut))
		}
	}

	cmd = exec.Command("go", "test", "-count=1", "-v", "./spec/")
	cmd.Dir = outDir
	testOut, testErr := cmd.CombinedOutput()
	output := string(testOut)

	passCount := strings.Count(output, "--- PASS:")
	failCount := strings.Count(output, "--- FAIL:")
	t.Logf("Results: %d passed, %d failed", passCount, failCount)

	for line := range strings.SplitSeq(output, "\n") {
		if strings.Contains(line, "FAIL") || strings.Contains(line, "Fatal") || strings.Contains(line, "panic") {
			t.Log(line)
		}
	}

	if len(droppedFiles) > 0 {
		var sb strings.Builder
		fmt.Fprintf(&sb, "Dropped %d generated file(s) due to compile errors:\n", len(droppedFiles))
		for _, d := range droppedFiles {
			fmt.Fprintf(&sb, "  [%s] %s\n      %s\n", d.Phase, d.File, d.Error)
		}
		t.Errorf("%s", sb.String())
	}

	if testErr != nil {
		t.Errorf("Tests had failures: %v", testErr)
	}
}
