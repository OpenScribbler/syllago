package moat

// MOAT v0.6.0 registry manifest format (spec §Registry Manifest).
//
// A manifest is a signed JSON document served at a stable URL. Every install
// and sync verifies (in order):
//   1. Manifest bytes SHA-256 matches the bundle's subject digest.
//   2. Sigstore/Rekor signature over those bytes.
//   3. registry_signing_profile matches the configured identity.
//   4. Per-item attestation (Rekor log index → canonical payload → hash).
//
// This file handles step-0 parsing. Signing verification lives in
// sigstore_verify.go; per-item verification in verify.go; config-level
// signing-profile matching in the config package.
//
// Schema version discipline: the spec pins schema_version=1 today. Unknown
// versions MUST be rejected — a future bump will add explicit grace-period
// handling here (similar to the attestation payload _version grace period in
// verify.go). Rejecting unknown versions is the safe default.

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ManifestSchemaVersion is the only schema version this client accepts.
const ManifestSchemaVersion = 1

// Revocation reasons — closed set per spec §Registry Manifest.
const (
	RevocationReasonMalicious       = "malicious"
	RevocationReasonCompromised     = "compromised"
	RevocationReasonDeprecated      = "deprecated"
	RevocationReasonPolicyViolation = "policy_violation"
)

// Revocation sources — spec §Revocation Mechanism. Absent defaults to
// "registry" (fail-closed).
const (
	RevocationSourceRegistry  = "registry"
	RevocationSourcePublisher = "publisher"
)

// validRevocationReasons is the closed set for validation.
var validRevocationReasons = map[string]bool{
	RevocationReasonMalicious:       true,
	RevocationReasonCompromised:     true,
	RevocationReasonDeprecated:      true,
	RevocationReasonPolicyViolation: true,
}

// validRevocationSources is the closed set for validation.
var validRevocationSources = map[string]bool{
	RevocationSourceRegistry:  true,
	RevocationSourcePublisher: true,
}

// Manifest is a MOAT registry manifest document. Parse with ParseManifest.
type Manifest struct {
	SchemaVersion          int            `json:"schema_version"`
	ManifestURI            string         `json:"manifest_uri"`
	Name                   string         `json:"name"`
	Operator               string         `json:"operator"`
	UpdatedAt              time.Time      `json:"updated_at"`
	Expires                *time.Time     `json:"expires,omitempty"`
	SelfPublished          bool           `json:"self_published,omitempty"`
	RegistrySigningProfile SigningProfile `json:"registry_signing_profile"`
	Content                []ContentEntry `json:"content"`
	Revocations            []Revocation   `json:"revocations"`
}

// SigningProfile is defined in verify.go (shared between manifest entries
// and per-item attestation). Both the registry-level
// registry_signing_profile and per-item content[].signing_profile use the
// same issuer+subject shape; implementations MUST NOT conflate the two
// semantically, but sharing one struct keeps the JSON unmarshal wiring
// uniform.

// ContentEntry is one row in manifest.content[].
type ContentEntry struct {
	Name                    string          `json:"name"`
	DisplayName             string          `json:"display_name"`
	Type                    string          `json:"type"` // skill|agent|rules|command
	ContentHash             string          `json:"content_hash"`
	SourceURI               string          `json:"source_uri"`
	AttestedAt              time.Time       `json:"attested_at"`
	PrivateRepo             bool            `json:"private_repo"`
	RekorLogIndex           *int64          `json:"rekor_log_index,omitempty"`
	SigningProfile          *SigningProfile `json:"signing_profile,omitempty"`
	DerivedFrom             string          `json:"derived_from,omitempty"`
	Version                 string          `json:"version,omitempty"`
	ScanStatus              json.RawMessage `json:"scan_status,omitempty"`
	AttestationHashMismatch bool            `json:"attestation_hash_mismatch,omitempty"`
}

// TrustTier classifies ContentEntry by the attestation fields present.
// See spec §Trust Tier Determination. This is a computed value, not a
// serialized field.
type TrustTier int

const (
	TrustTierUnsigned     TrustTier = iota // no rekor_log_index
	TrustTierSigned                        // rekor_log_index present, no per-item signing_profile
	TrustTierDualAttested                  // rekor_log_index + signing_profile both present
)

// String returns the normative tier label ("UNSIGNED", "SIGNED",
// "DUAL-ATTESTED") used in the lockfile entries[].trust_tier field.
func (t TrustTier) String() string {
	switch t {
	case TrustTierDualAttested:
		return "DUAL-ATTESTED"
	case TrustTierSigned:
		return "SIGNED"
	case TrustTierUnsigned:
		return "UNSIGNED"
	}
	return "UNKNOWN"
}

// TrustTier computes the trust tier from the entry's attestation fields.
// Absence of rekor_log_index is the Unsigned signal; presence of both
// rekor_log_index AND per-item signing_profile is the Dual-Attested signal.
func (c *ContentEntry) TrustTier() TrustTier {
	if c.RekorLogIndex == nil {
		return TrustTierUnsigned
	}
	if c.SigningProfile != nil {
		return TrustTierDualAttested
	}
	return TrustTierSigned
}

// Revocation is one row in manifest.revocations[].
type Revocation struct {
	ContentHash string `json:"content_hash"`
	Reason      string `json:"reason"` // malicious|compromised|deprecated|policy_violation
	DetailsURL  string `json:"details_url"`
	Source      string `json:"source,omitempty"` // registry|publisher (default: registry)
}

