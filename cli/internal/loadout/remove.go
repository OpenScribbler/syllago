package loadout

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
)

// ErrNoActiveLoadout is returned when no snapshot is found to revert.
var ErrNoActiveLoadout = errors.New("no active loadout to remove")

// RemoveOptions configures a loadout remove operation.
type RemoveOptions struct {
	Auto        bool // if true, skip confirmation; used by --auto flag from SessionEnd hook
	ProjectRoot string
}

// RemoveResult describes what was reverted.
type RemoveResult struct {
	RestoredFiles   []string // absolute paths of files restored from snapshot
	RemovedFiles    []string // absolute paths of files the apply created, deleted
	RemovedSymlinks []string // absolute paths of symlinks deleted
	LoadoutName     string
}

// Remove reads the active snapshot, restores backed-up files, deletes symlinks,
// cleans up installed.json entries (those with source == "loadout:<name>"),
// and deletes the snapshot directory.
//
// How it works:
//  1. Load the most recent snapshot manifest.
//  2. Restore all backed-up files (settings.json, .claude.json, an MCP
//     config) and delete the ones the apply created. This reverts hook
//     entries, MCP entries, AND the SessionEnd hook -- because we backed up
//     the pre-apply state of these files.
//  3. Delete symlinks that were created during apply.
//  4. Remove the installed.json entries tagged with "loadout:<name>". The
//     apply does not back installed.json up, so entries a later install
//     added stay. A failure here keeps the snapshot, so remove can run again.
//  5. Delete the snapshot directory.
//
// Gotcha: Symlink deletion ignores ErrNotExist because the user may have
// manually removed a symlink before running remove.
func Remove(opts RemoveOptions) (*RemoveResult, error) {
	// Callers hold the install lock from the snapshot they show the user
	// through this call, so Remove reverts the loadout they confirmed.
	manifest, snapshotDir, err := snapshot.Load(opts.ProjectRoot)
	if errors.Is(err, snapshot.ErrNoSnapshot) {
		return nil, ErrNoActiveLoadout
	}
	if err != nil {
		return nil, err
	}

	result := &RemoveResult{
		LoadoutName: manifest.LoadoutName,
	}

	// Step 1: Restore backed-up files
	home, _ := os.UserHomeDir()
	SkipInstalledBackup(manifest, opts.ProjectRoot)
	if err := snapshot.Restore(snapshotDir, manifest); err != nil {
		return nil, err
	}
	for _, rel := range manifest.BackedUpFiles {
		result.RestoredFiles = append(result.RestoredFiles, manifest.Destination(home, rel))
	}
	result.RemovedFiles = manifest.CreatedFiles

	// Step 2: Delete symlinks
	for _, sr := range manifest.Symlinks {
		err := os.Remove(sr.Path)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		result.RemovedSymlinks = append(result.RemovedSymlinks, sr.Path)
	}

	// Step 3: Clean installed.json entries for this loadout. Apply does not
	// back the file up, so records added after the apply survive. Drop the
	// loadout-tagged entries, along with the records of anything installed
	// after the apply into a file step 1 restored or deleted.
	inst, err := installer.LoadInstalled(opts.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("loading installed.json: %w", err)
	}
	inst = cleanInstalledEntries(inst, "loadout:"+manifest.LoadoutName)
	installer.ForgetReverted(inst, opts.ProjectRoot, slices.Concat(result.RestoredFiles, result.RemovedFiles))
	if err := installer.SaveInstalled(opts.ProjectRoot, inst); err != nil {
		return nil, fmt.Errorf("saving installed.json: %w", err)
	}

	// Step 4: Delete snapshot
	if err := snapshot.Delete(snapshotDir); err != nil {
		return nil, err
	}

	return result, nil
}

// SkipInstalledBackup drops installed.json from the files manifest restores.
// Snapshots from earlier versions back it up; remove cleans the loadout's
// records from it instead, which keeps the records of anything installed
// after the apply. It compares files rather than paths, so a project
// reached through another spelling of its path still matches.
func SkipInstalledBackup(manifest *snapshot.SnapshotManifest, projectRoot string) {
	current, err := os.Stat(filepath.Join(projectRoot, ".syllago", "installed.json"))
	if err != nil {
		return
	}
	home, _ := os.UserHomeDir()
	manifest.BackedUpFiles = slices.DeleteFunc(manifest.BackedUpFiles, func(rel string) bool {
		fi, err := os.Stat(manifest.Destination(home, rel))
		return err == nil && os.SameFile(fi, current)
	})
}

// cleanInstalledEntries removes all entries from Installed that match the given source.
func cleanInstalledEntries(inst *installer.Installed, source string) *installer.Installed {
	var hooks []installer.InstalledHook
	for _, h := range inst.Hooks {
		if h.Source != source {
			hooks = append(hooks, h)
		}
	}
	inst.Hooks = hooks

	var mcp []installer.InstalledMCP
	for _, m := range inst.MCP {
		if m.Source != source {
			mcp = append(mcp, m)
		}
	}
	inst.MCP = mcp

	var symlinks []installer.InstalledSymlink
	for _, s := range inst.Symlinks {
		if s.Source != source {
			symlinks = append(symlinks, s)
		}
	}
	inst.Symlinks = symlinks

	return inst
}
