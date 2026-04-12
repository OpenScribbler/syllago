package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// capFixtureDir creates a temp directory with the given YAML files under
// a provider-formats/ subdirectory. Returns the path to provider-formats/.
func capFixtureDir(t *testing.T, files map[string]string) string {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, "provider-formats")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	return dir
}

// loadCapManifest runs _gencapabilities with the given provider-formats dir and
// returns the parsed CapabilitiesManifest.
func loadCapManifest(t *testing.T, providerFormatsDir string) CapabilitiesManifest {
	t.Helper()
	orig := capabilitiesProviderFormatsDir
	capabilitiesProviderFormatsDir = providerFormatsDir
	t.Cleanup(func() { capabilitiesProviderFormatsDir = orig })

	raw := captureStdout(t, func() {
		if err := gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil); err != nil {
			t.Fatalf("_gencapabilities failed: %v", err)
		}
	})

	var manifest CapabilitiesManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("output is not valid JSON: %v\nfirst 200 bytes: %s",
			err, string(raw[:min(200, len(raw))]))
	}
	return manifest
}

const minimalSupportedYAML = `
provider: test-provider
last_fetched_at: "2026-04-12T00:00:00Z"
last_changed_at: "2026-04-11T00:00:00Z"
generation_method: subagent

content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com/docs"
        type: documentation
        fetch_method: md_url
        content_hash: "sha256:abc123"
        fetched_at: "2026-04-11T21:00:00Z"
    canonical_mappings:
      display_name:
        supported: true
        mechanism: "yaml frontmatter key: name"
        confidence: confirmed
      global_scope:
        supported: false
        mechanism: "not documented"
        confidence: confirmed
        paths: []
    provider_extensions:
      - id: cool_feature
        name: Cool Feature
        description: "Does something cool."
        source_ref: "https://example.com/docs#cool"
        graduation_candidate: true
`

const unsupportedTypeYAML = `
provider: sparse-provider
last_fetched_at: "2026-04-12T00:00:00Z"
last_changed_at: "2026-04-10T00:00:00Z"
generation_method: human-edited

content_types:
  skills:
    status: unsupported
    sources: []
    canonical_mappings: {}
    provider_extensions: []
`

const multipleContentTypesYAML = `
provider: rich-provider
last_fetched_at: "2026-04-12T00:00:00Z"
last_changed_at: "2026-04-11T12:00:00Z"
generation_method: subagent

content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com/skills"
        type: documentation
        fetch_method: md_url
        content_hash: "sha256:111"
        fetched_at: "2026-04-11T00:00:00Z"
    canonical_mappings:
      display_name:
        supported: true
        mechanism: "frontmatter name"
        confidence: confirmed
    provider_extensions: []
  rules:
    status: supported
    sources:
      - uri: "https://example.com/rules"
        type: documentation
        fetch_method: md_url
        content_hash: "sha256:222"
        fetched_at: "2026-04-11T00:00:00Z"
    canonical_mappings: {}
    provider_extensions:
      - id: rule_ext
        name: Rule Extension
        description: "An extension for rules."
        graduation_candidate: false
`

const extensionNoSourceRefYAML = `
provider: no-source-ref
last_fetched_at: "2026-04-12T00:00:00Z"
last_changed_at: "2026-04-11T00:00:00Z"
generation_method: human-edited

content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: no_ref_ext
        name: No Ref Extension
        description: "Extension without a source_ref."
`

// --- Root structure tests ---

func TestGencapabilities_RootStructure(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"test-provider.yaml": minimalSupportedYAML,
	})
	m := loadCapManifest(t, dir)

	if m.Version != "1" {
		t.Errorf("version = %q, want %q", m.Version, "1")
	}
	if m.GeneratedAt == "" {
		t.Error("generated_at is empty")
	}
	// generated_at must be a valid RFC3339 timestamp
	if !strings.Contains(m.GeneratedAt, "T") || !strings.Contains(m.GeneratedAt, "Z") {
		t.Errorf("generated_at %q does not look like RFC3339", m.GeneratedAt)
	}
	if m.Providers == nil {
		t.Error("providers is nil")
	}
}

// --- Provider presence and slug tests ---

func TestGencapabilities_ProviderSlugFromFilename(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"test-provider.yaml": minimalSupportedYAML,
	})
	m := loadCapManifest(t, dir)

	if _, ok := m.Providers["test-provider"]; !ok {
		t.Error("provider 'test-provider' missing from output")
	}
}

