package moat

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixtureSignedPayload is the canonical payload format used by meta-registry
// Phase 0 — the same shape Rekor data.hash.value is computed over. Tests
// use a stable literal so the expected sha256 is reproducible.
const fixtureSignedPayload = `{"_version":1,"content_hash":"sha256:3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b"}`

func TestNewLockfile_Invariants(t *testing.T) {
	lf := NewLockfile()
	if lf.Version != LockfileSchemaVersion {
		t.Errorf("version = %d, want %d", lf.Version, LockfileSchemaVersion)
	}
	if lf.Registries == nil {
		t.Error("Registries must be non-nil map")
	}
	if lf.Entries == nil {
		t.Error("Entries must be non-nil slice")
	}
	if lf.RevokedHashes == nil {
		t.Error("RevokedHashes must be non-nil slice")
	}
}

func TestLockfile_RoundTripEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "moat-lockfile.json")

	lf := NewLockfile()
	if err := lf.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadLockfile(path)
	if err != nil {
		t.Fatalf("LoadLockfile: %v", err)
	}
	if loaded.Version != LockfileSchemaVersion {
		t.Errorf("round-trip version: got %d", loaded.Version)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// Spec requires empty slices/maps, not JSON null. Verify the wire.
	if !strings.Contains(string(raw), `"entries": []`) {
		t.Errorf("expected empty entries array in wire, got:\n%s", raw)
	}
	if !strings.Contains(string(raw), `"revoked_hashes": []`) {
		t.Errorf("expected empty revoked_hashes array in wire, got:\n%s", raw)
	}
	if !strings.Contains(string(raw), `"registries": {}`) {
		t.Errorf("expected empty registries object in wire, got:\n%s", raw)
	}
}

func TestLockfile_LoadMissingFileReturnsFresh(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	lf, err := LoadLockfile(path)
	if err != nil {
		t.Fatalf("missing file must return a fresh lockfile, got error: %v", err)
	}
	if lf.Version != LockfileSchemaVersion {
		t.Errorf("fresh lockfile version = %d", lf.Version)
	}
	if len(lf.Entries) != 0 {
		t.Errorf("fresh lockfile has entries")
	}
}

func TestLockfile_ParseRejectsUnknownVersion(t *testing.T) {
	raw := []byte(`{"moat_lockfile_version":99,"entries":[],"revoked_hashes":[]}`)
	_, err := ParseLockfile(raw)
	if err == nil || !strings.Contains(err.Error(), "unsupported moat_lockfile_version") {
		t.Errorf("expected unsupported-version error, got: %v", err)
	}
}

func TestLockfile_ParseRejectsMissingVersion(t *testing.T) {
	raw := []byte(`{"entries":[],"revoked_hashes":[]}`)
	_, err := ParseLockfile(raw)
	if err == nil || !strings.Contains(err.Error(), "moat_lockfile_version") {
		t.Errorf("expected missing-version error, got: %v", err)
	}
}

func TestLockfile_UpgradePathInitializesRegistries(t *testing.T) {
	// Pre-v0.6.0 lockfile shape — no `registries` key. Per spec §Lockfile
	// the client initializes it and sets fetched_at on next successful
	// fetch rather than treating every installed item as stale.
	raw := []byte(`{
		"moat_lockfile_version": 1,
		"entries": [],
		"revoked_hashes": []
	}`)
	lf, err := ParseLockfile(raw)
	if err != nil {
		t.Fatalf("ParseLockfile: %v", err)
	}
	if lf.Registries == nil {
		t.Fatal("Registries must be initialized to empty map, got nil")
	}
	if len(lf.Registries) != 0 {
		t.Errorf("Registries must start empty on upgrade, got %d entries", len(lf.Registries))
	}

	// Simulating the next successful fetch.
	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	lf.SetRegistryFetchedAt("https://example.com/moat-manifest.json", now)
	if lf.Registries["https://example.com/moat-manifest.json"].FetchedAt.IsZero() {
		t.Error("SetRegistryFetchedAt did not record timestamp")
	}
}

