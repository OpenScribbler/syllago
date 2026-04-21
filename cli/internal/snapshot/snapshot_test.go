package snapshot

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// TestRestore_MissingBackupFile_ReturnsError exercises the "backup file
// deleted between Create and Restore" failure mode. Restore must return
// an error rather than silently succeeding (which would leave the target
// in its unrestored, potentially-corrupted state with no caller signal).
//
// Regression target: the original TestRestore_RestoresContent only covered
// the happy round-trip — it did not verify Restore fails loudly when the
// backup is gone.
func TestRestore_MissingBackupFile_ReturnsError(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("getting home dir: %v", err)
	}

	projectRoot := t.TempDir()
	testDir := filepath.Join(home, ".syllago-test-missing-"+filepath.Base(projectRoot))
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(testDir) })

	testFile := filepath.Join(testDir, "settings.json")
	originalContent := []byte(`{"pre-apply": true}`)
	if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
		t.Fatal(err)
	}

	snapshotDir, err := Create(projectRoot, "missing-backup", "keep",
		[]string{testFile}, nil, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Simulate apply: overwrite target with "post-apply" content.
	postApplyContent := []byte(`{"post-apply": true}`)
	if err := os.WriteFile(testFile, postApplyContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Corrupt the snapshot: delete the backed-up file. A legitimate operator
	// would never do this, but an interrupted process, disk error, or external
	// cleanup could. Restore must surface the failure.
	filesDir := filepath.Join(snapshotDir, "files")
	var backupPath string
	err = filepath.Walk(filesDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			backupPath = path
		}
		return nil
	})
	if err != nil || backupPath == "" {
		t.Fatalf("could not locate backup file under %s: %v", filesDir, err)
	}
	if err := os.Remove(backupPath); err != nil {
		t.Fatalf("removing backup file: %v", err)
	}

	manifest, loadedDir, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	err = Restore(loadedDir, manifest)
	if err == nil {
		t.Fatal("Restore should fail when backup file is missing, but succeeded")
	}
	if !strings.Contains(err.Error(), "restoring") {
		t.Errorf("error should identify the restore step; got: %v", err)
	}

	// Target must be untouched by the failed Restore. If Restore partially
	// wrote and then erred, the target would be neither original nor
	// post-apply — that's exactly the "inconsistent state" the acceptance
	// criteria warns about.
	got, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("reading target after failed restore: %v", err)
	}
	if string(got) != string(postApplyContent) {
		t.Errorf("target file mutated by failed Restore\ngot:  %s\nwant: %s (unchanged from post-apply)", got, postApplyContent)
	}
}

// TestRestore_TruncatedBackupFile_Refuses pins the hardened behavior:
// when the backup file is truncated (or otherwise corrupted) between Create
// and Restore, Restore must refuse rather than overwrite the target with
// the damaged content.
//
// Before syllago-wlqqb copyFile used io.Copy with O_TRUNC on the destination
// and no integrity check — a zero-byte backup silently produced a zero-byte
// target with no error, a latent data-loss risk under partial writes or
// tampering. After the fix, Create records a sha256 per backup file and
// Restore verifies each digest; mismatches short-circuit with
// ErrRestoreCorruptBackup before the destination is touched.
func TestRestore_TruncatedBackupFile_Refuses(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("getting home dir: %v", err)
	}

	projectRoot := t.TempDir()
	testDir := filepath.Join(home, ".syllago-test-truncated-"+filepath.Base(projectRoot))
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(testDir) })

	testFile := filepath.Join(testDir, "settings.json")
	originalContent := []byte(`{"many":"bytes","here":"filler"}`)
	if err := os.WriteFile(testFile, originalContent, 0644); err != nil {
		t.Fatal(err)
	}

	snapshotDir, err := Create(projectRoot, "truncated-backup", "keep",
		[]string{testFile}, nil, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Apply-time modification — the installed/post-apply content that
	// should survive a refused Restore.
	postApplyContent := []byte(`{"post":"apply"}`)
	if err := os.WriteFile(testFile, postApplyContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Truncate the backup to zero bytes — simulates a partial write or
	// filesystem-level tamper between Create and Restore.
	filesDir := filepath.Join(snapshotDir, "files")
	var backupPath string
	_ = filepath.Walk(filesDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			backupPath = path
		}
		return nil
	})
	if backupPath == "" {
		t.Fatal("could not locate backup file")
	}
	if err := os.Truncate(backupPath, 0); err != nil {
		t.Fatalf("truncating backup: %v", err)
	}

	manifest, loadedDir, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	restoreErr := Restore(loadedDir, manifest)
	if restoreErr == nil {
		t.Fatal("Restore succeeded with a truncated backup — integrity hardening regressed (Restore must refuse with ErrRestoreCorruptBackup)")
	}
	if !errors.Is(restoreErr, ErrRestoreCorruptBackup) {
		t.Errorf("Restore error is not ErrRestoreCorruptBackup; got: %v", restoreErr)
	}

	// The target must still hold the post-apply content — a refusal means
	// the destination was never touched. If the content is now zero bytes,
	// the hash check ran AFTER the destination was opened (or didn't run
	// at all) and the fix is incomplete.
	got, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("reading target: %v", err)
	}
	if string(got) != string(postApplyContent) {
		t.Errorf("target clobbered despite refused Restore — integrity check ran too late\ngot:  %q (%d bytes)\nwant: %q (post-apply, unchanged)",
			got, len(got), postApplyContent)
	}
}

