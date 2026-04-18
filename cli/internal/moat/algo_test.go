package moat

import (
	"strings"
	"testing"
)

// TestParseContentHash locks the MOAT v0.6.0 algorithm allowlist. The table
// covers each normative class: accepted, forbidden (hard failure), and
// unrecognized (hard failure). Manifests carrying any value from the
// forbidden/unknown rows MUST NOT parse.
func TestParseContentHash(t *testing.T) {
	t.Parallel()

	const (
		sha256Digest = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // sha256("")
		sha512Digest = "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"
	)

	tests := []struct {
		name        string
		in          string
		wantAlgo    string
		wantErrSubs string // when non-empty, parse must fail and include this substring
	}{
		// Accepted.
		{"sha256_lowercase_ok", "sha256:" + sha256Digest, HashAlgoSHA256, ""},
		{"sha512_lowercase_ok", "sha512:" + sha512Digest, HashAlgoSHA512, ""},

		// Forbidden algorithms — HARD failure per spec.
		{"sha1_forbidden", "sha1:" + strings.Repeat("a", 40), "", "allowlist"},
		{"md5_forbidden", "md5:" + strings.Repeat("a", 32), "", "allowlist"},

		// Unrecognized algorithms — HARD failure per spec (fail-closed).
		{"blake2_unknown", "blake2:" + strings.Repeat("a", 64), "", "allowlist"},
		{"typo_sha_256", "sha-256:" + sha256Digest, "", "allowlist"},

		// Format errors.
		{"empty", "", "", "empty"},
		{"no_separator", "sha256" + sha256Digest, "", "missing ':'"},
		{"empty_algorithm", ":" + sha256Digest, "", "empty algorithm"},
		{"empty_hex", "sha256:", "", "empty hex"},

		// Length errors.
		{"sha256_too_short", "sha256:abc", "", "64"},
		{"sha256_too_long", "sha256:" + sha256Digest + "ff", "", "64"},
		{"sha512_wrong_length", "sha512:" + sha256Digest, "", "128"},

		// Case sensitivity — manifests MUST be lowercase hex.
		{"sha256_uppercase_hex_rejected", "sha256:" + strings.ToUpper(sha256Digest), "", "non-lowercase"},
		{"sha256_mixed_case_hex_rejected", "sha256:E3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", "", "non-lowercase"},

		// Algorithm-name case sensitivity — MOAT spec uses lowercase.
		{"sha256_upper_algo_rejected", "SHA256:" + sha256Digest, "", "allowlist"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			algo, hex, err := ParseContentHash(tt.in)
			if tt.wantErrSubs != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got success (algo=%q, hex=%q)",
						tt.wantErrSubs, algo, hex)
				}
				if !strings.Contains(err.Error(), tt.wantErrSubs) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrSubs)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if algo != tt.wantAlgo {
				t.Errorf("algo = %q; want %q", algo, tt.wantAlgo)
			}
		})
	}
}

// TestManifest_RejectsBadAlgoInContent proves the allowlist is wired into
// manifest validation — a content entry with sha1 fails ParseManifest.
func TestManifest_RejectsBadAlgoInContent(t *testing.T) {
	t.Parallel()

	const sha1Hash = "sha1:" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	entry := `{
      "name": "bad", "display_name": "Bad", "type": "skill",
      "content_hash": "` + sha1Hash + `", "source_uri": "u",
      "attested_at": "2026-04-08T00:00:00Z", "private_repo": false
    }`
	data := buildManifest(t, entry)

	_, err := ParseManifest(data)
	if err == nil {
		t.Fatal("manifest with sha1 content_hash must be rejected")
	}
	if !strings.Contains(err.Error(), "allowlist") {
		t.Errorf("error = %q; want allowlist message", err)
	}
}

// TestManifest_RejectsBadAlgoInRevocation proves revocation entries go
// through the same allowlist check.
func TestManifest_RejectsBadAlgoInRevocation(t *testing.T) {
	t.Parallel()

	base := strings.Replace(minimalManifestJSON,
		`"revocations": []`,
		`"revocations": [{"content_hash": "md5:`+strings.Repeat("a", 32)+`", "reason": "malicious", "details_url": "https://x/1"}]`,
		1)

	_, err := ParseManifest([]byte(base))
	if err == nil {
		t.Fatal("revocation with md5 content_hash must be rejected")
	}
	if !strings.Contains(err.Error(), "allowlist") {
		t.Errorf("error = %q; want allowlist message", err)
	}
}
