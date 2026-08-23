package registryops

import (
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/regdiff"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// GitSyncDiff computes the item-level diff between two refs of a plain git
// registry's clone. Returns nil when the diff cannot be computed; diff display
// is best-effort for presentation surfaces.
func GitSyncDiff(regName, oldHead, newHead string) *regdiff.Diff {
	cloneDir, err := registry.CloneDir(regName)
	if err != nil {
		return nil
	}

	items := gitItemRefs(regName, cloneDir)
	types := catalog.AllContentTypes()
	knownTypeDirs := make([]string, 0, len(types))
	for _, ct := range types {
		knownTypeDirs = append(knownTypeDirs, string(ct))
	}

	d, err := regdiff.GitDiff(regName, cloneDir, oldHead, newHead, items, knownTypeDirs)
	if err != nil {
		return nil
	}
	return &d
}

func gitItemRefs(regName, cloneDir string) []regdiff.ItemRef {
	cat, err := catalog.ScanRegistriesOnly([]catalog.RegistrySource{
		{Name: regName, Path: cloneDir},
	})
	if err != nil {
		return nil
	}

	refs := make([]regdiff.ItemRef, 0, len(cat.Items))
	for _, item := range cat.Items {
		rel, err := filepath.Rel(cloneDir, item.Path)
		if err != nil {
			continue
		}
		cleaned := filepath.Clean(rel)
		relSlash := filepath.ToSlash(cleaned)
		if filepath.IsAbs(cleaned) || relSlash == ".." || strings.HasPrefix(relSlash, "../") {
			continue
		}
		refs = append(refs, regdiff.ItemRef{
			Type: string(item.Type),
			Name: item.Name,
			Dir:  relSlash,
		})
	}
	return refs
}
