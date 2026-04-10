// Package provmon implements provider source manifest loading, URL health
// checking, and change detection for the provider monitoring pipeline.
//
// It reads YAML manifests from docs/provider-sources/ and provides:
//   - Manifest loading and validation
//   - Concurrent URL health checking (HEAD requests)
//   - Change detection via GitHub Releases API or content hashing
package provmon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Manifest represents a single provider source manifest (one YAML file).
type Manifest struct {
	SchemaVersion   string          `yaml:"schema_version"`
	LastVerified    string          `yaml:"last_verified"`
	ProviderVersion string          `yaml:"provider_version"`
	Slug            string          `yaml:"slug"`
	DisplayName     string          `yaml:"display_name"`
	Vendor          string          `yaml:"vendor"`
	Status          string          `yaml:"status"`     // active | archived | beta
	FetchTier       string          `yaml:"fetch_tier"` // gh-api | llms-txt | html-scrape
	Repo            string          `yaml:"repo,omitempty"`
	RepoBranch      string          `yaml:"repo_branch,omitempty"`
	Successor       *Successor      `yaml:"successor,omitempty"`
	ChangeDetection ChangeDetection `yaml:"change_detection"`
	Changelog       []LabeledURL    `yaml:"changelog,omitempty"`
	ConfigSchema    *ConfigSchema   `yaml:"config_schema,omitempty"`
	ContentTypes    ContentTypes    `yaml:"content_types"`
}

// Successor points to the active replacement for an archived provider.
type Successor struct {
	Slug        string `yaml:"slug"`
	DisplayName string `yaml:"display_name,omitempty"`
	Repo        string `yaml:"repo"`
	RepoBranch  string `yaml:"repo_branch,omitempty"`
	Status      string `yaml:"status,omitempty"`
}

// ChangeDetection defines how to detect when provider content changes.
type ChangeDetection struct {
	Method   string `yaml:"method"` // github-releases | github-commits | content-hash
	Endpoint string `yaml:"endpoint"`
}

// LabeledURL is a URL with a human-readable label.
type LabeledURL struct {
	URL   string `yaml:"url"`
	Label string `yaml:"label"`
}

// ConfigSchema points to a machine-readable config schema covering multiple content types.
type ConfigSchema struct {
	URL    string `yaml:"url"`
	Format string `yaml:"format"` // json-schema | json | zod | rust-schemars | go-struct | none
	Draft  string `yaml:"draft,omitempty"`
}

// ContentTypes holds source definitions for each content type.
type ContentTypes struct {
	Rules    ContentType `yaml:"rules"`
	Hooks    ContentType `yaml:"hooks"`
	MCP      ContentType `yaml:"mcp"`
	Skills   ContentType `yaml:"skills"`
	Agents   ContentType `yaml:"agents"`
	Commands ContentType `yaml:"commands"`
}

// ContentType is either a list of sources (supported) or supported=false.
type ContentType struct {
	Supported *bool         `yaml:"supported,omitempty"` // nil means supported (has sources)
	Sources   []SourceEntry `yaml:"sources,omitempty"`
}

// IsSupported returns true if this content type has sources or supported is not explicitly false.
func (ct ContentType) IsSupported() bool {
	if ct.Supported != nil {
		return *ct.Supported
	}
	return len(ct.Sources) > 0
}

// SourceEntry is a single fetchable URL with metadata about what it contains.
type SourceEntry struct {
	URL      string   `yaml:"url"`
	Type     string   `yaml:"type"`     // schema | source-code | docs | example
	Format   string   `yaml:"format"`   // json-schema | typescript | rust | go | markdown | etc.
	Extracts []string `yaml:"extracts"` // what data this URL provides
	JSONPath string   `yaml:"json_path,omitempty"`
}

// AllURLs returns every URL referenced in the manifest (sources, changelog, config schema, change detection).
func (m *Manifest) AllURLs() []string {
	var urls []string

	urls = append(urls, m.ChangeDetection.Endpoint)

	for _, cl := range m.Changelog {
		urls = append(urls, cl.URL)
	}

	if m.ConfigSchema != nil {
		urls = append(urls, m.ConfigSchema.URL)
	}

	for _, ct := range m.allContentTypes() {
		for _, src := range ct.Sources {
			urls = append(urls, src.URL)
		}
	}

	return urls
}

func (m *Manifest) allContentTypes() []ContentType {
	return []ContentType{
		m.ContentTypes.Rules,
		m.ContentTypes.Hooks,
		m.ContentTypes.MCP,
		m.ContentTypes.Skills,
		m.ContentTypes.Agents,
		m.ContentTypes.Commands,
	}
}

// LoadManifest reads and parses a single YAML manifest file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest %s: %w", path, err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest %s: %w", path, err)
	}

	if m.Slug == "" {
		return nil, fmt.Errorf("manifest %s: missing required field 'slug'", path)
	}

	return &m, nil
}

// LoadAllManifests reads all .yaml files from a directory (excluding _template.yaml).
func LoadAllManifests(dir string) ([]*Manifest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading manifest directory %s: %w", dir, err)
	}

	var manifests []*Manifest
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".yaml") || name == "_template.yaml" {
			continue
		}

		m, err := LoadManifest(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, m)
	}

	return manifests, nil
}
