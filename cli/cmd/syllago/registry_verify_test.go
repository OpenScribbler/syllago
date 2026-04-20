package main

// Tests for verifyRegistryForAdd (ADR 0007 slice-2c).
//
// Coverage strategy: the cryptographic heavy lifting is already exhaustively
// tested in internal/moat/manifest_verify_test.go. This file tests the
// POLICY layer — how the add_cmd gate maps VerifyManifest outcomes to
// StructuredError codes, honors staleness gates, and emits trust labels.
//
// Two indirection points make this possible without live Sigstore fixtures:
//   - verifyManifestFn: swapped to return canned VerifyError values.
//   - verifyTrustedRootFn: swapped to inject arbitrary staleness buckets.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// freshBundledRoot returns a TrustedRootInfo that passes the staleness gate.
// Used in tests that are not exercising the staleness branch themselves.
func freshBundledRoot() moat.TrustedRootInfo {
	return moat.TrustedRootInfo{
		Source:   moat.TrustedRootSourceBundled,
		Status:   moat.TrustedRootStatusFresh,
		IssuedAt: time.Now().AddDate(0, 0, -1),
		Bytes:    []byte(`{"mediaType":"test"}`),
	}
}

// withStubbedVerifiers wires the two indirection points for one test case
// and registers cleanup. Helpers keep the per-test boilerplate to one line.
// The trusted-root stub takes the slice-2d (reg, override, now) signature;
// tests that don't exercise the override branch pass a closure that ignores
// the override argument.
func withStubbedVerifiers(
	t *testing.T,
	vm func([]byte, []byte, *moat.SigningProfile, []byte) (moat.VerificationResult, error),
	tr func(*config.Registry, string, time.Time) (moat.TrustedRootInfo, error),
) {
	t.Helper()
	origVM := verifyManifestFn
	origTR := verifyTrustedRootFn
	verifyManifestFn = vm
	verifyTrustedRootFn = tr
	t.Cleanup(func() {
		verifyManifestFn = origVM
		verifyTrustedRootFn = origTR
	})
}

// stubbedFreshBundledRoot is a convenience trusted-root stub for tests that
// just need verification to move past the staleness gate with the bundled
// default in effect.
func stubbedFreshBundledRoot(_ *config.Registry, _ string, _ time.Time) (moat.TrustedRootInfo, error) {
	return freshBundledRoot(), nil
}

// writeManifestFixture drops placeholder manifest + bundle files into a
// temp clone dir. Contents are byte-opaque to the tests here — the canned
// verifyManifestFn is what decides pass/fail.
func writeManifestFixture(t *testing.T, cloneDir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(cloneDir, manifestFileName), []byte(`{"schema_version":1}`), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cloneDir, bundleFileName), []byte(`{"bundle":"stub"}`), 0644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
}

// pinnedGitHubProfile returns a pinned profile matching a MOAT GitHub
// Actions registry — the most common real-world case.
func pinnedGitHubProfile() *config.SigningProfile {
	return &config.SigningProfile{
		Issuer:            moat.GitHubActionsIssuer,
		Subject:           "https://github.com/OpenScribbler/syllago-meta-registry/.github/workflows/moat-registry.yml@refs/heads/master",
		RepositoryID:      "1193220959",
		RepositoryOwnerID: "263775997",
	}
}

// expectStructuredErrorCode asserts err is a StructuredError with the given code.
func expectStructuredErrorCode(t *testing.T, err error, wantCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected StructuredError %q, got nil", wantCode)
	}
	var se output.StructuredError
	if !errors.As(err, &se) {
		t.Fatalf("expected StructuredError, got %T: %v", err, err)
	}
	if se.Code != wantCode {
		t.Fatalf("error code mismatch: got %q want %q (err=%v)", se.Code, wantCode, err)
	}
}

// TestVerifyRegistryForAdd_NilReg — defensive path: nil registry returns
// (nil, nil) so callers can chain without nil-check boilerplate.
func TestVerifyRegistryForAdd_NilReg(t *testing.T) {
	t.Parallel()
	out, err := verifyRegistryForAdd(nil, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil outcome, got %+v", out)
	}
}

