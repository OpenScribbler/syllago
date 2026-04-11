package capmon

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateSources validates that a provider's source manifest has at least one
// source URI for each content type, unless the content type is explicitly marked
// as not supported (supported: false).
func ValidateSources(sourcesDir, provider string) error {
	path := filepath.Join(sourcesDir, provider+".yaml")
	manifest, err := LoadSourceManifest(path)
	if err != nil {
		return fmt.Errorf("validate sources for %s: %w", provider, err)
	}

	var errs []string
	for ct, ctSource := range manifest.ContentTypes {
		// Skip explicitly unsupported content types.
		if ctSource.Supported != nil && !*ctSource.Supported {
			continue
		}
		if len(ctSource.Sources) == 0 {
			errs = append(errs, fmt.Sprintf("content_types.%s: no source URIs and not marked as supported: false", ct))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("source manifest validation failed for %s:\n  %s", provider, strings.Join(errs, "\n  "))
	}
	return nil
}
