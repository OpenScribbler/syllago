package main

// End-to-end integration tests for `syllago install <registry>/<item>`
// (bead syllago-bk2vg — third slice of parent bead syllago-svdwc).
//
// These tests exercise the full runInstallFromRegistry pipeline:
//
//   moatSync (stubbed) -> gate evaluation -> fetchAndRecord -> lockfile
//
// The stub at moatSyncFn lets us inject a verified SyncResult without
// standing up real sigstore + Rekor fixtures here — those are covered
// in cli/internal/moat's unit tests. The integration surface we actually
// want to exercise end-to-end is the CLI dispatch + gate + fetch +
// lockfile persistence, which runs against a real http server for the
// source artifact and a real project-root lockfile.
//
// Scenarios map to the bk2vg bead acceptance criteria:
//
//   1. Clean UNSIGNED install proceeds and appends a LockEntry honoring the
//      manifest's trust tier.
//   2. Registry-source revocation refuses install with a structured
//      MOAT_008 error (nonzero exit).
//   3. Publisher-warn under headless mode exits 12.
//   4. Private-content under headless mode exits 10.
//   5. Tier-below-policy returns a structured MOAT_009 error.
//
// Scenario 4 (publisher-warn interactive Y proceeds) is covered at the
// unit level in install_moat_test.go; duplicating the stdin scenario at
// the integration tier adds no coverage of the integration seams.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/moatinstall"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// integrationTestProvider is a minimal provider stub for the integration
// suite. Skills install under <home>/.testprovider/skills/<name> so tests
// using t.Setenv("HOME", ...) can assert against a deterministic path.
func integrationTestProvider() provider.Provider {
	return provider.Provider{
		Name:      "Test Provider",
		Slug:      "testprovider",
		ConfigDir: ".testprovider",
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			switch ct {
			case catalog.Skills:
				return filepath.Join(homeDir, ".testprovider", "skills")
			case catalog.Agents:
				return filepath.Join(homeDir, ".testprovider", "agents")
			case catalog.Rules:
				return filepath.Join(homeDir, ".testprovider", "rules")
			case catalog.Commands:
				return filepath.Join(homeDir, ".testprovider", "commands")
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			switch ct {
			case catalog.Skills, catalog.Agents, catalog.Rules, catalog.Commands:
				return true
			}
			return false
		},
	}
}

// integrationEnv wires the integration-test seams and cleans up on
// t.Cleanup. It returns the project root and a helper that reads the
// persisted lockfile for post-condition checks.
type integrationEnv struct {
	projectRoot  string
	cacheRoot    string
	syncResultFn func() (moat.SyncResult, error)
}

func setupIntegrationEnv(t *testing.T) *integrationEnv {
	t.Helper()
	env := &integrationEnv{
		projectRoot: t.TempDir(),
		cacheRoot:   t.TempDir(),
	}

	t.Cleanup(withTLSClient(t))

	origCache := moatinstall.SourceCacheDir
	moatinstall.SourceCacheDir = func() (string, error) { return env.cacheRoot, nil }
	t.Cleanup(func() { moatinstall.SourceCacheDir = origCache })

	// Fail-closed default: if a test doesn't set syncResultFn, force a
	// loud error so stub drift is caught.
	origSync := moatSyncFn
	moatSyncFn = func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		if env.syncResultFn == nil {
			return moat.SyncResult{}, errors.New("integration test did not wire syncResultFn")
		}
		return env.syncResultFn()
	}
	t.Cleanup(func() { moatSyncFn = origSync })

	return env
}

func TestInstallIntegration_CleanUnsignedSucceeds(t *testing.T) {
	env := setupIntegrationEnv(t)

	// Pin HOME to projectRoot so the test provider's InstallDir resolves
	// under the temp tree — keeps the symlink target predictable and lets
	// t.TempDir's auto-cleanup take care of artifacts.
	t.Setenv("HOME", env.projectRoot)

	// Serve a real tarball over TLS so the whole fetch+extract pipeline
	// runs, not just the stubbed SyncResult.
	body := buildTarGz(t, map[string]string{"SKILL.md": "# from registry\n"})
	sum := sha256.Sum256(body)
	contentHash := "sha256:" + hex.EncodeToString(sum[:])
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	env.syncResultFn = func() (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL: "https://example.com/manifest.json",
			Manifest: &moat.Manifest{Content: []moat.ContentEntry{{
				Name:        "my-skill",
				Type:        "skill",
				ContentHash: contentHash,
				SourceURI:   srv.URL,
				AttestedAt:  time.Now().UTC(),
			}}},
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
		}, nil
	}

	out := &bytes.Buffer{}
	cfg := cfgWithPinnedMOATRegistry()
	prov := integrationTestProvider()
	err := runInstallFromRegistry(
		context.Background(),
		out,
		&bytes.Buffer{},
		cfg,
		env.projectRoot,
		"example",
		"my-skill",
		&prov,
		installer.MethodSymlink,
		"",
		false,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("clean install returned error: %v", err)
	}
	if !strings.Contains(out.String(), "installed example/my-skill (UNSIGNED)") {
		t.Errorf("success message missing; got %q", out.String())
	}

	// Lockfile: exactly one entry with the fetched content_hash.
	lfPath := moat.LockfilePath(env.projectRoot)
	lf, err := moat.LoadLockfile(lfPath)
	if err != nil {
		t.Fatalf("LoadLockfile: %v", err)
	}
	if len(lf.Entries) != 1 {
		t.Fatalf("lockfile should have 1 entry, got %d", len(lf.Entries))
	}
	if lf.Entries[0].ContentHash != contentHash {
		t.Errorf("lockfile hash = %q, want %q", lf.Entries[0].ContentHash, contentHash)
	}
	if lf.Entries[0].TrustTier != "UNSIGNED" {
		t.Errorf("lockfile trust_tier = %q, want UNSIGNED", lf.Entries[0].TrustTier)
	}

	// Cache: the tarball's SKILL.md should be materialized under the
	// per-item cache dir.
	matches, _ := filepath.Glob(filepath.Join(env.cacheRoot, "example", "my-skill", "*", "SKILL.md"))
	if len(matches) != 1 {
		t.Errorf("expected exactly 1 SKILL.md in cache, got %d (root=%s)", len(matches), env.cacheRoot)
	}

	// Provider-side install: the symlink should land at the provider's
	// skills dir under the pinned HOME.
	want := filepath.Join(env.projectRoot, ".testprovider", "skills", "my-skill")
	info, err := os.Lstat(want)
	if err != nil {
		t.Fatalf("Lstat provider install path %q: %v", want, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected symlink at %q, got mode %v", want, info.Mode())
	}
}

