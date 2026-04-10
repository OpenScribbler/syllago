package catalog

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SafeResolve joins baseDir and untrustedPath, resolves symlinks, and validates
// that the result stays within baseDir. Returns an error if the path escapes.
// This guards against both "../" traversal and symlink escape attacks.
func SafeResolve(baseDir, untrustedPath string) (string, error) {
	// Reject absolute untrusted paths — filepath.Join would embed them as a
	// subdirectory segment, producing a false-safe result like /base/tmp/evil.
	if filepath.IsAbs(untrustedPath) {
		return "", fmt.Errorf("path escapes base: %s not under %s", untrustedPath, baseDir)
	}
	joined := filepath.Clean(filepath.Join(baseDir, untrustedPath))
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		// If the path doesn't exist yet (e.g., destination for copy), fall back
		// to the cleaned path for containment check only.
		resolved = joined
	}
	base := filepath.Clean(baseDir)
	if !strings.HasPrefix(resolved, base+string(filepath.Separator)) && resolved != base {
		return "", fmt.Errorf("path escapes base: %s not under %s", untrustedPath, baseDir)
	}
	return resolved, nil
}
