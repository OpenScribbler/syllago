package main

// Tests for the registry-sync MOAT dispatcher (bead syllago-gj7ad).
//
// Strategy mirrors registry_verify_test.go: swap moatSyncFn for canned
// SyncResult values so each G-18 exit-code branch is exercised without
// standing up a live httptest + bundle pair per test. One end-to-end test
// (TestSyncMOAT_EndToEnd_HappyPath) does spin up a real httptest server to
// prove the dispatcher's wiring through moat.Sync, lockfile persistence, and
// config save.

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// withStubbedMoatSync swaps the package-level moatSyncFn seam. Tests that
// use this helper MUST NOT run in parallel with each other (they mutate a
// shared global). The pattern matches withStubbedVerifiers in
// registry_verify_test.go.
func withStubbedMoatSync(
	t *testing.T,
	fn func(context.Context, *config.Registry, *moat.Lockfile, []byte, *moat.Fetcher, time.Time) (moat.SyncResult, error),
) {
	t.Helper()
	orig := moatSyncFn
	moatSyncFn = fn
	t.Cleanup(func() { moatSyncFn = orig })
}

// tempProjectRoot returns a temp directory that `config.Save` can write
// into (it needs a .syllago subdir path; the package creates it on save).
func tempProjectRoot(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// moatRegFixture returns a MOAT-typed registry suitable for most dispatcher
// tests. Copy-then-mutate for tests that want a pinned profile.
func moatRegFixture(url string) config.Registry {
	return config.Registry{
		Name:        "example",
		URL:         url,
		Type:        config.RegistryTypeMOAT,
		ManifestURI: url,
	}
}

// incomingProfile is the profile canned into every stubbed SyncResult.
func incomingProfile() config.SigningProfile {
	return config.SigningProfile{
		Issuer:            "https://token.actions.githubusercontent.com",
		Subject:           "repo:example/registry:ref:refs/heads/main",
		ProfileVersion:    1,
		RepositoryID:      "100",
		RepositoryOwnerID: "200",
	}
}

func TestSyncMOAT_HappyPath_PinnedProfile(t *testing.T) {
	// No t.Parallel — swaps package-level moatSyncFn.
	root := tempProjectRoot(t)
	pinned := incomingProfile()
	reg := moatRegFixture("https://registry.example.com/manifest.json")
	reg.SigningProfile = &pinned
	cfg := &config.Config{Registries: []config.Registry{reg}}

	fetchedAt := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	withStubbedMoatSync(t, func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL:      "https://registry.example.com/manifest.json",
			BundleURL:        "https://registry.example.com/manifest.json.sigstore",
			ETag:             `"v42"`,
			FetchedAt:        fetchedAt,
			IncomingProfile:  incomingProfile(),
			Staleness:        moat.StalenessFresh,
			RevocationsAdded: 3,
			Manifest:         &moat.Manifest{},
		}, nil
	})

	var out, errW bytes.Buffer
	code, err := syncMOATRegistry(context.Background(), &out, &errW, cfg, &cfg.Registries[0], root, "", time.Now(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d; want 0", code)
	}
	if !strings.Contains(out.String(), "verified") {
		t.Errorf("expected 'verified' in stdout, got %q", out.String())
	}
	if cfg.Registries[0].ManifestETag != `"v42"` {
		t.Errorf("ManifestETag = %q; want %q", cfg.Registries[0].ManifestETag, `"v42"`)
	}
	if cfg.Registries[0].LastFetchedAt == nil || !cfg.Registries[0].LastFetchedAt.Equal(fetchedAt) {
		t.Errorf("LastFetchedAt not persisted to %s", fetchedAt)
	}
	// Config file should exist on disk.
	if _, err := os.Stat(filepath.Join(root, ".syllago", "config.json")); err != nil {
		t.Errorf("config not saved: %v", err)
	}
	// Lockfile should exist on disk.
	if _, err := os.Stat(filepath.Join(root, ".syllago", "moat-lockfile.json")); err != nil {
		t.Errorf("lockfile not saved: %v", err)
	}
}

