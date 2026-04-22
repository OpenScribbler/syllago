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
//   - Provider-specific types (Rules, Hooks, Commands): match by type + providerSlug + name.
//     These live under content/<type>/<provider>/<name>/ so provider scoping is required.
//   - Universal types (Skills, Agents, MCP): match by type + name only.
//     These are provider-agnostic and live under content/<type>/<name>/.
//   - The catalog's Items slice is already ordered by precedence (local > content > registry),
//     so the first match wins.
func Resolve(manifest *Manifest, cat *catalog.Catalog, providerSlug string) ([]ResolvedRef, error) {
	var refs []ResolvedRef
	var missing []string

	for ct, itemRefs := range manifest.RefsByType() {
		for _, ref := range itemRefs {
			item, found := findItem(cat, ct, ref.Name, providerSlug)
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

// findItem searches the catalog for a matching item.
// For provider-specific types, it matches on type + provider + name.
// For universal types, it matches on type + name only.
func findItem(cat *catalog.Catalog, ct catalog.ContentType, name string, provider string) (catalog.ContentItem, bool) {
	for _, item := range cat.Items {
		if item.Type != ct || item.Name != name {
			continue
		}
		// Provider-specific types require provider match
		if !ct.IsUniversal() && ct != catalog.Loadouts {
			if item.Provider != provider {
				continue
			}
		}
		return item, true
	}
	return catalog.ContentItem{}, false
}
