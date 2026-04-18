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
