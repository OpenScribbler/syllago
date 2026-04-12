# `_gencapabilities` Implementation Plan

*Date: 2026-04-12*
*Design doc: `docs/plans/2026-04-12-syllago-docs-capabilities-pipeline-design.md`*
*Scope: Phase 1 only — Go command + tests + release integration*

---

## Overview

4 tasks in strict sequence. Each task is TDD (RED → GREEN → commit).

```
T1 → T2 → T3 → T4
```

- **T1:** Define Go types for the output schema (no logic)
- **T2:** Write failing tests against those types (RED)
- **T3:** Implement `_gencapabilities` to make tests pass (GREEN)
- **T4:** Integrate into `release.yml`

---

## Background: How This Differs from `_genproviders`

`_genproviders` reads data from Go structs in the `provider` package (compiled-in). `_gencapabilities` reads from YAML files on disk (`docs/provider-formats/*.yaml`). The output structures are completely separate — this command does not reuse any `_genproviders` types.

The YAML input contains internal fields (`confidence`, `graduation_candidate`, `generation_method`, `content_hash`, `fetch_method`) that must be read but never emitted. The output schema in the design doc is the complete spec: no fields added beyond what is documented.

---

## Task T1: Define Output Types in `gencapabilities.go`

**Description:** Create `cli/cmd/syllago/gencapabilities.go` with all Go types needed for the JSON output schema and YAML input parsing. No logic beyond type definitions and the cobra command registration. This is the foundation the tests in T2 compile against.

**Files to create:**
- `cli/cmd/syllago/gencapabilities.go`

**Input YAML types** — these map to the provider-format files. Fields present in YAML but excluded from output are included in the input structs with their correct types so the YAML parser can read them; they are simply never referenced when building output.

```go
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
    Provider       string                         `yaml:"provider"`
    LastFetchedAt  string                         `yaml:"last_fetched_at"`   // internal only
    LastChangedAt  string                         `yaml:"last_changed_at"`
    GenerationMethod string                       `yaml:"generation_method"` // internal only
    ContentTypes   map[string]capContentTypeYAML  `yaml:"content_types"`
}

// capContentTypeYAML is one entry under content_types in the provider YAML.
type capContentTypeYAML struct {
    Status             string                        `yaml:"status"`
    Sources            []capSourceYAML               `yaml:"sources"`
    CanonicalMappings  map[string]capMappingYAML     `yaml:"canonical_mappings"`
    ProviderExtensions []capExtensionYAML            `yaml:"provider_extensions"`
}

// capSourceYAML is a single entry in the sources list.
// fetch_method and content_hash are present in YAML but internal-only.
type capSourceYAML struct {
    URI         string `yaml:"uri"`
    Type        string `yaml:"type"`
    FetchMethod string `yaml:"fetch_method"`  // internal only
    ContentHash string `yaml:"content_hash"`  // internal only
    FetchedAt   string `yaml:"fetched_at"`
}

// capMappingYAML is a single canonical key entry under canonical_mappings.
// confidence is present in YAML but must never be emitted.
type capMappingYAML struct {
    Supported  bool     `yaml:"supported"`
    Mechanism  string   `yaml:"mechanism"`
    Paths      []string `yaml:"paths"`
    Confidence string   `yaml:"confidence"` // internal only
}

// capExtensionYAML is a single provider_extensions entry.
// graduation_candidate is present in YAML but must never be emitted.
type capExtensionYAML struct {
    ID                  string `yaml:"id"`
    Name                string `yaml:"name"`
    Description         string `yaml:"description"`
    SourceRef           string `yaml:"source_ref"`
    GraduationCandidate *bool  `yaml:"graduation_candidate"` // internal only
}

// --- JSON output types ---

// CapabilitiesManifest is the top-level structure of capabilities.json.
type CapabilitiesManifest struct {
    Version     string                               `json:"version"`
    GeneratedAt string                               `json:"generated_at"`
    Providers   map[string]map[string]CapContentType `json:"providers"`
}

// CapContentType describes a single provider+content_type combination in the output.
type CapContentType struct {
    Status             string                     `json:"status"`
    LastChangedAt      string                     `json:"last_changed_at"`
    Sources            []CapSource                `json:"sources"`
    CanonicalMappings  map[string]CapMapping      `json:"canonical_mappings"`
    ProviderExtensions []CapExtension             `json:"provider_extensions"`
}

// CapSource is the public-facing source entry (fetch_method and content_hash stripped).
type CapSource struct {
    URI       string `json:"uri"`
    Type      string `json:"type"`
    FetchedAt string `json:"fetched_at"`
}

// CapMapping is the public-facing canonical key entry (confidence stripped).
type CapMapping struct {
    Supported bool     `json:"supported"`
    Mechanism string   `json:"mechanism"`
    Paths     []string `json:"paths,omitempty"`
}

// CapExtension is the public-facing provider extension (graduation_candidate stripped).
// source_ref is omitempty because some extensions in the YAML omit it.
type CapExtension struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    SourceRef   string `json:"source_ref,omitempty"`
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
```

