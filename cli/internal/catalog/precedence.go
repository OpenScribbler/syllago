package catalog

import "strings"

// itemPrecedence returns the precedence level for an item (lower number = higher priority).
func itemPrecedence(item ContentItem) int {
	if item.Library {
		return 0 // highest
	}
	if item.Registry == "" && !item.IsBuiltin() {
		return 1 // shared
	}
	if item.Registry != "" {
		return 2 // registry
	}
	return 3 // built-in (lowest)
}

// applyPrecedence deduplicates items by (name, type), keeping the highest-precedence
// version in Items and moving others to Overridden.
// Loadouts include provider in the dedup key because the same loadout name under
// different providers represents distinct loadouts (different content references).
func applyPrecedence(cat *Catalog) {
	type key struct {
		name     string
		typ      ContentType
		provider string // only set for Loadouts
	}

	best := make(map[key]int) // key → index in kept of winning item

	var kept []ContentItem
	var overridden []ContentItem

	for _, item := range cat.Items {
		k := key{strings.ToLower(item.Name), item.Type, ""}
		if item.Type == Loadouts {
			k.provider = item.Provider
		}
		winIdx, exists := best[k]
		if !exists {
			best[k] = len(kept)
			kept = append(kept, item)
			continue
		}
		challenger := itemPrecedence(item)
		current := itemPrecedence(kept[winIdx])
		if challenger < current {
			// Challenger wins — move existing winner to overridden, replace in kept
			overridden = append(overridden, kept[winIdx])
			best[k] = winIdx // index stays same, we replace in-place
			kept[winIdx] = item
		} else {
			// Current wins — challenger goes to overridden
			overridden = append(overridden, item)
		}
	}

	cat.Items = kept
	cat.Overridden = overridden
}

// OverridesFor returns overridden items for a given (name, type) pair.
func (c *Catalog) OverridesFor(name string, ct ContentType) []ContentItem {
	var result []ContentItem
	lower := strings.ToLower(name)
	for _, item := range c.Overridden {
		if strings.ToLower(item.Name) == lower && item.Type == ct {
			result = append(result, item)
		}
	}
	return result
}
