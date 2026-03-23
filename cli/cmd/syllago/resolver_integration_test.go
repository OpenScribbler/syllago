package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestInstall_ConfigPerTypePathOverride(t *testing.T) {
	globalDir := setupGlobalLibrary(t) // has skills/my-skill
	withGlobalLibrary(t, globalDir)

	tempHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	// Custom install location via per-type config override
	customSkillsDir := filepath.Join(tempHome, "custom-skills-dir")

	slug := "test-install-prov"
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = append(provider.AllProviders, provider.Provider{
		Name: "TestInstall",
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

	// Write global config with per-type path override
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

	// Project config (install_cmd loads both)
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)
	os.WriteFile(filepath.Join(projectRoot, ".syllago", "config.json"), []byte(`{"providers":[]}`), 0644)

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	stdout, _ := output.SetForTest(t)
	output.JSON = true

	installCmd.Flags().Set("to", slug)
	installCmd.Flags().Set("type", "skills")
	installCmd.Flags().Set("base-dir", "")
	t.Cleanup(func() {
		installCmd.Flags().Set("to", "")
		installCmd.Flags().Set("type", "")
		installCmd.Flags().Set("base-dir", "")
	})

	err := installCmd.RunE(installCmd, []string{})
	if err != nil {
		t.Fatalf("install with config per-type path failed: %v", err)
	}

	// Parse JSON output to verify install path
	var result installResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, stdout.String())
	}

	if len(result.Installed) == 0 {
		t.Fatal("expected at least one installed item")
	}

	// The installed path should be under the custom dir, not default
	installedPath := result.Installed[0].Path
	if !strings.HasPrefix(installedPath, customSkillsDir) {
		t.Errorf("expected installed path under %s, got %s", customSkillsDir, installedPath)
	}

	// Verify nothing at default path
	defaultInstall := filepath.Join(tempHome, "test-skills")
	if _, err := os.Stat(defaultInstall); err == nil {
		t.Error("should NOT install under default path when config per-type override is set")
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
