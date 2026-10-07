package loadout

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
)

// ApplyOptions configures a loadout apply operation.
type ApplyOptions struct {
	Mode        string                  // "preview", "try", or "keep"
	Method      installer.InstallMethod // "symlink" (default) or "copy"
	ProjectRoot string
	HomeDir     string               // defaults to os.UserHomeDir() if empty
	RepoRoot    string               // catalog repo root for symlink source resolution
	Resolver    *config.PathResolver // optional path resolver for custom locations

	// SkipUnsupported applies the loadout without the hooks whose events the
	// target provider has no settings key for, instead of failing. Skipped
	// hooks are reported in ApplyResult.Warnings.
	SkipUnsupported bool

	// Force applies hooks with high-severity scanner findings, which the
	// apply otherwise refuses before changing anything.
	Force bool
}

// ScannerFindingsError rejects an apply because the loadout contains hooks
// with high-severity scanner findings. Callers can detect it with
// errors.As to suggest --force.
type ScannerFindingsError struct {
	Problems []string // one "name — problem" line per refused hook
}

func (e *ScannerFindingsError) Error() string {
	return fmt.Sprintf("%d hook(s) have high-severity scanner findings:\n  %s",
		len(e.Problems), strings.Join(e.Problems, "\n  "))
}

// UnsupportedHooksError rejects an apply because the loadout contains hooks
// whose events the target provider cannot read (syllago-xqlc1). Callers can
// detect it with errors.As to suggest --skip-unsupported.
type UnsupportedHooksError struct {
	Provider string
	Problems []string // one "name — problem" line per rejected hook
}

func (e *UnsupportedHooksError) Error() string {
	return fmt.Sprintf("%d hook(s) cannot be applied to %s:\n  %s",
		len(e.Problems), e.Provider, strings.Join(e.Problems, "\n  "))
}

// ApplyResult describes what happened during apply.
type ApplyResult struct {
	Actions     []PlannedAction // what was done (or planned, for preview)
	SnapshotDir string          // set on success for try/keep modes
	Warnings    []string

	// AutoRevertArmed reports whether a session-end auto-revert hook was
	// injected (try mode only). False when the provider has no session_end
	// event — the CLI must not then promise auto-revert.
	AutoRevertArmed bool
}

// restoreSnapshot is replaced in tests to fail a rollback.
var restoreSnapshot = snapshot.Restore

