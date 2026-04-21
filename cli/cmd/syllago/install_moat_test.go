package main

// Tests for the MOAT registry-sourced install dispatcher (bead
// syllago-elvv3). Strategy mirrors registry_sync_moat_test.go: stub
// moatSyncFn with canned SyncResult values for each gate branch, and
// capture moatSyncExit through a package-level seam swap so os.Exit does
// not terminate the test process.
//
// This file covers the first svdwc slice — parse, sync, lookup, and
// --dry-run summary. The non-dry-run path is asserted to return a
// transitional structured error until syllago-raivj lands.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// --- parseRegistryItemSyntax --------------------------------------------

func TestParseRegistryItemSyntax_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		wantReg string
		wantIt  string
		wantOK  bool
	}{
		{"my-registry/my-item", "my-registry", "my-item", true},
		{"a/b", "a", "b", true},
		{"plain-item", "", "", false},
		{"", "", "", false},
		{"/leading", "", "", false},
		{"trailing/", "", "", false},
		{"a/b/c", "", "", false},
		{"//", "", "", false},
	}
	for _, c := range cases {
		gotReg, gotIt, gotOK := parseRegistryItemSyntax(c.in)
		if gotReg != c.wantReg || gotIt != c.wantIt || gotOK != c.wantOK {
			t.Errorf("parseRegistryItemSyntax(%q) = (%q, %q, %v); want (%q, %q, %v)",
				c.in, gotReg, gotIt, gotOK, c.wantReg, c.wantIt, c.wantOK)
		}
	}
}

// --- shortHash ----------------------------------------------------------

func TestShortHash_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"sha256:abcdef0123456789abcdef", "sha256:abcdef012345…"},
		{"short", "short"},                                // no colon — returned verbatim
		{"sha256:abc", "sha256:abc"},                      // malformed (too short after colon) — verbatim
		{"sha256:12345678901234", "sha256:123456789012…"}, // exactly on the boundary
	}
	for _, c := range cases {
		if got := shortHash(c.in); got != c.want {
			t.Errorf("shortHash(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

// --- runInstallFromRegistry: routing errors ----------------------------

func TestRunInstallFromRegistry_RegistryNotFound(t *testing.T) {
	// No t.Parallel — may mutate moatSyncFn if downstream logic ever calls it.
	cfg := &config.Config{Registries: []config.Registry{}}
	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfg,
		t.TempDir(),
		"ghost",
		"item",
		true,
		time.Now(),
	)
	assertStructuredCode(t, err, output.ErrRegistryNotFound)
}

func TestRunInstallFromRegistry_NotMOATRegistry(t *testing.T) {
	cfg := &config.Config{Registries: []config.Registry{{
		Name: "git-only",
		URL:  "https://example.com/repo.git",
		Type: config.RegistryTypeGit,
	}}}
	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfg,
		t.TempDir(),
		"git-only",
		"item",
		true,
		time.Now(),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
}

// --- runInstallFromRegistry: G-18 exit paths ---------------------------

func TestRunInstallFromRegistry_SyncTOFUExits10(t *testing.T) {
	code := runInstallWithStubbedSyncResult(t, moat.SyncResult{
		ManifestURL: "https://example.com/m",
		IsTOFU:      true,
		Manifest:    &moat.Manifest{},
	}, nil)
	if code != moat.ExitMoatTOFUAcceptance {
		t.Errorf("TOFU exit = %d; want %d", code, moat.ExitMoatTOFUAcceptance)
	}
}

func TestRunInstallFromRegistry_SyncProfileChangedExits11(t *testing.T) {
	code := runInstallWithStubbedSyncResult(t, moat.SyncResult{
		ManifestURL:    "https://example.com/m",
		ProfileChanged: true,
		Manifest:       &moat.Manifest{},
	}, nil)
	if code != moat.ExitMoatSigningProfileChange {
		t.Errorf("ProfileChanged exit = %d; want %d", code, moat.ExitMoatSigningProfileChange)
	}
}

func TestRunInstallFromRegistry_SyncStaleExits13(t *testing.T) {
	code := runInstallWithStubbedSyncResult(t, moat.SyncResult{
		ManifestURL: "https://example.com/m",
		Staleness:   moat.StalenessExpired,
		Manifest:    &moat.Manifest{Content: []moat.ContentEntry{}},
	}, nil)
	if code != moat.ExitMoatManifestStale {
		t.Errorf("Stale exit = %d; want %d", code, moat.ExitMoatManifestStale)
	}
}

// --- runInstallFromRegistry: verify error surfaces as structured --------

func TestRunInstallFromRegistry_VerifyErrorMapsToStructured(t *testing.T) {
	orig := moatSyncFn
	moatSyncFn = func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{}, &moat.VerifyError{Code: moat.CodeIdentityMismatch, Message: "cert subject mismatch"}
	}
	t.Cleanup(func() { moatSyncFn = orig })

	cfg := cfgWithPinnedMOATRegistry()
	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfg,
		t.TempDir(),
		"example",
		"item",
		true,
		time.Now(),
	)
	assertStructuredCode(t, err, output.ErrMoatIdentityMismatch)
}