// TestVerifyRegistryForAdd_UnpinnedGitRegistry — the slice-1 back-compat
// path. Unsigned git registry → legacy mode, nothing to verify.
func TestVerifyRegistryForAdd_UnpinnedGitRegistry(t *testing.T) {
	t.Parallel()
	reg := &config.Registry{Name: "someone/legacy", Type: config.RegistryTypeGit}
	out, err := verifyRegistryForAdd(reg, t.TempDir(), "")
	if err != nil {
		t.Fatalf("unsigned git registry should bypass verify: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil outcome for legacy mode, got %+v", out)
	}
}

// TestVerifyRegistryForAdd_MoatTypedWithoutProfile — hard fail: registry
// claims to be MOAT but has no profile. Caller must re-add explicitly.
func TestVerifyRegistryForAdd_MoatTypedWithoutProfile(t *testing.T) {
	t.Parallel()
	reg := &config.Registry{Name: "corrupt/moat", Type: config.RegistryTypeMOAT}
	_, err := verifyRegistryForAdd(reg, t.TempDir(), "")
	expectStructuredErrorCode(t, err, output.ErrMoatIdentityUnpinned)
}

// TestVerifyRegistryForAdd_HappyPath — pinned profile + manifest files +
// verify OK → outcome labeled "signed" with numeric-ID match reported.
func TestVerifyRegistryForAdd_HappyPath(t *testing.T) {
	cloneDir := t.TempDir()
	writeManifestFixture(t, cloneDir)
	withStubbedVerifiers(t,
		func(_, _ []byte, _ *moat.SigningProfile, _ []byte) (moat.VerificationResult, error) {
			return moat.VerificationResult{
				SignatureValid:        true,
				CertificateChainValid: true,
				RekorProofValid:       true,
				IdentityMatches:       true,
				NumericIDMatched:      true,
			}, nil
		},
		stubbedFreshBundledRoot,
	)

	reg := &config.Registry{
		Name:           "OpenScribbler/syllago-meta-registry",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: pinnedGitHubProfile(),
	}
	out, err := verifyRegistryForAdd(reg, cloneDir, "")
	if err != nil {
		t.Fatalf("happy path must succeed: %v", err)
	}
	if out == nil {
		t.Fatal("happy path must return a non-nil outcome")
	}
	if out.Label != trustSigned {
		t.Errorf("Label = %q, want %q", out.Label, trustSigned)
	}
	if !out.NumericIDMatched {
		t.Errorf("NumericIDMatched should propagate from verify result")
	}
	if out.ProfileVersion != moat.ProfileVersionV1 {
		t.Errorf("ProfileVersion = %d, want %d", out.ProfileVersion, moat.ProfileVersionV1)
	}
}

// TestVerifyRegistryForAdd_IdentityMismatch — verify returns
// MOAT_IDENTITY_MISMATCH → gate maps to MOAT_003 and aborts add.
func TestVerifyRegistryForAdd_IdentityMismatch(t *testing.T) {
	cloneDir := t.TempDir()
	writeManifestFixture(t, cloneDir)
	withStubbedVerifiers(t,
		func(_, _ []byte, _ *moat.SigningProfile, _ []byte) (moat.VerificationResult, error) {
			return moat.VerificationResult{}, &moat.VerifyError{
				Code:    moat.CodeIdentityMismatch,
				Message: "repository_id mismatch: got=\"9999\" want=\"1193220959\"",
			}
		},
		stubbedFreshBundledRoot,
	)

	reg := &config.Registry{
		Name:           "OpenScribbler/syllago-meta-registry",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: pinnedGitHubProfile(),
	}
	_, err := verifyRegistryForAdd(reg, cloneDir, "")
	expectStructuredErrorCode(t, err, output.ErrMoatIdentityMismatch)
}

