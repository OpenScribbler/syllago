package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// minimalCanonicalKeysYAML is a minimal valid canonical-keys.yaml fixture for
// unit tests that need loadCanonicalKeys to succeed without a real spec file.
const minimalCanonicalKeysYAML = `
content_types:
  skills:
    display_name:
      description: "Human-readable display name for the skill."
      type: string
`

// capFixtureDir creates a temp directory with the given YAML files under a
// provider-formats/ subdirectory, and also writes a minimal
// spec/canonical-keys.yaml so unit tests don't require the real spec file.
// Returns the path to provider-formats/.
func capFixtureDir(t *testing.T, files map[string]string) string {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, "provider-formats")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir provider-formats: %v", err)
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	specDir := filepath.Join(base, "spec")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatalf("mkdir spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "canonical-keys.yaml"), []byte(minimalCanonicalKeysYAML), 0644); err != nil {
		t.Fatalf("write canonical-keys.yaml: %v", err)
	}
	return dir
}

// loadCapManifest runs _gencapabilities with the given provider-formats dir and
// returns the parsed CapabilitiesManifest.
// It derives the canonical-keys spec path from the parent of providerFormatsDir
// (e.g. <base>/spec/canonical-keys.yaml), which works for both temp unit-test
// trees created by capFixtureDir and real repo trees (docs/provider-formats →
// docs/spec/canonical-keys.yaml).
func loadCapManifest(t *testing.T, providerFormatsDir string) CapabilitiesManifest {
	t.Helper()

	origDir := capabilitiesProviderFormatsDir
	capabilitiesProviderFormatsDir = providerFormatsDir
	t.Cleanup(func() { capabilitiesProviderFormatsDir = origDir })

	origSpec := canonicalKeysSpecPath
	canonicalKeysSpecPath = filepath.Join(filepath.Dir(providerFormatsDir), "spec", "canonical-keys.yaml")
	t.Cleanup(func() { canonicalKeysSpecPath = origSpec })

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
		origDir := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = dir
		defer func() { capabilitiesProviderFormatsDir = origDir }()
		origSpec := canonicalKeysSpecPath
		canonicalKeysSpecPath = filepath.Join(filepath.Dir(dir), "spec", "canonical-keys.yaml")
		defer func() { canonicalKeysSpecPath = origSpec }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil)
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
		origDir := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = dir
		defer func() { capabilitiesProviderFormatsDir = origDir }()
		origSpec := canonicalKeysSpecPath
		canonicalKeysSpecPath = filepath.Join(filepath.Dir(dir), "spec", "canonical-keys.yaml")
		defer func() { canonicalKeysSpecPath = origSpec }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil)
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
		origDir := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = dir
		defer func() { capabilitiesProviderFormatsDir = origDir }()
		origSpec := canonicalKeysSpecPath
		canonicalKeysSpecPath = filepath.Join(filepath.Dir(dir), "spec", "canonical-keys.yaml")
		defer func() { canonicalKeysSpecPath = origSpec }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil)
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
		origDir := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = dir
		defer func() { capabilitiesProviderFormatsDir = origDir }()
		origSpec := canonicalKeysSpecPath
		canonicalKeysSpecPath = filepath.Join(filepath.Dir(dir), "spec", "canonical-keys.yaml")
		defer func() { canonicalKeysSpecPath = origSpec }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil)
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
		origDir := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = dir
		defer func() { capabilitiesProviderFormatsDir = origDir }()
		origSpec := canonicalKeysSpecPath
		canonicalKeysSpecPath = filepath.Join(filepath.Dir(dir), "spec", "canonical-keys.yaml")
		defer func() { canonicalKeysSpecPath = origSpec }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil)
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
		origDir := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = dir
		defer func() { capabilitiesProviderFormatsDir = origDir }()
		origSpec := canonicalKeysSpecPath
		canonicalKeysSpecPath = filepath.Join(filepath.Dir(dir), "spec", "canonical-keys.yaml")
		defer func() { canonicalKeysSpecPath = origSpec }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil)
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
		origDir := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = providerFormatsDir
		defer func() { capabilitiesProviderFormatsDir = origDir }()
		origSpec := canonicalKeysSpecPath
		canonicalKeysSpecPath = filepath.Join(filepath.Dir(providerFormatsDir), "spec", "canonical-keys.yaml")
		defer func() { canonicalKeysSpecPath = origSpec }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil)
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

