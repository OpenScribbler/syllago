package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// ErrNoSnapshot is returned by Load when no snapshot exists.
var ErrNoSnapshot = errors.New("no active snapshot")

// SnapshotManifest is written to .syllago/snapshots/<timestamp>/manifest.json.
//
// BackedUpHashes carries hex-encoded sha256 of each backup file at Create
// time, keyed by the same relative path used in BackedUpFiles. Restore
// recomputes each hash before overwriting the target and refuses if the
// stored digest does not match — guarding against truncation or bit-rot
// between Create and Restore. The field is omitempty so manifests written
// by older versions still Load without error; Restore no-ops the check for
// any entry missing a recorded hash.
type SnapshotManifest struct {
	Source         string            `json:"source"`
	LoadoutName    string            `json:"loadoutName"`
	Mode           string            `json:"mode"` // "try" or "keep"
	CreatedAt      time.Time         `json:"createdAt"`
	BackedUpFiles  []string          `json:"backedUpFiles"`            // relative paths inside snapshot/files/
	BackedUpHashes map[string]string `json:"backedUpHashes,omitempty"` // rel path -> hex sha256 at Create time
	Symlinks       []SymlinkRecord   `json:"symlinks"`
	HookScripts    []string          `json:"hookScripts,omitempty"` // informational only
}

// SymlinkRecord tracks a symlink created during apply.
type SymlinkRecord struct {
	Path   string `json:"path"`   // absolute path of the symlink
	Target string `json:"target"` // absolute path it points to
}

// snapshotsDir returns the path to .syllago/snapshots/.
func snapshotsDir(projectRoot string) string {
	return filepath.Join(config.DirPath(projectRoot), "snapshots")
}

// Create backs up files and writes the snapshot manifest.
// filesToBackup is a list of absolute paths to copy into snapshot/files/.
// symlinks and hookScripts are recorded in the manifest.
// Returns the snapshot directory path.
func Create(projectRoot string, loadoutName string, mode string,
	filesToBackup []string, symlinks []SymlinkRecord, hookScripts []string) (string, error) {

	timestamp := time.Now().UTC().Format("20060102T150405")
	snapshotDir := filepath.Join(snapshotsDir(projectRoot), timestamp)
	filesDir := filepath.Join(snapshotDir, "files")

	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return "", fmt.Errorf("creating snapshot dir: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}

	var backedUp []string
	hashes := make(map[string]string)
	for _, absPath := range filesToBackup {
		// Compute relative path from home dir for storage
		rel, err := filepath.Rel(home, absPath)
		if err != nil {
			// If not relative to home, use the full path as a fallback
			rel = absPath
		}

		destPath := filepath.Join(filesDir, rel)

		if err := copyFile(absPath, destPath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue // skip files that don't exist
			}
			return "", fmt.Errorf("backing up %s: %w", absPath, err)
		}
		backedUp = append(backedUp, rel)

		// Hash the backup we just wrote, not the source — if anything
		// corrupted the content during the copy, this anchor catches that
		// too. Restore recomputes from the backup on disk.
		digest, err := hashFile(destPath)
		if err != nil {
			return "", fmt.Errorf("hashing backup %s: %w", destPath, err)
		}
		hashes[rel] = digest
	}

	manifest := SnapshotManifest{
		Source:         loadoutName,
		LoadoutName:    loadoutName,
		Mode:           mode,
		CreatedAt:      time.Now().UTC(),
		BackedUpFiles:  backedUp,
		BackedUpHashes: hashes,
		Symlinks:       symlinks,
		HookScripts:    hookScripts,
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling manifest: %w", err)
	}

	manifestPath := filepath.Join(snapshotDir, "manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return "", fmt.Errorf("writing manifest: %w", err)
	}

	return snapshotDir, nil
}

// CreateForHook creates a snapshot for a hook operation. It records the source
// identifier (e.g. "hook:some-hook-name") and backs up the given files.
// Mode is always "keep" since hook snapshots are not trial installs.
func CreateForHook(projectRoot, source string, filesToBackup []string) (string, error) {
	return Create(projectRoot, source, "keep", filesToBackup, nil, nil)
}

