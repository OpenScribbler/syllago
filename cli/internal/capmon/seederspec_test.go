package capmon_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

const validSpecYAML = `provider: test-provider
content_type: skills
format: yaml-frontmatter
format_doc_provenance: https://example.com/docs
extraction_gaps:
  - license field not found in any source
source_excerpt: "name: My Skill"
proposed_mappings:
  - canonical_key: display_name
    supported: true
    mechanism: "yaml frontmatter key: name"
    source_field: name
    source_value: "My Skill"
    confidence: confirmed
    notes: "Directly mapped"
  - canonical_key: description
    supported: true
    mechanism: "yaml frontmatter key: description"
    source_field: description
    source_value: "Does things"
    confidence: inferred
notes: "Looks good"
`

func TestLoadSeederSpec_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-provider-skills.yaml")
	if err := os.WriteFile(path, []byte(validSpecYAML), 0644); err != nil {
		t.Fatal(err)
	}

	spec, err := capmon.LoadSeederSpec(path)
	if err != nil {
		t.Fatalf("LoadSeederSpec: %v", err)
	}

	if spec.Provider != "test-provider" {
		t.Errorf("Provider: got %q, want %q", spec.Provider, "test-provider")
	}
	if spec.ContentType != "skills" {
		t.Errorf("ContentType: got %q, want %q", spec.ContentType, "skills")
	}
	if spec.Format != "yaml-frontmatter" {
		t.Errorf("Format: got %q, want %q", spec.Format, "yaml-frontmatter")
	}
	if len(spec.ProposedMappings) != 2 {
		t.Fatalf("ProposedMappings: got %d, want 2", len(spec.ProposedMappings))
	}
	if spec.ProposedMappings[0].CanonicalKey != "display_name" {
		t.Errorf("ProposedMappings[0].CanonicalKey: got %q, want %q", spec.ProposedMappings[0].CanonicalKey, "display_name")
	}
	if spec.ProposedMappings[0].Confidence != "confirmed" {
		t.Errorf("ProposedMappings[0].Confidence: got %q, want %q", spec.ProposedMappings[0].Confidence, "confirmed")
	}
	if spec.ProposedMappings[1].Confidence != "inferred" {
		t.Errorf("ProposedMappings[1].Confidence: got %q, want %q", spec.ProposedMappings[1].Confidence, "inferred")
	}
}

func TestSeederSpecPath(t *testing.T) {
	cases := []struct {
		provider    string
		contentType string
		want        string
	}{
		{"my-provider", "skills", "/some/dir/my-provider-skills.yaml"},
		{"my-provider", "rules", "/some/dir/my-provider-rules.yaml"},
		{"my-provider", "agents", "/some/dir/my-provider-agents.yaml"},
	}
	for _, tc := range cases {
		got := capmon.SeederSpecPath("/some/dir", tc.provider, tc.contentType)
		if got != tc.want {
			t.Errorf("SeederSpecPath(%q, %q, %q): got %q, want %q",
				"/some/dir", tc.provider, tc.contentType, got, tc.want)
		}
	}
}
