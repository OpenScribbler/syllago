package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// TestBuildRegistryCards_SourceNameSurvivesManifestOverride is the regression
// test for the bug where a registry's display name (from registry.yaml) was
// the same field used as the operational identity for delete/sync. The card
// must keep the source/config name in sourceName even when registry.yaml
// declares a different display name.
//
// Real-world example: config has "OpenScribbler/syllago-meta-registry",
// registry.yaml says `name: syllago-meta-registry`. Card must display
// "syllago-meta-registry" but operate on "OpenScribbler/syllago-meta-registry".
func TestBuildRegistryCards_SourceNameSurvivesManifestOverride(t *testing.T) {
	t.Parallel()

	// Create a fake clone dir with a registry.yaml that overrides the name.
	cloneDir := t.TempDir()
	const yamlContent = "name: display-name-from-manifest\nversion: \"1.0\"\ndescription: test\n"
	if err := os.WriteFile(filepath.Join(cloneDir, "registry.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write registry.yaml: %v", err)
	}

	sources := []catalog.RegistrySource{
		{Name: "owner/full-config-identity", Path: cloneDir},
	}
	cat := &catalog.Catalog{Items: nil}

	cards := buildRegistryCards(sources, cat)
	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
	c := cards[0]

	// Display name MAY be the manifest override.
	if c.name != "display-name-from-manifest" {
		t.Errorf("expected display name from manifest, got %q", c.name)
	}
	// Operational identity MUST be the config source name.
	if c.sourceName != "owner/full-config-identity" {
		t.Errorf("sourceName must be config identity, got %q (name=%q)", c.sourceName, c.name)
	}
}

// TestBuildRegistryCards_SourceNameWhenNoManifest verifies sourceName is set
// correctly even when registry.yaml is missing — display name falls back to
// the config name and both fields end up equal, but sourceName is still set
// independently so any downstream override of name doesn't compromise identity.
func TestBuildRegistryCards_SourceNameWhenNoManifest(t *testing.T) {
	t.Parallel()
	cloneDir := t.TempDir() // no registry.yaml

	sources := []catalog.RegistrySource{
		{Name: "plain/registry", Path: cloneDir},
	}
	cards := buildRegistryCards(sources, &catalog.Catalog{})

	if len(cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(cards))
	}
	if cards[0].sourceName != "plain/registry" {
		t.Errorf("sourceName=%q, want %q", cards[0].sourceName, "plain/registry")
	}
	if cards[0].name != "plain/registry" {
		t.Errorf("name=%q, want fallback to source name %q", cards[0].name, "plain/registry")
	}
}
