package readme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate_WithDescription(t *testing.T) {
	t.Parallel()
	got := Generate("my-cool-skill", "skills", "A skill that does things")

	if !strings.Contains(got, "# My Cool Skill") {
		t.Errorf("expected title, got:\n%s", got)
	}
	if !strings.Contains(got, "A skill that does things") {
		t.Errorf("expected description, got:\n%s", got)
	}
	if !strings.Contains(got, "**Type:** skills") {
		t.Errorf("expected type line, got:\n%s", got)
	}
}

func TestGenerate_WithoutDescription(t *testing.T) {
	t.Parallel()
	got := Generate("test-item", "rules", "")

	if !strings.Contains(got, "# Test Item") {
		t.Errorf("expected title, got:\n%s", got)
	}
	if !strings.Contains(got, "**Type:** rules") {
		t.Errorf("expected type line, got:\n%s", got)
	}
	// Should not have an empty paragraph between title and type
	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if i > 0 && i < len(lines)-1 && line == "" {
			next := lines[i+1]
			if next == "" {
				t.Error("should not have double blank lines without description")
			}
		}
	}
}

func TestEnsureReadme_Creates(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	itemDir := filepath.Join(tmp, "my-item")
	os.MkdirAll(itemDir, 0755)

	created, err := EnsureReadme(itemDir, "my-item", "skills", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected created=true for missing README")
	}

	data, err := os.ReadFile(filepath.Join(itemDir, "README.md"))
	if err != nil {
		t.Fatalf("README.md should exist: %v", err)
	}
	if !strings.Contains(string(data), "# My Item") {
		t.Errorf("expected title in README, got:\n%s", string(data))
	}
}

func TestEnsureReadme_SkipsExisting(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	itemDir := filepath.Join(tmp, "my-item")
	os.MkdirAll(itemDir, 0755)

	// Write a pre-existing README
	existing := "# Existing README\n\nDo not overwrite me.\n"
	os.WriteFile(filepath.Join(itemDir, "README.md"), []byte(existing), 0644)

	created, err := EnsureReadme(itemDir, "my-item", "skills", "new description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created {
		t.Error("expected created=false for existing README")
	}

	// Verify content was NOT overwritten
	data, err := os.ReadFile(filepath.Join(itemDir, "README.md"))
	if err != nil {
		t.Fatalf("README.md should still exist: %v", err)
	}
	if string(data) != existing {
		t.Errorf("existing README was overwritten, got:\n%s", string(data))
	}
}
