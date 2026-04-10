package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestSyllagoDetector_ProviderSlug(t *testing.T) {
	t.Parallel()
	d := &SyllagoDetector{}
	if slug := d.ProviderSlug(); slug != "syllago" {
		t.Errorf("ProviderSlug() = %q, want %q", slug, "syllago")
	}
}

func TestSyllagoDetector_Patterns(t *testing.T) {
	t.Parallel()
	d := &SyllagoDetector{}
	pats := d.Patterns()

	if len(pats) != 7 {
		t.Fatalf("Patterns() returned %d patterns, want 7", len(pats))
	}
	for i, p := range pats {
		if p.Confidence != 0.95 {
			t.Errorf("Patterns()[%d].Confidence = %v, want 0.95", i, p.Confidence)
		}
	}
}

func TestSyllagoDetector_Classify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		setup    func(t *testing.T, root string)
		wantNil  bool
		wantName string
		wantType catalog.ContentType
	}{
		{
			name: "skill with frontmatter",
			path: "skills/my-skill/SKILL.md",
			setup: func(t *testing.T, root string) {
				t.Helper()
				dir := filepath.Join(root, "skills", "my-skill")
				os.MkdirAll(dir, 0755)
				os.WriteFile(filepath.Join(dir, "SKILL.md"),
					[]byte("---\nname: My Skill\ndescription: Does things\n---\nBody.\n"), 0644)
			},
			wantName: "my-skill",
			wantType: catalog.Skills,
		},
		{
			name: "hook in provider subdirectory",
			path: "hooks/claude-code/lint/hook.json",
			setup: func(t *testing.T, root string) {
				t.Helper()
				dir := filepath.Join(root, "hooks", "claude-code", "lint")
				os.MkdirAll(dir, 0755)
				os.WriteFile(filepath.Join(dir, "hook.json"),
					[]byte(`{"event": "PostToolUse"}`), 0644)
			},
			wantName: "lint",
			wantType: catalog.Hooks,
		},
		{
			name:    "missing path returns nil",
			path:    "skills/ghost/SKILL.md",
			setup:   func(t *testing.T, root string) {},
			wantNil: true,
		},
		{
			name: "file exceeding 1MB returns nil",
			path: "skills/big-skill/SKILL.md",
			setup: func(t *testing.T, root string) {
				t.Helper()
				dir := filepath.Join(root, "skills", "big-skill")
				os.MkdirAll(dir, 0755)
				// Create a file larger than 1MB.
				bigData := make([]byte, 1024*1024+1)
				os.WriteFile(filepath.Join(dir, "SKILL.md"), bigData, 0644)
			},
			wantNil: true,
		},
	}

	d := &SyllagoDetector{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			tt.setup(t, root)

			items, err := d.Classify(tt.path, root)
			if err != nil {
				t.Fatalf("Classify returned error: %v", err)
			}

			if tt.wantNil {
				if items != nil {
					t.Errorf("expected nil items, got %d", len(items))
				}
				return
			}

			if len(items) != 1 {
				t.Fatalf("expected 1 item, got %d", len(items))
			}

			item := items[0]
			if item.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", item.Name, tt.wantName)
			}
			if item.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", item.Type, tt.wantType)
			}
			if item.Confidence != 0.95 {
				t.Errorf("Confidence = %v, want 0.95", item.Confidence)
			}
			if item.Provider != "syllago" {
				t.Errorf("Provider = %q, want %q", item.Provider, "syllago")
			}
		})
	}
}

func TestSyllagoDetector_ContentHash(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := filepath.Join(root, "skills", "hash-test")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"),
		[]byte("---\nname: Hash Test\n---\nContent.\n"), 0644)

	d := &SyllagoDetector{}
	items, err := d.Classify("skills/hash-test/SKILL.md", root)
	if err != nil {
		t.Fatalf("Classify error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	hash := items[0].ContentHash
	if len(hash) != 64 {
		t.Errorf("ContentHash length = %d, want 64 (SHA-256 hex)", len(hash))
	}
	if strings.Trim(hash, "0123456789abcdef") != "" {
		t.Errorf("ContentHash contains non-hex characters: %q", hash)
	}
}
