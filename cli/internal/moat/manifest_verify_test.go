package moat

// Integration tests for VerifyManifest against the real syllago-meta-registry
// Phase 0 Publisher Action fixture. The raw Rekor JSON response captured in
// testdata/rekor-syllago-guide.json is re-assembled into a v0.3 .sigstore
// bundle via BuildBundle (test-only per ADR 0007) and fed to the production
// VerifyManifest primitive. Production callers receive .sigstore bundles
// directly from publishers — the test just synthesizes the same shape.

import (
	"errors"
	"strings"
	"testing"

	sgbundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
)

// buildFixtureBundleBytes assembles a v0.3 .sigstore bundle from the Phase 0
// Rekor fixture and returns (artifactBytes, bundleBytes). artifactBytes is
// the canonical payload the bundle signs over; VerifyManifest's manifestBytes
// parameter expects exactly these bytes.
func buildFixtureBundleBytes(t *testing.T) (artifact, bundleBytes []byte) {
	t.Helper()
	att := loadAttestation(t)
	item := findItem(t, att, "syllago-guide")
	rekorRaw := loadRekorFixture(t)
	payload := CanonicalPayloadFor(item.ContentHash)
	b, err := BuildBundle(rekorRaw, payload)
	if err != nil {
		t.Fatalf("building fixture bundle: %v", err)
	}
	j, err := b.MarshalJSON()
	if err != nil {
		t.Fatalf("marshaling bundle: %v", err)
	}
	return payload, j
}

// extractFixtureNumericIDs parses the fixture bundle's leaf cert and returns
// the GitHub OIDC numeric IDs. Tests use this to pin the correct IDs (for a
// matching test) or derive wrong IDs (for a mismatch test).
func extractFixtureNumericIDs(t *testing.T, bundleBytes []byte) (repoID, ownerID string) {
	t.Helper()
	b := &sgbundle.Bundle{}
	if err := b.UnmarshalJSON(bundleBytes); err != nil {
		t.Fatalf("re-parsing bundle: %v", err)
	}
	vc, err := b.VerificationContent()
	if err != nil {
		t.Fatalf("getting verification content: %v", err)
	}
	cert := vc.Certificate()
	if cert == nil {
		t.Fatal("bundle had no leaf cert")
	}
	sum, err := certificate.SummarizeCertificate(cert)
	if err != nil {
		t.Fatalf("summarizing cert: %v", err)
	}
	return sum.SourceRepositoryIdentifier, sum.SourceRepositoryOwnerIdentifier
}

// expectVerifyError is a helper that asserts err is a *VerifyError with the
// given code. On mismatch it t.Fatalf's with a clear diagnostic.
func expectVerifyError(t *testing.T, err error, wantCode string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected *VerifyError with code %q, got nil", wantCode)
	}
	var ve *VerifyError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *VerifyError, got %T: %v", err, err)
	}
	if ve.Code != wantCode {
		t.Fatalf("VerifyError code mismatch: got=%q want=%q (err=%v)", ve.Code, wantCode, err)
	}
}

// TestVerifyManifest_HappyPath_TOFU verifies a real Phase 0 signature with a
// pinned profile that does not yet carry numeric IDs (TOFU-capture mode).
// All crypto flags true, NumericIDMatched false (nothing to match against).
func TestVerifyManifest_HappyPath_TOFU(t *testing.T) {
	t.Parallel()
	artifact, bundleBytes := buildFixtureBundleBytes(t)
	profile := expectedProfile()
	trustedRoot := loadTrustedRoot(t)

	result, err := VerifyManifest(artifact, bundleBytes, &profile, trustedRoot)
	if err != nil {
		t.Fatalf("VerifyManifest must succeed on valid fixture: %v", err)
	}
	if !result.SignatureValid || !result.CertificateChainValid || !result.RekorProofValid || !result.IdentityMatches {
		t.Errorf("expected all crypto flags true, got %+v", result)
	}
	if result.NumericIDMatched {
		t.Errorf("expected NumericIDMatched=false in TOFU mode (no pinned IDs), got true")
	}
	if result.RevocationChecked {
		t.Errorf("RevocationChecked must be false in slice 1 (got true)")
	}
}

