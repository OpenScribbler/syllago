package loadout

import (
	"fmt"
	"os"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"gopkg.in/yaml.v3"
)

// Manifest represents a parsed loadout.yaml file.
type Manifest struct {
	Kind        string   `yaml:"kind"`               // must be "loadout"
	Version     int      `yaml:"version"`            // must be 1
	Provider    string   `yaml:"provider,omitempty"` // e.g. "claude-code"; optional, can be set via --to flag
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Rules       []string `yaml:"rules,omitempty"`
	Hooks       []string `yaml:"hooks,omitempty"`
	Skills      []string `yaml:"skills,omitempty"`
	Agents      []string `yaml:"agents,omitempty"`
	MCP         []string `yaml:"mcp,omitempty"`
	Commands    []string `yaml:"commands,omitempty"`
	Prompts     []string `yaml:"prompts,omitempty"`
	Apps        []string `yaml:"apps,omitempty"`
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

	return &m, nil
}

// ItemCount returns the total number of referenced items across all sections.
func (m *Manifest) ItemCount() int {
	return len(m.Rules) + len(m.Hooks) + len(m.Skills) + len(m.Agents) +
		len(m.MCP) + len(m.Commands) + len(m.Prompts) + len(m.Apps)
}

// RefsByType returns a map of ContentType -> []name for all non-empty sections.
func (m *Manifest) RefsByType() map[catalog.ContentType][]string {
	result := make(map[catalog.ContentType][]string)
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
	if len(m.Prompts) > 0 {
		result[catalog.Prompts] = m.Prompts
	}
	if len(m.Apps) > 0 {
		result[catalog.Apps] = m.Apps
	}
	return result
}
