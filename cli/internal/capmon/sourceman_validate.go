package capmon

import (
	"fmt"
	"path/filepath"
	"strings"
)

// KnownHealingStrategies is the set of healing strategy names accepted in
// source manifests. Unknown strategies fail validation.
var KnownHealingStrategies = map[string]bool{
	"redirect":      true,
	"github-rename": true,
	"variant":       true,
}

// ValidateSources validates that a provider's source manifest has at least one
// source URI for each content type, unless the content type is explicitly marked
// as not supported (supported: false). It also validates healing configuration
// per source.
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
		for i, src := range ctSource.Sources {
			if src.Healing == nil {
				continue
			}
			for _, strat := range src.Healing.Strategies {
				if !KnownHealingStrategies[strat] {
					errs = append(errs, fmt.Sprintf("content_types.%s.sources[%d].healing.strategies: unknown strategy %q (valid: redirect, github-rename, variant)", ct, i, strat))
				}
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("source manifest validation failed for %s:\n  %s", provider, strings.Join(errs, "\n  "))
	}
	return nil
}
