package registryops

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// stubRemoveFn swaps the orchestrator's clone-deletion seam for the duration
// of the test. fn receives the registry name and returns the error to
// surface to the orchestrator.
func stubRemoveFn(t *testing.T, fn func(name string) error) {
	t.Helper()
	orig := RemoveFn
	RemoveFn = fn
	t.Cleanup(func() { RemoveFn = orig })
}

// withGlobalDir points config.LoadGlobal/SaveGlobal at a tempdir and seeds
// it with the given registries. Returns nothing — the side effect is the
// reset cleanup.
func withGlobalDir(t *testing.T, regs []config.Registry) {
	t.Helper()
	dir := t.TempDir()
	orig := config.GlobalDirOverride
	config.GlobalDirOverride = dir
	t.Cleanup(func() { config.GlobalDirOverride = orig })

	if len(regs) > 0 {
		if err := config.SaveGlobal(&config.Config{Registries: regs}); err != nil {
			t.Fatalf("seed config.SaveGlobal: %v", err)
		}
	}
}

func TestRemoveRegistry_PrunesNamedEntry(t *testing.T) {
	withGlobalDir(t, []config.Registry{
		{Name: "keep", URL: "https://example.com/keep.git"},
		{Name: "drop", URL: "https://example.com/drop.git"},
	})
	stubRemoveFn(t, func(name string) error { return nil })

	out, err := RemoveRegistry(RemoveOpts{Name: "drop"})
	if err != nil {
		t.Fatalf("RemoveRegistry: %v", err)
	}
	if out.Name != "drop" {
		t.Errorf("out.Name = %q, want %q", out.Name, "drop")
	}
	if out.CloneRemoveErr != nil {
		t.Errorf("CloneRemoveErr = %v, want nil", out.CloneRemoveErr)
	}

	cfg, _ := config.LoadGlobal()
	if len(cfg.Registries) != 1 || cfg.Registries[0].Name != "keep" {
		t.Errorf("post-remove registries = %+v, want [keep]", cfg.Registries)
	}
}

func TestRemoveRegistry_NotFound(t *testing.T) {
	withGlobalDir(t, []config.Registry{
		{Name: "only", URL: "https://example.com/only.git"},
	})
	stubRemoveFn(t, func(name string) error {
		t.Fatalf("RemoveFn must not be called when name not found, was called with %q", name)
		return nil
	})

	_, err := RemoveRegistry(RemoveOpts{Name: "ghost"})
	if !errors.Is(err, ErrRemoveNotFound) {
		t.Fatalf("err = %v, want errors.Is(ErrRemoveNotFound)", err)
	}

	// Config must be untouched on not-found — no partial write.
	cfg, _ := config.LoadGlobal()
	if len(cfg.Registries) != 1 || cfg.Registries[0].Name != "only" {
		t.Errorf("config mutated on not-found: %+v", cfg.Registries)
	}
}

func TestRemoveRegistry_CloneFailureIsSoft(t *testing.T) {
	withGlobalDir(t, []config.Registry{
		{Name: "drop", URL: "https://example.com/drop.git"},
	})
	cloneErr := errors.New("permission denied")
	stubRemoveFn(t, func(name string) error { return cloneErr })

	out, err := RemoveRegistry(RemoveOpts{Name: "drop"})
	if err != nil {
		t.Fatalf("RemoveRegistry: %v", err)
	}
	if out.CloneRemoveErr == nil || !errors.Is(out.CloneRemoveErr, cloneErr) {
		t.Errorf("CloneRemoveErr = %v, want wraps %v", out.CloneRemoveErr, cloneErr)
	}

	// Config save still happened — the registry must be gone.
	cfg, _ := config.LoadGlobal()
	if len(cfg.Registries) != 0 {
		t.Errorf("registry still in config after soft-fail clone: %+v", cfg.Registries)
	}
}

