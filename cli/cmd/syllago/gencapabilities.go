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
	Name        string `yaml:"name,omitempty"`
	Section     string `yaml:"section,omitempty"`
}

// capMappingYAML is a single canonical key entry under canonical_mappings.
// confidence is present in YAML but must never be emitted.
//
// ProviderField names the actual native field (frontmatter key, config key,
// TOML field) when the mapping corresponds to a specific named field. Omitted
// when the mapping is structural or behavioral.
//
// ExtensionID points to a provider_extensions entry that describes the same
// concept in provider-specific detail. The downstream component renders one
// unified row when set.
type capMappingYAML struct {
	Supported     bool     `yaml:"supported"`
	Mechanism     string   `yaml:"mechanism"`
	Paths         []string `yaml:"paths"`
	Confidence    string   `yaml:"confidence"` // internal only
	ProviderField string   `yaml:"provider_field"`
	ExtensionID   string   `yaml:"extension_id"`
}

// capExtensionExampleYAML is one entry in provider_extensions[i].examples.
type capExtensionExampleYAML struct {
	Title string `yaml:"title,omitempty"`
	Lang  string `yaml:"lang"`
	Code  string `yaml:"code"`
	Note  string `yaml:"note,omitempty"`
}

// capExtensionYAML is a single provider_extensions entry.
// graduation_candidate is present in YAML but must never be emitted.
//
// Summary is one sentence (~150 chars max) — replaces the older Description
// field. ProviderField names the actual native field when the extension
// describes a frontmatter key, config key, or TOML field; absent for behavioral
// extensions. Conversion declares what happens to the feature during format
// conversion: translated | embedded | dropped | preserved | not-portable.
type capExtensionYAML struct {
	ID                  string                    `yaml:"id"`
	Name                string                    `yaml:"name"`
	Summary             string                    `yaml:"summary"`
	SourceRef           string                    `yaml:"source_ref"`
	GraduationCandidate *bool                     `yaml:"graduation_candidate"` // internal only
	Required            *bool                     `yaml:"required"`
	ValueType           string                    `yaml:"value_type,omitempty"`
	Examples            []capExtensionExampleYAML `yaml:"examples,omitempty"`
	ProviderField       string                    `yaml:"provider_field"`
	Conversion          string                    `yaml:"conversion"`
}

// canonicalKeysYAML is the top-level structure of docs/spec/canonical-keys.yaml.
type canonicalKeysYAML struct {
	ContentTypes map[string]map[string]canonicalKeyEntryYAML `yaml:"content_types"`
}

// canonicalKeyEntryYAML is a single key entry under content_types.<type>.
type canonicalKeyEntryYAML struct {
	Description string `yaml:"description"`
	Type        string `yaml:"type"`
}

// --- JSON output types ---

// CapabilitiesManifest is the top-level structure of capabilities.json.
type CapabilitiesManifest struct {
	Version       string                                 `json:"version"`
	GeneratedAt   string                                 `json:"generated_at"`
	DataQuality   DataQuality                            `json:"data_quality"`
	CanonicalKeys map[string]map[string]CanonicalKeyMeta `json:"canonical_keys"`
	Providers     map[string]map[string]CapContentType   `json:"providers"`
}

// DataQualityEntry holds unspecified-field counts for one provider.
type DataQualityEntry struct {
	UnspecifiedRequiredCount  int    `json:"unspecified_required_count"`
	UnspecifiedValueTypeCount int    `json:"unspecified_value_type_count"`
	UnspecifiedExamplesCount  int    `json:"unspecified_examples_count"`
	TrackingIssue             string `json:"tracking_issue,omitempty"`
}

// DataQuality is the top-level data_quality block in capabilities.json.
type DataQuality struct {
	Providers map[string]DataQualityEntry `json:"providers"`
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
	Name      string `json:"name,omitempty"`
	Section   string `json:"section,omitempty"`
}

// CapMapping is the public-facing canonical key entry (confidence stripped).
//
// ProviderField names the actual native field (e.g., "name", "description",
// "disable-model-invocation"); empty for structural mappings. ExtensionID
// links to a CapExtension entry that describes the same concept in
// provider-specific detail; downstream renders one unified row when set.
type CapMapping struct {
	Supported     bool     `json:"supported"`
	Mechanism     string   `json:"mechanism"`
	Paths         []string `json:"paths,omitempty"`
	ProviderField string   `json:"provider_field,omitempty"`
	ExtensionID   string   `json:"extension_id,omitempty"`
}

// CapExampleEntry is the public-facing example entry in provider_extensions.
type CapExampleEntry struct {
	Title string `json:"title,omitempty"`
	Lang  string `json:"lang"`
	Code  string `json:"code"`
	Note  string `json:"note,omitempty"`
}

// CapExtension is the public-facing provider extension (graduation_candidate stripped).
// source_ref is omitempty because some extensions in the YAML omit it.
// required has no omitempty: null must be emitted explicitly so downstream
// consumers can render a three-state badge (required / optional / unknown).
// conversion has no omitempty: every extension must declare its conversion
// behavior; an empty value indicates a missing required field, not a default.
type CapExtension struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Summary       string            `json:"summary"`
	SourceRef     string            `json:"source_ref,omitempty"`
	Required      *bool             `json:"required"`
	ValueType     string            `json:"value_type,omitempty"`
	Examples      []CapExampleEntry `json:"examples,omitempty"`
	ProviderField string            `json:"provider_field,omitempty"`
	Conversion    string            `json:"conversion"`
}

