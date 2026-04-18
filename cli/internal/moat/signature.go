package moat

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
)

// verifySignature checks that the signature from a hashedrekord body was
// produced by the holder of cert's private key over payload. Uses ECDSA over
// SHA-256 — the only algorithm Fulcio currently issues for keyless signing.
//
// Signature bytes are ASN.1 DER-encoded; Go's ecdsa.VerifyASN1 handles that
// decoding. The signed digest is sha256(payload) — matching what Rekor
// records as data.hash.value.
//
// This function does not validate the cert chain back to a Fulcio root or
// the Rekor inclusion proof. Those are separate trust checks layered on
// top of raw signature validity (see verify.go package docstring).
func verifySignature(cert *x509.Certificate, body *hashedrekordBody, payload []byte) error {
	sigBytes, err := base64.StdEncoding.DecodeString(body.Spec.Signature.Content)
	if err != nil {
		return fmt.Errorf("base64 decoding signature: %w", err)
	}

	pubKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("expected ECDSA public key, got %T", cert.PublicKey)
	}

	digest := sha256.Sum256(payload)
	if !ecdsa.VerifyASN1(pubKey, digest[:], sigBytes) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}
