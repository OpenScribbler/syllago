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

// ProfileVersion values in use. Unversioned profiles (captured before this
// field existed) default to ProfileVersionV1 on load. A future bump to v2
// signals GitLab/Buildkite/etc. issuer support.
const (
	ProfileVersionV1 = 1
)

// SigningProfile is the expected OIDC identity for a Publisher or Registry
// workflow. For GitHub Actions keyless signing, the issuer is always
// https://token.actions.githubusercontent.com; the subject is the workflow
// URI (e.g. https://github.com/OWNER/REPO/.github/workflows/NAME.yml@REF).
//
// The JSON tags serve manifest.go's registry_signing_profile and per-item
// signing_profile fields — the wire representation uses lowercase keys.
// Verification code that holds SigningProfile values in memory is unaffected.
//
// Numeric ID binding (RepositoryID, RepositoryOwnerID) closes the GitHub
// repo-transfer forgery vector: the SAN subject is derived from mutable
// owner/repo names that a transferee can re-register, but the GitHub OIDC
// extensions at OIDs 1.3.6.1.4.1.57264.1.15 (repo) and .1.17 (owner) are
// immutable numeric identifiers. When Issuer is the GitHub Actions issuer,
// verifiers MUST compare both numeric IDs in addition to the SAN. See ADR
// 0007 and the header of manifest_verify.go for why .12/.13 is wrong.
type SigningProfile struct {
	Issuer  string `json:"issuer"`
	Subject string `json:"subject"`

	// ProfileVersion tracks schema shape for additive extensions. Absent or
	// zero on load means v1 (back-compat for profiles captured before the
	// field existed). New captures set this explicitly.
	ProfileVersion int `json:"profile_version,omitempty"`

	// SubjectRegex and IssuerRegex are optional match relaxations for when
	// an exact string match is too strict (e.g. branch rotation). When set,
	// they take precedence over the exact Subject/Issuer equality check.
	// Reserved for slice 2+ usage; verifiers in slice 1 compare Subject/Issuer
	// exactly.
	SubjectRegex string `json:"subject_regex,omitempty"`
	IssuerRegex  string `json:"issuer_regex,omitempty"`

	// RepositoryID and RepositoryOwnerID are the GitHub OIDC numeric
	// identifiers extracted from the Fulcio cert. Populated at pin-time by
	// TOFU capture from the first observed cert. When Issuer is the GitHub
	// Actions issuer, the verifier MUST match both.
	RepositoryID      string `json:"repository_id,omitempty"`
	RepositoryOwnerID string `json:"repository_owner_id,omitempty"`
}

// GitHubActionsIssuer is the OIDC issuer string for workflows running in
// GitHub Actions. Used to gate the numeric-ID match.
const GitHubActionsIssuer = "https://token.actions.githubusercontent.com"

// EffectiveProfileVersion returns the profile version with the v1 default
// applied for pre-versioning profiles.
func (s SigningProfile) EffectiveProfileVersion() int {
	if s.ProfileVersion == 0 {
		return ProfileVersionV1
	}
	return s.ProfileVersion
}

// RequiresNumericIDMatch reports whether the issuer is the GitHub Actions
// issuer, in which case verifiers MUST match RepositoryID and
// RepositoryOwnerID against the OIDC extensions on the cert.
func (s SigningProfile) RequiresNumericIDMatch() bool {
	return s.Issuer == GitHubActionsIssuer
}

// CurrentPayloadVersion is the canonical-payload schema version Syllago
// emits and expects today (spec v0.6.0 §Attestation Payload). A future
// schema bump increments this constant, triggering a grace period during
// which SupportedPayloadVersions carries both the prior and the new value.
const CurrentPayloadVersion = 1

// SupportedPayloadVersions is the closed set of `_version` values a
// conforming verifier MUST accept. Spec §Version Transition requires
// clients to honor both the prior and the current value for six months
// after a schema bump; outside that window, only CurrentPayloadVersion
// appears here. Today the slice has one element; during a grace period it
// will carry two. Order is intentional: most-recent first, so dispatchers
// that iterate can prefer the current version when multiple match (which
// should never happen in practice — each `_version` produces a distinct
// canonical payload hash — but the ordering keeps behavior deterministic).
//
// Not a const because Go has no slice constants; callers MUST treat it as
// read-only. The spec change that triggers a grace period is the only
// legitimate reason to edit this declaration.
var SupportedPayloadVersions = []int{CurrentPayloadVersion}

// IsSupportedPayloadVersion reports whether v is an accepted canonical
// payload `_version` value (in-grace or current, per the spec §Version
// Transition rule). Intended as the ordering-step-2 gate described in
// ADR 0007 G-14.
func IsSupportedPayloadVersion(v int) bool {
	for _, s := range SupportedPayloadVersions {
		if s == v {
			return true
		}
	}
	return false
}

// CanonicalPayloadForVersion returns the exact byte sequence a conforming
// verifier hashes and expects Rekor's `data.hash.value` to match, for the
// given `_version` and content hash.
//
// Returns (nil, false) if v is not in SupportedPayloadVersions. Callers
// MUST check the bool before using the bytes — a successful match against
// an unsupported version is a signal to reject, not to repair.
//
// TOCTOU-safety (ADR 0007 G-14): this builder NEVER reads `_version` from
// the wire. Both inputs (v, contentHash) are client-controlled — either
// hard-coded or taken from the lockfile/manifest entry the client already
// decided to install. A grace-period-aware verifier iterates
// SupportedPayloadVersions and calls this function once per version, then
// checks which (if any) matches Rekor's recorded hash — the content_hash
// is fixed per call, and the `_version` tried is pre-approved, so there
// is no window where an attacker-supplied field decides the code path.
//
// The serialization is normative: compact JSON, no whitespace, field
// order `_version` then `content_hash`. Produced by hand because Go's
// json.Marshal alphabetizes struct fields and would corrupt the order.
func CanonicalPayloadForVersion(v int, contentHash string) (payload []byte, ok bool) {
	if !IsSupportedPayloadVersion(v) {
		return nil, false
	}
	// The contentHash value is JSON-encoded (via json.Marshal on a
	// string) to handle escaping of characters that could appear in a
	// future hash algorithm prefix.
	hashJSON, _ := json.Marshal(contentHash)
	return fmt.Appendf(nil, `{"_version":%d,"content_hash":%s}`, v, hashJSON), true
}

// CanonicalPayloadFor returns the canonical payload bytes for the current
// payload version. Convenience wrapper around CanonicalPayloadForVersion
// for call sites that don't need grace-period awareness. Any drift in
// output here breaks every signature verification — the byte sequence is
// fixture-anchored against the syllago-meta-registry Phase 0 Rekor entry
// (see rekor_test.go and canonical_payload_test.go).
func CanonicalPayloadFor(contentHash string) []byte {
	payload, _ := CanonicalPayloadForVersion(CurrentPayloadVersion, contentHash)
	return payload
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
