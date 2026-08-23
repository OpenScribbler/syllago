package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/regdiff"
)

const registryDiffChangeLimit = 20

// printRegistryDiff renders an item-level change summary for one registry
// sync. No-op when d is nil, d.OldRef is empty (no baseline - first sync
// would list every item as added), d.UpToDate, or there are no changes.
func printRegistryDiff(w io.Writer, d *regdiff.Diff) {
	if output.JSON || output.Quiet || d == nil || strings.TrimSpace(d.OldRef) == "" || d.UpToDate {
		return
	}
	if len(d.Changes) == 0 && len(d.OtherPaths) == 0 {
		return
	}

	fmt.Fprintln(w, "Changes since last sync:")
	limit := len(d.Changes)
	if limit > registryDiffChangeLimit {
		limit = registryDiffChangeLimit
	}
	for _, change := range d.Changes[:limit] {
		fmt.Fprintf(w, "  %s %s/%s\n", registryDiffSymbol(change.Kind), change.Type, trimRegistryDiffName(change.Name))
	}
	if more := len(d.Changes) - limit; more > 0 {
		fmt.Fprintf(w, "  … and %d more\n", more)
	}
	if len(d.OtherPaths) > 0 {
		fmt.Fprintf(w, "  (plus %d other changed files)\n", len(d.OtherPaths))
	}
}

func registryDiffSymbol(kind regdiff.Kind) string {
	switch kind {
	case regdiff.KindAdded:
		return "+"
	case regdiff.KindRemoved:
		return "-"
	default:
		return "~"
	}
}

func trimRegistryDiffName(name string) string {
	for _, ext := range []string{".md", ".mdc", ".markdown", ".yaml", ".yml", ".json", ".toml"} {
		if strings.HasSuffix(name, ext) {
			return strings.TrimSuffix(name, ext)
		}
	}
	return name
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
