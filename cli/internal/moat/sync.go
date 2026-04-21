package moat

// MOAT registry-sync orchestration (ADR 0007 Phase 2a).
//
// Sync composes the lower-level primitives from Phase 1 into one end-to-end
// flow that a caller (CLI command, installer hook, TUI action) can invoke
// without re-assembling the pieces each time:
//
//   1. Conditional manifest GET (fetch.go) honoring stored ETag.
//   2. Sigstore bundle GET at manifest_uri + ".sigstore".
//   3. Cryptographic verification (manifest_verify.go).
//   4. Signing-profile classification: TOFU / changed / match.
//   5. Staleness classification against the 72h window (freshness.go).
//   6. Archival revocation sync for registry-source entries (lockfile.go).
//
// What Sync DOES NOT do, on purpose:
//   - Persist trust decisions on config.Registry. The TOFU prompt and the
//     re-approval prompt both live in the CLI layer that owns the TTY.
//     Silently promoting a changed signing profile would violate G-18 row 2.
//   - Emit any stdout/stderr output. Callers decide how to surface status.
//   - Make the install/abort decision. The SyncResult carries the flags a
//     caller maps into ExitCodeFor(FailureSigningProfileChange) or a
//     `syllago registry approve` nudge, per context.
//
// Why a single Sync entrypoint instead of leaving callers to stitch the
// primitives themselves: the fetch→verify→parse order is load-bearing, the
// TOFU-vs-pinned branch for VerifyManifest's identity argument is subtle,
// and the lockfile-mutation side-effect has to fire exactly when a sync
// *actually* succeeded (G-9 — a failed fetch MUST NOT reset the clock).
// Centralizing the sequence keeps the install flow, the TUI action, and the
// `registry sync` CLI command all verifying the same invariants.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// BundleURLSuffix is the MOAT spec convention: the sigstore bundle for a
// registry manifest is published at `{manifest_uri}.sigstore`. A separate
// URL (rather than embedding in the manifest JSON) means the bundle can be
// re-signed or re-notarized without re-publishing the manifest bytes — the
// JSON digest stays stable across bundle rotations.
const BundleURLSuffix = ".sigstore"

// syncVerifyFn is the indirection point for tests. Production callers use
// VerifyManifest directly. Tests in this package swap in a stub that
// returns canned VerificationResult / *VerifyError values so the Sync
// flow can be exercised without access to a signing key. Follows the same
// pattern as verifyManifestFn in cli/cmd/syllago/registry_verify.go.
var syncVerifyFn = VerifyManifest

// SyncResult aggregates the observations from a single successful Sync call.
// Paired with a nil error, every field is populated according to the path
// that was taken (NotModified short-circuits most fields; see comments).
type SyncResult struct {
	// ManifestURL is the URL that was fetched — copied from the registry
	// record so callers can include it in logs without re-reading the
	// config.
	ManifestURL string

	// BundleURL is ManifestURL + BundleURLSuffix. Pre-computed so callers
	// logging a verification failure can include the exact URL they'd need
	// to curl to reproduce.
	BundleURL string

	// NotModified is true when the server returned 304 on the manifest GET.
	// In that case Manifest, ManifestBytes, and Verification are zero-valued
	// and no bundle fetch or crypto verification was performed — the
	// previously cached manifest is still authoritative. FetchedAt is still
	// advanced so the 72h staleness clock moves forward.
	NotModified bool

	// ETag is the latest ETag header (200 response) or the previously-stored
	// ETag echoed back on 304. Callers persist this into
	// config.Registry.ManifestETag for the next conditional GET.
	ETag string

	// Manifest is the parsed manifest. nil on NotModified.
	Manifest *Manifest

	// ManifestBytes is the verbatim response body from the manifest GET.
	// Preserved so a caller that wants to re-verify against a different
	// trusted root (e.g. user-supplied --trusted-root) can do so without
	// re-fetching. nil on NotModified.
	ManifestBytes []byte

	// Verification captures what cryptographic checks passed. Zero value on
	// NotModified. On a verification *failure* Sync returns a non-nil error
	// and a partially-populated SyncResult — Verification is zero-valued in
	// that case because VerifyManifest signals failure via error only.
	Verification VerificationResult

	// IncomingProfile is the signing profile observed on the manifest,
	// translated to the config package shape. Callers consume this to:
	//   - persist into config.Registry.SigningProfile after TOFU accept
	//   - diff against the current pinned profile to present a re-approval
	//     prompt (though ProfileChanged already tells them it differs)
	IncomingProfile config.SigningProfile

	// IsTOFU is true when the registry record had no pinned signing profile
	// prior to this sync (SigningProfile nil or IsZero). The caller MUST
	// gate persistence of IncomingProfile on an interactive approval —
	// non-interactive callers MUST treat this as
	// FailureTOFUAcceptance (G-18 row 1). Sync itself takes no action; the
	// verification still ran so callers know the wire claim is
	// crypto-sound, but whether to TRUST it is a human decision.
	IsTOFU bool

	// ProfileChanged is true when an existing pinned profile exists AND the
	// incoming profile does not equal it. Non-interactive callers MUST exit
	// with FailureSigningProfileChange (G-18 row 2). Interactive callers
	// SHOULD prompt for re-approval via `syllago registry approve <name>`.
	ProfileChanged bool

	// Staleness classifies the manifest against the 72h window paired with
	// manifest.Expires, using the caller-supplied `now`. Computed from the
	// successful fetch timestamp on the 200 path, or from the previous
	// fetched_at on NotModified (which has just been bumped to `now`).
	Staleness StalenessStatus

	// RevocationsAdded counts newly-archived registry-source revocations.
	// Zero on NotModified (no new manifest bytes to merge). Publisher-source
	// revocations are intentionally excluded — they are warn-once-per-session
	// per the two-tier contract (ADR 0007 G-8/G-17).
	RevocationsAdded int

	// PrivateContentCount is the number of ContentEntry rows with
	// private_repo == true. Zero on NotModified. Surfaced for the private-
	// content warning that precedes a bulk install (ADR 0007 G-10).
	PrivateContentCount int

	// FetchedAt is the client-clock timestamp recorded for this sync. The
	// lockfile's registries[url].fetched_at has already been updated to this
	// value by the time Sync returns (when the sync succeeded — see Sync's
	// doc for when fetched_at advances).
	FetchedAt time.Time
}

