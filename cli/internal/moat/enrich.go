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
// catalog.ContentItem (TrustTier, Revoked, RevocationReason) so the TUI
// gallery and listings can render a trust badge on registry-sourced
// items without re-parsing the manifest. It does NOT mutate any other
// ContentItem field — name/type/path/provider are already set by the
// catalog scan; enrichment only fills in the trust surface.

import (
	"os"
	"path/filepath"
	"strings"

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

// materializeMOATItems injects one ContentItem per manifest entry into
// cat.Items for the given registry. MOAT cache dirs hold only
// manifest.json + signature.bundle — no content tree — so the filesystem
// scanner finds zero items for MOAT registries. Without materialization
// the gallery card reads "0 items" and the library/explorer never see
// registry rows even after a successful sync.
//
// Synthesized items leave Path empty: the actual content blob is fetched
// at install time via entry.SourceURI, never pre-staged in the cache.
// Type maps through FromMOATType; entries whose type is not MOAT-
// recognized (hooks, mcp, future spec extensions) are skipped per the
// spec's "conforming clients MUST ignore unknown types" rule.
//
// Idempotent on (Registry, Type, Name): if a future change pre-stages
// content under the registry source path so the scanner produces rows,
// this helper will not duplicate them. A nil catalog or nil manifest is
// a no-op.
//
// Equivalent to materializeMOATItemsWithCache with an empty cacheDir; kept
// as a wrapper for the producer path that does not have a cacheDir handy.
func materializeMOATItems(cat *catalog.Catalog, registryName string, m *Manifest) {
	materializeMOATItemsWithCache(cat, registryName, m, "")
}

// materializeMOATItemsWithCache is the cache-aware variant that powers
// invisible content preview. When cacheDir is non-empty and the per-item
// content cache populated by sync (see contentcache.go) holds bytes for
// the entry, this function fills ContentItem.Path + Files so the TUI
// preview can render content directly. Cache misses leave Path empty —
// the install-time fetch path remains the staging boundary.
//
// cacheDir is the global syllago dir (config.GlobalDirPath result), the
// same value passed to EnrichFromMOATManifests / WriteContentCache.
// An empty cacheDir disables cache lookup entirely (used by tests and the
// no-cacheDir wrapper above).
func materializeMOATItemsWithCache(cat *catalog.Catalog, registryName string, m *Manifest, cacheDir string) {
	if cat == nil || m == nil {
		return
	}
	existing := make(map[string]bool, len(cat.Items))
	for _, it := range cat.Items {
		if it.Registry != registryName {
			continue
		}
		existing[string(it.Type)+"/"+it.Name] = true
	}
	for i := range m.Content {
		entry := &m.Content[i]
		ct, ok := FromMOATType(entry.Type)
		if !ok {
			continue
		}
		key := string(ct) + "/" + entry.Name
		if existing[key] {
			continue
		}
		item := catalog.ContentItem{
			Name:        entry.Name,
			DisplayName: entry.DisplayName,
			Type:        ct,
			Registry:    registryName,
			Source:      registryName,
		}
		if cacheDir != "" {
			if categoryDir, ok := CategoryDirForMOATType(entry.Type); ok {
				if cachedPath, err := ContentCachePathFor(cacheDir, registryName, categoryDir, entry.Name); err == nil {
					if info, statErr := os.Stat(cachedPath); statErr == nil && info.IsDir() {
						item.Path = cachedPath
						item.Files = collectCachedFiles(cachedPath)
						enrichFromCachedFrontmatter(&item, cachedPath)
					}
				}
			}
		}
		cat.Items = append(cat.Items, item)
		existing[key] = true
	}
}

// enrichFromCachedFrontmatter mirrors the library-page scanner: skills read
// SKILL.md frontmatter, agents read AGENT.md, populating DisplayName +
// Description. Other content types have no frontmatter convention at this
// boundary and are left untouched. Read errors and missing frontmatter are
// silent — same behavior as catalog.scanner so registry items present
// identically to local items in the TUI.
func enrichFromCachedFrontmatter(item *catalog.ContentItem, cachedPath string) {
	var fmFile string
	switch item.Type {
	case catalog.Skills:
		fmFile = "SKILL.md"
	case catalog.Agents:
		fmFile = "AGENT.md"
	default:
		return
	}
	data, err := os.ReadFile(filepath.Join(cachedPath, fmFile))
	if err != nil {
		return
	}
	fm, fmErr := catalog.ParseFrontmatter(data)
	if fmErr != nil {
		return
	}
	if fm.Name != "" {
		item.DisplayName = fm.Name
	}
	if fm.Description != "" {
		item.Description = fm.Description
	}
}

// collectCachedFiles walks itemDir and returns relative paths of every
// non-hidden regular file. Mirrors catalog.collectFiles (package-private)
// but lives here to keep the moat → catalog import direction one-way.
func collectCachedFiles(itemDir string) []string {
	var files []string
	_ = filepath.WalkDir(itemDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(itemDir, path)
		if relErr != nil {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	return files
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
//   - Set item.PrivateRepo from entry.PrivateRepo (per-item G-10 declaration,
//     independent of registry-level Visibility probe). Populated even when
//     no revocation is present.
//   - If any revocation in m.Revocations covers the entry's ContentHash:
//     set item.Revoked, item.RevocationReason, item.RevocationSource,
//     item.RevocationDetailsURL, and item.Revoker. Publisher-controlled
//     strings (Reason, DetailsURL, Revoker) pass through SanitizeForDisplay
//     at this boundary so downstream consumers never see attacker-controlled
//     terminal bytes. The enrich step is the single chokepoint — no later
//     consumer needs to re-sanitize.
//
// A nil catalog or nil manifest is a no-op. Items from other registries
// (or with empty Registry) are left completely untouched.
//
// Revocation source (registry vs publisher) is NOT considered for the
// user-facing Revoked badge per AD-7 Panel C9 (both collapse to the same
// glyph). The source IS exposed via item.RevocationSource so drill-down text
// can show "(publisher)" vs "(registry)" without breaking the collapse
// rule. Install-flow enforcement uses RevocationSet / installer.PreInstallCheck
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
		item.PrivateRepo = entry.PrivateRepo
		item.RegistrySubject = SanitizeForDisplay(m.RegistrySigningProfile.Subject)
		item.RegistryIssuer = SanitizeForDisplay(m.RegistrySigningProfile.Issuer)
		item.RegistryOperator = SanitizeForDisplay(m.Operator)
		if entry.SigningProfile != nil {
			item.PublisherSubject = SanitizeForDisplay(entry.SigningProfile.Subject)
			item.PublisherIssuer = SanitizeForDisplay(entry.SigningProfile.Issuer)
		}
		if rev, ok := revByHash[entry.ContentHash]; ok {
			item.Revoked = true
			item.RevocationReason = SanitizeForDisplay(rev.Reason)
			item.RevocationSource = rev.EffectiveSource()
			item.RevocationDetailsURL = SanitizeForDisplay(rev.DetailsURL)
			item.Revoker = resolveRevoker(rev, m, entry)
		}
	}
}

// resolveRevoker derives the revoker identity from the manifest and
// entry. The rule mirrors MOAT spec v0.6.0:
//
//   - registry source → Manifest.Operator, falling back to
//     Manifest.RegistrySigningProfile.Subject. Manifest.validate()
//     guarantees Subject is non-empty at parse time, so the fallback is
//     always populated.
//   - publisher source → ContentEntry.SigningProfile.Subject when present,
//     else a literal "(publisher — identity not provided)" sentinel so the
//     drill-down banner still has non-empty text to render.
//
// All returned strings pass through SanitizeForDisplay here so callers
// can splice the value into TUI cells without re-scrubbing. An unknown
// source (shouldn't happen after manifest validation, but be defensive)
// returns empty — consumers branch on "".
func resolveRevoker(rev *Revocation, m *Manifest, entry *ContentEntry) string {
	switch rev.EffectiveSource() {
	case RevocationSourceRegistry:
		if m.Operator != "" {
			return SanitizeForDisplay(m.Operator)
		}
		return SanitizeForDisplay(m.RegistrySigningProfile.Subject)
	case RevocationSourcePublisher:
		if entry.SigningProfile != nil && entry.SigningProfile.Subject != "" {
			return SanitizeForDisplay(entry.SigningProfile.Subject)
		}
		return "(publisher — identity not provided)"
	}
	return ""
}
