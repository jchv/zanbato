package zanbato

import (
	"bytes"
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

func TestSpecGo(t *testing.T) {
	t.Parallel()

	wd, err := os.Getwd()
	require.NoError(t, err)

	outDir := filepath.Join(wd, ".test", "go")
	formatsOutDir := filepath.Join(outDir, "test_formats")
	specOutDir := filepath.Join(outDir, "spec")
	require.NoError(t, os.RemoveAll(outDir))
	require.NoError(t, os.MkdirAll(formatsOutDir, 0o755))
	require.NoError(t, os.MkdirAll(specOutDir, 0o755))

	// Phase 1: emit Go parsers for every KSY.
	resolvedStructs := map[string]*kaitai.Struct{}
	emittedIDs := map[string]struct{}{}
	generated := map[string]struct{}{}
	for _, src := range testSources {
		matches, err := filepath.Glob(filepath.Join(src.formatsDir, "*.ksy"))
		if err != nil {
			continue
		}
		for _, match := range matches {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("emit panic %s: %v", filepath.Base(match), r)
					}
				}()
				resolver := resolve.NewOSResolverWithPaths(testResolverPaths)
				emitter := golang.NewEmitter("test_formats", resolver)
				emitter.SetCompat(kaitai.KaitaiStruct_0_11)
				basename, struc, err := resolver.Resolve("", match)
				if err != nil {
					return
				}
				resolvedStructs[string(struc.ID)] = struc
				resolveImports(resolver, resolvedStructs, basename, struc)
				artifacts := emitter.Emit(basename, struc)
				for _, artifact := range artifacts {
					outname := filepath.Join(formatsOutDir, artifact.Filename)
					if _, exists := generated[artifact.Filename]; exists {
						// Overwrite only if the new version uses any-typed Root_
						// (i.e. an import-context version, which must accept any
						// root at runtime).
						if !bytes.Contains(artifact.Body, []byte("Root_ any")) {
							continue
						}
					}
					if err := os.WriteFile(outname, artifact.Body, 0o644); err != nil {
						t.Errorf("write %s: %v", outname, err)
						return
					}
					generated[artifact.Filename] = struct{}{}
				}
				emittedIDs[string(struc.ID)] = struct{}{}
			}()
		}
	}

	// Phase 2: emit Go test files for every KST.
	var result specResult
	result.parsersOK = len(emittedIDs)
	for _, src := range testSources {
		kstFiles, err := filepath.Glob(filepath.Join(src.kstDir, "*.kst"))
		if err != nil {
			continue
		}
		absSrcDir := filepath.Join(wd, src.srcDir)
		kstEmitter := golang.NewKSTEmitter("spec", "test_formats", absSrcDir+"/")
		kstEmitter.ResolvedStructs = resolvedStructs

		for _, kstFile := range kstFiles {
			result.total++
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("KST emit panic %s: %v", filepath.Base(kstFile), r)
						result.emitErr++
					}
				}()
				spec, err := kst.ParseFile(kstFile)
				if err != nil {
					result.emitErr++
					return
				}
				ks := resolvedStructs[spec.ID]
				if ks == nil {
					result.emitErr++
					return
				}
				filename, body := kstEmitter.Emit(spec, ks)
				if err := os.WriteFile(filepath.Join(specOutDir, filename), body, 0o644); err != nil {
					result.emitErr++
				}
			}()
		}
	}

	// Phase 3: write test scaffolding (custom processes + go.mod files).
	writeGoTestScaffolding(t, outDir, formatsOutDir)

	for _, dir := range []string{formatsOutDir, outDir} {
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Errorf("go mod tidy in %s: %v", dir, err)
		}
	}

	// Phase 4: build test_formats, dropping files that fail to compile.
	dropUntilBuilds(t, outDir, "test_formats build",
		[]string{"go", "build", "test_formats"},
		[]string{"test_formats/"})

	// Phase 5: build spec tests, counting drops as compile errors.
	result.compileErr = dropUntilBuilds(t, outDir, "spec build",
		[]string{"go", "test", "-c", "-o", "/dev/null", "./spec/"},
		[]string{"test_formats/", "spec/"})

	// Phase 6: run the surviving tests, parse PASS/FAIL into the result.
	cmd := exec.Command("go", "test", "-count=1", "-v", "./spec/")
	cmd.Dir = outDir
	testOut, _ := cmd.CombinedOutput()
	parseGoTestOutput(string(testOut), &result)

	result.log(t)
}

