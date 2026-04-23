package main

// Registry manifest verification gate for `syllago add --from <registry>`
// (ADR 0007 slice-2c). This is where the slice-1 VerifyManifest primitive
// finally sees real user traffic.
//
// The gate sits at the boundary where registry content transitions into the
// user's library. Verifying here means every path that reads from a
// MOAT-pinned registry — `add`, loadout install, TUI import — picks up the
// same policy.
//
// Precedence, in order:
//  1. Registry has no pinned SigningProfile AND Type != MOAT
//     → legacy unsigned mode (back-compat with pre-MOAT configs).
//  2. Registry has a pinned SigningProfile:
//     a. manifest.json + manifest.json.sigstore missing → MOAT_006.
//     b. bundled trusted root expired (>365d) → MOAT_005.
//     c. ProfileVersion unknown → MOAT_004 (wrapping MOAT_INVALID).
//     d. VerifyManifest returns MOAT_IDENTITY_MISMATCH → MOAT_003.
//     e. VerifyManifest returns any other *VerifyError → MOAT_004.
//     f. Success → emit "signed" trust label, proceed.
//
// Operational vocabulary (per ADR 0007): three-state trust label is
// signed / unsigned / invalid. The word "verified" is reserved for when
// revocation checking lands in slice 3.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// Canonical filenames for the manifest and its sigstore bundle inside a
// registry checkout. Convention-over-configuration keeps slice-2c
// implementations simple; slice 3+ adds manifest_uri override support for
// registries that serve the manifest from a separate URL.
const (
	manifestFileName = "manifest.json"
	bundleFileName   = "manifest.json.sigstore"
)

// trustLabel is the slice-1 three-state operational label. Intentionally
// NOT "verified" — that string is reserved for the revocation-checked
// state in slice 3.
type trustLabel string

const (
	trustSigned   trustLabel = "signed"
	trustUnsigned trustLabel = "unsigned"
	trustInvalid  trustLabel = "invalid"
)

// verifyOutcome captures what the registry-verify gate produced. Passed to
// the caller so it can emit the trust label in output.
type verifyOutcome struct {
	Label            trustLabel
	Source           moat.TrustedRootSource
	TrustedRootPath  string // populated only when Source is path (override in use)
	ProfileVersion   int
	ResultSummary    string // one-line human summary for stdout
	NumericIDMatched bool
}

// verifyManifestFn is the indirection point for tests. Production callers
// use moat.VerifyManifest directly.
var verifyManifestFn = moat.VerifyManifest

// verifyTrustedRootFn returns the trusted-root info to use for a given
// registry, CLI override, and wall-clock. Precedence per ADR 0007 slice 2d:
//  1. override != "" (the --trusted-root CLI flag) wins over everything.
//  2. reg.TrustedRoot is used if set.
//  3. Fall back to the bundled root.
//
// An error return means the operator's override path is unusable — callers
// must surface MOAT_007 rather than silently fall back to the bundled root,
// which would be a trust downgrade the operator did not authorize.
//
// Tests override this to inject canned trusted-root info without touching
// the filesystem or embedded bundle.
var verifyTrustedRootFn = func(reg *config.Registry, override string, now time.Time) (moat.TrustedRootInfo, error) {
	path := override
	if path == "" && reg != nil {
		path = reg.TrustedRoot
	}
	if path == "" {
		return moat.BundledTrustedRoot(now), nil
	}
	return moat.TrustedRootFromPath(path, now)
}

// trustedRootPathForRegistry returns the resolved path the verifier will
// load when called with (reg, override) — used by emitters to name the file
// in operator-visible output. Kept separate from verifyTrustedRootFn so
// tests that swap the loader don't lose the path-reporting signal.
func trustedRootPathForRegistry(reg *config.Registry, override string) string {
	if override != "" {
		return override
	}
	if reg != nil {
		return reg.TrustedRoot
	}
	return ""
}

