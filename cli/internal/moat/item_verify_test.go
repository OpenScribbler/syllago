package moat

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// TestVerifyAttestationItem_HappyPath exercises the full per-item
// verification chain against the real Phase-0 meta-registry fixture:
// Fulcio chain back to the bundled trusted root at integratedTime, Rekor
// SET + inclusion proof, canonical-payload binding, identity match. If this
// passes, G-5's production path is validated end-to-end.
func TestVerifyAttestationItem_HappyPath(t *testing.T) {
	t.Parallel()

	att := loadAttestation(t)
	item := findItem(t, att, "syllago-guide")
	rekorRaw := loadRekorFixture(t)
	tr := loadTrustedRoot(t)
	profile := expectedProfile()

	result, err := VerifyAttestationItem(item, &profile, rekorRaw, tr)
	if err != nil {
		t.Fatalf("VerifyAttestationItem must succeed on valid fixtures: %v", err)
	}
	if !result.SignatureValid || !result.CertificateChainValid || !result.RekorProofValid || !result.IdentityMatches {
		t.Fatalf("all positive flags must be set: %+v", result)
	}
	if result.RevocationChecked {
		t.Errorf("RevocationChecked must stay false in slice 1/2 (got true)")
	}
	// TOFU-capture mode: profile has no pinned IDs, so NumericIDMatched is
	// expected false even for the GHA issuer. A pinned-ID variant is
	// exercised in TestVerifyAttestationItem_NumericIDMatch.
	if result.NumericIDMatched {
		t.Errorf("NumericIDMatched must be false when profile pins no IDs")
	}
}

// TestVerifyAttestationItem_RejectsWrongIdentity confirms the pinned-profile
// check actually fires. Swapping the subject for an attacker-controlled
// workflow URI must produce MOAT_IDENTITY_MISMATCH.
func TestVerifyAttestationItem_RejectsWrongIdentity(t *testing.T) {
	t.Parallel()

	att := loadAttestation(t)
	item := findItem(t, att, "syllago-guide")
	rekorRaw := loadRekorFixture(t)
	tr := loadTrustedRoot(t)

	wrong := expectedProfile()
	wrong.Subject = "https://github.com/attacker/fake-repo/.github/workflows/evil.yml@refs/heads/main"

	_, err := VerifyAttestationItem(item, &wrong, rekorRaw, tr)
	if err == nil {
		t.Fatal("expected identity mismatch, got nil")
	}
	var ve *VerifyError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *VerifyError, got %T: %v", err, err)
	}
	if ve.Code != CodeIdentityMismatch {
		t.Errorf("got code=%q want=%q", ve.Code, CodeIdentityMismatch)
	}
}

// TestVerifyAttestationItem_RejectsContentHashTamper confirms that mutating
// the item's content hash breaks the canonical-payload binding and produces
// MOAT_INVALID — this is the property that makes per-item attestation
// sound against content substitution.
func TestVerifyAttestationItem_RejectsContentHashTamper(t *testing.T) {
	t.Parallel()

	att := loadAttestation(t)
	item := findItem(t, att, "syllago-guide")
	rekorRaw := loadRekorFixture(t)
	tr := loadTrustedRoot(t)
	profile := expectedProfile()

	tampered := item
	tampered.ContentHash = "sha256:" + zeros64

	_, err := VerifyAttestationItem(tampered, &profile, rekorRaw, tr)
	if err == nil {
		t.Fatal("expected canonical payload hash mismatch, got nil")
	}
	var ve *VerifyError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *VerifyError, got %T: %v", err, err)
	}
	if ve.Code != CodeInvalid {
		t.Errorf("got code=%q want=%q", ve.Code, CodeInvalid)
	}
	if !strings.Contains(ve.Error(), "canonical payload hash mismatch") {
		t.Errorf("error did not name canonical payload mismatch: %v", err)
	}
}

