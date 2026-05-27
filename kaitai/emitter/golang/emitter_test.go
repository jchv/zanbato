package golang

import (
	"strings"
	"testing"

	"github.com/jchv/zanbato/kaitai"
	"github.com/jchv/zanbato/kaitai/emitter"
	"github.com/jchv/zanbato/kaitai/resolve"
)

func TestEmitterReuseResetsState(t *testing.T) {
	e := NewEmitter("test_formats", resolve.NewOSResolver())

	first := &kaitai.Struct{ID: "first"}
	second := &kaitai.Struct{ID: "second"}

	firstArtifacts := e.Emit("first.ksy", first)
	requireArtifact(t, firstArtifacts, "first.go")

	secondArtifacts := e.Emit("second.ksy", second)
	requireArtifact(t, secondArtifacts, "second.go")
	rejectArtifact(t, secondArtifacts, "first.go")

	// Reusing the emitter for a previously emitted struct should also work.
	// This catches stale visited-state bugs, not just stale artifact slices.
	againArtifacts := e.Emit("first.ksy", first)
	requireArtifact(t, againArtifacts, "first.go")
	rejectArtifact(t, againArtifacts, "second.go")
}

func TestEncodingNormalization(t *testing.T) {
	e := NewEmitter("test_formats", resolve.NewOSResolver())
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "utf8 hyphen", in: "UTF-8", want: "UTF8"},
		{name: "utf8 underscore", in: "utf_8", want: "UTF8"},
		{name: "utf16 le hyphen", in: "UTF-16LE", want: "UTF16LE"},
		{name: "utf16 le underscore", in: "utf_16_le", want: "UTF16LE"},
		{name: "windows code page", in: "windows-1252", want: "WINDOWS1252"},
		{name: "iso alias", in: "iso_8859-1", want: "ISO88591"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := e.normalizeEncoding(test.in); got != test.want {
				t.Fatalf("normalizeEncoding(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}

	if e.needsEncodingConversion("UTF_8") {
		t.Fatal("UTF_8 should not require an explicit decoder")
	}
	if !e.needsEncodingConversion("UTF_16_LE") {
		t.Fatal("UTF_16_LE should require an explicit decoder")
	}

}

func TestUnsupportedEncodingGeneratesRuntimeErrorExpression(t *testing.T) {
	e := NewEmitter("test_formats", resolve.NewOSResolver())
	unit := &goUnit{imports: map[string]string{}}

	decoder := e.encodingDecoder(unit, "KOI8-R")
	if !strings.Contains(decoder, "unsupported string encoding: %s") ||
		!strings.Contains(decoder, `"KOI8R"`) ||
		!strings.Contains(decoder, "*encoding.Decoder") {
		t.Fatalf("unexpected unsupported decoder expression: %s", decoder)
	}

	encoder := e.encodingEncoder(unit, "KOI8_R")
	if !strings.Contains(encoder, "unsupported string encoding: %s") ||
		!strings.Contains(encoder, `"KOI8R"`) ||
		!strings.Contains(encoder, "*encoding.Encoder") {
		t.Fatalf("unexpected unsupported encoder expression: %s", encoder)
	}

	if unit.imports["fmt"] != "fmt" {
		t.Fatalf("unsupported encoding expression should import fmt, got imports %v", unit.imports)
	}
	if unit.imports["golang.org/x/text/encoding"] != "encoding" {
		t.Fatalf("unsupported encoding expression should import x/text/encoding, got imports %v", unit.imports)
	}
}

func requireArtifact(t *testing.T, artifacts []emitter.Artifact, filename string) {
	t.Helper()
	if !hasArtifact(artifacts, filename) {
		t.Fatalf("expected artifact %q, got %v", filename, artifactNames(artifacts))
	}
}

func rejectArtifact(t *testing.T, artifacts []emitter.Artifact, filename string) {
	t.Helper()
	if hasArtifact(artifacts, filename) {
		t.Fatalf("unexpected artifact %q in %v", filename, artifactNames(artifacts))
	}
}

func hasArtifact(artifacts []emitter.Artifact, filename string) bool {
	for _, artifact := range artifacts {
		if artifact.Filename == filename {
			return true
		}
	}
	return false
}

func artifactNames(artifacts []emitter.Artifact) []string {
	names := make([]string, len(artifacts))
	for i, artifact := range artifacts {
		names[i] = artifact.Filename
	}
	return names
}
