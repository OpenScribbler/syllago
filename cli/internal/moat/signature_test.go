package moat

import (
	"os"
	"testing"
)

// TestVerifySignature_FromFixture is the spike's cryptographic proof point:
// given the Rekor-fetched cert + signature + our reconstructed canonical
// payload, ECDSA verification succeeds. This demonstrates that the Publisher
// Action's signature covers exactly the byte sequence CanonicalPayloadFor
// produces — the complete sign/verify loop works offline.
func TestVerifySignature_FromFixture(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("testdata/rekor-syllago-guide.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	entry, err := parseRekorEntry(raw)
	if err != nil {
		t.Fatalf("parsing Rekor response: %v", err)
	}
	body, err := decodeHashedRekordBody(entry.Body)
	if err != nil {
		t.Fatalf("decoding hashedrekord body: %v", err)
	}
	cert, err := extractCert(body)
	if err != nil {
		t.Fatalf("extracting cert: %v", err)
	}

	const fixtureHash = "sha256:f997b299344032fb6f12c80b86dffad33a1ad2ec0c23bd2476ce3d4c8781a6f2"
	payload := CanonicalPayloadFor(fixtureHash)

	if err := verifySignature(cert, body, payload); err != nil {
		t.Fatalf("signature must verify against canonical payload: %v", err)
	}
}

// TestVerifySignature_RejectsTamperedPayload guards the negative path: if an
// attacker swaps in a different payload, verification must fail. Without
// this, a verify that always succeeds (e.g., if we silently fell back to a
// no-op) would pass the positive test while being useless.
func TestVerifySignature_RejectsTamperedPayload(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("testdata/rekor-syllago-guide.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	entry, err := parseRekorEntry(raw)
	if err != nil {
		t.Fatalf("parsing Rekor response: %v", err)
	}
	body, err := decodeHashedRekordBody(entry.Body)
	if err != nil {
		t.Fatalf("decoding hashedrekord body: %v", err)
	}
	cert, err := extractCert(body)
	if err != nil {
		t.Fatalf("extracting cert: %v", err)
	}

	tampered := []byte(`{"_version":1,"content_hash":"sha256:0000000000000000000000000000000000000000000000000000000000000000"}`)
	if err := verifySignature(cert, body, tampered); err == nil {
		t.Fatal("signature must not verify against a tampered payload")
	}
}
