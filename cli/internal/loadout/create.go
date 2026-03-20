package loadout

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"gopkg.in/yaml.v3"
)

// BuildManifest constructs a Manifest from discrete inputs.
// Kind="loadout" and Version=1 are set automatically.
// Empty slices in items are treated as absent (nil) so yaml omitempty drops them.
func BuildManifest(provider, name, description string, items map[catalog.ContentType][]string) *Manifest {
	m := &Manifest{
		Kind:        "loadout",
		Version:     1,
		Provider:    provider,
		Name:        name,
		Description: description,
	}

	// Only assign non-empty slices; empty slices become nil so omitempty works correctly.
	if v := items[catalog.Rules]; len(v) > 0 {
		m.Rules = v
	}
	if v := items[catalog.Hooks]; len(v) > 0 {
		m.Hooks = v
	}
	if v := items[catalog.Skills]; len(v) > 0 {
		m.Skills = v
	}
	if v := items[catalog.Agents]; len(v) > 0 {
		m.Agents = v
	}
	if v := items[catalog.MCP]; len(v) > 0 {
		m.MCP = v
	}
	if v := items[catalog.Commands]; len(v) > 0 {
		m.Commands = v
	}

	return m
}

// WriteManifest marshals m to YAML and writes to destDir/<m.Name>/loadout.yaml.
// Creates destDir/<m.Name>/ if needed. Returns the written file path.
func WriteManifest(m *Manifest, destDir string) (string, error) {
	outDir := filepath.Join(destDir, m.Name)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("creating loadout dir: %w", err)
	}

	data, err := yaml.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("marshaling manifest: %w", err)
	}

	outPath := filepath.Join(outDir, "loadout.yaml")
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return "", fmt.Errorf("writing loadout.yaml: %w", err)
	}

	return outPath, nil
}
