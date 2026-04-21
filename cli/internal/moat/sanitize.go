package moat

// Display-string sanitization for publisher-controlled fields (MOAT Phase 2c,
// ADR 0007 §Trust Surfacing).
//
// Revocation reasons, details URLs, and publisher identities flow from
// attacker-controlled inputs (a malicious publisher can stuff ANSI escapes,
// bidi overrides, or C0/C1 control bytes into any string field of a
// manifest). Without sanitization those bytes can rewrite terminal state,
// invert glyph meanings, or spoof surrounding UI chrome. The enrich
// boundary is the single chokepoint where every publisher-controlled
// string crosses from the parse layer into display structures, so we
// scrub here once and consumers treat the output as trusted for display.
//
// This is a defense-in-depth layer — the integrity of what the user reads
// is separate from whether the revocation decision itself was sound (the
// trust chain rooted at sigstore proves the latter). We strip rather than
// quote-escape because the consumer surfaces (TUI metapanel, toast text,
// `syllago trust-status` output) are not designed to render escaped
// control-byte representations; stripping keeps the string readable while
// removing the attack surface.

import (
	"strings"
	"unicode"
)

// SanitizeForDisplay returns s with terminal-unsafe and bidi-ambiguous
// characters removed. The output is always single-line, safe to splice
// into TUI cells, toast text, and stdout writes without further escaping.
//
// What gets stripped:
//   - ANSI CSI (ESC '[' ... final-byte) and OSC (ESC ']' ... BEL | ESC '\')
//     sequences, plus any orphan ESC bytes that survive.
//   - C0 control bytes (0x00–0x1F) except \t (0x09) which is normalized to
//     a single space. \n (0x0A) and \r (0x0D) are normalized to a single
//     space so callers get single-line output.
//   - DEL (0x7F) and C1 control bytes (0x80–0x9F).
//   - Unicode bidi override codepoints (U+202A–U+202E, U+2066–U+2069).
//   - Line / paragraph separators (U+2028, U+2029).
//   - Replacement character (U+FFFD) that Go's utf8 decoding produces for
//     invalid byte sequences — we drop it rather than surface corruption.
//
// What's preserved:
//   - All printable Unicode (including non-ASCII letters, CJK, combining
//     marks), intentionally. A user named "café" still renders correctly.
//
// Multiple whitespace characters after normalization collapse to a single
// space, and leading/trailing whitespace is trimmed. Empty input or input
// that sanitizes to pure whitespace returns "".
func SanitizeForDisplay(s string) string {
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))

	runes := []rune(s)
	i := 0
	for i < len(runes) {
		r := runes[i]

		// Strip ANSI escape sequences wholesale. Swallow the ESC plus
		// whichever dialect follows — CSI (ESC [ ... final-byte 0x40-0x7E)
		// or OSC (ESC ] ... terminator BEL or ESC \). Also handle bare
		// ESC and unknown dialects by dropping ESC alone and letting the
		// loop continue scrubbing any remaining bytes on their own merits.
		if r == 0x1B { // ESC
			i = skipANSI(runes, i)
			continue
		}

		// Drop bidi override codepoints that can reorder surrounding text.
		// Attack vector: U+202E (RIGHT-TO-LEFT OVERRIDE) makes "evil.exe"
		// render as "exe.live" — classic filename spoof.
		if (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) {
			i++
			continue
		}

		// Drop line/paragraph separators — callers want single-line.
		if r == 0x2028 || r == 0x2029 {
			i++
			continue
		}

		// Drop DEL and C1 controls.
		if r == 0x7F || (r >= 0x80 && r <= 0x9F) {
			i++
			continue
		}

		// Drop Go's UTF-8 replacement rune — appears for invalid input
		// sequences and we don't want to surface "publisher sent garbage."
		if r == 0xFFFD {
			i++
			continue
		}

		// Normalize C0 controls (0x00–0x1F): tab, newline, CR → single
		// space; anything else → drop entirely.
		if r < 0x20 {
			switch r {
			case '\t', '\n', '\r':
				b.WriteByte(' ')
			}
			i++
			continue
		}

		// Everything else — including non-ASCII printable runes — passes
		// through unchanged.
		b.WriteRune(r)
		i++
	}

	// Collapse runs of whitespace and trim. This handles the common case
	// where an attacker tries to inflate output by stuffing tabs or
	// newlines that each normalize to a space.
	return collapseWhitespace(b.String())
}

// skipANSI returns the index immediately after a complete ANSI escape
// sequence starting at runes[i] (which must be ESC). Recognized dialects:
//   - CSI: ESC [ <any> ... <final 0x40-0x7E>
//   - OSC: ESC ] <any> ... (BEL | ESC \)
//   - Single-char escapes: ESC <any one byte>
//
// On malformed or truncated input the function advances past the ESC and
// whatever follows is handled on the next loop iteration. We never copy
// escape bytes into the output — the whole sequence is dropped.
func skipANSI(runes []rune, i int) int {
	// Past the ESC.
	i++
	if i >= len(runes) {
		return i
	}

	switch runes[i] {
	case '[':
		// CSI: consume up to and including the final byte in 0x40-0x7E.
		i++
		for i < len(runes) {
			c := runes[i]
			i++
			if c >= 0x40 && c <= 0x7E {
				return i
			}
		}
		return i
	case ']':
		// OSC: consume up to and including the terminator.
		i++
		for i < len(runes) {
			c := runes[i]
			if c == 0x07 { // BEL
				return i + 1
			}
			if c == 0x1B && i+1 < len(runes) && runes[i+1] == '\\' {
				return i + 2
			}
			i++
		}
		return i
	default:
		// Unknown single-char escape — drop the ESC and its following
		// byte. Safer to drop too much than risk preserving attack bytes.
		return i + 1
	}
}

// collapseWhitespace replaces runs of Unicode whitespace with single
// spaces and trims leading / trailing whitespace.
func collapseWhitespace(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))

	prevSpace := true // pretend previous was space so leading ws drops
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}

	out := b.String()
	if len(out) > 0 && out[len(out)-1] == ' ' {
		out = out[:len(out)-1]
	}
	return out
}
