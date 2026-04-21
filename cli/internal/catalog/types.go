package catalog

import "github.com/OpenScribbler/syllago/cli/internal/metadata"

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
	// (Dual-Attested / Signed / Unsigned / Unknown); Recalled flips on when
	// the publisher or registry has revoked this content hash (G-8). These
	// fields stay zero for items not sourced from a MOAT manifest — a git
	// registry item is indistinguishable from "trust question not asked."
	// See AD-7 and UserFacingBadge for the collapse rules.
	//
	// The drill-down fields below (PrivateRepo, RecallSource, RecallDetailsURL,
	// RecallIssuer) are populated by moat.EnrichCatalog. Publisher-controlled
	// strings (RecallReason, RecallDetailsURL, RecallIssuer) are pre-sanitized
	// at the enrich boundary via moat.SanitizeForDisplay — consumers treat
	// the values as trusted for display. Field naming note: PrivateRepo
	// matches moat.ContentEntry.PrivateRepo (the source field) rather than
	// using an Is-prefix, which is reserved for methods in Go.
	TrustTier        TrustTier
	Recalled         bool
	RecallReason     string // sanitized; populated when Recalled
	PrivateRepo      bool   // mirrors moat.ContentEntry.PrivateRepo (G-10)
	RecallSource     string // "registry" or "publisher" when Recalled; empty otherwise
	RecallDetailsURL string // sanitized URL when Recalled; may be empty
	RecallIssuer     string // sanitized revoker identity; empty when not Recalled
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

// Catalog holds all discovered content items and the repo root they came from.
type Catalog struct {
	Items      []ContentItem
	Overridden []ContentItem // lower-precedence items shadowed by higher-precedence ones
	Warnings   []string      // non-fatal scan warnings (collected instead of printing to stderr)
	RepoRoot   string
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
