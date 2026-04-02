package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const idPrefix = "syl_"
const idHexBytes = 6 // 6 bytes = 12 hex chars

// generateID returns a new pseudonymous ID in the form syl_a1b2c3d4e5f6.
// Uses crypto/rand — not derived from any machine or user information.
// The ID is persistent across sessions (stored in ~/.syllago/telemetry.json) but is
// pseudonymous, not truly anonymous — it can be used to correlate events over time.
func generateID() (string, error) {
	b := make([]byte, idHexBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating anonymous ID: %w", err)
	}
	return idPrefix + hex.EncodeToString(b), nil
}

// isValidID returns true if id has the correct syl_ prefix and hex suffix.
func isValidID(id string) bool {
	if len(id) != len(idPrefix)+idHexBytes*2 {
		return false
	}
	if id[:len(idPrefix)] != idPrefix {
		return false
	}
	for _, c := range id[len(idPrefix):] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