// verifyRegistryForAdd is the main entry point called from runAddFromRegistry.
// Returns (nil, nil) for unsigned registries where slice-1 back-compat applies
// (caller proceeds silently). Returns a populated outcome for MOAT registries
// that passed verification. Returns a *StructuredError for verification
// failures — callers should return that error straight up the stack.
//
// trustedRootOverride is the optional --trusted-root CLI flag. Empty means
// "consult reg.TrustedRoot; fall back to bundled."
func verifyRegistryForAdd(reg *config.Registry, cloneDir string, trustedRootOverride string) (*verifyOutcome, error) {
	if reg == nil {
		return nil, nil
	}

	// Legacy unsigned path: no pinned profile AND not explicitly MOAT.
	// This preserves back-compat for git registries added before slice-2b.
	if reg.SigningProfile == nil && !reg.IsMOAT() {
		return nil, nil
	}

	// Pinned profile required for any MOAT verification.
	if reg.SigningProfile == nil {
		return nil, output.NewStructuredErrorDetail(
			output.ErrMoatIdentityUnpinned,
			fmt.Sprintf("registry %q is typed MOAT but has no signing profile on disk", reg.Name),
			"Re-add the registry with --signing-identity or remove and re-add without --moat.",
			"See "+MoatPinningDocsURL+" for the pinning workflow.",
		)
	}

	profile := reg.SigningProfile

	// Reject unknown ProfileVersion before spending any I/O on verification.
	// Only v1 is defined; v2+ is reserved for future issuer additions.
	if profile.ProfileVersion != 0 && profile.ProfileVersion != moat.ProfileVersionV1 {
		return nil, output.NewStructuredErrorDetail(
			output.ErrMoatInvalid,
			fmt.Sprintf("registry %q has unknown signing profile version %d", reg.Name, profile.ProfileVersion),
			"Upgrade syllago to a release that supports this profile version, or re-add the registry with a v1 profile.",
			fmt.Sprintf("Supported profile versions: %d.", moat.ProfileVersionV1),
		)
	}

	// Load the trusted root — CLI override > reg.TrustedRoot > bundled. An
	// error from the loader is MOAT_007 (operator config broken). Silent
	// fall-back to the bundled root would be a trust downgrade.
	rootInfo, rootErr := verifyTrustedRootFn(reg, trustedRootOverride, time.Now())
	if rootErr != nil {
		return nil, output.NewStructuredErrorDetail(
			output.ErrMoatTrustedRootOverride,
			fmt.Sprintf("cannot load operator-supplied trusted root for registry %q", reg.Name),
			"Fix the path or remove it to fall back to the bundled trusted root. See `syllago moat trust status` for what's in effect now.",
			rootErr.Error(),
		)
	}

	// Staleness check applies only to the bundled root — override paths
	// are the operator's responsibility (moat.TrustedRootFromPath always
	// returns Status=Fresh).
	if rootInfo.Status == moat.TrustedRootStatusExpired ||
		rootInfo.Status == moat.TrustedRootStatusMissing ||
		rootInfo.Status == moat.TrustedRootStatusCorrupt {
		return nil, output.NewStructuredErrorDetail(
			output.ErrMoatTrustedRootStale,
			"bundled Sigstore trusted root is expired — cannot verify",
			"Run `syllago update` to pick up a fresh trusted root. See `syllago moat trust status` for details.",
			moat.StalenessMessage(rootInfo),
		)
	}

	// Emit the operator info line for override paths BEFORE reading the
	// manifest. Auditors reading stderr on a verify-fail need to see which
	// root was in effect, even when verification later errors out.
	trustedRootPath := trustedRootPathForRegistry(reg, trustedRootOverride)
	emitTrustedRootPathInfo(reg.Name, rootInfo.Source, trustedRootPath)

	// Load manifest + bundle from the clone directory. Missing files with a
	// pinned profile is MOAT_006 — do not silently fall through to unsigned.
	manifestBytes, bundleBytes, found, loadErr := readManifestFromCheckout(cloneDir)
	if loadErr != nil {
		return nil, output.NewStructuredErrorDetail(
			output.ErrMoatInvalid,
			fmt.Sprintf("cannot read manifest for registry %q", reg.Name),
			"Run `syllago registry sync` to refresh, or inspect the clone at ~/.syllago/registries/"+reg.Name+"/.",
			loadErr.Error(),
		)
	}
	if !found {
		return nil, output.NewStructuredErrorDetail(
			output.ErrMoatUnsignedWithPin,
			fmt.Sprintf("registry %q has a pinned signing profile but no signed manifest in checkout", reg.Name),
			"Run `syllago registry sync` to refresh. If the registry does not publish a MOAT manifest, remove it and re-add without --signing-* flags.",
			fmt.Sprintf("expected %s + %s under %s/", manifestFileName, bundleFileName, cloneDir),
		)
	}

	// Translate config.SigningProfile → moat.SigningProfile for the verifier.
	moatProfile := &moat.SigningProfile{
		Issuer:            profile.Issuer,
		Subject:           profile.Subject,
		ProfileVersion:    moat.ProfileVersionV1,
		SubjectRegex:      profile.SubjectRegex,
		IssuerRegex:       profile.IssuerRegex,
		RepositoryID:      profile.RepositoryID,
		RepositoryOwnerID: profile.RepositoryOwnerID,
	}

	result, err := verifyManifestFn(manifestBytes, bundleBytes, moatProfile, rootInfo.Bytes)
	if err != nil {
		return nil, classifyVerifyError(reg.Name, err)
	}

	summary := fmt.Sprintf(
		"Verified %s manifest — sig ok, identity ok (numeric-id: %s), root: %s.",
		reg.Name, numericIDLabel(result.NumericIDMatched), rootInfo.Source,
	)

	return &verifyOutcome{
		Label:            trustSigned,
		Source:           rootInfo.Source,
		TrustedRootPath:  trustedRootPath,
		ProfileVersion:   moatProfile.EffectiveProfileVersion(),
		ResultSummary:    summary,
		NumericIDMatched: result.NumericIDMatched,
	}, nil
}