// EffectiveSource returns the revocation source, applying the "absent →
// registry" default from spec §Revocation Mechanism.
func (r *Revocation) EffectiveSource() string {
	if r.Source == "" {
		return RevocationSourceRegistry
	}
	return r.Source
}

// ParseManifest decodes raw bytes into a Manifest and validates structural
// invariants. Bytes are the verbatim response body — the caller is expected
// to hash and verify them against the signature bundle separately.
//
// Validation covers:
//   - schema_version == 1 (unknown versions rejected)
//   - all REQUIRED fields present and non-empty
//   - content[].type in the closed set {skill, agent, rules, command}
//   - (name, type) uniqueness within content[] — spec §Registry Manifest
//   - revocations[].reason in the closed set
//   - revocations[].source if present in {registry, publisher}
//   - Signed and Dual-Attested tiers have the required attestation fields
//
// Unknown fields are accepted (forward-compatibility for additive changes);
// structural violations like missing required fields or malformed timestamps
// return a descriptive error.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest json: %w", err)
	}
	if err := m.validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Manifest) validate() error {
	if m.SchemaVersion != ManifestSchemaVersion {
		return fmt.Errorf("manifest schema_version %d: only %d is supported",
			m.SchemaVersion, ManifestSchemaVersion)
	}
	if m.ManifestURI == "" {
		return errors.New("manifest missing required field: manifest_uri")
	}
	if m.Name == "" {
		return errors.New("manifest missing required field: name")
	}
	if m.Operator == "" {
		return errors.New("manifest missing required field: operator")
	}
	if m.UpdatedAt.IsZero() {
		return errors.New("manifest missing required field: updated_at")
	}
	if m.RegistrySigningProfile.Issuer == "" {
		return errors.New("manifest missing required field: registry_signing_profile.issuer")
	}
	if m.RegistrySigningProfile.Subject == "" {
		return errors.New("manifest missing required field: registry_signing_profile.subject")
	}
	if m.Content == nil {
		return errors.New("manifest missing required field: content")
	}
	if m.Revocations == nil {
		return errors.New("manifest missing required field: revocations (use [] if none)")
	}

	seen := make(map[string]int) // "name|type" → first index for collision diagnostics
	for i := range m.Content {
		if err := m.Content[i].validate(i); err != nil {
			return err
		}
		key := m.Content[i].Name + "|" + m.Content[i].Type
		if prior, dup := seen[key]; dup {
			return fmt.Errorf("manifest content[%d]: (name=%q, type=%q) collides with content[%d] — (name, type) must be unique",
				i, m.Content[i].Name, m.Content[i].Type, prior)
		}
		seen[key] = i
	}

	for i := range m.Revocations {
		if err := m.Revocations[i].validate(i); err != nil {
			return err
		}
	}

	return nil
}

func (c *ContentEntry) validate(idx int) error {
	if c.Name == "" {
		return fmt.Errorf("manifest content[%d]: missing required field: name", idx)
	}
	if c.DisplayName == "" {
		return fmt.Errorf("manifest content[%d] (%s): missing required field: display_name", idx, c.Name)
	}
	if _, ok := FromMOATType(c.Type); !ok {
		return fmt.Errorf("manifest content[%d] (%s): type %q not in closed set {skill, agent, rules, command}",
			idx, c.Name, c.Type)
	}
	if c.ContentHash == "" {
		return fmt.Errorf("manifest content[%d] (%s): missing required field: content_hash", idx, c.Name)
	}
	if _, _, err := ParseContentHash(c.ContentHash); err != nil {
		return fmt.Errorf("manifest content[%d] (%s): %w", idx, c.Name, err)
	}
	if c.SourceURI == "" {
		return fmt.Errorf("manifest content[%d] (%s): missing required field: source_uri", idx, c.Name)
	}
	if c.AttestedAt.IsZero() {
		return fmt.Errorf("manifest content[%d] (%s): missing required field: attested_at", idx, c.Name)
	}
	// Dual-Attested requires rekor_log_index AND signing_profile.
	if c.SigningProfile != nil && c.RekorLogIndex == nil {
		return fmt.Errorf("manifest content[%d] (%s): signing_profile present without rekor_log_index — Dual-Attested tier requires both",
			idx, c.Name)
	}
	if c.SigningProfile != nil {
		if c.SigningProfile.Issuer == "" || c.SigningProfile.Subject == "" {
			return fmt.Errorf("manifest content[%d] (%s): signing_profile requires both issuer and subject", idx, c.Name)
		}
	}
	return nil
}

func (r *Revocation) validate(idx int) error {
	if r.ContentHash == "" {
		return fmt.Errorf("manifest revocations[%d]: missing required field: content_hash", idx)
	}
	if _, _, err := ParseContentHash(r.ContentHash); err != nil {
		return fmt.Errorf("manifest revocations[%d]: %w", idx, err)
	}
	if !validRevocationReasons[r.Reason] {
		return fmt.Errorf("manifest revocations[%d]: reason %q not in closed set {malicious, compromised, deprecated, policy_violation}",
			idx, r.Reason)
	}
	// details_url is REQUIRED for registry-source revocations, OPTIONAL for
	// publisher-source. EffectiveSource handles the absent→registry default.
	if r.EffectiveSource() == RevocationSourceRegistry && r.DetailsURL == "" {
		return fmt.Errorf("manifest revocations[%d]: details_url required for registry-source revocations", idx)
	}
	if r.Source != "" && !validRevocationSources[r.Source] {
		return fmt.Errorf("manifest revocations[%d]: source %q not in {registry, publisher}", idx, r.Source)
	}
	return nil
}
