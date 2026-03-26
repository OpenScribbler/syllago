package main

import (
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
