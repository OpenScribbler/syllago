package capmon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testFormatDocYAML = `provider: amp
last_fetched_at: "2026-04-10T14:00:00Z"
last_changed_at: "2026-04-08T09:00:00Z"
generation_method: subagent

content_types:
  skills:
    status: supported
    sources:
      - uri: "https://ampcode.com/manual/agent-skills.md"
        type: documentation
        fetch_method: md_url
        content_hash: "sha256:abc123def456"
        fetched_at: "2026-04-10T14:00:00Z"
    canonical_mappings:
      display_name:
        supported: true
        mechanism: "yaml frontmatter key: name (required)"
        confidence: confirmed
      project_scope:
        supported: true
        mechanism: ".agents/skills/<name>/SKILL.md"
        paths:
          - ".agents/skills/<name>/SKILL.md"
        confidence: confirmed
    provider_extensions:
      - id: mcp_bundling
        name: "MCP server bundling"
        summary: "Skills can include an mcp.json file to bundle an MCP server."
        source_ref: "https://ampcode.com/manual/agent-skills.md"
        graduation_candidate: false
        graduation_notes: ""
        conversion: embedded
    loading_model: "Lazy — loaded on demand"
    notes: ""
`

func TestLoadFormatDoc_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "amp.yaml")
	if err := os.WriteFile(path, []byte(testFormatDocYAML), 0644); err != nil {
		t.Fatal(err)
	}

	doc, err := LoadFormatDoc(path)
	if err != nil {
		t.Fatalf("LoadFormatDoc: %v", err)
	}

	// Basic fields
	if doc.Provider != "amp" {
		t.Errorf("Provider = %q, want %q", doc.Provider, "amp")
	}
	if doc.LastFetchedAt != "2026-04-10T14:00:00Z" {
		t.Errorf("LastFetchedAt = %q, want %q", doc.LastFetchedAt, "2026-04-10T14:00:00Z")
	}
	if doc.GenerationMethod != "subagent" {
		t.Errorf("GenerationMethod = %q, want %q", doc.GenerationMethod, "subagent")
	}

	// Content type
	skills, ok := doc.ContentTypes["skills"]
	if !ok {
		t.Fatal("content_types.skills missing")
	}
	if skills.Status != "supported" {
		t.Errorf("skills.Status = %q, want %q", skills.Status, "supported")
	}

	// Sources — content_hash must survive round-trip
	if len(skills.Sources) != 1 {
		t.Fatalf("skills.Sources len = %d, want 1", len(skills.Sources))
	}
	src := skills.Sources[0]
	if src.ContentHash != "sha256:abc123def456" {
		t.Errorf("source content_hash = %q, want %q", src.ContentHash, "sha256:abc123def456")
	}
	if src.URI != "https://ampcode.com/manual/agent-skills.md" {
		t.Errorf("source uri = %q, want %q", src.URI, "https://ampcode.com/manual/agent-skills.md")
	}

	// Canonical mappings
	dm, ok := skills.CanonicalMappings["display_name"]
	if !ok {
		t.Fatal("canonical_mappings.display_name missing")
	}
	if !dm.Supported {
		t.Error("display_name.supported should be true")
	}
	if dm.Confidence != "confirmed" {
		t.Errorf("display_name.confidence = %q, want %q", dm.Confidence, "confirmed")
	}

	// provider_extensions
	if len(skills.ProviderExtensions) != 1 {
		t.Fatalf("provider_extensions len = %d, want 1", len(skills.ProviderExtensions))
	}
	ext := skills.ProviderExtensions[0]
	if ext.ID != "mcp_bundling" {
		t.Errorf("extension ID = %q, want %q", ext.ID, "mcp_bundling")
	}
	if ext.SourceRef != "https://ampcode.com/manual/agent-skills.md" {
		t.Errorf("extension SourceRef = %q, want %q", ext.SourceRef, "https://ampcode.com/manual/agent-skills.md")
	}
	if ext.GraduationCandidate {
		t.Error("graduation_candidate should be false")
	}
}

func TestFormatDocPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		formatsDir string
		provider   string
		want       string
	}{
		{"docs/provider-formats", "amp", "docs/provider-formats/amp.yaml"},
		{"docs/provider-formats", "claude-code", "docs/provider-formats/claude-code.yaml"},
		{"/abs/path", "zed", "/abs/path/zed.yaml"},
	}
	for _, tc := range cases {
		got := FormatDocPath(tc.formatsDir, tc.provider)
		if got != tc.want {
			t.Errorf("FormatDocPath(%q, %q) = %q, want %q", tc.formatsDir, tc.provider, got, tc.want)
		}
	}
}

func TestLoadFormatDoc_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := LoadFormatDoc("/nonexistent/path/file.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// TestValidateAllFormatDocs validates every .yaml file in docs/provider-formats/
// against the canonical-keys.yaml vocabulary. This test acts as a gatekeeper:
// all real format docs must pass ValidateFormatDoc before merging.
func TestValidateAllFormatDocs(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	formatsDir := filepath.Join(repoRoot, "docs", "provider-formats")
	canonicalKeysPath := filepath.Join(repoRoot, "docs", "spec", "canonical-keys.yaml")

	if _, err := os.Stat(formatsDir); os.IsNotExist(err) {
		t.Skip("docs/provider-formats dir not found")
	}

	entries, err := os.ReadDir(formatsDir)
	if err != nil {
		t.Fatalf("read formats dir: %v", err)
	}

	var providers []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".yaml" {
			providers = append(providers, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	if len(providers) == 0 {
		t.Skip("no .yaml format docs found")
	}

	for _, provider := range providers {
		provider := provider
		t.Run(provider, func(t *testing.T) {
			t.Parallel()
			if err := ValidateFormatDoc(formatsDir, canonicalKeysPath, provider); err != nil {
				t.Errorf("ValidateFormatDoc(%q): %v", provider, err)
			}
		})
	}
}

func TestProviderExtension_NewFieldRoundTrip(t *testing.T) {
	t.Parallel()
	yamlContent := `
provider: test-provider
last_fetched_at: "2026-04-14T00:00:00Z"
generation_method: human-edited
content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: model_field
        name: Model Field
        summary: "Controls which model is used."
        source_ref: "https://example.com"
        required: true
        value_type: "string"
        conversion: embedded
        examples:
          - title: "Fast model"
            lang: yaml
            code: |
              model: claude-haiku
            note: "Default if absent."
      - id: optional_field
        name: Optional Field
        summary: "An optional capability."
        source_ref: "https://example.com"
        required: false
        conversion: embedded
      - id: unspecified_field
        name: Unspecified Field
        summary: "We do not know if this is required."
        source_ref: "https://example.com"
        conversion: embedded
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test-provider.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	doc, err := LoadFormatDoc(path)
	if err != nil {
		t.Fatalf("LoadFormatDoc: %v", err)
	}
	exts := doc.ContentTypes["skills"].ProviderExtensions
	if len(exts) != 3 {
		t.Fatalf("expected 3 extensions, got %d", len(exts))
	}

	// required: true
	if exts[0].Required == nil || *exts[0].Required != true {
		t.Errorf("exts[0].Required: want *true, got %v", exts[0].Required)
	}
	if exts[0].ValueType != "string" {
		t.Errorf("exts[0].ValueType = %q, want %q", exts[0].ValueType, "string")
	}
	if len(exts[0].Examples) != 1 {
		t.Fatalf("exts[0].Examples: want 1, got %d", len(exts[0].Examples))
	}
	ex := exts[0].Examples[0]
	if ex.Title != "Fast model" || ex.Lang != "yaml" || ex.Note != "Default if absent." {
		t.Errorf("example fields wrong: %+v", ex)
	}
	if ex.Code == "" {
		t.Error("example.code must be non-empty")
	}

	// required: false
	if exts[1].Required == nil || *exts[1].Required != false {
		t.Errorf("exts[1].Required: want *false, got %v", exts[1].Required)
	}

	// required absent → nil
	if exts[2].Required != nil {
		t.Errorf("exts[2].Required: want nil, got %v", exts[2].Required)
	}
}