// --- Provider extension new fields (required, value_type, examples) ---

const extensionWithNewFieldsYAML = `
provider: rich-extension
last_fetched_at: "2026-04-14T00:00:00Z"
last_changed_at: "2026-04-14T00:00:00Z"
generation_method: subagent

content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: model_field
        name: Model Field
        description: "Which model to use."
        source_ref: "https://example.com"
        required: true
        value_type: "string"
        examples:
          - lang: yaml
            code: "model: claude-haiku"
            note: "Fast tier."
      - id: opt_field
        name: Optional Field
        description: "An optional thing."
        source_ref: "https://example.com"
        required: false
      - id: unspec_field
        name: Unspecified Field
        description: "Unknown required status."
        source_ref: "https://example.com"
        graduation_candidate: false
`

func TestGencapabilities_ExtensionRequiredFieldTrue(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"rich-extension.yaml": extensionWithNewFieldsYAML,
	})
	m := loadCapManifest(t, dir)
	exts := m.Providers["rich-extension"]["skills"].ProviderExtensions
	if len(exts) < 1 {
		t.Fatalf("expected >=1 extensions, got %d", len(exts))
	}
	if exts[0].Required == nil || *exts[0].Required != true {
		t.Errorf("exts[0].Required: want *true, got %v", exts[0].Required)
	}
	if exts[0].ValueType != "string" {
		t.Errorf("exts[0].ValueType = %q, want %q", exts[0].ValueType, "string")
	}
}

func TestGencapabilities_ExtensionRequiredFieldFalse(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"rich-extension.yaml": extensionWithNewFieldsYAML,
	})
	m := loadCapManifest(t, dir)
	exts := m.Providers["rich-extension"]["skills"].ProviderExtensions
	if len(exts) < 2 {
		t.Fatalf("expected >=2 extensions, got %d", len(exts))
	}
	if exts[1].Required == nil || *exts[1].Required != false {
		t.Errorf("exts[1].Required: want *false, got %v", exts[1].Required)
	}
}

func TestGencapabilities_ExtensionRequiredFieldNull(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"rich-extension.yaml": extensionWithNewFieldsYAML,
	})
	raw := captureStdout(t, func() {
		origDir := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = dir
		defer func() { capabilitiesProviderFormatsDir = origDir }()
		origSpec := canonicalKeysSpecPath
		canonicalKeysSpecPath = filepath.Join(filepath.Dir(dir), "spec", "canonical-keys.yaml")
		defer func() { canonicalKeysSpecPath = origSpec }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil)
	})
	// Encoder uses SetIndent("  ","  "), so "required": null with a space. Accept either form for robustness.
	s := string(raw)
	if !strings.Contains(s, `"required": null`) && !strings.Contains(s, `"required":null`) {
		t.Error("extension with absent required must emit \"required\": null in JSON")
	}
}

func TestGencapabilities_ExtensionExamplesPassthrough(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"rich-extension.yaml": extensionWithNewFieldsYAML,
	})
	m := loadCapManifest(t, dir)
	exts := m.Providers["rich-extension"]["skills"].ProviderExtensions
	if len(exts) < 1 {
		t.Fatalf("expected >=1 extensions, got %d", len(exts))
	}
	if len(exts[0].Examples) != 1 {
		t.Fatalf("exts[0].Examples: want 1, got %d", len(exts[0].Examples))
	}
	ex := exts[0].Examples[0]
	if ex.Lang != "yaml" {
		t.Errorf("example.lang = %q, want %q", ex.Lang, "yaml")
	}
	if ex.Code != "model: claude-haiku" {
		t.Errorf("example.code = %q, want %q", ex.Code, "model: claude-haiku")
	}
	if ex.Note != "Fast tier." {
		t.Errorf("example.note = %q, want %q", ex.Note, "Fast tier.")
	}
}

const extensionWithMixedQualityYAML = `
provider: quality-test
last_fetched_at: "2026-04-14T00:00:00Z"
last_changed_at: "2026-04-14T00:00:00Z"
generation_method: subagent

content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: full_ext
        name: Full Extension
        description: "Has all depth fields."
        source_ref: "https://example.com"
        required: true
        value_type: "string"
        examples:
          - lang: yaml
            code: "model: x"
      - id: bare_ext
        name: Bare Extension
        description: "Has no depth fields."
        source_ref: "https://example.com"
`

