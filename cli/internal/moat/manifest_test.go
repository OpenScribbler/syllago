package moat

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Test fixtures: valid-format sha256 digests (64 hex chars). The exact bytes
// don't matter — only the shape is validated by ParseManifest.
const (
	testHashA   = "sha256:" + "1111111111111111111111111111111111111111111111111111111111111111"
	testHashB   = "sha256:" + "2222222222222222222222222222222222222222222222222222222222222222"
	testHashC   = "sha256:" + "3333333333333333333333333333333333333333333333333333333333333333"
	testHashD   = "sha256:" + "4444444444444444444444444444444444444444444444444444444444444444"
	testHashBad = "sha256:" + "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// minimalManifestJSON is the smallest spec-conformant manifest — used as
// the baseline for validation tests that mutate one field at a time.
const minimalManifestJSON = `{
  "schema_version": 1,
  "manifest_uri": "https://example.com/moat-manifest.json",
  "name": "Example Registry",
  "operator": "Example Operator",
  "updated_at": "2026-04-09T00:00:00Z",
  "registry_signing_profile": {
    "issuer": "https://token.actions.githubusercontent.com",
    "subject": "repo:owner/repo:ref:refs/heads/main"
  },
  "content": [],
  "revocations": []
}`

// TestParseManifest_Minimal verifies that the minimal required fields
// round-trip cleanly through ParseManifest and validate.
func TestParseManifest_Minimal(t *testing.T) {
	t.Parallel()

	m, err := ParseManifest([]byte(minimalManifestJSON))
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if m.SchemaVersion != 1 {
		t.Errorf("SchemaVersion = %d; want 1", m.SchemaVersion)
	}
	if m.Name != "Example Registry" {
		t.Errorf("Name = %q; want Example Registry", m.Name)
	}
	if m.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should parse RFC 3339 timestamp")
	}
	if m.RegistrySigningProfile.Issuer == "" {
		t.Error("registry_signing_profile.issuer should parse")
	}
	if m.Content == nil {
		t.Error("Content should be non-nil empty slice")
	}
	if m.Revocations == nil {
		t.Error("Revocations should be non-nil empty slice")
	}
}

// TestParseManifest_FullExample covers the spec example manifest with a
// Signed-tier entry. Lock every parsed field so future refactors catch
// accidental drops.
func TestParseManifest_FullExample(t *testing.T) {
	t.Parallel()

	const full = `{
  "schema_version": 1,
  "manifest_uri": "https://example.com/moat-manifest.json",
  "name": "Example Registry",
  "operator": "Example Operator",
  "updated_at": "2026-04-09T00:00:00Z",
  "self_published": true,
  "registry_signing_profile": {
    "issuer": "https://token.actions.githubusercontent.com",
    "subject": "repo:owner/repo:ref:refs/heads/main"
  },
  "content": [
    {
      "name": "my-skill",
      "display_name": "My Skill",
      "type": "skill",
      "content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111",
      "source_uri": "https://github.com/owner/repo",
      "attested_at": "2026-04-08T00:00:00Z",
      "private_repo": false,
      "rekor_log_index": 1336116369
    }
  ],
  "revocations": [
    {
      "content_hash": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      "reason": "malicious",
      "details_url": "https://example.com/revocations/1"
    }
  ]
}`
	m, err := ParseManifest([]byte(full))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !m.SelfPublished {
		t.Error("self_published should be true")
	}
	if len(m.Content) != 1 {
		t.Fatalf("Content len = %d; want 1", len(m.Content))
	}
	got := m.Content[0]
	if got.Name != "my-skill" || got.Type != "skill" || got.ContentHash != testHashA {
		t.Errorf("content[0] fields unexpected: %+v", got)
	}
	if got.RekorLogIndex == nil || *got.RekorLogIndex != 1336116369 {
		t.Errorf("rekor_log_index = %v; want 1336116369", got.RekorLogIndex)
	}
	if got.TrustTier() != TrustTierSigned {
		t.Errorf("TrustTier() = %v; want Signed", got.TrustTier())
	}
	if len(m.Revocations) != 1 || m.Revocations[0].Reason != RevocationReasonMalicious {
		t.Errorf("revocations[0] unexpected: %+v", m.Revocations)
	}
}

