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

	os.MkdirAll(filepath.Join(tmpDir, ".nesco", "snapshots"), 0755)

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
