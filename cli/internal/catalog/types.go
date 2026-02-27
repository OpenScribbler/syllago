package catalog

import "github.com/OpenScribbler/nesco/cli/internal/metadata"

// ContentType represents a category of content in the repo.
type ContentType string

const (
	Skills        ContentType = "skills"
	Agents        ContentType = "agents"
	Prompts       ContentType = "prompts"
	MCP           ContentType = "mcp"
	Apps          ContentType = "apps"
	Rules         ContentType = "rules"
	Hooks         ContentType = "hooks"
	Commands      ContentType = "commands"
	SearchResults ContentType = "search"   // virtual type for cross-category search results
	MyTools       ContentType = "local" // virtual type for local items view
)

// AllContentTypes returns all content types in display order.
func AllContentTypes() []ContentType {
	return []ContentType{Skills, Agents, Prompts, MCP, Apps, Rules, Hooks, Commands}
}

// IsUniversal returns true if this content type works with any AI tool
// (as opposed to provider-specific types like rules, hooks, commands).
func (ct ContentType) IsUniversal() bool {
	switch ct {
	case Skills, Agents, Prompts, MCP, Apps:
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
	case Prompts:
		return "Prompts"
	case MCP:
		return "MCP Configs"
	case Apps:
		return "Apps"
	case Rules:
		return "Rules"
	case Hooks:
		return "Hooks"
	case Commands:
		return "Commands"
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
	Name               string
	DisplayName        string // human-readable name from frontmatter (skills only), falls back to Name
	Description        string
	Type               ContentType
	Path               string         // absolute path to the item directory or file
	Provider           string         // for provider-specific content (rules, hooks, commands), which provider
	Body               string         // full text content (used by Prompts for display/clipboard)
	ReadmeBody         string         // raw README.md content (for rendering in detail view)
	Files              []string       // relative paths of all files in item directory
	SupportedProviders []string       // provider slugs this item works with (apps only), e.g. ["claude-code", "gemini-cli"]
	Meta               *metadata.Meta // loaded from .nesco.yaml if present
	Local              bool           // true if item lives in local/
	Registry           string         // non-empty if item came from a git registry (value is the registry name)
}

// Catalog holds all discovered content items and the repo root they came from.
type Catalog struct {
	Items    []ContentItem
	RepoRoot string
}

// ByType returns all items of a given type.
func (c *Catalog) ByType(ct ContentType) []ContentItem {
	var result []ContentItem
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

// ByTypeLocal returns local-only items of a given type.
func (c *Catalog) ByTypeLocal(ct ContentType) []ContentItem {
	var result []ContentItem
	for _, item := range c.Items {
		if item.Type == ct && item.Local {
			result = append(result, item)
		}
	}
	return result
}

// ByTypeShared returns shared-only items of a given type (from the main repo, not registries).
func (c *Catalog) ByTypeShared(ct ContentType) []ContentItem {
	var result []ContentItem
	for _, item := range c.Items {
		if item.Type == ct && !item.Local && item.Registry == "" {
			result = append(result, item)
		}
	}
	return result
}

// CountLocal returns the total number of local items across all types.
func (c *Catalog) CountLocal() int {
	count := 0
	for _, item := range c.Items {
		if item.Local {
			count++
		}
	}
	return count
}

// ByRegistry returns all items from a specific named registry.
func (c *Catalog) ByRegistry(name string) []ContentItem {
	var result []ContentItem
	for _, item := range c.Items {
		if item.Registry == name {
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
