package emitter

import "github.com/jchv/zanbato/kaitai"

// Artifact represents a single file emitted by the emitter.
type Artifact struct {
	Filename string
	Body     []byte
}

// Emitter is a type that can emit source code for kaitai structs.
type Emitter interface {
	Emit(Struct kaitai.Struct) []Artifact
}
