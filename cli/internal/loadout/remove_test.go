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

	originalContent := []byte(`{"original": true}`)
	testFile := filepath.Join(t.TempDir(), "test-settings.json")
	os.WriteFile(testFile, originalContent, 0644)

	// Create a snapshot that backs up the test file
	_, err := snapshot.Create(projectRoot, "test-loadout", "keep",
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
		[]snapshot.SymlinkRecord{{Path: filepath.Join(projectRoot, "nonexistent", "symlink"), Target: filepath.Join(projectRoot, "whatever")}},
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
			{Path: filepath.Join(projectRoot, "some", "path"), Target: filepath.Join(projectRoot, "some", "target"), Source: "loadout:test-loadout", InstalledAt: time.Now()},
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

// A copy is deleted with everything under it, so a recorded path that is
// relative, unclean, or the filesystem root is refused before remove
// restores or deletes anything. These paths reach only the pure check,
// never a delete.
func TestCheckRemovablePath(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		path string
		ok   bool
	}{
		{"relative/copy", false},
		{"/", false},                       // path-literal-ok: checked by a pure function, never deleted
		{"//", false},                      // path-literal-ok: checked by a pure function, never deleted
		{"/tmp/..", false},                 // path-literal-ok: checked by a pure function, never deleted
		{"/tmp/./x", false},                // path-literal-ok: checked by a pure function, never deleted
		{"/home/u/.claude/skills/x", true}, // path-literal-ok: checked by a pure function, never deleted
	} {
		if err := checkRemovablePath(tc.path); (err == nil) != tc.ok {
			t.Errorf("checkRemovablePath(%q) = %v, want ok=%v", tc.path, err, tc.ok)
		}
	}
}

// Every unclean path sits inside the test's temp dir, so a broken check can
// delete nothing outside it. A restore or delete that ran first fails the
// test: it would revert backedUp, delete created, or delete the placed copy.
func TestRemove_RefusesUncleanPaths(t *testing.T) {
	t.Parallel()
	for name, unclean := range map[string]func(root string) string{
		"parent":       func(root string) string { return filepath.Join(root, "placed") + "/.." },
		"double slash": func(root string) string { return root + "//placed" },
		"dot":          func(root string) string { return root + "/./placed" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			projectRoot := t.TempDir()
			backedUp := filepath.Join(projectRoot, "backed-up.json")
			created := filepath.Join(projectRoot, "created.json")
			placed := filepath.Join(projectRoot, "placed")
			if err := os.WriteFile(backedUp, []byte("before"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(placed, 0o755); err != nil {
				t.Fatal(err)
			}
			snapshotDir, err := snapshot.Create(projectRoot, "test-loadout", "keep", []string{backedUp, created},
				[]snapshot.SymlinkRecord{{Path: placed, Copied: true}, {Path: unclean(projectRoot), Copied: true}}, nil)
			if err != nil {
				t.Fatalf("creating snapshot: %v", err)
			}
			// What the apply wrote: a changed backup and a new file.
			if err := os.WriteFile(backedUp, []byte("after"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(created, []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := Remove(RemoveOptions{Auto: true, ProjectRoot: projectRoot}); err == nil || !containsAll(err.Error(), "not a clean absolute path", "undo what its manifest.json lists by hand, reading any relative path from the directory the loadout was applied in", snapshotDir) {
				t.Fatalf("Remove: got %v, want the unclean path refused with the steps to undo it by hand", err)
			}
			for _, p := range []string{created, placed} {
				if _, err := os.Stat(p); err != nil {
					t.Errorf("%s should be untouched: %v", p, err)
				}
			}
			if got, err := os.ReadFile(backedUp); err != nil || string(got) != "after" {
				t.Errorf("backed-up.json was restored: %q, %v", got, err)
			}
			if _, _, err := snapshot.Load(projectRoot); err != nil {
				t.Errorf("snapshot should remain: %v", err)
			}
		})
	}
}