// CanonicalKeyMeta is a single canonical key's metadata in the JSON output.
type CanonicalKeyMeta struct {
	Description string `json:"description"`
	Type        string `json:"type"`
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

var canonicalKeysSpecPath = filepath.Join("..", "docs", "spec", "canonical-keys.yaml")

func loadCanonicalKeys(specPath string) (map[string]map[string]CanonicalKeyMeta, error) {
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("reading canonical-keys.yaml: %w", err)
	}

	var doc canonicalKeysYAML
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parsing canonical-keys.yaml: %w", err)
	}

	result := make(map[string]map[string]CanonicalKeyMeta, len(doc.ContentTypes))
	for ct, keys := range doc.ContentTypes {
		result[ct] = make(map[string]CanonicalKeyMeta, len(keys))
		for keyName, entry := range keys {
			result[ct][keyName] = CanonicalKeyMeta{
				Description: strings.TrimSpace(entry.Description),
				Type:        entry.Type,
			}
		}
	}
	return result, nil
}

func runGencapabilities(_ *cobra.Command, _ []string) error {
	entries, trackingIssues, err := loadProviderFormatsDir(capabilitiesProviderFormatsDir)
	if err != nil {
		return fmt.Errorf("loading provider formats: %w", err)
	}

	canonicalKeys, err := loadCanonicalKeys(canonicalKeysSpecPath)
	if err != nil {
		return fmt.Errorf("loading canonical keys: %w", err)
	}

	manifest := CapabilitiesManifest{
		Version:       "1",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		DataQuality:   computeDataQuality(entries, trackingIssues),
		CanonicalKeys: canonicalKeys,
		Providers:     entries,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(manifest)
}

// qualitySidecar is the minimal shape of docs/provider-formats/<slug>.quality.json.
// These sidecars are written by the capmon check pipeline (jtafb) when it opens a
// GitHub issue for an allow-list warning; they carry the issue URL back into
// generated capability manifests so operators can link from a data-quality row
// to the active tracking issue.
type qualitySidecar struct {
	TrackingIssue string `json:"tracking_issue"`
}

// loadProviderFormatsDir reads all *.yaml files in dir and returns the providers
// map plus a parallel map of slug → tracking issue URL, populated from optional
// <slug>.quality.json sidecars. The sidecars are gitignored and written by the
// capmon pipeline; missing sidecars yield an empty entry (not an error). The
// provider slug is derived from the filename stem (e.g., "claude-code.yaml" →
// "claude-code").
func loadProviderFormatsDir(dir string) (map[string]map[string]CapContentType, map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("readdir %q: %w", dir, err)
	}

	providers := make(map[string]map[string]CapContentType)
	trackingIssues := make(map[string]string)

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
			return nil, nil, fmt.Errorf("reading %q: %w", path, err)
		}

		var doc capYAML
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return nil, nil, fmt.Errorf("parsing %q: %w", path, err)
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

		// Optional sidecar: <slug>.quality.json. Missing is normal.
		sidecarPath := filepath.Join(dir, slug+".quality.json")
		sidecarBytes, err := os.ReadFile(sidecarPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, nil, fmt.Errorf("reading %q: %w", sidecarPath, err)
			}
			continue
		}
		var sc qualitySidecar
		if err := json.Unmarshal(sidecarBytes, &sc); err != nil {
			return nil, nil, fmt.Errorf("parsing %q: %w", sidecarPath, err)
		}
		if sc.TrackingIssue != "" {
			trackingIssues[slug] = sc.TrackingIssue
		}
	}

	return providers, trackingIssues, nil
}

// computeDataQuality builds the data_quality block from the already-built providers map.
// For each provider, counts extensions across all content types that have unspecified
// required / value_type / examples. trackingIssues[slug] populates the per-provider
// TrackingIssue URL when a <slug>.quality.json sidecar was read by loadProviderFormatsDir.
func computeDataQuality(providers map[string]map[string]CapContentType, trackingIssues map[string]string) DataQuality {
	dq := DataQuality{Providers: make(map[string]DataQualityEntry)}
	for slug, contentTypes := range providers {
		var entry DataQualityEntry
		for _, ct := range contentTypes {
			for _, ext := range ct.ProviderExtensions {
				if ext.Required == nil {
					entry.UnspecifiedRequiredCount++
				}
				if ext.ValueType == "" {
					entry.UnspecifiedValueTypeCount++
				}
				if len(ext.Examples) == 0 {
					entry.UnspecifiedExamplesCount++
				}
			}
		}
		entry.TrackingIssue = trackingIssues[slug]
		dq.Providers[slug] = entry
	}
	return dq
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
			Name:      s.Name,
			Section:   s.Section,
		})
	}

	// Build canonical mappings — strip confidence, propagate provider_field and extension_id.
	mappings := make(map[string]CapMapping, len(ct.CanonicalMappings))
	for key, m := range ct.CanonicalMappings {
		var paths []string
		if len(m.Paths) > 0 {
			paths = m.Paths
		}
		mappings[key] = CapMapping{
			Supported:     m.Supported,
			Mechanism:     m.Mechanism,
			Paths:         paths,
			ProviderField: m.ProviderField,
			ExtensionID:   m.ExtensionID,
		}
	}

	// Build provider extensions — strip graduation_candidate, propagate new fields.
	extensions := make([]CapExtension, 0, len(ct.ProviderExtensions))
	for _, ext := range ct.ProviderExtensions {
		examples := make([]CapExampleEntry, 0, len(ext.Examples))
		for _, ex := range ext.Examples {
			examples = append(examples, CapExampleEntry(ex))
		}
		extensions = append(extensions, CapExtension{
			ID:            ext.ID,
			Name:          ext.Name,
			Summary:       strings.TrimSpace(ext.Summary),
			SourceRef:     ext.SourceRef,
			Required:      ext.Required,
			ValueType:     ext.ValueType,
			Examples:      examples,
			ProviderField: ext.ProviderField,
			Conversion:    ext.Conversion,
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
