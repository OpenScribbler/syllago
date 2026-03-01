package loadout

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
)

func TestRemove_NoSnapshot(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	_, err := Remove(RemoveOptions{ProjectRoot: projectRoot})
	if err != ErrNoActiveLoadout {
		t.Errorf("expected ErrNoActiveLoadout, got %v", err)
	}
}

func TestRemove_RestoresFiles(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	// Use the real home so that snapshot's filepath.Rel(home, path)
	// produces valid relative paths. The snapshot package stores files
	// relative to os.UserHomeDir() and restores them there.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("getting home dir: %v", err)
	}

	// Create a temp file under the real home for backup testing
	testDir := filepath.Join(home, ".syllago-test-remove-"+filepath.Base(projectRoot))
	os.MkdirAll(testDir, 0755)
	t.Cleanup(func() { os.RemoveAll(testDir) })

	originalContent := []byte(`{"original": true}`)
	testFile := filepath.Join(testDir, "test-settings.json")
	os.WriteFile(testFile, originalContent, 0644)

	// Create a snapshot that backs up the test file
	_, err = snapshot.Create(projectRoot, "test-loadout", "keep",
		[]string{testFile}, nil, nil)
	if err != nil {
		t.Fatalf("creating snapshot: %v", err)
	}

	// Modify the file to simulate loadout apply
	os.WriteFile(testFile, []byte(`{"modified": true}`), 0644)

	result, err := Remove(RemoveOptions{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.LoadoutName != "test-loadout" {
		t.Errorf("expected loadout name test-loadout, got %s", result.LoadoutName)
	}

	// Verify the file was restored
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("reading test file: %v", err)
	}
	if string(data) != string(originalContent) {
		t.Errorf("file not restored: got %s, want %s", string(data), string(originalContent))
	}
}

func TestRemove_DeletesSymlinks(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	// Create a symlink that the snapshot tracks
	targetDir := t.TempDir()
	symlinkPath := filepath.Join(targetDir, "test-symlink")
	sourceDir := t.TempDir()
	os.Symlink(sourceDir, symlinkPath)

	// Create snapshot with the symlink record
	_, err := snapshot.Create(projectRoot, "test-loadout", "keep",
		nil,
		[]snapshot.SymlinkRecord{{Path: symlinkPath, Target: sourceDir}},
		nil)
	if err != nil {
		t.Fatalf("creating snapshot: %v", err)
	}

	result, err := Remove(RemoveOptions{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify symlink was removed
	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Error("symlink should have been deleted")
	}

	if len(result.RemovedSymlinks) != 1 {
		t.Errorf("expected 1 removed symlink, got %d", len(result.RemovedSymlinks))
	}
}

func TestRemove_SymlinkAlreadyGone(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	// Create snapshot with a symlink that doesn't exist
	_, err := snapshot.Create(projectRoot, "test-loadout", "keep",
		nil,
		[]snapshot.SymlinkRecord{{Path: "/nonexistent/symlink", Target: "/whatever"}},
		nil)
	if err != nil {
		t.Fatalf("creating snapshot: %v", err)
	}

	// Should not error when symlink is already gone
	result, err := Remove(RemoveOptions{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("unexpected error (should ignore missing symlink): %v", err)
	}
	if result.LoadoutName != "test-loadout" {
		t.Errorf("expected loadout name test-loadout, got %s", result.LoadoutName)
	}
}

func TestRemove_CleansInstalledJSON(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	// Write installed.json with entries from this loadout AND from other sources
	inst := &installer.Installed{
		Hooks: []installer.InstalledHook{
			{Name: "loadout-hook", Event: "PostToolUse", Source: "loadout:test-loadout", InstalledAt: time.Now()},
			{Name: "export-hook", Event: "PreToolUse", Source: "export", InstalledAt: time.Now()},
		},
		Symlinks: []installer.InstalledSymlink{
			{Path: "/some/path", Target: "/some/target", Source: "loadout:test-loadout", InstalledAt: time.Now()},
		},
	}
	data, _ := json.MarshalIndent(inst, "", "  ")
	os.WriteFile(filepath.Join(projectRoot, ".syllago", "installed.json"), data, 0644)

	// Create a minimal snapshot
	_, err := snapshot.Create(projectRoot, "test-loadout", "keep", nil, nil, nil)
	if err != nil {
		t.Fatalf("creating snapshot: %v", err)
	}

	_, err = Remove(RemoveOptions{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify loadout entries were removed but export entries remain
	cleaned, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("loading installed.json: %v", err)
	}
	if len(cleaned.Hooks) != 1 {
		t.Errorf("expected 1 hook remaining (export), got %d", len(cleaned.Hooks))
	}
	if len(cleaned.Hooks) > 0 && cleaned.Hooks[0].Source != "export" {
		t.Errorf("expected remaining hook source to be 'export', got %q", cleaned.Hooks[0].Source)
	}
	if len(cleaned.Symlinks) != 0 {
		t.Errorf("expected 0 symlinks remaining, got %d", len(cleaned.Symlinks))
	}
}