// TestParseManifest_TrustTiers exercises the three normative tiers based on
// the presence/absence of rekor_log_index and per-item signing_profile.
func TestParseManifest_TrustTiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		entryJSON  string
		wantTier   TrustTier
		wantString string
	}{
		{
			name: "unsigned_no_rekor",
			entryJSON: `{
      "name": "item", "display_name": "Item", "type": "skill",
      "content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "source_uri": "https://example.com/r",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false
    }`,
			wantTier:   TrustTierUnsigned,
			wantString: "UNSIGNED",
		},
		{
			name: "signed_rekor_only",
			entryJSON: `{
      "name": "item", "display_name": "Item", "type": "skill",
      "content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "source_uri": "https://example.com/r",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false,
      "rekor_log_index": 42
    }`,
			wantTier:   TrustTierSigned,
			wantString: "SIGNED",
		},
		{
			name: "dual_attested_rekor_and_profile",
			entryJSON: `{
      "name": "item", "display_name": "Item", "type": "skill",
      "content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "source_uri": "https://example.com/r",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false,
      "rekor_log_index": 42,
      "signing_profile": {
        "issuer": "https://token.actions.githubusercontent.com",
        "subject": "repo:pub/pub:ref:refs/heads/main"
      }
    }`,
			wantTier:   TrustTierDualAttested,
			wantString: "DUAL-ATTESTED",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := buildManifest(t, tt.entryJSON)
			m, err := ParseManifest(data)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := m.Content[0].TrustTier(); got != tt.wantTier {
				t.Errorf("TrustTier() = %v; want %v", got, tt.wantTier)
			}
			if got := m.Content[0].TrustTier().String(); got != tt.wantString {
				t.Errorf("TrustTier().String() = %q; want %q", got, tt.wantString)
			}
		})
	}
}

// TestParseManifest_AttestationHashMismatchDowngrade covers the G-13
// contract: when `attestation_hash_mismatch: true` is set, TrustTier() MUST
// return Signed (never Dual-Attested) even if signing_profile is populated.
// The spec has the Registry Action downgrade server-side; this test locks
// the defensive client-side enforcement so a misbehaving or compromised
// registry cannot re-elevate the tier by leaving stale signing_profile on
// a mismatched entry.
//
// Also verifies the bounds of the rule: an Unsigned entry (no rekor index)
// stays Unsigned regardless of the flag — mismatch does not synthesize a
// trust anchor where none existed.
func TestParseManifest_AttestationHashMismatchDowngrade(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		entryJSON string
		wantTier  TrustTier
	}{
		{
			// Publisher attested, registry detected mismatch, registry
			// LEFT signing_profile in the entry. A naive tier check would
			// return DUAL-ATTESTED. The downgrade rule forces SIGNED.
			name: "mismatch_with_signing_profile_downgrades_to_signed",
			entryJSON: `{
      "name": "item", "display_name": "Item", "type": "skill",
      "content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "source_uri": "https://example.com/r",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false,
      "rekor_log_index": 42,
      "signing_profile": {
        "issuer": "https://token.actions.githubusercontent.com",
        "subject": "repo:pub/pub:ref:refs/heads/main"
      },
      "attestation_hash_mismatch": true
    }`,
			wantTier: TrustTierSigned,
		},
		{
			// Mismatch set but no signing_profile — already Signed; rule
			// doesn't flip the tier in either direction. Guards against
			// an accidental Unsigned demotion.
			name: "mismatch_without_signing_profile_stays_signed",
			entryJSON: `{
      "name": "item", "display_name": "Item", "type": "skill",
      "content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "source_uri": "https://example.com/r",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false,
      "rekor_log_index": 42,
      "attestation_hash_mismatch": true
    }`,
			wantTier: TrustTierSigned,
		},
		{
			// No rekor index → Unsigned. Mismatch flag MUST NOT synthesize
			// a trust anchor. This locks the rule's precondition: the
			// downgrade operates on existing Signed/Dual-Attested tiers,
			// not on Unsigned entries.
			name: "mismatch_without_rekor_stays_unsigned",
			entryJSON: `{
      "name": "item", "display_name": "Item", "type": "skill",
      "content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "source_uri": "https://example.com/r",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false,
      "attestation_hash_mismatch": true
    }`,
			wantTier: TrustTierUnsigned,
		},
		{
			// Absence of the flag must still allow Dual-Attested to
			// compute cleanly. Guards against accidental false-default
			// behavior (e.g., treating zero-valued bool as "true").
			name: "no_mismatch_flag_with_signing_profile_dual_attested",
			entryJSON: `{
      "name": "item", "display_name": "Item", "type": "skill",
      "content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "source_uri": "https://example.com/r",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false,
      "rekor_log_index": 42,
      "signing_profile": {
        "issuer": "https://token.actions.githubusercontent.com",
        "subject": "repo:pub/pub:ref:refs/heads/main"
      }
    }`,
			wantTier: TrustTierDualAttested,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := buildManifest(t, tt.entryJSON)
			m, err := ParseManifest(data)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := m.Content[0].TrustTier(); got != tt.wantTier {
				t.Errorf("TrustTier() = %v; want %v (entry: %s)",
					got, tt.wantTier, tt.entryJSON)
			}
		})
	}
}

