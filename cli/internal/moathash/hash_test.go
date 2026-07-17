package moathash

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFinalExtension(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		want string
	}{
		{"SKILL.md", ".md"},
		{"foo.tar.gz", ".gz"},
		{"README", ""},
		{"Makefile", ""},
		{".gitignore", ""},
		{".env.example", ".example"},
		{"a-b", ""},
		{"a.b", ".b"},
		{"UPPER.MD", ".md"},
		{"", ""},
	}
	for _, c := range cases {
		if got := finalExtension(c.name); got != c.want {
			t.Errorf("finalExtension(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestIsText(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mustWrite := func(name string, data []byte) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
		return p
	}

	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"plain.md", []byte("# hello\n"), true},
		{"config.yaml", []byte("x: 1\n"), true},
		{"script.sh", []byte("echo hi\n"), true},
		{"readme", []byte("no extension\n"), false},
		{".gitignore", []byte("*.log\n"), false},
		{"data.bin", []byte("binary"), false},
		{"icon.png", []byte("\x89PNG\r\n\x1a\n"), false},
		{"nul.json", []byte("{\"k\":\"v\x00\"}\n"), false},
	}
	for _, c := range cases {
		p := mustWrite(c.name, c.data)
		got, err := isText(p)
		if err != nil {
			t.Fatalf("isText(%s): %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("isText(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}
