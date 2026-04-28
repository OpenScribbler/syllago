package telemetry

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// TestPromptCLIConsent_Answers covers every input shape the consent prompt
// must classify correctly. The function defaults to "No" on anything except
// an explicit y/yes (any case, surrounded by whitespace) — locked in here so
// a future refactor can't widen the truthy set unintentionally and start
// recording consent for users who typed garbage.
func TestPromptCLIConsent_Answers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    bool
		wantErr bool
	}{
		// --- Yes branch ---
		{"y_lowercase", "y\n", true, false},
		{"Y_uppercase", "Y\n", true, false},
		{"yes_lowercase", "yes\n", true, false},
		{"yes_mixed_case", "Yes\n", true, false},
		{"YES_uppercase", "YES\n", true, false},
		{"y_with_leading_whitespace", "  y\n", true, false},
		{"y_with_trailing_whitespace", "y   \n", true, false},
		{"yes_no_trailing_newline", "yes", true, false},

		// --- No branch (default) ---
		{"n_lowercase", "n\n", false, false},
		{"N_uppercase", "N\n", false, false},
		{"no_lowercase", "no\n", false, false},
		{"empty_line_defaults_to_no", "\n", false, false},
		{"immediate_eof_defaults_to_no", "", false, false},
		{"whitespace_only", "   \n", false, false},

		// --- Anything-but-y/yes lands in the No branch. The cases below
		// are deliberate: a partial-match like "yeah" must not opt in,
		// because the user clearly typed a word and wasn't shown one of
		// the documented answers. Defaulting these to No protects against
		// false-positive consent from a typo or alternate-language reply.
		{"garbage_text_defaults_to_no", "maybe\n", false, false},
		{"yeah_does_not_match", "yeah\n", false, false},
		{"yep_does_not_match", "yep\n", false, false},
		{"y_with_extra_chars", "y!\n", false, false},
		{"numeric_answer", "1\n", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			got, err := PromptCLIConsent(&out, strings.NewReader(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("PromptCLIConsent err=%v, wantErr=%v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("PromptCLIConsent(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestPromptCLIConsent_NilStreams pins that a nil writer or reader is
// rejected up front rather than panicking. A caller that hits this branch
// has lost stdin or stderr — the only safe response is "no consent" without
// blocking.
func TestPromptCLIConsent_NilStreams(t *testing.T) {
	t.Parallel()

	got, err := PromptCLIConsent(nil, strings.NewReader("y\n"))
	if got || !errors.Is(err, ErrNoTTY) {
		t.Errorf("nil writer: got=%v err=%v, want false + ErrNoTTY", got, err)
	}

	got, err = PromptCLIConsent(&bytes.Buffer{}, nil)
	if got || !errors.Is(err, ErrNoTTY) {
		t.Errorf("nil reader: got=%v err=%v, want false + ErrNoTTY", got, err)
	}
}

// errReader returns a non-EOF error on the first read. Used to verify the
// prompt surfaces unexpected I/O failures instead of silently treating them
// as "no consent" — a silent default would mask broken terminals.
type errReader struct{ err error }

func (r errReader) Read(_ []byte) (int, error) { return 0, r.err }

func TestPromptCLIConsent_ReadError(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	wantErr := errors.New("simulated stdin failure")
	got, err := PromptCLIConsent(&out, errReader{err: wantErr})
	if got {
		t.Errorf("expected false on read error, got true")
	}
	if err == nil || !errors.Is(err, wantErr) {
		t.Errorf("expected wrapped %v, got %v", wantErr, err)
	}
}

// TestPromptCLIConsent_OutputContainsDisclosure pins the user-visible text
// that must appear before the y/N prompt. If a future refactor accidentally
// drops the disclosure (or the prompt text), this test fails — not a CI
// regression we'd notice from green-bar tests otherwise.
func TestPromptCLIConsent_OutputContainsDisclosure(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if _, err := PromptCLIConsent(&out, strings.NewReader("n\n")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := out.String()

	mustContain := []string{
		"syllago needs your help",                  // title from RenderDisclosure
		"What gets collected (only if you opt in)", // collected-list heading
		"Never collected",                          // never-list heading
		DocsURL,                                    // docs link
		CodeURL,                                    // code link
		"Enable anonymous usage data? [y/N]:",      // the actual prompt line
	}
	for _, want := range mustContain {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, got)
		}
	}
}

// TestPromptCLIConsent_ConfirmationMessages pins the exact wording shown to
// the user after they answer. The "thank you" line on Yes is part of the
// trust contract; the "stays off" line on No must reassure the user their
// choice was recorded.
func TestPromptCLIConsent_ConfirmationMessages(t *testing.T) {
	t.Parallel()

	t.Run("yes_shows_thank_you", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		got, _ := PromptCLIConsent(&out, strings.NewReader("y\n"))
		if !got {
			t.Fatal("expected true for 'y'")
		}
		if !strings.Contains(out.String(), "Thank you") {
			t.Errorf("yes confirmation must include 'Thank you'; got:\n%s", out.String())
		}
		if !strings.Contains(out.String(), "telemetry off") {
			t.Errorf("yes confirmation must mention how to disable later; got:\n%s", out.String())
		}
	})

	t.Run("no_shows_stays_off", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		got, _ := PromptCLIConsent(&out, strings.NewReader("n\n"))
		if got {
			t.Fatal("expected false for 'n'")
		}
		if !strings.Contains(out.String(), "telemetry stays off") {
			t.Errorf("no confirmation must contain 'telemetry stays off'; got:\n%s", out.String())
		}
		if !strings.Contains(out.String(), "telemetry on") {
			t.Errorf("no confirmation must show how to opt in later; got:\n%s", out.String())
		}
	})
}

// TestRenderDisclosure_StableFormat pins the structure that both the prompt
// and `syllago telemetry status` rely on: paragraph appeal, two enumerated
// lists, both URLs, and the change-anytime guidance. This is the only
// canonical disclosure surface for the CLI.
func TestRenderDisclosure_StableFormat(t *testing.T) {
	t.Parallel()
	got := RenderDisclosure()

	mustContain := []string{
		"syllago needs your help",
		"What gets collected (only if you opt in):",
		"Never collected:",
		"Read the docs: " + DocsURL,
		"Read the code: " + CodeURL,
		"syllago telemetry on",
		"syllago telemetry off",
		"syllago telemetry reset",
	}
	for _, want := range mustContain {
		if !strings.Contains(got, want) {
			t.Errorf("RenderDisclosure missing %q\n--- output ---\n%s", want, got)
		}
	}

	// Every CollectedItems / NeverItems entry must appear verbatim — this
	// is what makes the disclosure stand on its own without forcing the
	// user to follow a link.
	for _, item := range CollectedItems() {
		if !strings.Contains(got, item) {
			t.Errorf("RenderDisclosure missing collected item %q", item)
		}
	}
	for _, item := range NeverItems() {
		if !strings.Contains(got, item) {
			t.Errorf("RenderDisclosure missing never-item %q", item)
		}
	}
}

// TestWrap_BehavesAsParagraph pins the small wrap helper used to render the
// MaintainerAppeal paragraph. Long words that exceed width must move to the
// next line rather than truncate; an empty input must not produce output
// that breaks the surrounding box layout.
func TestWrap_BehavesAsParagraph(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		in     string
		width  int
		indent string
		want   []string // each line, in order
	}{
		{
			name:   "fits_in_one_line",
			in:     "short text",
			width:  20,
			indent: "  ",
			want:   []string{"short text"},
		},
		{
			name: "wraps_on_word_boundary",
			// "one two" fits (7 chars). Adding "three" would push it to 13,
			// so a break is inserted; "three four" fits exactly at 10, then
			// "five" forces another break.
			in:     "one two three four five",
			width:  10,
			indent: "  ",
			want:   []string{"one two", "  three four", "  five"},
		},
		{
			name:   "empty_input",
			in:     "",
			width:  10,
			indent: "  ",
			want:   []string{""},
		},
		{
			name:   "zero_width_returns_input",
			in:     "anything",
			width:  0,
			indent: "  ",
			want:   []string{"anything"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := wrap(tt.in, tt.width, tt.indent)
			gotLines := strings.Split(got, "\n")
			if len(gotLines) != len(tt.want) {
				t.Fatalf("line count mismatch: got %d, want %d\n  got: %q\n  want: %q", len(gotLines), len(tt.want), gotLines, tt.want)
			}
			for i := range tt.want {
				if gotLines[i] != tt.want[i] {
					t.Errorf("line %d: got %q, want %q", i, gotLines[i], tt.want[i])
				}
			}
		})
	}
}

// Compile-time check that errReader implements io.Reader.
var _ io.Reader = errReader{}
