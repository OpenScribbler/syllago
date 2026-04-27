package loadout

import (
	"fmt"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// ResolvedRef links a manifest entry to its catalog item.
type ResolvedRef struct {
	Type catalog.ContentType
	Name string
	Item catalog.ContentItem
}

// Resolve resolves all manifest references against the catalog for a specific provider.
// Returns an error describing every unresolved ref (not just the first).
//
// providerSlug is the target provider slug used to filter provider-specific catalog
// items (Rules, Hooks, Commands). Pass the slug of the provider being applied to;
// for multi-provider loadouts this will differ from manifest.Provider (which may be empty).
//
// Why report all missing refs at once: if a loadout references 5 items and 3 are
// missing, the user sees all 3 problems in one pass rather than fixing them one at a
// time. This is a common CLI UX pattern for validation errors.
//
// Resolution strategy:
//   - Provider-specific types (Rules, Hooks, Commands): match by type + name, preferring
//     items scoped to providerSlug, then falling back to the manifest's source providers
//     (manifest.Provider and manifest.Providers). The fallback enables cross-provider
//     apply: a loadout authored for claude-code can be applied --to gemini-cli and
//     syllago reads the claude-code content, converting it to the target on install.
//   - Universal types (Skills, Agents, MCP): match by type + name only.
//     These are provider-agnostic and live under content/<type>/<name>/.
//   - The catalog's Items slice is already ordered by precedence (local > content > registry),
//     so the first match wins.
func Resolve(manifest *Manifest, cat *catalog.Catalog, providerSlug string) ([]ResolvedRef, error) {
	var refs []ResolvedRef
	var missing []string

	fallbacks := sourceProviderFallbacks(manifest, providerSlug)

	for ct, itemRefs := range manifest.RefsByType() {
		for _, ref := range itemRefs {
			item, found := findItem(cat, ct, ref.Name, providerSlug, fallbacks)
			if !found {
				missing = append(missing, fmt.Sprintf("%s: %s not found", ct, ref.Name))
				continue
			}
			refs = append(refs, ResolvedRef{
				Type: ct,
				Name: ref.Name,
				Item: item,
			})
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("unresolved references:\n  %s", strings.Join(missing, "\n  "))
	}
	return refs, nil
}

// sourceProviderFallbacks returns the manifest's declared source providers, in
// preference order, excluding the target slug (it's already tried first by findItem).
// Used so cross-provider apply can find content under the manifest's source provider
// when the target doesn't have a flavored copy.
func sourceProviderFallbacks(manifest *Manifest, target string) []string {
	seen := map[string]bool{target: true, "": true}
	var out []string
	add := func(slug string) {
		if seen[slug] {
			return
		}
		seen[slug] = true
		out = append(out, slug)
	}
	add(manifest.Provider)
	for _, p := range manifest.Providers {
		add(p)
	}
	return out
}

// findItem searches the catalog for a matching item.
// For provider-specific types, it tries the target provider first, then each fallback
// in order. For universal types, it matches on type + name only.
func findItem(cat *catalog.Catalog, ct catalog.ContentType, name string, target string, fallbacks []string) (catalog.ContentItem, bool) {
	if ct.IsUniversal() || ct == catalog.Loadouts {
		for _, item := range cat.Items {
			if item.Type == ct && item.Name == name {
				return item, true
			}
		}
		return catalog.ContentItem{}, false
	}
	for _, slug := range append([]string{target}, fallbacks...) {
		for _, item := range cat.Items {
			if item.Type != ct || item.Name != name {
				continue
			}
			if item.Provider == slug {
				return item, true
			}
		}
	}
	return catalog.ContentItem{}, false
}