// TestVerifyAttestationItem_RejectsLogIndexMismatch confirms the LogIndex
// binding. An attestation row claiming a different log index than the
// fetched Rekor entry must fail — otherwise an attacker could substitute
// one Rekor entry for another signed by the same publisher.
func TestVerifyAttestationItem_RejectsLogIndexMismatch(t *testing.T) {
	t.Parallel()

	att := loadAttestation(t)
	item := findItem(t, att, "syllago-guide")
	rekorRaw := loadRekorFixture(t)
	tr := loadTrustedRoot(t)
	profile := expectedProfile()

	item.RekorLogIndex = 0

	_, err := VerifyAttestationItem(item, &profile, rekorRaw, tr)
	if err == nil {
		t.Fatal("expected log index mismatch, got nil")
	}
	var ve *VerifyError
	if !errors.As(err, &ve) || ve.Code != CodeInvalid {
		t.Fatalf("expected MOAT_INVALID, got %v", err)
	}
	if !strings.Contains(ve.Error(), "rekor log index mismatch") {
		t.Errorf("error did not name log index mismatch: %v", err)
	}
}

// TestVerifyAttestationItem_UnpinnedProfile confirms the precondition: a
// caller passing nil or empty profile gets MOAT_IDENTITY_UNPINNED, not a
// cryptic nil-deref later in the chain.
func TestVerifyAttestationItem_UnpinnedProfile(t *testing.T) {
	t.Parallel()

	item := AttestationItem{ContentHash: "sha256:" + zeros64, RekorLogIndex: 1}
	tr := []byte(`{"mediaType":"application/vnd.dev.sigstore.trustedroot+json;version=0.1"}`)

	for _, tc := range []struct {
		name    string
		profile *SigningProfile
	}{
		{"nil_profile", nil},
		{"empty_subject", &SigningProfile{Issuer: GitHubActionsIssuer}},
		{"empty_issuer", &SigningProfile{Subject: "https://example.com/a.yml"}},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := VerifyAttestationItem(item, tc.profile, []byte("noop"), tr)
			var ve *VerifyError
			if !errors.As(err, &ve) || ve.Code != CodeIdentityUnpinned {
				t.Fatalf("expected MOAT_IDENTITY_UNPINNED, got %v", err)
			}
		})
	}
}

// TestVerifyAttestationItem_MissingTrustedRoot confirms that an empty
// trusted root is surfaced with MOAT_TRUSTED_ROOT_MISSING before any Rekor
// parsing happens.
func TestVerifyAttestationItem_MissingTrustedRoot(t *testing.T) {
	t.Parallel()
	profile := expectedProfile()
	_, err := VerifyAttestationItem(AttestationItem{}, &profile, []byte("x"), nil)
	var ve *VerifyError
	if !errors.As(err, &ve) || ve.Code != CodeTrustedRootMissing {
		t.Fatalf("expected MOAT_TRUSTED_ROOT_MISSING, got %v", err)
	}
}

// TestVerifyAttestationItem_UnsignedItem confirms that an item with no Rekor
// response bytes (empty rekorRaw) is classified MOAT_UNSIGNED rather than
// collapsing to a generic parse error. The caller treats this state as
// "item has no attestation" not "attestation is malformed."
func TestVerifyAttestationItem_UnsignedItem(t *testing.T) {
	t.Parallel()
	profile := expectedProfile()
	tr := loadTrustedRoot(t)
	_, err := VerifyAttestationItem(AttestationItem{}, &profile, nil, tr)
	var ve *VerifyError
	if !errors.As(err, &ve) || ve.Code != CodeUnsigned {
		t.Fatalf("expected MOAT_UNSIGNED, got %v", err)
	}
}

