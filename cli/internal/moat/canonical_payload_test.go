package moat

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestCanonicalPayloadFor locks the byte-exact canonical payload the Publisher
// Action hashes and signs. Any drift — field order, whitespace, _version type,
// JSON escaping — silently breaks every downstream signature verification.
func TestCanonicalPayloadFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		hash string
		want string
	}{
		{
			// syllago-guide's content_hash from testdata/moat-attestation.json,
			// signed into Rekor at logIndex 1336116369 by the meta-registry
			// Phase 0 Publisher Action run.
			name: "fixture_syllago_guide",
			hash: "sha256:f997b299344032fb6f12c80b86dffad33a1ad2ec0c23bd2476ce3d4c8781a6f2",
			want: `{"_version":1,"content_hash":"sha256:f997b299344032fb6f12c80b86dffad33a1ad2ec0c23bd2476ce3d4c8781a6f2"}`,
		},
		{
			name: "empty_hash",
			hash: "",
			want: `{"_version":1,"content_hash":""}`,
		},
		{
			// MOAT v0.6.0 normative test vector (bead syllago-o3czr). The payload
			// bytes below are the normative serialization that every conforming
			// implementation must produce; see the sha256 assertion below.
			name: "v0_6_0_normative_vector",
			hash: "sha256:3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b",
			want: `{"_version":1,"content_hash":"sha256:3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b"}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := CanonicalPayloadFor(tt.hash)
			if !bytes.Equal(got, []byte(tt.want)) {
				t.Errorf("payload mismatch\ngot:  %s\nwant: %s", got, tt.want)
			}
		})
	}
}

// TestCanonicalPayloadFor_NormativeDigest checks the SHA-256 digest of the
// v0.6.0 normative test vector payload. The payload-bytes test above would
// catch a regression in the byte sequence; this test additionally locks the
// hash value that must match what Rekor records for a signed attestation —
// so a regression caught here means downstream Rekor verification would
// already be broken for this input. Belt and suspenders.
func TestCanonicalPayloadFor_NormativeDigest(t *testing.T) {
	t.Parallel()
	const (
		inputHash   = "sha256:3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b"
		wantDigest  = "b7d70330da474c9d32efe29dd4e23c4a0901a7ca222e12bdbc84d17e4e5f69a4"
		wantPayload = `{"_version":1,"content_hash":"sha256:3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b"}`
	)

	payload := CanonicalPayloadFor(inputHash)
	if string(payload) != wantPayload {
		t.Fatalf("canonical payload drift:\n  got:  %s\n  want: %s", payload, wantPayload)
	}

	sum := sha256.Sum256(payload)
	if got := hex.EncodeToString(sum[:]); got != wantDigest {
		t.Errorf("SHA-256 digest of normative payload mismatch:\n  got:  %s\n  want: %s", got, wantDigest)
	}
}

// TestCurrentPayloadVersion_IsOne is a drift guard: a change to this
// constant is a spec change (grace-period transition), not a tuning knob.
// If this test fails, either the spec moved or the constant was altered
// unintentionally. Either way, SupportedPayloadVersions and every fixture
// anchored on `_version:1` need a coordinated review.
func TestCurrentPayloadVersion_IsOne(t *testing.T) {
	t.Parallel()
	if CurrentPayloadVersion != 1 {
		t.Errorf("CurrentPayloadVersion = %d, want 1 (spec v0.6.0)", CurrentPayloadVersion)
	}
}

// TestSupportedPayloadVersions_OnlyCurrentToday captures today's (pre-bump)
// state: exactly one accepted version. When a grace period opens, this
// test needs to be updated to assert both prior and current are accepted —
// the failure message should make the grace-period intent explicit.
func TestSupportedPayloadVersions_OnlyCurrentToday(t *testing.T) {
	t.Parallel()
	if len(SupportedPayloadVersions) != 1 {
		t.Errorf("SupportedPayloadVersions length = %d, want 1 outside grace period",
			len(SupportedPayloadVersions))
	}
	if SupportedPayloadVersions[0] != CurrentPayloadVersion {
		t.Errorf("SupportedPayloadVersions[0] = %d, want CurrentPayloadVersion=%d",
			SupportedPayloadVersions[0], CurrentPayloadVersion)
	}
}

// TestIsSupportedPayloadVersion covers the ordering-step-2 gate: recognized
// versions return true, everything else false. An unknown version is a
// hard reject per spec §Version Transition after the grace period; during
// a future grace, new values would be added to SupportedPayloadVersions,
// and this test would be extended to assert both.
func TestIsSupportedPayloadVersion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		v    int
		want bool
	}{
		{1, true}, // current
		{0, false},
		{2, false}, // future, not yet in-grace
		{-1, false},
		{999, false},
	}
	for _, tc := range cases {
		if got := IsSupportedPayloadVersion(tc.v); got != tc.want {
			t.Errorf("IsSupportedPayloadVersion(%d) = %v, want %v", tc.v, got, tc.want)
		}
	}
}

// TestCanonicalPayloadForVersion_V1_MatchesShortcut proves that the
// CanonicalPayloadFor shortcut and CanonicalPayloadForVersion(1, hash)
// produce byte-exact identical output. A regression here means the v1
// short-circuit silently drifted from the versioned builder — which
// would break Rekor fixture anchors used by sigstore_verify_test.go
// and rekor_test.go.
func TestCanonicalPayloadForVersion_V1_MatchesShortcut(t *testing.T) {
	t.Parallel()
	const hash = "sha256:f997b299344032fb6f12c80b86dffad33a1ad2ec0c23bd2476ce3d4c8781a6f2"
	shortcut := CanonicalPayloadFor(hash)
	versioned, ok := CanonicalPayloadForVersion(CurrentPayloadVersion, hash)
	if !ok {
		t.Fatalf("CanonicalPayloadForVersion(%d, _) returned ok=false for current version",
			CurrentPayloadVersion)
	}
	if !bytes.Equal(shortcut, versioned) {
		t.Errorf("shortcut/versioned drift:\n  CanonicalPayloadFor:        %s\n  CanonicalPayloadForVersion: %s",
			shortcut, versioned)
	}
}

// TestCanonicalPayloadForVersion_UnknownReturnsNil locks the rejection
// contract: an unsupported version MUST yield (nil, false) — never a
// partial payload, never a panic. A verifier that gets (nil, false) knows
// to reject; a verifier that gets non-nil bytes knows the version was
// pre-approved and the bytes are safe to compare against Rekor.
//
// Covers the TOCTOU-defense: if the dispatcher were to fall through to
// `v:1` on any unknown v, the TOCTOU window would reopen.
func TestCanonicalPayloadForVersion_UnknownReturnsNil(t *testing.T) {
	t.Parallel()
	const hash = "sha256:f997b299344032fb6f12c80b86dffad33a1ad2ec0c23bd2476ce3d4c8781a6f2"
	for _, v := range []int{0, 2, 99, -1} {
		payload, ok := CanonicalPayloadForVersion(v, hash)
		if ok {
			t.Errorf("CanonicalPayloadForVersion(%d, _) ok = true, want false", v)
		}
		if payload != nil {
			t.Errorf("CanonicalPayloadForVersion(%d, _) payload = %q, want nil", v, payload)
		}
	}
}

// TestCanonicalPayloadForVersion_VersionAppearsFirst is a structural
// invariant: the `_version` field MUST appear before `content_hash` in
// every supported version's serialized output. Publishers hash these
// exact bytes; if Go's default struct serializer were ever substituted
// here and alphabetized the fields, `content_hash` would come first and
// every signature would fail. This test catches that class of regression
// across any version the slice carries.
func TestCanonicalPayloadForVersion_VersionAppearsFirst(t *testing.T) {
	t.Parallel()
	const hash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	for _, v := range SupportedPayloadVersions {
		payload, ok := CanonicalPayloadForVersion(v, hash)
		if !ok {
			t.Fatalf("supported version %d unexpectedly rejected", v)
		}
		vIdx := bytes.Index(payload, []byte(`"_version"`))
		hIdx := bytes.Index(payload, []byte(`"content_hash"`))
		if vIdx < 0 || hIdx < 0 {
			t.Fatalf("v%d payload missing required keys: %s", v, payload)
		}
		if vIdx > hIdx {
			t.Errorf("v%d payload has content_hash before _version: %s", v, payload)
		}
	}
}
