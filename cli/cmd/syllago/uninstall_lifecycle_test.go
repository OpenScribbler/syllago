package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func setUninstallFlags(t *testing.T, from, typ string) {
	t.Helper()
	uninstallCmd.Flags().Set("from", from)
	uninstallCmd.Flags().Set("force", "true")
	uninstallCmd.Flags().Set("type", typ)
	t.Cleanup(func() {
		uninstallCmd.Flags().Set("from", "")
		uninstallCmd.Flags().Set("force", "false")
		uninstallCmd.Flags().Set("type", "")
	})
}

func withProjectRoot(t *testing.T, root string) {
	t.Helper()
	orig := findProjectRoot
	findProjectRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findProjectRoot = orig })
}

func TestUninstall_WaitsForInstallLock(t *testing.T) {
	setupInstalledSkill(t)
	withProjectRoot(t, t.TempDir())
	output.SetForTest(t)
	holdInstallLock(t)
	setUninstallFlags(t, "test-prov", "")

	requireLockedError(t, uninstallCmd.RunE(uninstallCmd, []string{"my-skill"}))
}

// An item installed under a configured per-type path is found and removed
// there, the same place install put it.
func TestUninstall_RemovesFromConfiguredPath(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	withProjectRoot(t, t.TempDir())
	addTestProvider(t, "test-prov", "Test Provider", t.TempDir())
	output.SetForTest(t)

	configDir := t.TempDir()
	origConfig := config.GlobalDirOverride
	config.GlobalDirOverride = configDir
	t.Cleanup(func() { config.GlobalDirOverride = origConfig })
	customDir := filepath.Join(t.TempDir(), "custom-skills")
	cfg := &config.Config{ProviderPaths: map[string]config.ProviderPathConfig{
		"test-prov": {Paths: map[string]string{string(catalog.Skills): customDir}},
	}}
	if err := config.SaveGlobal(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatal(err)
	}
	placed := filepath.Join(customDir, "my-skill")
	if err := os.Symlink(filepath.Join(globalDir, "skills", "my-skill"), placed); err != nil {
		t.Fatal(err)
	}
	setUninstallFlags(t, "test-prov", "")

	if err := uninstallCmd.RunE(uninstallCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Lstat(placed); !os.IsNotExist(err) {
		t.Errorf("%s still exists", placed)
	}
}

func TestUninstall_MonolithicRuleWaitsForInstallLock(t *testing.T) {
	projectRoot := t.TempDir()
	globalDir := t.TempDir()
	seedLibraryRule(t, globalDir, "claude-code", "foo", "# foo\n\nAppend me.\n")
	withProjectRoot(t, projectRoot)
	withGlobalLibrary(t, globalDir)
	origConfig := config.GlobalDirOverride
	config.GlobalDirOverride = t.TempDir()
	t.Cleanup(func() { config.GlobalDirOverride = origConfig })
	output.SetForTest(t)

	installCmd.Flags().Set("to", "claude-code")
	installCmd.Flags().Set("method", "append")
	installCmd.Flags().Set("type", "rules")
	t.Cleanup(func() {
		installCmd.Flags().Set("to", "")
		installCmd.Flags().Set("method", "symlink")
		installCmd.Flags().Set("type", "")
	})
	if err := installCmd.RunE(installCmd, []string{"foo"}); err != nil {
		t.Fatalf("install append: %v", err)
	}
	holdInstallLock(t)
	setUninstallFlags(t, "claude-code", "rules")

	requireLockedError(t, uninstallCmd.RunE(uninstallCmd, []string{"foo"}))
}