// Apply resolves, validates, and applies a loadout to the provider.
//
// The sequence is: Resolve -> Validate -> Preview -> Snapshot -> Apply items -> Record.
// If any step after snapshot creation fails, the snapshot is restored (all-or-nothing).
//
// Modes:
//   - "preview": computes actions without touching files. Good for dry runs.
//   - "try": applies changes and injects a SessionEnd hook that auto-reverts on session close.
//   - "keep": applies changes permanently.
//
// Gotchas:
//   - The snapshot is taken BEFORE any changes are made, so rollback always has clean state.
//   - The SessionEnd hook injected for "try" mode is NOT recorded in installed.json --
//     it lives only in the backed-up settings.json and gets reverted with the snapshot.
func Apply(manifest *Manifest, cat *catalog.Catalog, prov provider.Provider, opts ApplyOptions) (*ApplyResult, error) {
	refs, actions, opts, err := plan(manifest, cat, prov, opts)
	if err != nil {
		return nil, err
	}

	// For preview mode, return immediately
	if opts.Mode == "preview" {
		return &ApplyResult{Actions: actions}, nil
	}

	filesToBackup, err := refuse(actions, prov, opts)
	if err != nil {
		return nil, err
	}

	// Step 4: Create the snapshot. Each placement is recorded as it is
	// made, so the snapshot starts with none.
	var hookScripts []string
	for _, a := range actions {
		if a.Action == "merge-hook" {
			hookScripts = append(hookScripts, a.Name)
		}
	}

	snapshotDir, err := snapshot.Create(opts.ProjectRoot, manifest.Name, opts.Mode,
		filesToBackup, nil, hookScripts)
	if err != nil {
		return nil, fmt.Errorf("creating snapshot: %w", err)
	}

	// Step 5: Apply each action. On failure, rollback.
	var warnings []string
	for _, a := range actions {
		if a.Action == "skip-unsupported" {
			warnings = append(warnings, fmt.Sprintf("skipped %s: %s", a.Name, a.Problem))
		}
	}
	placeWarnings, placed, applyErr := applyActions(actions, refs, prov, opts, manifest.Name, snapshotDir)
	if applyErr != nil {
		// Rollback: restore snapshot and clean up. Read this apply's own
		// snapshot, so another one in the directory cannot stop the restore.
		sm, readErr := snapshot.ReadManifest(snapshotDir)
		var restoreErr error
		if readErr == nil {
			restoreErr = restoreSnapshot(snapshotDir, sm)
		}
		restored := readErr == nil && restoreErr == nil
		// A placement that will not go fails the rollback like a restore
		// that will not, so the snapshot stays and records it.
		removed, unplaceErr := unplace(placed, snapshotDir)
		restoreErr = errors.Join(restoreErr, unplaceErr)
		var recorded []snapshot.SymlinkRecord
		if readErr == nil {
			recorded = sm.Symlinks
		}
		left := leftBehind(placed, removed, recorded)
		// A snapshot that did not restore is the only copy of the files the
		// apply changed, so it stays: loadout remove retries the restore, and
		// a restore that keeps failing leaves the backups to copy by hand and
		// the files and symlinks the apply created to delete. The directory blocks the
		// next apply until it is gone, so each message says to delete it.
		if readErr != nil {
			return nil, fmt.Errorf("applying loadout: %w; rolling back failed: %w; %s cannot be read, so copy its backups back by hand, then delete that directory%s", applyErr, errors.Join(readErr, restoreErr), snapshotDir, left)
		}
		if restoreErr != nil {
			// A placement this rollback deleted, or a created file that is
			// gone, is not the apply's to delete if it appears. Until that
			// is recorded, remove could delete one, so it is not offered.
			// A restore that went through recorded the created files itself.
			if err := errors.Join(snapshot.Forget(snapshotDir, removed), snapshot.DropUncreated(snapshotDir)); err != nil {
				var excepted []string
				if len(removed) > 0 {
					excepted = append(excepted, strings.Join(removed, ", ")+", which rollback already deleted")
				}
				// A restore that failed partway names what it deleted.
				if !restored {
					excepted = append(excepted, "any path the restore error names as deleted")
				}
				except := ""
				if len(excepted) > 0 {
					except = " except " + strings.Join(excepted, ", and ")
				}
				return nil, fmt.Errorf("applying loadout: %w; rolling back failed: %w; copy the backups in %s back by hand, delete the files and symlinks its manifest.json lists under createdFiles and symlinks%s, then delete that directory%s", applyErr, errors.Join(restoreErr, err), snapshotDir, except, left)
			}
			return nil, fmt.Errorf("applying loadout: %w; rolling back failed: %w; run 'syllago loadout remove' to retry it, or copy the backups in %s back by hand, delete the files and symlinks its manifest.json lists under createdFiles and symlinks, then delete that directory%s", applyErr, restoreErr, snapshotDir, left)
		}
		if err := snapshot.Delete(snapshotDir); err != nil {
			return nil, fmt.Errorf("applying loadout (rolled back): %w; deleting the snapshot failed: %w; delete %s by hand rather than running 'syllago loadout remove'", applyErr, err, snapshotDir)
		}
		return nil, fmt.Errorf("applying loadout (rolled back): %w", applyErr)
	}
	warnings = append(warnings, placeWarnings...)

	// Step 6 (C7): For "try" mode, inject a session-end hook for auto-revert.
	autoRevertArmed := false
	if opts.Mode == "try" {
		injected, err := injectSessionEndHook(prov, opts.HomeDir, opts.Resolver)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed to inject session-end hook: %v", err))
		} else if !injected {
			warnings = append(warnings, fmt.Sprintf("%s has no session-end hook event, so this loadout cannot auto-revert; run 'syllago loadout remove' to undo it", prov.Name))
		}
		autoRevertArmed = injected
	}
	if err := snapshot.DropUncreated(snapshotDir); err != nil {
		warnings = append(warnings, fmt.Sprintf("files this apply did not create may be deleted by loadout remove: %v", err))
	}

	return &ApplyResult{
		Actions:         actions,
		SnapshotDir:     snapshotDir,
		Warnings:        warnings,
		AutoRevertArmed: autoRevertArmed,
	}, nil
}