// TestVerifyManifest_HappyPath_NumericIDsMatched pins the exact numeric IDs
// from the fixture cert and confirms NumericIDMatched flips to true.
func TestVerifyManifest_HappyPath_NumericIDsMatched(t *testing.T) {
	t.Parallel()
	artifact, bundleBytes := buildFixtureBundleBytes(t)
	repoID, ownerID := extractFixtureNumericIDs(t, bundleBytes)
	if repoID == "" || ownerID == "" {
		t.Fatalf("fixture cert must carry numeric IDs; got repoID=%q ownerID=%q", repoID, ownerID)
	}

	profile := expectedProfile()
	profile.RepositoryID = repoID
	profile.RepositoryOwnerID = ownerID
	trustedRoot := loadTrustedRoot(t)

	result, err := VerifyManifest(artifact, bundleBytes, &profile, trustedRoot)
	if err != nil {
		t.Fatalf("VerifyManifest must succeed with correct numeric IDs: %v", err)
	}
	if !result.NumericIDMatched {
		t.Errorf("expected NumericIDMatched=true when both IDs pinned correctly")
	}
}

// TestVerifyManifest_NumericIDMismatch pins a wrong numeric repository_id and
// asserts the verifier hard-fails with MOAT_IDENTITY_MISMATCH even though
// every other signal (SAN, issuer, chain, Rekor) is correct. This is the
// repo-transfer-forgery test.
func TestVerifyManifest_NumericIDMismatch(t *testing.T) {
	t.Parallel()
	artifact, bundleBytes := buildFixtureBundleBytes(t)
	_, ownerID := extractFixtureNumericIDs(t, bundleBytes)

	profile := expectedProfile()
	profile.RepositoryID = "999999999" // wrong
	profile.RepositoryOwnerID = ownerID
	trustedRoot := loadTrustedRoot(t)

	_, err := VerifyManifest(artifact, bundleBytes, &profile, trustedRoot)
	expectVerifyError(t, err, CodeIdentityMismatch)
	if !strings.Contains(err.Error(), "repository_id") {
		t.Errorf("expected error to name repository_id, got: %v", err)
	}
}

// TestVerifyManifest_UnpinnedProfile — nil profile must hard-fail with
// MOAT_IDENTITY_UNPINNED. Refuses to verify without an operator-approved
// identity; implicit trust is the attack surface.
func TestVerifyManifest_UnpinnedProfile(t *testing.T) {
	t.Parallel()
	artifact, bundleBytes := buildFixtureBundleBytes(t)
	trustedRoot := loadTrustedRoot(t)

	_, err := VerifyManifest(artifact, bundleBytes, nil, trustedRoot)
	expectVerifyError(t, err, CodeIdentityUnpinned)
}

// TestVerifyManifest_EmptyTrustedRoot — zero-length trusted root must fail
// with MOAT_TRUSTED_ROOT_MISSING. Distinguishes "caller forgot to load the
// bundled root" from "trusted root JSON is corrupt".
func TestVerifyManifest_EmptyTrustedRoot(t *testing.T) {
	t.Parallel()
	artifact, bundleBytes := buildFixtureBundleBytes(t)
	profile := expectedProfile()

	_, err := VerifyManifest(artifact, bundleBytes, &profile, nil)
	expectVerifyError(t, err, CodeTrustedRootMissing)
}

// TestVerifyManifest_CorruptTrustedRoot — malformed trusted root JSON must
// fail with MOAT_TRUSTED_ROOT_CORRUPT. Operators with broken corporate
// trust roots get a distinct code for structured logging.
func TestVerifyManifest_CorruptTrustedRoot(t *testing.T) {
	t.Parallel()
	artifact, bundleBytes := buildFixtureBundleBytes(t)
	profile := expectedProfile()

	_, err := VerifyManifest(artifact, bundleBytes, &profile, []byte("{not json"))
	expectVerifyError(t, err, CodeTrustedRootCorrupt)
}

// TestVerifyManifest_EmptyBundle — zero-length bundle must surface as
// MOAT_UNSIGNED (the operational label for "no signature at all"). Do NOT
// let this collapse into MOAT_INVALID; callers render these differently.
func TestVerifyManifest_EmptyBundle(t *testing.T) {
	t.Parallel()
	artifact, _ := buildFixtureBundleBytes(t)
	profile := expectedProfile()
	trustedRoot := loadTrustedRoot(t)

	_, err := VerifyManifest(artifact, nil, &profile, trustedRoot)
	expectVerifyError(t, err, CodeUnsigned)
}

// TestVerifyManifest_EmptyManifest — zero-length artifact bytes must fail
// with MOAT_INVALID. The manifest parameter is mandatory and there is no
// sensible "default artifact" that could hide a misuse.
func TestVerifyManifest_EmptyManifest(t *testing.T) {
	t.Parallel()
	_, bundleBytes := buildFixtureBundleBytes(t)
	profile := expectedProfile()
	trustedRoot := loadTrustedRoot(t)

	_, err := VerifyManifest(nil, bundleBytes, &profile, trustedRoot)
	expectVerifyError(t, err, CodeInvalid)
}

