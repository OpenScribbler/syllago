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
