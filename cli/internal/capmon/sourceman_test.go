package capmon_test

import (
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
