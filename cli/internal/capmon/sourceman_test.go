package capmon_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestLoadSourceManifest(t *testing.T) {
	path := filepath.Join("testdata", "fixtures", "source-manifests", "claude-code-minimal.yaml")
	m, err := capmon.LoadSourceManifest(path)
	if err != nil {
		t.Fatalf("LoadSourceManifest: %v", err)
	}
	if m.Slug != "claude-code" {
		t.Errorf("Slug = %q, want %q", m.Slug, "claude-code")
	}
	hooks, ok := m.ContentTypes["hooks"]
	if !ok {
		t.Fatal("no hooks content type")
	}
	if len(hooks.Sources) == 0 {
		t.Error("hooks has no sources")
	}
	src := hooks.Sources[0]
	if src.Format != "html" {
		t.Errorf("Format = %q, want html", src.Format)
	}
	if src.Selector.Primary == "" {
		t.Error("Selector.Primary is empty")
	}
	if src.Selector.ExpectedContains == "" {
		t.Error("Selector.ExpectedContains is empty")
	}
}

func TestLoadSourceManifest_NotFound(t *testing.T) {
	_, err := capmon.LoadSourceManifest("testdata/does-not-exist.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadAllSourceManifests(t *testing.T) {
	dir := filepath.Join("testdata", "fixtures", "source-manifests")
	manifests, err := capmon.LoadAllSourceManifests(dir)
	if err != nil {
		t.Fatalf("LoadAllSourceManifests: %v", err)
	}
	if len(manifests) == 0 {
		t.Error("expected at least one manifest")
	}
	var found bool
	for _, m := range manifests {
		if m.Slug == "claude-code" {
			found = true
		}
	}
	if !found {
		t.Error("expected claude-code manifest in results")
	}
}

func TestLoadAllSourceManifests_MissingDir(t *testing.T) {
	_, err := capmon.LoadAllSourceManifests("testdata/does-not-exist/")
	if err == nil {
		t.Error("expected error for missing directory")
	}
}

func TestLoadAllSourceManifests_SkipsTemplate(t *testing.T) {
	dir := t.TempDir()
	// Write a real manifest and a template that should be skipped
	yamlContent := "schema_version: \"1\"\nslug: test-provider\n"
	if err := os.WriteFile(filepath.Join(dir, "test-provider.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "_template.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	manifests, err := capmon.LoadAllSourceManifests(dir)
	if err != nil {
		t.Fatalf("LoadAllSourceManifests: %v", err)
	}
	if len(manifests) != 1 {
		t.Errorf("expected 1 manifest (template skipped), got %d", len(manifests))
	}
}

func TestSourceEntry_HealingConfig_Absent(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `schema_version: "1"
slug: test-provider
content_types:
  skills:
    sources:
      - url: "https://example.com/skills"
        type: documentation
        format: md
        selector: {}
`
	path := filepath.Join(dir, "test-provider.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := capmon.LoadSourceManifest(path)
	if err != nil {
		t.Fatalf("LoadSourceManifest: %v", err)
	}
	src := m.ContentTypes["skills"].Sources[0]
	if src.Healing != nil {
		t.Errorf("Healing = %+v, want nil when block absent", src.Healing)
	}
	if !src.IsHealingEnabled() {
		t.Error("IsHealingEnabled() = false, want true when healing block absent")
	}
	got := src.EffectiveStrategies()
	if len(got) != len(capmon.DefaultHealingStrategies) {
		t.Errorf("EffectiveStrategies() len = %d, want %d", len(got), len(capmon.DefaultHealingStrategies))
	}
}

func TestSourceEntry_HealingConfig_EnabledFalse(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `schema_version: "1"
slug: test-provider
content_types:
  skills:
    sources:
      - url: "https://example.com/skills"
        type: documentation
        format: md
        selector: {}
        healing:
          enabled: false
`
	path := filepath.Join(dir, "test-provider.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := capmon.LoadSourceManifest(path)
	if err != nil {
		t.Fatalf("LoadSourceManifest: %v", err)
	}
	src := m.ContentTypes["skills"].Sources[0]
	if src.Healing == nil {
		t.Fatal("Healing = nil, want non-nil")
	}
	if src.IsHealingEnabled() {
		t.Error("IsHealingEnabled() = true, want false when enabled: false")
	}
}

func TestSourceEntry_HealingConfig_CustomStrategies(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `schema_version: "1"
slug: test-provider
content_types:
  skills:
    sources:
      - url: "https://example.com/skills"
        type: documentation
        format: md
        selector: {}
        healing:
          strategies:
            - redirect
            - variant
`
	path := filepath.Join(dir, "test-provider.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := capmon.LoadSourceManifest(path)
	if err != nil {
		t.Fatalf("LoadSourceManifest: %v", err)
	}
	src := m.ContentTypes["skills"].Sources[0]
	if !src.IsHealingEnabled() {
		t.Error("IsHealingEnabled() = false, want true when strategies set but enabled unset")
	}
	got := src.EffectiveStrategies()
	if len(got) != 2 || got[0] != "redirect" || got[1] != "variant" {
		t.Errorf("EffectiveStrategies() = %v, want [redirect variant]", got)
	}
}

func TestSourceEntry_HealingConfig_EmptyStrategiesUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `schema_version: "1"
slug: test-provider
content_types:
  skills:
    sources:
      - url: "https://example.com/skills"
        type: documentation
        format: md
        selector: {}
        healing:
          enabled: true
`
	path := filepath.Join(dir, "test-provider.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := capmon.LoadSourceManifest(path)
	if err != nil {
		t.Fatalf("LoadSourceManifest: %v", err)
	}
	src := m.ContentTypes["skills"].Sources[0]
	got := src.EffectiveStrategies()
	if len(got) != len(capmon.DefaultHealingStrategies) {
		t.Errorf("EffectiveStrategies() len = %d, want default len %d", len(got), len(capmon.DefaultHealingStrategies))
	}
}

func TestSourceEntry_EffectiveStrategies_ReturnsCopy(t *testing.T) {
	src := capmon.SourceEntry{
		Healing: &capmon.HealingConfig{
			Strategies: []string{"redirect", "variant"},
		},
	}
	got := src.EffectiveStrategies()
	got[0] = "mutated"
	got2 := src.EffectiveStrategies()
	if got2[0] != "redirect" {
		t.Errorf("EffectiveStrategies() returned aliased slice: after mutation got %q, want %q", got2[0], "redirect")
	}
}

func TestSourceManifest_DocsConventions_Absent(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `schema_version: "1"
slug: test-provider
content_types:
  skills:
    sources:
      - url: "https://example.com/skills"
        type: documentation
        format: md
        selector: {}
`
	path := filepath.Join(dir, "test-provider.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := capmon.LoadSourceManifest(path)
	if err != nil {
		t.Fatalf("LoadSourceManifest: %v", err)
	}
	if m.DocsConventions != nil {
		t.Errorf("DocsConventions = %+v, want nil when block absent", m.DocsConventions)
	}
}

func TestSourceManifest_DocsConventions_Present(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `schema_version: "1"
slug: test-provider
docs_conventions:
  query_param_required: "?internal"
  auth_gated_paths:
    - "/auth/sign-in"
    - "/login"
  retired_paths:
    - "/manual/hooks.md"
content_types:
  skills:
    sources:
      - url: "https://example.com/skills"
        type: documentation
        format: md
        selector: {}
`
	path := filepath.Join(dir, "test-provider.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := capmon.LoadSourceManifest(path)
	if err != nil {
		t.Fatalf("LoadSourceManifest: %v", err)
	}
	if m.DocsConventions == nil {
		t.Fatal("DocsConventions = nil, want non-nil")
	}
	if got := m.DocsConventions.QueryParamRequired; got != "?internal" {
		t.Errorf("QueryParamRequired = %q, want %q", got, "?internal")
	}
	wantAuth := []string{"/auth/sign-in", "/login"}
	if got := m.DocsConventions.AuthGatedPaths; len(got) != len(wantAuth) || got[0] != wantAuth[0] || got[1] != wantAuth[1] {
		t.Errorf("AuthGatedPaths = %v, want %v", got, wantAuth)
	}
	wantRetired := []string{"/manual/hooks.md"}
	if got := m.DocsConventions.RetiredPaths; len(got) != len(wantRetired) || got[0] != wantRetired[0] {
		t.Errorf("RetiredPaths = %v, want %v", got, wantRetired)
	}
}

// TestAmpManifest_DocsConventions_Populated pins the real amp.yaml to the
// values that motivated this bead's docs_conventions block. If amp.yaml
// regresses on these fields, the heal pipeline silently loses the
// auth-gate diagnostic and re-introduces the /manual/hooks/index.md
// variant-generation bug.
func TestAmpManifest_DocsConventions_Populated(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "provider-sources", "amp.yaml")
	m, err := capmon.LoadSourceManifest(path)
	if err != nil {
		t.Fatalf("LoadSourceManifest(amp.yaml): %v", err)
	}
	if m.DocsConventions == nil {
		t.Fatal("amp.yaml DocsConventions = nil, want populated block")
	}
	if m.DocsConventions.QueryParamRequired != "?internal" {
		t.Errorf("amp QueryParamRequired = %q, want %q",
			m.DocsConventions.QueryParamRequired, "?internal")
	}
	wantGate := map[string]bool{"/auth/sign-in": false, "/login": false}
	for _, g := range m.DocsConventions.AuthGatedPaths {
		if _, ok := wantGate[g]; ok {
			wantGate[g] = true
		}
	}
	for g, seen := range wantGate {
		if !seen {
			t.Errorf("amp AuthGatedPaths missing %q; got %v", g, m.DocsConventions.AuthGatedPaths)
		}
	}
	wantRetired := "/manual/hooks.md"
	found := false
	for _, p := range m.DocsConventions.RetiredPaths {
		if p == wantRetired {
			found = true
		}
	}
	if !found {
		t.Errorf("amp RetiredPaths missing %q; got %v", wantRetired, m.DocsConventions.RetiredPaths)
	}
}
