// cli/internal/catalog/detect.go
package catalog

import (
	"os"
	"path/filepath"
	"strings"
)

// markerFiles lists content marker filenames and their human-readable type
// labels in priority order. A slice is used instead of a map to guarantee
// deterministic iteration order.
var markerFiles = []struct {
	marker string
	label  string
}{
	{"SKILL.md", "Skill"},
	{"AGENT.md", "Agent"},
}

// DetectContent performs a lightweight check on a path to determine if it
// contains recognizable content. For directories, it checks for marker files
// (SKILL.md, AGENT.md, etc.). For files, it returns the file extension.
//
// Returns (typeLabel, true) if content is detected, or ("", false) if not.
// This is intentionally cheap — it uses os.Stat, not full catalog.Scan().
func DetectContent(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}

	if info.IsDir() {
		// Check for marker files in priority order
		for _, mf := range markerFiles {
			if _, err := os.Stat(filepath.Join(path, mf.marker)); err == nil {
				return mf.label, true
			}
		}
		return "", false
	}

	// For files, return the extension as a simple indicator
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" {
		return ext, true
	}
	return "", false
}
