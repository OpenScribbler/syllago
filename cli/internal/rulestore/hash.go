// Package rulestore is the on-disk persistence layer for library rules (D11).
package rulestore

import (
	"fmt"
	"regexp"
	"strings"
)

var filenameHashRe = regexp.MustCompile(`^sha256-[0-9a-f]{64}\.md$`)

// hashToFilename converts a canonical "<algo>:<hex>" hash into its
// .history/<algo>-<hex>.md filename (D11). No error return: operates
// only on already-validated canonical hashes.
func hashToFilename(hash string) string {
	// Single conversion: `:` -> `-`, append ".md".
	return strings.Replace(hash, ":", "-", 1) + ".md"
}

// filenameToHash converts a .history filename back to its canonical
// "<algo>:<hex>" form (D11). Returns an error for any malformed filename
// so the loader can fail with a specific load error.
func filenameToHash(name string) (string, error) {
	if !filenameHashRe.MatchString(name) {
		return "", fmt.Errorf("malformed history filename: %q (want sha256-<64-hex>.md)", name)
	}
	trimmed := strings.TrimSuffix(name, ".md")
	return strings.Replace(trimmed, "-", ":", 1), nil
}
