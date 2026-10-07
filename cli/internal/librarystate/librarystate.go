// Package librarystate answers "what does the user have right now": the
// trust-enriched catalog, installed.json, and the rule-append verification
// computed from the two. The TUI's first load and its refresh both build it
// through FromScan, so they read the same state by the same path.
package librarystate

import (
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installcheck"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/rulestore"
)

// Snapshot is one load of the user's library state.
type Snapshot struct {
	*moat.ScanResult
	// Installed is installed.json. It is empty, never nil, when the file is
	// missing or malformed.
	Installed *installer.Installed
	// Verification is the D16 scan of Installed against the library rules.
	// It is nil only when no project root was given.
	Verification *installcheck.VerificationResult
}

// Load runs moat.LoadAndScan and passes the result to FromScan. Only a
// catalog-scan failure returns an error.
//
// It does file I/O and, on its first call in a process, sigstore
// verification, so the TUI must call it from a tea.Cmd.
func Load(root, projectRoot string, now time.Time) (*Snapshot, error) {
	scan, err := moat.LoadAndScan(root, projectRoot, now)
	if err != nil {
		return nil, err
	}
	return FromScan(scan, projectRoot), nil
}

// FromScan reads installed state against the catalog in scan. A caller that
// changes the library between its scan and this call must rescan first:
// the verification cache is keyed by target file, so it never notices a
// library rule that disappeared.
func FromScan(scan *moat.ScanResult, projectRoot string) *Snapshot {
	snap := &Snapshot{ScanResult: scan, Installed: &installer.Installed{}}
	if projectRoot == "" {
		return snap
	}
	if inst, err := installer.LoadInstalled(projectRoot); err == nil {
		snap.Installed = inst
	}
	snap.Verification = verify(snap.Installed, scan.Catalog.Items)
	return snap
}

// verify runs the D16 scan over inst and the library rules among items.
// Rule items' Path fields point at the on-disk rule directory (D11 layout:
// rule.md + .syllago.yaml + .history/). A rule that fails to load is
// skipped, and Scan reports records whose LibraryID it lacks as Modified.
func verify(inst *installer.Installed, items []catalog.ContentItem) *installcheck.VerificationResult {
	library := map[string]*rulestore.Loaded{}
	for i := range items {
		it := items[i]
		if it.Type != catalog.Rules || !it.Library || it.Path == "" {
			continue
		}
		loaded, err := rulestore.LoadRule(it.Path)
		if err != nil {
			continue
		}
		library[loaded.Meta.ID] = loaded
	}
	return installcheck.Scan(inst, library)
}
