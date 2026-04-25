package moat

// VerifyManifest is the MOAT slice-1 production primitive for registry
// manifest verification. It consumes a sigstore bundle (the .sigstore file
// published alongside the manifest JSON) directly via sgbundle and verifies:
//
//  1. Signature validity: the bundle's signature covers the exact manifest
//     bytes (artifact digest comparison, not subject-name trust).
//  2. Certificate chain: the signing cert chains back to the Fulcio CA in
//     the trusted root.
//  3. Rekor inclusion: the transparency log entry's inclusion proof
//     verifies against the Rekor public key in the trusted root.
//  4. Identity match: the cert's OIDC issuer matches pinnedProfile.Issuer
//     (or IssuerRegex) and the SAN subject matches pinnedProfile.Subject
//     (or SubjectRegex). Regex variants let allowlist entries cover repos
//     that publish from multiple workflow paths.
//  5. Numeric-ID match (GitHub only): when Issuer is the GitHub Actions
//     issuer, the cert's RepositoryID and RepositoryOwnerID OIDC
//     extensions MUST match the pinned profile. This closes the
//     repo-transfer forgery vector — see ADR 0007.
//
// Revocation is NOT checked in slice 1. The returned VerificationResult
// exposes RevocationChecked=false explicitly so callers cannot collapse
// a successful verification to "verified" without acknowledging the gap.
//
// No live Rekor calls. Offline verification against the public key in the
// trusted root is sufficient because the Publisher Action includes the
// Signed Entry Timestamp (SET) and inclusion proof in the .sigstore bundle.
//
// Error discipline: returns a *VerifyError carrying a MOAT_* code when
// verification fails in a way the caller should surface structurally
// (identity mismatch, unpinned profile, bundle malformed). Low-level
// sigstore-go errors propagate wrapped with MOAT_INVALID.

