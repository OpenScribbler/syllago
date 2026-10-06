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
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// ErrNoSnapshot is returned by Load when no snapshot exists.
var ErrNoSnapshot = errors.New("no active snapshot")

// SnapshotManifest is written to .syllago/snapshots/<timestamp>[-NNN]/manifest.json.
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
	// Destinations maps each backup's path inside files/ to the absolute
	// path it was copied from. Manifests written before it existed key each
	// backup by its path relative to the home directory, with "../" for a
	// file outside it, and restore to the home directory joined with it.
	Destinations map[string]string `json:"destinations,omitempty"`
	// CreatedFiles holds the absolute paths that did not exist at Create.
	// Restore removes them, so a config the apply created goes with it.
	CreatedFiles []string `json:"createdFiles,omitempty"`
	// RevertedFiles holds the created files a remove or rollback already
	// deleted. Restore leaves whatever is there now alone, but a retried
	// remove still forgets the installs that went into them.
	RevertedFiles []string `json:"revertedFiles,omitempty"`
}

// Destination returns the absolute path the backup at rel restores to.
func (m *SnapshotManifest) Destination(home, rel string) string {
	if dest, ok := m.Destinations[rel]; ok {
		return dest
	}
	return filepath.Join(home, rel)
}

// SymlinkRecord tracks a symlink created during apply, or the copy placed
// instead when the apply ran in copy mode.
type SymlinkRecord struct {
	Path   string `json:"path"`             // absolute path of the symlink or copy
	Target string `json:"target"`           // absolute path it points to, or was copied from
	Copied bool   `json:"copied,omitempty"` // a copy, which may be a directory
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
	snapshotDir, err := makeSnapshotDir(snapshotsDir(projectRoot), timestamp)
	if err != nil {
		return "", err
	}
	filesDir := filepath.Join(snapshotDir, "files")
	if err := os.Mkdir(filesDir, 0755); err != nil {
		return "", fmt.Errorf("creating snapshot dir: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home dir: %w", err)
	}

	var backedUp []string
	hashes := make(map[string]string)
	destinations := make(map[string]string)
	var created []string
	absPaths := make([]string, len(filesToBackup))
	for i, path := range filesToBackup {
		if absPaths[i], err = filepath.Abs(path); err != nil {
			return "", fmt.Errorf("backing up %s: %w", path, err)
		}
	}
	keys := backupKeys(home, absPaths)
	for i, absPath := range absPaths {
		rel := keys[i]
		destPath := filepath.Join(filesDir, rel)

		if err := copyFile(absPath, destPath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// A symlink to a missing target reads as missing too, but
				// it is the user's, so only an empty path counts as created.
				if _, err := os.Lstat(absPath); errors.Is(err, fs.ErrNotExist) {
					created = append(created, absPath)
				}
				continue
			}
			return "", fmt.Errorf("backing up %s: %w", absPath, err)
		}
		backedUp = append(backedUp, rel)
		destinations[rel] = absPath

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
		Destinations:   destinations,
		CreatedFiles:   created,
	}

	if err := writeManifest(snapshotDir, &manifest); err != nil {
		return "", err
	}

	return snapshotDir, nil
}

// makeSnapshotDir creates a directory under parent named for timestamp and
// returns its path. A multi-provider apply takes one snapshot per provider,
// often within one second, so a taken name gets a counter suffix rather
// than sharing the directory. The suffix has a fixed width so that Load's
// sort by name keeps creation order: T150405, T150405-001, T150405-002.
func makeSnapshotDir(parent, timestamp string) (string, error) {
	if err := os.MkdirAll(parent, 0755); err != nil {
		return "", fmt.Errorf("creating snapshot dir: %w", err)
	}
	for n := 0; n < 1000; n++ {
		name := timestamp
		if n > 0 {
			name = fmt.Sprintf("%s-%03d", timestamp, n)
		}
		dir := filepath.Join(parent, name)
		err := os.Mkdir(dir, 0755)
		if err == nil {
			return dir, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("creating snapshot dir: %w", err)
		}
	}
	return "", fmt.Errorf("creating snapshot dir: every name for %s is taken", timestamp)
}

