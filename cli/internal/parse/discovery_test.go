package parse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/provider"
)

func TestDiscoverFindsFiles(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".claude", "rules"), 0755)
	os.WriteFile(filepath.Join(tmp, "CLAUDE.md"), []byte("# Rules"), 0644)
	os.WriteFile(filepath.Join(tmp, ".claude", "rules", "test.md"), []byte("test rule"), 0644)

	prov := provider.Provider{
		Slug: "claude-code",
		DiscoveryPaths: func(root string, ct catalog.ContentType) []string {
			switch ct {
			case catalog.Rules:
				return []string{
					filepath.Join(root, "CLAUDE.md"),
					filepath.Join(root, ".claude", "rules"),
				}
			}
			return nil
		},
	}

	report := Discover(prov, tmp)
	if report.Counts[catalog.Rules] != 2 {
		t.Errorf("expected 2 rules files, got %d", report.Counts[catalog.Rules])
	}
}

func TestDiscoverEmptyProject(t *testing.T) {
	tmp := t.TempDir()
	prov := provider.Provider{
		Slug: "cursor",
		DiscoveryPaths: func(root string, ct catalog.ContentType) []string {
			return []string{filepath.Join(root, ".cursor", "rules")}
		},
	}

	report := Discover(prov, tmp)
	if len(report.Files) != 0 {
		t.Errorf("expected 0 files in empty project, got %d", len(report.Files))
	}
}

func TestClassifyByExtension(t *testing.T) {
	tests := []struct {
		path string
		want catalog.ContentType
		ok   bool
	}{
		{"CLAUDE.md", catalog.Rules, true},
		{"/proj/.cursor/rules/test.mdc", catalog.Rules, true},
		{"/proj/.claude/skills/test/SKILL.md", catalog.Skills, true},
		{"/proj/random.txt", "", false},
	}
	for _, tt := range tests {
		got, ok := ClassifyByExtension(tt.path)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ClassifyByExtension(%q) = (%q, %v), want (%q, %v)", tt.path, got, ok, tt.want, tt.ok)
		}
	}
}