func TestRunInstallFromRegistry_TransportErrorMapsToMoatInvalid(t *testing.T) {
	orig := moatSyncFn
	moatSyncFn = func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{}, errors.New("dial tcp: connection refused")
	}
	t.Cleanup(func() { moatSyncFn = orig })

	cfg := cfgWithPinnedMOATRegistry()
	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfg,
		t.TempDir(),
		"example",
		"item",
		true,
		time.Now(),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
}

// --- runInstallFromRegistry: lookup outcomes ---------------------------

func TestRunInstallFromRegistry_ItemNotInManifest(t *testing.T) {
	orig := moatSyncFn
	moatSyncFn = func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL: "https://example.com/m",
			Manifest: &moat.Manifest{Content: []moat.ContentEntry{
				{Name: "other", ContentHash: "sha256:" + strings.Repeat("0", 64)},
			}},
			IncomingProfile: incomingProfile(),
		}, nil
	}
	t.Cleanup(func() { moatSyncFn = orig })

	cfg := cfgWithPinnedMOATRegistry()
	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfg,
		t.TempDir(),
		"example",
		"missing",
		true,
		time.Now(),
	)
	assertStructuredCode(t, err, output.ErrInstallItemNotFound)
}

func TestRunInstallFromRegistry_NotModifiedReturnsHint(t *testing.T) {
	orig := moatSyncFn
	moatSyncFn = func(_ context.Context, _ *config.Registry, lf *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		// Emulate Sync's 304 side-effect: lockfile fetched_at advances.
		lf.SetRegistryFetchedAt("https://example.com/m", time.Now().UTC())
		return moat.SyncResult{
			ManifestURL: "https://example.com/m",
			NotModified: true,
			FetchedAt:   time.Now().UTC(),
			Staleness:   moat.StalenessFresh,
		}, nil
	}
	t.Cleanup(func() { moatSyncFn = orig })

	cfg := cfgWithPinnedMOATRegistry()
	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfg,
		t.TempDir(),
		"example",
		"item",
		true,
		time.Now(),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
}

// --- runInstallFromRegistry: dry-run happy path ------------------------

func TestRunInstallFromRegistry_DryRunPrintsSummary(t *testing.T) {
	orig := moatSyncFn
	hash := "sha256:" + strings.Repeat("ab", 32)
	rekorIdx := int64(12345)
	moatSyncFn = func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL: "https://example.com/m",
			ETag:        `"v1"`,
			FetchedAt:   time.Now().UTC(),
			Manifest: &moat.Manifest{Content: []moat.ContentEntry{{
				Name:          "my-skill",
				Type:          "skill",
				ContentHash:   hash,
				SourceURI:     "git+https://example.com/repo.git",
				RekorLogIndex: &rekorIdx,
			}}},
			IncomingProfile:  incomingProfile(),
			Staleness:        moat.StalenessFresh,
			RevocationsAdded: 0,
		}, nil
	}
	t.Cleanup(func() { moatSyncFn = orig })

	var out bytes.Buffer
	cfg := cfgWithPinnedMOATRegistry()
	err := runInstallFromRegistry(
		context.Background(),
		&out,
		&bytes.Buffer{},
		cfg,
		t.TempDir(),
		"example",
		"my-skill",
		true,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	s := out.String()
	for _, needle := range []string{"dry-run", "my-skill", "SIGNED", "sha256:abababab", "gate=proceed"} {
		if !strings.Contains(s, needle) {
			t.Errorf("dry-run output missing %q; got:\n%s", needle, s)
		}
	}
}

