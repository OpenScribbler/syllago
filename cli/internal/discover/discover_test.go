package discover

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverMonolithicRules_Empty(t *testing.T) {
	tmp := t.TempDir()
	got, err := DiscoverMonolithicRules(tmp, "", []string{"CLAUDE.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 candidates, got %d: %+v", len(got), got)
	}
}

func TestDiscoverMonolithicRules_NestedDirs(t *testing.T) {
	tmp := t.TempDir()
	// Create nested structure.
	mustMkdir := func(p string) {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite := func(p string) {
		if err := os.WriteFile(p, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustMkdir(filepath.Join(tmp, "apps", "web"))
	mustMkdir(filepath.Join(tmp, "apps", "api"))
	mustWrite(filepath.Join(tmp, "CLAUDE.md"))
	mustWrite(filepath.Join(tmp, "apps", "web", "CLAUDE.md"))
	mustWrite(filepath.Join(tmp, "apps", "api", "AGENTS.md"))

	got, err := DiscoverMonolithicRules(tmp, "", []string{"CLAUDE.md", "AGENTS.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 candidates, got %d: %+v", len(got), got)
	}
	for _, c := range got {
		if c.Scope != "project" {
			t.Errorf("candidate %s: scope = %q, want project", c.AbsPath, c.Scope)
		}
		if filepath.Base(c.AbsPath) != c.Filename {
			t.Errorf("candidate %s: filename %q does not match path", c.AbsPath, c.Filename)
		}
	}
}
