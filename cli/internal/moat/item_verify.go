package moat

// VerifyAttestationItem is the production per-item Rekor verification entry
// point (ADR 0007 G-5). Unlike the spike path in verify.go's VerifyItem, this
// function enforces the full trust chain:
//
//  1. Rekor entry shape (hashedrekord v0.0.1) and LogIndex match.
//  2. Canonical payload hash match: sha256(CanonicalPayloadFor(item.ContentHash))
//     equals the body's data.hash.value — binds the signature to the content
//     hash the client is about to install.
//  3. ECDSA signature covers the canonical payload.
//  4. Rekor Signed Entry Timestamp (SET) verifies against the Rekor public key
//     from the trusted root — binds the entry to Rekor at integratedTime.
//  5. Rekor inclusion proof verifies against the checkpoint signed by Rekor —
//     binds the entry to a specific position in the transparency log.
//  6. Fulcio cert chain validates back to the trusted-root Fulcio CA at
//     integratedTime — hybrid-model certificate trust (Braun 2013).
//  7. Cert identity (issuer + SAN subject) matches the pinned signing profile
//     exactly. Regex relaxation is reserved for slice 2+ and is not applied
//     here; callers pinning regex-only profiles will mismatch in this slice.
//  8. GitHub numeric-ID binding: when Issuer is the GitHub Actions issuer,
//     the cert's SourceRepositoryIdentifier and SourceRepositoryOwnerIdentifier
//     extensions (OID .1.15 / .1.17) MUST match the pinned profile's
//     RepositoryID / RepositoryOwnerID. Closes the repo-transfer forgery
//     vector. See manifest_verify.go for the OID correction history.
//
// Unknown `_version` values in the canonical payload are rejected implicitly:
// CanonicalPayloadFor hard-codes `_version:1`, so a publisher who signed a
// payload with `_version:2` would produce a different data.hash.value and
// step 2 would fail. The spec requires this rejection (§Ordering: Content
// Hash Before _version) and syllago inherits it for free.
//
// Revocation is NOT checked here — that lands with G-8. VerificationResult
// carries RevocationChecked=false so callers cannot collapse the result to
// "verified" without reading the bit explicitly.

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	rekorv1 "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tlog"
	sgsig "github.com/sigstore/sigstore/pkg/signature"
)

// Fulcio OID extensions for immutable numeric identifiers, duplicated from
// manifest_verify.go's package-level doc for direct use from this file.
// Splitting them out here keeps item_verify.go self-contained for readers.
var (
	fulcioRepoIDOID      = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 15}
	fulcioRepoOwnerIDOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 17}
)

