package moat

// TUI install-gate input aggregator (ADR 0007 Phase 2c, bead syllago-u0jna).
//
// The TUI drives installs from a catalog view that has already been enriched
// via EnrichFromMOATManifests at rescan time. At dispatch (click Install),
// it must run installer.PreInstallCheck with an authoritative view of the
// gate inputs — the same parsed manifests that produced the trust badges
// the user just saw. Re-reading the cached files keeps the gate and the
// display consistent without a per-click network sync (which would block
// the event loop under the Elm rules in .claude/rules/tui-elm.md).
//
// Why this function does NOT share the read loop with EnrichFromMOATManifests:
//
//   - Enrichment needs the verify-time cache + signature.bundle gating that
//     drives fail-closed behavior for unpinned profiles; this file only
//     needs the parsed manifest (ContentEntry lookup + Revocation
//     aggregation). Coupling would force callers to pay the verification
//     cost every time they want gate data.
//   - Per-registry failures are already surfaced as cat.Warnings by the
//     enrich path, so this file is intentionally silent on the same
//     failures — the user has already seen them in the catalog warnings.
//   - The TUI's rescan happens on 'R' (user-initiated, rare). Reading each
//     manifest a second time at <1KB-per-registry is negligible compared
//     to the sigstore verify the enrichment path runs.

import (
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// GateInputs bundles the per-registry data installer.PreInstallCheck needs
// for a TUI install dispatch. Constructed from the same cached manifests
// EnrichFromMOATManifests reads at scan time, but without the verification
// step (already performed at enrich time or at sync time).
//
// Lifecycle: freshly constructed per rescan. The moat.Session, by contrast,
// persists for the TUI run — publisher-warn confirmations must survive an
// 'R' refresh or the user would re-prompt on every redraw.
//
// When the caller has no MOAT registries configured, the returned struct
// is non-nil but empty (Manifests and ManifestURIs are empty maps and
// RevSet is a fresh empty set). Downstream code can treat
// `len(gi.Manifests) == 0` as "skip the gate" without nil guards.
type GateInputs struct {
	// RevSet aggregates every MOAT registry's revocations. Passed verbatim
	// to installer.PreInstallCheck so the two-tier contract (ADR 0007 G-8)
	// can be enforced against the freshest on-disk view.
	RevSet *RevocationSet

	// Manifests indexes parsed Manifest objects by registry Name. The TUI
	// resolves ContentEntry via moat.FindContentEntry(m, item.Name) at
	// dispatch time rather than carrying ContentEntry pointers through
	// catalog.ContentItem (which would bloat the catalog-scan API for
	// every non-MOAT consumer).
	Manifests map[string]*Manifest

	// ManifestURIs indexes each registry's canonical manifest_uri by Name.
	// Used as the Session key so a publisher revocation confirmed against
	// registry A never auto-suppresses the same hash observed on registry
	// B. Matches the registryURL argument the CLI install path passes to
	// installer.PreInstallCheck.
	ManifestURIs map[string]string
}

// HasRegistry reports whether the given registry name has a parsed manifest
// in this GateInputs. Callers use this to short-circuit the gate check for
// items whose registry is not MOAT-backed (or whose cache is missing) —
// those items route through the legacy install path with no revocation
// data, which is the safe default.
func (gi *GateInputs) HasRegistry(name string) bool {
	if gi == nil {
		return false
	}
	_, ok := gi.Manifests[name]
	return ok
}

// BuildGateInputs walks cfg.Registries, loads the cached manifest for each
// MOAT-type registry under <cacheDir>/moat/registries/<name>/manifest.json,
// and aggregates the results. Errors per registry (missing cache, unparse-
// able manifest, name escaping the cache tree) are silent — the enrich
// path has already produced the user-visible warnings.
//
// A nil cfg or an unresolvable cacheDir returns an empty (non-nil)
// GateInputs. The zero/empty case is a legitimate runtime state (no
// registries configured, or the cache tree was just deleted), not an
// error condition.
func BuildGateInputs(cfg *config.Config, cacheDir string) *GateInputs {
	gi := &GateInputs{
		RevSet:       NewRevocationSet(),
		Manifests:    make(map[string]*Manifest),
		ManifestURIs: make(map[string]string),
	}
	if cfg == nil {
		return gi
	}
	absCache, err := filepath.Abs(cacheDir)
	if err != nil {
		return gi
	}
	for i := range cfg.Registries {
		reg := &cfg.Registries[i]
		if !reg.IsMOAT() {
			continue
		}
		manifestPath, _, err := manifestCachePathsFor(absCache, reg.Name)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		m, err := ParseManifest(data)
		if err != nil {
			continue
		}
		gi.Manifests[reg.Name] = m
		gi.ManifestURIs[reg.Name] = reg.ManifestURI
		gi.RevSet.AddFromManifest(m, reg.ManifestURI)
	}
	return gi
}
