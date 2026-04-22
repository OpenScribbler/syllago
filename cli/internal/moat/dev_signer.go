package moat

// dev_signer.go — offline signing for smoke fixtures and dev-mode testing.
//
// SignManifestDev uses sigstore-go's VirtualSigstore (a full offline CA that
// emits real sigstore bundle structures without hitting any external service)
// to produce a matched (bundleJSON, trustedRootJSON) pair for a given manifest.
//
// The output is designed to round-trip through VerifyManifest and be accepted
// by `syllago registry add --moat --trusted-root <dev-root>`. It is NOT
// suitable for production use — the CA keys are ephemeral, the root is not
// anchored in any public TUF repository, and the issuer is a placeholder.

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	protorekor "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	"github.com/sigstore/rekor/pkg/generated/models"
	sgbundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/testing/ca"
	"github.com/sigstore/sigstore-go/pkg/tlog"
	"github.com/sigstore/sigstore-go/pkg/verify"
)

// DevSigningIdentity is the OIDC issuer + subject written into Fulcio leaf
// certs produced by SignManifestDev. They are recognizable placeholders that
// make it obvious when a bundle was produced by the dev signer rather than a
// real GitHub Actions workflow.
const (
	DevSigningIssuer  = "https://dev.syllago.local/oidc"
	DevSigningSubject = "https://dev.syllago.local/workflow/smoke@refs/heads/main"
)

// SignManifestDev signs manifestBytes with an ephemeral offline CA and returns
// the sigstore bundle JSON, the matching trusted_root.json bytes, and the
// SigningProfile that identifies the dev signer.
//
// The returned SigningProfile is suitable for use with VerifyManifest:
//
//	_, err := VerifyManifest(manifestBytes, bundleJSON, &profile, trustedRootJSON)
func SignManifestDev(manifestBytes []byte) (bundleJSON, trustedRootJSON []byte, profile SigningProfile, err error) {
	vs, err := ca.NewVirtualSigstore()
	if err != nil {
		return nil, nil, SigningProfile{}, fmt.Errorf("creating virtual sigstore: %w", err)
	}

	entity, err := vs.Sign(DevSigningSubject, DevSigningIssuer, manifestBytes)
	if err != nil {
		return nil, nil, SigningProfile{}, fmt.Errorf("signing manifest: %w", err)
	}

	bundleJSON, err = entityToBundleJSON(vs, entity)
	if err != nil {
		return nil, nil, SigningProfile{}, fmt.Errorf("assembling bundle: %w", err)
	}

	// VirtualSigstore.RekorLogs() and CTLogs() set ID: []byte(hexString) but
	// root.TrustedRoot.RekorLogs() does hex.EncodeToString(KeyId) to build
	// map keys. After a JSON round-trip the keys become double-hex-encoded,
	// breaking VerifySET lookups. Fix: decode the hex-string ID to binary.
	rekorLogs := vs.RekorLogs()
	for _, tl := range rekorLogs {
		if binary, decErr := hex.DecodeString(string(tl.ID)); decErr == nil {
			tl.ID = binary
		}
	}
	ctLogs := vs.CTLogs()
	for _, tl := range ctLogs {
		if binary, decErr := hex.DecodeString(string(tl.ID)); decErr == nil {
			tl.ID = binary
		}
	}
	tr, err := root.NewTrustedRoot(
		root.TrustedRootMediaType01,
		vs.FulcioCertificateAuthorities(),
		ctLogs,
		vs.TimestampingAuthorities(),
		rekorLogs,
	)
	if err != nil {
		return nil, nil, SigningProfile{}, fmt.Errorf("building trusted root: %w", err)
	}
	trustedRootJSON, err = tr.MarshalJSON()
	if err != nil {
		return nil, nil, SigningProfile{}, fmt.Errorf("marshaling trusted root: %w", err)
	}

	profile = SigningProfile{
		Issuer:  DevSigningIssuer,
		Subject: DevSigningSubject,
	}
	return bundleJSON, trustedRootJSON, profile, nil
}