// TestVerifyRegistryForAdd_GenericVerifyErrorCollapses — any other
// VerifyError code collapses to MOAT_004 (invalid).
func TestVerifyRegistryForAdd_GenericVerifyErrorCollapses(t *testing.T) {
	cloneDir := t.TempDir()
	writeManifestFixture(t, cloneDir)
	withStubbedVerifiers(t,
		func(_, _ []byte, _ *moat.SigningProfile, _ []byte) (moat.VerificationResult, error) {
			return moat.VerificationResult{}, &moat.VerifyError{
				Code:    moat.CodeInvalid,
				Message: "sigstore-go verify: signature invalid",
			}
		},
		stubbedFreshBundledRoot,
	)

	reg := &config.Registry{
		Name:           "OpenScribbler/syllago-meta-registry",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: pinnedGitHubProfile(),
	}
	_, err := verifyRegistryForAdd(reg, cloneDir, "")
	expectStructuredErrorCode(t, err, output.ErrMoatInvalid)
}

// TestVerifyRegistryForAdd_TrustedRootStaleCode — verify returns a
// trusted-root-stale code → gate maps to MOAT_005.
func TestVerifyRegistryForAdd_TrustedRootStaleCode(t *testing.T) {
	cloneDir := t.TempDir()
	writeManifestFixture(t, cloneDir)
	withStubbedVerifiers(t,
		func(_, _ []byte, _ *moat.SigningProfile, _ []byte) (moat.VerificationResult, error) {
			return moat.VerificationResult{}, &moat.VerifyError{
				Code:    moat.CodeTrustedRootMissing,
				Message: "trusted root bytes empty",
			}
		},
		stubbedFreshBundledRoot,
	)

	reg := &config.Registry{
		Name:           "OpenScribbler/syllago-meta-registry",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: pinnedGitHubProfile(),
	}
	_, err := verifyRegistryForAdd(reg, cloneDir, "")
	expectStructuredErrorCode(t, err, output.ErrMoatTrustedRootStale)
}

// TestVerifyRegistryForAdd_VerifyIdentityUnpinnedCode — verify returns
// MOAT_IDENTITY_UNPINNED → gate maps to MOAT_001.
func TestVerifyRegistryForAdd_VerifyIdentityUnpinnedCode(t *testing.T) {
	cloneDir := t.TempDir()
	writeManifestFixture(t, cloneDir)
	withStubbedVerifiers(t,
		func(_, _ []byte, _ *moat.SigningProfile, _ []byte) (moat.VerificationResult, error) {
			return moat.VerificationResult{}, &moat.VerifyError{
				Code:    moat.CodeIdentityUnpinned,
				Message: "pinned signing profile required",
			}
		},
		stubbedFreshBundledRoot,
	)

	reg := &config.Registry{
		Name:           "OpenScribbler/syllago-meta-registry",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: pinnedGitHubProfile(),
	}
	_, err := verifyRegistryForAdd(reg, cloneDir, "")
	expectStructuredErrorCode(t, err, output.ErrMoatIdentityUnpinned)
}

// TestVerifyRegistryForAdd_PlainErrorCollapses — a non-VerifyError
// (e.g., a generic fmt.Errorf) still maps cleanly to MOAT_004 rather than
// leaking the raw Go error.
func TestVerifyRegistryForAdd_PlainErrorCollapses(t *testing.T) {
	cloneDir := t.TempDir()
	writeManifestFixture(t, cloneDir)
	withStubbedVerifiers(t,
		func(_, _ []byte, _ *moat.SigningProfile, _ []byte) (moat.VerificationResult, error) {
			return moat.VerificationResult{}, errors.New("something unexpected")
		},
		stubbedFreshBundledRoot,
	)

	reg := &config.Registry{
		Name:           "OpenScribbler/syllago-meta-registry",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: pinnedGitHubProfile(),
	}
	_, err := verifyRegistryForAdd(reg, cloneDir, "")
	expectStructuredErrorCode(t, err, output.ErrMoatInvalid)
}

