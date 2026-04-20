package moat

// Catalog enrichment helpers (ADR 0007 Phase 2, bead syllago-kvf66).
//
// Two narrow helpers that bridge a freshly-synced Manifest to the catalog
// package's ContentItem view. The moat package owns these because moat
// already imports catalog (typemap.go) — catalog cannot import moat
// without a cycle, so the direction is fixed.
//
// FindContentEntry is the lookup call the install-flow runs after
// Sync: given the parsed Manifest and the item name requested, return
// the matching *ContentEntry (or nil if the registry does not serve a
// row by that name).
//
// EnrichCatalog populates the existing display-only fields on
// catalog.ContentItem (TrustTier, Recalled, RecallReason) so the TUI
// gallery and listings can render a trust badge on registry-sourced
// items without re-parsing the manifest. It does NOT mutate any other
// ContentItem field — name/type/path/provider are already set by the
// catalog scan; enrichment only fills in the trust surface.

import (
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// FindContentEntry looks up a content entry by name in the manifest's
// Content slice. Linear scan — manifests are small (O(100) at most in
// practice) and a map index would add allocation cost for no gain.
//
// Returns (entry, true) on hit. A nil manifest or a miss returns
// (nil, false) — the caller MUST check the bool rather than deref a
// possibly-nil pointer. Ambiguous-name handling (G-16 compound-key
// uniqueness) lives upstream in ParseManifest, so callers do not need
// to defend against duplicates here.
func FindContentEntry(m *Manifest, name string) (*ContentEntry, bool) {
	if m == nil {
		return nil, false
	}
	for i := range m.Content {
		if m.Content[i].Name == name {
			return &m.Content[i], true
		}
	}
	return nil, false
}

// moatTierToCatalogTier maps the moat package's internal tier enum to
// the catalog package's equivalent. The enums are separate (moat owns
// the normative classification; catalog owns the display layer) so a
// mapping function is the seam. The zero value on the catalog side is
// TrustTierUnknown, reserved for items that were never sourced from a
// MOAT manifest — we never return it here because every input to this
// function is a MOAT entry by construction.
func moatTierToCatalogTier(t TrustTier) catalog.TrustTier {
	switch t {
	case TrustTierDualAttested:
		return catalog.TrustTierDualAttested
	case TrustTierSigned:
		return catalog.TrustTierSigned
	case TrustTierUnsigned:
		return catalog.TrustTierUnsigned
	}
	// Unreachable with the current enum, but return Unknown rather than
	// assume — catalog code treats Unknown as "no claim" which is the
	// safe default if a future TrustTier value appears here.
	return catalog.TrustTierUnknown
}

// EnrichCatalog populates the display-only trust fields on every
// ContentItem whose Registry field matches registryName, using the
// manifest's content rows and revocations list.
//
// For each matching item:
//   - Find the manifest ContentEntry by item.Name. Skip enrichment if
//     absent (the registry clone carries a file the manifest does not
//     list — e.g., in-progress content the publisher has not yet
//     attested).
//   - Set item.TrustTier from entry.TrustTier() (with G-13 downgrade).
//   - If any revocation in m.Revocations covers the entry's ContentHash:
//     set item.Recalled = true and item.RecallReason from the first
//     matching revocation's Reason. A verbatim-carried opaque Reason is
//     acceptable per the MOAT spec — callers display it as-is.
//
// A nil catalog or nil manifest is a no-op. Items from other registries
// (or with empty Registry) are left completely untouched.
//
// Revocation source (registry vs publisher) is NOT considered here —
// display-layer Recalled collapses both under the user-facing Recalled
// badge per AD-7 Panel C9. Install-flow enforcement uses RevocationSet
// directly and still branches on source for the two-tier contract.
func EnrichCatalog(cat *catalog.Catalog, registryName string, m *Manifest) {
	if cat == nil || m == nil {
		return
	}

	// Build a hash → first-matching-revocation index so we only scan
	// m.Revocations once regardless of how many items share a hash.
	// (In practice a hash appears at most once in revocations[], but the
	// index sidesteps that assumption.)
	revByHash := make(map[string]*Revocation, len(m.Revocations))
	for i := range m.Revocations {
		h := m.Revocations[i].ContentHash
		if _, ok := revByHash[h]; !ok {
			revByHash[h] = &m.Revocations[i]
		}
	}

	for i := range cat.Items {
		item := &cat.Items[i]
		if item.Registry != registryName {
			continue
		}
		entry, ok := FindContentEntry(m, item.Name)
		if !ok {
			continue
		}
		item.TrustTier = moatTierToCatalogTier(entry.TrustTier())
		if rev, ok := revByHash[entry.ContentHash]; ok {
			item.Recalled = true
			item.RecallReason = rev.Reason
		}
	}
}
