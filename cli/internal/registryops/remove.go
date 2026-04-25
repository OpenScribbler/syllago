package registryops

// Shared registry-remove orchestrator (bead syllago-nb5ed Phase C, extended
// for MOAT cleanup in syllago-teat0).
//
// Before extraction the TUI's doRegistryRemoveCmd carried a project-local
// fallback (config.Load on contentRoot/projectRoot) that was dead code after
// the registries-are-global decision (2026-04-24, ADR via syllago-fhtxa).
// The CLI's registryRemoveCmd was already global-only; both surfaces now go
// through this single function so neither can drift back to layered logic.
//
// Clone deletion is best-effort: when the config save succeeded but the
// clone removal failed, the registry is no longer configured (state is
// consistent), but a stale on-disk clone remains. The orchestrator surfaces
// that as a non-fatal CloneRemoveErr field on the outcome so callers can
// emit a soft warning instead of a hard error.
//
// MOAT cleanup (syllago-teat0): when the removed registry has a
// ManifestURI (i.e. it was a MOAT registry), this orchestrator also:
//
//  1. Removes the on-disk manifest cache subtree at
//     <CacheDir>/moat/registries/<name>/ via moat.RemoveManifestCache.
//     Without this, EnrichFromMOATManifests would keep finding the cache
//     and rendering "MOAT cache missing" warnings every rescan after the
//     registry was already removed from config.
//
//  2. Prunes the per-registry pin state from the project's
//     <ProjectRoot>/.syllago/moat-lockfile.json via Lockfile.PruneRegistry.
//     Leaving the entry behind would mean a re-add at the same URL would
//     inherit a stale fetched_at and bypass freshness enforcement on the
//     first sync.
//
// Both cleanups are explicitly scoped: they MUST NOT touch
// lockfile.entries[] (the user's installed-item ledger — uninstalling a
// registry's clone does not retroactively uninstall items the user already
// chose to install) or lockfile.revoked_hashes[] (append-only by spec
// §Revocation Archival, ADR 0007 G-15 — pruning would let a user
// "un-revoke" content by remove+re-add, defeating G-15).

