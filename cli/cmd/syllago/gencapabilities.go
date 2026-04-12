package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// --- YAML input types ---

// capYAML is the top-level structure of a docs/provider-formats/*.yaml file.
// Fields present in YAML but internal-only are included so yaml.v3 parses them
// without warnings; they are never referenced when building the JSON output.
type capYAML struct {
	Provider         string                        `yaml:"provider"`
	LastFetchedAt    string                        `yaml:"last_fetched_at"` // internal only
	LastChangedAt    string                        `yaml:"last_changed_at"`
	GenerationMethod string                        `yaml:"generation_method"` // internal only
	ContentTypes     map[string]capContentTypeYAML `yaml:"content_types"`
}

// capContentTypeYAML is one entry under content_types in the provider YAML.
type capContentTypeYAML struct {
	Status             string                    `yaml:"status"`
	Sources            []capSourceYAML           `yaml:"sources"`
	CanonicalMappings  map[string]capMappingYAML `yaml:"canonical_mappings"`
	ProviderExtensions []capExtensionYAML        `yaml:"provider_extensions"`
}

// capSourceYAML is a single entry in the sources list.
// fetch_method and content_hash are present in YAML but internal-only.
type capSourceYAML struct {
	URI         string `yaml:"uri"`
	Type        string `yaml:"type"`
	FetchMethod string `yaml:"fetch_method"` // internal only
	ContentHash string `yaml:"content_hash"` // internal only
	FetchedAt   string `yaml:"fetched_at"`
}

// capMappingYAML is a single canonical key entry under canonical_mappings.
// confidence is present in YAML but must never be emitted.
type capMappingYAML struct {
	Supported  bool     `yaml:"supported"`
	Mechanism  string   `yaml:"mechanism"`
	Paths      []string `yaml:"paths"`
	Confidence string   `yaml:"confidence"` // internal only
}

// capExtensionYAML is a single provider_extensions entry.
// graduation_candidate is present in YAML but must never be emitted.
type capExtensionYAML struct {
	ID                  string `yaml:"id"`
	Name                string `yaml:"name"`
	Description         string `yaml:"description"`
	SourceRef           string `yaml:"source_ref"`
	GraduationCandidate *bool  `yaml:"graduation_candidate"` // internal only
}

// --- JSON output types ---

// CapabilitiesManifest is the top-level structure of capabilities.json.
type CapabilitiesManifest struct {
	Version     string                               `json:"version"`
	GeneratedAt string                               `json:"generated_at"`
	Providers   map[string]map[string]CapContentType `json:"providers"`
}

// CapContentType describes a single provider+content_type combination in the output.
type CapContentType struct {
	Status             string                `json:"status"`
	LastChangedAt      string                `json:"last_changed_at"`
	Sources            []CapSource           `json:"sources"`
	CanonicalMappings  map[string]CapMapping `json:"canonical_mappings"`
	ProviderExtensions []CapExtension        `json:"provider_extensions"`
}

// CapSource is the public-facing source entry (fetch_method and content_hash stripped).
type CapSource struct {
	URI       string `json:"uri"`
	Type      string `json:"type"`
	FetchedAt string `json:"fetched_at"`
}

// CapMapping is the public-facing canonical key entry (confidence stripped).
type CapMapping struct {
	Supported bool     `json:"supported"`
	Mechanism string   `json:"mechanism"`
	Paths     []string `json:"paths,omitempty"`
}

// CapExtension is the public-facing provider extension (graduation_candidate stripped).
// source_ref is omitempty because some extensions in the YAML omit it.
type CapExtension struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SourceRef   string `json:"source_ref,omitempty"`
}

// --- Cobra command ---

var gencapabilitiesCmd = &cobra.Command{
	Use:    "_gencapabilities",
	Short:  "Generate capabilities.json manifest from provider format YAML files",
	Hidden: true,
	RunE:   runGencapabilities,
}

func init() {
	rootCmd.AddCommand(gencapabilitiesCmd)
}

// capabilitiesProviderFormatsDir is the path to the directory containing
// docs/provider-formats/*.yaml files. Overridable in tests.
var capabilitiesProviderFormatsDir = filepath.Join("..", "docs", "provider-formats")

func runGencapabilities(_ *cobra.Command, _ []string) error {
	entries, err := loadProviderFormatsDir(capabilitiesProviderFormatsDir)
	if err != nil {
		return fmt.Errorf("loading provider formats: %w", err)
	}

	manifest := CapabilitiesManifest{
		Version:     "1",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Providers:   entries,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(manifest)
}

// loadProviderFormatsDir reads all *.yaml files in dir and returns the
// providers map. The provider slug is derived from the filename stem
// (e.g., "claude-code.yaml" → "claude-code").
func loadProviderFormatsDir(dir string) (map[string]map[string]CapContentType, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("readdir %q: %w", dir, err)
	}

	providers := make(map[string]map[string]CapContentType)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}

		slug := strings.TrimSuffix(name, ".yaml")
		path := filepath.Join(dir, name)

		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", path, err)
		}

		var doc capYAML
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("parsing %q: %w", path, err)
		}

		contentTypes := make(map[string]CapContentType)
		// Sort content type keys for deterministic output.
		ctKeys := make([]string, 0, len(doc.ContentTypes))
		for k := range doc.ContentTypes {
			ctKeys = append(ctKeys, k)
		}
		sort.Strings(ctKeys)

		for _, ctKey := range ctKeys {
			ct := doc.ContentTypes[ctKey]
			contentTypes[ctKey] = buildCapEntry(doc.LastChangedAt, ct)
		}

		providers[slug] = contentTypes
	}

	return providers, nil
}

// buildCapEntry converts a YAML content type entry to the public JSON output
// structure, stripping all internal-only fields.
func buildCapEntry(lastChangedAt string, ct capContentTypeYAML) CapContentType {
	// Build sources — strip fetch_method and content_hash.
	sources := make([]CapSource, 0, len(ct.Sources))
	for _, s := range ct.Sources {
		sources = append(sources, CapSource{
			URI:       s.URI,
			Type:      s.Type,
			FetchedAt: s.FetchedAt,
		})
	}

	// Build canonical mappings — strip confidence.
	mappings := make(map[string]CapMapping, len(ct.CanonicalMappings))
	for key, m := range ct.CanonicalMappings {
		var paths []string
		if len(m.Paths) > 0 {
			paths = m.Paths
		}
		mappings[key] = CapMapping{
			Supported: m.Supported,
			Mechanism: m.Mechanism,
			Paths:     paths,
		}
	}

	// Build provider extensions — strip graduation_candidate.
	extensions := make([]CapExtension, 0, len(ct.ProviderExtensions))
	for _, ext := range ct.ProviderExtensions {
		extensions = append(extensions, CapExtension{
			ID:          ext.ID,
			Name:        ext.Name,
			Description: strings.TrimSpace(ext.Description),
			SourceRef:   ext.SourceRef,
		})
	}

	return CapContentType{
		Status:             ct.Status,
		LastChangedAt:      lastChangedAt,
		Sources:            sources,
		CanonicalMappings:  mappings,
		ProviderExtensions: extensions,
	}
}