// TestVerifyRegistryForAdd_MissingManifestWithPin — pinned profile but
// no manifest.json in the checkout → MOAT_006.
func TestVerifyRegistryForAdd_MissingManifestWithPin(t *testing.T) {
	cloneDir := t.TempDir()
	// Deliberately no manifest files written.
	withStubbedVerifiers(t,
		func(_, _ []byte, _ *moat.SigningProfile, _ []byte) (moat.VerificationResult, error) {
			t.Fatal("verifyManifestFn should not run when manifest is missing")
			return moat.VerificationResult{}, nil
		},
		stubbedFreshBundledRoot,
	)

	reg := &config.Registry{
		Name:           "OpenScribbler/syllago-meta-registry",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: pinnedGitHubProfile(),
	}
	_, err := verifyRegistryForAdd(reg, cloneDir, "")
	expectStructuredErrorCode(t, err, output.ErrMoatUnsignedWithPin)
}

// TestVerifyRegistryForAdd_MissingBundleOnly — manifest exists but bundle
// is absent → still MOAT_006 (both files required for verification).
func TestVerifyRegistryForAdd_MissingBundleOnly(t *testing.T) {
	cloneDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cloneDir, manifestFileName), []byte("{}"), 0644); err != nil {
		t.Fatalf("seeding manifest: %v", err)
	}
	withStubbedVerifiers(t,
		func(_, _ []byte, _ *moat.SigningProfile, _ []byte) (moat.VerificationResult, error) {
			t.Fatal("verifyManifestFn should not run when bundle is missing")
			return moat.VerificationResult{}, nil
		},
		stubbedFreshBundledRoot,
	)

	reg := &config.Registry{
		Name:           "OpenScribbler/syllago-meta-registry",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: pinnedGitHubProfile(),
	}
	_, err := verifyRegistryForAdd(reg, cloneDir, "")
	expectStructuredErrorCode(t, err, output.ErrMoatUnsignedWithPin)
}

// TestVerifyRegistryForAdd_TrustedRootExpired — inject an expired bundled
// root and confirm MOAT_005 fires BEFORE we spend I/O reading the manifest.
func TestVerifyRegistryForAdd_TrustedRootExpired(t *testing.T) {
	cloneDir := t.TempDir()
	// No files written — if staleness check fails to short-circuit, the
	// read-manifest path would return MOAT_006 instead of MOAT_005.
	withStubbedVerifiers(t,
		func(_, _ []byte, _ *moat.SigningProfile, _ []byte) (moat.VerificationResult, error) {
			t.Fatal("verifyManifestFn should not run when trusted root is expired")
			return moat.VerificationResult{}, nil
		},
		func(*config.Registry, string, time.Time) (moat.TrustedRootInfo, error) {
			return moat.TrustedRootInfo{
				Source:    moat.TrustedRootSourceBundled,
				Status:    moat.TrustedRootStatusExpired,
				IssuedAt:  time.Now().AddDate(-2, 0, 0),
				AgeDays:   730,
				CliffDate: time.Now().AddDate(-1, 0, 0),
				Bytes:     []byte("stale"),
			}, nil
		},
	)

	reg := &config.Registry{
		Name:           "OpenScribbler/syllago-meta-registry",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: pinnedGitHubProfile(),
	}
	_, err := verifyRegistryForAdd(reg, cloneDir, "")
	expectStructuredErrorCode(t, err, output.ErrMoatTrustedRootStale)
}

// TestVerifyRegistryForAdd_UnknownProfileVersion — pinned profile declares
// an unsupported version. Gate must reject before any verify work.
func TestVerifyRegistryForAdd_UnknownProfileVersion(t *testing.T) {
	cloneDir := t.TempDir()
	writeManifestFixture(t, cloneDir)
	withStubbedVerifiers(t,
		func(_, _ []byte, _ *moat.SigningProfile, _ []byte) (moat.VerificationResult, error) {
			t.Fatal("verifyManifestFn should not run for unknown profile versions")
			return moat.VerificationResult{}, nil
		},
		stubbedFreshBundledRoot,
	)

	profile := pinnedGitHubProfile()
	profile.ProfileVersion = 99
	reg := &config.Registry{
		Name:           "future/registry",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: profile,
	}
	_, err := verifyRegistryForAdd(reg, cloneDir, "")
	expectStructuredErrorCode(t, err, output.ErrMoatInvalid)
}

