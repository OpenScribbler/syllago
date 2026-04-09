package capmon

import (
	"fmt"
	"strings"
)

// yamlStructuralChars is the set of characters that have structural meaning
// in YAML when they appear at the start of a scalar.
const yamlStructuralChars = "{}[]:#&*!|>@%`"

// SanitizeExtractedString normalizes an extracted string value before writing
// to extracted.json. Exported for use by all extractor implementations.
func SanitizeExtractedString(s string) string {
	// 1. Strip trailing newlines
	s = strings.TrimRight(s, "\n\r")
	// 2. Cap at 512 bytes, append "[truncated]" if exceeded
	if len(s) > 512 {
		s = s[:500] + " [truncated]"
	}
	// 3. Percent-encode YAML structural chars in first non-whitespace position
	trimmed := strings.TrimLeft(s, " \t")
	if len(trimmed) > 0 && strings.ContainsRune(yamlStructuralChars, rune(trimmed[0])) {
		indent := s[:len(s)-len(trimmed)]
		s = indent + fmt.Sprintf("%%%02X", trimmed[0]) + trimmed[1:]
	}
	return s
}