// Sync runs the end-to-end MOAT registry-sync flow described at the top of
// this file. See SyncResult for the exact observations returned.
//
// Parameters:
//   - ctx: cancellation cancels the in-flight fetch(es) promptly.
//   - reg: must be non-nil, reg.IsMOAT() must be true, and reg.ManifestURI
//     must be non-empty. reg.SigningProfile may be nil/zero (first-sync
//     TOFU path).
//   - lf: must be non-nil. Sync mutates lf.Registries and lf.RevokedHashes
//     only when a sync actually succeeds (304 or 200 + verified).
//   - trustedRootJSON: the Sigstore trusted_root.json bytes to use for
//     verification. Typically the bundled default; a future slice may wire
//     reg.TrustedRoot in for per-registry overrides.
//   - fetcher: may be nil; a default Fetcher with DefaultFetchTimeout is
//     constructed in that case. Tests override to inject an httptest URL.
//   - now: supplied explicitly so tests can pin the clock for staleness
//     classification. Real callers pass time.Now().
//
// Error contract:
//   - Programmer errors (nil registry, wrong type, missing URI) return a
//     plain error with no SyncResult side-effects.
//   - Transport errors (DNS, TLS, timeout) return the wrapped error; the
//     lockfile is NOT mutated. Callers SHOULD NOT treat these as MOAT
//     non-interactive failures — they are ordinary I/O errors.
//   - Verification failures propagate the *VerifyError unchanged (so
//     callers can errors.As to inspect MOAT_* codes). The lockfile is NOT
//     mutated on verification failure.
//   - Classification (TOFU / ProfileChanged) is reported via SyncResult
//     flags, NOT via error. Callers map these to NonInteractiveFailure
//     based on their own interactive/non-interactive context.
//
// Side effects (on success only):
//   - lf.Registries[reg.ManifestURI].FetchedAt is set to FetchedAt.
//   - lf.RevokedHashes appended with any new registry-source revocation
//     hashes from the manifest (additive; existing entries never pruned).
//
// Callers persist SyncResult.ETag into reg.ManifestETag themselves — the
// moat package intentionally has no write-access to the config file.
func Sync(
	ctx context.Context,
	reg *config.Registry,
	lf *Lockfile,
	trustedRootJSON []byte,
	fetcher *Fetcher,
	now time.Time,
) (SyncResult, error) {
	if reg == nil {
		return SyncResult{}, errors.New("moat.Sync: registry is nil")
	}
	if lf == nil {
		return SyncResult{}, errors.New("moat.Sync: lockfile is nil")
	}
	if !reg.IsMOAT() {
		return SyncResult{}, fmt.Errorf("moat.Sync: registry type %q is not MOAT", reg.Type)
	}
	if reg.ManifestURI == "" {
		return SyncResult{}, errors.New("moat.Sync: registry manifest_uri is empty")
	}
	if fetcher == nil {
		fetcher = &Fetcher{}
	}

	result := SyncResult{
		ManifestURL: reg.ManifestURI,
		BundleURL:   reg.ManifestURI + BundleURLSuffix,
	}

	fr, err := fetcher.Fetch(ctx, reg.ManifestURI, reg.ManifestETag)
	if err != nil {
		return result, fmt.Errorf("moat.Sync: fetch manifest: %w", err)
	}
	result.ETag = fr.ETag
	result.FetchedAt = fr.FetchedAt

	if fr.NotModified {
		// 304 path: the prior manifest bytes remain authoritative. We do not
		// re-fetch the bundle (nothing changed to re-verify) and we do not
		// run VerifyManifest. Advance the staleness clock so subsequent
		// install calls see a fresh window.
		lf.SetRegistryFetchedAt(reg.ManifestURI, fr.FetchedAt)
		result.NotModified = true
		result.Staleness = CheckStaleness(fr.FetchedAt, nil, now)
		return result, nil
	}

	result.ManifestBytes = fr.Bytes
	result.Manifest = fr.Manifest

	bundleBytes, err := fetchBundleBytes(ctx, fetcher, result.BundleURL)
	if err != nil {
		return result, fmt.Errorf("moat.Sync: fetch bundle: %w", err)
	}

	pinned := pinnedProfileForVerify(reg, result.Manifest)
	verification, err := syncVerifyFn(fr.Bytes, bundleBytes, pinned, trustedRootJSON)
	if err != nil {
		return result, err
	}
	result.Verification = verification

	incoming := config.SigningProfile{
		Issuer:            result.Manifest.RegistrySigningProfile.Issuer,
		Subject:           result.Manifest.RegistrySigningProfile.Subject,
		ProfileVersion:    result.Manifest.RegistrySigningProfile.ProfileVersion,
		SubjectRegex:      result.Manifest.RegistrySigningProfile.SubjectRegex,
		IssuerRegex:       result.Manifest.RegistrySigningProfile.IssuerRegex,
		RepositoryID:      result.Manifest.RegistrySigningProfile.RepositoryID,
		RepositoryOwnerID: result.Manifest.RegistrySigningProfile.RepositoryOwnerID,
	}
	result.IncomingProfile = incoming

	switch {
	case reg.SigningProfile == nil || reg.SigningProfile.IsZero():
		result.IsTOFU = true
	case reg.NeedsSigningProfileReapproval(incoming):
		result.ProfileChanged = true
	}

	result.Staleness = CheckStaleness(result.FetchedAt, result.Manifest.Expires, now)
	result.RevocationsAdded = lf.SyncRegistryRevocations(result.Manifest)

	for i := range result.Manifest.Content {
		if result.Manifest.Content[i].PrivateRepo {
			result.PrivateContentCount++
		}
	}

	lf.SetRegistryFetchedAt(reg.ManifestURI, result.FetchedAt)

	return result, nil
}

