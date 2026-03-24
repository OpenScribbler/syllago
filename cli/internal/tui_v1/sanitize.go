package tui_v1

import (
	"strings"
	"unicode/utf8"
)

// StripControlChars removes ANSI escape sequences and control characters from a string.
// Preserves newlines (\n) and tabs (\t) but strips all other control characters including:
//   - ESC (0x1B) and all escape sequences (CSI, OSC, etc.)
//   - C0 controls (0x00-0x1F except \n and \t)
//   - DEL (0x7F)
//   - C1 controls (0x80-0x9F)
//
// This prevents terminal escape injection attacks like clipboard poisoning (OSC 52),
// visual spoofing (cursor movement), and title injection.
func StripControlChars(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	i := 0
	for i < len(s) {
		// Handle multi-byte UTF-8 starting at >= 0x80
		if s[i] >= 0x80 {
			r, size := utf8.DecodeRuneInString(s[i:])

			// Skip C1 controls (0x80-0x9F)
			if r >= 0x80 && r <= 0x9F {
				i += size
				continue
			}

			// Valid UTF-8, include it
			result.WriteString(s[i : i+size])
			i += size
			continue
		}

		// Single-byte ASCII handling
		r := rune(s[i])

		// ESC character — skip entire escape sequence
		if r == 0x1B {
			i++
			i = skipEscapeSequence(s, i)
			continue
		}

		// C0 controls: skip everything except \n and \t
		if r < 0x20 {
			if r == '\n' || r == '\t' {
				result.WriteByte(byte(r))
			}
			i++
			continue
		}

		// DEL (0x7F)
		if r == 0x7F {
			i++
			continue
		}

		// Normal printable ASCII
		result.WriteByte(byte(r))
		i++
	}

	return result.String()
}

// skipEscapeSequence advances past an ANSI escape sequence starting after the ESC byte.
func skipEscapeSequence(s string, start int) int {
	if start >= len(s) {
		return start
	}

	// OSC sequences: ESC ] ... ST (ST = ESC \ or BEL 0x07)
	if s[start] == ']' {
		for i := start + 1; i < len(s); i++ {
			if s[i] == 0x07 { // BEL
				return i + 1
			}
			if s[i] == 0x1B && i+1 < len(s) && s[i+1] == '\\' { // ST
				return i + 2
			}
		}
		return len(s)
	}

	// CSI sequences: ESC [ ... (letter or @)
	if s[start] == '[' {
		for i := start + 1; i < len(s); i++ {
			ch := s[i]
			if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '@' {
				return i + 1
			}
		}
		return len(s)
	}

	// Other escape sequences (single character after ESC)
	return start + 1
}
