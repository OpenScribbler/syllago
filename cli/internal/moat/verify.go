// Package moat implements a conforming client for MOAT v0.6.0 registries.
//
// STATUS: SPIKE (syllago-9jzgr). This file is the seed for the full Phase 2
// implementation. The spike goal is to verify one real Rekor entry from the
// syllago-meta-registry Phase 0 Publisher Action output end-to-end using
// sigstore-go. If that works, the integration path for Phase 2 is validated.
//
// Verification flow (per moat-spec.md §Trust Model):
//
//  1. Reconstruct the canonical payload from the item's content_hash field.
//  2. Compute SHA-256 of the canonical payload — this is the expected
//     data.hash.value recorded in the Rekor hashedrekord entry.
//  3. Fetch the Rekor entry at rekor_log_index from the public Rekor instance.
//  4. Confirm Rekor data.hash.value matches the recomputed hash (signature
//     covers the right payload).
//  5. Extract the signing certificate from the Rekor body; verify the chain
//     back to the Fulcio root.
//  6. Confirm the certificate's OIDC SAN equals the expected signing profile
//     subject, and the issuer extension matches profile.Issuer.
//  7. Verify the Rekor Signed Entry Timestamp (inclusion proof).
package moat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Attestation is the root structure of moat-attestation.json.
type Attestation struct {
	SchemaVersion        int               `json:"schema_version"`
	AttestedAt           string            `json:"attested_at"`
	PublisherWorkflowRef string            `json:"publisher_workflow_ref"`
	PrivateRepo          bool              `json:"private_repo"`
	Items                []AttestationItem `json:"items"`
	Revocations          []any             `json:"revocations"`
}

// AttestationItem is a single per-item attestation entry.
type AttestationItem struct {
	Name          string `json:"name"`
	ContentHash   string `json:"content_hash"`
	SourceRef     string `json:"source_ref"`
	RekorLogID    string `json:"rekor_log_id"`
	RekorLogIndex int64  `json:"rekor_log_index"`
}

// SigningProfile is the expected OIDC identity for a Publisher or Registry
// workflow. For GitHub Actions keyless signing, the issuer is always
// https://token.actions.githubusercontent.com; the subject is the workflow
// URI (e.g. https://github.com/OWNER/REPO/.github/workflows/NAME.yml@REF).
//
// The JSON tags serve manifest.go's registry_signing_profile and per-item
// signing_profile fields — the wire representation uses lowercase keys.
// Verification code that holds SigningProfile values in memory is unaffected.
type SigningProfile struct {
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`
}

// CanonicalPayloadFor returns the exact byte sequence that the Publisher
// Action hashes and signs for a given content hash. The serialization is
// normative: compact JSON, no whitespace, field order fixed.
//
// This is NOT produced by json.Marshal because Go's encoding/json alphabetizes
// struct fields; the spec fixes "_version" first, "content_hash" second. Any
// drift here causes every signature to fail verification.
func CanonicalPayloadFor(contentHash string) []byte {
	// Manually serialized to lock field order. The contentHash value is
	// JSON-encoded (via json.Marshal on a string) to handle escaping of
	// characters that could appear in a future hash algorithm prefix.
	hashJSON, _ := json.Marshal(contentHash)
	return fmt.Appendf(nil, `{"_version":1,"content_hash":%s}`, hashJSON)
}

// VerifyItem performs offline verification of a single attestation item
// against a pre-fetched Rekor entry. In Phase 2, rekorRaw will be fetched
// live from rekor.sigstore.dev; for the spike, callers pass fixture bytes.
//
// Verified invariants:
//  1. rekorRaw parses as a one-entry Rekor API response.
//  2. The entry's LogIndex matches item.RekorLogIndex.
//  3. The body is a hashedrekord (apiVersion 0.0.1).
//  4. sha256(CanonicalPayloadFor(item.ContentHash)) equals body.Spec.Data.Hash.Value.
//  5. The body's ECDSA signature verifies against the canonical payload
//     using the public key from the body's publicKey PEM.
//  6. The cert's OIDC identity (issuer extension + first URI SAN) equals
//     the expected profile.
//
// Not yet verified (delegated to sigstore-go in the Phase 2 production
// implementation): Fulcio cert chain trust, Rekor Signed Entry Timestamp
// and inclusion proof, and certificate validity at the integrated time.
func VerifyItem(item AttestationItem, profile SigningProfile, rekorRaw []byte) error {
	entry, err := parseRekorEntry(rekorRaw)
	if err != nil {
		return fmt.Errorf("parsing Rekor entry: %w", err)
	}
	if entry.LogIndex != item.RekorLogIndex {
		return fmt.Errorf("rekor log index mismatch: entry=%d, item=%d", entry.LogIndex, item.RekorLogIndex)
	}

	body, err := decodeHashedRekordBody(entry.Body)
	if err != nil {
		return fmt.Errorf("decoding hashedrekord body: %w", err)
	}

	payload := CanonicalPayloadFor(item.ContentHash)
	digest := sha256.Sum256(payload)
	expectedHash := hex.EncodeToString(digest[:])
	if body.Spec.Data.Hash.Value != expectedHash {
		return fmt.Errorf("rekor data.hash.value mismatch: computed=%s recorded=%s",
			expectedHash, body.Spec.Data.Hash.Value)
	}

	cert, err := extractCert(body)
	if err != nil {
		return fmt.Errorf("extracting cert: %w", err)
	}
	if err := verifySignature(cert, body, payload); err != nil {
		return fmt.Errorf("verifying signature: %w", err)
	}

	issuer, subject, err := extractIdentity(cert)
	if err != nil {
		return fmt.Errorf("extracting identity: %w", err)
	}
	if issuer != profile.Issuer {
		return fmt.Errorf("cert issuer mismatch: got=%q want=%q", issuer, profile.Issuer)
	}
	if subject != profile.Subject {
		return fmt.Errorf("cert subject mismatch: got=%q want=%q", subject, profile.Subject)
	}
	return nil
}