// unplace deletes the symlinks and copies a failed apply placed, records
// each deletion in the snapshot in snapshotDir, and returns the paths it
// deleted.
func unplace(placed []snapshot.SymlinkRecord, snapshotDir string) ([]string, error) {
	var removed []string
	var errs []error
	for _, sr := range placed {
		if err := removePlaced(sr); err != nil {
			errs = append(errs, err)
			continue
		}
		removed = append(removed, sr.Path)
		// Recorded at once, so a rollback killed partway leaves no deleted
		// placement for a later remove. One that cannot be recorded stops
		// the deleting, so at most one deleted path stays recorded, and the
		// caller names it.
		if err := snapshot.Forget(snapshotDir, []string{sr.Path}); err != nil {
			errs = append(errs, err)
			break
		}
	}
	return removed, errors.Join(errs...)
}

// leftBehind names the placements rollback could not delete. A placement
// whose record failed to save is in no manifest, so every rollback error
// names it as one to delete by hand; one the snapshot records is left to
// whatever deletes the rest, so a path a provider writes again after a
// remove is not deleted a second time.
func leftBehind(placed []snapshot.SymlinkRecord, removed []string, recorded []snapshot.SymlinkRecord) string {
	var left, unrecorded []string
	for _, sr := range placed {
		switch {
		case slices.Contains(removed, sr.Path):
		case slices.ContainsFunc(recorded, func(r snapshot.SymlinkRecord) bool { return r.Path == sr.Path }):
			left = append(left, sr.Path)
		default:
			unrecorded = append(unrecorded, sr.Path)
		}
	}
	msg := ""
	if len(left) > 0 {
		msg += "; rollback could not delete " + strings.Join(left, ", ")
	}
	if len(unrecorded) > 0 {
		msg += "; rollback could not delete " + strings.Join(unrecorded, ", ") + ", which the snapshot does not record, so delete those by hand"
	}
	return msg
}

// plan resolves, validates, and previews a loadout, filling opts.HomeDir.
func plan(manifest *Manifest, cat *catalog.Catalog, prov provider.Provider, opts ApplyOptions) ([]ResolvedRef, []PlannedAction, ApplyOptions, error) {
	// Callers that change state hold the install lock across their
	// active-loadout check and every Apply, so the check stays true.
	if opts.HomeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, nil, opts, fmt.Errorf("getting home dir: %w", err)
		}
		opts.HomeDir = home
	}

	// Step 1: Resolve all manifest references to catalog items
	refs, err := Resolve(manifest, cat, prov.Slug)
	if err != nil {
		return nil, nil, opts, fmt.Errorf("resolving references: %w", err)
	}

	// Step 2: Validate resolved refs
	issues := Validate(refs, prov)
	if len(issues) > 0 {
		msg := "validation failed:"
		for _, issue := range issues {
			msg += fmt.Sprintf("\n  %s: %s", issue.Ref.Name, issue.Problem)
		}
		return nil, nil, opts, fmt.Errorf("%s", msg)
	}

	// Step 3: Preview what would happen
	actions, err := Preview(refs, prov, opts.ProjectRoot, opts.HomeDir, opts.Resolver)
	if err != nil {
		return nil, nil, opts, fmt.Errorf("previewing: %w", err)
	}

	return refs, actions, opts, nil
}

// Check runs every refusal Apply makes before it changes anything, without
// changing anything, whatever opts.Mode says. A caller applying to several
// providers checks them all first, so a refusal for one leaves the others
// unapplied too.
func Check(manifest *Manifest, cat *catalog.Catalog, prov provider.Provider, opts ApplyOptions) error {
	_, actions, opts, err := plan(manifest, cat, prov, opts)
	if err != nil {
		return err
	}
	_, err = refuse(actions, prov, opts)
	return err
}

