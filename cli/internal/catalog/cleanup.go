package catalog

import (
	"os"
)

// CleanupResult describes a local item that was cleaned up.
type CleanupResult struct {
	Name string
	Type ContentType
	Path string
}

// CleanupPromotedItems removes library items whose ID, name, and type
// all match a shared item. This happens after a promote PR is merged and pulled —
// the shared copy now exists, so the library copy is redundant.
//
// Requiring all three fields to match (not just ID) prevents accidental deletion
// from UUID collisions or malicious ID duplication.
func CleanupPromotedItems(cat *Catalog) ([]CleanupResult, error) {
	// Build a map of shared items keyed by ID, storing name and type for validation.
	// Shared items may be in cat.Items or cat.Overridden (the local copy wins precedence,
	// pushing the shared copy to Overridden after applyPrecedence runs).
	type sharedInfo struct {
		Name string
		Type ContentType
	}
	sharedByID := make(map[string]sharedInfo)
	for _, item := range append(cat.Items, cat.Overridden...) {
		if !item.Library && item.Registry == "" && item.Meta != nil && item.Meta.ID != "" {
			sharedByID[item.Meta.ID] = sharedInfo{Name: item.Name, Type: item.Type}
		}
	}

	var cleaned []CleanupResult
	allItems := append(cat.Items, cat.Overridden...)
	for _, item := range allItems {
		if !item.Library || item.Meta == nil || item.Meta.ID == "" {
			continue
		}
		if shared, exists := sharedByID[item.Meta.ID]; exists {
			// Require name and type to match (defense against ID collision)
			if shared.Name != item.Name || shared.Type != item.Type {
				continue
			}
			if err := os.RemoveAll(item.Path); err != nil {
				return cleaned, err
			}
			cleaned = append(cleaned, CleanupResult{
				Name: item.Name,
				Type: item.Type,
				Path: item.Path,
			})
		}
	}

	return cleaned, nil
}
