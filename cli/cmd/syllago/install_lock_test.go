package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
	"github.com/OpenScribbler/syllago/cli/internal/syllagolock"
)

// holdInstallLock points the lock at a temp global dir, shortens the wait,
// and holds the lock for the rest of the test.
func holdInstallLock(t *testing.T) {
	t.Helper()
	origDir := config.GlobalDirOverride
	config.GlobalDirOverride = t.TempDir()
	origTimeout := syllagolock.DefaultTimeout
	syllagolock.DefaultTimeout = 50 * time.Millisecond
	release, err := syllagolock.Acquire(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		release()
		syllagolock.DefaultTimeout = origTimeout
		config.GlobalDirOverride = origDir
	})
}

func requireLockedError(t *testing.T, err error) {
	t.Helper()
	var se output.StructuredError
	if !errors.As(err, &se) || se.Code != output.ErrSystemLocked {
		t.Fatalf("error = %v, want %s", err, output.ErrSystemLocked)
	}
}

func TestRunLoadoutApply_KeepWaitsForInstallLock(t *testing.T) {
	setupLoadoutApplyRepo(t, "demo", "claude-code", map[string][]string{"rules": {"r1"}})
	output.SetForTest(t)
	holdInstallLock(t)
	loadoutApplyCmd.Flags().Set("keep", "true")
	defer resetLoadoutApplyFlags()

	requireLockedError(t, loadoutApplyCmd.RunE(loadoutApplyCmd, []string{"demo"}))
}

// The lock comes before the catalog scan, so an apply never acts on a
// loadout it read before another writer finished.
func TestRunLoadoutApply_KeepTakesInstallLockBeforeReadingLoadout(t *testing.T) {
	setupLoadoutApplyRepo(t, "demo", "claude-code", nil)
	output.SetForTest(t)
	holdInstallLock(t)
	loadoutApplyCmd.Flags().Set("keep", "true")
	defer resetLoadoutApplyFlags()

	requireLockedError(t, loadoutApplyCmd.RunE(loadoutApplyCmd, []string{"not-yet-read"}))
}

func TestRunLoadoutApply_PreviewRunsWhileInstallLockHeld(t *testing.T) {
	root := setupLoadoutApplyRepo(t, "demo", "claude-code", map[string][]string{"rules": {"demo-rule"}})
	output.SetForTest(t)
	ruleDir := filepath.Join(root, "rules", "claude-code", "demo-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# Demo rule\n"), 0644)
	holdInstallLock(t)
	defer resetLoadoutApplyFlags()

	if err := loadoutApplyCmd.RunE(loadoutApplyCmd, []string{"demo"}); err != nil {
		t.Fatalf("preview should not wait on the lock, got %v", err)
	}
}

func TestRunLoadoutRemove_WaitsForInstallLock(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	output.SetForTest(t)
	withFakeRepoRoot(t, root)
	withNonInteractiveLoadout(t)
	writeSnapshot(t, root, &snapshot.SnapshotManifest{
		LoadoutName: "demo",
		Mode:        "keep",
		CreatedAt:   time.Date(2026, 3, 25, 14, 30, 0, 0, time.UTC),
	})
	holdInstallLock(t)
	loadoutRemoveCmd.Flags().Set("auto", "true")
	t.Cleanup(func() { loadoutRemoveCmd.Flags().Set("auto", "false") })

	requireLockedError(t, loadoutRemoveCmd.RunE(loadoutRemoveCmd, []string{}))
	if _, _, err := snapshot.Load(root); err != nil {
		t.Errorf("snapshot should survive a locked-out remove: %v", err)
	}
}