// TestVerifyAttestationItem_NumericIDMismatch confirms the GitHub numeric-ID
// binding fires when a pinned ID does not match the cert. A transferee
// re-registering the owner/repo name gets a different numeric ID, so a
// pinned ID that disagrees with the cert MUST reject — this closes the
// repo-rename forgery vector.
func TestVerifyAttestationItem_NumericIDMismatch(t *testing.T) {
	t.Parallel()

	att := loadAttestation(t)
	item := findItem(t, att, "syllago-guide")
	rekorRaw := loadRekorFixture(t)
	tr := loadTrustedRoot(t)

	profile := expectedProfile()
	profile.RepositoryID = "99999999" // deliberately wrong

	_, err := VerifyAttestationItem(item, &profile, rekorRaw, tr)
	if err == nil {
		t.Fatal("expected numeric-ID mismatch, got nil")
	}
	var ve *VerifyError
	if !errors.As(err, &ve) || ve.Code != CodeIdentityMismatch {
		t.Fatalf("expected MOAT_IDENTITY_MISMATCH, got %v", err)
	}
	if !strings.Contains(ve.Error(), "repository_id mismatch") {
		t.Errorf("error did not name repository_id: %v", err)
	}
}

// TestVerifyAttestationItem_NumericIDMatch confirms the positive path for
// the numeric-ID binding: when the profile pins the cert's actual numeric
// IDs, NumericIDMatched flips to true. We discover the real IDs from the
// fixture cert and feed them back to validate the loop.
func TestVerifyAttestationItem_NumericIDMatch(t *testing.T) {
	t.Parallel()

	att := loadAttestation(t)
	item := findItem(t, att, "syllago-guide")
	rekorRaw := loadRekorFixture(t)
	tr := loadTrustedRoot(t)

	repoID, ownerID := extractFixtureNumericIDsFromRekor(t, rekorRaw)
	if repoID == "" || ownerID == "" {
		t.Fatalf("fixture cert is missing expected numeric-ID extensions: repo=%q owner=%q", repoID, ownerID)
	}

	profile := expectedProfile()
	profile.RepositoryID = repoID
	profile.RepositoryOwnerID = ownerID

	result, err := VerifyAttestationItem(item, &profile, rekorRaw, tr)
	if err != nil {
		t.Fatalf("VerifyAttestationItem with pinned IDs must succeed: %v", err)
	}
	if !result.NumericIDMatched {
		t.Errorf("NumericIDMatched must be true when pinned IDs match cert")
	}
}

// extractFixtureNumericIDsFromRekor pulls the Fulcio .1.15 / .1.17 extensions
// out of the cert embedded in a raw Rekor response so
// TestVerifyAttestationItem_NumericIDMatch can round-trip them. This is the
// Rekor-path counterpart to extractFixtureNumericIDs (which parses a
// .sigstore bundle) in manifest_verify_test.go — kept separate because the
// per-item path never constructs a bundle. Production callers that need the
// IDs for TOFU capture should read them off the VerificationResult once we
// surface them (G-6).
func extractFixtureNumericIDsFromRekor(t *testing.T, rekorRaw []byte) (repoID, ownerID string) {
	t.Helper()
	var resp rekorResponse
	if err := json.Unmarshal(rekorRaw, &resp); err != nil {
		t.Fatalf("parsing rekor fixture: %v", err)
	}
	var entry rekorEntry
	for _, e := range resp {
		entry = e
	}
	body, err := decodeHashedRekordBody(entry.Body)
	if err != nil {
		t.Fatalf("decoding body: %v", err)
	}
	cert, err := extractCert(body)
	if err != nil {
		t.Fatalf("extracting cert: %v", err)
	}
	repo, owner, err := readNumericIDsFromCert(cert)
	if err != nil {
		t.Fatalf("reading numeric IDs: %v", err)
	}
	// Sanity — Fulcio emits these as strings; a cert that returned all-zero
	// would indicate an ASN.1 parse bug in parseUTF8StringExt.
	if bytes.ContainsRune([]byte(repo), 0) || bytes.ContainsRune([]byte(owner), 0) {
		t.Fatalf("numeric IDs contain null bytes; parse bug: repo=%q owner=%q", repo, owner)
	}
	return repo, owner
}
