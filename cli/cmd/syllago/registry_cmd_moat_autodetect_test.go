package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/OpenScribbler/syllago/cli/internal/registryops"
)

// stubSyncOne replaces the orchestrator's sync seam with a zero-value happy
// path so the chained post-add sync does not try to fetch real sigstore
// artifacts. Required for any test that adds a MOAT registry — both the
// allowlist and self-declaration branches now auto-sync after add (S4).
func stubSyncOne(t *testing.T) {
	t.Helper()
	orig := registryops.SyncOneFn
	registryops.SyncOneFn = func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		// Return enough to satisfy SyncOne's persistence path: a non-nil
		// manifest with a non-empty IncomingProfile so the trust state can
		// be written without crypto verification firing.
		return moat.SyncResult{
			NotModified: true,
			FetchedAt:   time.Now().UTC(),
		}, nil
	}
	t.Cleanup(func() { registryops.SyncOneFn = orig })
}

// stubClone swaps the orchestrator's clone seam (registryops.CloneFn) with a
// stub that creates a fake clone dir at registry.CloneDir(name) containing a
// registry.yaml with the given content. Restored on t.Cleanup.
func stubClone(t *testing.T, yamlContent string) {
	t.Helper()
	orig := registryops.CloneFn
	registryops.CloneFn = func(url, name, ref string) error {
		cloneDir, err := registry.CloneDir(name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(cloneDir, 0755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(cloneDir, "registry.yaml"), []byte(yamlContent), 0644)
	}
	t.Cleanup(func() { registryops.CloneFn = orig })
}

func TestRegistryAutoMOAT_AllowlistURL_SetsManifestURI(t *testing.T) {
	// No t.Parallel — swaps package-level cloneFn and registry.OverrideProbeForTest.
	// registry add for the meta-registry URL must auto-set type=moat + ManifestURI
	// from the bundled allowlist — no --moat flag required.
	root := withRegistryProjectAndCache(t, nil, &config.Config{})
	output.SetForTest(t)
	overrideProbe(t, func(url string) (string, error) { return "public", nil })

	// Fake clone: minimal registry.yaml with no manifest_uri — allowlist provides it.
	stubClone(t, "name: syllago-meta-registry\nversion: \"1.0\"\n")
	stubSyncOne(t)

	err := registryAddCmd.RunE(registryAddCmd, []string{"https://github.com/OpenScribbler/syllago-meta-registry"})
	if err != nil {
		t.Fatalf("registry add failed: %v", err)
	}

	_ = root
	got, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(got.Registries) != 1 {
		t.Fatalf("expected 1 registry, got %d", len(got.Registries))
	}
	r := got.Registries[0]
	if r.Type != config.RegistryTypeMOAT {
		t.Errorf("expected type=moat, got %q", r.Type)
	}
	if r.ManifestURI == "" {
		t.Error("expected non-empty ManifestURI from allowlist auto-detection")
	}
	wantPrefix := "https://raw.githubusercontent.com/OpenScribbler/syllago-meta-registry/"
	if !strings.HasPrefix(r.ManifestURI, wantPrefix) {
		t.Errorf("ManifestURI %q does not start with %q", r.ManifestURI, wantPrefix)
	}
}

func TestRegistryAutoMOAT_RegistryYAML_SetsManifestURI(t *testing.T) {
	// No t.Parallel — swaps package-level cloneFn and registry.OverrideProbeForTest.
	// registry add for a non-allowlisted URL that declares manifest_uri in registry.yaml
	// must auto-set type=moat + ManifestURI from that self-declaration.
	root := withRegistryProjectAndCache(t, nil, &config.Config{})
	output.SetForTest(t)
	overrideProbe(t, func(url string) (string, error) { return "public", nil })

	const testURL = "https://github.com/example/non-allowlisted-registry"
	const wantManifestURI = "https://raw.githubusercontent.com/example/non-allowlisted-registry/moat-registry/registry.json"

	stubClone(t, "name: non-allowlisted-registry\nversion: \"1.0\"\nmanifest_uri: "+wantManifestURI+"\n")
	// Self-declared MOAT without --yes does not chain auto-sync, so no
	// SyncOne stub is needed here.

	err := registryAddCmd.RunE(registryAddCmd, []string{testURL})
	if err != nil {
		t.Fatalf("registry add failed: %v", err)
	}

	_ = root
	got, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(got.Registries) != 1 {
		t.Fatalf("expected 1 registry, got %d", len(got.Registries))
	}
	r := got.Registries[0]
	if r.Type != config.RegistryTypeMOAT {
		t.Errorf("expected type=moat, got %q", r.Type)
	}
	if r.ManifestURI != wantManifestURI {
		t.Errorf("expected ManifestURI %q, got %q", wantManifestURI, r.ManifestURI)
	}
}

// TestRegistryAdd_AllowlistChainsAutoSync is the regression for syllago-43qoo:
// adding a MOAT registry via allowlist match must auto-chain a sync so the
// manifest cache is populated before the next list/scan. Without this, trust
// shows Unknown until the user runs a separate `syllago registry sync`.
func TestRegistryAdd_AllowlistChainsAutoSync(t *testing.T) {
	withRegistryProjectAndCache(t, nil, &config.Config{})
	output.SetForTest(t)
	overrideProbe(t, func(url string) (string, error) { return "public", nil })
	stubClone(t, "name: syllago-meta-registry\nversion: \"1.0\"\n")

	called := false
	orig := registryops.SyncOneFn
	registryops.SyncOneFn = func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		called = true
		return moat.SyncResult{NotModified: true, FetchedAt: time.Now().UTC()}, nil
	}
	t.Cleanup(func() { registryops.SyncOneFn = orig })

	if err := registryAddCmd.RunE(registryAddCmd, []string{"https://github.com/OpenScribbler/syllago-meta-registry"}); err != nil {
		t.Fatalf("registry add failed: %v", err)
	}
	if !called {
		t.Fatal("expected auto-chained sync after allowlist-pinned MOAT add, but SyncOneFn was not called")
	}
}

