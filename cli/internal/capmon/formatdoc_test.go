package capmon

import (
	"os"
	"path/filepath"
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
        description: "Skills can include an mcp.json file to bundle an MCP server."
        source_ref: "https://ampcode.com/manual/agent-skills.md"
        graduation_candidate: false
        graduation_notes: ""
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
