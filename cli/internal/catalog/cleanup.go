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

// CleanupPromotedItems removes local (my-tools/) items whose ID matches a shared item.
// This happens after a promote PR is merged and pulled — the shared copy now exists,
// so the local copy is redundant.
func CleanupPromotedItems(cat *Catalog) ([]CleanupResult, error) {
	// Build a set of shared item IDs
	sharedIDs := make(map[string]bool)
	for _, item := range cat.Items {
		if !item.Local && item.Meta != nil && item.Meta.ID != "" {
			sharedIDs[item.Meta.ID] = true
		}
	}

	var cleaned []CleanupResult
	for _, item := range cat.Items {
		if !item.Local || item.Meta == nil || item.Meta.ID == "" {
			continue
		}
		if sharedIDs[item.Meta.ID] {
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
