package moat

import "testing"

// TestSanitizeForDisplay walks the full attack matrix we care about at the
// enrich boundary. Every case documents the concrete threat it defends
// against so a reviewer can tell, at a glance, whether the test's intent
// and the code's behavior align.
func TestSanitizeForDisplay(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		// Baseline — empty and benign inputs round-trip unchanged.
		{"empty", "", ""},
		{"ascii only", "compromised key material", "compromised key material"},
		{"non-ascii printable preserved", "café résumé naïve", "café résumé naïve"},

		// ANSI CSI — the SGR color sequence used by most terminal-attack
		// PoCs. A malicious registry could paint "Recalled" in green.
		{
			"csi sgr colored text",
			"\x1b[31mdanger\x1b[0m message",
			"danger message",
		},
		{
			"csi cursor move",
			"line1\x1b[2;5Hline2",
			"line1line2",
		},

		// ANSI OSC — window-title and hyperlink sequences can inject
		// arbitrary shell commands into terminals that honor them. Stripping
		// is total: the sequence (terminator included) disappears with no
		// whitespace substitution, so adjacent visible runes close up.
		{
			"osc window title",
			"a\x1b]0;spoofed title\x07b",
			"ab",
		},
		{
			"osc with st terminator",
			"a\x1b]8;;http://evil\x1b\\click\x1b]8;;\x1b\\b",
			"aclickb",
		},

		// Bidi overrides — classic filename-spoof vector (U+202E). Any of
		// the directional control codepoints must disappear entirely.
		{
			"rlo override",
			"evil\u202Eexe.txt",
			"evilexe.txt",
		},
		{
			"all bidi range",
			"a\u202Ab\u202Bc\u202Cd\u202De\u202Ef\u2066g\u2067h\u2068i\u2069j",
			"abcdefghij",
		},

		// Line separators — U+2028 / U+2029 bypass naive newline stripping.
		{
			"ls and ps",
			"a\u2028b\u2029c",
			"abc",
		},

		// C0 controls — tab / newline / CR normalize to spaces; other C0
		// bytes drop silently. Multiple-in-a-row collapse via the final
		// whitespace pass.
		{
			"tab and newline normalized",
			"line1\tline2\nline3\rline4",
			"line1 line2 line3 line4",
		},
		{
			"null byte dropped",
			"a\x00b\x01c",
			"abc",
		},
		{
			"multiple whitespace collapses",
			"a\t\t\n\nb",
			"a b",
		},

		// DEL and C1 controls — terminal-emulator territory, must strip.
		{
			"del byte",
			"a\x7Fb",
			"ab",
		},
		{
			"c1 controls",
			"a\x80b\x9Fc",
			"abc",
		},

		// Invalid UTF-8 replacement rune — we drop it rather than surface
		// "publisher sent garbage" to the user.
		{
			"replacement rune",
			"a\uFFFDb",
			"ab",
		},

		// Trim — leading/trailing whitespace falls away after collapse.
		{
			"leading and trailing whitespace",
			"  \t hello \n  ",
			"hello",
		},
		{
			"pure whitespace returns empty",
			" \t\n ",
			"",
		},

		// Multi-layer attack — a real-world malicious reason combining
		// color, bidi, and control bytes. Must reduce to clean text.
		{
			"layered attack",
			"\x1b[31;1mverified\x1b[0m\u202E by registry\x00",
			"verified by registry",
		},

		// Orphan ESC — the ESC byte alone without a valid dialect must
		// also drop. Safety over preservation.
		{
			"orphan escape",
			"a\x1bb",
			"a",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := SanitizeForDisplay(tc.in)
			if got != tc.want {
				t.Errorf("SanitizeForDisplay(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestSanitizeForDisplay_Idempotent proves that feeding the output back in
// produces the same result. Any future edit that accidentally introduces
// stateful behavior (e.g. a LUT that gets mutated during scan) would break
// this invariant.
func TestSanitizeForDisplay_Idempotent(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"plain text",
		"\x1b[31mred\x1b[0m",
		"café résumé",
		"a\u202Eb",
		"line1\nline2",
	}
	for _, in := range inputs {
		once := SanitizeForDisplay(in)
		twice := SanitizeForDisplay(once)
		if once != twice {
			t.Errorf("not idempotent for %q: once=%q twice=%q", in, once, twice)
		}
	}
}