// readManifestFromCheckout reads manifest.json and manifest.json.sigstore
// from the registry checkout. Returns found=false when EITHER file is
// missing — both must exist for verification.
func readManifestFromCheckout(cloneDir string) (manifestBytes, bundleBytes []byte, found bool, err error) {
	manifestPath := filepath.Join(cloneDir, manifestFileName)
	bundlePath := filepath.Join(cloneDir, bundleFileName)

	manifestBytes, mErr := os.ReadFile(manifestPath)
	if errors.Is(mErr, os.ErrNotExist) {
		return nil, nil, false, nil
	}
	if mErr != nil {
		return nil, nil, false, fmt.Errorf("reading %s: %w", manifestPath, mErr)
	}

	bundleBytes, bErr := os.ReadFile(bundlePath)
	if errors.Is(bErr, os.ErrNotExist) {
		return nil, nil, false, nil
	}
	if bErr != nil {
		return nil, nil, false, fmt.Errorf("reading %s: %w", bundlePath, bErr)
	}

	return manifestBytes, bundleBytes, true, nil
}

// classifyVerifyError maps a *moat.VerifyError code to the correct syllago
// structured error code. Non-VerifyError values collapse to MOAT_004.
func classifyVerifyError(regName string, err error) error {
	var ve *moat.VerifyError
	if errors.As(err, &ve) {
		switch ve.Code {
		case moat.CodeIdentityMismatch:
			return output.NewStructuredErrorDetail(
				output.ErrMoatIdentityMismatch,
				fmt.Sprintf("manifest cert does not match pinned profile for registry %q", regName),
				"Re-verify the registry's signing identity out-of-band, then re-add with refreshed --signing-repository-id / --signing-repository-owner-id.",
				err.Error(),
			)
		case moat.CodeTrustedRootStale, moat.CodeTrustedRootMissing, moat.CodeTrustedRootCorrupt:
			return output.NewStructuredErrorDetail(
				output.ErrMoatTrustedRootStale,
				fmt.Sprintf("bundled trusted root unusable while verifying registry %q", regName),
				"Run `syllago update` to refresh the bundled Sigstore trusted root.",
				err.Error(),
			)
		case moat.CodeIdentityUnpinned:
			return output.NewStructuredErrorDetail(
				output.ErrMoatIdentityUnpinned,
				fmt.Sprintf("registry %q has no pinned signing profile", regName),
				"Re-add with --signing-identity or request an allowlist entry. See "+MoatPinningDocsURL+".",
				err.Error(),
			)
		}
	}
	return output.NewStructuredErrorDetail(
		output.ErrMoatInvalid,
		fmt.Sprintf("manifest verification failed for registry %q", regName),
		"Run `syllago registry sync` to refresh. If the error persists, the registry re-uploaded the manifest without re-signing — contact the registry operator.",
		err.Error(),
	)
}

// numericIDLabel returns a short human-readable label for the numeric-ID
// state. "matched" when we actively compared against pinned IDs; "not
// pinned" when the profile had no numeric IDs to match.
func numericIDLabel(matched bool) string {
	if matched {
		return "matched"
	}
	return "not pinned"
}

// emitTrustLabel writes the three-state trust label to stdout. Separate from
// the stderr warning path so JSON callers can parse trust state out of
// structured output in slice-3 without string-matching warnings.
func emitTrustLabel(outcome *verifyOutcome, regName string) {
	if outcome == nil || output.Quiet || output.JSON {
		return
	}
	fmt.Fprintf(output.Writer, "  Trust: %s (registry %s, root: %s)\n",
		outcome.Label, regName, outcome.Source)
}

// emitTrustedRootPathInfo writes an auditor-visible marker naming the
// operator-supplied trusted_root.json in effect. Emitted to stderr in the
// stable key=value form specified by ADR 0007 D1, so CI pipelines can grep
// on it. Silent when the bundled root is in use — that's the default case
// and spamming it would train operators to ignore the line.
func emitTrustedRootPathInfo(regName string, source moat.TrustedRootSource, path string) {
	if source != moat.TrustedRootSourcePathFlag || path == "" {
		return
	}
	if output.Quiet || output.JSON {
		return
	}
	fmt.Fprintf(output.ErrWriter, "moat.trusted_root_path=%s (registry=%s)\n", path, regName)
}