func TestGencapabilities_MultipleProviders(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"test-provider.yaml":   minimalSupportedYAML,
		"sparse-provider.yaml": unsupportedTypeYAML,
	})
	m := loadCapManifest(t, dir)

	if len(m.Providers) != 2 {
		t.Errorf("provider count = %d, want 2", len(m.Providers))
	}
	if _, ok := m.Providers["test-provider"]; !ok {
		t.Error("test-provider missing")
	}
	if _, ok := m.Providers["sparse-provider"]; !ok {
		t.Error("sparse-provider missing")
	}
}

// --- Content type tests ---

func TestGencapabilities_SupportedContentType(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"test-provider.yaml": minimalSupportedYAML,
	})
	m := loadCapManifest(t, dir)

	skills, ok := m.Providers["test-provider"]["skills"]
	if !ok {
		t.Fatal("skills content type missing for test-provider")
	}
	if skills.Status != "supported" {
		t.Errorf("status = %q, want %q", skills.Status, "supported")
	}
}

func TestGencapabilities_UnsupportedContentType(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"sparse-provider.yaml": unsupportedTypeYAML,
	})
	m := loadCapManifest(t, dir)

	skills, ok := m.Providers["sparse-provider"]["skills"]
	if !ok {
		t.Fatal("skills content type missing for sparse-provider")
	}
	if skills.Status != "unsupported" {
		t.Errorf("status = %q, want %q", skills.Status, "unsupported")
	}
}

func TestGencapabilities_MultipleContentTypes(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"rich-provider.yaml": multipleContentTypesYAML,
	})
	m := loadCapManifest(t, dir)

	prov := m.Providers["rich-provider"]
	if _, ok := prov["skills"]; !ok {
		t.Error("skills content type missing")
	}
	if _, ok := prov["rules"]; !ok {
		t.Error("rules content type missing")
	}
}

// --- Field filtering tests (the critical correctness requirement) ---

func TestGencapabilities_ConfidenceNotEmitted(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"test-provider.yaml": minimalSupportedYAML,
	})
	raw := captureStdout(t, func() {
		orig := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = dir
		defer func() { capabilitiesProviderFormatsDir = orig }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil) //nolint:errcheck
	})

	if strings.Contains(string(raw), "confidence") {
		t.Error("output contains 'confidence' — internal field must not be emitted")
	}
}

func TestGencapabilities_GraduationCandidateNotEmitted(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"test-provider.yaml": minimalSupportedYAML,
	})
	raw := captureStdout(t, func() {
		orig := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = dir
		defer func() { capabilitiesProviderFormatsDir = orig }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil) //nolint:errcheck
	})

	if strings.Contains(string(raw), "graduation_candidate") {
		t.Error("output contains 'graduation_candidate' — internal field must not be emitted")
	}
}

func TestGencapabilities_GenerationMethodNotEmitted(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"test-provider.yaml": minimalSupportedYAML,
	})
	raw := captureStdout(t, func() {
		orig := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = dir
		defer func() { capabilitiesProviderFormatsDir = orig }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil) //nolint:errcheck
	})

	if strings.Contains(string(raw), "generation_method") {
		t.Error("output contains 'generation_method' — internal field must not be emitted")
	}
}

func TestGencapabilities_ContentHashNotEmitted(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"test-provider.yaml": minimalSupportedYAML,
	})
	raw := captureStdout(t, func() {
		orig := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = dir
		defer func() { capabilitiesProviderFormatsDir = orig }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil) //nolint:errcheck
	})

	if strings.Contains(string(raw), "content_hash") {
		t.Error("output contains 'content_hash' — internal field must not be emitted")
	}
}

func TestGencapabilities_FetchMethodNotEmitted(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"test-provider.yaml": minimalSupportedYAML,
	})
	raw := captureStdout(t, func() {
		orig := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = dir
		defer func() { capabilitiesProviderFormatsDir = orig }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil) //nolint:errcheck
	})

	if strings.Contains(string(raw), "fetch_method") {
		t.Error("output contains 'fetch_method' — internal field must not be emitted")
	}
}

// --- Source field tests ---

