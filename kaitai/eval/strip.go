package eval

import (
	"bytes"
	"fmt"
	"io"

	kaitai_io "github.com/jchw-forks/kaitai_struct_go_runtime/kaitai"
)

// stripBytes strips a single-byte terminator and/or pad-right from data.
func stripBytes(data []byte, terminator int, padRight int, include bool) []byte {
	if terminator >= 0 {
		if i := bytes.IndexByte(data, byte(terminator)); i != -1 {
			if include {
				return data[:i+1]
			}
			return data[:i]
		}
	}
	// Terminator not found (or not specified): strip pad bytes from right
	padByte := padRight
	if padByte < 0 && terminator >= 0 {
		padByte = terminator
	}
	if padByte >= 0 {
		data = kaitai_io.BytesStripRight(data, byte(padByte))
	}
	return data
}

// stripPadRightMulti strips trailing pad bytes from a UTF-16-encoded byte
// sequence. KS semantics for pad-right on multi-byte encodings is to strip
// raw bytes from the right (not aligned code units), matching the
// upstream Java/Python implementations.
func stripPadRightMulti(data []byte, pad byte) []byte {
	for len(data) > 0 && data[len(data)-1] == pad {
		data = data[:len(data)-1]
	}
	return data
}

// stripBytesMulti strips a 2-byte terminator from data (for UTF-16).
// Searches for aligned 2-byte sequences of [term, term].
func stripBytesMulti(data []byte, term byte, include bool) []byte {
	termSeq := []byte{term, term}
	for i := 0; i+1 < len(data); i += 2 {
		if data[i] == termSeq[0] && data[i+1] == termSeq[1] {
			if include {
				return data[:i+2]
			}
			return data[:i]
		}
	}
	return data
}

// readBytesTermMulti reads bytes until a multi-byte terminator sequence is found.
// Used for UTF-16 where the null terminator is 2 bytes.
func readBytesTermMulti(stream *Stream, term []byte, include bool, consume bool) ([]byte, error) {
	termLen := len(term)
	var result []byte
	buf := make([]byte, termLen)
	for {
		_, err := io.ReadFull(stream, buf)
		if err != nil {
			// EOF before finding terminator - return what we have
			return result, nil
		}
		if bytes.Equal(buf, term) {
			if include {
				result = append(result, buf...)
			}
			if !consume {
				// Seek back past the terminator.
				if _, err := stream.Seek(-int64(termLen), io.SeekCurrent); err != nil {
					return result, fmt.Errorf("readBytesTermMulti: seek back past terminator: %w", err)
				}
			}
			return result, nil
		}
		result = append(result, buf...)
	}
}
