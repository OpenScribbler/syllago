package moat

import "bytes"

// FileHash returns the file-level MOAT hash as lowercase hex without an
// algorithm prefix, classifying the file with the same text/binary heuristic
// used by ContentHash.
func FileHash(path string) (string, error) {
	text, err := isText(path)
	if err != nil {
		return "", err
	}
	if text {
		return hashText(path)
	}
	return hashBinary(path)
}

// CanonicalText returns the bytes-level canonical text form used by MOAT:
// strip a leading UTF-8 BOM, then normalize CRLF and lone CR to LF.
func CanonicalText(data []byte) []byte {
	chunk := bytes.TrimPrefix(data, utf8BOM)
	var out bytes.Buffer
	pendingCR := false
	if len(chunk) > 0 {
		normalizeChunk(&out, chunk, &pendingCR)
	}
	if pendingCR {
		_ = out.WriteByte(0x0A)
	}
	return out.Bytes()
}
