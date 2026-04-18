package moat

import (
	"bytes"
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
