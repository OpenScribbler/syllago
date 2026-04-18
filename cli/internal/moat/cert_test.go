package moat

import (
	"os"
	"testing"
)

// TestExtractCert_AndIdentity reads the checked-in Rekor fixture, pulls the
// Fulcio cert and its OIDC identity out, and asserts both match what the
// syllago-meta-registry Phase 0 Publisher Action should produce under GitHub
// Actions keyless signing:
//
//   - Issuer:  https://token.actions.githubusercontent.com
//   - Subject: https://github.com/OpenScribbler/syllago-meta-registry/
//     .github/workflows/moat.yml@refs/heads/master
//
// If these drift, the SigningProfile equality check in Phase 2 verification
// will also drift — so this test is the contract for the identity policy.
func TestExtractCert_AndIdentity(t *testing.T) {
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

	// Cert must have exactly one URI SAN — the workflow identity.
	if got, want := len(cert.URIs), 1; got != want {
		t.Errorf("URI SAN count: got %d, want %d", got, want)
	}

	issuer, subject, err := extractIdentity(cert)
	if err != nil {
		t.Fatalf("extracting identity: %v", err)
	}

	const wantIssuer = "https://token.actions.githubusercontent.com"
	const wantSubject = "https://github.com/OpenScribbler/syllago-meta-registry/.github/workflows/moat.yml@refs/heads/master"

	if issuer != wantIssuer {
		t.Errorf("issuer: got %q, want %q", issuer, wantIssuer)
	}
	if subject != wantSubject {
		t.Errorf("subject: got %q, want %q", subject, wantSubject)
	}
}
