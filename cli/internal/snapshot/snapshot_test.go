package snapshot

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCreate_BacksUpFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a file to back up
	testFile := filepath.Join(tmpDir, "test-file.json")
	content := []byte(`{"key": "value"}`)
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	symlinks := []SymlinkRecord{
		{Path: "/tmp/link1", Target: "/tmp/target1"},
	}

	snapshotDir, err := Create(tmpDir, "test-loadout", "keep",
		[]string{testFile}, symlinks, []string{"./script.sh"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify manifest exists
	manifestPath := filepath.Join(snapshotDir, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest not created: %v", err)
	}

	// Verify files directory exists
	filesDir := filepath.Join(snapshotDir, "files")
	if _, err := os.Stat(filesDir); err != nil {
		t.Fatalf("files dir not created: %v", err)
	}
}

func TestCreate_SkipsMissingFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Try to back up a file that doesn't exist
	_, err := Create(tmpDir, "test-loadout", "keep",
		[]string{"/nonexistent/file.json"}, nil, nil)
	if err != nil {
		t.Fatalf("Create should not fail for missing files: %v", err)
	}
}

func TestLoad_NoSnapshot(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_, _, err := Load(tmpDir)
	if !errors.Is(err, ErrNoSnapshot) {
		t.Fatalf("expected ErrNoSnapshot, got: %v", err)
	}
}

func TestLoad_EmptySnapshotsDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".syllago", "snapshots"), 0755)

	_, _, err := Load(tmpDir)
	if !errors.Is(err, ErrNoSnapshot) {
		t.Fatalf("expected ErrNoSnapshot for empty dir, got: %v", err)
	}
}