func TestLockfile_SetRegistryFetchedAtUTCNormalization(t *testing.T) {
	// The spec requires RFC 3339 UTC timestamps. Verify that a non-UTC
	// input is normalized so cross-client lockfiles don't fight over
	// timezone representation.
	lf := NewLockfile()
	pacific := time.FixedZone("PST", -8*3600)
	local := time.Date(2026, 4, 20, 4, 0, 0, 0, pacific) // 12:00 UTC
	lf.SetRegistryFetchedAt("https://example.com/m.json", local)

	got := lf.Registries["https://example.com/m.json"].FetchedAt
	if got.Location() != time.UTC {
		t.Errorf("expected UTC location, got %v", got.Location())
	}
	if got.Hour() != 12 {
		t.Errorf("expected 12:00 UTC, got %v", got)
	}
}

func TestLockfile_AddEntry_SignedValidates(t *testing.T) {
	lf := NewLockfile()
	payload := fixtureSignedPayload
	expectedHash := sha256HexOf([]byte(payload))

	err := lf.AddEntry(LockEntry{
		Name:              "my-skill",
		Type:              "skill",
		Registry:          "https://example.com/moat-manifest.json",
		ContentHash:       "sha256:3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b",
		TrustTier:         LockTrustTierSigned,
		AttestedAt:        time.Now().UTC(),
		PinnedAt:          time.Now().UTC(),
		AttestationBundle: json.RawMessage(`{"base64Signature":"fake","cert":"fake"}`),
		SignedPayload:     &payload,
	}, expectedHash)
	if err != nil {
		t.Fatalf("AddEntry with correct hash: %v", err)
	}
	if len(lf.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(lf.Entries))
	}
}

func TestLockfile_AddEntry_SignedRejectsHashMismatch(t *testing.T) {
	lf := NewLockfile()
	payload := fixtureSignedPayload
	// A different hash to simulate the Rekor data.hash.value not matching
	// — the attack surface the pre-write check defends against.
	wrongHash := sha256HexOf([]byte("different content"))

	err := lf.AddEntry(LockEntry{
		Name:              "my-skill",
		Type:              "skill",
		Registry:          "https://example.com/m.json",
		ContentHash:       "sha256:deadbeef",
		TrustTier:         LockTrustTierSigned,
		AttestedAt:        time.Now().UTC(),
		PinnedAt:          time.Now().UTC(),
		AttestationBundle: json.RawMessage(`{"fake":"bundle"}`),
		SignedPayload:     &payload,
	}, wrongHash)
	if !errors.Is(err, ErrSignedPayloadHashMismatch) {
		t.Fatalf("expected ErrSignedPayloadHashMismatch, got: %v", err)
	}
	if len(lf.Entries) != 0 {
		t.Errorf("entry must NOT be appended on hash mismatch; lockfile has %d entries", len(lf.Entries))
	}
}

func TestLockfile_AddEntry_DualAttestedRequiresBundle(t *testing.T) {
	lf := NewLockfile()
	payload := fixtureSignedPayload
	expectedHash := sha256HexOf([]byte(payload))

	// Missing AttestationBundle.
	err := lf.AddEntry(LockEntry{
		Name:          "my-skill",
		Type:          "skill",
		Registry:      "https://example.com/m.json",
		ContentHash:   "sha256:abc",
		TrustTier:     LockTrustTierDualAttested,
		AttestedAt:    time.Now().UTC(),
		PinnedAt:      time.Now().UTC(),
		SignedPayload: &payload,
	}, expectedHash)
	if err == nil || !strings.Contains(err.Error(), "attestation_bundle") {
		t.Errorf("expected attestation_bundle required error, got: %v", err)
	}
}

func TestLockfile_AddEntry_UnsignedRejectsPayload(t *testing.T) {
	lf := NewLockfile()
	payload := "should-not-be-here"
	err := lf.AddEntry(LockEntry{
		Name:          "my-skill",
		Type:          "skill",
		Registry:      "https://example.com/m.json",
		ContentHash:   "sha256:abc",
		TrustTier:     LockTrustTierUnsigned,
		AttestedAt:    time.Now().UTC(),
		PinnedAt:      time.Now().UTC(),
		SignedPayload: &payload,
	}, "")
	if err == nil || !strings.Contains(err.Error(), "UNSIGNED") {
		t.Errorf("expected UNSIGNED-with-payload error, got: %v", err)
	}
}

