package eval

import (
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/unicode"
)

// getEncoding returns the text encoding for a given encoding name.
func getEncoding(name string) encoding.Encoding {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(name, "-", ""), "_", ""))
	switch normalized {
	case "UTF8", "":
		return nil // UTF-8 is native, no decoding needed
	case "ASCII":
		return nil // ASCII is a subset of UTF-8
	case "UTF16LE":
		return unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM)
	case "UTF16BE":
		return unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
	case "SHIFTJIS", "SJIS":
		return japanese.ShiftJIS
	case "IBM437", "CP437":
		return charmap.CodePage437
	case "ISO88591", "LATIN1":
		return charmap.ISO8859_1
	case "WINDOWS1252", "CP1252":
		return charmap.Windows1252
	default:
		return nil // unknown encoding, treat as raw bytes
	}
}

// decodeString decodes raw bytes to a Go string using the given encoding name.
func decodeString(data []byte, enc string) string {
	e := getEncoding(enc)
	if e == nil {
		return string(data) // UTF-8 or unknown
	}
	decoded, err := e.NewDecoder().Bytes(data)
	if err != nil {
		return string(data) // fallback to raw
	}
	return string(decoded)
}