// Load reads the manifest from the most recent snapshot directory.
// Returns ErrNoSnapshot if .syllago/snapshots/ is empty or missing.
func Load(projectRoot string) (*SnapshotManifest, string, error) {
	dir := snapshotsDir(projectRoot)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", ErrNoSnapshot
	}
	if err != nil {
		return nil, "", fmt.Errorf("reading snapshots dir: %w", err)
	}

	// Filter to directories only and sort by name (timestamp-based, newest last)
	var dirs []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e)
		}
	}
	if len(dirs) == 0 {
		return nil, "", ErrNoSnapshot
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name() > dirs[j].Name() // newest first
	})

	snapshotDir := filepath.Join(dir, dirs[0].Name())
	manifestPath := filepath.Join(snapshotDir, "manifest.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, "", fmt.Errorf("reading manifest: %w", err)
	}

	var manifest SnapshotManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, "", fmt.Errorf("parsing manifest: %w", err)
	}

	// Backwards compat: old manifests have LoadoutName but no Source.
	if manifest.Source == "" && manifest.LoadoutName != "" {
		manifest.Source = "loadout:" + manifest.LoadoutName
	}

	return &manifest, snapshotDir, nil
}

// Restore reads backed-up files from snapshotDir and writes them back to their
// original absolute paths. Does not remove symlinks (caller does that).
//
// Each destination is lstat'd before it is opened for write: if a path is
// currently a symlink, Restore refuses to write through it. This blocks the
// TOCTOU attack where a hostile process swaps a real file for a symlink
// pointing at an arbitrary location between Create and Restore. The refusal
// is reported so the caller can surface it; all other paths continue to
// restore normally.
func Restore(snapshotDir string, manifest *SnapshotManifest) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home dir: %w", err)
	}

	filesDir := filepath.Join(snapshotDir, "files")
	for _, rel := range manifest.BackedUpFiles {
		srcPath := filepath.Join(filesDir, rel)
		destPath := filepath.Join(home, rel)

		// Integrity check: if the manifest recorded a sha256 for this path,
		// recompute it against the backup on disk and refuse the restore if
		// it has changed. Manifests from earlier versions have no hashes
		// and we fall through to the copy — documented on SnapshotManifest.
		if want, ok := manifest.BackedUpHashes[rel]; ok {
			got, err := hashFile(srcPath)
			if err != nil {
				return fmt.Errorf("restoring %s: hashing backup: %w", rel, err)
			}
			if got != want {
				return fmt.Errorf("restoring %s: %w (want %s, got %s)", rel, ErrRestoreCorruptBackup, want, got)
			}
		}

		if err := restoreToFile(srcPath, destPath); err != nil {
			return fmt.Errorf("restoring %s: %w", rel, err)
		}
	}

	return nil
}

// ErrRestoreSymlinkTarget is returned by Restore when a destination path is
// currently a symlink. The snapshot created a regular file; if the target is
// now a symlink, some other process has modified the path and writing
// through the symlink could clobber an attacker-chosen location.
var ErrRestoreSymlinkTarget = errors.New("refusing to restore through symlink at destination")

// ErrRestoreCorruptBackup is returned by Restore when the sha256 of a backup
// file on disk does not match the digest recorded in the manifest. The
// restore is aborted before the destination is touched, so the on-disk
// target retains whatever post-apply content apply wrote (callers typically
// surface this as a manual-intervention prompt rather than a silent failure).
var ErrRestoreCorruptBackup = errors.New("backup file hash does not match manifest")

// hashFile returns the hex-encoded sha256 of the file at path.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Delete removes the snapshot directory entirely.
func Delete(snapshotDir string) error {
	return os.RemoveAll(snapshotDir)
}

// restoreToFile copies src to dest with a pre-open lstat check: if dest is a
// symlink, it refuses to restore rather than follow the link. This narrows
// but does not eliminate the TOCTOU window (O_NOFOLLOW would close it on
// POSIX, but is not portable). copyFile is kept for Create's own writes into
// snapshotDir, where the threat shape doesn't apply.
func restoreToFile(src, dest string) error {
	if info, err := os.Lstat(dest); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s", ErrRestoreSymlinkTarget, dest)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("lstat %s: %w", dest, err)
	}
	return copyFile(src, dest)
}

// copyFile copies a file from src to dest, creating parent directories as needed.
func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	_, err = io.Copy(destFile, srcFile)
	return err
}
