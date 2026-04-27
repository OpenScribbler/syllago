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

// KnownConventions is the set of cross-provider convention names accepted
// as a substitute for source URIs. A content type with one of these names
// in its `convention:` field passes validation even with empty sources —
// the convention itself is the spec, and there is no upstream URL to monitor.
var KnownConventions = map[string]bool{
	"cross-provider-agents-md": true,
	"cross-provider-skill-md":  true,
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
		if ctSource.Convention != "" && !KnownConventions[ctSource.Convention] {
			errs = append(errs, fmt.Sprintf("content_types.%s: unknown convention %q (valid: cross-provider-agents-md, cross-provider-skill-md)", ct, ctSource.Convention))
		}
		if len(ctSource.Sources) == 0 && ctSource.Convention == "" {
			errs = append(errs, fmt.Sprintf("content_types.%s: no source URIs and not marked as supported: false (use convention: cross-provider-agents-md or cross-provider-skill-md if implemented via cross-provider convention)", ct))
		}
		for i, src := range ctSource.Sources {
			if src.Healing == nil {
				continue
			}
			for _, strategy := range src.Healing.Strategies {
				if !KnownHealingStrategies[strategy] {
					errs = append(errs, fmt.Sprintf("content_types.%s.sources[%d].healing.strategies: unknown strategy %q (valid: redirect, github-rename, variant)", ct, i, strategy))
				}
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("source manifest validation failed for %s:\n  %s", provider, strings.Join(errs, "\n  "))
	}
	return nil
}