func TestLockfile_AddEntry_UnsignedOK(t *testing.T) {
	lf := NewLockfile()
	err := lf.AddEntry(LockEntry{
		Name:        "unsigned-skill",
		Type:        "skill",
		Registry:    "https://example.com/m.json",
		ContentHash: "sha256:abc",
		TrustTier:   LockTrustTierUnsigned,
		AttestedAt:  time.Now().UTC(),
		PinnedAt:    time.Now().UTC(),
	}, "")
	if err != nil {
		t.Fatalf("AddEntry UNSIGNED: %v", err)
	}
	if len(lf.Entries) != 1 {
		t.Errorf("expected 1 entry")
	}
	// AttestationBundle must be normalized to JSON null so the wire shows
	// `"attestation_bundle": null` rather than omitting the field.
	if !jsonIsNull(lf.Entries[0].AttestationBundle) {
		t.Errorf("UNSIGNED entry AttestationBundle must be JSON null, got %q",
			string(lf.Entries[0].AttestationBundle))
	}
}

func TestLockfile_AddEntry_RejectsUnknownTier(t *testing.T) {
	lf := NewLockfile()
	err := lf.AddEntry(LockEntry{
		Name:        "x",
		Type:        "skill",
		Registry:    "https://example.com/m.json",
		ContentHash: "sha256:abc",
		TrustTier:   "PARTIALLY-ATTESTED", // not in closed set
		AttestedAt:  time.Now().UTC(),
		PinnedAt:    time.Now().UTC(),
	}, "")
	if err == nil || !strings.Contains(err.Error(), "unknown trust_tier") {
		t.Errorf("expected unknown-tier error, got: %v", err)
	}
}

func TestLockfile_AddEntry_RequiresRequiredFields(t *testing.T) {
	lf := NewLockfile()
	err := lf.AddEntry(LockEntry{
		// name deliberately empty
		Type:        "skill",
		Registry:    "https://example.com/m.json",
		ContentHash: "sha256:abc",
		TrustTier:   LockTrustTierUnsigned,
	}, "")
	if err == nil || !strings.Contains(err.Error(), "name") {
		t.Errorf("expected required-field error, got: %v", err)
	}
}

func TestLockfile_Revocation_AddAndCheck(t *testing.T) {
	lf := NewLockfile()
	h := "sha256:abc"

	if lf.IsRevoked(h) {
		t.Error("fresh lockfile must not report revocation")
	}
	lf.AddRevokedHash(h)
	if !lf.IsRevoked(h) {
		t.Error("AddRevokedHash did not update IsRevoked result")
	}

	// Duplicate add must not grow the slice.
	lf.AddRevokedHash(h)
	if len(lf.RevokedHashes) != 1 {
		t.Errorf("duplicate AddRevokedHash grew slice to %d", len(lf.RevokedHashes))
	}
}

