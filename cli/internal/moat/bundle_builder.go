package moat

// bundle_builder.go — Rekor API → sigstore-go Bundle translation.
//
// BuildBundle and VerifyItemSigstore were previously gated to _test.go scope
// (sigstore_spike_test.go) per ADR 0007's read-only verification stance. They
// are promoted here to enable the `moat sign` CLI subcommand (syllago-92i4c)
// and offline smoke-fixture generation without requiring `go test` internals.
//
// The design invariants are unchanged: production MOAT verification still
// consumes .sigstore bundles directly via sgbundle.LoadJSONFromReader; these
// helpers are the construction path for operators who need to produce bundles
// from raw Rekor API responses (e.g. registry publishers, CI tooling, dev-mode
// offline fixture generation).

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protorekor "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	"github.com/sigstore/rekor/pkg/generated/models"
	"github.com/sigstore/rekor/pkg/tle"
	sgbundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

// bundleV03MediaType is the required mediaType for Sigstore bundle v0.3 with
// a single X.509 leaf certificate as verification material. The v0.3 format
// is the only shape sigstore-go accepts for new "keyless" Fulcio signatures.
const bundleV03MediaType = "application/vnd.dev.sigstore.bundle.v0.3+json"

// BuildBundle translates a raw Rekor API response (the exact JSON bytes
// returned by GET /api/v1/log/entries?logIndex=<n>) into a sigstore-go
// Bundle representing the same signing ceremony MOAT's Publisher Action
// performed.
//
// canonicalPayload must be the exact bytes CanonicalPayloadFor produces —
// this becomes the MessageSignature's subject, and sigstore-go's
// WithArtifact reader recomputes the digest over it during verification.
//
// The resulting Bundle is ready to pass to sigstore-go's Verifier.Verify
// with a TrustedRoot (Fulcio + Rekor keys) and a policy constraining the
// expected OIDC identity.
func BuildBundle(rekorRaw []byte, canonicalPayload []byte) (*sgbundle.Bundle, error) {
	entry, err := parseRekorEntry(rekorRaw)
	if err != nil {
		return nil, fmt.Errorf("parsing Rekor entry: %w", err)
	}
	body, err := decodeHashedRekordBody(entry.Body)
	if err != nil {
		return nil, fmt.Errorf("decoding hashedrekord body: %w", err)
	}
	cert, err := extractCert(body)
	if err != nil {
		return nil, fmt.Errorf("extracting cert: %w", err)
	}
	sigBytes, err := base64.StdEncoding.DecodeString(body.Spec.Signature.Content)
	if err != nil {
		return nil, fmt.Errorf("base64 decoding signature: %w", err)
	}

	tlogEntry, err := rekorEntryToTLE(entry)
	if err != nil {
		return nil, fmt.Errorf("converting Rekor entry to TLE: %w", err)
	}

	digest := sha256.Sum256(canonicalPayload)

	pbundle := &protobundle.Bundle{
		MediaType: bundleV03MediaType,
		VerificationMaterial: &protobundle.VerificationMaterial{
			Content: &protobundle.VerificationMaterial_Certificate{
				Certificate: &protocommon.X509Certificate{
					RawBytes: cert.Raw,
				},
			},
			TlogEntries: []*protorekor.TransparencyLogEntry{tlogEntry},
		},
		Content: &protobundle.Bundle_MessageSignature{
			MessageSignature: &protocommon.MessageSignature{
				MessageDigest: &protocommon.HashOutput{
					Algorithm: protocommon.HashAlgorithm_SHA2_256,
					Digest:    digest[:],
				},
				Signature: sigBytes,
			},
		},
	}

	b, err := sgbundle.NewBundle(pbundle)
	if err != nil {
		return nil, fmt.Errorf("constructing sigstore bundle: %w", err)
	}
	return b, nil
}

// rekorEntryToTLE converts our local rekorEntry (parsed from the Rekor API
// JSON response) into a sigstore protobuf TransparencyLogEntry.
//
// We round-trip through models.LogEntryAnon because tle.GenerateTransparencyLogEntry
// is the canonical converter maintained by the Rekor team — reimplementing
// it risks drift as the protobuf spec evolves.
func rekorEntryToTLE(entry *rekorEntry) (*protorekor.TransparencyLogEntry, error) {
	setBytes, err := base64.StdEncoding.DecodeString(entry.Verification.SignedEntryTimestamp)
	if err != nil {
		return nil, fmt.Errorf("base64 decoding SET: %w", err)
	}
	anon := models.LogEntryAnon{
		Body:           entry.Body,
		IntegratedTime: ptr(entry.IntegratedTime),
		LogID:          ptr(entry.LogID),
		LogIndex:       ptr(entry.LogIndex),
		Verification: &models.LogEntryAnonVerification{
			SignedEntryTimestamp: setBytes,
			InclusionProof: &models.InclusionProof{
				Checkpoint: ptr(entry.Verification.InclusionProof.Checkpoint),
				Hashes:     entry.Verification.InclusionProof.Hashes,
				LogIndex:   ptr(entry.Verification.InclusionProof.LogIndex),
				RootHash:   ptr(entry.Verification.InclusionProof.RootHash),
				TreeSize:   ptr(entry.Verification.InclusionProof.TreeSize),
			},
		},
	}
	return tle.GenerateTransparencyLogEntry(anon)
}

// VerifyItemSigstore performs full cryptographic + trust verification of a
// MOAT attestation item using sigstore-go. Fulcio cert chain is checked
// against trusted roots, Rekor inclusion proof and SET are verified, and the
// OIDC identity is constrained to the expected publisher profile.
//
// trustedRootJSON is the contents of a Sigstore trusted_root.json file
// (public-good instance), obtained either via TUF at runtime or bundled
// at build time for offline use.
func VerifyItemSigstore(item AttestationItem, profile SigningProfile, rekorRaw []byte, trustedRootJSON []byte) error {
	payload := CanonicalPayloadFor(item.ContentHash)
	b, err := BuildBundle(rekorRaw, payload)
	if err != nil {
		return fmt.Errorf("building bundle: %w", err)
	}

	tr, err := root.NewTrustedRootFromJSON(trustedRootJSON)
	if err != nil {
		return fmt.Errorf("loading trusted root: %w", err)
	}

	sev, err := verify.NewVerifier(tr,
		verify.WithTransparencyLog(1),
		verify.WithIntegratedTimestamps(1),
		verify.WithSignedCertificateTimestamps(1),
	)
	if err != nil {
		return fmt.Errorf("building verifier: %w", err)
	}

	certID, err := verify.NewShortCertificateIdentity(profile.Issuer, "", profile.Subject, "")
	if err != nil {
		return fmt.Errorf("building certificate identity: %w", err)
	}

	policy := verify.NewPolicy(
		verify.WithArtifact(bytes.NewReader(payload)),
		verify.WithCertificateIdentity(certID),
	)

	if _, err := sev.Verify(b, policy); err != nil {
		return fmt.Errorf("sigstore-go verify: %w", err)
	}
	return nil
}

// ptr returns the address of its argument. Used to populate pointer-typed
// fields in generated Rekor model structs inline.
func ptr[T any](v T) *T { return &v }
