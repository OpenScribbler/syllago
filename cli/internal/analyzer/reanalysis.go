package analyzer

import (
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// ReanalysisResult holds the outcome of a sync hash comparison.
type ReanalysisResult struct {
	Unchanged []*registry.ManifestItem // path exists, hash unchanged
	Changed   []string                 // relative paths where hash changed
	Missing   []string                 // relative paths that no longer exist
	Warnings  []string
}

// DiffManifest compares the current filesystem state against an existing manifest.
// For each item in the manifest, it reads the file at item.Path (relative to repoRoot)
// and compares its SHA-256 against item.ContentHash.
// Items with empty ContentHash are treated as Unchanged (authored manifests without hashes).
func DiffManifest(manifest *registry.Manifest, repoRoot string) ReanalysisResult {
	var result ReanalysisResult
	if manifest == nil {
		return result
	}
	for i := range manifest.Items {
		item := &manifest.Items[i]
		if item.ContentHash == "" {
			result.Unchanged = append(result.Unchanged, item)
			continue
		}
		absPath := filepath.Join(repoRoot, item.Path)
		data, err := os.ReadFile(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				result.Missing = append(result.Missing, item.Path)
			} else {
				result.Warnings = append(result.Warnings, "reading "+item.Path+": "+err.Error())
			}
			continue
		}
		currentHash := hashBytes(data)
		if currentHash == item.ContentHash {
			result.Unchanged = append(result.Unchanged, item)
		} else {
			result.Changed = append(result.Changed, item.Path)
		}
	}
	return result
}

// PreserveUserMetadata merges user-edited display metadata from an existing manifest item
// into a newly-analyzed DetectedItem. Only non-empty user values are preserved.
func PreserveUserMetadata(existing *registry.ManifestItem, detected *DetectedItem) {
	if existing.DisplayName != "" && detected.DisplayName == "" {
		detected.DisplayName = existing.DisplayName
	}
	if existing.Description != "" && detected.Description == "" {
		detected.Description = existing.Description
	}
}