// TestRemoveRegistry_RemovesMOATManifestCache verifies that a removed
// MOAT registry leaves no manifest cache subtree behind. Without this,
// EnrichFromMOATManifests would warn about an orphaned cache forever.
func TestRemoveRegistry_RemovesMOATManifestCache(t *testing.T) {
	cacheDir := t.TempDir()
	withGlobalDir(t, []config.Registry{
		{
			Name:        "drop",
			URL:         "https://example.com/drop.git",
			Type:        config.RegistryTypeMOAT,
			ManifestURI: "https://example.com/manifest.json",
		},
	})
	stubRemoveFn(t, func(name string) error { return nil })

	// Seed a cache subtree so we can assert it's gone afterward.
	if err := moat.WriteManifestCache(cacheDir, "drop", []byte("manifest"), []byte("bundle")); err != nil {
		t.Fatalf("seed moat cache: %v", err)
	}

	out, err := RemoveRegistry(RemoveOpts{
		Name:     "drop",
		CacheDir: cacheDir,
	})
	if err != nil {
		t.Fatalf("RemoveRegistry: %v", err)
	}
	if out.ManifestCacheRemoveErr != nil {
		t.Errorf("ManifestCacheRemoveErr = %v, want nil", out.ManifestCacheRemoveErr)
	}

	regCacheDir := filepath.Join(cacheDir, "moat", "registries", "drop")
	if _, err := os.Stat(regCacheDir); !os.IsNotExist(err) {
		t.Errorf("expected MOAT cache subtree gone, got err=%v", err)
	}
}

// TestRemoveRegistry_PrunesLockfilePinState verifies that a registry's
// per-registry pin state in moat-lockfile.json is gone after remove, so a
// later re-add at the same URL doesn't inherit stale freshness.
func TestRemoveRegistry_PrunesLockfilePinState(t *testing.T) {
	cacheDir := t.TempDir()
	projectRoot := t.TempDir()
	manifestURI := "https://example.com/manifest.json"

	withGlobalDir(t, []config.Registry{
		{
			Name:        "drop",
			URL:         "https://example.com/drop.git",
			Type:        config.RegistryTypeMOAT,
			ManifestURI: manifestURI,
		},
	})
	stubRemoveFn(t, func(name string) error { return nil })

	// Seed lockfile with pin state for the registry.
	lf := moat.NewLockfile()
	lf.SetRegistryFetchedAt(manifestURI, time.Now())
	lockfilePath := moat.LockfilePath(projectRoot)
	if err := lf.Save(lockfilePath); err != nil {
		t.Fatalf("seed lockfile: %v", err)
	}

	out, err := RemoveRegistry(RemoveOpts{
		Name:        "drop",
		ProjectRoot: projectRoot,
		CacheDir:    cacheDir,
	})
	if err != nil {
		t.Fatalf("RemoveRegistry: %v", err)
	}
	if out.LockfilePruneErr != nil {
		t.Errorf("LockfilePruneErr = %v, want nil", out.LockfilePruneErr)
	}

	got, err := moat.LoadLockfile(lockfilePath)
	if err != nil {
		t.Fatalf("post-remove load: %v", err)
	}
	if _, ok := got.Registries[manifestURI]; ok {
		t.Errorf("manifestURI still in lockfile.Registries after remove")
	}
}