// TestVerifyManifest_MalformedBundle — garbage bytes must fail bundle
// parsing and surface as MOAT_INVALID. Covers the case where a registry
// mirror corrupts the .sigstore file (truncation, wrong content-type, etc.)
func TestVerifyManifest_MalformedBundle(t *testing.T) {
	t.Parallel()
	artifact, _ := buildFixtureBundleBytes(t)
	profile := expectedProfile()
	trustedRoot := loadTrustedRoot(t)

	_, err := VerifyManifest(artifact, []byte("not a sigstore bundle"), &profile, trustedRoot)
	expectVerifyError(t, err, CodeInvalid)
}

// TestVerifyManifest_WrongSubject — pinned profile with an attacker-chosen
// subject must hard-fail. This is the baseline SAN match sigstore-go enforces
// via the certificate identity policy.
func TestVerifyManifest_WrongSubject(t *testing.T) {
	t.Parallel()
	artifact, bundleBytes := buildFixtureBundleBytes(t)
	profile := SigningProfile{
		Issuer:  expectedProfile().Issuer,
		Subject: "https://github.com/attacker/fake-repo/.github/workflows/evil.yml@refs/heads/main",
	}
	trustedRoot := loadTrustedRoot(t)

	_, err := VerifyManifest(artifact, bundleBytes, &profile, trustedRoot)
	if err == nil {
		t.Fatal("VerifyManifest must reject wrong subject; got nil error")
	}
	// sigstore-go's error taxonomy is not stable; we accept either
	// MOAT_IDENTITY_MISMATCH (our heuristic caught it) or MOAT_INVALID
	// (sigstore-go reported something we couldn't classify as identity).
	var ve *VerifyError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *VerifyError, got %T: %v", err, err)
	}
	if ve.Code != CodeIdentityMismatch && ve.Code != CodeInvalid {
		t.Errorf("expected %s or %s, got %s (err=%v)",
			CodeIdentityMismatch, CodeInvalid, ve.Code, err)
	}
}

// TestVerifyManifest_TamperedArtifact — swapping the manifest bytes for
// different content must fail the digest comparison the sigstore-go verifier
// runs via WithArtifact. MOAT_INVALID surfaces because the signature no
// longer covers the supplied bytes.
func TestVerifyManifest_TamperedArtifact(t *testing.T) {
	t.Parallel()
	_, bundleBytes := buildFixtureBundleBytes(t)
	profile := expectedProfile()
	trustedRoot := loadTrustedRoot(t)

	// Valid JSON, valid UTF-8 — just not what the bundle signs over.
	tampered := []byte(`{"_version":1,"content_hash":"sha256:` + zeros64 + `"}`)

	_, err := VerifyManifest(tampered, bundleBytes, &profile, trustedRoot)
	expectVerifyError(t, err, CodeInvalid)
}

// TestVerifyManifest_WrongTrustAnchor — valid-shape trusted root JSON with
// no Fulcio CAs and no Rekor keys must fail verification. This is the
// reduced-form "rotated-key" test: a trust anchor that doesn't contain the
// material needed to validate the bundle. The full rotated-key test (bundle
// signed against v1 keys, verified against v2 trusted root) needs a second
// captured trust root and is deferred to slice 2+ per ADR 0007.
//
// Slice-1 error taxonomy: either CodeTrustedRootCorrupt (sigstore-go rejects
// the minimal JSON at parse time) or CodeInvalid (parse succeeds, verify
// fails with no matching key). Both are acceptable; the test accepts either
// to avoid coupling to sigstore-go's internal parse/verify boundary.
func TestVerifyManifest_WrongTrustAnchor(t *testing.T) {
	t.Parallel()
	artifact, bundleBytes := buildFixtureBundleBytes(t)
	profile := expectedProfile()

	// Minimal trusted root payload: valid JSON + the mediaType sigstore-go
	// inspects, but no CA/log material. Serves as a stand-in for "trusted
	// root that doesn't cover the key material in the bundle."
	empty := []byte(`{
		"mediaType": "application/vnd.dev.sigstore.trustedroot+json;version=0.1",
		"tlogs": [],
		"certificateAuthorities": [],
		"ctlogs": [],
		"timestampAuthorities": []
	}`)

	_, err := VerifyManifest(artifact, bundleBytes, &profile, empty)
	if err == nil {
		t.Fatal("VerifyManifest must reject a bundle whose key material is absent from the trusted root")
	}
	var ve *VerifyError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *VerifyError, got %T: %v", err, err)
	}
	if ve.Code != CodeInvalid && ve.Code != CodeTrustedRootCorrupt {
		t.Errorf("expected %s or %s, got %s (err=%v)",
			CodeInvalid, CodeTrustedRootCorrupt, ve.Code, err)
	}
}