func TestSyncMOAT_NotModified(t *testing.T) {
	root := tempProjectRoot(t)
	pinned := incomingProfile()
	reg := moatRegFixture("https://registry.example.com/manifest.json")
	reg.SigningProfile = &pinned
	reg.ManifestETag = `"v42"`
	cfg := &config.Config{Registries: []config.Registry{reg}}

	fetchedAt := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	withStubbedMoatSync(t, func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL: "https://registry.example.com/manifest.json",
			NotModified: true,
			ETag:        `"v42"`,
			FetchedAt:   fetchedAt,
			Staleness:   moat.StalenessFresh,
		}, nil
	})

	var out, errW bytes.Buffer
	code, err := syncMOATRegistry(context.Background(), &out, &errW, cfg, &cfg.Registries[0], root, "", time.Now(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d; want 0", code)
	}
	if !strings.Contains(out.String(), "not-modified") {
		t.Errorf("expected 'not-modified' in stdout, got %q", out.String())
	}
}

func TestSyncMOAT_TOFU_WithoutYes_ReturnsExit10(t *testing.T) {
	root := tempProjectRoot(t)
	reg := moatRegFixture("https://registry.example.com/manifest.json")
	cfg := &config.Config{Registries: []config.Registry{reg}}

	withStubbedMoatSync(t, func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{
			IsTOFU:          true,
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
			Manifest:        &moat.Manifest{},
		}, nil
	})

	var out, errW bytes.Buffer
	code, err := syncMOATRegistry(context.Background(), &out, &errW, cfg, &cfg.Registries[0], root, "", time.Now(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != moat.ExitMoatTOFUAcceptance {
		t.Fatalf("exit code = %d; want %d", code, moat.ExitMoatTOFUAcceptance)
	}
	if cfg.Registries[0].SigningProfile != nil {
		t.Errorf("SigningProfile should NOT be persisted on TOFU without --yes; got %+v", cfg.Registries[0].SigningProfile)
	}
	// Config file should NOT be written on gated path.
	if _, err := os.Stat(filepath.Join(root, ".syllago", "config.json")); !os.IsNotExist(err) {
		t.Errorf("config was saved on gated path; expected no save")
	}
	if !strings.Contains(errW.String(), "interactive approval") {
		t.Errorf("expected TOFU actionable message on stderr, got %q", errW.String())
	}
}

func TestSyncMOAT_TOFU_WithYes_PersistsProfile(t *testing.T) {
	root := tempProjectRoot(t)
	reg := moatRegFixture("https://registry.example.com/manifest.json")
	cfg := &config.Config{Registries: []config.Registry{reg}}

	fetchedAt := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	withStubbedMoatSync(t, func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{
			ETag:            `"v42"`,
			FetchedAt:       fetchedAt,
			IsTOFU:          true,
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
			Manifest:        &moat.Manifest{},
		}, nil
	})

	var out, errW bytes.Buffer
	code, err := syncMOATRegistry(context.Background(), &out, &errW, cfg, &cfg.Registries[0], root, "", time.Now(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit code = %d; want 0", code)
	}
	if cfg.Registries[0].SigningProfile == nil {
		t.Fatal("SigningProfile not persisted after TOFU+yes")
	}
	if cfg.Registries[0].SigningProfile.Subject != incomingProfile().Subject {
		t.Errorf("SigningProfile.Subject = %q; want %q",
			cfg.Registries[0].SigningProfile.Subject, incomingProfile().Subject)
	}
	if !strings.Contains(out.String(), "tofu-accepted") {
		t.Errorf("expected 'tofu-accepted' in stdout, got %q", out.String())
	}
}

func TestSyncMOAT_ProfileChanged_ReturnsExit11(t *testing.T) {
	root := tempProjectRoot(t)
	pinned := incomingProfile()
	pinned.Subject = "repo:example/registry:ref:refs/heads/old"
	reg := moatRegFixture("https://registry.example.com/manifest.json")
	reg.SigningProfile = &pinned
	cfg := &config.Config{Registries: []config.Registry{reg}}

	withStubbedMoatSync(t, func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{
			ProfileChanged:  true,
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
			Manifest:        &moat.Manifest{},
		}, nil
	})

	var out, errW bytes.Buffer
	// ProfileChanged MUST gate even with --yes; re-approval requires
	// removing and re-adding the registry interactively.
	code, err := syncMOATRegistry(context.Background(), &out, &errW, cfg, &cfg.Registries[0], root, "", time.Now(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != moat.ExitMoatSigningProfileChange {
		t.Fatalf("exit code = %d; want %d", code, moat.ExitMoatSigningProfileChange)
	}
	if cfg.Registries[0].SigningProfile.Subject != "repo:example/registry:ref:refs/heads/old" {
		t.Errorf("SigningProfile should not be mutated on gated path; got %q",
			cfg.Registries[0].SigningProfile.Subject)
	}
	if !strings.Contains(errW.String(), "re-approve") {
		t.Errorf("expected re-approve hint on stderr, got %q", errW.String())
	}
}

func TestSyncMOAT_StalenessExpired_ReturnsExit13(t *testing.T) {
	root := tempProjectRoot(t)
	pinned := incomingProfile()
	reg := moatRegFixture("https://registry.example.com/manifest.json")
	reg.SigningProfile = &pinned
	cfg := &config.Config{Registries: []config.Registry{reg}}

	withStubbedMoatSync(t, func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessExpired,
			Manifest:        &moat.Manifest{},
		}, nil
	})

	var out, errW bytes.Buffer
	code, err := syncMOATRegistry(context.Background(), &out, &errW, cfg, &cfg.Registries[0], root, "", time.Now(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != moat.ExitMoatManifestStale {
		t.Fatalf("exit code = %d; want %d", code, moat.ExitMoatManifestStale)
	}
	if !strings.Contains(errW.String(), "stale") {
		t.Errorf("expected 'stale' in stderr, got %q", errW.String())
	}
}

func TestSyncMOAT_VerifyError_ReturnsStructuredError(t *testing.T) {
	root := tempProjectRoot(t)
	pinned := incomingProfile()
	reg := moatRegFixture("https://registry.example.com/manifest.json")
	reg.SigningProfile = &pinned
	cfg := &config.Config{Registries: []config.Registry{reg}}

	withStubbedMoatSync(t, func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{}, &moat.VerifyError{
			Code:    moat.CodeIdentityMismatch,
			Message: "subject mismatch",
		}
	})

	var out, errW bytes.Buffer
	_, err := syncMOATRegistry(context.Background(), &out, &errW, cfg, &cfg.Registries[0], root, "", time.Now(), false)
	if err == nil {
		t.Fatal("expected error for verify failure, got nil")
	}
	// Should wrap to the MOAT_003 identity-mismatch code via classifyVerifyError.
	if !strings.Contains(err.Error(), "MOAT_003") {
		t.Errorf("expected MOAT_003 in error, got %q", err.Error())
	}
}

func TestSyncMOAT_TransportError_ReturnsStructuredError(t *testing.T) {
	root := tempProjectRoot(t)
	pinned := incomingProfile()
	reg := moatRegFixture("https://registry.example.com/manifest.json")
	reg.SigningProfile = &pinned
	cfg := &config.Config{Registries: []config.Registry{reg}}

	withStubbedMoatSync(t, func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{}, errors.New("dial tcp: connection refused")
	})

	var out, errW bytes.Buffer
	_, err := syncMOATRegistry(context.Background(), &out, &errW, cfg, &cfg.Registries[0], root, "", time.Now(), false)
	if err == nil {
		t.Fatal("expected error for transport failure, got nil")
	}
	if !strings.Contains(err.Error(), "MOAT_004") {
		t.Errorf("expected MOAT_004 wrap for transport error, got %q", err.Error())
	}
}

