package installer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
)

// TestHookInstallAndUninstall_LeaveNoSnapshot: the settings write is atomic,
// so hook install and uninstall keep no snapshot. One left behind reads as
// an active loadout to every loadout command, and removing it would delete
// a settings file the install created along with everything written there
// since.
func TestHookInstallAndUninstall_LeaveNoSnapshot(t *testing.T) {
	isolateLegacyRoot(t)
	item, projectRoot := writeCanonicalHookItem(t, "guard", "before_tool_execute", "shell", "echo hi")
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	overrideHookSettingsPath(t, settingsPath)

	if _, err := installHook(item, provider.ClaudeCode, projectRoot, ScanOptions{}); err != nil {
		t.Fatalf("installHook: %v", err)
	}
	if _, _, err := snapshot.Load(projectRoot); !errors.Is(err, snapshot.ErrNoSnapshot) {
		t.Errorf("after install: got %v, want no snapshot", err)
	}
	if _, err := uninstallHook(item, provider.ClaudeCode, projectRoot); err != nil {
		t.Fatalf("uninstallHook: %v", err)
	}
	if _, _, err := snapshot.Load(projectRoot); !errors.Is(err, snapshot.ErrNoSnapshot) {
		t.Errorf("after uninstall: got %v, want no snapshot", err)
	}
	if _, err := os.Stat(settingsPath); err != nil {
		t.Errorf("settings file: %v", err)
	}
}
