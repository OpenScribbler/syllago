package moat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalText(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{
			name: "strips bom",
			in:   append(append([]byte{}, testUTF8BOM...), []byte("hello\n")...),
			want: "hello\n",
		},
		{
			name: "normalizes crlf and cr mix",
			in:   []byte("a\r\nb\rc\n"),
			want: "a\nb\nc\n",
		},
		{
			name: "lone cr at eof",
			in:   []byte("a\r"),
			want: "a\n",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := string(CanonicalText(tc.in)); got != tc.want {
				t.Fatalf("CanonicalText() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFileHashClassifiesTextAndBinary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	raw := append(append([]byte{}, testUTF8BOM...), []byte("a\r\nb\r")...)

	textPath := filepath.Join(dir, "note.md")
	if err := os.WriteFile(textPath, raw, 0o644); err != nil {
		t.Fatalf("write text: %v", err)
	}
	textHash, err := FileHash(textPath)
	if err != nil {
		t.Fatalf("FileHash(text): %v", err)
	}
	if want := sha256HexOf([]byte("a\nb\n")); textHash != want {
		t.Fatalf("FileHash(text) = %s, want %s", textHash, want)
	}

	binaryPath := filepath.Join(dir, "note.bin")
	if err := os.WriteFile(binaryPath, raw, 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	binaryHash, err := FileHash(binaryPath)
	if err != nil {
		t.Fatalf("FileHash(binary): %v", err)
	}
	if want := sha256HexOf(raw); binaryHash != want {
		t.Fatalf("FileHash(binary) = %s, want %s", binaryHash, want)
	}

	nulJSON := filepath.Join(dir, "data.json")
	nulRaw := []byte("{\"k\":\"v\x00\"}\r\n")
	if err := os.WriteFile(nulJSON, nulRaw, 0o644); err != nil {
		t.Fatalf("write nul json: %v", err)
	}
	nulHash, err := FileHash(nulJSON)
	if err != nil {
		t.Fatalf("FileHash(nul json): %v", err)
	}
	if want := sha256HexOf(nulRaw); nulHash != want {
		t.Fatalf("FileHash(nul json) = %s, want raw binary hash %s", nulHash, want)
	}
}