// TestContentEntry_TrustTierDowngradeDirect exercises the downgrade without
// the JSON parser in the loop. If the parse test fails, this narrows the
// blame to the tier function itself versus struct-tag wiring.
func TestContentEntry_TrustTierDowngradeDirect(t *testing.T) {
	t.Parallel()

	idx := int64(42)
	profile := &SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "repo:pub/pub:ref:refs/heads/main",
	}

	// Baseline: without the flag, rekor + profile → Dual-Attested.
	clean := ContentEntry{RekorLogIndex: &idx, SigningProfile: profile}
	if got := clean.TrustTier(); got != TrustTierDualAttested {
		t.Fatalf("baseline (no mismatch) should be DUAL-ATTESTED, got %v", got)
	}

	// With the flag set, even the identical attestation surface MUST
	// downgrade to Signed. This is the G-13 defensive rule: the client
	// never returns DUAL-ATTESTED when the publisher's attestation does
	// not cover the current content.
	mismatched := clean
	mismatched.AttestationHashMismatch = true
	if got := mismatched.TrustTier(); got != TrustTierSigned {
		t.Errorf("downgrade rule violated: mismatch + profile should be SIGNED, got %v", got)
	}
}

// TestParseManifest_ValidationErrors locks the exact contract for malformed
// manifests. Every case names a REQUIRED field that is missing or invalid.
func TestParseManifest_ValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mutate      func(m map[string]any)
		wantErrSubs string
	}{
		{
			name: "wrong_schema_version",
			mutate: func(m map[string]any) {
				m["schema_version"] = 2
			},
			wantErrSubs: "schema_version",
		},
		{
			name: "missing_manifest_uri",
			mutate: func(m map[string]any) {
				delete(m, "manifest_uri")
			},
			wantErrSubs: "manifest_uri",
		},
		{
			name: "missing_name",
			mutate: func(m map[string]any) {
				delete(m, "name")
			},
			wantErrSubs: "name",
		},
		{
			name: "missing_operator",
			mutate: func(m map[string]any) {
				delete(m, "operator")
			},
			wantErrSubs: "operator",
		},
		{
			name: "missing_updated_at",
			mutate: func(m map[string]any) {
				delete(m, "updated_at")
			},
			wantErrSubs: "updated_at",
		},
		{
			name: "missing_signing_profile_issuer",
			mutate: func(m map[string]any) {
				m["registry_signing_profile"] = map[string]any{"subject": "x"}
			},
			wantErrSubs: "registry_signing_profile.issuer",
		},
		{
			name: "missing_signing_profile_subject",
			mutate: func(m map[string]any) {
				m["registry_signing_profile"] = map[string]any{"issuer": "x"}
			},
			wantErrSubs: "registry_signing_profile.subject",
		},
		{
			name: "missing_content_array",
			mutate: func(m map[string]any) {
				delete(m, "content")
			},
			wantErrSubs: "content",
		},
		{
			name: "missing_revocations_array",
			mutate: func(m map[string]any) {
				delete(m, "revocations")
			},
			wantErrSubs: "revocations",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var base map[string]any
			if err := json.Unmarshal([]byte(minimalManifestJSON), &base); err != nil {
				t.Fatalf("setup decode: %v", err)
			}
			tt.mutate(base)
			data, err := json.Marshal(base)
			if err != nil {
				t.Fatalf("setup re-encode: %v", err)
			}
			_, err = ParseManifest(data)
			if err == nil {
				t.Fatalf("expected error mentioning %q", tt.wantErrSubs)
			}
			if !strings.Contains(err.Error(), tt.wantErrSubs) {
				t.Errorf("error %q does not mention %q", err.Error(), tt.wantErrSubs)
			}
		})
	}
}