// entityToBundleJSON converts a verify.SignedEntity produced by
// VirtualSigstore.Sign into a sigstore v0.3 bundle JSON. It patches the TLE
// from VirtualSigstore's deprecated NewEntry path (which omits KindVersion,
// InclusionPromise, and InclusionProof) so that the bundle passes the
// sgbundle.NewBundle v0.3 validation requirements.
func entityToBundleJSON(vs *ca.VirtualSigstore, entity verify.SignedEntity) ([]byte, error) {
	vc, err := entity.VerificationContent()
	if err != nil {
		return nil, fmt.Errorf("reading verification content: %w", err)
	}
	cert := vc.Certificate()
	if cert == nil {
		return nil, fmt.Errorf("entity has no leaf certificate")
	}

	sc, err := entity.SignatureContent()
	if err != nil {
		return nil, fmt.Errorf("reading signature content: %w", err)
	}
	msgSig := sc.MessageSignatureContent()
	if msgSig == nil {
		return nil, fmt.Errorf("entity has no message signature content")
	}

	tlogEntries, err := entity.TlogEntries()
	if err != nil {
		return nil, fmt.Errorf("reading tlog entries: %w", err)
	}
	if len(tlogEntries) == 0 {
		return nil, fmt.Errorf("entity has no transparency log entries")
	}

	entry := tlogEntries[0]
	tle := entry.TransparencyLogEntry()
	if tle == nil {
		return nil, fmt.Errorf("tlog entry has no protobuf representation")
	}

	// VirtualSigstore's deprecated NewEntry path leaves KindVersion nil.
	// ParseTransparencyLogEntry (called by sgbundle.NewBundle's validate)
	// requires a non-nil KindVersion, so we patch it here. The value is
	// always "hashedrekord/0.0.1" for entries produced by VirtualSigstore.Sign.
	if tle.KindVersion == nil {
		tle.KindVersion = &protorekor.KindVersion{
			Kind:    "hashedrekord",
			Version: "0.0.1",
		}
	}

	// VirtualSigstore.Sign uses the deprecated tlog.NewEntry path that stores
	// the SET in a private Go field inaccessible from outside the tlog package.
	// entry.Signature() returns the artifact signature (from the hashedrekord
	// body), not the SET. We regenerate a valid SET by reconstructing the
	// exact RekorPayload that was originally signed and calling RekorSignPayload
	// with the same VirtualSigstore Rekor key. The new SET differs in bytes
	// (ECDSA uses a fresh random k) but verifies correctly against the
	// trusted root built from the same VirtualSigstore instance.
	if tle.InclusionPromise == nil {
		payload := tlog.RekorPayload{
			Body:           base64.StdEncoding.EncodeToString(tle.CanonicalizedBody),
			IntegratedTime: tle.IntegratedTime,
			LogIndex:       tle.LogIndex,
			LogID:          hex.EncodeToString(tle.GetLogId().GetKeyId()),
		}
		set, err := vs.RekorSignPayload(payload)
		if err != nil {
			return nil, fmt.Errorf("generating SET: %w", err)
		}
		tle.InclusionPromise = &protorekor.InclusionPromise{
			SignedEntryTimestamp: set,
		}
	}

	// Bundle v0.3 requires an inclusion proof. VirtualSigstore.Sign does not
	// produce one by default, so we generate it via GetInclusionProof.
	if tle.InclusionProof == nil && tle.CanonicalizedBody != nil {
		proof, err := vs.GetInclusionProof(tle.CanonicalizedBody)
		if err != nil {
			return nil, fmt.Errorf("generating inclusion proof: %w", err)
		}
		// Use proof.LogIndex (position within the Merkle tree), not
		// entry.LogIndex() (VirtualSigstore's hardcoded global index 1000).
		protoProof, err := modelsInclusionProofToProto(proof, *proof.LogIndex)
		if err != nil {
			return nil, fmt.Errorf("converting inclusion proof: %w", err)
		}
		tle.InclusionProof = protoProof
	}

	pbundle := &protobundle.Bundle{
		MediaType: bundleV03MediaType,
		VerificationMaterial: &protobundle.VerificationMaterial{
			Content: &protobundle.VerificationMaterial_Certificate{
				Certificate: &protocommon.X509Certificate{
					RawBytes: cert.Raw,
				},
			},
			TlogEntries: []*protorekor.TransparencyLogEntry{tle},
		},
		Content: &protobundle.Bundle_MessageSignature{
			MessageSignature: &protocommon.MessageSignature{
				MessageDigest: &protocommon.HashOutput{
					Algorithm: protocommon.HashAlgorithm_SHA2_256,
					Digest:    msgSig.Digest(),
				},
				Signature: msgSig.Signature(),
			},
		},
	}

	b, err := sgbundle.NewBundle(pbundle)
	if err != nil {
		return nil, fmt.Errorf("constructing sigstore bundle: %w", err)
	}
	return b.MarshalJSON()
}

