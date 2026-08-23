package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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

// stubClone swaps the orchestrator's clone seam (registryops.CloneFn) with a
// stub that creates a fake clone dir at registry.CloneDir(name) containing a
// committed registry.yaml with the given content. Restored on t.Cleanup.
func stubClone(t *testing.T, yamlContent string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	orig := registryops.CloneFn
	registryops.CloneFn = func(url, name, ref string) error {
		cloneDir, err := registry.CloneDir(name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(cloneDir, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(cloneDir, "registry.yaml"), []byte(yamlContent), 0644); err != nil {
			return err
		}
		for _, args := range [][]string{
			{"init"},
			{"config", "user.email", "test@example.com"},
			{"config", "user.name", "Test User"},
			{"add", "-A"},
			{"commit", "-m", "initial"},
		} {
			cmd := exec.Command("git", args...)
			cmd.Dir = cloneDir
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("git %v: %w: %s", args, err, strings.TrimSpace(string(out)))
			}
		}
		return nil
	}
	t.Cleanup(func() { registryops.CloneFn = orig })
}

func TestRegistryAutoMOAT_AllowlistURL_SetsManifestURI(t *testing.T) {
	// No t.Parallel — swaps package-level cloneFn and registry.OverrideProbeForTest.
	// registry add for the meta-registry URL must auto-set type=moat + ManifestURI
	// from the bundled allowlist — no clone or --moat flag required. With the
	// local-only default, the allowlist check runs before any network I/O.
	root := withRegistryProjectAndCache(t, nil, &config.Config{})
	output.SetForTest(t)
	// No clone stub needed — SkipClone=true is the default; no network call.
	// No sync stub needed — auto-sync only happens with --sync.

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
	// No t.Parallel — swaps package-level cloneFn.
	// registry add --sync for a non-allowlisted URL that declares manifest_uri in
	// registry.yaml must auto-set type=moat + ManifestURI from that self-declaration.
	// This requires --sync because registry.yaml is only readable after the clone.
	root := withRegistryProjectAndCache(t, nil, &config.Config{})
	output.SetForTest(t)
	overrideProbe(t, func(url string) (string, error) { return "public", nil })

	const testURL = "https://github.com/example/non-allowlisted-registry"
	const wantManifestURI = "https://raw.githubusercontent.com/example/non-allowlisted-registry/moat-registry/registry.json"

	stubClone(t, "name: non-allowlisted-registry\nversion: \"1.0\"\nmanifest_uri: "+wantManifestURI+"\n")
	// Self-declared MOAT without --yes does not chain auto-sync, so no
	// SyncOne stub is needed here.

	registryAddCmd.Flags().Set("sync", "true")
	t.Cleanup(func() { registryAddCmd.Flags().Set("sync", "false") })

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

// TestRegistryAdd_AllowlistSyncFlagChainsAutoSync verifies that adding a
// MOAT registry via allowlist match WITH --sync auto-chains a sync so the
// manifest cache is populated before the next list/scan. Without --sync,
// the registry is registered only (local-only default) and sync is deferred.
func TestRegistryAdd_AllowlistSyncFlagChainsAutoSync(t *testing.T) {
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

	registryAddCmd.Flags().Set("sync", "true")
	t.Cleanup(func() { registryAddCmd.Flags().Set("sync", "false") })

	if err := registryAddCmd.RunE(registryAddCmd, []string{"https://github.com/OpenScribbler/syllago-meta-registry"}); err != nil {
		t.Fatalf("registry add failed: %v", err)
	}
	if !called {
		t.Fatal("expected auto-chained sync after allowlist-pinned MOAT add with --sync, but SyncOneFn was not called")
	}
}

// TestRegistryAdd_DefaultNoSync verifies that the local-only default (no --sync)
// does not attempt any network I/O — SyncOneFn must not be called.
func TestRegistryAdd_DefaultNoSync(t *testing.T) {
	withRegistryProjectAndCache(t, nil, &config.Config{})
	output.SetForTest(t)

	called := false
	orig := registryops.SyncOneFn
	registryops.SyncOneFn = func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		called = true
		return moat.SyncResult{}, nil
	}
	t.Cleanup(func() { registryops.SyncOneFn = orig })

	origClone := registryops.CloneFn
	registryops.CloneFn = func(url, name, ref string) error {
		t.Errorf("CloneFn called during local-only add (URL=%s) — no clone should happen without --sync", url)
		return nil
	}
	t.Cleanup(func() { registryops.CloneFn = origClone })

	if err := registryAddCmd.RunE(registryAddCmd, []string{"https://github.com/OpenScribbler/syllago-meta-registry"}); err != nil {
		t.Fatalf("registry add failed: %v", err)
	}
	if called {
		t.Error("SyncOneFn called during local-only add — no sync should happen without --sync")
	}
}