func TestGencapabilities_SourceFieldsPresent(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"test-provider.yaml": minimalSupportedYAML,
	})
	m := loadCapManifest(t, dir)

	sources := m.Providers["test-provider"]["skills"].Sources
	if len(sources) != 1 {
		t.Fatalf("sources count = %d, want 1", len(sources))
	}
	s := sources[0]
	if s.URI != "https://example.com/docs" {
		t.Errorf("source.uri = %q, want %q", s.URI, "https://example.com/docs")
	}
	if s.Type != "documentation" {
		t.Errorf("source.type = %q, want %q", s.Type, "documentation")
	}
	if s.FetchedAt != "2026-04-11T21:00:00Z" {
		t.Errorf("source.fetched_at = %q, want %q", s.FetchedAt, "2026-04-11T21:00:00Z")
	}
}

// --- Canonical mapping tests ---

func TestGencapabilities_CanonicalMappingFields(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"test-provider.yaml": minimalSupportedYAML,
	})
	m := loadCapManifest(t, dir)

	mappings := m.Providers["test-provider"]["skills"].CanonicalMappings
	dn, ok := mappings["display_name"]
	if !ok {
		t.Fatal("display_name mapping missing")
	}
	if !dn.Supported {
		t.Error("display_name.supported = false, want true")
	}
	if dn.Mechanism != "yaml frontmatter key: name" {
		t.Errorf("display_name.mechanism = %q", dn.Mechanism)
	}

	gs, ok := mappings["global_scope"]
	if !ok {
		t.Fatal("global_scope mapping missing")
	}
	if gs.Supported {
		t.Error("global_scope.supported = true, want false")
	}
}

func TestGencapabilities_CanonicalMappingPathsOmittedWhenEmpty(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"test-provider.yaml": minimalSupportedYAML,
	})
	raw := captureStdout(t, func() {
		orig := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = dir
		defer func() { capabilitiesProviderFormatsDir = orig }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil) //nolint:errcheck
	})

	// global_scope in the fixture has paths: [] — should be omitted via omitempty
	var raw2 map[string]interface{}
	if err := json.Unmarshal(raw, &raw2); err != nil {
		t.Fatalf("parse: %v", err)
	}
	providers := raw2["providers"].(map[string]interface{})
	prov := providers["test-provider"].(map[string]interface{})
	skills := prov["skills"].(map[string]interface{})
	mappings := skills["canonical_mappings"].(map[string]interface{})
	gs := mappings["global_scope"].(map[string]interface{})
	if _, hasPath := gs["paths"]; hasPath {
		t.Error("global_scope has 'paths' key but it should be omitted when empty")
	}
}

// --- Provider extension tests ---

func TestGencapabilities_ExtensionFields(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"test-provider.yaml": minimalSupportedYAML,
	})
	m := loadCapManifest(t, dir)

	exts := m.Providers["test-provider"]["skills"].ProviderExtensions
	if len(exts) != 1 {
		t.Fatalf("extensions count = %d, want 1", len(exts))
	}
	ext := exts[0]
	if ext.ID != "cool_feature" {
		t.Errorf("extension.id = %q, want %q", ext.ID, "cool_feature")
	}
	if ext.Name != "Cool Feature" {
		t.Errorf("extension.name = %q, want %q", ext.Name, "Cool Feature")
	}
	if ext.SourceRef != "https://example.com/docs#cool" {
		t.Errorf("extension.source_ref = %q", ext.SourceRef)
	}
}

func TestGencapabilities_ExtensionSourceRefOmittedWhenMissing(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"no-source-ref.yaml": extensionNoSourceRefYAML,
	})
	m := loadCapManifest(t, dir)

	exts := m.Providers["no-source-ref"]["skills"].ProviderExtensions
	if len(exts) != 1 {
		t.Fatalf("extensions count = %d, want 1", len(exts))
	}
	if exts[0].SourceRef != "" {
		t.Errorf("extension.source_ref = %q, want empty", exts[0].SourceRef)
	}
}

// --- last_changed_at propagation test ---

func TestGencapabilities_LastChangedAtPropagated(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"test-provider.yaml": minimalSupportedYAML,
	})
	m := loadCapManifest(t, dir)

	skills := m.Providers["test-provider"]["skills"]
	if skills.LastChangedAt != "2026-04-11T00:00:00Z" {
		t.Errorf("last_changed_at = %q, want %q", skills.LastChangedAt, "2026-04-11T00:00:00Z")
	}
}

