package capmon

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProposedMapping describes how a single canonical capability key maps to a
// provider's native format fields.
type ProposedMapping struct {
	CanonicalKey string `yaml:"canonical_key"`
	Supported    bool   `yaml:"supported"`
	Mechanism    string `yaml:"mechanism"`
	SourceField  string `yaml:"source_field"`
	SourceValue  string `yaml:"source_value"`
	Confidence   string `yaml:"confidence"` // confirmed | inferred | unknown | ""
	Notes        string `yaml:"notes,omitempty"`
}

// SeederSpec is the human-in-the-loop review document produced by the
// inspection bead for a single (provider, content_type) pair.
type SeederSpec struct {
	Provider            string            `yaml:"provider"`
	ContentType         string            `yaml:"content_type"`
	Format              string            `yaml:"format"`
	FormatDocProvenance string            `yaml:"format_doc_provenance,omitempty"`
	SourceURIs          []string          `yaml:"source_uris,omitempty"`
	ExtractionGaps      []string          `yaml:"extraction_gaps,omitempty"`
	SourceExcerpt       string            `yaml:"source_excerpt,omitempty"`
	ProposedMappings    []ProposedMapping `yaml:"proposed_mappings"`
	Notes               string            `yaml:"notes,omitempty"`
}

// SeederSpecPath returns the conventional path for a seeder spec file given
// the directory, provider slug, and content type.
func SeederSpecPath(specsDir, provider, contentType string) string {
	return filepath.Join(specsDir, provider+"-"+contentType+".yaml")
}

// LoadSeederSpec reads and unmarshals a seeder spec YAML file at the given path.
func LoadSeederSpec(path string) (*SeederSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read seeder spec %s: %w", path, err)
	}
	var spec SeederSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse seeder spec %s: %w", path, err)
	}
	return &spec, nil
}
