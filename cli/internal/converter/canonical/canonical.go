// Package canonical provides the single normalization helper used at
// write, scan, and search time for all monolithic-file byte paths (D12).
package canonical

import "bytes"

// Normalize applies the canonical byte form per D12:
// - CRLF -> LF
// - Strip leading UTF-8 BOM
// - Exactly one trailing newline
// Trailing whitespace on lines, indentation, unicode, and casing are preserved.
func Normalize(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	b = bytes.TrimRight(b, "\n")
	return append(b, '\n')
}