// --- runInstallFromRegistry: non-dry-run drives the Proceed-branch fetcher.
// Gates-clean UNSIGNED items flow through fetchAndRecord; SIGNED/DUAL-ATTESTED
// and unsupported source_uri schemes surface MOAT_004 without touching disk.

func TestRunInstallFromRegistry_NonDryRunGitSchemeUnsupported(t *testing.T) {
	orig := moatSyncFn
	hash := "sha256:" + strings.Repeat("cd", 32)
	moatSyncFn = func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL: "https://example.com/m",
			Manifest: &moat.Manifest{Content: []moat.ContentEntry{{
				Name:        "my-skill",
				Type:        "skill",
				ContentHash: hash,
				SourceURI:   "git+https://example.com/repo.git",
			}}},
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
		}, nil
	}
	t.Cleanup(func() { moatSyncFn = orig })

	cfg := cfgWithPinnedMOATRegistry()
	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfg,
		t.TempDir(),
		"example",
		"my-skill",
		false,
		time.Now(),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	var se output.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "source_uri scheme not supported") {
		t.Errorf("expected scheme-not-supported message; got %+v", err)
	}
}

func TestRunInstallFromRegistry_NonDryRunSignedTierDeferred(t *testing.T) {
	orig := moatSyncFn
	hash := "sha256:" + strings.Repeat("cd", 32)
	idx := int64(42)
	moatSyncFn = func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL: "https://example.com/m",
			Manifest: &moat.Manifest{Content: []moat.ContentEntry{{
				Name:          "my-skill",
				Type:          "skill",
				ContentHash:   hash,
				SourceURI:     "https://example.com/my-skill.tar.gz",
				RekorLogIndex: &idx,
			}}},
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
		}, nil
	}
	t.Cleanup(func() { moatSyncFn = orig })

	cfg := cfgWithPinnedMOATRegistry()
	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfg,
		t.TempDir(),
		"example",
		"my-skill",
		false,
		time.Now(),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid)
	var se output.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "trust tier SIGNED") {
		t.Errorf("expected SIGNED-tier deferred message; got %+v", err)
	}
}

// --- helpers -----------------------------------------------------------

// runInstallWithStubbedSyncResult captures the exit code passed to
// moatSyncExit without terminating the test process. Returns the captured
// code (0 if the dispatcher never called moatSyncExit).
func runInstallWithStubbedSyncResult(t *testing.T, res moat.SyncResult, injectErr error) int {
	t.Helper()
	origFn := moatSyncFn
	moatSyncFn = func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return res, injectErr
	}
	t.Cleanup(func() { moatSyncFn = origFn })

	captured := 0
	origExit := moatSyncExit
	moatSyncExit = func(code int) { captured = code }
	t.Cleanup(func() { moatSyncExit = origExit })

	cfg := cfgWithPinnedMOATRegistry()
	_ = runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfg,
		t.TempDir(),
		"example",
		"item",
		true,
		time.Now(),
	)
	return captured
}

// cfgWithPinnedMOATRegistry returns a config with one MOAT registry whose
// SigningProfile matches incomingProfile() — ensures the default stubbed
// SyncResult doesn't accidentally trip the IsTOFU branch.
func cfgWithPinnedMOATRegistry() *config.Config {
	pinned := incomingProfile()
	return &config.Config{Registries: []config.Registry{{
		Name:           "example",
		URL:            "https://example.com/m",
		Type:           config.RegistryTypeMOAT,
		ManifestURI:    "https://example.com/m",
		SigningProfile: &pinned,
	}}}
}

// assertStructuredCode fails the test if err is nil or is not a structured
// error carrying the expected code. Centralizing this helper keeps the
// per-test boilerplate short.
func assertStructuredCode(t *testing.T, err error, wantCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", wantCode)
	}
	var se output.StructuredError
	if !errors.As(err, &se) {
		t.Fatalf("expected output.StructuredError, got %T: %v", err, err)
	}
	if se.Code != wantCode {
		t.Errorf("error code = %s; want %s (msg: %s)", se.Code, wantCode, se.Error())
	}
}

// --- gate-branch tests: HardBlock / PublisherWarn / PrivatePrompt /
// TierBelowPolicy. Each swaps moatSyncFn to return a manifest whose
// content/revocation shape drives the target decision; interactive vs
// headless is controlled via the moatInstallInteractiveFn seam and the
// moatInstallPromptFn seam so we never touch real stdin/stdout TTY
// detection (which varies across CI environments).

