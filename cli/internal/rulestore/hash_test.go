package rulestore

import (
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