const extensionAllFullYAML = `
provider: all-full
last_fetched_at: "2026-04-14T00:00:00Z"
last_changed_at: "2026-04-14T00:00:00Z"
generation_method: subagent

content_types:
  skills:
    status: supported
    sources: []
    canonical_mappings: {}
    provider_extensions:
      - id: first
        name: First
        description: "Fully populated."
        source_ref: "https://example.com"
        required: false
        value_type: "bool"
        examples:
          - lang: yaml
            code: "enabled: true"
      - id: second
        name: Second
        description: "Also fully populated."
        source_ref: "https://example.com"
        required: true
        value_type: "string"
        examples:
          - lang: yaml
            code: "name: foo"
`

func TestGencapabilities_DataQualityBlockPresent(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"quality-test.yaml": extensionWithMixedQualityYAML,
	})
	raw := captureStdout(t, func() {
		orig := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = dir
		defer func() { capabilitiesProviderFormatsDir = orig }()
		origSpec := canonicalKeysSpecPath
		canonicalKeysSpecPath = filepath.Join(filepath.Dir(dir), "spec", "canonical-keys.yaml")
		defer func() { canonicalKeysSpecPath = origSpec }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil)
	})
	if !strings.Contains(string(raw), `"data_quality"`) {
		t.Error("manifest must contain \"data_quality\" key")
	}
}

func TestGencapabilities_DataQualityCountsAccurate(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"quality-test.yaml": extensionWithMixedQualityYAML,
	})
	m := loadCapManifest(t, dir)
	entry, ok := m.DataQuality.Providers["quality-test"]
	if !ok {
		t.Fatal("data_quality.providers.quality-test missing")
	}
	if entry.UnspecifiedRequiredCount != 1 {
		t.Errorf("UnspecifiedRequiredCount = %d, want 1", entry.UnspecifiedRequiredCount)
	}
	if entry.UnspecifiedValueTypeCount != 1 {
		t.Errorf("UnspecifiedValueTypeCount = %d, want 1", entry.UnspecifiedValueTypeCount)
	}
	if entry.UnspecifiedExamplesCount != 1 {
		t.Errorf("UnspecifiedExamplesCount = %d, want 1", entry.UnspecifiedExamplesCount)
	}
}

func TestGencapabilities_DataQualityAllZero(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"all-full.yaml": extensionAllFullYAML,
	})
	m := loadCapManifest(t, dir)
	entry, ok := m.DataQuality.Providers["all-full"]
	if !ok {
		t.Fatal("data_quality.providers.all-full missing")
	}
	if entry.UnspecifiedRequiredCount != 0 || entry.UnspecifiedValueTypeCount != 0 || entry.UnspecifiedExamplesCount != 0 {
		t.Errorf("all counts must be zero when every extension is fully populated, got %+v", entry)
	}
}

func TestGencapabilities_DataQualityGeneratedAtIsRFC3339UTC(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"quality-test.yaml": extensionWithMixedQualityYAML,
	})
	m := loadCapManifest(t, dir)
	if _, err := time.Parse(time.RFC3339, m.GeneratedAt); err != nil {
		t.Errorf("generated_at %q is not RFC3339: %v", m.GeneratedAt, err)
	}
	if !strings.HasSuffix(m.GeneratedAt, "Z") {
		t.Errorf("generated_at %q must end with Z (UTC)", m.GeneratedAt)
	}
}

const sourceWithNameSectionYAML = `
provider: named-source
last_fetched_at: "2026-04-14T00:00:00Z"
last_changed_at: "2026-04-14T00:00:00Z"
generation_method: subagent

content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com/docs"
        type: documentation
        fetch_method: md_url
        content_hash: "sha256:zzz"
        fetched_at: "2026-04-14T00:00:00Z"
        name: "Skills Handbook"
        section: "Authoring Skills"
    canonical_mappings: {}
    provider_extensions: []
`

