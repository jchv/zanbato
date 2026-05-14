package zanbato

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/jchv/zanbato/kaitai/emitter/golang"
	"github.com/jchv/zanbato/kaitai/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodeGeneration(t *testing.T) {
	t.Parallel()

	var m sync.RWMutex

	matches, err := filepath.Glob("internal/third_party/kaitai_struct_tests/formats/*.ksy")
	require.NoError(t, err)

	knownFailing := map[string]struct{}{}
	newlyFailing := []string{}
	newlyPassing := []string{}
	t.Cleanup(func() {
		m.Lock()
		if len(knownFailing) > 0 {
			t.Errorf("Unknown tests in known failing list: %v", knownFailing)
		}
		if len(newlyFailing) > 0 {
			t.Errorf("New tests failing: %#v", newlyFailing)
		}
		if len(newlyPassing) > 0 {
			t.Errorf("New tests passing: %#v", newlyPassing)
		}
		m.Unlock()
	})

	for _, match := range matches {
		match := match
		name := filepath.Base(match)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tf := func() {
				resolver := resolve.NewOSResolverWithPaths([]string{
					"internal/third_party/kaitai_struct_formats",
					"internal/third_party/kaitai_struct_tests/formats",
					"internal/third_party/kaitai_struct_tests/formats/ks_path",
				})
				emitter := golang.NewEmitter("test_formats", resolver)
				basename, struc, err := resolver.Resolve("", match)
				if err != nil {
					panic(fmt.Errorf("error resolving root struct: %w", err))
				}

				outDir := t.TempDir()
				artifacts := emitter.Emit(basename, struc)
				for _, artifact := range artifacts {
					outname := filepath.Join(outDir, artifact.Filename)
					if err := os.WriteFile(outname, artifact.Body, 0o644); err != nil {
						panic(fmt.Errorf("error writing %s: %w", outname, err))
					}
				}
			}

			m.RLock()
			_, ok := knownFailing[name]
			m.RUnlock()
			if ok {
				if !assert.Panics(t, tf) {
					m.Lock()
					newlyPassing = append(newlyPassing, name)
					m.Unlock()
				}
				m.Lock()
				delete(knownFailing, name)
				m.Unlock()
			} else if !assert.NotPanics(t, tf) {
				m.Lock()
				newlyFailing = append(newlyFailing, name)
				m.Unlock()
			}
		})
	}
}