import (
	"bytes"
	"fmt"
	"strings"

	sgbundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

// Error codes. Stable strings so CI pipelines and structured logs can grep
// on them. See ADR 0007 for the full taxonomy and reserved future codes.
const (
	CodeSigned             = "MOAT_SIGNED"
	CodeUnsigned           = "MOAT_UNSIGNED"
	CodeInvalid            = "MOAT_INVALID"
	CodeIdentityMismatch   = "MOAT_IDENTITY_MISMATCH"
	CodeIdentityUnpinned   = "MOAT_IDENTITY_UNPINNED"
	CodeTrustedRootStale   = "MOAT_TRUSTED_ROOT_STALE"
	CodeTrustedRootMissing = "MOAT_TRUSTED_ROOT_MISSING"
	CodeTrustedRootCorrupt = "MOAT_TRUSTED_ROOT_CORRUPT"
)

// Fulcio OID extensions for immutable numeric identifiers. These are
// populated on the signing cert for every GitHub Actions workflow and
// cannot be controlled by a transferee of the source repository.
//
//	.1.15 — SourceRepositoryIdentifier (numeric repository_id)
//	.1.17 — SourceRepositoryOwnerIdentifier (numeric repository_owner_id)
//
// The panel discussion that produced ADR 0007 cited .1.12 and .1.13 as the
// numeric IDs; those OIDs are actually SourceRepositoryURI and
// SourceRepositoryDigest (both strings, both mutable). The correct numeric
// OIDs per sigstore-go v1.1.4 and the Fulcio OID registry are .1.15/.1.17.
// See: https://github.com/sigstore/fulcio/blob/main/docs/oid-info.md

// VerificationResult captures what VerifyManifest checked and the outcome
// of each check. Consumers SHOULD NOT collapse a successful result to
// "verified" — the word is reserved for when revocation checking lands in
// a later slice. Slice 1's operational label is "signed" when all crypto
// checks pass.
type VerificationResult struct {
	SignatureValid        bool
	CertificateChainValid bool
	RekorProofValid       bool
	IdentityMatches       bool
	NumericIDMatched      bool // true when GitHub issuer + numeric IDs matched; false for non-GitHub or when no IDs were pinned
	RevocationChecked     bool // always false in slice 1

	// Observability: which trusted root was in effect. Useful for structured
	// logs and audit trails. Empty string means caller did not pass source
	// metadata.
	TrustedRootSource string
}

// VerifyError carries a MOAT_* code alongside the underlying cause. Callers
// use errors.As to inspect the code for structured error paths. The code
// vocabulary is defined above; see ADR 0007 for the reserved-but-not-yet-used
// codes.
type VerifyError struct {
	Code    string
	Message string
	Cause   error
}

func (e *VerifyError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *VerifyError) Unwrap() error { return e.Cause }

// verifyError is a constructor helper that keeps call sites tidy.
func verifyError(code, message string, cause error) *VerifyError {
	return &VerifyError{Code: code, Message: message, Cause: cause}
}

// VerifyManifest performs cryptographic verification of a MOAT registry
// manifest against a sigstore bundle and a pinned signing profile.
//
// manifestBytes: exact bytes of the manifest JSON as served by the registry.
// bundleBytes: exact bytes of the .sigstore bundle (v0.3 media type).
// pinnedProfile: operator-approved identity; must be non-nil.
// trustedRootJSON: Sigstore trusted_root.json (bundled or operator override).
//
// Returns a populated VerificationResult and a nil error on success. On
// verification failure, returns a zero result and a *VerifyError.
func VerifyManifest(
	manifestBytes []byte,
	bundleBytes []byte,
	pinnedProfile *SigningProfile,
	trustedRootJSON []byte,
) (VerificationResult, error) {
	if pinnedProfile == nil {
		return VerificationResult{}, verifyError(CodeIdentityUnpinned,
			"pinned signing profile required; refusing to verify without one", nil)
	}
	if len(trustedRootJSON) == 0 {
		return VerificationResult{}, verifyError(CodeTrustedRootMissing,
			"trusted root bytes empty", nil)
	}
	if len(bundleBytes) == 0 {
		return VerificationResult{}, verifyError(CodeUnsigned,
			"bundle bytes empty; manifest has no signature", nil)
	}
	if len(manifestBytes) == 0 {
		return VerificationResult{}, verifyError(CodeInvalid,
			"manifest bytes empty", nil)
	}

	// Load the trusted root (Fulcio CA + Rekor keys + timestamp authorities).
	tr, err := root.NewTrustedRootFromJSON(trustedRootJSON)
	if err != nil {
		return VerificationResult{}, verifyError(CodeTrustedRootCorrupt,
			"parsing trusted root", err)
	}

	// Load the sigstore bundle directly — no BuildBundle. Publishers emit
	// v0.3 bundles and sigstore-go can parse them natively. sigstore-go
	// v1.1.4 exposes LoadJSONFromPath for files; for in-memory bytes we
	// construct an empty Bundle and UnmarshalJSON.
	bundle := &sgbundle.Bundle{}
	if err := bundle.UnmarshalJSON(bundleBytes); err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"parsing sigstore bundle", err)
	}

	// Build the verifier. We require a transparency-log entry (Rekor
	// inclusion proof) and at least one integrated timestamp to bind the
	// signature to a point in time within cert validity.
	sev, err := verify.NewVerifier(tr,
		verify.WithTransparencyLog(1),
		verify.WithIntegratedTimestamps(1),
	)
	if err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"constructing verifier", err)
	}

	// sigstore-go's NewSANMatcher / NewIssuerMatcher accept literal-only,
	// regex-only, or both. Allowlist entries for repos that publish from
	// multiple workflow paths (e.g. moat.yml AND moat-publisher.yml) carry
	// only SubjectRegex; passing "" through here would leave sigstore-go with
	// no SAN criteria and trip its "must be subject alternative name
	// criteria" guard. Forwarding both fields lets either form verify.
	certID, err := verify.NewShortCertificateIdentity(
		pinnedProfile.Issuer,
		pinnedProfile.IssuerRegex,
		pinnedProfile.Subject,
		pinnedProfile.SubjectRegex,
	)
	if err != nil {
		return VerificationResult{}, verifyError(CodeIdentityMismatch,
			"building certificate identity matcher", err)
	}

	policy := verify.NewPolicy(
		verify.WithArtifact(bytes.NewReader(manifestBytes)),
		verify.WithCertificateIdentity(certID),
	)

	sgResult, err := sev.Verify(bundle, policy)
	if err != nil {
		// sigstore-go collapses crypto, chain, inclusion-proof, and identity
		// failures into a single error. We classify MOAT_INVALID for
		// signature/chain/proof failures and MOAT_IDENTITY_MISMATCH when the
		// error clearly names an identity mismatch. Heuristic classification
		// is acceptable here because callers needing finer discrimination
		// can pre-check the cert identity out-of-band.
		if isIdentityMismatch(err) {
			return VerificationResult{}, verifyError(CodeIdentityMismatch,
				"cert identity does not match pinned profile", err)
		}
		return VerificationResult{}, verifyError(CodeInvalid,
			"sigstore-go verify", err)
	}

	// Apply the GitHub numeric-ID binding. sigstore-go's standard verifier
	// matches on SAN + issuer but does not enforce the Fulcio numeric-ID
	// extensions; we do it explicitly here. NumericIDMatched is set to true
	// ONLY when a pinned ID was present and matched the cert — TOFU mode
	// (no pinned IDs) leaves it false so callers can distinguish
	// "actively matched" from "not-yet-pinned" without reading the profile.
	//
	// sgResult.VerifiedIdentity is the *pinned* policy that matched, not the
	// cert's parsed extensions — we extract the cert from the bundle instead.
	_ = sgResult
	numericIDMatched := false
	if pinnedProfile.RequiresNumericIDMatch() {
		matched, err := matchNumericIDs(bundle, pinnedProfile)
		if err != nil {
			return VerificationResult{}, err
		}
		numericIDMatched = matched
	}

	return VerificationResult{
		SignatureValid:        true,
		CertificateChainValid: true,
		RekorProofValid:       true,
		IdentityMatches:       true,
		NumericIDMatched:      numericIDMatched,
		RevocationChecked:     false,
	}, nil
}

