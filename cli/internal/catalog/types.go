package catalog

import (
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// ContentType represents a category of content in the repo.
type ContentType string

const (
	Skills        ContentType = "skills"
	Agents        ContentType = "agents"
	MCP           ContentType = "mcp"
	Rules         ContentType = "rules"
	Hooks         ContentType = "hooks"
	Commands      ContentType = "commands"
	Loadouts      ContentType = "loadouts"
	SearchResults ContentType = "search"  // virtual type for cross-category search results
	Library       ContentType = "library" // virtual type for global library items view
)

// AllContentTypes returns all content types in display order.
func AllContentTypes() []ContentType {
	return []ContentType{Skills, Agents, MCP, Rules, Hooks, Commands, Loadouts}
}

// IsUniversal returns true if this content type works with any AI tool
// (as opposed to provider-specific types like rules, hooks, commands).
func (ct ContentType) IsUniversal() bool {
	switch ct {
	case Skills, Agents, MCP:
		return true
	}
	return false
}

// Label returns a human-readable display label.
func (ct ContentType) Label() string {
	switch ct {
	case Skills:
		return "Skills"
	case Agents:
		return "Agents"
	case MCP:
		return "MCP Servers"
	case Rules:
		return "Rules"
	case Hooks:
		return "Hooks"
	case Commands:
		return "Commands"
	case Loadouts:
		return "Loadouts"
	}
	return string(ct)
}

// RegistrySource describes a registry to include in a multi-source scan.
type RegistrySource struct {
	Name string // registry name (used to tag items)
	Path string // absolute path to the registry clone directory
}

// ContentItem represents a single discoverable piece of content in the repo.
type ContentItem struct {
	Name        string
	DisplayName string // human-readable name from frontmatter (skills only), falls back to Name
	Description string
	Type        ContentType
	Path        string         // absolute path to the item directory or file
	Provider    string         // for provider-specific content (rules, hooks, commands), which provider
	Files       []string       // relative paths of all files in item directory
	ServerKey   string         // for MCP: which server entry in config.json this item represents
	Meta        *metadata.Meta // loaded from .syllago.yaml if present
	Library     bool           // true if item lives in the global content library (~/.syllago/content/)
	Registry    string         // non-empty if item came from a git registry (value is the registry name)
	Source      string         // "project", "global", "library", or registry name

	// MOAT trust state. TrustTier is the normative internal classification
	// (Dual-Attested / Signed / Unsigned / Unknown); Revoked flips on when
	// the publisher or registry has revoked this content hash (G-8). These
	// fields stay zero for items not sourced from a MOAT manifest — a git
	// registry item is indistinguishable from "trust question not asked."
	// See AD-7 and UserFacingBadge for the collapse rules.
	//
	// The drill-down fields below (PrivateRepo, RevocationSource,
	// RevocationDetailsURL, Revoker) are populated by moat.EnrichCatalog.
	// Publisher-controlled strings (RevocationReason, RevocationDetailsURL,
	// Revoker) are pre-sanitized at the enrich boundary via
	// moat.SanitizeForDisplay — consumers treat the values as trusted for
	// display. Field naming note: PrivateRepo matches moat.ContentEntry.PrivateRepo
	// (the source field) rather than using an Is-prefix, which is reserved for
	// methods in Go. Terminology note: "revoked" / "revocation" aligns with the
	// MOAT protocol (Revocation records, RevocationSource) — earlier drafts
	// used "recalled" as a user-facing synonym, but collapsing on the
	// protocol's term keeps the code and UI consistent.
	TrustTier            TrustTier
	Revoked              bool
	RevocationReason     string // sanitized; populated when Revoked
	PrivateRepo          bool   // mirrors moat.ContentEntry.PrivateRepo (G-10)
	RevocationSource     string // "registry" or "publisher" when Revoked; empty otherwise
	RevocationDetailsURL string // sanitized URL when Revoked; may be empty
	Revoker              string // sanitized revoker identity; empty when not Revoked

	// Signing profile fields populated by moat.EnrichCatalog for items from
	// MOAT-type registries. Publisher* come from the per-item signing_profile
	// (absent for Signed-tier items where the registry attests but individual
	// entries do not). Registry* are duplicated from the manifest-level
	// registry_signing_profile + operator so the Trust Inspector can render
	// the full attestation chain per item without reaching into the registry
	// aggregate. All values are sanitized at the enrich boundary.
	PublisherSubject string
	PublisherIssuer  string
	RegistrySubject  string
	RegistryIssuer   string
	RegistryOperator string
}

// IsExample returns true if this item is tagged as example content.
func (ci *ContentItem) IsExample() bool {
	if ci.Meta == nil {
		return false
	}
	for _, tag := range ci.Meta.Tags {
		if tag == "example" {
			return true
		}
	}
	return false
}

// IsBuiltin returns true if this item is tagged as built-in meta-content.
func (ci *ContentItem) IsBuiltin() bool {
	if ci.Meta == nil {
		return false
	}
	for _, tag := range ci.Meta.Tags {
		if tag == "builtin" {
			return true
		}
	}
	return false
}

// RegistryTrust is the per-registry trust aggregate populated by
// moat.EnrichFromMOATManifests. Lives on Catalog so the TUI gallery and
// Trust Inspector can render registry-level trust surfaces (card glyph,
// preview panel Trust section, inspector fields) without re-parsing the
// manifest or re-walking the lockfile. Only populated for MOAT-type
// registries — non-MOAT registries (git, local) leave the map entry absent,
// which callers treat as "no trust claim made."
//
// Tier reflects the display-facing state:
//   - Signed when the registry is Fresh and has a valid registry_signing_profile.
//   - Unsigned when Staleness != Fresh (per MOAT spec, stale caches disable
//     trust decisions — the manifest claim is stale, not absent, so we
//     downgrade rather than dropping the entry).
//
// Staleness strings: "Fresh", "Stale", "Expired", "Missing". Missing fills
// in when the cache file is absent (never synced or deleted since).
type RegistryTrust struct {
	Name          string
	Tier          TrustTier
	Issuer        string
	Subject       string
	Operator      string
	ManifestURI   string
	FetchedAt     time.Time
	Staleness     string
	TotalItems    int
	VerifiedItems int
	RevokedItems  int
	PrivateItems  int
}

// Catalog holds all discovered content items and the repo root they came from.
type Catalog struct {
	Items      []ContentItem
	Overridden []ContentItem // lower-precedence items shadowed by higher-precedence ones
	Warnings   []string      // non-fatal scan warnings (collected instead of printing to stderr)
	RepoRoot   string

	// RegistryTrusts aggregates MOAT trust state per registry name. Keyed by
	// catalog registry name (matches ContentItem.Registry). Populated only
	// for MOAT-type registries; absence means "no MOAT trust claim for this
	// registry." See moat.EnrichFromMOATManifests for the producer.
	RegistryTrusts map[string]*RegistryTrust
}

// ByType returns all items of a given type.
func (c *Catalog) ByType(ct ContentType) []ContentItem {
	result := make([]ContentItem, 0, len(c.Items)/4)
	for _, item := range c.Items {
		if item.Type == ct {
			result = append(result, item)
		}
	}
	return result
}

// CountByType returns item count per type.
func (c *Catalog) CountByType() map[ContentType]int {
	counts := make(map[ContentType]int)
	for _, item := range c.Items {
		counts[item.Type]++
	}
	return counts
}

// ByTypeShared returns shared-only items of a given type (from the main repo, not registries).
func (c *Catalog) ByTypeShared(ct ContentType) []ContentItem {
	result := make([]ContentItem, 0, len(c.Items)/4)
	for _, item := range c.Items {
		if item.Type == ct && !item.Library && item.Registry == "" {
			result = append(result, item)
		}
	}
	return result
}

// ByRegistry returns all items from a specific named registry.
func (c *Catalog) ByRegistry(name string) []ContentItem {
	result := make([]ContentItem, 0, len(c.Items)/4)
	for _, item := range c.Items {
		if item.Registry == name {
			result = append(result, item)
		}
	}
	return result
}

// CountLibrary returns the number of items sourced from the global content library.
func (c *Catalog) CountLibrary() int {
	count := 0
	for _, item := range c.Items {
		if item.Library {
			count++
		}
	}
	return count
}

// ByTypeLibrary returns items of the given type that came from the global library.
func (c *Catalog) ByTypeLibrary(ct ContentType) []ContentItem {
	result := make([]ContentItem, 0, len(c.Items)/4)
	for _, item := range c.Items {
		if item.Type == ct && item.Library {
			result = append(result, item)
		}
	}
	return result
}

// CountRegistry returns the number of items from a specific named registry.
func (c *Catalog) CountRegistry(name string) int {
	count := 0
	for _, item := range c.Items {
		if item.Registry == name {
			count++
		}
	}
	return count
}
