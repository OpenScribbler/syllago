package rulestore

import (
	"regexp"
	"strings"
	"testing"
)

func TestHashToFilename(t *testing.T) {
	t.Parallel()
	hex := strings.Repeat("a", 64)
	got := hashToFilename("sha256:" + hex)
	want := "sha256-" + hex + ".md"
	if got != want {
		t.Errorf("hashToFilename: got %q, want %q", got, want)
	}
}

func TestFilenameToHash_Valid(t *testing.T) {
	t.Parallel()
	hex := strings.Repeat("b", 64)
	name := "sha256-" + hex + ".md"
	got, err := filenameToHash(name)
	if err != nil {
		t.Fatalf("filenameToHash: unexpected error: %v", err)
	}
	want := "sha256:" + hex
	if got != want {
		t.Errorf("filenameToHash: got %q, want %q", got, want)
	}
	// Roundtrip: hashToFilename(filenameToHash(name)) == name.
	if round := hashToFilename(got); round != name {
		t.Errorf("roundtrip: got %q, want %q", round, name)
	}
}

func TestHashBody(t *testing.T) {
	t.Parallel()
	canonicalHashRe := regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	got := HashBody([]byte("hello\n"))
	if !canonicalHashRe.MatchString(got) {
		t.Errorf("HashBody: %q does not match sha256:<64-hex>", got)
	}
	// Determinism.
	if other := HashBody([]byte("hello\n")); other != got {
		t.Errorf("HashBody: not deterministic: %q vs %q", got, other)
	}
	// Different inputs produce different hashes.
	if diff := HashBody([]byte("world\n")); diff == got {
		t.Errorf("HashBody: distinct inputs produced same hash %q", got)
	}
	// Normalization: CRLF input must hash to the same thing as LF input (D12).
	lf := HashBody([]byte("a\nb\n"))
	crlf := HashBody([]byte("a\r\nb\r\n"))
	if lf != crlf {
		t.Errorf("HashBody: CRLF vs LF should normalize equal, got %q vs %q", lf, crlf)
	}
}

func TestFilenameToHash_Malformed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
	}{
		{"missing extension", "sha256-" + strings.Repeat("a", 64)},
		{"missing dash", "sha256" + strings.Repeat("a", 64) + ".md"},
		{"wrong hex length", "sha256-" + strings.Repeat("a", 63) + ".md"},
		{"wrong algo", "md5-" + strings.Repeat("a", 64) + ".md"},
		{"uppercase hex", "sha256-" + strings.Repeat("A", 64) + ".md"},
		{"non-hex chars", "sha256-" + strings.Repeat("g", 64) + ".md"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := filenameToHash(tc.input)
			if err == nil {
				t.Fatalf("expected error for %q, got nil", tc.input)
			}
			if !strings.Contains(err.Error(), "malformed history filename") {
				t.Errorf("error %q does not contain %q", err.Error(), "malformed history filename")
			}
		})
	}
}
