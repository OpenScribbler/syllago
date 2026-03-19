package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/parse"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// setupImportWithCustomPaths creates a test environment where:
// - HOME points to a temp dir with a global config that has provider path overrides
// - A custom directory contains rules at a non-standard location
// - The project root has a .syllago dir
// Returns (tempHome, projectRoot, customRulesDir).
func setupImportWithCustomPaths(t *testing.T) (string, string, string) {
	t.Helper()

	tempHome := t.TempDir()
	projectRoot := t.TempDir()
	customRulesDir := filepath.Join(tempHome, "custom-location", "rules")

	// Set HOME so config.LoadGlobal reads from our temp dir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	// Write global config with a per-type path override for claude-code rules
	syllagoDir := filepath.Join(tempHome, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	globalConfig := map[string]any{
		"providers": []string{},
		"provider_paths": map[string]any{
			"claude-code": map[string]any{
				"paths": map[string]string{
					"rules": customRulesDir,
				},
			},
		},
	}
	configJSON, _ := json.Marshal(globalConfig)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), configJSON, 0644)

	// Create project root with .syllago dir
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)
	os.WriteFile(filepath.Join(projectRoot, ".syllago", "config.json"), []byte(`{"providers":[]}`), 0644)

	// Place rules at the custom location
	os.MkdirAll(customRulesDir, 0755)
	os.WriteFile(filepath.Join(customRulesDir, "custom-rule.md"), []byte("# Custom Rule"), 0644)

	return tempHome, projectRoot, customRulesDir
}

func TestImportPreview_UsesConfiguredPerTypePath(t *testing.T) {
	_, projectRoot, customRulesDir := setupImportWithCustomPaths(t)

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	importCmd.Flags().Set("from", "claude-code")
	importCmd.Flags().Set("type", "rules")
	importCmd.Flags().Set("name", "")
	importCmd.Flags().Set("preview", "true")
	importCmd.Flags().Set("base-dir", "")

	if err := importCmd.RunE(importCmd, []string{}); err != nil {
		t.Fatalf("import --preview failed: %v", err)
	}

	var report parse.DiscoveryReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, stdout.String())
	}

	// Should find the rule at the custom path
	found := false
	for _, f := range report.Files {
		if strings.Contains(f.Path, "custom-rule.md") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to discover custom-rule.md from %s, got files: %v", customRulesDir, report.Files)
	}
}

func TestImportPreview_BaseDirFlagOverridesConfig(t *testing.T) {
	tempHome := t.TempDir()
	projectRoot := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	// Global config has a baseDir override (not per-type, so CLI --base-dir can win)
	syllagoDir := filepath.Join(tempHome, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	configBase := filepath.Join(tempHome, "config-base")
	globalConfig := map[string]any{
		"providers": []string{},
		"provider_paths": map[string]any{
			"claude-code": map[string]any{
				"base_dir": configBase,
			},
		},
	}
	configJSON, _ := json.Marshal(globalConfig)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), configJSON, 0644)

	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)
	os.WriteFile(filepath.Join(projectRoot, ".syllago", "config.json"), []byte(`{"providers":[]}`), 0644)

	// CLI --base-dir points to a different location with claude rules structure
	cliBase := t.TempDir()
	// Claude Code rules live at <base>/.claude/rules/ per the provider definition
	cliRulesDir := filepath.Join(cliBase, ".claude", "rules")
	os.MkdirAll(cliRulesDir, 0755)
	os.WriteFile(filepath.Join(cliRulesDir, "cli-rule.md"), []byte("# CLI Rule"), 0644)

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	importCmd.Flags().Set("from", "claude-code")
	importCmd.Flags().Set("type", "rules")
	importCmd.Flags().Set("name", "")
	importCmd.Flags().Set("preview", "true")
	importCmd.Flags().Set("base-dir", cliBase)
	t.Cleanup(func() { importCmd.Flags().Set("base-dir", "") })

	if err := importCmd.RunE(importCmd, []string{}); err != nil {
		t.Fatalf("import --preview --base-dir failed: %v", err)
	}

	var report parse.DiscoveryReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, stdout.String())
	}

	// Should find cli-rule.md from --base-dir, not config-rules
	found := false
	for _, f := range report.Files {
		if strings.Contains(f.Path, "cli-rule.md") {
			found = true
		}
		if strings.Contains(f.Path, "custom-rule") {
			t.Error("should NOT find config-rules content when --base-dir is set")
		}
	}
	if !found {
		t.Errorf("expected to discover cli-rule.md from --base-dir, got files: %v", report.Files)
	}
}

func TestSyncAndExport_ConfigPathOverrideRoutesToCustomPath(t *testing.T) {
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)

	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	// Custom install location via per-type config override
	customSkillsDir := filepath.Join(tempHome, "custom-skills-dir")

	// Use a test provider that installs skills under <base>/test-skills/
	slug := "test-export-prov"
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = append(provider.AllProviders, provider.Provider{
		Name: "TestExport",
		Slug: slug,
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			if ct == catalog.Skills {
				return filepath.Join(homeDir, "test-skills")
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Skills
		},
	})
	t.Cleanup(func() { provider.AllProviders = orig })

	// Write global config with per-type path override for the test provider
	syllagoDir := filepath.Join(tempHome, ".syllago")
	os.MkdirAll(syllagoDir, 0755)
	globalConfig := map[string]any{
		"providers": []string{},
		"provider_paths": map[string]any{
			slug: map[string]any{
				"paths": map[string]string{
					"skills": customSkillsDir,
				},
			},
		},
	}
	configJSON, _ := json.Marshal(globalConfig)
	os.WriteFile(filepath.Join(syllagoDir, "config.json"), configJSON, 0644)

	// Create .syllago/config.json in the repo
	os.MkdirAll(filepath.Join(root, ".syllago"), 0755)
	os.WriteFile(filepath.Join(root, ".syllago", "config.json"), []byte(`{"providers":[]}`), 0644)

	_, _ = output.SetForTest(t)

	syncAndExportCmd.Flags().Set("to", slug)
	syncAndExportCmd.Flags().Set("type", "skills")
	syncAndExportCmd.Flags().Set("name", "greeting")
	syncAndExportCmd.Flags().Set("source", "shared")
	t.Cleanup(func() {
		syncAndExportCmd.Flags().Set("to", "")
		syncAndExportCmd.Flags().Set("type", "")
		syncAndExportCmd.Flags().Set("name", "")
		syncAndExportCmd.Flags().Set("source", "local")
	})

	err := syncAndExportCmd.RunE(syncAndExportCmd, []string{})
	if err != nil {
		t.Fatalf("sync-and-export with config path override failed: %v", err)
	}

	// Verify the skill was installed under the custom dir, not the default
	installed := filepath.Join(customSkillsDir, "greeting")
	if _, err := os.Stat(installed); err != nil {
		t.Errorf("expected skill installed at %s, got error: %v", installed, err)
	}

	// Verify nothing was installed under default path (tempHome/test-skills)
	defaultInstall := filepath.Join(tempHome, "test-skills")
	if _, err := os.Stat(defaultInstall); err == nil {
		t.Error("should NOT install under default path when config override is set")
	}
}
