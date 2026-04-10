package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
)

// HashFile computes the SHA-256 hash of a file's contents.
func HashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return HashBytes(data), nil
}

// HashBytes computes the SHA-256 hex digest of arbitrary data.
func HashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// DriftEntry reports a single content integrity mismatch.
type DriftEntry struct {
	Type         string `json:"type"` // "hook", "mcp", or "symlink"
	Name         string `json:"name"`
	ExpectedHash string `json:"expectedHash"`
	ActualHash   string `json:"actualHash"` // empty if file is missing
	Status       string `json:"status"`     // "modified", "missing", or "ok"
}

// VerifyIntegrity checks all installed content for drift.
// Items without a stored hash are skipped (pre-existing installs).
func VerifyIntegrity(projectRoot string) ([]DriftEntry, error) {
	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("loading installed.json: %w", err)
	}

	var drifted []DriftEntry

	for _, s := range inst.Symlinks {
		if s.ContentHash == "" {
			continue
		}
		actual, err := HashFile(s.Target)
		if err != nil {
			drifted = append(drifted, DriftEntry{
				Type:         "symlink",
				Name:         s.Path,
				ExpectedHash: s.ContentHash,
				Status:       "missing",
			})
			continue
		}
		if actual != s.ContentHash {
			drifted = append(drifted, DriftEntry{
				Type:         "symlink",
				Name:         s.Path,
				ExpectedHash: s.ContentHash,
				ActualHash:   actual,
				Status:       "modified",
			})
		}
	}

	for _, m := range inst.MCP {
		if m.ContentHash == "" {
			continue
		}
		// MCP entries don't track their installed file path directly.
		// Drift detection for MCP requires reading the settings file,
		// which is deferred to syllago doctor.
	}

	return drifted, nil
}
