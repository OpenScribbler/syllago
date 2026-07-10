package parse

import (
	"bytes"
	"strings"
)

// SplitFrontmatter splits content into its YAML frontmatter block and body.
//
// CRLF line endings are normalized to LF before scanning. A well-formed
// block starts with an opening "---\n" fence at byte 0 and ends at the next
// "---\n" fence; ok reports whether one was found. When ok is true,
// yamlBytes holds the bytes between the fences (ready for yaml.Unmarshal)
// and body holds the whitespace-trimmed content after the closing fence.
// When ok is false, yamlBytes is nil and body is the whole normalized,
// whitespace-trimmed content — callers that treat missing frontmatter as
// valid plain-markdown input can use body directly.
//
// A closing fence must be followed by a newline; a bare "---" at end of
// file is not recognized. That matches the historical behavior of the
// converter parsers this helper replaced (catalog.ParseFrontmatterWithBody
// intentionally diverges and accepts the EOF form).
func SplitFrontmatter(content []byte) (yamlBytes []byte, body string, ok bool) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))

	opening := []byte("---\n")
	if !bytes.HasPrefix(normalized, opening) {
		return nil, strings.TrimSpace(string(normalized)), false
	}

	rest := normalized[len(opening):]
	closingIdx := bytes.Index(rest, opening)
	if closingIdx == -1 {
		return nil, strings.TrimSpace(string(normalized)), false
	}

	return rest[:closingIdx], strings.TrimSpace(string(rest[closingIdx+len(opening):])), true
}
