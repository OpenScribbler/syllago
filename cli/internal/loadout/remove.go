package loadout

import (
	"errors"
	"os"

	"github.com/OpenScribbler/nesco/cli/internal/installer"
	"github.com/OpenScribbler/nesco/cli/internal/snapshot"
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
	RemovedSymlinks []string // absolute paths of symlinks deleted
	LoadoutName     string
}

// Remove reads the active snapshot, restores backed-up files, deletes symlinks,
// cleans up installed.json entries (those with source == "loadout:<name>"),
// and deletes the snapshot directory.
//
// How it works:
//  1. Load the most recent snapshot manifest.
//  2. Restore all backed-up files (settings.json, .claude.json, installed.json).
//     This automatically reverts hook entries, MCP entries, AND the SessionEnd
//     hook -- because we backed up the pre-apply state of these files.
//  3. Delete symlinks that were created during apply.
//  4. Clean up any installed.json entries tagged with "loadout:<name>".
//     Note: step 2 already restored installed.json from snapshot, so this step
//     handles the edge case where installed.json was modified after the loadout
//     was applied (e.g., user exported something separately).
//  5. Delete the snapshot directory.
//
// Gotcha: Symlink deletion ignores ErrNotExist because the user may have
// manually removed a symlink before running remove.
func Remove(opts RemoveOptions) (*RemoveResult, error) {
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
	if err := snapshot.Restore(snapshotDir, manifest); err != nil {
		return nil, err
	}
	home, _ := os.UserHomeDir()
	for _, rel := range manifest.BackedUpFiles {
		if home != "" {
			result.RestoredFiles = append(result.RestoredFiles, home+"/"+rel)
		} else {
			result.RestoredFiles = append(result.RestoredFiles, rel)
		}
	}

	// Step 2: Delete symlinks
	for _, sr := range manifest.Symlinks {
		err := os.Remove(sr.Path)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		result.RemovedSymlinks = append(result.RemovedSymlinks, sr.Path)
	}

	// Step 3: Clean installed.json entries for this loadout
	// The snapshot restore already put back the pre-apply installed.json,
	// but if anything was added to installed.json after the loadout apply,
	// we want to keep those entries. So we load the current state and remove
	// only the loadout-tagged entries.
	inst, err := installer.LoadInstalled(opts.ProjectRoot)
	if err == nil {
		source := "loadout:" + manifest.LoadoutName
		inst = cleanInstalledEntries(inst, source)
		installer.SaveInstalled(opts.ProjectRoot, inst)
	}

	// Step 4: Delete snapshot
	if err := snapshot.Delete(snapshotDir); err != nil {
		return nil, err
	}

	return result, nil
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
