package capmon

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SourceManifest is the parsed form of docs/provider-sources/<slug>.yaml.
type SourceManifest struct {
	SchemaVersion string                       `yaml:"schema_version"`
	Slug          string                       `yaml:"slug"`
	DisplayName   string                       `yaml:"display_name"`
	LastVerified  string                       `yaml:"last_verified"`
	FetchTier     string                       `yaml:"fetch_tier,omitempty"`
	FetchMethod   string                       `yaml:"fetch_method,omitempty"`
	ContentTypes  map[string]ContentTypeSource `yaml:"content_types"`
}

// ContentTypeSource groups all source entries for one content type.
// Supported is a pointer to distinguish "not specified" from explicitly false.
// When Supported is explicitly false, the content type is not supported by this provider
// and source URI checks are skipped.
//
// Convention names a cross-provider implementation pattern (e.g. cross-provider-agents-md,
// cross-provider-skill-md) when the provider has no native upstream documentation
// to monitor. Setting Convention satisfies the validator's "must have sources"
// requirement for content types implemented purely via convention.
type ContentTypeSource struct {
	Supported  *bool         `yaml:"supported,omitempty"`
	Convention string        `yaml:"convention,omitempty"`
	Sources    []SourceEntry `yaml:"sources"`
}

// SourceEntry is one source URL with its selector and extraction hints.
type SourceEntry struct {
	URL         string         `yaml:"url"`
	Type        string         `yaml:"type"`
	Format      string         `yaml:"format"`
	Selector    SelectorConfig `yaml:"selector"`
	Extracts    []string       `yaml:"extracts,omitempty"`
	FetchMethod string         `yaml:"fetch_method,omitempty"` // overrides manifest-level
	Healing     *HealingConfig `yaml:"healing,omitempty"`
}

// HealingConfig controls reactive URL healing behavior for a single source.
// When nil or unset, reactive healing runs with all default strategies.
// Set Enabled=false to opt out — useful for stable sources that should fail loudly.
type HealingConfig struct {
	Enabled    *bool    `yaml:"enabled,omitempty"`
	Strategies []string `yaml:"strategies,omitempty"`
}

// DefaultHealingStrategies lists the strategies tried in order when a source's
// Healing.Strategies list is empty. Each strategy is a pluggable heal path
// registered by the capmon package.
var DefaultHealingStrategies = []string{"redirect", "github-rename", "variant"}

// IsHealingEnabled reports whether reactive healing should run for this source.
// Default is true — callers must opt out via healing.enabled: false.
func (s SourceEntry) IsHealingEnabled() bool {
	if s.Healing == nil || s.Healing.Enabled == nil {
		return true
	}
	return *s.Healing.Enabled
}

// EffectiveStrategies returns the healing strategy list for this source.
// An empty or absent configuration falls back to DefaultHealingStrategies.
func (s SourceEntry) EffectiveStrategies() []string {
	if s.Healing == nil || len(s.Healing.Strategies) == 0 {
		return DefaultHealingStrategies
	}
	out := make([]string, len(s.Healing.Strategies))
	copy(out, s.Healing.Strategies)
	return out
}

// LoadSourceManifest parses a single provider-sources YAML file.
func LoadSourceManifest(path string) (*SourceManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read source manifest %s: %w", path, err)
	}
	var m SourceManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse source manifest %s: %w", path, err)
	}
	return &m, nil
}

// LoadAllSourceManifests loads all *.yaml files from a directory.
func LoadAllSourceManifests(dir string) ([]*SourceManifest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}
	var manifests []*SourceManifest
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		if e.Name() == "_template.yaml" {
			continue
		}
		m, err := LoadSourceManifest(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, m)
	}
	return manifests, nil
}