// syncResultWithManifest returns a moatSyncFn stub that yields a manifest
// with the given content + revocation rows. The incoming SigningProfile
// matches cfgWithPinnedMOATRegistry so the TOFU branch is skipped.
func syncResultWithManifest(entries []moat.ContentEntry, revs []moat.Revocation) func(context.Context, *config.Registry, *moat.Lockfile, []byte, *moat.Fetcher, time.Time) (moat.SyncResult, error) {
	return func(_ context.Context, _ *config.Registry, _ *moat.Lockfile, _ []byte, _ *moat.Fetcher, _ time.Time) (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL: "https://example.com/m",
			Manifest: &moat.Manifest{
				Content:     entries,
				Revocations: revs,
			},
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
			FetchedAt:       time.Now().UTC(),
		}, nil
	}
}

// withInstallGateStubs swaps the install-gate I/O seams to deterministic
// values so tests exercising the gate branches do not read real stdin or
// depend on TTY detection. Restores originals on t.Cleanup. The returned
// function yields the captured exit code (0 if moatSyncExit never fired).
func withInstallGateStubs(t *testing.T, interactive, promptYes bool) *int {
	t.Helper()
	capturedExit := 0
	origExit := moatSyncExit
	moatSyncExit = func(code int) { capturedExit = code }

	origInteractive := moatInstallInteractiveFn
	moatInstallInteractiveFn = func() bool { return interactive }

	origPrompt := moatInstallPromptFn
	moatInstallPromptFn = func(io.Writer, string) bool { return promptYes }

	t.Cleanup(func() {
		moatSyncExit = origExit
		moatInstallInteractiveFn = origInteractive
		moatInstallPromptFn = origPrompt
	})
	return &capturedExit
}

// signedManifestEntry returns a ContentEntry that lands in the SIGNED
// trust tier (rekor_log_index present, no per-item signing_profile).
func signedManifestEntry(name, hash string) moat.ContentEntry {
	idx := int64(42)
	return moat.ContentEntry{
		Name:          name,
		Type:          "skill",
		ContentHash:   hash,
		RekorLogIndex: &idx,
	}
}

// TestRunInstallFromRegistry_HardBlockReturnsStructured verifies that a
// registry-source revocation refuses the install with MOAT_008. The
// revocation is attached to the same content_hash the operator is trying
// to install, so RevocationSet.Lookup fires and PreInstallCheck returns
// MOATGateHardBlock.
func TestRunInstallFromRegistry_HardBlockReturnsStructured(t *testing.T) {
	hash := "sha256:" + strings.Repeat("aa", 32)
	orig := moatSyncFn
	moatSyncFn = syncResultWithManifest(
		[]moat.ContentEntry{signedManifestEntry("my-skill", hash)},
		[]moat.Revocation{{ContentHash: hash, Reason: moat.RevocationReasonMalicious, Source: moat.RevocationSourceRegistry, DetailsURL: "https://example.com/rev"}},
	)
	t.Cleanup(func() { moatSyncFn = orig })

	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfgWithPinnedMOATRegistry(),
		t.TempDir(),
		"example",
		"my-skill",
		false,
		time.Now(),
	)
	assertStructuredCode(t, err, output.ErrMoatRevocationBlock)
}

// TestRunInstallFromRegistry_PublisherWarnHeadlessExits12 checks the G-18
// non-interactive path. Manifest lists a publisher-source revocation;
// moatInstallInteractiveFn returns false, so the prompt is skipped and
// moatSyncExit(12) fires.
func TestRunInstallFromRegistry_PublisherWarnHeadlessExits12(t *testing.T) {
	hash := "sha256:" + strings.Repeat("bb", 32)
	orig := moatSyncFn
	moatSyncFn = syncResultWithManifest(
		[]moat.ContentEntry{signedManifestEntry("my-skill", hash)},
		[]moat.Revocation{{ContentHash: hash, Reason: moat.RevocationReasonDeprecated, Source: moat.RevocationSourcePublisher, DetailsURL: "https://example.com/rev"}},
	)
	t.Cleanup(func() { moatSyncFn = orig })

	capturedExit := withInstallGateStubs(t, false, false)

	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfgWithPinnedMOATRegistry(),
		t.TempDir(),
		"example",
		"my-skill",
		false,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("publisher-warn headless should return nil (moatSyncExit handles exit); got %v", err)
	}
	if *capturedExit != moat.ExitMoatPublisherRevocation {
		t.Errorf("exit code = %d; want %d", *capturedExit, moat.ExitMoatPublisherRevocation)
	}
}

