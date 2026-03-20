package provider

import (
	"os"
)

// ProviderPathLookup provides custom path overrides for detection.
// Implemented by *config.PathResolver (structural satisfaction, no import needed).
type ProviderPathLookup interface {
	BaseDir(slug string) string
}

// DetectProviders checks the filesystem for installed AI coding tools
// and returns a slice of Providers with Detected set appropriately.
func DetectProviders() []Provider {
	return DetectProvidersWithResolver(nil)
}

// DetectProvidersWithResolver checks default paths AND custom paths from the lookup.
// A provider is Detected=true if either:
//  1. Its standard Detect function returns true (checks hardcoded paths), OR
//  2. The lookup has a configured BaseDir for that provider and it exists on disk.
//
// Pass nil for lookup to get standard detection behavior.
func DetectProvidersWithResolver(lookup ProviderPathLookup) []Provider {
	home, err := os.UserHomeDir()
	if err != nil {
		return AllProviders // return all as not detected
	}

	var result []Provider
	for _, p := range AllProviders {
		detected := p // copy

		// Standard detection via hardcoded paths.
		if p.Detect != nil {
			detected.Detected = p.Detect(home)
		}

		// Custom path detection: if a base dir is configured and exists on disk,
		// the provider is present regardless of standard detection result.
		// Do NOT call p.Detect(customBaseDir) — Detect functions check hardcoded
		// subdirectories relative to home (e.g. filepath.Join(homeDir, ".claude")),
		// so passing a custom dir would check the wrong path.
		if !detected.Detected && lookup != nil {
			if baseDir := lookup.BaseDir(p.Slug); baseDir != "" {
				if _, statErr := os.Stat(baseDir); statErr == nil {
					detected.Detected = true
				}
			}
		}

		result = append(result, detected)
	}
	return result
}
