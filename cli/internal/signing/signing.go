// Package signing provides cryptographic signing and verification for hook content.
//
// Two signing methods are supported:
//   - Sigstore/cosign (keyless OIDC-based) — primary, frictionless identity
//   - GPG/PGP (traditional key-based) — secondary, for air-gapped environments
//
// This package defines interfaces and types. Implementations are in separate files
// and will be added when external dependencies (sigstore-go, go-crypto) are integrated.
package signing

import "time"

// Method identifies the signing method used.
type Method string

const (
	MethodSigstore Method = "sigstore"
	MethodGPG      Method = "gpg"
)

// Signer signs hook content and produces a SignatureBundle.
type Signer interface {
	// Sign produces a signature for the given content bytes.
	// The identity parameter provides signer identity context (email, key ID, etc.).
	Sign(content []byte, identity string) (*SignatureBundle, error)

	// Method returns which signing method this signer uses.
	Method() Method
}

// Verifier checks signatures against content and trust policies.
type Verifier interface {
	// Verify checks that the signature in the bundle is valid for the content.
	// Returns the verified identity on success, or an error on failure.
	Verify(content []byte, bundle *SignatureBundle) (*VerifiedIdentity, error)

	// Method returns which signing method this verifier handles.
	Method() Method
}

// SignatureBundle holds a signature and its metadata.
// Stored in .syllago.yaml or alongside hook.json as hook.sig.
type SignatureBundle struct {
	Method    Method    `json:"method"`              // "sigstore" or "gpg"
	Signature []byte    `json:"signature"`           // raw signature bytes (base64 in JSON)
	Identity  string    `json:"identity"`            // signer identity (email for sigstore, key ID for GPG)
	Issuer    string    `json:"issuer,omitempty"`    // OIDC issuer (sigstore only)
	Timestamp time.Time `json:"timestamp"`           // when the signature was created
	LogIndex  int64     `json:"logIndex,omitempty"`  // Rekor transparency log index (sigstore only)
	PublicKey []byte    `json:"publicKey,omitempty"` // GPG public key (GPG only)
}

// VerifiedIdentity is the result of successful verification.
type VerifiedIdentity struct {
	Identity string    `json:"identity"` // verified email or key ID
	Issuer   string    `json:"issuer"`   // OIDC issuer or "gpg"
	SignedAt time.Time `json:"signedAt"` // when the content was signed
}

// ContentHash holds per-file SHA256 hashes for integrity verification.
type ContentHash struct {
	File   string `json:"file"`   // relative path within hook directory
	SHA256 string `json:"sha256"` // hex-encoded SHA256 hash
}

// TrustPolicy defines what signatures are required for a trust tier.
type TrustPolicy struct {
	// RequireSignature means content must be signed to install.
	RequireSignature bool `json:"requireSignature"`

	// AllowedMethods restricts which signing methods are accepted.
	// Empty means any method is accepted.
	AllowedMethods []Method `json:"allowedMethods,omitempty"`

	// AllowedIdentities restricts which signer identities are trusted.
	// Supports glob patterns (e.g., "*@acme.com").
	// Empty means any verified identity is accepted.
	AllowedIdentities []string `json:"allowedIdentities,omitempty"`

	// AllowedIssuers restricts which OIDC issuers are trusted (sigstore only).
	// Empty means any issuer is accepted.
	AllowedIssuers []string `json:"allowedIssuers,omitempty"`
}

// RevocationEntry marks a specific hook version or identity as revoked.
type RevocationEntry struct {
	// Type is "hook" (specific hook revoked) or "identity" (all content from identity revoked).
	Type string `json:"type"`

	// HookName identifies the revoked hook (when Type is "hook").
	HookName string `json:"hookName,omitempty"`

	// Identity identifies the revoked signer (when Type is "identity").
	Identity string `json:"identity,omitempty"`

	// Reason explains why the revocation was issued.
	Reason string `json:"reason"`

	// RevokedAt is when the revocation was issued.
	RevokedAt time.Time `json:"revokedAt"`

	// ContentHash is the SHA256 of the specific content version revoked (optional).
	ContentHash string `json:"contentHash,omitempty"`
}

// RevocationList is a collection of revocation entries, typically distributed
// alongside a registry or as a standalone file.
type RevocationList struct {
	Version int               `json:"version"`
	Entries []RevocationEntry `json:"entries"`
}