// writeManifest replaces the snapshot's manifest through a rename, so a
// failed write leaves the previous one readable.
func writeManifest(snapshotDir string, manifest *SnapshotManifest) error {
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	manifestPath := filepath.Join(snapshotDir, "manifest.json")
	tmp := manifestPath + ".tmp"
	if err := os.WriteFile(tmp, manifestData, 0644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}
	if err := os.Rename(tmp, manifestPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("writing manifest: %w", err)
	}
	return nil
}

// AddSymlink records a path the apply has just claimed. An apply records
// each one only once it exists, so a crash leaves an unrecorded copy
// rather than a recorded path that remove would delete from someone else.
func AddSymlink(snapshotDir string, rec SymlinkRecord) error {
	manifest, err := ReadManifest(snapshotDir)
	if err != nil {
		return err
	}
	manifest.Symlinks = append(manifest.Symlinks, rec)
	return writeManifest(snapshotDir, manifest)
}

// Forget drops the paths a caller has just deleted from the snapshot's
// Symlinks, and moves them from CreatedFiles to RevertedFiles. Whatever
// appears at one of them afterward is not the apply's, even before anyone
// looks, so the record goes by the delete rather than by what exists when
// the snapshot is rewritten.
func Forget(snapshotDir string, deleted []string) error {
	if len(deleted) == 0 {
		return nil
	}
	manifest, err := ReadManifest(snapshotDir)
	if err != nil {
		return err
	}
	manifest.CreatedFiles = slices.DeleteFunc(manifest.CreatedFiles, func(p string) bool {
		if !slices.Contains(deleted, p) {
			return false
		}
		manifest.RevertedFiles = append(manifest.RevertedFiles, p)
		return true
	})
	manifest.Symlinks = slices.DeleteFunc(manifest.Symlinks, func(sr SymlinkRecord) bool {
		return slices.Contains(deleted, sr.Path)
	})
	return writeManifest(snapshotDir, manifest)
}

// DropUncreated drops from the snapshot's CreatedFiles and Symlinks each
// path that does not exist. An apply calls it once its writes are done, or
// once a failed rollback has deleted some of them: a path the apply did not
// write belongs to whoever makes it later.
func DropUncreated(snapshotDir string) error {
	manifest, err := ReadManifest(snapshotDir)
	if err != nil {
		return err
	}
	exists := func(p string) bool {
		_, err := os.Lstat(p)
		return !errors.Is(err, fs.ErrNotExist)
	}
	var created []string
	for _, p := range manifest.CreatedFiles {
		if exists(p) {
			created = append(created, p)
		}
	}
	var symlinks []SymlinkRecord
	for _, sr := range manifest.Symlinks {
		if exists(sr.Path) {
			symlinks = append(symlinks, sr)
		}
	}
	if len(created) == len(manifest.CreatedFiles) && len(symlinks) == len(manifest.Symlinks) {
		return nil
	}
	manifest.CreatedFiles = created
	manifest.Symlinks = symlinks
	return writeManifest(snapshotDir, manifest)
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

	// Filter to snapshots and sort by name (timestamp-based, newest first).
	// Earlier versions wrote backups of files outside the home directory
	// beside the snapshots rather than inside one, and a directory without
	// a manifest is one of those.
	var dirs []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		_, err := os.Stat(filepath.Join(dir, e.Name(), "manifest.json"))
		if err == nil {
			dirs = append(dirs, e)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, "", fmt.Errorf("reading snapshot %s: %w", e.Name(), err)
		}
	}
	if len(dirs) == 0 {
		return nil, "", ErrNoSnapshot
	}

	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name() > dirs[j].Name() // newest first
	})

	for _, d := range dirs {
		snapshotDir := filepath.Join(dir, d.Name())
		manifest, err := ReadManifest(snapshotDir)
		if err != nil {
			return nil, "", err
		}

		// Earlier versions took a snapshot for every hook install and
		// uninstall and never deleted it. None is a loadout, and restoring
		// one would drop every hook installed since.
		if strings.HasPrefix(manifest.LoadoutName, "hook-install:") || strings.HasPrefix(manifest.LoadoutName, "hook-uninstall:") {
			continue
		}

		return manifest, snapshotDir, nil
	}
	return nil, "", ErrNoSnapshot
}