// TestParseManifest_ContentEntryValidation exercises per-entry validation.
func TestParseManifest_ContentEntryValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		entry       string
		wantErrSubs string
	}{
		{
			name: "unknown_type",
			entry: `{
      "name": "x", "display_name": "X", "type": "widget",
      "content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "source_uri": "u",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false
    }`,
			wantErrSubs: "closed set",
		},
		{
			name: "missing_content_hash",
			entry: `{
      "name": "x", "display_name": "X", "type": "skill",
      "source_uri": "u",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false
    }`,
			wantErrSubs: "content_hash",
		},
		{
			name: "signing_profile_without_rekor",
			entry: `{
      "name": "x", "display_name": "X", "type": "skill",
      "content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "source_uri": "u",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false,
      "signing_profile": {"issuer": "i", "subject": "s"}
    }`,
			wantErrSubs: "Dual-Attested",
		},
		{
			name: "signing_profile_missing_subject",
			entry: `{
      "name": "x", "display_name": "X", "type": "skill",
      "content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "source_uri": "u",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false,
      "rekor_log_index": 1,
      "signing_profile": {"issuer": "i"}
    }`,
			wantErrSubs: "issuer and subject",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := buildManifest(t, tt.entry)
			_, err := ParseManifest(data)
			if err == nil {
				t.Fatalf("expected error mentioning %q", tt.wantErrSubs)
			}
			if !strings.Contains(err.Error(), tt.wantErrSubs) {
				t.Errorf("error %q does not mention %q", err.Error(), tt.wantErrSubs)
			}
		})
	}
}

// TestParseManifest_NameTypeUniqueness locks the spec §Registry Manifest
// constraint: content[].(name, type) pairs MUST be unique within a single
// manifest. A duplicate is a malformed manifest the client must reject.
func TestParseManifest_NameTypeUniqueness(t *testing.T) {
	t.Parallel()

	const duplicate = `{
      "name": "dup", "display_name": "Dup", "type": "skill",
      "content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "source_uri": "u",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false
    },
    {
      "name": "dup", "display_name": "Dup Again", "type": "skill",
      "content_hash": "sha256:2222222222222222222222222222222222222222222222222222222222222222", "source_uri": "u",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false
    }`
	data := buildManifest(t, duplicate)
	_, err := ParseManifest(data)
	if err == nil {
		t.Fatal("expected name+type uniqueness error")
	}
	if !strings.Contains(err.Error(), "must be unique") {
		t.Errorf("error = %q; want uniqueness message", err)
	}

	// Same name with different type MUST be allowed.
	const sameNameDiffType = `{
      "name": "twin", "display_name": "Twin Skill", "type": "skill",
      "content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "source_uri": "u",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false
    },
    {
      "name": "twin", "display_name": "Twin Agent", "type": "agent",
      "content_hash": "sha256:2222222222222222222222222222222222222222222222222222222222222222", "source_uri": "u",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false
    }`
	data = buildManifest(t, sameNameDiffType)
	if _, err := ParseManifest(data); err != nil {
		t.Errorf("same name + different type should be allowed, got: %v", err)
	}
}

