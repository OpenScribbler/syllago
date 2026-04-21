package provmon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	t.Parallel()

	yaml := `
schema_version: "1"
last_verified: "2026-03-27"
slug: test-provider
display_name: Test Provider
vendor: Test Corp
status: active
fetch_tier: gh-api
repo: test/repo
repo_branch: main
change_detection:
  method: github-releases
  endpoint: https://api.github.com/repos/test/repo/releases/latest
  baseline: "v1.0.0"
changelog:
  - url: https://example.com/changelog
    label: Changelog
config_schema:
  url: https://example.com/schema.json
  format: json-schema
  draft: "2020-12"
content_types:
  rules:
    sources:
      - url: https://example.com/rules.md
        type: docs
        format: markdown
        extracts: [file_format, file_locations]
  hooks:
    supported: false
  mcp:
    sources:
      - url: https://example.com/mcp.md
        type: docs
        format: markdown
        extracts: [server_config_fields]
  skills:
    supported: false
  agents:
    supported: false
  commands:
    supported: false
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test-provider.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest() error: %v", err)
	}

	if m.Slug != "test-provider" {
		t.Errorf("Slug = %q, want %q", m.Slug, "test-provider")
	}
	if m.DisplayName != "Test Provider" {
		t.Errorf("DisplayName = %q, want %q", m.DisplayName, "Test Provider")
	}
	if m.Status != "active" {
		t.Errorf("Status = %q, want %q", m.Status, "active")
	}
	if m.FetchTier != "gh-api" {
		t.Errorf("FetchTier = %q, want %q", m.FetchTier, "gh-api")
	}
	if m.ConfigSchema == nil || m.ConfigSchema.Draft != "2020-12" {
		t.Error("ConfigSchema not parsed correctly")
	}

	// Rules should be supported with 1 source.
	if !m.ContentTypes.Rules.IsSupported() {
		t.Error("Rules should be supported")
	}
	if len(m.ContentTypes.Rules.Sources) != 1 {
		t.Errorf("Rules sources = %d, want 1", len(m.ContentTypes.Rules.Sources))
	}

	// Hooks should not be supported.
	if m.ContentTypes.Hooks.IsSupported() {
		t.Error("Hooks should not be supported")
	}
}

func TestLoadManifest_MissingSlugs(t *testing.T) {
	t.Parallel()

	yaml := `
schema_version: "1"
display_name: No Slug
vendor: Test
status: active
fetch_tier: gh-api
change_detection:
  method: source-hash
  endpoint: https://example.com
content_types:
  rules:
    supported: false
  hooks:
    supported: false
  mcp:
    supported: false
  skills:
    supported: false
  agents:
    supported: false
  commands:
    supported: false
`
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	_, err := LoadManifest(path)
	if err == nil {
		t.Fatal("expected error for missing slug")
	}
}

func TestAllURLs(t *testing.T) {
	t.Parallel()

	m := &Manifest{
		ChangeDetection: ChangeDetection{Endpoint: "https://api.example.com/releases"},
		Changelog:       []LabeledURL{{URL: "https://example.com/changelog"}},
		ConfigSchema:    &ConfigSchema{URL: "https://example.com/schema.json"},
		ContentTypes: ContentTypes{
			Rules: ContentType{Sources: []SourceEntry{
				{URL: "https://example.com/rules.md"},
				{URL: "https://example.com/rules2.md"},
			}},
			Hooks: ContentType{Sources: []SourceEntry{
				{URL: "https://example.com/hooks.ts"},
			}},
		},
	}

	urls := m.AllURLs()
	if len(urls) != 6 {
		t.Errorf("AllURLs() returned %d URLs, want 6", len(urls))
	}
}

func TestLoadAllManifests(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Write two valid manifests.
	for _, slug := range []string{"alpha", "beta"} {
		yaml := `
schema_version: "1"
last_verified: "2026-03-27"
slug: ` + slug + `
display_name: ` + slug + `
vendor: Test
status: active
fetch_tier: gh-api
change_detection:
  method: source-hash
  endpoint: https://example.com
content_types:
  rules:
    supported: false
  hooks:
    supported: false
  mcp:
    supported: false
  skills:
    supported: false
  agents:
    supported: false
  commands:
    supported: false
`
		os.WriteFile(filepath.Join(dir, slug+".yaml"), []byte(yaml), 0644)
	}

	// Write a template that should be skipped.
	os.WriteFile(filepath.Join(dir, "_template.yaml"), []byte("slug: skip"), 0644)

	// Write a non-yaml file that should be skipped.
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# hi"), 0644)

	manifests, err := LoadAllManifests(dir)
	if err != nil {
		t.Fatalf("LoadAllManifests() error: %v", err)
	}
	if len(manifests) != 2 {
		t.Errorf("LoadAllManifests() returned %d manifests, want 2", len(manifests))
	}
}

func TestLoadAllManifests_RealManifests(t *testing.T) {
	// Load the actual manifests from the repo. Skip if not in repo context.
	dir := filepath.Join("..", "..", "..", "docs", "provider-sources")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skip("docs/provider-sources not found — not running from repo root")
	}

	manifests, err := LoadAllManifests(dir)
	if err != nil {
		t.Fatalf("LoadAllManifests() error: %v", err)
	}

	if len(manifests) < 10 {
		t.Errorf("expected at least 10 manifests, got %d", len(manifests))
	}

	// Verify every manifest has required fields.
	for _, m := range manifests {
		t.Run(m.Slug, func(t *testing.T) {
			if m.Slug == "" {
				t.Error("missing slug")
			}
			if m.DisplayName == "" {
				t.Error("missing display_name")
			}
			if m.Status == "" {
				t.Error("missing status")
			}
			if m.FetchTier == "" {
				t.Error("missing fetch_tier")
			}
			if m.ChangeDetection.Method == "" {
				t.Error("missing change_detection.method")
			}

			// Every manifest should have at least some URLs.
			urls := m.AllURLs()
			if len(urls) == 0 {
				t.Error("manifest has no URLs")
			}
		})
	}
}
