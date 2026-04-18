package moat

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"testing"
)

// TestRekorBody_HashMatchesCanonicalPayload is the spike's end-to-end hash
// anchor: it proves the canonical payload computed by CanonicalPayloadFor
// matches the hash recorded in a real Rekor entry. If this test passes, the
// canonical payload format is correct and the remaining verification work
// (signature, cert chain, OIDC identity, inclusion proof) has a sound base.
//
// The fixture rekor-syllago-guide.json was captured live from
// https://rekor.sigstore.dev/api/v1/log/entries?logIndex=1336116369
// during the syllago-9jzgr spike — signed by the Publisher Action during
// syllago-meta-registry Phase 0 bootstrap.
func TestRekorBody_HashMatchesCanonicalPayload(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("testdata/rekor-syllago-guide.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	entry, err := parseRekorEntry(raw)
	if err != nil {
		t.Fatalf("parsing Rekor response: %v", err)
	}
	if entry.LogIndex != 1336116369 {
		t.Errorf("expected logIndex=1336116369, got %d", entry.LogIndex)
	}

	body, err := decodeHashedRekordBody(entry.Body)
	if err != nil {
		t.Fatalf("decoding hashedrekord body: %v", err)
	}
	if body.Spec.Data.Hash.Algorithm != "sha256" {
		t.Errorf("expected algorithm=sha256, got %q", body.Spec.Data.Hash.Algorithm)
	}

	const fixtureHash = "sha256:f997b299344032fb6f12c80b86dffad33a1ad2ec0c23bd2476ce3d4c8781a6f2"
	payload := CanonicalPayloadFor(fixtureHash)
	sum := sha256.Sum256(payload)
	expected := hex.EncodeToString(sum[:])

	if body.Spec.Data.Hash.Value != expected {
		t.Errorf("canonical payload hash does not match Rekor record\n"+
			"  computed sha256(payload for %s) = %s\n"+
			"  rekor recorded value              = %s",
			fixtureHash, expected, body.Spec.Data.Hash.Value)
	}
}

// TestDecodeHashedRekordBody_RejectsWrongKind guards the spec-compliance check
// — if Rekor ever returned a non-hashedrekord entry at our logIndex (misuse,
// protocol change), decoding must fail loudly rather than produce zero values.
func TestDecodeHashedRekordBody_RejectsWrongKind(t *testing.T) {
	t.Parallel()

	// base64 of a different rekor kind
	body := encodeBody(t, `{"apiVersion":"0.0.1","kind":"intoto","spec":{}}`)
	if _, err := decodeHashedRekordBody(body); err == nil {
		t.Fatal("expected error for kind=intoto, got nil")
	}

	body = encodeBody(t, `{"apiVersion":"0.0.2","kind":"hashedrekord","spec":{}}`)
	if _, err := decodeHashedRekordBody(body); err == nil {
		t.Fatal("expected error for apiVersion=0.0.2, got nil")
	}
}

func encodeBody(t *testing.T, raw string) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString([]byte(raw))
}
