package registryops

import (
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// NormalizeRegistryIdentity folds a registry name for near-duplicate
// comparison: lowercase, underscores folded to hyphens.
func NormalizeRegistryIdentity(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "_", "-")
}

// NormalizeRegistryURL folds a git URL for near-duplicate comparison:
// lowercase, trailing "/" and ".git" stripped.
func NormalizeRegistryURL(url string) string {
	normalized := strings.ToLower(url)
	for {
		trimmed := strings.TrimSuffix(normalized, "/")
		trimmed = strings.TrimSuffix(trimmed, ".git")
		if trimmed == normalized {
			return normalized
		}
		normalized = trimmed
	}
}

// FindSimilarRegistries returns the names of configured registries that
// near-duplicate the candidate name or URL without exact-matching the name.
func FindSimilarRegistries(existing []config.Registry, name, url string) []string {
	normalizedName := NormalizeRegistryIdentity(name)
	normalizedURL := NormalizeRegistryURL(url)

	var similar []string
	for _, r := range existing {
		if r.Name == name {
			continue
		}
		if NormalizeRegistryIdentity(r.Name) == normalizedName {
			similar = append(similar, r.Name)
			continue
		}
		if r.URL == "" || url == "" {
			continue
		}
		if NormalizeRegistryURL(r.URL) == normalizedURL {
			similar = append(similar, r.Name)
		}
	}
	return similar
}