// TestReadManifestFromCheckout_IOError — a file exists but can't be read
// (permission denied, etc.) must surface as an error rather than a silent
// "not found." Simulated by making manifest.json unreadable.
func TestReadManifestFromCheckout_IOError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file permission checks")
	}
	cloneDir := t.TempDir()
	manifestPath := filepath.Join(cloneDir, manifestFileName)
	if err := os.WriteFile(manifestPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cloneDir, bundleFileName), []byte(`{}`), 0644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	if err := os.Chmod(manifestPath, 0000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(manifestPath, 0644) })

	_, _, _, err := readManifestFromCheckout(cloneDir)
	if err == nil {
		t.Fatal("expected permission error, got nil")
	}
}

// TestNumericIDLabel — trivial pure function; cover both branches.
func TestNumericIDLabel(t *testing.T) {
	t.Parallel()
	if numericIDLabel(true) != "matched" {
		t.Errorf("matched case wrong: got %q", numericIDLabel(true))
	}
	if numericIDLabel(false) != "not pinned" {
		t.Errorf("unmatched case wrong: got %q", numericIDLabel(false))
	}
}

// TestEmitTrustLabel_QuietOrJSONSuppresses — trust labels must not spam
// quiet/JSON callers, but the normal path must write them.
func TestEmitTrustLabel_QuietOrJSONSuppresses(t *testing.T) {
	// Cannot t.Parallel — mutates package-level output globals.
	buf, _ := output.SetForTest(t)

	// Nil outcome is a no-op.
	emitTrustLabel(nil, "x")
	if buf.Len() != 0 {
		t.Errorf("nil outcome emitted output: %q", buf.String())
	}

	out := &verifyOutcome{Label: trustSigned, Source: moat.TrustedRootSourceBundled}
	// Quiet mode.
	origQuiet := output.Quiet
	output.Quiet = true
	emitTrustLabel(out, "reg")
	output.Quiet = origQuiet
	if buf.Len() != 0 {
		t.Errorf("quiet mode emitted output: %q", buf.String())
	}

	// JSON mode.
	origJSON := output.JSON
	output.JSON = true
	emitTrustLabel(out, "reg")
	output.JSON = origJSON
	if buf.Len() != 0 {
		t.Errorf("JSON mode emitted output: %q", buf.String())
	}

	// Normal mode: writes.
	emitTrustLabel(out, "reg")
	if buf.Len() == 0 {
		t.Error("normal mode must write trust label")
	}
}

// --------------------------------------------------------------------
// Slice-2d tests: per-registry trusted-root override & --trusted-root flag.
// --------------------------------------------------------------------

// writeTrustedRootFile drops a minimal valid trusted_root.json into a temp
// path and returns the path. Used by the slice-2d override tests which
// exercise the REAL verifyTrustedRootFn (not a stub) — the whole point is
// to prove the default loader reaches the file through reg.TrustedRoot or
// the override parameter.
func writeTrustedRootFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "trusted_root.json")
	if err := os.WriteFile(path, []byte(`{"mediaType":"test"}`), 0644); err != nil {
		t.Fatalf("write trusted root: %v", err)
	}
	return path
}