// dropUntilBuilds repeatedly runs `buildCmd` in outDir, removing any generated
// file mentioned in compiler-error lines whose path starts with one of the
// given prefixes. Returns the number of files dropped. Each iteration must
// make progress (drop at least one file) or the test fatals.
func dropUntilBuilds(t *testing.T, outDir, phase string, buildCmd, prefixes []string) int {
	t.Helper()
	dropped := 0
	for range 50 {
		cmd := exec.Command(buildCmd[0], buildCmd[1:]...)
		cmd.Dir = outDir
		out, err := cmd.CombinedOutput()
		if err == nil {
			return dropped
		}
		n := dropBadFiles(outDir, out, prefixes)
		if n == 0 {
			t.Fatalf("%s failed with no identifiable files to remove:\n%s", phase, string(out))
		}
		dropped += n
	}
	t.Fatalf("%s did not converge after 50 iterations", phase)
	return dropped
}

// dropBadFiles parses Go compiler-error output, deletes any file whose path
// starts with one of the given prefixes, and returns the number deleted.
func dropBadFiles(outDir string, buildOut []byte, prefixes []string) int {
	removed := map[string]bool{}
	for line := range strings.SplitSeq(string(buildOut), "\n") {
		var badFile string
		for _, prefix := range prefixes {
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			if i := strings.Index(line, ":"); i > 0 {
				badFile = line[:i]
			}
			break
		}
		if badFile == "" || removed[badFile] {
			continue
		}
		_ = os.Remove(filepath.Join(outDir, badFile))
		removed[badFile] = true
	}
	return len(removed)
}

// parseGoTestOutput scans `go test -v` output, classifying PASS/FAIL lines as
// base-test (runOK/runErr) vs *_Roundtrip (roundOK/roundFail).
func parseGoTestOutput(output string, result *specResult) {
	type status struct{ pass, fail bool }
	bases := map[string]*status{}
	rts := map[string]*status{}
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		var pass bool
		var rest string
		if r, ok := strings.CutPrefix(line, "--- PASS: "); ok {
			pass = true
			rest = r
		} else if r, ok := strings.CutPrefix(line, "--- FAIL: "); ok {
			rest = r
		} else {
			continue
		}
		name := strings.Fields(rest)[0]
		target := bases
		if base, isRT := strings.CutSuffix(name, "_Roundtrip"); isRT {
			name = base
			target = rts
		}
		s, ok := target[name]
		if !ok {
			s = &status{}
			target[name] = s
		}
		if pass {
			s.pass = true
		} else {
			s.fail = true
		}
	}
	for _, s := range bases {
		switch {
		case s.fail:
			result.runErr++
		case s.pass:
			result.runOK++
		}
	}
	for _, s := range rts {
		switch {
		case s.fail:
			result.roundFail++
		case s.pass:
			result.roundOK++
		}
	}
}

// writeGoTestScaffolding writes the go.mod files and custom-process source
// files that the generated tests depend on at compile/run time.
func writeGoTestScaffolding(t *testing.T, outDir, formatsOutDir string) {
	t.Helper()
	for name, content := range goTestCustomProcesses {
		require.NoError(t, os.WriteFile(filepath.Join(formatsOutDir, name), []byte(content), 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(outDir, "go.mod"), []byte(goSpecModuleMod), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(formatsOutDir, "go.mod"), []byte(goFormatsModuleMod), 0o644))
}

const goSpecModuleMod = `module spec_test

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

const goFormatsModuleMod = `module test_formats

go 1.24.0

require (
	github.com/jchw-forks/kaitai_struct_go_runtime v0.0.0-20260517224813-0f63727a30a6
)

require (
	golang.org/x/text v0.34.0 // indirect
)
`

// goTestCustomProcesses are the custom-process Go sources referenced by upstream
// KSTs. They live here rather than under testdata so the test stays self-contained.
var goTestCustomProcesses = map[string]string{
	"custom_fx_no_args.go": `package test_formats

type CustomFxNoArgs struct{}

func NewCustomFxNoArgs() *CustomFxNoArgs { return &CustomFxNoArgs{} }

func (p *CustomFxNoArgs) Decode(data []byte) []byte {
	r := make([]byte, len(data)+2)
	r[0] = '_'
	copy(r[1:], data)
	r[len(data)+1] = '_'
	return r
}

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

type MyCustomFx struct{ key int }

func NewMyCustomFx(key int, flag bool, _ []byte) *MyCustomFx {
	k := key
	if !flag {
		k = -key
	}
	return &MyCustomFx{key: k}
}

func (p *MyCustomFx) Decode(data []byte) []byte {
	r := make([]byte, len(data))
	for i, b := range data {
		r[i] = byte((int(b) + p.key) % 0x100)
	}
	return r
}

func (p *MyCustomFx) Encode(data []byte) []byte {
	r := make([]byte, len(data))
	for i, b := range data {
		r[i] = byte((int(b) - p.key + 0x100) % 0x100)
	}
	return r
}
`,
	"custom_fx.go": `package test_formats

type CustomFx struct{}

func NewCustomFx(_ int) *CustomFx { return &CustomFx{} }

func (p *CustomFx) Decode(data []byte) []byte {
	r := make([]byte, len(data)+2)
	r[0] = '_'
	copy(r[1:], data)
	r[len(data)+1] = '_'
	return r
}

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