The `runGencapabilities` function body and helpers are added in T3.

**Why a new file instead of adding to an existing file:** Each gen command has its own file in this package (`genproviders.go`, `gentelemetry.go`, `gendocs.go`). Consistent with that pattern. Also avoids any sequencing risk with imports (per project CLAUDE.md Go edit patterns rule).

**Success criteria:**
- `cd cli && go build ./cmd/syllago` → pass — package compiles with new file (even with empty `runGencapabilities` stub returning `nil`)
- `cd cli && go vet ./cmd/syllago` → pass — no type errors or unused imports

---

## Task T2: Write Failing Tests

**Depends on:** T1

**Description:** Create `cli/cmd/syllago/gencapabilities_test.go` with all tests. These tests compile but fail because `runGencapabilities` is a stub returning `nil` with empty output. The test file defines fixtures inline rather than reading real YAML files, which keeps tests hermetic and fast.

The test helper `captureStdout` already exists in `genproviders_test.go` and is available within the same package.

**Files to create:**
- `cli/cmd/syllago/gencapabilities_test.go`

**Complete test file:**

```go
package main

import (
    "encoding/json"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

// fixtureDir creates a temp directory with the given YAML files under
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
        "test-provider.yaml":  minimalSupportedYAML,
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
        orig := capabilitiesProviderFormatsDir
        capabilitiesProviderFormatsDir = dir
        defer func() { capabilitiesProviderFormatsDir = orig }()
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
        orig := capabilitiesProviderFormatsDir
        capabilitiesProviderFormatsDir = dir
        defer func() { capabilitiesProviderFormatsDir = orig }()
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
        orig := capabilitiesProviderFormatsDir
        capabilitiesProviderFormatsDir = dir
        defer func() { capabilitiesProviderFormatsDir = orig }()
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
        orig := capabilitiesProviderFormatsDir
        capabilitiesProviderFormatsDir = dir
        defer func() { capabilitiesProviderFormatsDir = orig }()
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
        orig := capabilitiesProviderFormatsDir
        capabilitiesProviderFormatsDir = dir
        defer func() { capabilitiesProviderFormatsDir = orig }()
        gencapabilitiesCmd.RunE(gencapabilitiesCmd, nil)
    })

    // global_scope in the fixture has paths: [] — should be omitted via omitempty
    // We check the raw JSON doesn't have "paths":[] for that key.
    // Rather than parse deeply, check that null or [] is absent near "global_scope".
    // A reliable check: CapMapping.Paths has omitempty, so empty/nil means no key in JSON.
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

// findRepoRoot walks up from the test binary's working directory to find
// the repo root, identified by the presence of docs/provider-formats/.
func findRepoRoot(t *testing.T) string {
    t.Helper()
    dir, err := os.Getwd()
    if err != nil {
        t.Fatalf("getwd: %v", err)
    }
    for {
        candidate := filepath.Join(dir, "docs", "provider-formats")
        if _, err := os.Stat(candidate); err == nil {
            return dir
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            t.Fatal("cannot find repo root (docs/provider-formats not found in any ancestor)")
        }
        dir = parent
    }
}
```

**Why `capabilitiesProviderFormatsDir` is a package-level variable:** The same pattern used by `findProjectRoot` (a function override) in other commands, and by `catalog.GlobalContentDirOverride`. A package-level string var lets tests redirect the command to fixture dirs without touching the filesystem outside `t.TempDir()`. Declared in T3's implementation alongside `runGencapabilities`.

**Success criteria:**
- `cd cli && go build ./cmd/syllago` → pass — non-test package compiles cleanly (go build does not compile _test.go files, so the reference to `capabilitiesProviderFormatsDir` in the test file is not evaluated here)
- `cd cli && go test ./cmd/syllago/... -run TestGencapabilities` → fail — compile error: `undefined: capabilitiesProviderFormatsDir` (this variable is declared in T3; the compile failure is the expected RED state for T2)

---

## Task T3: Implement `runGencapabilities`

**Depends on:** T2

**Description:** Add the implementation to `gencapabilities.go` by appending the package-level `capabilitiesProviderFormatsDir` variable, `runGencapabilities`, and two private helpers `loadProviderFormatsDir` and `buildCapEntry` after the `init()` function. T1 already includes `"strings"` in the import block — do NOT add it again (duplicate imports cause a compile error). All tests from T2 must pass.