// isIdentityMismatch returns true when the sigstore-go error is clearly an
// identity-match failure rather than a crypto/proof/chain failure.
// sigstore-go's error taxonomy is not stable, so this is a best-effort
// string classifier. When in doubt, fall through to MOAT_INVALID.
func isIdentityMismatch(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return stringContainsAny(msg,
		"certificate identity",
		"san does not match",
		"SAN",
		"subject",
		"issuer",
	)
}

func stringContainsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

// matchNumericIDs extracts the GitHub OIDC numeric-ID extensions from the
// verified cert and compares them to the pinned profile.
//
// Return contract:
//   - (true, nil)  — at least one pinned ID was present on the profile and
//     matched the cert. This is the "actively matched" signal.
//   - (false, nil) — pinned profile declared no IDs; TOFU-capture mode.
//     The caller is expected to harvest the cert's IDs and persist them on
//     the profile for future verifications. No mismatch occurred.
//   - (_, *VerifyError) — a pinned ID did not match, or the cert is missing
//     the numeric-ID extensions entirely (non-conforming GitHub token).
//     MOAT_IDENTITY_MISMATCH in both cases.
func matchNumericIDs(b *sgbundle.Bundle, pinned *SigningProfile) (bool, error) {
	gotRepoID, gotOwnerID, err := extensionsFromBundle(b)
	if err != nil {
		return false, verifyError(CodeIdentityMismatch,
			"extracting numeric-ID extensions", err)
	}

	if pinned.RepositoryID != "" && gotRepoID != pinned.RepositoryID {
		return false, verifyError(CodeIdentityMismatch,
			fmt.Sprintf("repository_id mismatch: got=%q want=%q",
				gotRepoID, pinned.RepositoryID), nil)
	}
	if pinned.RepositoryOwnerID != "" && gotOwnerID != pinned.RepositoryOwnerID {
		return false, verifyError(CodeIdentityMismatch,
			fmt.Sprintf("repository_owner_id mismatch: got=%q want=%q",
				gotOwnerID, pinned.RepositoryOwnerID), nil)
	}

	// TOFU-capture mode: nothing pinned, so nothing to "match." The cert
	// did carry the extensions (extensionsFromSigstoreResult errors when
	// they're absent), so the caller can harvest gotRepoID/gotOwnerID from
	// the sigstore result directly when it wants to persist them.
	if pinned.RepositoryID == "" && pinned.RepositoryOwnerID == "" {
		return false, nil
	}
	return true, nil
}

// extensionsFromBundle parses the leaf cert out of a sigstore bundle and
// returns the GitHub OIDC numeric-ID extensions.
//
// This reads the CERT, not sgResult.VerifiedIdentity. sigstore-go copies the
// *pinned policy* onto VerificationResult.VerifiedIdentity — the cert's own
// extensions are never surfaced on the result struct. So TOFU-capture callers
// that pin empty IDs would always see empty gotRepoID/gotOwnerID if we read
// from sgResult. Reading from the bundle's cert is the only path that works
// for both TOFU capture and pinned-ID match.
//
// The immutable numeric IDs live at OID 1.3.6.1.4.1.57264.1.15 (repo) and
// .1.17 (owner) — NOT .12/.13, which are URI/digest strings that move
// with a repo transfer. See the header comment of this file for why this
// correction matters for the repo-transfer forgery vector.
func extensionsFromBundle(b *sgbundle.Bundle) (repoID, ownerID string, err error) {
	if b == nil {
		return "", "", fmt.Errorf("bundle is nil")
	}
	vc, err := b.VerificationContent()
	if err != nil {
		return "", "", fmt.Errorf("reading bundle verification content: %w", err)
	}
	cert := vc.Certificate()
	if cert == nil {
		return "", "", fmt.Errorf("bundle has no leaf certificate")
	}
	sum, err := certificate.SummarizeCertificate(cert)
	if err != nil {
		return "", "", fmt.Errorf("summarizing cert: %w", err)
	}

	repoID = sum.SourceRepositoryIdentifier
	ownerID = sum.SourceRepositoryOwnerIdentifier

	if repoID == "" && ownerID == "" {
		return "", "", fmt.Errorf("cert has no numeric-ID extensions; non-conforming GitHub token")
	}
	return repoID, ownerID, nil
}
