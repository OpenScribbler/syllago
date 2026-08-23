package registryops

import (
	"os"
	"sort"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/regdiff"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// DriftKind classifies one installed item's divergence from its source registry.
type DriftKind string

const (
	DriftChanged DriftKind = "changed" // upstream content differs from the library copy
	DriftMissing DriftKind = "missing" // item no longer exists upstream (renamed or deleted)
)

// InstalledDrift is one installed item's drift against its source registry.
type InstalledDrift struct {
	Registry  string
	Type      string // content type slug, e.g. "skills"
	Name      string
	Kind      DriftKind
	Providers []string // provider slugs with placements for this item, sorted, deduped
}

// InstalledGitDrift reports drift for every installed item that originated
// from the plain git registry regName, comparing the current clone contents
// against the library copies. Best-effort: any store/clone/discovery error
// returns nil (drift display must never fail a sync).
func InstalledGitDrift(regName string) []InstalledDrift {
	path, err := installstore.DefaultPath()
	if err != nil {
		return nil
	}
	store, err := installstore.Load(path)
	if err != nil {
		return nil
	}
	records := store.ByRegistry(regName)
	if len(records) == 0 {
		return nil
	}

	cloneDir, err := registry.CloneDir(regName)
	if err != nil {
		return nil
	}
	info, err := os.Stat(cloneDir)
	if err != nil || !info.IsDir() {
		return nil
	}
	globalDir := catalog.GlobalContentDir()
	if globalDir == "" {
		return nil
	}

	items, err := add.DiscoverFromRegistry(regName, cloneDir, globalDir)
	if err != nil {
		return nil
	}
	discovered := make(map[installedDriftKey]add.ItemStatus, len(items))
	for _, item := range items {
		key := installedDriftKey{contentType: string(item.Type), name: item.Name}
		discovered[key] = mergeInstalledDriftStatus(discovered[key], item.Status)
	}

	drifts := make([]InstalledDrift, 0)
	for _, rec := range records {
		key := installedDriftKey{contentType: rec.Type, name: rec.Name}
		status, ok := discovered[key]
		if !ok {
			drifts = append(drifts, InstalledDrift{
				Registry:  regName,
				Type:      rec.Type,
				Name:      rec.Name,
				Kind:      DriftMissing,
				Providers: installedDriftProviders(rec),
			})
			continue
		}
		if status == add.StatusOutdated {
			drifts = append(drifts, InstalledDrift{
				Registry:  regName,
				Type:      rec.Type,
				Name:      rec.Name,
				Kind:      DriftChanged,
				Providers: installedDriftProviders(rec),
			})
		}
	}

	sort.Slice(drifts, func(i, j int) bool {
		if drifts[i].Type != drifts[j].Type {
			return drifts[i].Type < drifts[j].Type
		}
		return drifts[i].Name < drifts[j].Name
	})
	return drifts
}

// InstalledMOATDrift reports drift for installed items from the MOAT
// registry regName, derived from this sync's manifest diff. Unlike the git
// path this is diff-based: only items whose manifest entry changed in THIS
// sync are reported (library-copy hashes are not comparable to manifest
// content hashes, so there is no baseline for pre-existing staleness).
// Best-effort: nil diff, no changes, or any store error returns nil.
func InstalledMOATDrift(regName string, diff *regdiff.Diff) []InstalledDrift {
	if diff == nil || len(diff.Changes) == 0 {
		return nil
	}

	path, err := installstore.DefaultPath()
	if err != nil {
		return nil
	}
	store, err := installstore.Load(path)
	if err != nil {
		return nil
	}
	records := store.ByRegistry(regName)
	if len(records) == 0 {
		return nil
	}

	changes := make(map[installedDriftKey]regdiff.Kind, len(diff.Changes))
	for _, change := range diff.Changes {
		key := installedDriftKey{contentType: change.Type, name: change.Name}
		changes[key] = change.Kind
	}

	drifts := make([]InstalledDrift, 0)
	for _, rec := range records {
		key := installedDriftKey{contentType: rec.Type, name: rec.Name}
		switch changes[key] {
		case regdiff.KindModified:
			drifts = append(drifts, InstalledDrift{
				Registry:  regName,
				Type:      rec.Type,
				Name:      rec.Name,
				Kind:      DriftChanged,
				Providers: installedDriftProviders(rec),
			})
		case regdiff.KindRemoved:
			drifts = append(drifts, InstalledDrift{
				Registry:  regName,
				Type:      rec.Type,
				Name:      rec.Name,
				Kind:      DriftMissing,
				Providers: installedDriftProviders(rec),
			})
		}
	}

	sort.Slice(drifts, func(i, j int) bool {
		if drifts[i].Type != drifts[j].Type {
			return drifts[i].Type < drifts[j].Type
		}
		return drifts[i].Name < drifts[j].Name
	})
	return drifts
}

type installedDriftKey struct {
	contentType string
	name        string
}

func mergeInstalledDriftStatus(existing, incoming add.ItemStatus) add.ItemStatus {
	if existing == add.StatusOutdated || incoming == add.StatusOutdated {
		return add.StatusOutdated
	}
	if existing == add.StatusInLibrary || incoming == add.StatusInLibrary {
		return add.StatusInLibrary
	}
	return add.StatusNew
}

func installedDriftProviders(rec installstore.Record) []string {
	seen := make(map[string]struct{}, len(rec.Placements))
	for _, placement := range rec.Placements {
		if placement.Provider == "" {
			continue
		}
		seen[placement.Provider] = struct{}{}
	}
	providers := make([]string, 0, len(seen))
	for provider := range seen {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	return providers
}