// TestRunInstallFromRegistry_PublisherWarnInteractiveYesProceeds checks
// the interactive Y path. After the prompt answers Yes, the gate returns
// proceed and the caller lands in the "fetch deferred" structured error
// (MOAT_004), NOT the revocation-block error. This is how we know the
// suppression + proceed composition works end-to-end.
func TestRunInstallFromRegistry_PublisherWarnInteractiveYesProceeds(t *testing.T) {
	hash := "sha256:" + strings.Repeat("cc", 32)
	orig := moatSyncFn
	moatSyncFn = syncResultWithManifest(
		[]moat.ContentEntry{signedManifestEntry("my-skill", hash)},
		[]moat.Revocation{{ContentHash: hash, Reason: moat.RevocationReasonMalicious, Source: moat.RevocationSourcePublisher, DetailsURL: "https://example.com/rev"}},
	)
	t.Cleanup(func() { moatSyncFn = orig })

	capturedExit := withInstallGateStubs(t, true, true)

	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfgWithPinnedMOATRegistry(),
		t.TempDir(),
		"example",
		"my-skill",
		false,
		time.Now(),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid) // "cleared MOAT gates" deferred-fetch error
	if *capturedExit != 0 {
		t.Errorf("expected no exit() call after operator Y; got %d", *capturedExit)
	}
}

// TestRunInstallFromRegistry_PublisherWarnInteractiveNoRefuses confirms
// the interactive N path returns nil with no exit fired — the operator
// declined, so the command exits cleanly without a structured error (the
// refusal message is written to stderr for the operator's log).
func TestRunInstallFromRegistry_PublisherWarnInteractiveNoRefuses(t *testing.T) {
	hash := "sha256:" + strings.Repeat("dd", 32)
	orig := moatSyncFn
	moatSyncFn = syncResultWithManifest(
		[]moat.ContentEntry{signedManifestEntry("my-skill", hash)},
		[]moat.Revocation{{ContentHash: hash, Source: moat.RevocationSourcePublisher, Reason: moat.RevocationReasonMalicious, DetailsURL: "https://example.com/rev"}},
	)
	t.Cleanup(func() { moatSyncFn = orig })

	capturedExit := withInstallGateStubs(t, true, false)

	var errBuf bytes.Buffer
	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&errBuf,
		cfgWithPinnedMOATRegistry(),
		t.TempDir(),
		"example",
		"my-skill",
		false,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("operator-N should return nil; got %v", err)
	}
	if *capturedExit != 0 {
		t.Errorf("expected no exit(); got %d", *capturedExit)
	}
	if !strings.Contains(errBuf.String(), "refused by operator") {
		t.Errorf("expected stderr to note operator refusal; got %q", errBuf.String())
	}
}

// TestRunInstallFromRegistry_PrivatePromptHeadlessExits10 covers the
// G-10 + G-18 composition: a private_repo item in headless mode maps to
// ExitMoatTOFUAcceptance (10), per the moat_gate.go contract.
func TestRunInstallFromRegistry_PrivatePromptHeadlessExits10(t *testing.T) {
	hash := "sha256:" + strings.Repeat("ee", 32)
	entry := signedManifestEntry("my-skill", hash)
	entry.PrivateRepo = true
	orig := moatSyncFn
	moatSyncFn = syncResultWithManifest([]moat.ContentEntry{entry}, nil)
	t.Cleanup(func() { moatSyncFn = orig })

	capturedExit := withInstallGateStubs(t, false, false)

	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfgWithPinnedMOATRegistry(),
		t.TempDir(),
		"example",
		"my-skill",
		false,
		time.Now(),
	)
	if err != nil {
		t.Fatalf("private-prompt headless should return nil; got %v", err)
	}
	if *capturedExit != moat.ExitMoatTOFUAcceptance {
		t.Errorf("exit code = %d; want %d", *capturedExit, moat.ExitMoatTOFUAcceptance)
	}
}