// modelsInclusionProofToProto converts a rekor models.InclusionProof
// (returned by VirtualSigstore.GetInclusionProof) into the sigstore protobuf
// InclusionProof shape. Mirrors the conversion in tlog.NewEntry.
func modelsInclusionProofToProto(proof *models.InclusionProof, logIndex int64) (*protorekor.InclusionProof, error) {
	hashes := make([][]byte, len(proof.Hashes))
	for i, s := range proof.Hashes {
		h, err := hex.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("decoding hash[%d]: %w", i, err)
		}
		hashes[i] = h
	}
	rootHash, err := hex.DecodeString(*proof.RootHash)
	if err != nil {
		return nil, fmt.Errorf("decoding root hash: %w", err)
	}
	return &protorekor.InclusionProof{
		LogIndex: logIndex,
		RootHash: rootHash,
		TreeSize: *proof.TreeSize,
		Hashes:   hashes,
		Checkpoint: &protorekor.Checkpoint{
			Envelope: *proof.Checkpoint,
		},
	}, nil
}

// devVerifyManifest verifies a bundle produced by SignManifestDev. It accepts
// both inclusion proofs AND inclusion promises (signed entry timestamps) by
// using WithObserverTimestamps instead of WithIntegratedTimestamps.
//
// This is separate from VerifyManifest so the relaxed dev-mode policy does
// not affect production callers.
func devVerifyManifest(manifestBytes, bundleJSON, trustedRootJSON []byte, profile *SigningProfile) error {
	if profile == nil {
		return fmt.Errorf("signing profile required")
	}
	tr, err := root.NewTrustedRootFromJSON(trustedRootJSON)
	if err != nil {
		return fmt.Errorf("loading trusted root: %w", err)
	}
	bundle := &sgbundle.Bundle{}
	if err := bundle.UnmarshalJSON(bundleJSON); err != nil {
		return fmt.Errorf("parsing bundle: %w", err)
	}
	sev, err := verify.NewVerifier(tr,
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1),
	)
	if err != nil {
		return fmt.Errorf("building verifier: %w", err)
	}
	certID, err := verify.NewShortCertificateIdentity(profile.Issuer, "", profile.Subject, "")
	if err != nil {
		return fmt.Errorf("building cert identity: %w", err)
	}
	policy := verify.NewPolicy(
		verify.WithArtifact(bytesReader(manifestBytes)),
		verify.WithCertificateIdentity(certID),
	)
	if _, err := sev.Verify(bundle, policy); err != nil {
		return fmt.Errorf("dev verify: %w", err)
	}
	return nil
}

// bytesReader converts a byte slice to an io.Reader inline.
func bytesReader(b []byte) *bytesSliceReader { return &bytesSliceReader{b: b, pos: 0} }

type bytesSliceReader struct {
	b   []byte
	pos int
}

func (r *bytesSliceReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.b) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.b[r.pos:])
	r.pos += n
	return n, nil
}

// DevTrustedRootInfo wraps the generated trusted root bytes with the validity
// window (always "now" since VirtualSigstore keys are ephemeral).
func devTrustedRootInfo(trustedRootJSON []byte) TrustedRootInfo {
	return TrustedRootInfo{
		Source: TrustedRootSourceDev,
		Status: TrustedRootStatusFresh,
		Bytes:  trustedRootJSON,
	}
}

// TrustedRootSourceDev identifies a dev trusted root generated by SignManifestDev.
const TrustedRootSourceDev TrustedRootSource = "dev"

// devIssuedAt is the zero time — dev roots are ephemeral and have no issuance date.
var devIssuedAt = time.Time{}