func TestSyncMOAT_TrustedRootExpired_ReturnsStructuredError(t *testing.T) {
	t.Parallel()
	pinned := incomingProfile()
	reg := moatRegFixture("https://registry.example.com/manifest.json")
	reg.SigningProfile = &pinned
	cfg := &config.Config{Registries: []config.Registry{reg}}

	// Far future now — past the 365-day cliff from the bundled issued-at
	// constant. Exercises the pre-flight trusted-root staleness gate.
	farFuture := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := syncMOATRegistry(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, cfg, &cfg.Registries[0], t.TempDir(), "", farFuture, false)
	if err == nil {
		t.Fatal("expected error for expired trusted root")
	}
	if !strings.Contains(err.Error(), "MOAT_005") {
		t.Errorf("expected MOAT_005 wrap, got %q", err.Error())
	}
}

func TestFindRegistryByName(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Registries: []config.Registry{
			{Name: "alpha"}, {Name: "beta"},
		},
	}
	if got := findRegistryByName(cfg, "alpha"); got == nil || got.Name != "alpha" {
		t.Errorf("findRegistryByName(alpha) = %+v; want alpha", got)
	}
	if got := findRegistryByName(cfg, "missing"); got != nil {
		t.Errorf("findRegistryByName(missing) = %+v; want nil", got)
	}
	// Returned pointer must be into the slice so mutations persist.
	got := findRegistryByName(cfg, "beta")
	got.ManifestETag = `"mutated"`
	if cfg.Registries[1].ManifestETag != `"mutated"` {
		t.Errorf("findRegistryByName should return pointer into cfg.Registries")
	}
}