func TestLockfile_Interop_RoundTripAgainstSpecShape(t *testing.T) {
	// Construct an entry and verify the serialized JSON uses exactly the
	// field names the spec documents. Cross-client interop is normative
	// — a different client reading this output must find every required
	// key in the correct shape.
	lf := NewLockfile()
	payload := fixtureSignedPayload
	expectedHash := sha256HexOf([]byte(payload))

	attestedAt := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	pinnedAt := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)

	lf.SetRegistryFetchedAt("https://example.com/moat-manifest.json", pinnedAt)
	err := lf.AddEntry(LockEntry{
		Name:              "greeter",
		Type:              "skill",
		Registry:          "https://example.com/moat-manifest.json",
		ContentHash:       "sha256:3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b",
		TrustTier:         LockTrustTierDualAttested,
		AttestedAt:        attestedAt,
		PinnedAt:          pinnedAt,
		AttestationBundle: json.RawMessage(`{"base64Signature":"AA==","cert":"BB=="}`),
		SignedPayload:     &payload,
	}, expectedHash)
	if err != nil {
		t.Fatalf("AddEntry: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "lockfile.json")
	if err := lf.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, _ := os.ReadFile(path)

	// Spot-check every top-level and entry key the spec names REQUIRED.
	wantSubstrings := []string{
		`"moat_lockfile_version": 1`,
		`"registries"`,
		`"https://example.com/moat-manifest.json"`,
		`"fetched_at"`,
		`"entries"`,
		`"name": "greeter"`,
		`"type": "skill"`,
		`"registry": "https://example.com/moat-manifest.json"`,
		`"content_hash": "sha256:`,
		`"trust_tier": "DUAL-ATTESTED"`,
		`"attested_at": "2026-04-01T00:00:00Z"`,
		`"pinned_at": "2026-04-20T12:00:00Z"`,
		`"attestation_bundle"`,
		`"signed_payload"`,
		`"revoked_hashes": []`,
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(string(raw), want) {
			t.Errorf("wire missing %q; got:\n%s", want, raw)
		}
	}

	// Round-trip: parse the emitted bytes and confirm semantic equality
	// on every field that matters for interop.
	reloaded, err := ParseLockfile(raw)
	if err != nil {
		t.Fatalf("round-trip parse: %v", err)
	}
	if reloaded.Entries[0].TrustTier != LockTrustTierDualAttested {
		t.Errorf("round-trip trust_tier lost")
	}
	if *reloaded.Entries[0].SignedPayload != payload {
		t.Errorf("round-trip signed_payload altered")
	}
	// The attestation_bundle must round-trip structurally (internal JSON
	// whitespace may change because json.MarshalIndent re-indents nested
	// RawMessage; cosign's offline verification does not depend on
	// whitespace inside the bundle — it parses the bundle fields and
	// verifies the signature over signed_payload bytes, which DO survive
	// byte-exactly because they're carried as a string).
	var got, want map[string]any
	if err := json.Unmarshal(reloaded.Entries[0].AttestationBundle, &got); err != nil {
		t.Fatalf("reloaded attestation_bundle not valid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(`{"base64Signature":"AA==","cert":"BB=="}`), &want); err != nil {
		t.Fatalf("fixture bundle JSON: %v", err)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("attestation_bundle field %q: got %v, want %v", k, got[k], v)
		}
	}
}

func TestLockfile_SaveAtomicity_NoTempLeftOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".syllago", "moat-lockfile.json")

	lf := NewLockfile()
	if err := lf.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	var tempFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			tempFiles = append(tempFiles, e.Name())
		}
	}
	if len(tempFiles) > 0 {
		t.Errorf("Save left temp files behind: %v", tempFiles)
	}
}

func TestLockfile_LockfilePath(t *testing.T) {
	got := LockfilePath("/proj/root")
	want := filepath.Join("/proj/root", ".syllago", "moat-lockfile.json")
	if got != want {
		t.Errorf("LockfilePath = %q, want %q", got, want)
	}
}

func TestTrustTierLabel(t *testing.T) {
	cases := []struct {
		tier TrustTier
		want string
	}{
		{TrustTierDualAttested, "DUAL-ATTESTED"},
		{TrustTierSigned, "SIGNED"},
		{TrustTierUnsigned, "UNSIGNED"},
	}
	for _, tc := range cases {
		if got := TrustTierLabel(tc.tier); got != tc.want {
			t.Errorf("TrustTierLabel(%v) = %q, want %q", tc.tier, got, tc.want)
		}
	}
}

func TestJSONIsNull(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"null", true},
		{" null", true},
		{"null ", true},
		{"  null\n", true},
		{`{"a":1}`, false},
		{`"null"`, false}, // JSON string "null" is not literal null
		{"", false},
	}
	for _, tc := range cases {
		got := jsonIsNull(json.RawMessage(tc.raw))
		if got != tc.want {
			t.Errorf("jsonIsNull(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}
