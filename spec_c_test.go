package zanbato

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jchv/zanbato/kaitai"
	emitterc "github.com/jchv/zanbato/kaitai/emitter/c"
	"github.com/jchv/zanbato/kaitai/kst"
	"github.com/jchv/zanbato/kaitai/resolve"
	"github.com/jchv/zanbato/kaitai/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

func TestSpecC(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("cc"); err != nil {
		t.Skip("cc not on PATH; skipping C spec tests")
	}

	wd, err := os.Getwd()
	require.NoError(t, err)

	outDir := filepath.Join(wd, ".test", "c")
	require.NoError(t, os.RemoveAll(outDir))
	require.NoError(t, os.MkdirAll(outDir, 0o755))
	require.NoError(t, copyFileForTest(
		filepath.Join(wd, "runtime", "c", "zanbato.h"),
		filepath.Join(outDir, "zanbato.h"),
	))

	// Phase 1: emit C parsers for every KSY.
	resolvedStructs := map[string]*kaitai.Struct{}
	emittedIDs := map[string]struct{}{}
	for _, src := range testSources {
		matches, err := filepath.Glob(filepath.Join(src.formatsDir, "*.ksy"))
		if err != nil {
			continue
		}
		for _, m := range matches {
			resolver := resolve.NewOSResolverWithPaths(testResolverPaths)
			em := emitterc.NewEmitter(resolver)
			em.SetCompat(kaitai.KaitaiStruct_0_11)
			basename, struc, err := resolver.Resolve("", m)
			if err != nil {
				continue
			}
			resolvedStructs[string(struc.ID)] = struc
			resolveImports(resolver, resolvedStructs, basename, struc)
			artifacts, panicMsg := em.EmitSafe(basename, struc)
			if panicMsg != "" {
				continue
			}
			for _, a := range artifacts {
				_ = os.WriteFile(filepath.Join(outDir, a.Filename), a.Body, 0o644)
			}
			emittedIDs[string(struc.ID)] = struct{}{}
		}
	}

	// Phase 2: gather KST work.
	type kstWork struct {
		absSrcDir string
		kstPath   string
	}
	var works []kstWork
	for _, src := range testSources {
		matches, err := filepath.Glob(filepath.Join(src.kstDir, "*.kst"))
		if err != nil {
			continue
		}
		absSrcDir := filepath.Join(wd, src.srcDir)
		for _, kf := range matches {
			works = append(works, kstWork{absSrcDir: absSrcDir, kstPath: kf})
		}
	}

	result := specResult{
		parsersOK: len(emittedIDs),
		total:     len(works),
	}

	// Phase 3: per-KST emit, compile, run in parallel.
	var (
		emitErr, compileErr, runErr, runOK atomic.Int64
		roundOK, roundFail                 atomic.Int64
	)
	g, _ := errgroup.WithContext(context.Background())
	g.SetLimit(max(runtime.NumCPU(), 1))

	for _, w := range works {
		g.Go(func() error {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("emit panic %s: %v", w.kstPath, r)
					emitErr.Add(1)
				}
			}()

			spec, err := kst.ParseFile(w.kstPath)
			if err != nil {
				emitErr.Add(1)
				return nil
			}
			ks := resolvedStructs[spec.ID]
			if ks == nil {
				emitErr.Add(1)
				return nil
			}
			parserSrc := filepath.Join(outDir, strings.ToLower(string(ks.ID))+".c")
			if _, err := os.Stat(parserSrc); err != nil {
				emitErr.Add(1)
				return nil
			}

			resolver := resolve.NewOSResolverWithPaths(testResolverPaths)
			em := emitterc.NewEmitter(resolver)
			kstEmitter := emitterc.NewKSTEmitter(w.absSrcDir)
			kstEmitter.ResolvedStructs = resolvedStructs
			filename, body := kstEmitter.Emit(spec, ks, em)
			testPath := filepath.Join(outDir, filename)
			if err := os.WriteFile(testPath, body, 0o644); err != nil {
				emitErr.Add(1)
				return nil
			}

			exe := filepath.Join(outDir, spec.ID+"_test")
			args := []string{
				"-std=c99", "-O0", "-g",
				"-I", outDir, "-I", "runtime/c",
				testPath, parserSrc, "runtime/c/zb_test_prereqs.c",
				"-o", exe, "-lz",
			}
			linkDeps(&args, outDir, resolvedStructs, ks, map[string]bool{
				parserSrc: true,
				testPath:  true,
			})

			out, err := exec.Command("cc", args...).CombinedOutput()
			if err != nil {
				t.Errorf("compile error %s: %s", w.kstPath, string(out))
				compileErr.Add(1)
				return nil
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			runOut, runErrL := exec.CommandContext(ctx, exe).CombinedOutput()
			if ctx.Err() == context.DeadlineExceeded || runErrL != nil {
				runErr.Add(1)
				return nil
			}
			runOK.Add(1)
			outStr := string(runOut)
			if strings.Contains(outStr, "roundtrip OK") {
				roundOK.Add(1)
			} else if strings.Contains(outStr, "roundtrip mismatch") {
				roundFail.Add(1)
			}
			return nil
		})
	}
	_ = g.Wait()

	result.emitErr = int(emitErr.Load())
	result.compileErr = int(compileErr.Load())
	result.runErr = int(runErr.Load())
	result.runOK = int(runOK.Load())
	result.roundOK = int(roundOK.Load())
	result.roundFail = int(roundFail.Load())
	result.log(t)
}

// linkDeps appends all dependency .c files (imports + opaque-type refs) to args
// so that cc has every translation unit it needs to link the test executable.
func linkDeps(args *[]string, outDir string, resolved map[string]*kaitai.Struct, ks *kaitai.Struct, linked map[string]bool) {
	visited := map[string]bool{string(ks.ID): true}
	link := func(name string) {
		p := filepath.Join(outDir, name+".c")
		if linked[p] {
			return
		}
		if _, err := os.Stat(p); err == nil {
			*args = append(*args, p)
			linked[p] = true
		}
	}
	var walk func(s *kaitai.Struct)
	walk = func(s *kaitai.Struct) {
		for _, imp := range s.Meta.Imports {
			impName := strings.ToLower(filepath.Base(imp))
			link(impName)
			if impStruct := resolved[impName]; impStruct != nil && !visited[string(impStruct.ID)] {
				visited[string(impStruct.ID)] = true
				walk(impStruct)
			}
		}
		if !s.Meta.OpaqueTypes {
			return
		}
		walkAttr := func(a *kaitai.Attr) {
			if a == nil {
				return
			}
			var names []string
			if a.Type.TypeRef != nil && a.Type.TypeRef.Kind == types.User && a.Type.TypeRef.User != nil {
				names = append(names, a.Type.TypeRef.User.Name)
			}
			if a.Type.TypeSwitch != nil {
				for _, c := range a.Type.TypeSwitch.Cases {
					if c.Kind == types.User && c.User != nil {
						names = append(names, c.User.Name)
					}
				}
			}
			for _, n := range names {
				if strings.Contains(n, "::") {
					continue
				}
				impName := strings.ToLower(n)
				link(impName)
				if impStruct := resolved[impName]; impStruct != nil && !visited[string(impStruct.ID)] {
					visited[string(impStruct.ID)] = true
					walk(impStruct)
				}
			}
		}
		for _, a := range s.Seq {
			walkAttr(a)
		}
		for _, a := range s.Instances {
			walkAttr(a)
		}
		for _, child := range s.Structs {
			walk(child)
		}
	}
	walk(ks)
}
