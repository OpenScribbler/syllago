package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/OpenScribbler/nesco/cli/internal/config"
)

// ErrNoSnapshot is returned by Load when no snapshot exists.
var ErrNoSnapshot = errors.New("no active snapshot")

// SnapshotManifest is written to .nesco/snapshots/<timestamp>/manifest.json.
type SnapshotManifest struct {
	LoadoutName   string          `json:"loadoutName"`
	Mode          string          `json:"mode"` // "try" or "keep"
	CreatedAt     time.Time       `json:"createdAt"`
	BackedUpFiles []string        `json:"backedUpFiles"` // relative paths inside snapshot/files/
	Symlinks      []SymlinkRecord `json:"symlinks"`
	HookScripts   []string        `json:"hookScripts,omitempty"` // informational only
}

// SymlinkRecord tracks a symlink created during apply.
type SymlinkRecord struct {
	Path   string `json:"path"`   // absolute path of the symlink
	Target string `json:"target"` // absolute path it points to
}

// snapshotsDir returns the path to .nesco/snapshots/.
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
	}

	manifest := SnapshotManifest{
		LoadoutName:   loadoutName,
		Mode:          mode,
		CreatedAt:     time.Now().UTC(),
		BackedUpFiles: backedUp,
		Symlinks:      symlinks,
		HookScripts:   hookScripts,
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

// Load reads the manifest from the most recent snapshot directory.
// Returns ErrNoSnapshot if .nesco/snapshots/ is empty or missing.
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

	return &manifest, snapshotDir, nil
}

// Restore reads backed-up files from snapshotDir and writes them back to their
// original absolute paths. Does not remove symlinks (caller does that).
func Restore(snapshotDir string, manifest *SnapshotManifest) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home dir: %w", err)
	}

	filesDir := filepath.Join(snapshotDir, "files")
	for _, rel := range manifest.BackedUpFiles {
		srcPath := filepath.Join(filesDir, rel)
		destPath := filepath.Join(home, rel)

		if err := copyFile(srcPath, destPath); err != nil {
			return fmt.Errorf("restoring %s: %w", rel, err)
		}
	}

	return nil
}

// Delete removes the snapshot directory entirely.
func Delete(snapshotDir string) error {
	return os.RemoveAll(snapshotDir)
}

// copyFile copies a file from src to dest, creating parent directories as needed.
func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

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
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	return err
}