// refuse returns the error that stops an apply of actions before it
// changes anything, or the files the apply would back up.
func refuse(actions []PlannedAction, prov provider.Provider, opts ApplyOptions) ([]string, error) {
	// Check for conflicts before doing anything
	for _, a := range actions {
		if a.Action == "error-conflict" {
			return nil, fmt.Errorf("conflict: %s %s: %s", a.Type.Label(), a.Name, a.Problem)
		}
	}

	// Hooks the provider can't read fail the whole apply unless the caller
	// opted into a partial one — no silent partial coverage (syllago-xqlc1).
	var unsupported []string
	for _, a := range actions {
		if a.Action == "skip-unsupported" {
			unsupported = append(unsupported, fmt.Sprintf("%s — %s", a.Name, a.Problem))
		}
	}
	if len(unsupported) > 0 && !opts.SkipUnsupported {
		return nil, &UnsupportedHooksError{Provider: prov.Name, Problems: unsupported}
	}

	// A hook with high-severity scanner findings fails the apply before
	// anything changes, unless the caller forced it.
	var flagged []string
	for _, a := range actions {
		if a.Action == "merge-hook" && a.Problem != "" {
			flagged = append(flagged, fmt.Sprintf("%s — %s", a.Name, a.Problem))
		}
	}
	if len(flagged) > 0 && !opts.Force {
		return nil, &ScannerFindingsError{Problems: flagged}
	}

	// Remove deletes what an apply placed, so each path must be one remove
	// accepts, and no two items may claim the same one.
	dests := make(map[string]string)
	for _, a := range actions {
		if a.Action != "create-symlink" {
			continue
		}
		if err := checkRemovablePath(a.Detail); err != nil {
			return nil, fmt.Errorf("placing %s: %w", a.Name, err)
		}
		if other, ok := dests[a.Detail]; ok {
			return nil, fmt.Errorf("conflict: %s and %s in this loadout both install to %s", other, a.Name, a.Detail)
		}
		dests[a.Detail] = a.Name
	}
	// Nor may one sit inside another: deleting the outer one would take
	// the inner one with it while the snapshot still recorded it.
	for _, a := range actions {
		if a.Action != "create-symlink" {
			continue
		}
		for dir := filepath.Dir(a.Detail); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
			if other, ok := dests[dir]; ok {
				return nil, fmt.Errorf("conflict: %s in this loadout installs to %s, inside %s where %s installs", a.Name, a.Detail, dir, other)
			}
		}
	}

	// Collect the files the snapshot backs up.
	filesToBackup := collectBackupFiles(actions, prov, opts)
	// A settings file the apply may create is the snapshot's to delete, so
	// no item may install over it, around it, or inside it either.
	writes := make(map[string]bool)
	for _, f := range filesToBackup {
		for dir := filepath.Clean(f); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
			if other, ok := dests[dir]; ok {
				return nil, fmt.Errorf("conflict: %s in this loadout installs to %s, which holds %s that the loadout also writes", other, dir, f)
			}
		}
		writes[filepath.Clean(f)] = true
	}
	for _, a := range actions {
		if a.Action != "create-symlink" {
			continue
		}
		for dir := filepath.Dir(a.Detail); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
			if writes[dir] {
				return nil, fmt.Errorf("conflict: %s in this loadout installs to %s, inside %s that the loadout also writes", a.Name, a.Detail, dir)
			}
		}
	}
	return filesToBackup, nil
}

// claim creates dst as an empty file, an empty directory, or the symlink
// itself, failing if anything is already there, so a placement never
// writes into something the apply did not make.
func claim(srcPath, dst string, method installer.InstallMethod) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if method != installer.MethodCopy {
		return os.Symlink(srcPath, dst)
	}
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.Mkdir(dst, 0755)
	}
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}

