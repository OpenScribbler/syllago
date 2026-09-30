package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func TestLoadResolvesRetiredProviderSlug(t *testing.T) {
	_, stderr := output.SetForTest(t)

	path := filepath.Join(t.TempDir(), "config.json")
	data := `{"providers": ["claude-code", "windsurf"], "provider_paths": {"windsurf": {"base_dir": "/custom"}}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	if len(cfg.Providers) != 2 || cfg.Providers[1] != "devin" {
		t.Errorf("Providers = %v, want [claude-code devin]", cfg.Providers)
	}
	if _, ok := cfg.ProviderPaths["devin"]; !ok {
		t.Errorf("ProviderPaths keys = %v, want devin", cfg.ProviderPaths)
	}
	if _, ok := cfg.ProviderPaths["windsurf"]; ok {
		t.Errorf("ProviderPaths still has windsurf key: %v", cfg.ProviderPaths)
	}
	if !strings.Contains(stderr.String(), `provider "windsurf" was renamed to "devin"`) {
		t.Errorf("expected deprecation warning, got stderr %q", stderr.String())
	}
}
