package moat

import (
	"os"
	"strings"
	"testing"
	"time"
)

// trustedRootCapturedAt is the date testdata/trusted-root-public-good.json
// was copied from sigstore-go. Bump it every time you refresh the fixture
// and record the source commit in the comment. The TestTrustedRootFixture_
// FreshnessWindow guard uses this to fail loudly once the fixture drifts
// beyond trustedRootMaxAge, which is how we avoid silently passing sigstore
// tests against stale Fulcio / Rekor root material.
//
// Source: sigstore-go v1.1.4 examples/trusted-root.json.
var trustedRootCapturedAt = time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)

// trustedRootMaxAge is the upper bound beyond which the fixture is
// considered stale enough to fail tests. 90 days is long enough to avoid
// false alarms from normal sprint cadence but short enough to catch real
// Fulcio/Rekor key rotations (they rotate on the order of years, but slow
// drift in chain material is the exact class of bug this guard catches).
const trustedRootMaxAge = 90 * 24 * time.Hour

// TestBuildBundle_FromRekorFixture proves our translation from a raw Rekor
// API response to a sigstore-go Bundle is structurally sound. If
// sgbundle.NewBundle succeeds, the bundle's content + verification material
// oneofs are well-formed — which is the precondition for every downstream
// sigstore-go verify call.
//
// This does NOT perform trust verification; see
// TestVerifyItemSigstore_HappyPath for the full chain.
func TestBuildBundle_FromRekorFixture(t *testing.T) {
	t.Parallel()

	att := loadAttestation(t)
	item := findItem(t, att, "syllago-guide")
	rekorRaw := loadRekorFixture(t)
	payload := CanonicalPayloadFor(item.ContentHash)

	b, err := BuildBundle(rekorRaw, payload)
	if err != nil {
		t.Fatalf("BuildBundle must succeed on valid fixture: %v", err)
	}
	if b == nil {
		t.Fatal("BuildBundle returned nil bundle with no error")
	}
}

// TestVerifyItemSigstore_HappyPath is the spike's end-to-end acceptance
// criterion: one real Rekor entry from the meta-registry Phase 0 Publisher
// Action output verifies successfully via sigstore-go using the public-good
// trusted root.
//
// If this passes, the production verifier's design is validated — we know:
//
//  1. Our canonical payload matches what Rekor recorded.
//  2. The Fulcio leaf certificate chains back to the trusted root.
//  3. The Rekor SET and inclusion proof verify with the bundled log key.
//  4. The expected OIDC identity (issuer + workflow URI) matches the cert SAN.
//
// Using WithIntegratedTimestamps(1) means no separate RFC3161 timestamp is
// required — the Rekor integrated time alone is the observer timestamp.
func TestVerifyItemSigstore_HappyPath(t *testing.T) {
	t.Parallel()

	att := loadAttestation(t)
	item := findItem(t, att, "syllago-guide")
	rekorRaw := loadRekorFixture(t)
	profile := expectedProfile()
	trustedRootJSON := loadTrustedRoot(t)

	if err := VerifyItemSigstore(item, profile, rekorRaw, trustedRootJSON); err != nil {
		t.Fatalf("VerifyItemSigstore must succeed on valid fixtures: %v", err)
	}
}

// TestVerifyItemSigstore_RejectsWrongIdentity ensures the OIDC identity
// policy is actually being enforced. If we swap the expected subject for a
// fake workflow URI, sigstore-go must reject the bundle with a certificate
// identity mismatch — not silently succeed.
func TestVerifyItemSigstore_RejectsWrongIdentity(t *testing.T) {
	t.Parallel()

	att := loadAttestation(t)
	item := findItem(t, att, "syllago-guide")
	rekorRaw := loadRekorFixture(t)
	trustedRootJSON := loadTrustedRoot(t)

	wrongProfile := SigningProfile{
		Issuer:  expectedProfile().Issuer,
		Subject: "https://github.com/attacker/fake-repo/.github/workflows/evil.yml@refs/heads/main",
	}

	err := VerifyItemSigstore(item, wrongProfile, rekorRaw, trustedRootJSON)
	if err == nil {
		t.Fatal("VerifyItemSigstore must reject wrong identity, got nil error")
	}
	if !strings.Contains(err.Error(), "certificate") && !strings.Contains(err.Error(), "identity") && !strings.Contains(err.Error(), "SAN") {
		t.Errorf("rejection error must mention certificate, identity, or SAN — otherwise we cannot confirm OIDC identity policy was enforced for the right reason; got: %v", err)
	}
}

// TestVerifyItemSigstore_RejectsTamperedPayload ensures the digest carried
// in the MessageSignature is actually checked. If we construct a bundle
// with a content_hash that doesn't match the one the signature covers,
// verification must fail — otherwise the whole integrity guarantee is gone.
func TestVerifyItemSigstore_RejectsTamperedPayload(t *testing.T) {
	t.Parallel()

	att := loadAttestation(t)
	item := findItem(t, att, "syllago-guide")
	rekorRaw := loadRekorFixture(t)
	trustedRootJSON := loadTrustedRoot(t)

	tampered := item
	tampered.ContentHash = "sha256:" + zeros64

	err := VerifyItemSigstore(tampered, expectedProfile(), rekorRaw, trustedRootJSON)
	if err == nil {
		t.Fatal("VerifyItemSigstore must reject tampered payload, got nil error")
	}
}

// loadTrustedRoot reads the public-good trusted_root.json bundled in
// testdata. Copied verbatim from sigstore-go@v1.1.4/examples on 2026-04-17;
// refresh if the Fulcio / Rekor root keys rotate. TestTrustedRootFixture_
// FreshnessWindow enforces the refresh cadence — if this test fails, bump
// trustedRootCapturedAt and re-copy the fixture from the upstream source.
func loadTrustedRoot(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/trusted-root-public-good.json")
	if err != nil {
		t.Fatalf("reading trusted root fixture: %v", err)
	}
	return raw
}

// TestTrustedRootFixture_FreshnessWindow fails once the bundled public-good
// trusted_root.json has drifted further than trustedRootMaxAge from the
// recorded capture date. Without this guard, Fulcio/Rekor chain material
// could rotate and every downstream sigstore test would keep passing
// against stale keys — giving false confidence in MOAT verification.
//
// To resolve a failure here:
//  1. Copy the latest testdata/trusted-root-public-good.json from the
//     current sigstore-go release (examples/trusted-root.json or the
//     equivalent path — check the release notes).
//  2. Bump trustedRootCapturedAt in this file to the capture date.
//  3. Re-run this test; it must pass on the new date.
//
// If the refresh reveals that the keys rotated, the sigstore_verify tests
// may also need fixture updates (the bundle in testdata/syllago-guide.* was
// signed against the old chain). In that case, expect cascading updates
// and plan a full MOAT fixture regeneration.
func TestTrustedRootFixture_FreshnessWindow(t *testing.T) {
	t.Parallel()
	age := time.Since(trustedRootCapturedAt)
	if age > trustedRootMaxAge {
		ageDays := int(age.Hours() / 24)
		maxDays := int(trustedRootMaxAge.Hours() / 24)
		t.Errorf("trusted-root-public-good.json is stale: captured %s (%d days ago, max %d days) — refresh from sigstore-go and bump trustedRootCapturedAt",
			trustedRootCapturedAt.Format("2006-01-02"), ageDays, maxDays)
	}
}