// TestParseManifest_RevocationValidation covers each revocation-field rule.
func TestParseManifest_RevocationValidation(t *testing.T) {
	t.Parallel()

	base := func(r string) []byte {
		return []byte(strings.Replace(minimalManifestJSON,
			`"revocations": []`,
			`"revocations": [`+r+`]`, 1))
	}

	tests := []struct {
		name        string
		rev         string
		wantErrSubs string
	}{
		{
			name:        "missing_content_hash",
			rev:         `{"reason": "malicious", "details_url": "https://x/1"}`,
			wantErrSubs: "content_hash",
		},
		{
			name:        "unknown_reason",
			rev:         `{"content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "reason": "vibes", "details_url": "https://x/1"}`,
			wantErrSubs: "closed set",
		},
		{
			name:        "missing_details_url_registry_source",
			rev:         `{"content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "reason": "malicious"}`,
			wantErrSubs: "details_url",
		},
		{
			name:        "unknown_source",
			rev:         `{"content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "reason": "malicious", "details_url": "https://x/1", "source": "publisher2"}`,
			wantErrSubs: "publisher2",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseManifest(base(tt.rev))
			if err == nil {
				t.Fatalf("expected error mentioning %q", tt.wantErrSubs)
			}
			if !strings.Contains(err.Error(), tt.wantErrSubs) {
				t.Errorf("error = %q; want substring %q", err, tt.wantErrSubs)
			}
		})
	}
}

// TestRevocation_EffectiveSource verifies the "absent → registry" default.
func TestRevocation_EffectiveSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Revocation
		want string
	}{
		{"absent_defaults_registry", Revocation{}, RevocationSourceRegistry},
		{"explicit_registry", Revocation{Source: "registry"}, RevocationSourceRegistry},
		{"explicit_publisher", Revocation{Source: "publisher"}, RevocationSourcePublisher},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.in.EffectiveSource(); got != tt.want {
				t.Errorf("EffectiveSource() = %q; want %q", got, tt.want)
			}
		})
	}
}

// TestParseManifest_UnknownFieldsForwardCompat verifies that unknown
// top-level and per-entry fields are silently accepted. Clients MUST NOT
// reject manifests carrying forward-compatible additions.
func TestParseManifest_UnknownFieldsForwardCompat(t *testing.T) {
	t.Parallel()

	const future = `{
  "schema_version": 1,
  "manifest_uri": "https://example.com/moat-manifest.json",
  "name": "Example Registry",
  "operator": "Example Operator",
  "updated_at": "2026-04-09T00:00:00Z",
  "registry_signing_profile": {
    "issuer": "https://token.actions.githubusercontent.com",
    "subject": "repo:owner/repo:ref:refs/heads/main"
  },
  "new_top_level_field": {"arbitrary": ["payload", 42]},
  "content": [
    {
      "name": "x", "display_name": "X", "type": "skill",
      "content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111", "source_uri": "u",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false,
      "future_annotation": "reserved"
    }
  ],
  "revocations": []
}`
	if _, err := ParseManifest([]byte(future)); err != nil {
		t.Errorf("unknown-field tolerance broken: %v", err)
	}
}