func TestGencapabilities_SourceNameAndSectionPassthrough(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"named-source.yaml": sourceWithNameSectionYAML,
	})
	m := loadCapManifest(t, dir)
	srcs := m.Providers["named-source"]["skills"].Sources
	if len(srcs) != 1 {
		t.Fatalf("sources len = %d, want 1", len(srcs))
	}
	if srcs[0].Name != "Skills Handbook" {
		t.Errorf("source.name = %q, want %q", srcs[0].Name, "Skills Handbook")
	}
	if srcs[0].Section != "Authoring Skills" {
		t.Errorf("source.section = %q, want %q", srcs[0].Section, "Authoring Skills")
	}
}

func TestGencapabilities_GraduationCandidateStillNotEmitted(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"rich-extension.yaml": extensionWithNewFieldsYAML,
	})
	raw := captureStdout(t, func() {
		origDir := capabilitiesProviderFormatsDir
		capabilitiesProviderFormatsDir = dir
		defer func() { capabilitiesProviderFormatsDir = origDir }()
		origSpec := canonicalKeysSpecPath
		canonicalKeysSpecPath = filepath.Join(filepath.Dir(dir), "spec", "canonical-keys.yaml")
		defer func() { canonicalKeysSpecPath = origSpec }()
		gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil)
	})
	if strings.Contains(string(raw), "graduation_candidate") {
		t.Error("regression: new extension fields must not re-emit 'graduation_candidate'")
	}
}

// TestGencapabilities_TrackingIssueFromSidecar verifies that a <slug>.quality.json
// sidecar populates DataQuality.Providers[slug].TrackingIssue in the generated
// manifest. The sidecar is the back-channel by which the capmon pipeline surfaces
// open GitHub issues per provider.
func TestGencapabilities_TrackingIssueFromSidecar(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"quality-test.yaml": extensionWithMixedQualityYAML,
	})
	// Write a sidecar alongside the YAML.
	const issueURL = "https://github.com/example/syllago/issues/42"
	sidecar := []byte(`{"tracking_issue":"` + issueURL + `"}`)
	if err := os.WriteFile(filepath.Join(dir, "quality-test.quality.json"), sidecar, 0644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	manifest := loadCapManifest(t, dir)
	entry, ok := manifest.DataQuality.Providers["quality-test"]
	if !ok {
		t.Fatal("expected quality-test in DataQuality.Providers")
	}
	if entry.TrackingIssue != issueURL {
		t.Errorf("TrackingIssue = %q, want %q", entry.TrackingIssue, issueURL)
	}
}

// TestGencapabilities_TrackingIssueMissingSidecarIsFine verifies that absence of
// a <slug>.quality.json sidecar does not error and leaves TrackingIssue empty.
func TestGencapabilities_TrackingIssueMissingSidecarIsFine(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"quality-test.yaml": extensionWithMixedQualityYAML,
	})

	manifest := loadCapManifest(t, dir)
	entry, ok := manifest.DataQuality.Providers["quality-test"]
	if !ok {
		t.Fatal("expected quality-test in DataQuality.Providers")
	}
	if entry.TrackingIssue != "" {
		t.Errorf("TrackingIssue should be empty without sidecar, got %q", entry.TrackingIssue)
	}
}

// TestGencapabilities_TrackingIssueInvalidSidecarFails verifies that a malformed
// sidecar JSON produces a clear error rather than silently dropping the issue URL.
func TestGencapabilities_TrackingIssueInvalidSidecarFails(t *testing.T) {
	dir := capFixtureDir(t, map[string]string{
		"quality-test.yaml": extensionWithMixedQualityYAML,
	})
	if err := os.WriteFile(filepath.Join(dir, "quality-test.quality.json"), []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	origDir := capabilitiesProviderFormatsDir
	capabilitiesProviderFormatsDir = dir
	t.Cleanup(func() { capabilitiesProviderFormatsDir = origDir })
	origSpec := canonicalKeysSpecPath
	canonicalKeysSpecPath = filepath.Join(filepath.Dir(dir), "spec", "canonical-keys.yaml")
	t.Cleanup(func() { canonicalKeysSpecPath = origSpec })

	err := gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil)
	if err == nil {
		t.Fatal("expected error for malformed sidecar JSON")
	}
	if !strings.Contains(err.Error(), "quality-test.quality.json") {
		t.Errorf("error should mention the offending sidecar path, got: %v", err)
	}
}
