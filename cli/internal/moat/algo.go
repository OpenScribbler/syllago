package moat

// MOAT v0.6.0 content-hash algorithm allowlist (spec §What the Spec Defines).
//
// Policy (normative):
//   - sha256 REQUIRED — every conforming client MUST support it.
//   - sha512 OPTIONAL — a client MAY accept it.
//   - sha1, md5, and any algorithm with known practical collision attacks
//     are FORBIDDEN. Conforming clients MUST reject as a HARD failure.
//   - Unrecognized algorithm names MUST fail verification (fail-closed).
//
// Implementation: a strict allowlist. Anything outside the set is rejected.
// This naturally satisfies "unrecognized MUST refuse to verify" — there is
// no silent passthrough path for unknown algorithms.

import (
	"fmt"
	"strings"
)

// Hash-algorithm identifiers used in the `<algorithm>:<hex>` content_hash
// format.
const (
	HashAlgoSHA256 = "sha256"
	HashAlgoSHA512 = "sha512"
)

// hashAlgoHexLen maps each allowlisted algorithm to its expected hex-digest
// length. A length mismatch is a HARD failure — the value was truncated,
// padded, or hashed with a different algorithm than its label.
var hashAlgoHexLen = map[string]int{
	HashAlgoSHA256: 64,
	HashAlgoSHA512: 128,
}

// ParseContentHash splits an `<algorithm>:<hex>` string, validates the
// algorithm against the allowlist, and confirms the hex digest length.
//
// Returns:
//   - algorithm (lowercase, e.g. "sha256")
//   - hex digest (lowercase) — validated for length and hex alphabet
//   - error on any policy violation or malformed input
//
// Errors cover (normatively):
//   - missing/empty algorithm or hex prefix
//   - algorithm not in the allowlist (sha1/md5/unknown → HARD failure)
//   - hex digest wrong length for the claimed algorithm
//   - non-hex characters in the digest
func ParseContentHash(s string) (algorithm, hexDigest string, err error) {
	if s == "" {
		return "", "", fmt.Errorf("content_hash is empty")
	}
	idx := strings.IndexByte(s, ':')
	if idx < 0 {
		return "", "", fmt.Errorf("content_hash %q: missing ':' separator — expected <algorithm>:<hex>", s)
	}
	algo := s[:idx]
	hx := s[idx+1:]
	if algo == "" {
		return "", "", fmt.Errorf("content_hash %q: empty algorithm prefix", s)
	}
	if hx == "" {
		return "", "", fmt.Errorf("content_hash %q: empty hex digest", s)
	}
	wantLen, ok := hashAlgoHexLen[algo]
	if !ok {
		return "", "", fmt.Errorf("content_hash %q: algorithm %q is not in the MOAT allowlist {sha256, sha512} — sha1, md5, and unrecognized algorithms are HARD failures",
			s, algo)
	}
	if len(hx) != wantLen {
		return "", "", fmt.Errorf("content_hash %q: %s digest has %d hex chars; expected %d",
			s, algo, len(hx), wantLen)
	}
	if !isLowerHex(hx) {
		return "", "", fmt.Errorf("content_hash %q: hex digest contains non-lowercase-hex characters", s)
	}
	return algo, hx, nil
}

// isLowerHex reports whether s contains only [0-9a-f].
// Uppercase is intentionally rejected — manifests are canonical.
func isLowerHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}
