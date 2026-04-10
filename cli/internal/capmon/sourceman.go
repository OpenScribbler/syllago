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
type ContentTypeSource struct {
	Sources []SourceEntry `yaml:"sources"`
}

// SourceEntry is one source URL with its selector and extraction hints.
type SourceEntry struct {
	URL         string         `yaml:"url"`
	Type        string         `yaml:"type"`
	Format      string         `yaml:"format"`
	Selector    SelectorConfig `yaml:"selector"`
	Extracts    []string       `yaml:"extracts,omitempty"`
	FetchMethod string         `yaml:"fetch_method,omitempty"` // overrides manifest-level
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
