package kaitai

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNegative(t *testing.T) {
	dir := "../testdata/negative"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	for _, ent := range entries {
		if ent.IsDir() || filepath.Ext(ent.Name()) != ".ksy" {
			continue
		}
		ent := ent
		t.Run(ent.Name(), func(t *testing.T) {
			path := filepath.Join(dir, ent.Name())
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("opening %s: %v", path, err)
			}
			defer func() { _ = f.Close() }()
			if _, err := ParseStruct(f); err == nil {
				t.Errorf("expected %s to fail parsing, but it succeeded", ent.Name())
			}
		})
	}
}
