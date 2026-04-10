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
	ExtractionGaps      []string          `yaml:"extraction_gaps,omitempty"`
	SourceExcerpt       string            `yaml:"source_excerpt,omitempty"`
	ProposedMappings    []ProposedMapping `yaml:"proposed_mappings"`
	HumanAction         string            `yaml:"human_action"` // approve | adjust | skip
	ReviewedAt          string            `yaml:"reviewed_at,omitempty"`
	Notes               string            `yaml:"notes,omitempty"`
}

// SeederSpecPath returns the conventional path for a seeder spec file given
// the directory and the provider slug.
func SeederSpecPath(specsDir, provider string) string {
	return filepath.Join(specsDir, provider+"-skills.yaml")
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

// ValidateSeederSpec checks that the spec has a valid human_action and that
// all proposed mappings have valid confidence values.
func ValidateSeederSpec(spec *SeederSpec) error {
	switch spec.HumanAction {
	case "":
		return fmt.Errorf("seeder spec for %q has no human_action set; set to approve, adjust, or skip after review", spec.Provider)
	case "approve", "adjust":
		if spec.ReviewedAt == "" {
			return fmt.Errorf("seeder spec for %q has human_action %q but no reviewed_at timestamp", spec.Provider, spec.HumanAction)
		}
	case "skip":
		// valid without reviewed_at
	default:
		return fmt.Errorf("seeder spec for %q has invalid human_action %q; must be approve, adjust, or skip", spec.Provider, spec.HumanAction)
	}

	validConfidence := map[string]bool{
		"confirmed": true,
		"inferred":  true,
		"unknown":   true,
		"":          true,
	}
	for _, m := range spec.ProposedMappings {
		if !validConfidence[m.Confidence] {
			return fmt.Errorf("seeder spec for %q: proposed mapping %q has invalid confidence %q; must be confirmed, inferred, unknown, or empty", spec.Provider, m.CanonicalKey, m.Confidence)
		}
	}
	return nil
}
