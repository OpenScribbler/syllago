package analyzer

import "github.com/OpenScribbler/syllago/cli/internal/catalog"

// itemKey is the dedup identity for a DetectedItem.
type itemKey struct {
	contentType catalog.ContentType
	name        string
}

// DeduplicateItems processes classified items:
// 1. Suppresses hook-script items whose Path appears in another item's Scripts list.
// 2. Deduplicates same (type, name) items:
//   - Same content hash: keep highest confidence, record other paths in Providers alias.
//   - Different content hash: keep both (conflicts returned separately).
//
// Returns deduplicated items and conflict pairs.
func DeduplicateItems(items []*DetectedItem) (deduped []*DetectedItem, conflicts [][2]*DetectedItem) {
	// Step 1: Build set of script paths consumed by wired hooks.
	consumedScripts := make(map[string]bool)
	for _, item := range items {
		for _, s := range item.Scripts {
			consumedScripts[s] = true
		}
	}

	// Step 2: Filter out consumed hook-script items.
	var filtered []*DetectedItem
	for _, item := range items {
		if item.InternalLabel == "hook-script" && consumedScripts[item.Path] {
			continue // suppressed: consumed by a wired hook
		}
		filtered = append(filtered, item)
	}

	// Step 3: Dedup by (type, name).
	seen := make(map[itemKey]*DetectedItem)
	for _, item := range filtered {
		key := itemKey{item.Type, item.Name}
		existing, ok := seen[key]
		if !ok {
			seen[key] = item
			continue
		}
		if existing.ContentHash == item.ContentHash {
			// Same content: keep higher confidence, record alias.
			// Decision #19: syllago canonical detector always wins ties.
			incomingWins := item.Confidence > existing.Confidence ||
				(item.Confidence == existing.Confidence && providerPriority(item.Provider) < providerPriority(existing.Provider))
			if incomingWins {
				item.Providers = append(item.Providers, existing.Path)
				seen[key] = item
			} else {
				existing.Providers = append(existing.Providers, item.Path)
			}
		} else {
			// Different content: record conflict.
			conflicts = append(conflicts, [2]*DetectedItem{existing, item})
		}
	}

	for _, item := range seen {
		deduped = append(deduped, item)
	}
	return deduped, conflicts
}

// providerPriority returns a sort rank for a provider slug.
// Lower rank = higher priority in tiebreaks (Decision #19).
// syllago canonical > provider-specific > top-level agnostic.
func providerPriority(slug string) int {
	switch slug {
	case "syllago":
		return 0
	case "top-level":
		return 2
	default:
		return 1 // all named providers rank between syllago and top-level
	}
}