// applyActions executes each planned action against the filesystem and
// returns what the placements warn about and what it placed, recording
// each placement in the snapshot as it is made.
func applyActions(actions []PlannedAction, refs []ResolvedRef, prov provider.Provider, opts ApplyOptions, loadoutName, snapshotDir string) (warnings []string, placed []snapshot.SymlinkRecord, err error) {
	inst, err := installer.LoadInstalled(opts.ProjectRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("loading installed.json: %w", err)
	}

	source := "loadout:" + loadoutName

	for _, a := range actions {
		if a.Action == "skip-exists" || a.Action == "skip-unsupported" {
			continue
		}

		ref := findRefByName(refs, a.Type, a.Name)
		if ref == nil {
			return nil, placed, fmt.Errorf("internal error: no ref found for %s %s", a.Type, a.Name)
		}

		switch a.Action {
		case "create-symlink":
			srcPath := symlinkSource(*ref)
			// The preview found nothing here. Something that has appeared
			// since is not the apply's, so the claim fails and it is never
			// recorded as placed.
			if err := claim(srcPath, a.Detail, opts.Method); err != nil {
				return nil, placed, fmt.Errorf("placing %s at %s: %w", a.Name, a.Detail, err)
			}
			rec := snapshot.SymlinkRecord{Path: a.Detail, Target: srcPath, Copied: opts.Method == installer.MethodCopy}
			placed = append(placed, rec)
			if err := snapshot.AddSymlink(snapshotDir, rec); err != nil {
				return nil, placed, fmt.Errorf("recording %s: %w", a.Name, err)
			}
			if opts.Method == installer.MethodCopy {
				if err := installer.CopyContent(srcPath, a.Detail); err != nil {
					return nil, placed, fmt.Errorf("copying %s: %w", a.Name, err)
				}
			}
			inst.Symlinks = append(inst.Symlinks, installer.InstalledSymlink{
				Path:        a.Detail,
				Target:      srcPath,
				Source:      source,
				InstalledAt: time.Now(),
			})

		case "merge-hook":
			// The hook goes into the user's settings and fires in every
			// project, so its scripts live under the home directory rather
			// than in this project, where anything that can write to the
			// project could change what the hook runs. The directory is
			// recorded as a placed copy, so remove and rollback delete it.
			scriptsRoot := filepath.Join(opts.HomeDir, ".syllago", "loadout-hooks")
			if err := os.MkdirAll(scriptsRoot, 0755); err != nil {
				return nil, placed, fmt.Errorf("creating %s: %w", scriptsRoot, err)
			}
			scriptsDir, err := os.MkdirTemp(scriptsRoot, a.Name+"-")
			if err != nil {
				return nil, placed, fmt.Errorf("creating scripts directory for hook %s: %w", a.Name, err)
			}
			rec := snapshot.SymlinkRecord{Path: scriptsDir, Target: ref.Item.Path, Copied: true}
			placed = append(placed, rec)
			if err := snapshot.AddSymlink(snapshotDir, rec); err != nil {
				return nil, placed, fmt.Errorf("recording %s: %w", a.Name, err)
			}
			notices, err := applyHook(*ref, prov, opts, scriptsDir, inst, source)
			for _, n := range notices {
				warnings = append(warnings, n.Message)
			}
			if err != nil {
				return nil, placed, fmt.Errorf("merging hook %s: %w", a.Name, err)
			}

		case "merge-mcp":
			placement, err := installer.PlaceMCP(ref.Item, prov, opts.ProjectRoot, inst, source)
			if err != nil {
				return nil, placed, fmt.Errorf("merging MCP %s: %w", a.Name, err)
			}
			for _, n := range placement.Notices {
				warnings = append(warnings, n.Message)
			}
		}
	}

	// Save all tracking in one write
	if err := installer.SaveInstalled(opts.ProjectRoot, inst); err != nil {
		return nil, placed, fmt.Errorf("saving installed.json: %w", err)
	}

	return warnings, placed, nil
}

// settingsPathFor computes a provider's hook config path via the shared
// installer.HookConfigPath resolver, honoring the
// resolver's base dir override when configured. Returns an error for providers
// that HookConfigPath has no hook file for.
func settingsPathFor(prov provider.Provider, homeDir string, resolver *config.PathResolver) (string, error) {
	base := homeDir
	if resolver != nil {
		if bd := resolver.BaseDir(prov.Slug); bd != "" {
			base = bd
		}
	}
	return installer.HookConfigPath(prov, base)
}

// applyHook reads a hook item's hook file and places it through
// installer.PlaceHook, which scans the item and copies the scripts it runs
// into scriptsDir, recording the result under the loadout's source tag.
func applyHook(ref ResolvedRef, prov provider.Provider, opts ApplyOptions, scriptsDir string, inst *installer.Installed, source string) ([]installer.Notice, error) {
	h, err := hookManifest(ref.Item.Path)
	if err != nil {
		return nil, err
	}

	// Reject events the provider has no settings key for (syllago-xqlc1).
	// Apply already gates these via the skip-unsupported preview action;
	// this is the backstop at the merge point so dead config can never slip
	// through if the two diverge. PlaceHook validates the event first, so a
	// malformed one gets the injection-guard error.
	if converter.IsValidHookEvent(h.Event) && !converter.ProviderSupportsHookEvent(h.Event, prov.Slug) {
		return nil, fmt.Errorf("hook %q: %s does not support hook event %q", ref.Name, prov.Name, h.Event)
	}

	settingsPath, err := settingsPathFor(prov, opts.HomeDir, opts.Resolver)
	if err != nil {
		return nil, fmt.Errorf("hook %q: %w", ref.Name, err)
	}
	placement, err := installer.PlaceHook(ref.Item, h, prov, opts.ProjectRoot, settingsPath, scriptsDir, inst, source, installer.ScanOptions{Force: opts.Force})
	return placement.Notices, err
}