// TestVerifyRegistryForAdd_TrustedRootCLIOverride_SuccessPath — --trusted-root
// flag points at a valid file. Verifier reaches it, Source=path propagates to
// the outcome, TrustedRootPath is recorded for the info emitter.
//
// Stubs only verifyManifestFn — the real verifyTrustedRootFn must load the
// file. This test would regress if anyone turns the default loader into a
// no-op or forgets to consult the override parameter.
func TestVerifyRegistryForAdd_TrustedRootCLIOverride_SuccessPath(t *testing.T) {
	cloneDir := t.TempDir()
	writeManifestFixture(t, cloneDir)
	overridePath := writeTrustedRootFile(t)

	origVM := verifyManifestFn
	verifyManifestFn = func(_, _ []byte, _ *moat.SigningProfile, trustedRoot []byte) (moat.VerificationResult, error) {
		// Prove the override bytes reach the verifier — the bundled root
		// would be several KB, our fixture is 21 bytes.
		if len(trustedRoot) > 100 {
			t.Errorf("expected override trusted-root bytes (~21B), got %d", len(trustedRoot))
		}
		return moat.VerificationResult{SignatureValid: true, NumericIDMatched: true}, nil
	}
	t.Cleanup(func() { verifyManifestFn = origVM })

	reg := &config.Registry{
		Name:           "OpenScribbler/syllago-meta-registry",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: pinnedGitHubProfile(),
	}
	out, err := verifyRegistryForAdd(reg, cloneDir, overridePath)
	if err != nil {
		t.Fatalf("override success path must not error: %v", err)
	}
	if out == nil {
		t.Fatal("expected a populated outcome")
	}
	if out.Source != moat.TrustedRootSourcePathFlag {
		t.Errorf("Source = %q, want %q", out.Source, moat.TrustedRootSourcePathFlag)
	}
	if out.TrustedRootPath != overridePath {
		t.Errorf("TrustedRootPath = %q, want %q", out.TrustedRootPath, overridePath)
	}
}

// TestVerifyRegistryForAdd_TrustedRootRegistryConfig_UsedWhenNoCLIOverride —
// reg.TrustedRoot is consulted when the CLI flag is empty. Slice-1 already
// persisted the field; slice-2d is the wiring that finally honors it.
func TestVerifyRegistryForAdd_TrustedRootRegistryConfig_UsedWhenNoCLIOverride(t *testing.T) {
	cloneDir := t.TempDir()
	writeManifestFixture(t, cloneDir)
	regConfigPath := writeTrustedRootFile(t)

	origVM := verifyManifestFn
	verifyManifestFn = func(_, _ []byte, _ *moat.SigningProfile, _ []byte) (moat.VerificationResult, error) {
		return moat.VerificationResult{SignatureValid: true}, nil
	}
	t.Cleanup(func() { verifyManifestFn = origVM })

	reg := &config.Registry{
		Name:           "enterprise/internal",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: pinnedGitHubProfile(),
		TrustedRoot:    regConfigPath,
	}
	out, err := verifyRegistryForAdd(reg, cloneDir, "") // empty CLI override
	if err != nil {
		t.Fatalf("reg.TrustedRoot path must be used when CLI override is empty: %v", err)
	}
	if out.Source != moat.TrustedRootSourcePathFlag {
		t.Errorf("Source should reflect path override from reg config, got %q", out.Source)
	}
	if out.TrustedRootPath != regConfigPath {
		t.Errorf("TrustedRootPath = %q, want %q", out.TrustedRootPath, regConfigPath)
	}
}

