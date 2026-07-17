package moat

import "github.com/OpenScribbler/syllago/cli/internal/moathash"

// FileHash returns the file-level MOAT hash as lowercase hex without an
// algorithm prefix, classifying the file with the same text/binary heuristic
// used by ContentHash.
func FileHash(path string) (string, error) {
	return moathash.FileHash(path)
}

// CanonicalText returns the bytes-level canonical text form used by MOAT:
// strip a leading UTF-8 BOM, then normalize CRLF and lone CR to LF.
func CanonicalText(data []byte) []byte {
	return moathash.CanonicalText(data)
}
