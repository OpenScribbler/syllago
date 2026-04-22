package moat

import (
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"fmt"
)

// Fulcio encodes the OIDC issuer as an X.509 extension. The V2 OID uses an
// RFC 5280-compliant ASN.1 UTF8String; the legacy V1 OID stores the issuer
// URI as raw bytes. New Fulcio certs include both for compatibility.
var (
	fulcioIssuerV2OID     = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 8}
	fulcioIssuerLegacyOID = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}
)

// extractCert parses the signing certificate from a hashedrekord body's
// spec.signature.publicKey.content field. The content is base64-encoded PEM
// — double-decode (outer base64 from the hashedrekord wrapper, inner PEM
// from Fulcio) is required.
func extractCert(body *hashedrekordBody) (*x509.Certificate, error) {
	pemBytes, err := base64.StdEncoding.DecodeString(body.Spec.Signature.PublicKey.Content)
	if err != nil {
		return nil, fmt.Errorf("base64 decoding publicKey.content: %w", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in publicKey.content")
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("expected CERTIFICATE PEM block, got %q", block.Type)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing certificate: %w", err)
	}
	return cert, nil
}

// ExtractIdentityFromRekorRaw parses a raw Rekor API JSON response and returns
// the OIDC (issuer, subject) pair from the Fulcio certificate embedded in the
// hashedrekord entry. Used by `moat sign` to auto-derive the signing profile
// when the caller does not supply an explicit --identity file.
func ExtractIdentityFromRekorRaw(rekorRaw []byte) (issuer, subject string, err error) {
	entry, err := parseRekorEntry(rekorRaw)
	if err != nil {
		return "", "", fmt.Errorf("parsing rekor entry: %w", err)
	}
	body, err := decodeHashedRekordBody(entry.Body)
	if err != nil {
		return "", "", fmt.Errorf("decoding hashedrekord body: %w", err)
	}
	cert, err := extractCert(body)
	if err != nil {
		return "", "", fmt.Errorf("extracting cert: %w", err)
	}
	return extractIdentity(cert)
}

// extractIdentity pulls the OIDC issuer and subject from a Fulcio-issued
// certificate, matching the SigningProfile fields. Subject is the first URI
// SAN (workflow ref for GitHub Actions); issuer is the Fulcio issuer
// extension (https://token.actions.githubusercontent.com for GitHub).
func extractIdentity(cert *x509.Certificate) (issuer, subject string, err error) {
	if len(cert.URIs) == 0 {
		return "", "", fmt.Errorf("no URI SANs in certificate")
	}
	subject = cert.URIs[0].String()

	for _, ext := range cert.Extensions {
		if ext.Id.Equal(fulcioIssuerV2OID) {
			var s string
			if _, err := asn1.Unmarshal(ext.Value, &s); err != nil {
				return "", "", fmt.Errorf("parsing Fulcio issuer V2 extension: %w", err)
			}
			return s, subject, nil
		}
	}

	for _, ext := range cert.Extensions {
		if ext.Id.Equal(fulcioIssuerLegacyOID) {
			return string(ext.Value), subject, nil
		}
	}

	return "", "", fmt.Errorf("no Fulcio issuer extension (V2 OID %s, legacy OID %s)", fulcioIssuerV2OID, fulcioIssuerLegacyOID)
}