// TestVerifyRegistryForAdd_TrustedRootCLIOverride_WinsOverRegistryConfig —
// when both are set, the CLI flag must win. Matters for enterprise
// operators debugging a specific invocation without editing config.json.
func TestVerifyRegistryForAdd_TrustedRootCLIOverride_WinsOverRegistryConfig(t *testing.T) {
	cloneDir := t.TempDir()
	writeManifestFixture(t, cloneDir)
	cliPath := writeTrustedRootFile(t)
	regPath := writeTrustedRootFile(t)
	if cliPath == regPath {
		t.Fatal("temp dirs should produce distinct paths")
	}

	origVM := verifyManifestFn
	verifyManifestFn = func(_, _ []byte, _ *moat.SigningProfile, _ []byte) (moat.VerificationResult, error) {
		return moat.VerificationResult{SignatureValid: true}, nil
	}
	t.Cleanup(func() { verifyManifestFn = origVM })

	reg := &config.Registry{
		Name:           "enterprise/internal",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: pinnedGitHubProfile(),
		TrustedRoot:    regPath,
	}
	out, err := verifyRegistryForAdd(reg, cloneDir, cliPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TrustedRootPath != cliPath {
		t.Errorf("CLI flag must win: got %q, want %q", out.TrustedRootPath, cliPath)
	}
}

// TestVerifyRegistryForAdd_TrustedRootCLIOverride_MissingFile — --trusted-root
// names a path that doesn't exist → MOAT_007. Must NOT silently fall back to
// the bundled root (trust downgrade).
func TestVerifyRegistryForAdd_TrustedRootCLIOverride_MissingFile(t *testing.T) {
	cloneDir := t.TempDir()
	writeManifestFixture(t, cloneDir)
	missing := filepath.Join(t.TempDir(), "not-there.json")

	origVM := verifyManifestFn
	verifyManifestFn = func(_, _ []byte, _ *moat.SigningProfile, _ []byte) (moat.VerificationResult, error) {
		t.Fatal("verify must not run when trusted-root override is unloadable")
		return moat.VerificationResult{}, nil
	}
	t.Cleanup(func() { verifyManifestFn = origVM })

	reg := &config.Registry{
		Name:           "enterprise/internal",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: pinnedGitHubProfile(),
	}
	_, err := verifyRegistryForAdd(reg, cloneDir, missing)
	expectStructuredErrorCode(t, err, output.ErrMoatTrustedRootOverride)
}

// TestVerifyRegistryForAdd_TrustedRootCLIOverride_MalformedJSON — override
// path exists but isn't valid JSON → MOAT_007.
func TestVerifyRegistryForAdd_TrustedRootCLIOverride_MalformedJSON(t *testing.T) {
	cloneDir := t.TempDir()
	writeManifestFixture(t, cloneDir)
	badDir := t.TempDir()
	badPath := filepath.Join(badDir, "bad.json")
	if err := os.WriteFile(badPath, []byte("totally-not-json{{{"), 0644); err != nil {
		t.Fatalf("seeding bad file: %v", err)
	}

	origVM := verifyManifestFn
	verifyManifestFn = func(_, _ []byte, _ *moat.SigningProfile, _ []byte) (moat.VerificationResult, error) {
		t.Fatal("verify must not run when override JSON is malformed")
		return moat.VerificationResult{}, nil
	}
	t.Cleanup(func() { verifyManifestFn = origVM })

	reg := &config.Registry{
		Name:           "enterprise/internal",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: pinnedGitHubProfile(),
	}
	_, err := verifyRegistryForAdd(reg, cloneDir, badPath)
	expectStructuredErrorCode(t, err, output.ErrMoatTrustedRootOverride)
}

// TestVerifyRegistryForAdd_NoOverride_UsesBundled — the default path: no
// CLI flag, no reg.TrustedRoot → bundled root is used. Also exercises the
// real verifyTrustedRootFn path to prove it falls through to
// BundledTrustedRoot rather than erroring on an empty path.
func TestVerifyRegistryForAdd_NoOverride_UsesBundled(t *testing.T) {
	cloneDir := t.TempDir()
	writeManifestFixture(t, cloneDir)

	origVM := verifyManifestFn
	verifyManifestFn = func(_, _ []byte, _ *moat.SigningProfile, trustedRoot []byte) (moat.VerificationResult, error) {
		// Bundled root is a real embedded file, several KB. If the loader
		// defaulted to an empty path and errored, we'd never get here.
		if len(trustedRoot) < 500 {
			t.Errorf("expected bundled trusted-root bytes (>500B), got %d — override path leaked through?", len(trustedRoot))
		}
		return moat.VerificationResult{SignatureValid: true}, nil
	}
	t.Cleanup(func() { verifyManifestFn = origVM })

	reg := &config.Registry{
		Name:           "OpenScribbler/syllago-meta-registry",
		Type:           config.RegistryTypeMOAT,
		SigningProfile: pinnedGitHubProfile(),
	}
	out, err := verifyRegistryForAdd(reg, cloneDir, "")
	if err != nil {
		t.Fatalf("bundled-default path must succeed: %v", err)
	}
	if out.Source != moat.TrustedRootSourceBundled {
		t.Errorf("Source = %q, want %q (bundled default)", out.Source, moat.TrustedRootSourceBundled)
	}
	if out.TrustedRootPath != "" {
		t.Errorf("TrustedRootPath must be empty when bundled default is in use, got %q", out.TrustedRootPath)
	}
}

// TestTrustedRootPathForRegistry — table of resolver scenarios. The
// resolver is what emitTrustedRootPathInfo consults to name the file, so
// CLI-flag > reg.TrustedRoot > "" precedence must hold here too.
func TestTrustedRootPathForRegistry(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		reg      *config.Registry
		override string
		want     string
	}{
		{"both-empty", &config.Registry{}, "", ""},
		{"nil-reg-no-override", nil, "", ""},
		{"nil-reg-with-override", nil, "/tmp/x", "/tmp/x"},
		{"reg-config-only", &config.Registry{TrustedRoot: "/etc/y"}, "", "/etc/y"},
		{"cli-only", &config.Registry{}, "/tmp/cli", "/tmp/cli"},
		{"cli-wins-over-reg", &config.Registry{TrustedRoot: "/etc/y"}, "/tmp/cli", "/tmp/cli"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := trustedRootPathForRegistry(tc.reg, tc.override)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestEmitTrustedRootPathInfo_EmitsOnOverride — the info line is the
// auditor signal for "who chose this trusted root?" It must fire on the
// path-flag source and stay silent on the bundled source (otherwise
// operators would tune out the noise).
func TestEmitTrustedRootPathInfo_EmitsOnOverride(t *testing.T) {
	// Cannot t.Parallel — mutates output package globals.
	_, errBuf := output.SetForTest(t)

	// Bundled source: silent.
	emitTrustedRootPathInfo("reg", moat.TrustedRootSourceBundled, "/ignored")
	if errBuf.Len() != 0 {
		t.Errorf("bundled source must not emit, got %q", errBuf.String())
	}

	// Path source with empty path: defensive silent.
	emitTrustedRootPathInfo("reg", moat.TrustedRootSourcePathFlag, "")
	if errBuf.Len() != 0 {
		t.Errorf("empty path must not emit, got %q", errBuf.String())
	}

	// Path source with path: emits key=value line with both fields.
	emitTrustedRootPathInfo("corp", moat.TrustedRootSourcePathFlag, "/etc/x.json")
	got := errBuf.String()
	if !strings.Contains(got, "moat.trusted_root_path=/etc/x.json") {
		t.Errorf("info line missing key=value: %q", got)
	}
	if !strings.Contains(got, "registry=corp") {
		t.Errorf("info line missing registry label: %q", got)
	}
}

// TestEmitTrustedRootPathInfo_QuietAndJSONSuppress — JSON callers parse
// machine output structurally; free-form stderr would break that contract.
// Quiet mode must also suppress (operator opt-out).
func TestEmitTrustedRootPathInfo_QuietAndJSONSuppress(t *testing.T) {
	// Cannot t.Parallel — mutates output package globals.
	_, errBuf := output.SetForTest(t)

	origQuiet := output.Quiet
	output.Quiet = true
	emitTrustedRootPathInfo("reg", moat.TrustedRootSourcePathFlag, "/etc/x.json")
	output.Quiet = origQuiet
	if errBuf.Len() != 0 {
		t.Errorf("quiet mode must suppress, got %q", errBuf.String())
	}

	origJSON := output.JSON
	output.JSON = true
	emitTrustedRootPathInfo("reg", moat.TrustedRootSourcePathFlag, "/etc/x.json")
	output.JSON = origJSON
	if errBuf.Len() != 0 {
		t.Errorf("JSON mode must suppress, got %q", errBuf.String())
	}
}