**Files to modify:**
- `cli/cmd/syllago/gencapabilities.go`

**Complete implementation to append after the `init()` function:**

```go
// capabilitiesProviderFormatsDir is the path to the directory containing
// docs/provider-formats/*.yaml files. Overridable in tests.
var capabilitiesProviderFormatsDir = filepath.Join("..", "docs", "provider-formats")

func runGencapabilities(_ *cobra.Command, _ []string) error {
    entries, err := loadProviderFormatsDir(capabilitiesProviderFormatsDir)
    if err != nil {
        return fmt.Errorf("loading provider formats: %w", err)
    }

    manifest := CapabilitiesManifest{
        Version:     "1",
        GeneratedAt: time.Now().UTC().Format(time.RFC3339),
        Providers:   entries,
    }

    enc := json.NewEncoder(os.Stdout)
    enc.SetIndent("", "  ")
    return enc.Encode(manifest)
}

// loadProviderFormatsDir reads all *.yaml files in dir and returns the
// providers map. The provider slug is derived from the filename stem
// (e.g., "claude-code.yaml" → "claude-code").
func loadProviderFormatsDir(dir string) (map[string]map[string]CapContentType, error) {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return nil, fmt.Errorf("readdir %q: %w", dir, err)
    }

    providers := make(map[string]map[string]CapContentType)

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
            return nil, fmt.Errorf("reading %q: %w", path, err)
        }

        var doc capYAML
        if err := yaml.Unmarshal(raw, &doc); err != nil {
            return nil, fmt.Errorf("parsing %q: %w", path, err)
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
    }

    return providers, nil
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
        })
    }

    // Build canonical mappings — strip confidence.
    mappings := make(map[string]CapMapping, len(ct.CanonicalMappings))
    for key, m := range ct.CanonicalMappings {
        var paths []string
        if len(m.Paths) > 0 {
            paths = m.Paths
        }
        mappings[key] = CapMapping{
            Supported: m.Supported,
            Mechanism: m.Mechanism,
            Paths:     paths,
        }
    }

    // Build provider extensions — strip graduation_candidate.
    extensions := make([]CapExtension, 0, len(ct.ProviderExtensions))
    for _, ext := range ct.ProviderExtensions {
        extensions = append(extensions, CapExtension{
            ID:          ext.ID,
            Name:        ext.Name,
            Description: strings.TrimSpace(ext.Description),
            SourceRef:   ext.SourceRef,
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
```

**Implementation notes:**

1. **`capabilitiesProviderFormatsDir` default path** is `"../../../docs/provider-formats"`. When the command runs from its installed binary location (tests run from `cli/cmd/syllago`, production runs from `cli/`), the relative path resolves correctly. In production (release.yml), the binary runs from the `cli/` working directory, so the path resolves to `../docs/provider-formats` which is correct relative to `cli/`. Wait — actually the binary in release.yml runs from `cli/` where it was built, and `cli/` relative paths need to point 2 levels up to reach `docs/`. The correct default for the production case (binary running from `cli/`) is `"../docs/provider-formats"`. But tests run from `cli/cmd/syllago/` and need 3 levels up. The solution: use a path that works for the binary, and override in tests. The test calls `loadCapManifest` which sets `capabilitiesProviderFormatsDir` to the fixture dir, so the default value only matters for production. Set default to `"../docs/provider-formats"` (relative to where the binary runs from in release.yml: `cli/`).

   The integration test `TestGencapabilities_AllRealProviders` uses `findRepoRoot()` to find the absolute path, so the default value does not affect it.

2. **`strings.TrimSpace` on Description:** YAML folded scalars (the `>` block style used for extension descriptions) include a trailing `\n`. `TrimSpace` normalizes this. This is consistent with the YAML sanitization rule in MEMORY.md ("Sanitize all catalog text — YAML folded scalars add trailing \n").

3. **`sources`, `CanonicalMappings`, `ProviderExtensions` initialized with `make(..., 0)`:** Ensures `[]` not `null` in JSON output for empty collections. This is what the `TestGencapabilities_Empty*NotNil` tests verify.

4. **No `SyllagoVersion` in `CapabilitiesManifest`:** The design doc output schema does not include it. The capabilities data comes from YAML files, not the binary, so the binary version is not meaningful context for consumers.

**Success criteria:**
- `cd cli && go test ./cmd/syllago/... -run TestGencapabilities` → pass — all 22 tests pass
- `cd cli && go test ./cmd/syllago/... -run TestGencapabilities_AllRealProviders -v` → pass — all 14 real providers present, no forbidden fields in output
- `cd cli && go build -o /tmp/syllago-test ./cmd/syllago && /tmp/syllago-test _gencapabilities | python3 -m json.tool > /dev/null` → pass — output is valid JSON

