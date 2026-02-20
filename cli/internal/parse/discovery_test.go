package parse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/provider"
)

func TestDiscoverFindsFiles(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestDiscoverDirectoryStructuredContent(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Create a skills directory with subdirectory-based content,
	// mimicking .claude/skills/my-skill/SKILL.md
	skillDir := filepath.Join(tmp, ".claude", "skills", "my-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# My Skill"), 0644)
	os.WriteFile(filepath.Join(skillDir, "README.md"), []byte("# Readme"), 0644)

	// Also create a flat file alongside the subdirectory
	os.WriteFile(filepath.Join(tmp, ".claude", "skills", "flat.md"), []byte("flat"), 0644)

	prov := provider.Provider{
		Slug: "claude-code",
		DiscoveryPaths: func(root string, ct catalog.ContentType) []string {
			if ct == catalog.Skills {
				return []string{filepath.Join(root, ".claude", "skills")}
			}
			return nil
		},
	}

	report := Discover(prov, tmp)
	if report.Counts[catalog.Skills] != 3 {
		t.Errorf("expected 3 skills files, got %d", report.Counts[catalog.Skills])
		for _, f := range report.Files {
			t.Logf("  found: %s", f.Path)
		}
	}
}

func TestDiscoverTracksUnsupportedTypes(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Cursor only supports Rules — everything else should be unsupported.
	prov := provider.Provider{
		Slug: "cursor",
		DiscoveryPaths: func(root string, ct catalog.ContentType) []string {
			if ct == catalog.Rules {
				return []string{filepath.Join(root, ".cursor", "rules")}
			}
			return nil
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Rules
		},
	}

	report := Discover(prov, tmp)

	// Rules is supported, so it should NOT appear in Unsupported.
	for _, u := range report.Unsupported {
		if u == catalog.Rules {
			t.Error("Rules should not be in Unsupported list")
		}
	}

	// Skills, Agents, Prompts, MCP, Apps, Hooks, Commands are not supported.
	// AllContentTypes returns 8 types; only Rules is supported, so 7 unsupported.
	if len(report.Unsupported) != 7 {
		t.Errorf("expected 7 unsupported types, got %d: %v", len(report.Unsupported), report.Unsupported)
	}
}

func TestDiscoverRecordsSearchedPaths(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Provider supports Rules and Skills, but the directories don't exist.
	prov := provider.Provider{
		Slug: "test-prov",
		DiscoveryPaths: func(root string, ct catalog.ContentType) []string {
			switch ct {
			case catalog.Rules:
				return []string{
					filepath.Join(root, "RULES.md"),
					filepath.Join(root, ".test", "rules"),
				}
			case catalog.Skills:
				return []string{filepath.Join(root, ".test", "skills")}
			}
			return nil
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Rules || ct == catalog.Skills
		},
	}

	report := Discover(prov, tmp)

	// SearchedPaths should record the paths for supported types that had paths.
	rulesPaths, ok := report.SearchedPaths[catalog.Rules]
	if !ok {
		t.Fatal("expected SearchedPaths to contain Rules")
	}
	if len(rulesPaths) != 2 {
		t.Errorf("expected 2 searched paths for Rules, got %d", len(rulesPaths))
	}

	skillsPaths, ok := report.SearchedPaths[catalog.Skills]
	if !ok {
		t.Fatal("expected SearchedPaths to contain Skills")
	}
	if len(skillsPaths) != 1 {
		t.Errorf("expected 1 searched path for Skills, got %d", len(skillsPaths))
	}

	// Unsupported types should NOT have searched paths.
	if _, ok := report.SearchedPaths[catalog.MCP]; ok {
		t.Error("MCP is unsupported and should not have searched paths")
	}
}

func TestClassifyByExtension(t *testing.T) {
	t.Parallel()
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
