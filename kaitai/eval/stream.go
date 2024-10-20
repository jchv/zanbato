package eval

import (
	"io"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
)

type Stream = kaitai.Stream

func NewStream(r io.ReadSeeker) *Stream {
	return kaitai.NewStream(r)
}

func NewValidationNotEqualError(expected interface{}, actual interface{}, io *Stream, srcPath string) kaitai.ValidationNotEqualError {
	return kaitai.NewValidationNotEqualError(expected, actual, io, srcPath)
}