---

## Task T4: Release Integration

**Depends on:** T3

**Description:** Add `_gencapabilities` to `.github/workflows/release.yml` in the same step that runs `_genproviders`. Also add `capabilities.json` to both `gh release create` calls and to `sha256sum` in the checksums step.

**Files to modify:**
- `.github/workflows/release.yml`

**Change 1 — Generate step** (the step named "Generate commands.json, providers.json, and telemetry.json"):

Current `run` block:
```yaml
run: |
  LDFLAGS="-X main.version=${VERSION}"
  go build -ldflags "$LDFLAGS" -o syllago-gendocs ./cmd/syllago
  ./syllago-gendocs _gendocs > commands.json
  ./syllago-gendocs _genproviders > providers.json
  ./syllago-gendocs _gentelemetry > telemetry.json
  rm -f syllago-gendocs
```

Updated `run` block:
```yaml
run: |
  LDFLAGS="-X main.version=${VERSION}"
  go build -ldflags "$LDFLAGS" -o syllago-gendocs ./cmd/syllago
  ./syllago-gendocs _gendocs > commands.json
  ./syllago-gendocs _genproviders > providers.json
  ./syllago-gendocs _gentelemetry > telemetry.json
  ./syllago-gendocs _gencapabilities > capabilities.json
  rm -f syllago-gendocs
```

Also update the step name to reflect the new artifact:
```yaml
name: Generate commands.json, providers.json, telemetry.json, and capabilities.json
```

**Change 2 — Checksums step** (the step named "Generate checksums"):

Add `capabilities.json` to the `sha256sum` invocation:
```yaml
run: |
  sha256sum syllago-linux-amd64 syllago-linux-arm64 \
    syllago-darwin-amd64 syllago-darwin-arm64 \
    syllago-windows-amd64.exe syllago-windows-arm64.exe \
    commands.json providers.json telemetry.json capabilities.json sbom.spdx.json \
    > checksums.txt
```

**Change 3 — Both `gh release create` calls:**

Add `capabilities.json` to the artifact list in both the "Create GitHub Release (with release notes)" and "Create GitHub Release (auto-generated notes)" steps. Both currently end with:
```
commands.json providers.json telemetry.json sbom.spdx.json \
checksums.txt checksums.txt.bundle
```

Updated to:
```
commands.json providers.json telemetry.json capabilities.json sbom.spdx.json \
checksums.txt checksums.txt.bundle
```

**Why `_gencapabilities` runs after `_genproviders`:** The generate step uses a single binary invocation for all gen commands. Running capabilities last is safer since it reads from YAML files on disk (not Go structs) — if the YAML path resolution fails in CI, the other three artifacts are already generated before the failure. In practice, the YAML files are checked in, so this order has no real effect.

**Why `capabilities.json` is included in the checksum:** Consistent with all other release artifacts. Downstream consumers (syllago-docs sync script) should be able to verify integrity.

**Success criteria:**
- Validate the YAML change is syntactically correct: `python3 -c "import yaml, sys; yaml.safe_load(sys.stdin)" < .github/workflows/release.yml` → pass
- Inspect the diff: `git diff .github/workflows/release.yml` shows 5 added lines and 4 removed lines — the step name is a modification (-1 +1), the `_gencapabilities` invocation is a pure addition (+1), and `capabilities.json` appears in three modified lines (sha256sum + 2 gh release create blocks, each -1 +1) → pass, correctly isolated
- Code review: `capabilities.json` appears in the same positions as `providers.json` in all three modified blocks → pass

---

## Appendix: Path Resolution in Production vs Tests

The `capabilitiesProviderFormatsDir` default is `"../docs/provider-formats"`. This resolves correctly when:

1. **In release.yml:** The generate step runs with `working-directory: cli`. The binary built as `./syllago-gendocs` is invoked from `cli/`. `../docs/provider-formats` → `docs/provider-formats` from the repo root. Correct.

2. **In unit tests (`TestGencapabilities_*` with fixtures):** The test overrides `capabilitiesProviderFormatsDir` to a `t.TempDir()` path, so the default is not used.

3. **In the integration test (`TestGencapabilities_AllRealProviders`):** `findRepoRoot()` walks up from `os.Getwd()` (which is `cli/cmd/syllago` during `go test`) to find the repo root by the presence of `docs/provider-formats/`. It then passes the absolute path to `loadCapManifest`, overriding the default. The default is not used.

The default value `"../docs/provider-formats"` could theoretically be hit if someone runs the binary from the `cli/` directory directly on their machine. This is the intended use case for local testing of the gen commands.