// TestRegistryAdd_SelfDeclaredWithoutYesSkipsSync is the second leg of
// syllago-43qoo: when a self-declared MOAT registry is added without --yes,
// the orchestrator must NOT auto-chain sync — TOFU consent requires explicit
// approval. The hint to run `sync --yes` appears instead.
func TestRegistryAdd_SelfDeclaredWithoutYesSkipsSync(t *testing.T) {
	withRegistryProjectAndCache(t, nil, &config.Config{})
	stdout, _ := output.SetForTest(t)
	overrideProbe(t, func(url string) (string, error) { return "public", nil })

	const testURL = "https://github.com/example/self-declared-skip-sync"
	const wantManifestURI = "https://raw.githubusercontent.com/example/self-declared-skip-sync/moat-registry/registry.json"
	stubClone(t, "name: self-declared-skip-sync\nversion: \"1.0\"\nmanifest_uri: "+wantManifestURI+"\n")

	called := false
	orig := registryops.SyncOneFn
	registryops.SyncOneFn = func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		called = true
		return moat.SyncResult{}, nil
	}
	t.Cleanup(func() { registryops.SyncOneFn = orig })

	if err := registryAddCmd.RunE(registryAddCmd, []string{testURL}); err != nil {
		t.Fatalf("registry add failed: %v", err)
	}
	if called {
		t.Error("self-declared MOAT add without --yes must NOT auto-chain sync; SyncOneFn was called")
	}
	if !strings.Contains(stdout.String(), "sync --yes") {
		t.Errorf("expected manual-sync hint in stdout, got:\n%s", stdout.String())
	}
}

func TestRegistryList_TrustColumn(t *testing.T) {
	// registry list must show the TRUST column header and "moat" for a synced MOAT registry.
	now := time.Now()
	cfg := &config.Config{
		Registries: []config.Registry{
			{
				Name:          "example/moat-reg",
				URL:           "https://github.com/example/moat-reg",
				Type:          config.RegistryTypeMOAT,
				ManifestURI:   "https://raw.githubusercontent.com/example/moat-reg/moat-registry/registry.json",
				LastFetchedAt: &now,
			},
			{
				Name: "example/plain-reg",
				URL:  "https://github.com/example/plain-reg",
			},
		},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	stdout, _ := output.SetForTest(t)

	registryListCmd.SilenceUsage = true
	registryListCmd.SilenceErrors = true

	if err := registryListCmd.RunE(registryListCmd, nil); err != nil {
		t.Fatalf("registryListCmd.RunE: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, "TRUST") {
		t.Errorf("expected TRUST column header in output, got:\n%s", got)
	}
	if !strings.Contains(got, "moat") {
		t.Errorf("expected 'moat' in TRUST column for MOAT registry, got:\n%s", got)
	}
}