import (
	"errors"
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// RemoveFn is the test seam for clone deletion. Production calls
// registry.Remove. Tests swap it to assert what the orchestrator did
// without touching disk.
var RemoveFn = registry.Remove

var (
	// ErrRemoveNotFound: no registry with the given name in global config.
	ErrRemoveNotFound = errors.New("registry not found")

	// ErrRemoveSaveFailed: config.SaveGlobal failed after pruning the entry.
	// The clone is left intact in this case (we haven't removed it yet).
	ErrRemoveSaveFailed = errors.New("save config failed")
)

// RemoveOpts groups the inputs to RemoveRegistry. Only Name is required.
// ProjectRoot and CacheDir are optional — a caller that omits both still
// gets the legacy behavior (config prune + clone delete) with no MOAT
// cleanup. Both are needed for full MOAT cleanup; partial values cause
// the orchestrator to skip the parts it cannot reach.
type RemoveOpts struct {
	// Name is the registry name to remove (required).
	Name string

	// ProjectRoot is the project containing
	// .syllago/moat-lockfile.json. Empty skips lockfile pruning.
	// CLI passes findProjectRoot(); TUI passes a.projectRoot.
	ProjectRoot string

	// CacheDir is the global syllago cache directory (typically
	// config.GlobalDirPath()). Empty skips MOAT manifest-cache removal.
	CacheDir string
}

// RemoveOutcome reports what the orchestrator did. Callers use it to render
// surface-appropriate confirmation (CLI prints "Removed registry: %s"; TUI
// fires a registryRemoveDoneMsg toast).
type RemoveOutcome struct {
	// Name is the registry name that was removed (echo of input).
	Name string

	// CloneRemoveErr is non-nil when config save succeeded but the clone
	// directory could not be deleted. Treat as a warning, not a failure —
	// the registry is gone from config either way.
	CloneRemoveErr error

	// ManifestCacheRemoveErr is non-nil when the MOAT manifest cache
	// subtree could not be deleted. Treat as a warning — the next rescan
	// will warn about an orphaned cache, but trust state stays disabled
	// for an absent registry so there is no security impact.
	ManifestCacheRemoveErr error

	// LockfilePruneErr is non-nil when the lockfile load or save failed
	// while attempting to prune the registry's per-registry pin state.
	// Treat as a warning. Entries[] and RevokedHashes[] are NEVER touched
	// by this path even on success; this error covers I/O around
	// Registries[] only.
	LockfilePruneErr error
}

// RemoveRegistry orchestrates a single registry remove. Loads global
// config, prunes the named entry, saves, then best-effort deletes the
// clone, removes the MOAT manifest cache, and prunes lockfile pin state.
// Returns ErrRemoveNotFound when the name isn't configured (silent
// success on a typo is the bug class we're guarding against).
func RemoveRegistry(opts RemoveOpts) (RemoveOutcome, error) {
	out := RemoveOutcome{Name: opts.Name}

	cfg, err := config.LoadGlobal()
	if err != nil {
		return out, fmt.Errorf("load global config: %w", err)
	}

	// Capture the manifest URI BEFORE pruning so we can address the
	// lockfile pin state by URI after config has been rewritten. A
	// MOAT registry without a ManifestURI is a corrupt config row, but
	// rather than fail the remove we treat it as "no lockfile entry to
	// prune" — the user is already in a recovery flow, and refusing to
	// finish the remove because of that would only entrench the corruption.
	var manifestURI string
	found := false
	filtered := make([]config.Registry, 0, len(cfg.Registries))
	for _, r := range cfg.Registries {
		if r.Name == opts.Name {
			found = true
			manifestURI = r.ManifestURI
			continue
		}
		filtered = append(filtered, r)
	}
	if !found {
		return out, fmt.Errorf("%w: %q", ErrRemoveNotFound, opts.Name)
	}

	cfg.Registries = filtered
	if err := config.SaveGlobal(cfg); err != nil {
		return out, fmt.Errorf("%w: %w", ErrRemoveSaveFailed, err)
	}

	// Best-effort: a leftover clone is recoverable (rerun remove or rm -rf
	// the cache dir). A failed config save is the unrecoverable case, and
	// we already returned above on that.
	if err := RemoveFn(opts.Name); err != nil {
		out.CloneRemoveErr = err
	}

	// MOAT manifest-cache cleanup. Skipped silently when CacheDir is
	// empty (caller opted out) or when the registry name is invalid
	// (RemoveManifestCache validates the name; an invalid name means
	// nothing was cached for it under the safe path). Both branches are
	// soft because the registry is already gone from config.
	if opts.CacheDir != "" {
		if err := moat.RemoveManifestCache(opts.CacheDir, opts.Name); err != nil {
			out.ManifestCacheRemoveErr = err
		}
	}

	// MOAT lockfile pin-state cleanup. Only meaningful when we have both
	// a project root (lockfile lives per-project) and a ManifestURI (the
	// key in lf.Registries). A non-MOAT registry with no ManifestURI is
	// not in lf.Registries so this would be a no-op anyway — skip the
	// I/O. Entries[] and RevokedHashes[] are deliberately untouched per
	// the package doc.
	if opts.ProjectRoot != "" && manifestURI != "" {
		lockfilePath := moat.LockfilePath(opts.ProjectRoot)
		lf, lfErr := moat.LoadLockfile(lockfilePath)
		if lfErr != nil {
			out.LockfilePruneErr = lfErr
		} else {
			// Only write the lockfile back when the registry was
			// actually present. Avoids touching an untouched lockfile
			// (and its mtime, which the enrich-verify cache keys on)
			// on every remove.
			if _, ok := lf.Registries[manifestURI]; ok {
				lf.PruneRegistry(manifestURI)
				if err := lf.Save(lockfilePath); err != nil {
					out.LockfilePruneErr = err
				}
			}
		}
	}

	return out, nil
}