// TestRunInstallFromRegistry_PrivatePromptInteractiveYesProceeds checks
// the interactive Y branch for private content.
func TestRunInstallFromRegistry_PrivatePromptInteractiveYesProceeds(t *testing.T) {
	hash := "sha256:" + strings.Repeat("ff", 32)
	entry := signedManifestEntry("my-skill", hash)
	entry.PrivateRepo = true
	orig := moatSyncFn
	moatSyncFn = syncResultWithManifest([]moat.ContentEntry{entry}, nil)
	t.Cleanup(func() { moatSyncFn = orig })

	withInstallGateStubs(t, true, true)

	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfgWithPinnedMOATRegistry(),
		t.TempDir(),
		"example",
		"my-skill",
		false,
		time.Now(),
	)
	assertStructuredCode(t, err, output.ErrMoatInvalid) // deferred-fetch error after Y
}

// TestRunInstallFromRegistry_TierBelowPolicyReturnsStructured raises the
// min-tier seam to DualAttested and feeds a SIGNED entry — PreInstallCheck
// returns MOATGateTierBelowPolicy and the caller surfaces MOAT_009.
func TestRunInstallFromRegistry_TierBelowPolicyReturnsStructured(t *testing.T) {
	hash := "sha256:" + strings.Repeat("12", 32)
	orig := moatSyncFn
	moatSyncFn = syncResultWithManifest(
		[]moat.ContentEntry{signedManifestEntry("my-skill", hash)},
		nil,
	)
	t.Cleanup(func() { moatSyncFn = orig })

	origMin := moatInstallMinTier
	moatInstallMinTier = moat.TrustTierDualAttested
	t.Cleanup(func() { moatInstallMinTier = origMin })

	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfgWithPinnedMOATRegistry(),
		t.TempDir(),
		"example",
		"my-skill",
		false,
		time.Now(),
	)
	assertStructuredCode(t, err, output.ErrMoatTierBelowPolicy)
}

// TestRunInstallFromRegistry_DryRunPreviewsGateDecision confirms the
// dry-run summary carries the gate decision so operators can preview a
// non-proceed outcome without triggering the prompt.
func TestRunInstallFromRegistry_DryRunPreviewsGateDecision(t *testing.T) {
	hash := "sha256:" + strings.Repeat("34", 32)
	orig := moatSyncFn
	moatSyncFn = syncResultWithManifest(
		[]moat.ContentEntry{signedManifestEntry("my-skill", hash)},
		[]moat.Revocation{{ContentHash: hash, Source: moat.RevocationSourceRegistry, Reason: moat.RevocationReasonMalicious, DetailsURL: "https://example.com/rev"}},
	)
	t.Cleanup(func() { moatSyncFn = orig })

	err := runInstallFromRegistry(
		context.Background(),
		&bytes.Buffer{},
		&bytes.Buffer{},
		cfgWithPinnedMOATRegistry(),
		t.TempDir(),
		"example",
		"my-skill",
		true, // dry-run
		time.Now(),
	)
	// Dry-run STILL surfaces the hard-block rather than a summary —
	// printing "would install" for a revoked item would be misleading.
	assertStructuredCode(t, err, output.ErrMoatRevocationBlock)
}

// TestDefaultMoatInstallPrompt_ParsesAffirmative exercises the default
// stdin-backed prompt. We redirect os.Stdin to a pipe so the scanner
// reads a canned answer. Covered: "" (default Y), "y", "n", trailing
// whitespace, mixed case.
func TestDefaultMoatInstallPrompt_ParsesAffirmative(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"\n", true},
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{" YES \n", true},
		{"n\n", false},
		{"no\n", false},
		{"whatever\n", false},
	}
	for _, c := range cases {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe: %v", err)
		}
		origStdin := os.Stdin
		os.Stdin = r
		_, _ = w.Write([]byte(c.in))
		_ = w.Close()

		var out bytes.Buffer
		got := defaultMoatInstallPrompt(&out, "Q? ")
		os.Stdin = origStdin
		_ = r.Close()

		if got != c.want {
			t.Errorf("defaultMoatInstallPrompt(%q) = %v; want %v", c.in, got, c.want)
		}
		if !strings.Contains(out.String(), "Q?") {
			t.Errorf("prompt text missing; got %q", out.String())
		}
	}
}
