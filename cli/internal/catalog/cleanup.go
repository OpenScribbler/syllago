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

// CleanupPromotedItems removes local (my-tools/) items whose ID, name, and type
// all match a shared item. This happens after a promote PR is merged and pulled —
// the shared copy now exists, so the local copy is redundant.
//
// Requiring all three fields to match (not just ID) prevents accidental deletion
// from UUID collisions or malicious ID duplication.
func CleanupPromotedItems(cat *Catalog) ([]CleanupResult, error) {
	// Build a map of shared items keyed by ID, storing name and type for validation.
	type sharedInfo struct {
		Name string
		Type ContentType
	}
	sharedByID := make(map[string]sharedInfo)
	for _, item := range cat.Items {
		if !item.Local && item.Meta != nil && item.Meta.ID != "" {
			sharedByID[item.Meta.ID] = sharedInfo{Name: item.Name, Type: item.Type}
		}
	}

	var cleaned []CleanupResult
	for _, item := range cat.Items {
		if !item.Local || item.Meta == nil || item.Meta.ID == "" {
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
