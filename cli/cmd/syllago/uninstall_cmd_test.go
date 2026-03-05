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