func TestCreate_ManifestContents(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	symlinks := []SymlinkRecord{
		{Path: "/home/user/.claude/rules/my-rule.md", Target: "/repo/content/rules/my-rule.md"},
	}

	snapshotDir, err := Create(tmpDir, "my-loadout", "try",
		nil, symlinks, []string{"script.sh"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	manifest, loadedDir, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loadedDir != snapshotDir {
		t.Errorf("snapshot dir mismatch: got %s, want %s", loadedDir, snapshotDir)
	}
	if manifest.LoadoutName != "my-loadout" {
		t.Errorf("loadout name: got %s, want my-loadout", manifest.LoadoutName)
	}
	if manifest.Mode != "try" {
		t.Errorf("mode: got %s, want try", manifest.Mode)
	}
	if len(manifest.Symlinks) != 1 {
		t.Errorf("symlinks count: got %d, want 1", len(manifest.Symlinks))
	}
	if len(manifest.HookScripts) != 1 || manifest.HookScripts[0] != "script.sh" {
		t.Errorf("hook scripts: got %v, want [script.sh]", manifest.HookScripts)
	}
}

func TestRestore_RestoresContent(t *testing.T) {
	t.Parallel()

	// Restore writes files back to os.UserHomeDir()/rel, so we need to
	// use the real home dir. Create a subdirectory under home for isolation.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("getting home dir: %v", err)
	}

	projectRoot := t.TempDir()
	testDir := filepath.Join(home, ".syllago-test-restore-"+filepath.Base(projectRoot))
	os.MkdirAll(testDir, 0755)
	t.Cleanup(func() { os.RemoveAll(testDir) })

	// Write original content
	testFile := filepath.Join(testDir, "settings.json")
	originalContent := []byte(`{"original": true}`)
	os.WriteFile(testFile, originalContent, 0644)

	// Create snapshot that backs up the file
	snapshotDir, err := Create(projectRoot, "restore-test", "keep",
		[]string{testFile}, nil, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Overwrite the file (simulating apply changes)
	os.WriteFile(testFile, []byte(`{"modified": true}`), 0644)

	// Load and restore
	manifest, loadedDir, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loadedDir != snapshotDir {
		t.Fatalf("snapshot dir mismatch")
	}

	if err := Restore(snapshotDir, manifest); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify content was restored
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("reading restored file: %v", err)
	}
	if string(data) != string(originalContent) {
		t.Errorf("file not restored: got %q, want %q", string(data), string(originalContent))
	}
}

func TestLoad_ReturnsLatestSnapshot(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	// Create two snapshots with different timestamps by manually creating dirs.
	// The Load function sorts by directory name (timestamp-based, newest first).
	snapshotsPath := filepath.Join(projectRoot, ".syllago", "snapshots")

	// Create an older snapshot
	olderDir := filepath.Join(snapshotsPath, "20250101T000000")
	os.MkdirAll(olderDir, 0755)
	olderManifest := `{"loadoutName":"older","mode":"keep","createdAt":"2025-01-01T00:00:00Z","symlinks":[]}`
	os.WriteFile(filepath.Join(olderDir, "manifest.json"), []byte(olderManifest), 0644)

	// Create a newer snapshot
	newerDir := filepath.Join(snapshotsPath, "20250201T000000")
	os.MkdirAll(newerDir, 0755)
	newerManifest := `{"loadoutName":"newer","mode":"try","createdAt":"2025-02-01T00:00:00Z","symlinks":[]}`
	os.WriteFile(filepath.Join(newerDir, "manifest.json"), []byte(newerManifest), 0644)

	// Load should return the newer one
	manifest, dir, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if manifest.LoadoutName != "newer" {
		t.Errorf("expected newest snapshot (newer), got %q", manifest.LoadoutName)
	}
	if dir != newerDir {
		t.Errorf("expected dir %s, got %s", newerDir, dir)
	}
}

func TestCreateForHook(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	snapshotDir, err := CreateForHook(projectRoot, "hook:my-hook", nil)
	if err != nil {
		t.Fatalf("CreateForHook failed: %v", err)
	}

	manifest, loadedDir, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loadedDir != snapshotDir {
		t.Errorf("snapshot dir mismatch: got %s, want %s", loadedDir, snapshotDir)
	}
	if manifest.Source != "hook:my-hook" {
		t.Errorf("Source: got %q, want %q", manifest.Source, "hook:my-hook")
	}
	if manifest.Mode != "keep" {
		t.Errorf("Mode: got %q, want %q", manifest.Mode, "keep")
	}
}

func TestLoad_BackwardsCompat(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	// Write a manifest that has LoadoutName but no Source (old format).
	snapshotsPath := filepath.Join(projectRoot, ".syllago", "snapshots")
	snapshotDir := filepath.Join(snapshotsPath, "20250301T120000")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		t.Fatal(err)
	}
	oldManifest := `{"loadoutName":"my-loadout","mode":"keep","createdAt":"2025-03-01T12:00:00Z","symlinks":[]}`
	if err := os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), []byte(oldManifest), 0644); err != nil {
		t.Fatal(err)
	}

	manifest, _, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if manifest.Source != "loadout:my-loadout" {
		t.Errorf("Source: got %q, want %q", manifest.Source, "loadout:my-loadout")
	}
	if manifest.LoadoutName != "my-loadout" {
		t.Errorf("LoadoutName should be preserved: got %q", manifest.LoadoutName)
	}
}

func TestCreateForHook_Restore(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("getting home dir: %v", err)
	}

	projectRoot := t.TempDir()
	testDir := filepath.Join(home, ".syllago-test-hook-restore-"+filepath.Base(projectRoot))
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(testDir) })

	testFile := filepath.Join(testDir, "hook-settings.json")
	originalContent := []byte(`{"hook": "original"}`)
	if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
		t.Fatal(err)
	}

	snapshotDir, err := CreateForHook(projectRoot, "hook:my-hook", []string{testFile})
	if err != nil {
		t.Fatalf("CreateForHook failed: %v", err)
	}

	// Modify the file to simulate hook apply changes.
	if err := os.WriteFile(testFile, []byte(`{"hook": "modified"}`), 0644); err != nil {
		t.Fatal(err)
	}

	manifest, loadedDir, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loadedDir != snapshotDir {
		t.Fatalf("snapshot dir mismatch")
	}

	if err := Restore(snapshotDir, manifest); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("reading restored file: %v", err)
	}
	if string(data) != string(originalContent) {
		t.Errorf("file not restored: got %q, want %q", string(data), string(originalContent))
	}
}

func TestDelete_RemovesDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	snapshotDir, err := Create(tmpDir, "test", "keep", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(snapshotDir); err != nil {
		t.Fatalf("snapshot dir should exist: %v", err)
	}

	if err := Delete(snapshotDir); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, err := os.Stat(snapshotDir); !os.IsNotExist(err) {
		t.Fatal("snapshot dir should not exist after delete")
	}
}