// pinnedProfileForVerify returns the SigningProfile argument for
// VerifyManifest. Two paths:
//
//   - Registry has a pinned profile (normal sync): translate the stored
//     config.SigningProfile into the moat package shape. VerifyManifest
//     uses this to enforce cert-identity match AND numeric-ID match
//     (GitHub issuer).
//
//   - Registry has no pinned profile (first sync after `registry add`):
//     use the wire profile so VerifyManifest will still run and we get the
//     SignatureValid / CertificateChainValid / RekorProofValid signals.
//     The cert-identity check is effectively self-matching here — the
//     cert claims to be X and the profile we pass claims X — so it always
//     passes for a well-formed bundle. That is NOT a security hole: the
//     caller is expected to show IncomingProfile to the user and obtain
//     TOFU approval BEFORE persisting, and non-interactive callers must
//     exit with FailureTOFUAcceptance per G-18. The alternative (skip
//     verification on TOFU) would leave a crypto gap — a malformed or
//     wrong-issuer bundle would slip through the first sync and only be
//     caught on the second sync when a profile exists to compare against.
func pinnedProfileForVerify(reg *config.Registry, m *Manifest) *SigningProfile {
	if reg.SigningProfile != nil && !reg.SigningProfile.IsZero() {
		return &SigningProfile{
			Issuer:            reg.SigningProfile.Issuer,
			Subject:           reg.SigningProfile.Subject,
			ProfileVersion:    reg.SigningProfile.ProfileVersion,
			SubjectRegex:      reg.SigningProfile.SubjectRegex,
			IssuerRegex:       reg.SigningProfile.IssuerRegex,
			RepositoryID:      reg.SigningProfile.RepositoryID,
			RepositoryOwnerID: reg.SigningProfile.RepositoryOwnerID,
		}
	}
	p := m.RegistrySigningProfile
	return &p
}

// fetchBundleBytes GETs the bundle URL and returns the raw response body
// subject to MaxManifestBytes. Kept as a private helper rather than a
// method on Fetcher because bundles have distinct semantics from manifests
// (no ETag, no JSON parse) and exposing a general "fetch bytes" API on
// Fetcher would invite misuse.
//
// Separate from Fetcher.Fetch because that function assumes the body is a
// manifest JSON and invokes ParseManifest — wrapping would either require
// a branch on URL or bypassing validation, neither of which is clean.
func fetchBundleBytes(ctx context.Context, f *Fetcher, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", f.userAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := f.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return nil, fmt.Errorf("unexpected status %d %s",
			resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxManifestBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	if len(body) > MaxManifestBytes {
		return nil, fmt.Errorf("bundle exceeds %d-byte cap", MaxManifestBytes)
	}
	return body, nil
}
