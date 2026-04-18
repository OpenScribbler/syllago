package moat

import (
	"encoding/json"
	"os"
	"testing"
)

// TestVerifyItem_HappyPath is the spike's integration test: using two real
// fixtures (moat-attestation.json + rekor-syllago-guide.json) captured from
// the live syllago-meta-registry Phase 0 Publisher Action output, the
// orchestration function VerifyItem succeeds.
//
// This demonstrates the full offline verification path — payload, hash,
// signature, and OIDC identity all check out against the expected
// SigningProfile. Phase 2 layers sigstore-go on top for Fulcio chain and
// Rekor inclusion-proof validation.
func TestVerifyItem_HappyPath(t *testing.T) {
	t.Parallel()

	att := loadAttestation(t)
	item := findItem(t, att, "syllago-guide")
	rekorRaw := loadRekorFixture(t)
	profile := expectedProfile()

	if err := VerifyItem(item, profile, rekorRaw); err != nil {
		t.Fatalf("VerifyItem must succeed on valid fixtures: %v", err)
	}
}

// TestVerifyItem_NegativePaths covers the single orchestration surface with
// adversarial inputs. Each case mutates exactly one piece of the verification
// input and asserts the corresponding error. If any of these slipped through,
// the production verifier would accept signatures it should reject.
func TestVerifyItem_NegativePaths(t *testing.T) {
	t.Parallel()

	att := loadAttestation(t)
	base := findItem(t, att, "syllago-guide")
	rekorRaw := loadRekorFixture(t)
	profile := expectedProfile()

	tests := []struct {
		name  string
		item  AttestationItem
		prof  SigningProfile
		rekor []byte
	}{
		{
			name:  "wrong_log_index",
			item:  mutateItem(base, func(i *AttestationItem) { i.RekorLogIndex++ }),
			prof:  profile,
			rekor: rekorRaw,
		},
		{
			name:  "wrong_content_hash",
			item:  mutateItem(base, func(i *AttestationItem) { i.ContentHash = "sha256:" + zeros64 }),
			prof:  profile,
			rekor: rekorRaw,
		},
		{
			name:  "wrong_issuer",
			item:  base,
			prof:  SigningProfile{Issuer: "https://evil.example.com", Subject: profile.Subject},
			rekor: rekorRaw,
		},
		{
			name:  "wrong_subject",
			item:  base,
			prof:  SigningProfile{Issuer: profile.Issuer, Subject: "https://github.com/attacker/repo/.github/workflows/evil.yml@refs/heads/main"},
			rekor: rekorRaw,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := VerifyItem(tt.item, tt.prof, tt.rekor); err == nil {
				t.Fatalf("VerifyItem must reject %s, got nil error", tt.name)
			}
		})
	}
}

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

func mutateItem(base AttestationItem, f func(*AttestationItem)) AttestationItem {
	i := base
	f(&i)
	return i
}
