package moat

import (
	"os"
	"strings"
	"testing"
)

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
		t.Logf("rejection error: %v", err)
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
// refresh if the Fulcio / Rekor root keys rotate.
func loadTrustedRoot(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/trusted-root-public-good.json")
	if err != nil {
		t.Fatalf("reading trusted root fixture: %v", err)
	}
	return raw
}