// TestParseManifest_UpdatedAtRFC3339 verifies timestamps round-trip via
// time.Time's JSON unmarshal (RFC 3339).
func TestParseManifest_UpdatedAtRFC3339(t *testing.T) {
	t.Parallel()

	m, err := ParseManifest([]byte(minimalManifestJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want, _ := time.Parse(time.RFC3339, "2026-04-09T00:00:00Z")
	if !m.UpdatedAt.Equal(want) {
		t.Errorf("UpdatedAt = %v; want %v", m.UpdatedAt, want)
	}
}

// buildManifest splices one or more content entries into the minimal
// manifest, returning raw JSON bytes. It replaces the empty content array.
func buildManifest(t *testing.T, entryJSON string) []byte {
	t.Helper()
	out := strings.Replace(minimalManifestJSON, `"content": []`,
		`"content": [`+entryJSON+`]`, 1)
	return []byte(out)
}

// TestContentEntry_IsPrivate covers the per-item accessor. ADR 0007 G-10
// requires conforming clients to read visibility from the per-item flag,
// not from a registry-level default or probe result.
func TestContentEntry_IsPrivate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		entry *ContentEntry
		want  bool
	}{
		{"public_item", &ContentEntry{PrivateRepo: false}, false},
		{"private_item", &ContentEntry{PrivateRepo: true}, true},
		{"nil_receiver_treated_as_public", nil, false},
	}
	for _, tc := range cases {
		if got := tc.entry.IsPrivate(); got != tc.want {
			t.Errorf("%s: IsPrivate() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestParseManifest_PerItemPrivateRepoRoundTrip proves that per-item
// visibility survives JSON round-trip with independent values. This is the
// G-10 headline invariant: a manifest mixing private and public items
// MUST preserve both flags per-entry after parsing. If this regresses,
// the whole manifest would collapse to a single visibility label and
// private items could leak via bulk install flows.
func TestParseManifest_PerItemPrivateRepoRoundTrip(t *testing.T) {
	t.Parallel()
	raw := buildManifest(t, `{
      "name": "public-tool", "display_name": "Public Tool", "type": "skill",
      "content_hash": "sha256:1111111111111111111111111111111111111111111111111111111111111111",
      "source_uri": "https://example.com/pub", "attested_at": "2026-04-08T00:00:00Z",
      "private_repo": false
    },
    {
      "name": "private-tool", "display_name": "Private Tool", "type": "skill",
      "content_hash": "sha256:2222222222222222222222222222222222222222222222222222222222222222",
      "source_uri": "https://example.com/priv", "attested_at": "2026-04-08T00:00:00Z",
      "private_repo": true
    }`)

	m, err := ParseManifest(raw)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if len(m.Content) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(m.Content))
	}

	byName := map[string]ContentEntry{}
	for _, c := range m.Content {
		byName[c.Name] = c
	}

	pub := byName["public-tool"]
	priv := byName["private-tool"]
	if pub.IsPrivate() {
		t.Error("public-tool: IsPrivate()=true, want false — per-item flag collapsed")
	}
	if !priv.IsPrivate() {
		t.Error("private-tool: IsPrivate()=false, want true — per-item flag lost")
	}
}

// TestManifest_HasPrivateContent covers the install/sync-flow gate. A
// mixed-visibility manifest MUST surface the private item, so confirmation
// prompts fire before bulk install.
func TestManifest_HasPrivateContent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		m    *Manifest
		want bool
	}{
		{"nil_manifest", nil, false},
		{"empty_content", &Manifest{}, false},
		{"all_public", &Manifest{Content: []ContentEntry{
			{PrivateRepo: false}, {PrivateRepo: false},
		}}, false},
		{"all_private", &Manifest{Content: []ContentEntry{
			{PrivateRepo: true}, {PrivateRepo: true},
		}}, true},
		{"mixed_first_private", &Manifest{Content: []ContentEntry{
			{PrivateRepo: true}, {PrivateRepo: false},
		}}, true},
		{"mixed_last_private", &Manifest{Content: []ContentEntry{
			{PrivateRepo: false}, {PrivateRepo: true},
		}}, true},
	}
	for _, tc := range cases {
		if got := tc.m.HasPrivateContent(); got != tc.want {
			t.Errorf("%s: HasPrivateContent() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestManifest_PrivateContent_PreservesOrder locks the ordering contract:
// PrivateContent() returns entries in the manifest's own order so
// downstream prompts and summaries present the same layout the publisher
// emitted. Also covers nil receiver → nil and all-public → nil.
func TestManifest_PrivateContent_PreservesOrder(t *testing.T) {
	t.Parallel()
	var nilM *Manifest
	if got := nilM.PrivateContent(); got != nil {
		t.Errorf("nil receiver: PrivateContent() = %v, want nil", got)
	}

	allPublic := &Manifest{Content: []ContentEntry{
		{Name: "a", PrivateRepo: false}, {Name: "b", PrivateRepo: false},
	}}
	if got := allPublic.PrivateContent(); got != nil {
		t.Errorf("all_public: PrivateContent() = %v, want nil", got)
	}

	mixed := &Manifest{Content: []ContentEntry{
		{Name: "first", PrivateRepo: true},
		{Name: "second", PrivateRepo: false},
		{Name: "third", PrivateRepo: true},
		{Name: "fourth", PrivateRepo: false},
	}}
	got := mixed.PrivateContent()
	if len(got) != 2 {
		t.Fatalf("PrivateContent() len = %d, want 2", len(got))
	}
	if got[0].Name != "first" || got[1].Name != "third" {
		t.Errorf("ordering lost: got [%s, %s], want [first, third]", got[0].Name, got[1].Name)
	}
}
