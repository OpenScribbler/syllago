package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func TestUninstallUnknownProvider(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	uninstallCmd.Flags().Set("from", "nonexistent-provider")
	defer uninstallCmd.Flags().Set("from", "")

	err := uninstallCmd.RunE(uninstallCmd, []string{"my-skill"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "nonexistent-provider") {
		t.Errorf("expected provider name in error, got: %v", err)
	}
}

func TestUninstallItemNotFound(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	uninstallCmd.Flags().Set("from", "claude-code")
	defer uninstallCmd.Flags().Set("from", "")

	err := uninstallCmd.RunE(uninstallCmd, []string{"does-not-exist"})
	if err == nil {
		t.Fatal("expected error when item not found")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("expected item name in error, got: %v", err)
	}
}

func TestUninstallNotInstalledAnywhere(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	// No --from flag = check all providers. The skill isn't installed anywhere.
	err := uninstallCmd.RunE(uninstallCmd, []string{"my-skill"})
	if err == nil {
		t.Fatal("expected error when item not installed anywhere")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected 'not installed' in error, got: %v", err)
	}
}

func TestUninstallFlagsRegistered(t *testing.T) {
	flags := []string{"from", "force", "dry-run", "no-input", "type"}
	for _, name := range flags {
		if uninstallCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected --%s flag to be registered on uninstallCmd", name)
		}
	}
}

func TestUninstallNoArgs(t *testing.T) {
	// cobra.ExactArgs(1) validates before RunE is called.
	_, _ = output.SetForTest(t)

	err := uninstallCmd.Args(uninstallCmd, []string{})
	if err == nil {
		t.Fatal("expected error for no args")
	}
}

func TestUninstallItemNotFoundErrorCode(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	err := uninstallCmd.RunE(uninstallCmd, []string{"does-not-exist"})
	if err == nil {
		t.Fatal("expected error when item not found")
	}
	// StructuredError.Error() returns "[INSTALL_002] message" format
	if !strings.Contains(err.Error(), "INSTALL_002") {
		t.Errorf("expected INSTALL_002 error code, got: %v", err)
	}
}

func TestUninstallUnknownProviderErrorCode(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	uninstallCmd.Flags().Set("from", "fake-provider")
	defer uninstallCmd.Flags().Set("from", "")

	err := uninstallCmd.RunE(uninstallCmd, []string{"my-skill"})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "PROVIDER_001") {
		t.Errorf("expected PROVIDER_001 error code, got: %v", err)
	}
}

func TestUninstallWithTypeFilter(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	// my-skill exists but not as type "hooks"
	uninstallCmd.Flags().Set("type", "hooks")
	defer uninstallCmd.Flags().Set("type", "")

	err := uninstallCmd.RunE(uninstallCmd, []string{"my-skill"})
	if err == nil {
		t.Fatal("expected error when item not found with type filter")
	}
	if !strings.Contains(err.Error(), "INSTALL_002") {
		t.Errorf("expected INSTALL_002 error code, got: %v", err)
	}
}

func TestUninstallMultipleArgs(t *testing.T) {
	// cobra.ExactArgs(1) rejects more than one arg
	_, _ = output.SetForTest(t)

	err := uninstallCmd.Args(uninstallCmd, []string{"one", "two"})
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestUninstallNotInstalledInProvider(t *testing.T) {
	// Item exists in library but is not installed in the specified real provider.
	// This covers the CheckStatus != StatusInstalled path (lines 108-114).
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	// "claude-code" is a real provider slug, but my-skill is not installed there.
	uninstallCmd.Flags().Set("from", "claude-code")
	defer uninstallCmd.Flags().Set("from", "")

	err := uninstallCmd.RunE(uninstallCmd, []string{"my-skill"})
	if err == nil {
		t.Fatal("expected error when item not installed in provider")
	}
	if !strings.Contains(err.Error(), "INSTALL_005") {
		t.Errorf("expected INSTALL_005 error code, got: %v", err)
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Errorf("expected 'not installed' in error, got: %v", err)
	}
}

func TestUninstallNotInstalledAnywhereErrorCode(t *testing.T) {
	// No --from flag, item exists but not installed in any provider.
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	_, _ = output.SetForTest(t)

	err := uninstallCmd.RunE(uninstallCmd, []string{"my-skill"})
	if err == nil {
		t.Fatal("expected error when item not installed anywhere")
	}
	if !strings.Contains(err.Error(), "INSTALL_005") {
		t.Errorf("expected INSTALL_005 error code, got: %v", err)
	}
}

// setupInstalledSkill creates a global library with a skill and a test provider
// that has the skill "installed" via symlink. Returns the global dir.
func setupInstalledSkill(t *testing.T) string {
	t.Helper()
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)

	// Create a provider install location and symlink the skill there
	installBase := t.TempDir()
	skillInstallDir := filepath.Join(installBase, "skills")
	os.MkdirAll(skillInstallDir, 0755)

	// Create symlink: installBase/skills/my-skill -> globalDir/skills/my-skill
	srcSkill := filepath.Join(globalDir, "skills", "my-skill")
	dstLink := filepath.Join(skillInstallDir, "my-skill")
	os.Symlink(srcSkill, dstLink)

	// Register a test provider whose install dir points to installBase
	addTestProvider(t, "test-prov", "Test Provider", installBase)

	return globalDir
}

func TestUninstallDryRun(t *testing.T) {
	// Dry-run should show what would happen without making changes.
	setupInstalledSkill(t)
	stdout, _ := output.SetForTest(t)

	uninstallCmd.Flags().Set("from", "test-prov")
	uninstallCmd.Flags().Set("dry-run", "true")
	defer uninstallCmd.Flags().Set("from", "")
	defer uninstallCmd.Flags().Set("dry-run", "false")

	err := uninstallCmd.RunE(uninstallCmd, []string{"my-skill"})
	if err != nil {
		t.Fatalf("expected no error for dry-run, got: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("expected [dry-run] prefix in output, got: %s", out)
	}
	if !strings.Contains(out, "my-skill") {
		t.Errorf("expected item name in output, got: %s", out)
	}
}

func TestUninstallForce(t *testing.T) {
	// Force uninstall should skip confirmation and remove the symlink.
	setupInstalledSkill(t)
	stdout, _ := output.SetForTest(t)

	uninstallCmd.Flags().Set("from", "test-prov")
	uninstallCmd.Flags().Set("force", "true")
	defer uninstallCmd.Flags().Set("from", "")
	defer uninstallCmd.Flags().Set("force", "false")

	err := uninstallCmd.RunE(uninstallCmd, []string{"my-skill"})
	if err != nil {
		t.Fatalf("expected no error for force uninstall, got: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Removed") {
		t.Errorf("expected 'Removed' in output, got: %s", out)
	}
}

func TestUninstallForceJSON(t *testing.T) {
	// Force uninstall in JSON mode should output structured JSON.
	setupInstalledSkill(t)
	stdout, _ := output.SetForTest(t)
	output.JSON = true

	uninstallCmd.Flags().Set("from", "test-prov")
	uninstallCmd.Flags().Set("force", "true")
	defer uninstallCmd.Flags().Set("from", "")
	defer uninstallCmd.Flags().Set("force", "false")

	err := uninstallCmd.RunE(uninstallCmd, []string{"my-skill"})
	if err != nil {
		t.Fatalf("expected no error for force JSON uninstall, got: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "my-skill") {
		t.Errorf("expected item name in JSON output, got: %s", out)
	}
	if !strings.Contains(out, "uninstalled_from") {
		t.Errorf("expected 'uninstalled_from' key in JSON output, got: %s", out)
	}
}
