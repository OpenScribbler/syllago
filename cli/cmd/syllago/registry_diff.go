package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/regdiff"
	"github.com/OpenScribbler/syllago/cli/internal/registryops"
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
	printRegistryDiffLines(w, d)
}

func printRegistryDiffLines(w io.Writer, d *regdiff.Diff) {
	if d == nil {
		return
	}
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

func printInstalledDrift(w io.Writer, drifts []registryops.InstalledDrift) {
	if output.JSON || output.Quiet || len(drifts) == 0 {
		return
	}

	fmt.Fprintln(w, "Installed items drifted from upstream:")
	for _, drift := range drifts {
		switch drift.Kind {
		case registryops.DriftChanged:
			fmt.Fprintf(w, "  ~ %s/%s changed upstream — refresh: syllago add %s --from %s --force\n", drift.Type, drift.Name, drift.Name, drift.Registry)
		case registryops.DriftMissing:
			fmt.Fprintf(w, "  ! %s/%s no longer in registry%s — remove: syllago remove %s\n", drift.Type, drift.Name, installedDriftProviderText(drift.Providers), drift.Name)
		}
	}
}

func installedDriftProviderText(providers []string) string {
	if len(providers) == 0 {
		return ""
	}
	return " (installed to: " + strings.Join(providers, ", ") + ")"
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