// TestRegistryAdd_SelfDeclaredSyncWithoutYesSkipsAutoSync verifies that when
// --sync is passed for a self-declared MOAT registry (detected via registry.yaml)
// without --yes, the post-add auto-sync is skipped and a manual hint is shown.
// TOFU consent requires explicit approval via --yes.
func TestRegistryAdd_SelfDeclaredSyncWithoutYesSkipsAutoSync(t *testing.T) {
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

	registryAddCmd.Flags().Set("sync", "true")
	t.Cleanup(func() { registryAddCmd.Flags().Set("sync", "false") })

	if err := registryAddCmd.RunE(registryAddCmd, []string{testURL}); err != nil {
		t.Fatalf("registry add failed: %v", err)
	}
	if called {
		t.Error("self-declared MOAT add with --sync but without --yes must NOT auto-chain sync; SyncOneFn was called")
	}
	if !strings.Contains(stdout.String(), "sync --yes") {
		t.Errorf("expected manual-sync hint in stdout, got:\n%s", stdout.String())
	}
}

// TestRegistrySync_DeferredClonePlainGit verifies the register-only default's
// deferred-clone success path: a plain git registry registered without a clone
// is cloned on its first `registry sync`, stays git (no manifest_uri), and syncs
// cleanly. Regression for the finding that the first sync used pull-only Sync
// (which fails when no clone exists) instead of cloning.
func TestRegistrySync_DeferredClonePlainGit(t *testing.T) {
	// No t.Parallel — swaps package-level CloneFn + probe.
	cfg := &config.Config{
		Providers:  []string{"claude-code"},
		Registries: []config.Registry{{Name: "plain-reg", URL: "https://github.com/example/plain-reg"}},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	output.SetForTest(t)
	overrideProbe(t, func(url string) (string, error) { return "public", nil })
	stubClone(t, "name: plain-reg\nversion: \"1.0\"\n")

	if registry.IsCloned("plain-reg") {
		t.Fatal("precondition: registry should not be cloned before first sync")
	}

	registrySyncCmd.SilenceUsage = true
	registrySyncCmd.SilenceErrors = true
	if err := registrySyncCmd.RunE(registrySyncCmd, []string{"plain-reg"}); err != nil {
		t.Fatalf("deferred-clone sync failed: %v", err)
	}

	if !registry.IsCloned("plain-reg") {
		t.Error("expected registry to be cloned after first sync")
	}
	got, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got.Registries[0].Type == config.RegistryTypeMOAT {
		t.Errorf("plain git registry must not become MOAT, got type %q", got.Registries[0].Type)
	}
}

// TestRegistrySync_DeferredCloneDetectsSelfDeclaredMOAT is the regression for
// the sync-ordering bug: a register-only registry whose registry.yaml self-
// declares manifest_uri must be detected and upgraded to MOAT on the FIRST
// sync. Before the fix, tryUpgradeToMOAT ran before the deferred clone, so the
// self-declaration was invisible until a second sync.
func TestRegistrySync_DeferredCloneDetectsSelfDeclaredMOAT(t *testing.T) {
	// No t.Parallel — swaps package-level CloneFn, SyncOneFn, probe.
	const manifestURI = "https://raw.githubusercontent.com/example/self-decl/moat-registry/registry.json"
	cfg := &config.Config{
		Providers:  []string{"claude-code"},
		Registries: []config.Registry{{Name: "self-decl", URL: "https://github.com/example/self-decl"}},
	}
	withRegistryProjectAndCache(t, nil, cfg)
	output.SetForTest(t)
	overrideProbe(t, func(url string) (string, error) { return "public", nil })
	stubClone(t, "name: self-decl\nversion: \"1.0\"\nmanifest_uri: "+manifestURI+"\n")

	// Stub the post-upgrade MOAT sync so it does no real crypto/network. A
	// NotModified result clears every gate and returns exit 0.
	orig := registryops.SyncOneFn
	registryops.SyncOneFn = func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{NotModified: true, FetchedAt: time.Now().UTC()}, nil
	}
	t.Cleanup(func() { registryops.SyncOneFn = orig })

	registrySyncCmd.SilenceUsage = true
	registrySyncCmd.SilenceErrors = true
	registrySyncCmd.Flags().Set("yes", "true")
	t.Cleanup(func() { registrySyncCmd.Flags().Set("yes", "false") })

	if err := registrySyncCmd.RunE(registrySyncCmd, []string{"self-decl"}); err != nil {
		t.Fatalf("deferred-clone MOAT sync failed: %v", err)
	}

	got, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	r := got.Registries[0]
	if r.Type != config.RegistryTypeMOAT {
		t.Errorf("expected self-declared registry upgraded to MOAT on first sync, got type %q", r.Type)
	}
	if r.ManifestURI != manifestURI {
		t.Errorf("expected ManifestURI %q after first sync, got %q", manifestURI, r.ManifestURI)
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
