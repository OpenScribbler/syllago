package main

// Regression tests for the MOAT lockfile root (spec §Lockfile).
//
// The lockfile is project-scoped state pinned at
// <project root>/.syllago/moat-lockfile.json (moat.LockfileRelPath). Doctor
// reads it from the bare project root (doctor.CheckTrust →
// moat.LoadAndScan(projectRoot, projectRoot, now)). The sync command used to
// derive its lockfile root from findContentRepoRoot(), which appends the
// project config's content_root — so with `"content_root": "content"` the
// lockfile landed at <root>/content/.syllago/moat-lockfile.json and doctor's
// "manifest cache stale" warning never cleared after a successful sync.
//
// Strategy mirrors registry_sync_moat_test.go: stub registryops.SyncOneFn
// (the orchestrator's moat.Sync seam) with a canned fresh result, but drive
// the real `syllago registry sync` RunE so the root plumbed into
// SyncOpts.LockfileRoot is the one under test.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func TestRegistrySyncCmd_LockfileAtProjectRoot_NotContentRoot(t *testing.T) {
	// No t.Parallel — mutates SyncOneFn, GlobalDirOverride, findProjectRoot,
	// and output writers.
	root := tempProjectRoot(t)
	withFakeRepoRoot(t, root)
	output.SetForTest(t)

	// Project config sets content_root, so findContentRepoRoot() resolves to
	// <root>/content. The lockfile must still land at the bare project root.
	if err := os.MkdirAll(filepath.Join(root, "content"), 0755); err != nil {
		t.Fatalf("mkdir content root: %v", err)
	}

	pinned := incomingProfile()
	reg := moatRegFixture("https://registry.example.com/manifest.json")
	reg.SigningProfile = &pinned
	cfg := &config.Config{
		ContentRoot: "content",
		Registries:  []config.Registry{reg},
	}
	if err := config.SaveGlobal(cfg); err != nil {
		t.Fatalf("save initial cfg: %v", err)
	}

	fetchedAt := time.Now().UTC().Truncate(time.Second)
	withStubbedMoatSync(t, func(_ context.Context, r *config.Registry, lf *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		// Mirror moat.Sync's documented side effect: record the successful
		// fetch in the lockfile so staleness classification has a clock.
		lf.Registries[r.ManifestURI] = moat.RegistryLockState{FetchedAt: fetchedAt}
		return moat.SyncResult{
			ManifestURL:     r.ManifestURI,
			ETag:            `"v1"`,
			FetchedAt:       fetchedAt,
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
			Manifest:        &moat.Manifest{},
		}, nil
	})

	registrySyncCmd.SetContext(context.Background())
	if err := registrySyncCmd.RunE(registrySyncCmd, []string{"example"}); err != nil {
		t.Fatalf("registry sync: %v", err)
	}

	// Write side: the lockfile must be at the spec location...
	wantPath := filepath.Join(root, ".syllago", "moat-lockfile.json")
	if _, err := os.Stat(wantPath); err != nil {
		t.Errorf("lockfile not written at project root (%s): %v", wantPath, err)
	}
	// ...and NOT relocated under content_root, where doctor never looks.
	strayPath := filepath.Join(root, "content", ".syllago", "moat-lockfile.json")
	if _, err := os.Stat(strayPath); err == nil {
		t.Errorf("lockfile written under content_root (%s); doctor reads the project root and would report stale forever", strayPath)
	}

	// Read side: doctor.CheckTrust loads the lockfile via
	// moat.LockfilePath(projectRoot) and classifies with moat.CheckRegistry.
	// Same path + same call here — the just-synced registry must be Fresh.
	lf, err := moat.LoadLockfile(moat.LockfilePath(root))
	if err != nil {
		t.Fatalf("load lockfile from project root: %v", err)
	}
	if got := moat.CheckRegistry(lf, reg.ManifestURI, nil, fetchedAt.Add(time.Hour)); got != moat.StalenessFresh {
		t.Errorf("doctor-side staleness = %s; want Fresh", got)
	}
}
