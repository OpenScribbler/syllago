package capmon_test

import (
	"os"
	"path/filepath"
	"strings"
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
human_action: approve
reviewed_at: "2026-04-10T12:00:00Z"
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
	if spec.HumanAction != "approve" {
		t.Errorf("HumanAction: got %q, want %q", spec.HumanAction, "approve")
	}
	if spec.ReviewedAt != "2026-04-10T12:00:00Z" {
		t.Errorf("ReviewedAt: got %q, want %q", spec.ReviewedAt, "2026-04-10T12:00:00Z")
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
	got := capmon.SeederSpecPath("/some/dir", "my-provider")
	want := "/some/dir/my-provider-skills.yaml"
	if got != want {
		t.Errorf("SeederSpecPath: got %q, want %q", got, want)
	}
}

func TestValidateSeederSpec_EmptyHumanAction(t *testing.T) {
	spec := &capmon.SeederSpec{
		Provider:    "acme-provider",
		ContentType: "skills",
		HumanAction: "",
	}
	err := capmon.ValidateSeederSpec(spec)
	if err == nil {
		t.Fatal("expected error for empty human_action, got nil")
	}
	if !strings.Contains(err.Error(), "acme-provider") {
		t.Errorf("error should mention provider slug %q, got: %v", "acme-provider", err)
	}
}

func TestValidateSeederSpec_InvalidHumanAction(t *testing.T) {
	spec := &capmon.SeederSpec{
		Provider:    "acme-provider",
		ContentType: "skills",
		HumanAction: "invalid_value",
	}
	err := capmon.ValidateSeederSpec(spec)
	if err == nil {
		t.Fatal("expected error for invalid human_action, got nil")
	}
}

func TestValidateSeederSpec_ApproveWithoutReviewedAt(t *testing.T) {
	spec := &capmon.SeederSpec{
		Provider:    "acme-provider",
		ContentType: "skills",
		HumanAction: "approve",
		ReviewedAt:  "",
	}
	err := capmon.ValidateSeederSpec(spec)
	if err == nil {
		t.Fatal("expected error for approve without reviewed_at, got nil")
	}
}

func TestValidateSeederSpec_AdjustWithoutReviewedAt(t *testing.T) {
	spec := &capmon.SeederSpec{
		Provider:    "acme-provider",
		ContentType: "skills",
		HumanAction: "adjust",
		ReviewedAt:  "",
	}
	err := capmon.ValidateSeederSpec(spec)
	if err == nil {
		t.Fatal("expected error for adjust without reviewed_at, got nil")
	}
}

func TestValidateSeederSpec_SkipValid(t *testing.T) {
	spec := &capmon.SeederSpec{
		Provider:    "acme-provider",
		ContentType: "skills",
		HumanAction: "skip",
		ReviewedAt:  "", // skip does not require reviewed_at
	}
	err := capmon.ValidateSeederSpec(spec)
	if err != nil {
		t.Errorf("expected no error for skip without reviewed_at, got: %v", err)
	}
}

func TestValidateSeederSpec_InvalidConfidence(t *testing.T) {
	spec := &capmon.SeederSpec{
		Provider:    "acme-provider",
		ContentType: "skills",
		HumanAction: "skip",
		ProposedMappings: []capmon.ProposedMapping{
			{
				CanonicalKey: "display_name",
				Supported:    true,
				Confidence:   "invalid",
			},
		},
	}
	err := capmon.ValidateSeederSpec(spec)
	if err == nil {
		t.Fatal("expected error for invalid confidence value, got nil")
	}
}

func TestValidateSeederSpec_ConfirmedConfidence(t *testing.T) {
	spec := &capmon.SeederSpec{
		Provider:    "acme-provider",
		ContentType: "skills",
		HumanAction: "skip",
		ProposedMappings: []capmon.ProposedMapping{
			{
				CanonicalKey: "display_name",
				Supported:    true,
				Confidence:   "confirmed",
			},
		},
	}
	err := capmon.ValidateSeederSpec(spec)
	if err != nil {
		t.Errorf("expected no error for confirmed confidence, got: %v", err)
	}
}

func TestValidateSeederSpec_EmptyConfidenceAllowed(t *testing.T) {
	spec := &capmon.SeederSpec{
		Provider:    "acme-provider",
		ContentType: "skills",
		HumanAction: "skip",
		ProposedMappings: []capmon.ProposedMapping{
			{
				CanonicalKey: "display_name",
				Supported:    true,
				Confidence:   "",
			},
		},
	}
	err := capmon.ValidateSeederSpec(spec)
	if err != nil {
		t.Errorf("expected no error for empty confidence, got: %v", err)
	}
}

func TestValidateSeederSpec_ValidApproved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-provider-skills.yaml")
	if err := os.WriteFile(path, []byte(validSpecYAML), 0644); err != nil {
		t.Fatal(err)
	}

	spec, err := capmon.LoadSeederSpec(path)
	if err != nil {
		t.Fatalf("LoadSeederSpec: %v", err)
	}
	if err := capmon.ValidateSeederSpec(spec); err != nil {
		t.Errorf("ValidateSeederSpec on valid approved spec: %v", err)
	}
}