// TestRemoveRegistry_PreservesLockfileEntriesAndRevokedHashes is the
// load-bearing spec invariant. RevokedHashes is append-only per
// §Revocation Archival (ADR 0007 G-15). Entries[] is the user's
// installed-item ledger, separate from registry pin state. Remove MUST
// touch neither.
func TestRemoveRegistry_PreservesLockfileEntriesAndRevokedHashes(t *testing.T) {
	cacheDir := t.TempDir()
	projectRoot := t.TempDir()
	manifestURI := "https://example.com/manifest.json"

	withGlobalDir(t, []config.Registry{
		{
			Name:        "drop",
			URL:         "https://example.com/drop.git",
			Type:        config.RegistryTypeMOAT,
			ManifestURI: manifestURI,
		},
	})
	stubRemoveFn(t, func(name string) error { return nil })

	lf := moat.NewLockfile()
	lf.SetRegistryFetchedAt(manifestURI, time.Now())
	lf.Entries = append(lf.Entries, moat.LockEntry{
		Name:              "skill-A",
		Type:              "skill",
		Registry:          manifestURI,
		ContentHash:       "sha256:abc",
		TrustTier:         moat.LockTrustTierUnsigned,
		AttestationBundle: moat.NullAttestationBundle(),
	})
	lf.AddRevokedHash("sha256:bad-1")
	lf.AddRevokedHash("sha256:bad-2")

	lockfilePath := moat.LockfilePath(projectRoot)
	if err := lf.Save(lockfilePath); err != nil {
		t.Fatalf("seed lockfile: %v", err)
	}

	if _, err := RemoveRegistry(RemoveOpts{
		Name:        "drop",
		ProjectRoot: projectRoot,
		CacheDir:    cacheDir,
	}); err != nil {
		t.Fatalf("RemoveRegistry: %v", err)
	}

	got, err := moat.LoadLockfile(lockfilePath)
	if err != nil {
		t.Fatalf("post-remove load: %v", err)
	}
	if len(got.Entries) != 1 || got.Entries[0].Name != "skill-A" {
		t.Errorf("entries[] mutated by remove: %+v", got.Entries)
	}
	if len(got.RevokedHashes) != 2 {
		t.Errorf("revoked_hashes mutated (got %d, want 2): %+v", len(got.RevokedHashes), got.RevokedHashes)
	}
	if !got.IsRevoked("sha256:bad-1") || !got.IsRevoked("sha256:bad-2") {
		t.Errorf("revoked hashes lost after remove")
	}
}

// TestRemoveRegistry_NoMOATInputsSkipsCleanup verifies that omitting both
// CacheDir and ProjectRoot keeps backward compatibility — the orchestrator
// still removes from config and clone, no MOAT side effects. Used by
// callers that don't have a project context (or are removing a non-MOAT
// registry where MOAT cleanup would be meaningless).
func TestRemoveRegistry_NoMOATInputsSkipsCleanup(t *testing.T) {
	withGlobalDir(t, []config.Registry{
		{Name: "drop", URL: "https://example.com/drop.git"},
	})
	stubRemoveFn(t, func(name string) error { return nil })

	out, err := RemoveRegistry(RemoveOpts{Name: "drop"})
	if err != nil {
		t.Fatalf("RemoveRegistry: %v", err)
	}
	if out.ManifestCacheRemoveErr != nil {
		t.Errorf("ManifestCacheRemoveErr should be nil when CacheDir is empty, got %v", out.ManifestCacheRemoveErr)
	}
	if out.LockfilePruneErr != nil {
		t.Errorf("LockfilePruneErr should be nil when ProjectRoot is empty, got %v", out.LockfilePruneErr)
	}
}

// TestRemoveRegistry_NonMOATRegistrySkipsLockfile verifies that a registry
// without a ManifestURI (a plain git registry) is removed cleanly without
// the lockfile being touched at all — no Save, no mtime bump.
func TestRemoveRegistry_NonMOATRegistrySkipsLockfile(t *testing.T) {
	cacheDir := t.TempDir()
	projectRoot := t.TempDir()
	withGlobalDir(t, []config.Registry{
		{Name: "drop", URL: "https://example.com/drop.git"}, // no Type, no ManifestURI
	})
	stubRemoveFn(t, func(name string) error { return nil })

	lockfilePath := moat.LockfilePath(projectRoot)
	if _, err := os.Stat(lockfilePath); !os.IsNotExist(err) {
		t.Fatalf("test setup: lockfile should not exist yet, got %v", err)
	}

	if _, err := RemoveRegistry(RemoveOpts{
		Name:        "drop",
		ProjectRoot: projectRoot,
		CacheDir:    cacheDir,
	}); err != nil {
		t.Fatalf("RemoveRegistry: %v", err)
	}

	// Lockfile should NOT have been created — non-MOAT registry has nothing
	// to prune. Creating an empty lockfile would be churn.
	if _, err := os.Stat(lockfilePath); !os.IsNotExist(err) {
		t.Errorf("lockfile created on non-MOAT remove (mtime churn): %v", err)
	}
}
