package moat

// Shared test helpers for the moat package. The fixtures were captured from
// the live syllago-meta-registry Phase 0 Publisher Action output and anchor
// the item-verification tests (item_verify_test.go, sigstore_verify_test.go,
// item_verify_shard_index_test.go).

import (
	"encoding/json"
	"os"
	"testing"
)

const zeros64 = "0000000000000000000000000000000000000000000000000000000000000000"

func loadAttestation(t *testing.T) Attestation {
	t.Helper()
	raw, err := os.ReadFile("testdata/moat-attestation.json")
	if err != nil {
		t.Fatalf("reading attestation fixture: %v", err)
	}
	var att Attestation
	if err := json.Unmarshal(raw, &att); err != nil {
		t.Fatalf("parsing attestation: %v", err)
	}
	return att
}

func findItem(t *testing.T, att Attestation, name string) AttestationItem {
	t.Helper()
	for _, i := range att.Items {
		if i.Name == name {
			return i
		}
	}
	t.Fatalf("item %q not found in attestation (items=%d)", name, len(att.Items))
	return AttestationItem{}
}

func loadRekorFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/rekor-syllago-guide.json")
	if err != nil {
		t.Fatalf("reading Rekor fixture: %v", err)
	}
	return raw
}

func expectedProfile() SigningProfile {
	return SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "https://github.com/OpenScribbler/syllago-meta-registry/.github/workflows/moat.yml@refs/heads/master",
	}
}
