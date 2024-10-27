package eval

import (
	"io"

	"github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"
)

type StreamReader interface {
	io.ReadSeeker
	io.ReaderAt
}

type Stream = kaitai.Stream

func NewStream(r StreamReader) *Stream {
	return kaitai.NewStream(r)
}

func NewSubStream(stream *kaitai.Stream, off int64, n int64) *Stream {
	return kaitai.NewStream(io.NewSectionReader(stream.ReadSeeker.(io.ReaderAt), off, n))
}

func NewValidationNotEqualError(expected interface{}, actual interface{}, io *Stream, srcPath string) kaitai.ValidationNotEqualError {
	return kaitai.NewValidationNotEqualError(expected, actual, io, srcPath)
}