// --- Empty collections vs nil tests ---

func TestGencapabilities_EmptySourcesNotNil(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"sparse-provider.yaml": unsupportedTypeYAML,
	})
	m := loadCapManifest(t, dir)

	skills := m.Providers["sparse-provider"]["skills"]
	// sources: [] in YAML should produce [] not null in JSON
	raw, _ := json.Marshal(skills)
	if strings.Contains(string(raw), `"sources":null`) {
		t.Error("sources is null in JSON, want []")
	}
}

func TestGencapabilities_EmptyMappingsNotNil(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"sparse-provider.yaml": unsupportedTypeYAML,
	})
	m := loadCapManifest(t, dir)

	skills := m.Providers["sparse-provider"]["skills"]
	raw, _ := json.Marshal(skills)
	if strings.Contains(string(raw), `"canonical_mappings":null`) {
		t.Error("canonical_mappings is null in JSON, want {}")
	}
}

func TestGencapabilities_EmptyExtensionsNotNil(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"sparse-provider.yaml": unsupportedTypeYAML,
	})
	m := loadCapManifest(t, dir)

	skills := m.Providers["sparse-provider"]["skills"]
	raw, _ := json.Marshal(skills)
	if strings.Contains(string(raw), `"provider_extensions":null`) {
		t.Error("provider_extensions is null in JSON, want []")
	}
}

// --- Integration test against real provider-format files ---

func TestGencapabilities_AllRealProviders(t *testing.T) {
	// Locate the real docs/provider-formats directory relative to this file's
	// package location. The test binary runs from cli/cmd/syllago, so walk up
	// to find the repo root via the presence of docs/provider-formats/.
	repoRoot := findRepoRoot(t)
	providerFormatsDir := filepath.Join(repoRoot, "docs", "provider-formats")

	if _, err := os.Stat(providerFormatsDir); os.IsNotExist(err) {
		t.Skip("docs/provider-formats not found — skipping integration test")
	}

	m := loadCapManifest(t, providerFormatsDir)

	// Must have all 14 known providers.
	wantProviders := []string{
		"amp", "claude-code", "cline", "codex", "copilot-cli",
		"cursor", "factory-droid", "gemini-cli", "kiro", "opencode",
		"pi", "roo-code", "windsurf", "zed",
	}
	for _, slug := range wantProviders {
		if _, ok := m.Providers[slug]; !ok {
			t.Errorf("provider %q missing from real output", slug)
		}
	}

	// Version and generated_at must be present.
	if m.Version != "1" {
		t.Errorf("version = %q, want %q", m.Version, "1")
	}
	if m.GeneratedAt == "" {
		t.Error("generated_at is empty")
	}

	// No internal fields in raw JSON.
	raw := captureStdout(t, func() {
		orig := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = providerFormatsDir
		defer func() { capabilitiesProviderFormatsDir = orig }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil) //nolint:errcheck
	})
	for _, forbidden := range []string{"confidence", "graduation_candidate", "generation_method", "content_hash", "fetch_method"} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("real output contains forbidden field %q", forbidden)
		}
	}
}

// TestGencapabilities_ClaudeCodeSkillsHasMappings verifies that the most
// data-rich provider (claude-code skills) produces non-empty canonical_mappings
// and provider_extensions in the real output.
func TestGencapabilities_ClaudeCodeSkillsHasMappings(t *testing.T) {
	repoRoot := findRepoRoot(t)
	providerFormatsDir := filepath.Join(repoRoot, "docs", "provider-formats")
	if _, err := os.Stat(providerFormatsDir); os.IsNotExist(err) {
		t.Skip("docs/provider-formats not found")
	}

	m := loadCapManifest(t, providerFormatsDir)

	cc, ok := m.Providers["claude-code"]
	if !ok {
		t.Fatal("claude-code missing")
	}
	skills, ok := cc["skills"]
	if !ok {
		t.Fatal("claude-code skills missing")
	}
	if len(skills.CanonicalMappings) == 0 {
		t.Error("claude-code skills canonical_mappings is empty")
	}
	if len(skills.ProviderExtensions) == 0 {
		t.Error("claude-code skills provider_extensions is empty")
	}
	if len(skills.Sources) == 0 {
		t.Error("claude-code skills sources is empty")
	}
}
