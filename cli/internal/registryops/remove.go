package registryops

// Shared registry-remove orchestrator (bead syllago-nb5ed Phase C).
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

import (
	"errors"
	"fmt"

	"github.com/OpenScribbler/syllago/cli/internal/config"
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
}

// RemoveRegistry orchestrates a single registry remove. Loads global
// config, prunes the named entry, saves, then best-effort deletes the
// clone. Returns ErrRemoveNotFound when the name isn't configured (silent
// success on a typo is the bug class we're guarding against).
func RemoveRegistry(name string) (RemoveOutcome, error) {
	out := RemoveOutcome{Name: name}

	cfg, err := config.LoadGlobal()
	if err != nil {
		return out, fmt.Errorf("load global config: %w", err)
	}

	found := false
	filtered := make([]config.Registry, 0, len(cfg.Registries))
	for _, r := range cfg.Registries {
		if r.Name == name {
			found = true
			continue
		}
		filtered = append(filtered, r)
	}
	if !found {
		return out, fmt.Errorf("%w: %q", ErrRemoveNotFound, name)
	}

	cfg.Registries = filtered
	if err := config.SaveGlobal(cfg); err != nil {
		return out, fmt.Errorf("%w: %w", ErrRemoveSaveFailed, err)
	}

	// Best-effort: a leftover clone is recoverable (rerun remove or rm -rf
	// the cache dir). A failed config save is the unrecoverable case, and
	// we already returned above on that.
	if err := RemoveFn(name); err != nil {
		out.CloneRemoveErr = err
	}

	return out, nil
}