// ReadManifest reads the manifest of the snapshot in snapshotDir.
func ReadManifest(snapshotDir string) (*SnapshotManifest, error) {
	data, err := os.ReadFile(filepath.Join(snapshotDir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var manifest SnapshotManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	// Backwards compat: old manifests have LoadoutName but no Source.
	if manifest.Source == "" && manifest.LoadoutName != "" {
		manifest.Source = "loadout:" + manifest.LoadoutName
	}
	return &manifest, nil
}

// backupKeys names each file's backup under files/. A file in the home
// directory keys by its path under it, which earlier versions restore
// correctly too. A file outside it has no such path, and its absolute path
// cannot sit under files/ on Windows, so it takes a numbered outside-home
// key, skipping any number a home path already uses there.
func backupKeys(home string, absPaths []string) []string {
	keys := make([]string, len(absPaths))
	used := make(map[string]bool)
	for i, p := range absPaths {
		if r, err := filepath.Rel(home, p); err == nil && filepath.IsLocal(r) {
			keys[i] = r
			if parts := strings.Split(strings.ToLower(filepath.ToSlash(r)), "/"); len(parts) > 1 && parts[0] == "outside-home" {
				used[parts[1]] = true
			}
		}
	}
	n := 0
	for i, p := range absPaths {
		if keys[i] != "" {
			continue
		}
		for used[strconv.Itoa(n)] {
			n++
		}
		keys[i] = filepath.Join("outside-home", strconv.Itoa(n), filepath.Base(p))
		n++
	}
	return keys
}

// Restore reads backed-up files from snapshotDir and writes them back to their
// original absolute paths, then removes the files that did not exist at
// Create and moves them from CreatedFiles to RevertedFiles in the manifest
// on disk. Does not remove symlinks (caller does that).
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

	// Every path syllago records is clean and absolute, so any other came
	// from a hand edit: a relative one would resolve against wherever
	// syllago runs, and "/home/u/../x" names a file other than it appears to.
	for rel, dest := range manifest.Destinations {
		if !filepath.IsAbs(dest) || filepath.Clean(dest) != dest {
			return fmt.Errorf("restoring %s: destination %q is not a clean absolute path", rel, dest)
		}
	}
	for _, path := range manifest.CreatedFiles {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return fmt.Errorf("removing %q: not a clean absolute path", path)
		}
	}

	filesDir := filepath.Join(snapshotDir, "files")
	for _, rel := range manifest.BackedUpFiles {
		srcPath := filepath.Join(filesDir, rel)
		destPath := manifest.Destination(home, rel)

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
	for i, path := range manifest.CreatedFiles {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			// A retry must not delete what appears at the paths this one
			// already deleted.
			removeErr := fmt.Errorf("removing %s: %w", path, err)
			if err := Forget(snapshotDir, manifest.CreatedFiles[:i]); err != nil {
				return fmt.Errorf("%w; restore deleted %s but could not record that in %s: %w; move those paths from createdFiles to revertedFiles in its manifest.json before trying again", removeErr, strings.Join(manifest.CreatedFiles[:i], ", "), snapshotDir, err)
			}
			return removeErr
		}
	}
	// So does a retry after a later step fails or the run is killed.
	if err := Forget(snapshotDir, manifest.CreatedFiles); err != nil {
		return fmt.Errorf("restore deleted %s but could not record that in %s: %w; move those paths from createdFiles to revertedFiles in its manifest.json before trying again", strings.Join(manifest.CreatedFiles, ", "), snapshotDir, err)
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