// TestLoad_CorruptManifest_ReturnsParseError covers the case where the
// snapshot's manifest.json is present but not valid JSON. Load must refuse
// rather than returning a zero-valued manifest that a caller would then
// apply. If a caller were allowed to proceed with an empty manifest, it
// would think there's nothing to restore — silently skipping the entire
// rollback.
func TestLoad_CorruptManifest_ReturnsParseError(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()

	snapshotDir := filepath.Join(projectRoot, ".syllago", "snapshots", "20260420T000000")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Obviously-bad JSON. Real corruption could also be truncation mid-write
	// or partial overwrite; either way, json.Unmarshal must reject it.
	if err := os.WriteFile(filepath.Join(snapshotDir, "manifest.json"),
		[]byte(`{"loadoutName":"x", "mode":"keep", INVALID`), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := Load(projectRoot)
	if err == nil {
		t.Fatal("Load should refuse corrupt manifest, got nil error")
	}
	if !strings.Contains(err.Error(), "parsing manifest") {
		t.Errorf("error should identify parse failure; got: %v", err)
	}
	// Must NOT be ErrNoSnapshot — a corrupt manifest is an existing-but-bad
	// snapshot. Silently treating it as absent would cause Load's callers
	// (e.g. Remove, rollback) to skip restoration entirely.
	if errors.Is(err, ErrNoSnapshot) {
		t.Error("corrupt manifest must not be reported as ErrNoSnapshot")
	}
}

// TestRestore_TargetReplacedWithSymlink_Refuses pins the hardened behavior:
// when the target path is replaced with a symlink between Create and Restore,
// Restore must refuse rather than write backup bytes through the symlink to
// wherever the link points.
//
// Threat shape: attacker observes syllago is about to rollback settings.json,
// swaps it for a symlink pointing at ~/.ssh/authorized_keys or similar. The
// pre-hardening behavior (os.OpenFile with default follow-symlink semantics)
// would clobber the attacker-chosen file with whatever the snapshot held.
// After syllago-ac47s, Restore lstats the destination first and aborts with
// ErrRestoreSymlinkTarget.
//
// This test confines the "attacker" symlink to a path inside the test's
// temp dir — no real-world paths are touched. A Restore regression (e.g.
// reverting to a plain copyFile call) would cause the side-file assertion
// to fire with the restored backup bytes.
func TestRestore_TargetReplacedWithSymlink_Refuses(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("getting home dir: %v", err)
	}

	projectRoot := t.TempDir()
	testDir := filepath.Join(home, ".syllago-test-symlink-"+filepath.Base(projectRoot))
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(testDir) })

	targetPath := filepath.Join(testDir, "settings.json")
	originalContent := []byte(`{"legitimate":"content"}`)
	if err := os.WriteFile(targetPath, originalContent, 0644); err != nil {
		t.Fatal(err)
	}

	_, err = Create(projectRoot, "symlink-swap", "keep",
		[]string{targetPath}, nil, nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Attacker-substituted file: a sentinel in the test's own directory.
	sideFile := filepath.Join(testDir, "attacker-chose-this.txt")
	sentinelContent := []byte(`attacker-controlled-content-DO-NOT-CLOBBER`)
	if err := os.WriteFile(sideFile, sentinelContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Swap target with a symlink pointing at the side file.
	if err := os.Remove(targetPath); err != nil {
		t.Fatalf("removing original target: %v", err)
	}
	if err := os.Symlink(sideFile, targetPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	manifest, loadedDir, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	restoreErr := Restore(loadedDir, manifest)
	if restoreErr == nil {
		t.Fatal("Restore succeeded against a symlink target — TOCTOU hardening regressed (Restore must refuse with ErrRestoreSymlinkTarget)")
	}
	if !errors.Is(restoreErr, ErrRestoreSymlinkTarget) {
		t.Errorf("Restore error is not ErrRestoreSymlinkTarget; got: %v", restoreErr)
	}

	// Side file must still hold its sentinel content — the refusal means no
	// write followed the symlink. If this fails, the hardening fired too
	// late (e.g. lstat happened after the open), or the restore bypassed
	// restoreToFile entirely.
	sideData, err := os.ReadFile(sideFile)
	if err != nil {
		t.Fatalf("reading side file: %v", err)
	}
	if string(sideData) != string(sentinelContent) {
		t.Errorf("side file was clobbered through the symlink — TOCTOU hardening failed\ngot:  %s\nwant: %s",
			sideData, sentinelContent)
	}

	// Target is still the symlink we planted — we did not clobber it with a
	// regular file. A future iteration could choose to replace the symlink
	// with the restored file; for now the policy is pure refusal.
	info, err := os.Lstat(targetPath)
	if err != nil {
		t.Fatalf("lstat target: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("target is no longer a symlink — Restore unexpectedly replaced the symlink (policy is refuse+leave-untouched)")
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
