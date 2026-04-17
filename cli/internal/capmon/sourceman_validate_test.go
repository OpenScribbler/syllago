package capmon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestSourceManifest(t *testing.T, dir, provider, content string) string {
	t.Helper()
	path := filepath.Join(dir, provider+".yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidateSources_AllHaveSources(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestSourceManifest(t, dir, "test-provider", `schema_version: "1"
slug: test-provider
display_name: Test Provider
content_types:
  skills:
    sources:
      - url: "https://example.com/skills"
        type: documentation
        format: md
        selector: {}
`)
	err := ValidateSources(dir, "test-provider")
	if err != nil {
		t.Errorf("expected no error for valid manifest, got: %v", err)
	}
}

func TestValidateSources_MissingURIs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestSourceManifest(t, dir, "test-provider", `schema_version: "1"
slug: test-provider
display_name: Test Provider
content_types:
  skills:
    sources: []
`)
	err := ValidateSources(dir, "test-provider")
	if err == nil {
		t.Fatal("expected error for content type with no sources and no supported:false")
	}
}

func TestValidateSources_SupportedFalseSkipped(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestSourceManifest(t, dir, "test-provider", `schema_version: "1"
slug: test-provider
display_name: Test Provider
content_types:
  skills:
    supported: false
    sources: []
  agents:
    sources:
      - url: "https://example.com/agents"
        type: documentation
        format: md
        selector: {}
`)
	err := ValidateSources(dir, "test-provider")
	if err != nil {
		t.Errorf("expected no error when skills has supported:false, got: %v", err)
	}
}

func TestValidateSources_MissingManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	err := ValidateSources(dir, "nonexistent-provider")
	if err == nil {
		t.Fatal("expected error for missing manifest file")
	}
}

func TestValidateSources_KnownHealingStrategies(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestSourceManifest(t, dir, "test-provider", `schema_version: "1"
slug: test-provider
display_name: Test Provider
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
            - github-rename
            - variant
`)
	if err := ValidateSources(dir, "test-provider"); err != nil {
		t.Errorf("expected no error for known healing strategies, got: %v", err)
	}
}

func TestValidateSources_UnknownHealingStrategy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestSourceManifest(t, dir, "test-provider", `schema_version: "1"
slug: test-provider
display_name: Test Provider
content_types:
  skills:
    sources:
      - url: "https://example.com/skills"
        type: documentation
        format: md
        selector: {}
        healing:
          strategies:
            - archaeopteryx
`)
	err := ValidateSources(dir, "test-provider")
	if err == nil {
		t.Fatal("expected validation error for unknown healing strategy")
	}
	if !strings.Contains(err.Error(), "archaeopteryx") {
		t.Errorf("error should name the unknown strategy; got: %v", err)
	}
}

func TestValidateSources_HealingEnabledFalse_StrategyUnchecked(t *testing.T) {
	// enabled: false still leaves Strategies visible; we still validate them
	// because operators should be able to spot typos. But no strategies
	// listed with enabled:false must not error.
	t.Parallel()
	dir := t.TempDir()
	writeTestSourceManifest(t, dir, "test-provider", `schema_version: "1"
slug: test-provider
display_name: Test Provider
content_types:
  skills:
    sources:
      - url: "https://example.com/skills"
        type: documentation
        format: md
        selector: {}
        healing:
          enabled: false
`)
	if err := ValidateSources(dir, "test-provider"); err != nil {
		t.Errorf("expected no error for enabled: false with no strategies, got: %v", err)
	}
}