func TestSyncMOAT_NilRegistry(t *testing.T) {
	t.Parallel()
	_, err := syncMOATRegistry(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, &config.Config{}, nil, t.TempDir(), "", time.Now(), false)
	if err == nil || !strings.Contains(err.Error(), "registry is nil") {
		t.Fatalf("expected nil-registry error, got %v", err)
	}
}

func TestSyncMOAT_NonMOATRegistry(t *testing.T) {
	t.Parallel()
	reg := &config.Registry{Name: "git-only", Type: config.RegistryTypeGit, URL: "https://example.com/repo.git"}
	cfg := &config.Config{Registries: []config.Registry{*reg}}
	_, err := syncMOATRegistry(context.Background(), &bytes.Buffer{}, &bytes.Buffer{}, cfg, reg, t.TempDir(), "", time.Now(), false)
	if err == nil || !strings.Contains(err.Error(), "not MOAT") {
		t.Fatalf("expected not-MOAT error, got %v", err)
	}
}

// TestSyncMOAT_AllRegistriesOnDisk_PersistedAcrossCalls verifies that the
// dispatcher's persistence side-effects survive a fresh config load — i.e.,
// the next invocation of `syllago registry sync` sees the ETag from the
// previous sync so it can send If-None-Match. This is the load-bearing
// guarantee that keeps the 72h staleness clock moving without re-verifying
// unchanged manifests.
func TestSyncMOAT_PersistenceRoundTrip(t *testing.T) {
	root := tempProjectRoot(t)
	pinned := incomingProfile()
	reg := moatRegFixture("https://registry.example.com/manifest.json")
	reg.SigningProfile = &pinned
	cfg := &config.Config{Registries: []config.Registry{reg}}

	fetchedAt := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	withStubbedMoatSync(t, func(_ context.Context, _ *config.Registry, lf *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		// Emulate what moat.Sync does on success: advance the lockfile.
		lf.SetRegistryFetchedAt("https://registry.example.com/manifest.json", fetchedAt)
		return moat.SyncResult{
			ManifestURL:     "https://registry.example.com/manifest.json",
			ETag:            `"v42"`,
			FetchedAt:       fetchedAt,
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
			Manifest:        &moat.Manifest{},
		}, nil
	})

	var out, errW bytes.Buffer
	code, err := syncMOATRegistry(context.Background(), &out, &errW, cfg, &cfg.Registries[0], root, "", time.Now(), false)
	if err != nil || code != 0 {
		t.Fatalf("first sync failed: code=%d err=%v", code, err)
	}

	// Reload config from disk and verify ETag round-tripped.
	reloaded, err := config.Load(root)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if len(reloaded.Registries) != 1 || reloaded.Registries[0].ManifestETag != `"v42"` {
		t.Errorf("reloaded ManifestETag = %q; want %q", reloaded.Registries[0].ManifestETag, `"v42"`)
	}

	// Reload lockfile and verify fetched_at round-tripped.
	lfPath := moat.LockfilePath(root)
	lf, err := moat.LoadLockfile(lfPath)
	if err != nil {
		t.Fatalf("reload lockfile: %v", err)
	}
	raw, _ := os.ReadFile(lfPath)
	if !strings.Contains(string(raw), "fetched_at") {
		t.Errorf("lockfile missing fetched_at on disk: %s", raw)
	}
	_ = lf
}
