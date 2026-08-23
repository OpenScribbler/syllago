package regdiff

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// MOATDiff computes the item-level change set between two MOAT manifests.
// old may be nil (no cached baseline - first sync): returns a Diff with
// UpToDate false and nil Changes.
func MOATDiff(registry string, old, new *moat.Manifest) Diff {
	diff := Diff{Registry: registry}
	if old != nil {
		diff.OldRef = old.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if new != nil {
		diff.NewRef = new.UpdatedAt.UTC().Format(time.RFC3339)
	}
	if old == nil {
		return diff
	}

	oldItems := manifestItems(old)
	newItems := manifestItems(new)

	for key, newEntry := range newItems {
		oldEntry, ok := oldItems[key]
		switch {
		case !ok:
			diff.Changes = append(diff.Changes, ItemChange{Type: key.Type, Name: key.Name, Kind: KindAdded})
		case oldEntry.ContentHash != newEntry.ContentHash:
			diff.Changes = append(diff.Changes, ItemChange{Type: key.Type, Name: key.Name, Kind: KindModified})
		}
	}
	for key := range oldItems {
		if _, ok := newItems[key]; !ok {
			diff.Changes = append(diff.Changes, ItemChange{Type: key.Type, Name: key.Name, Kind: KindRemoved})
		}
	}

	sort.Slice(diff.Changes, func(i, j int) bool {
		if diff.Changes[i].Type != diff.Changes[j].Type {
			return diff.Changes[i].Type < diff.Changes[j].Type
		}
		return diff.Changes[i].Name < diff.Changes[j].Name
	})
	diff.UpToDate = len(diff.Changes) == 0
	return diff
}

// LoadCachedManifest reads the previously synced manifest for a registry
// from the MOAT manifest cache. A missing cache file returns (nil, nil) -
// no baseline yet. Corrupt JSON returns an error.
func LoadCachedManifest(cacheDir, name string) (*moat.Manifest, error) {
	path, err := moat.ManifestCachePath(cacheDir, name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read cached manifest: %w", err)
	}
	m, err := moat.ParseManifest(data)
	if err != nil {
		return nil, fmt.Errorf("parse cached manifest: %w", err)
	}
	return m, nil
}

func manifestItems(m *moat.Manifest) map[itemKey]moat.ContentEntry {
	items := make(map[itemKey]moat.ContentEntry)
	if m == nil {
		return items
	}
	for _, entry := range m.Content {
		items[itemKey{Type: entry.Type, Name: entry.Name}] = entry
	}
	return items
}