// VerifyAttestationItem verifies a single per-item Rekor entry against the
// pinned signing profile and the trusted root.
//
// item:            the attestation row being verified — provides the content
//
//	hash to reconstruct the canonical payload and the Rekor
//	coordinates (logIndex, logID).
//
// profile:         operator-approved identity. Nil or empty subject/issuer
//
//	results in MOAT_IDENTITY_UNPINNED.
//
// rekorRaw:        exact JSON bytes returned by the Rekor API for this entry
//
//	(the single-entry map keyed by UUID). Publishers capture
//	this at attestation time.
//
// trustedRootJSON: Sigstore trusted_root.json — bundled or operator-supplied.
//
// Returns a populated VerificationResult on success. On any failure returns
// a zero result and a *VerifyError carrying the appropriate MOAT_* code.
func VerifyAttestationItem(
	item AttestationItem,
	profile *SigningProfile,
	rekorRaw []byte,
	trustedRootJSON []byte,
) (VerificationResult, error) {
	if profile == nil || profile.Issuer == "" || profile.Subject == "" {
		return VerificationResult{}, verifyError(CodeIdentityUnpinned,
			"pinned signing profile required; refusing to verify without one", nil)
	}
	if len(trustedRootJSON) == 0 {
		return VerificationResult{}, verifyError(CodeTrustedRootMissing,
			"trusted root bytes empty", nil)
	}
	if len(rekorRaw) == 0 {
		return VerificationResult{}, verifyError(CodeUnsigned,
			"rekor response bytes empty; item has no attestation", nil)
	}

	tr, err := root.NewTrustedRootFromJSON(trustedRootJSON)
	if err != nil {
		return VerificationResult{}, verifyError(CodeTrustedRootCorrupt,
			"parsing trusted root", err)
	}

	entry, err := parseRekorEntry(rekorRaw)
	if err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"parsing Rekor entry", err)
	}
	if entry.LogIndex != item.RekorLogIndex {
		return VerificationResult{}, verifyError(CodeInvalid,
			fmt.Sprintf("rekor log index mismatch: entry=%d item=%d",
				entry.LogIndex, item.RekorLogIndex), nil)
	}

	// Decoded body JSON is what Rekor canonicalizes and signs for the SET.
	// Pass these exact bytes to tlog.NewEntry; sigstore-go re-canonicalizes
	// internally but needs the parsed body to classify the entry type.
	bodyBytes, err := base64.StdEncoding.DecodeString(entry.Body)
	if err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"base64-decoding Rekor body", err)
	}
	body, err := decodeHashedRekordBody(entry.Body)
	if err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"decoding hashedrekord body", err)
	}

	payload := CanonicalPayloadFor(item.ContentHash)
	digest := sha256.Sum256(payload)
	expectedHash := hex.EncodeToString(digest[:])
	if body.Spec.Data.Hash.Value != expectedHash {
		return VerificationResult{}, verifyError(CodeInvalid,
			fmt.Sprintf("canonical payload hash mismatch: computed=%s recorded=%s",
				expectedHash, body.Spec.Data.Hash.Value), nil)
	}

	cert, err := extractCert(body)
	if err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"extracting signing certificate", err)
	}
	if err := verifySignature(cert, body, payload); err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"signature verification", err)
	}

	// Build a tlog.Entry for SET + inclusion-proof verification.
	//
	// We construct a *rekorv1.TransparencyLogEntry by hand and hand it to
	// ParseTransparencyLogEntry rather than calling the deprecated
	// tlog.NewEntry: NewEntry overwrites InclusionProof.LogIndex with the
	// global (cross-shard) log index, which breaks inclusion-proof math for
	// entries on rekor.sigstore.dev's second shard (tree sizes are
	// shard-local, so global index >= shard size is routine). Going through
	// the proto directly preserves the shard-local logIndex Rekor returned.
	logIDBytes, err := hex.DecodeString(entry.LogID)
	if err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"decoding rekor logID", err)
	}
	setBytes, err := base64.StdEncoding.DecodeString(entry.Verification.SignedEntryTimestamp)
	if err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"decoding signedEntryTimestamp", err)
	}
	tle, err := buildTransparencyLogEntry(entry, bodyBytes, logIDBytes, setBytes)
	if err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"constructing transparency log entry", err)
	}
	tlogEntry, err := tlog.ParseTransparencyLogEntry(tle)
	if err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"parsing transparency log entry", err)
	}

	rekorLogs := tr.RekorLogs()
	if err := tlog.VerifySET(tlogEntry, rekorLogs); err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"rekor SET verification", err)
	}

	// Inclusion proof: pull the verifier for this log ID out of the trusted
	// root and verify the checkpoint signature + Merkle path.
	hexKeyID := hex.EncodeToString(logIDBytes)
	tlogVerifier, ok := rekorLogs[hexKeyID]
	if !ok {
		return VerificationResult{}, verifyError(CodeInvalid,
			fmt.Sprintf("rekor log %s not trusted", hexKeyID), nil)
	}
	sigVerifier, err := sgsig.LoadVerifier(tlogVerifier.PublicKey, tlogVerifier.SignatureHashFunc)
	if err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"loading rekor log verifier", err)
	}
	if err := tlog.VerifyInclusion(tlogEntry, sigVerifier); err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"rekor inclusion proof verification", err)
	}

	// Fulcio chain validation at the integrated time. One Fulcio CA in the
	// trusted root must accept the cert; others may legitimately reject
	// (different CA generations, different validity windows).
	integratedTime := time.Unix(entry.IntegratedTime, 0).UTC()
	if err := verifyFulcioChain(cert, tr.FulcioCertificateAuthorities(), integratedTime); err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"fulcio chain verification", err)
	}

	// Identity match — exact-equality against pinned profile. Slice 1 does
	// not honor the regex fields; ADR 0007 defers that to slice 2+.
	issuer, subject, err := extractIdentity(cert)
	if err != nil {
		return VerificationResult{}, verifyError(CodeInvalid,
			"extracting identity from cert", err)
	}
	if issuer != profile.Issuer {
		return VerificationResult{}, verifyError(CodeIdentityMismatch,
			fmt.Sprintf("cert issuer does not match pinned profile: got=%q want=%q",
				issuer, profile.Issuer), nil)
	}
	if subject != profile.Subject {
		return VerificationResult{}, verifyError(CodeIdentityMismatch,
			fmt.Sprintf("cert subject does not match pinned profile: got=%q want=%q",
				subject, profile.Subject), nil)
	}

	// GitHub numeric-ID binding. Only enforced when Issuer is the GHA
	// issuer. Non-GitHub issuers skip this check entirely; NumericIDMatched
	// stays false in that case.
	numericIDMatched := false
	if profile.RequiresNumericIDMatch() {
		matched, err := matchNumericIDsFromCert(cert, profile)
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

// verifyFulcioChain attempts to validate cert against each CA in the trusted
// root. A single success is enough; if all CAs reject, the last error is
// surfaced (any one failure reason is representative, and the sigstore-go
// CA type doesn't aggregate).
func verifyFulcioChain(cert *x509.Certificate, cas []root.CertificateAuthority, at time.Time) error {
	if len(cas) == 0 {
		return fmt.Errorf("trusted root has no Fulcio certificate authorities")
	}
	var lastErr error
	for _, ca := range cas {
		if _, err := ca.Verify(cert, at); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("no Fulcio CA accepted the cert at %s: %w", at.Format(time.RFC3339), lastErr)
}

// matchNumericIDsFromCert reads the GitHub OIDC numeric-ID extensions from
// cert and compares them to profile.RepositoryID / profile.RepositoryOwnerID.
// Returns (true, nil) when the profile pinned at least one ID and the cert
// matched. Returns (false, nil) when the profile pinned no IDs (TOFU-capture
// mode — not an error, but the binding did not fire). Returns a VerifyError
// with MOAT_IDENTITY_MISMATCH for any real mismatch.
func matchNumericIDsFromCert(cert *x509.Certificate, profile *SigningProfile) (bool, error) {
	gotRepoID, gotOwnerID, err := readNumericIDsFromCert(cert)
	if err != nil {
		return false, verifyError(CodeIdentityMismatch,
			"reading numeric-ID extensions from cert", err)
	}
	if profile.RepositoryID == "" && profile.RepositoryOwnerID == "" {
		// TOFU-capture mode. Caller can harvest the extensions out-of-band.
		return false, nil
	}
	if profile.RepositoryID != "" && gotRepoID != profile.RepositoryID {
		return false, verifyError(CodeIdentityMismatch,
			fmt.Sprintf("repository_id mismatch: got=%q want=%q",
				gotRepoID, profile.RepositoryID), nil)
	}
	if profile.RepositoryOwnerID != "" && gotOwnerID != profile.RepositoryOwnerID {
		return false, verifyError(CodeIdentityMismatch,
			fmt.Sprintf("repository_owner_id mismatch: got=%q want=%q",
				gotOwnerID, profile.RepositoryOwnerID), nil)
	}
	return true, nil
}

// readNumericIDsFromCert walks cert.Extensions for the immutable Fulcio
// numeric-ID OIDs (1.3.6.1.4.1.57264.1.15, .1.17) and returns their UTF8String
// values. Errors if the cert has neither — a non-conforming GitHub token.
func readNumericIDsFromCert(cert *x509.Certificate) (repoID, ownerID string, err error) {
	for _, ext := range cert.Extensions {
		switch {
		case ext.Id.Equal(fulcioRepoIDOID):
			repoID, err = parseUTF8StringExt(ext.Value)
			if err != nil {
				return "", "", fmt.Errorf("parsing repository_id extension: %w", err)
			}
		case ext.Id.Equal(fulcioRepoOwnerIDOID):
			ownerID, err = parseUTF8StringExt(ext.Value)
			if err != nil {
				return "", "", fmt.Errorf("parsing repository_owner_id extension: %w", err)
			}
		}
	}
	if repoID == "" && ownerID == "" {
		return "", "", fmt.Errorf("cert has no Fulcio numeric-ID extensions (.1.15/.1.17)")
	}
	return repoID, ownerID, nil
}

// parseUTF8StringExt decodes a Fulcio V2 ASN.1 UTF8String extension, falling
// back to raw bytes if the extension uses the legacy V1 encoding (plain
// bytes rather than a wrapping ASN.1 string). sigstore-go's certificate
// summarizer handles this internally; we reproduce the fallback here because
// we're reading the cert directly rather than through a sigstore bundle.
func parseUTF8StringExt(raw []byte) (string, error) {
	var s string
	rest, err := asn1.Unmarshal(raw, &s)
	if err == nil && len(rest) == 0 {
		return s, nil
	}
	if bytes.IndexByte(raw, 0) == -1 && len(raw) > 0 {
		return string(raw), nil
	}
	return "", fmt.Errorf("extension value is neither UTF8String nor legacy bytes")
}

// buildTransparencyLogEntry converts the Rekor API response into a
// protobuf-specs TransparencyLogEntry suitable for tlog.ParseTransparencyLogEntry.
//
// Critical detail: rekor.sigstore.dev runs multiple log shards. The top-level
// `logIndex` in the API response is a global virtual index that monotonically
// counts entries across all shards; the `verification.inclusionProof.logIndex`
// is the shard-local position that tree-size math must use. Populating both
// verbatim from the API response keeps the SET (which hashes the global
// index) and the inclusion proof (which indexes into a shard tree) correct.
func buildTransparencyLogEntry(entry *rekorEntry, body, logID, set []byte) (*rekorv1.TransparencyLogEntry, error) {
	tle := &rekorv1.TransparencyLogEntry{
		LogIndex: entry.LogIndex,
		LogId: &protocommon.LogId{
			KeyId: logID,
		},
		KindVersion: &rekorv1.KindVersion{
			Kind:    "hashedrekord",
			Version: "0.0.1",
		},
		IntegratedTime:    entry.IntegratedTime,
		CanonicalizedBody: body,
	}
	if len(set) > 0 {
		tle.InclusionPromise = &rekorv1.InclusionPromise{
			SignedEntryTimestamp: set,
		}
	}

	p := &entry.Verification.InclusionProof
	if p.TreeSize > 0 && len(p.Hashes) > 0 {
		rootHash, err := hex.DecodeString(p.RootHash)
		if err != nil {
			return nil, fmt.Errorf("decoding inclusion-proof root hash: %w", err)
		}
		hashes := make([][]byte, len(p.Hashes))
		for i, h := range p.Hashes {
			hashes[i], err = hex.DecodeString(h)
			if err != nil {
				return nil, fmt.Errorf("decoding inclusion-proof hash %d: %w", i, err)
			}
		}
		tle.InclusionProof = &rekorv1.InclusionProof{
			LogIndex: p.LogIndex, // shard-local, not entry.LogIndex
			RootHash: rootHash,
			TreeSize: p.TreeSize,
			Hashes:   hashes,
			Checkpoint: &rekorv1.Checkpoint{
				Envelope: p.Checkpoint,
			},
		}
	}
	return tle, nil
}