// injectSessionEndHook appends a session-end hook that runs
// "syllago loadout remove --auto" for try-mode auto-revert. The hook is NOT
// tracked in installed.json — it gets reverted when the snapshot restores the
// settings file.
//
// Returns injected=false (no error) when the provider has no session_end
// event: the key would be dead config the provider never reads, and for crush
// it would corrupt the real crush.json (wrong key and CC-shape group). Callers
// warn the user to revert manually in that case.
func injectSessionEndHook(prov provider.Provider, homeDir string, resolver *config.PathResolver) (injected bool, err error) {
	// Skip when the provider has no session_end event so we never write a key
	// the provider can't read (syllago-xqlc1).
	if _, ok := converter.TranslateHookEvent("session_end", prov.Slug); !ok {
		return false, nil
	}

	settingsPath, err := settingsPathFor(prov, homeDir, resolver)
	if err != nil {
		return false, err
	}

	// Encode the hook in the provider's native format via its adapter (mirrors
	// applyHook). A naive CC-shape append would write the wrong format for
	// non-CC providers (e.g. devin's split-event model). Not tracked in
	// installed.json — it is reverted when the snapshot restores the file.
	h := converter.Hook{
		Name:    "syllago-auto-revert",
		Event:   "session_end",
		Handler: converter.Handler{Type: "command", Command: "syllago loadout remove --auto"},
	}
	if _, err := installer.ApplyCanonicalHook(prov, h, settingsPath, ""); err != nil {
		return false, err
	}
	return true, nil
}

// collectBackupFiles determines which files need backing up before apply.
func collectBackupFiles(actions []PlannedAction, prov provider.Provider, opts ApplyOptions) []string {
	var files []string
	needsSettings := false
	needsMCPConfig := false

	for _, a := range actions {
		if a.Action == "merge-hook" {
			needsSettings = true
		}
		if a.Action == "merge-mcp" {
			needsMCPConfig = true
		}
	}

	// "try" mode backs up the settings file only when a session-end auto-revert
	// hook will actually be injected — i.e. the provider has a session_end
	// event. Backing it up otherwise means loadout remove would restore (and
	// so clobber) a file this apply never wrote to; for crush the settings
	// file is the real crush.json (syllago-xqlc1). Hooks in the loadout have
	// already set needsSettings via their merge-hook actions above.
	if opts.Mode == "try" && converter.ProviderSupportsHookEvent("session_end", prov.Slug) {
		needsSettings = true
	}

	if needsSettings {
		// Skip a provider whose hook path can't be resolved (Phase 1b /
		// adapter-less): there is nothing this apply will write there to back up.
		if path, err := settingsPathFor(prov, opts.HomeDir, opts.Resolver); err == nil {
			files = append(files, path)
		}
	}
	if needsMCPConfig {
		mcpPath, err := installer.MCPConfigPathFor(prov, opts.ProjectRoot)
		if err == nil {
			files = append(files, mcpPath)
		}
	}

	// installed.json is not backed up. Apply only adds records, and writes
	// them last, so a failed apply leaves the file alone; remove deletes the
	// loadout's records, which keeps the ones a later install adds, where
	// restoring the file would drop them.

	return files
}

// findRefByName finds a ResolvedRef by type and name.
func findRefByName(refs []ResolvedRef, ct catalog.ContentType, name string) *ResolvedRef {
	for i := range refs {
		if refs[i].Type == ct && refs[i].Name == name {
			return &refs[i]
		}
	}
	return nil
}

// findHookFile locates the hook JSON file in an item directory.
// Checks hook.json first, then falls back to any .json file.
func findHookFile(itemDir string) string {
	hookPath := filepath.Join(itemDir, "hook.json")
	if _, err := os.Stat(hookPath); err == nil {
		return hookPath
	}
	entries, err := os.ReadDir(itemDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			return filepath.Join(itemDir, e.Name())
		}
	}
	return ""
}
