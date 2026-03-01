package loadout

import (
	"fmt"
	"strings"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
)

// ResolvedRef links a manifest entry to its catalog item.
type ResolvedRef struct {
	Type catalog.ContentType
	Name string
	Item catalog.ContentItem
}

// Resolve resolves all manifest references against the catalog.
// Returns an error describing every unresolved ref (not just the first).
//
// Why report all missing refs at once: if a loadout references 5 items and 3 are
// missing, the user sees all 3 problems in one pass rather than fixing them one at a
// time. This is a common CLI UX pattern for validation errors.
//
// Resolution strategy:
//   - Provider-specific types (Rules, Hooks, Commands): match by type + provider + name.
//     These live under content/<type>/<provider>/<name>/ so provider scoping is required.
//   - Universal types (Skills, Agents, Prompts, MCP, Apps): match by type + name only.
//     These are provider-agnostic and live under content/<type>/<name>/.
//   - The catalog's Items slice is already ordered by precedence (local > content > registry),
//     so the first match wins.
func Resolve(manifest *Manifest, cat *catalog.Catalog) ([]ResolvedRef, error) {
	var refs []ResolvedRef
	var missing []string

	for ct, names := range manifest.RefsByType() {
		for _, name := range names {
			item, found := findItem(cat, ct, name, manifest.Provider)
			if !found {
				missing = append(missing, fmt.Sprintf("%s: %s not found", ct, name))
				continue
			}
			refs = append(refs, ResolvedRef{
				Type: ct,
				Name: name,
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