func TestInstallIntegration_RegistryRevocationRefuses(t *testing.T) {
	env := setupIntegrationEnv(t)

	contentHash := "sha256:" + strings.Repeat("ab", 32)
	env.syncResultFn = func() (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL: "https://example.com/manifest.json",
			Manifest: &moat.Manifest{
				Content: []moat.ContentEntry{{
					Name: "my-skill", Type: "skill", ContentHash: contentHash,
					SourceURI: "https://example.com/src.tar.gz",
				}},
				Revocations: []moat.Revocation{{
					ContentHash: contentHash,
					Source:      "registry",
					Reason:      "cve-2026-9999",
				}},
			},
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
		}, nil
	}

	cfg := cfgWithPinnedMOATRegistry()
	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfg,
		env.projectRoot,
		"example",
		"my-skill",
		nil,
		installer.MethodSymlink,
		"",
		false,
		time.Now(),
	)
	assertStructuredCode(t, err, output.ErrMoatRevocationBlock)

	// Lockfile must not have gained an entry.
	lfPath := moat.LockfilePath(env.projectRoot)
	if _, err := os.Stat(lfPath); err == nil {
		lf, _ := moat.LoadLockfile(lfPath)
		if len(lf.Entries) != 0 {
			t.Errorf("lockfile must not be mutated on revocation; got %d entries", len(lf.Entries))
		}
	}
}

func TestInstallIntegration_PublisherWarnHeadlessExits12(t *testing.T) {
	env := setupIntegrationEnv(t)
	exitCode := withInstallGateStubs(t, false, false)

	contentHash := "sha256:" + strings.Repeat("ab", 32)
	env.syncResultFn = func() (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL: "https://example.com/manifest.json",
			Manifest: &moat.Manifest{
				Content: []moat.ContentEntry{{
					Name: "my-skill", Type: "skill", ContentHash: contentHash,
					SourceURI: "https://example.com/src.tar.gz",
				}},
				Revocations: []moat.Revocation{{
					ContentHash: contentHash,
					Source:      "publisher",
					Reason:      "publisher-flagged",
				}},
			},
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
		}, nil
	}

	cfg := cfgWithPinnedMOATRegistry()
	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfg,
		env.projectRoot,
		"example",
		"my-skill",
		nil,
		installer.MethodSymlink,
		"",
		false,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("publisher-warn headless returned unexpected error: %v", err)
	}
	if *exitCode != moat.ExitMoatPublisherRevocation {
		t.Errorf("exit code = %d, want %d", *exitCode, moat.ExitMoatPublisherRevocation)
	}
}

func TestInstallIntegration_PrivatePromptHeadlessExits10(t *testing.T) {
	env := setupIntegrationEnv(t)
	exitCode := withInstallGateStubs(t, false, false)

	env.syncResultFn = func() (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL: "https://example.com/manifest.json",
			Manifest: &moat.Manifest{
				Content: []moat.ContentEntry{{
					Name: "my-skill", Type: "skill",
					ContentHash: "sha256:" + strings.Repeat("ab", 32),
					SourceURI:   "https://example.com/src.tar.gz",
					PrivateRepo: true,
				}},
			},
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
		}, nil
	}

	cfg := cfgWithPinnedMOATRegistry()
	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfg,
		env.projectRoot,
		"example",
		"my-skill",
		nil,
		installer.MethodSymlink,
		"",
		false,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("private-prompt headless returned unexpected error: %v", err)
	}
	if *exitCode != moat.ExitMoatTOFUAcceptance {
		t.Errorf("exit code = %d, want %d", *exitCode, moat.ExitMoatTOFUAcceptance)
	}
}

func TestInstallIntegration_TierBelowPolicyRefuses(t *testing.T) {
	env := setupIntegrationEnv(t)

	// Force the policy floor to DUAL-ATTESTED so an UNSIGNED entry is
	// rejected regardless of gate cleanliness.
	origTier := moatInstallMinTier
	moatInstallMinTier = moat.TrustTierDualAttested
	t.Cleanup(func() { moatInstallMinTier = origTier })

	env.syncResultFn = func() (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL: "https://example.com/manifest.json",
			Manifest: &moat.Manifest{Content: []moat.ContentEntry{{
				Name: "my-skill", Type: "skill",
				ContentHash: "sha256:" + strings.Repeat("ab", 32),
				SourceURI:   "https://example.com/src.tar.gz",
			}}},
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
		}, nil
	}

	cfg := cfgWithPinnedMOATRegistry()
	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfg,
		env.projectRoot,
		"example",
		"my-skill",
		nil,
		installer.MethodSymlink,
		"",
		false,
		time.Now(),
	)
	assertStructuredCode(t, err, output.ErrMoatTierBelowPolicy)
}
