package loadout

import (
	"fmt"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"gopkg.in/yaml.v3"
)

// ItemRef references a content item by name and optionally by ID (UUID).
// The ID is used to detect name-swap attacks: if a private item is replaced
// with a same-named public one, the ID mismatch triggers a warning at publish.
type ItemRef struct {
	Name string `yaml:"name"`
	ID   string `yaml:"id,omitempty"` // UUID from .syllago.yaml metadata
}

// Manifest represents a parsed loadout.yaml file.
type Manifest struct {
	Kind        string    `yaml:"kind"`               // must be "loadout"
	Version     int       `yaml:"version"`            // must be 1
	Provider    string    `yaml:"provider,omitempty"` // e.g. "claude-code"; optional, can be set via --to flag
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	Rules       []ItemRef `yaml:"rules,omitempty"`
	Hooks       []ItemRef `yaml:"hooks,omitempty"`
	Skills      []ItemRef `yaml:"skills,omitempty"`
	Agents      []ItemRef `yaml:"agents,omitempty"`
	MCP         []ItemRef `yaml:"mcp,omitempty"`
	Commands    []ItemRef `yaml:"commands,omitempty"`
}

// Parse reads and validates a loadout.yaml file.
// Returns an error if kind != "loadout", version != 1, or name is empty.
// Provider is optional — it can be specified at apply time via --to flag.
func Parse(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if m.Kind != "loadout" {
		return nil, fmt.Errorf("%s: kind must be \"loadout\", got %q", path, m.Kind)
	}
	if m.Version != 1 {
		return nil, fmt.Errorf("%s: version must be 1, got %d", path, m.Version)
	}
	if m.Name == "" {
		return nil, fmt.Errorf("%s: name is required", path)
	}
	if !catalog.IsValidItemName(m.Name) {
		return nil, fmt.Errorf("%s: invalid name %q — use only letters, numbers, hyphens, and underscores (no leading dash, max 100 chars)", path, m.Name)
	}

	return &m, nil
}

// UnmarshalYAML allows ItemRef to be unmarshaled from either a plain string
// (e.g., "my-rule") or a full object (e.g., {name: "my-rule", id: "abc-123"}).
// This lets loadout.yaml files use the simple string format while still
// supporting the richer struct format when IDs are present.
func (r *ItemRef) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		r.Name = value.Value
		return nil
	}
	// Full struct decode for mapping nodes.
	type plain ItemRef
	return value.Decode((*plain)(r))
}

// ItemCount returns the total number of referenced items across all sections.
func (m *Manifest) ItemCount() int {
	return len(m.Rules) + len(m.Hooks) + len(m.Skills) + len(m.Agents) +
		len(m.MCP) + len(m.Commands)
}

// RefsByType returns a map of ContentType -> []ItemRef for all non-empty sections.
func (m *Manifest) RefsByType() map[catalog.ContentType][]ItemRef {
	result := make(map[catalog.ContentType][]ItemRef)
	if len(m.Rules) > 0 {
		result[catalog.Rules] = m.Rules
	}
	if len(m.Hooks) > 0 {
		result[catalog.Hooks] = m.Hooks
	}
	if len(m.Skills) > 0 {
		result[catalog.Skills] = m.Skills
	}
	if len(m.Agents) > 0 {
		result[catalog.Agents] = m.Agents
	}
	if len(m.MCP) > 0 {
		result[catalog.MCP] = m.MCP
	}
	if len(m.Commands) > 0 {
		result[catalog.Commands] = m.Commands
	}
	return result
}

// RefNames extracts just the names from a slice of ItemRefs.
func RefNames(refs []ItemRef) []string {
	names := make([]string, len(refs))
	for i, r := range refs {
		names[i] = r.Name
	}
	return names
}

// NameRefsByType returns a map of ContentType -> []string (names only).
// Convenience method for callers that don't need IDs.
func (m *Manifest) NameRefsByType() map[catalog.ContentType][]string {
	result := make(map[catalog.ContentType][]string)
	for ct, refs := range m.RefsByType() {
		result[ct] = RefNames(refs)
	}
	return result
}
