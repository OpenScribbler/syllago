package parse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// stubProviderForDiscovery returns a Claude Code-like provider for testing discovery.
func stubProviderForDiscovery(slug string) provider.Provider {
	return provider.Provider{
		Name: "Test Provider",
		Slug: slug,
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			switch ct {
			case catalog.Rules:
				return filepath.Join(homeDir, ".provider", "rules")
			case catalog.Skills:
				return filepath.Join(homeDir, ".provider", "skills")
			case catalog.Hooks:
				return "__json_merge__"
			}
			return ""
		},
		DiscoveryPaths: func(projectRoot string, ct catalog.ContentType) []string {
			switch ct {
			case catalog.Rules:
				return []string{filepath.Join(projectRoot, ".provider", "rules")}
			case catalog.Skills:
				return []string{filepath.Join(projectRoot, ".provider", "skills")}
			}
			return nil
		},
		SupportsType: func(ct catalog.ContentType) bool {
			switch ct {
			case catalog.Rules, catalog.Skills, catalog.Hooks:
				return true
			}
			return false
		},
	}
}

// writeFile is a test helper that creates a file with parent dirs.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func TestDiscoverWithResolver_PerTypePath(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	prov := stubProviderForDiscovery("test-prov")

	// Place a skill at a custom (non-standard) location
	customSkillsDir := filepath.Join(tmp, "custom", "my-skills")
	writeFile(t, filepath.Join(customSkillsDir, "research", "SKILL.md"), "# Research Skill")

	// Place a rule at the standard location (should still be found)
	standardRulesDir := filepath.Join(tmp, ".provider", "rules")
	writeFile(t, filepath.Join(standardRulesDir, "security.md"), "# Security Rule")

	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"test-prov": {
				Paths: map[string]string{
					"skills": customSkillsDir,
				},
			},
		},
	}
	resolver := config.NewResolver(cfg, "")

	report := DiscoverWithResolver(prov, tmp, resolver)

	// Skills should come from the custom path
	skillCount := report.Counts[catalog.Skills]
	if skillCount != 1 {
		t.Errorf("expected 1 skill from custom path, got %d", skillCount)
	}

	// Rules should come from the standard path (no per-type override)
	ruleCount := report.Counts[catalog.Rules]
	if ruleCount != 1 {
		t.Errorf("expected 1 rule from standard path, got %d", ruleCount)
	}

	// Verify searched paths reflect the override
	skillPaths := report.SearchedPaths[catalog.Skills]
	if len(skillPaths) != 1 || skillPaths[0] != customSkillsDir {
		t.Errorf("expected skill search path %q, got %v", customSkillsDir, skillPaths)
	}
}

func TestDiscoverWithResolver_BaseDir(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	prov := stubProviderForDiscovery("test-prov")

	// Place content at the custom base (mirrors provider structure)
	customBase := filepath.Join(tmp, "custom-base")
	writeFile(t, filepath.Join(customBase, ".provider", "rules", "my-rule.md"), "# My Rule")
	writeFile(t, filepath.Join(customBase, ".provider", "skills", "my-skill", "SKILL.md"), "# My Skill")

	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"test-prov": {BaseDir: customBase},
		},
	}
	resolver := config.NewResolver(cfg, "")

	report := DiscoverWithResolver(prov, tmp, resolver)

	if report.Counts[catalog.Rules] != 1 {
		t.Errorf("expected 1 rule from custom base, got %d", report.Counts[catalog.Rules])
	}
	if report.Counts[catalog.Skills] != 1 {
		t.Errorf("expected 1 skill from custom base, got %d", report.Counts[catalog.Skills])
	}
}

func TestDiscoverWithResolver_CLIBaseDirOverridesConfig(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	prov := stubProviderForDiscovery("test-prov")

	// Config base has content
	configBase := filepath.Join(tmp, "config-base")
	writeFile(t, filepath.Join(configBase, ".provider", "rules", "config-rule.md"), "# Config Rule")

	// CLI base has different content
	cliBase := filepath.Join(tmp, "cli-base")
	writeFile(t, filepath.Join(cliBase, ".provider", "rules", "cli-rule.md"), "# CLI Rule")

	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"test-prov": {BaseDir: configBase},
		},
	}
	resolver := config.NewResolver(cfg, cliBase)

	report := DiscoverWithResolver(prov, tmp, resolver)

	// Should find the CLI base content, not the config base
	if report.Counts[catalog.Rules] != 1 {
		t.Errorf("expected 1 rule from CLI base, got %d", report.Counts[catalog.Rules])
	}
	// Verify the searched path is under the CLI base
	rulePaths := report.SearchedPaths[catalog.Rules]
	if len(rulePaths) != 1 || rulePaths[0] != filepath.Join(cliBase, ".provider", "rules") {
		t.Errorf("expected rule search path under CLI base, got %v", rulePaths)
	}
}

func TestDiscoverWithResolver_NilResolverUsesDefaults(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	prov := stubProviderForDiscovery("test-prov")

	writeFile(t, filepath.Join(tmp, ".provider", "rules", "default-rule.md"), "# Default Rule")

	report := DiscoverWithResolver(prov, tmp, nil)

	if report.Counts[catalog.Rules] != 1 {
		t.Errorf("expected 1 rule from default path, got %d", report.Counts[catalog.Rules])
	}
}

func TestDiscoverWithResolver_PerTypeOverridesBaseDir(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	prov := stubProviderForDiscovery("test-prov")

	// BaseDir has rules and skills at standard subpaths
	baseDir := filepath.Join(tmp, "base")
	writeFile(t, filepath.Join(baseDir, ".provider", "rules", "base-rule.md"), "# Base Rule")
	writeFile(t, filepath.Join(baseDir, ".provider", "skills", "base-skill", "SKILL.md"), "# Base Skill")

	// Per-type override for skills points elsewhere
	customSkills := filepath.Join(tmp, "custom-skills")
	writeFile(t, filepath.Join(customSkills, "custom-skill", "SKILL.md"), "# Custom Skill")

	cfg := &config.Config{
		ProviderPaths: map[string]config.ProviderPathConfig{
			"test-prov": {
				BaseDir: baseDir,
				Paths:   map[string]string{"skills": customSkills},
			},
		},
	}
	resolver := config.NewResolver(cfg, "")

	report := DiscoverWithResolver(prov, tmp, resolver)

	// Rules should come from baseDir (no per-type override)
	if report.Counts[catalog.Rules] != 1 {
		t.Errorf("expected 1 rule from baseDir, got %d", report.Counts[catalog.Rules])
	}
	// Skills should come from the per-type path, not baseDir
	if report.Counts[catalog.Skills] != 1 {
		t.Errorf("expected 1 skill from per-type path, got %d", report.Counts[catalog.Skills])
	}
	skillPaths := report.SearchedPaths[catalog.Skills]
	if len(skillPaths) != 1 || skillPaths[0] != customSkills {
		t.Errorf("expected skill path %q, got %v", customSkills, skillPaths)
	}
}
