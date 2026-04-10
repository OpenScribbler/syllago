package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestLoadScanAsConfig_Valid(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := "scan-as:\n  - type: skills\n    path: Packs/\n  - type: agents\n    path: src/bmm/agents/\n"
	os.WriteFile(filepath.Join(root, ".syllago.yaml"), []byte(content), 0644)

	cfg, err := LoadScanAsConfig(root)
	if err != nil {
		t.Fatalf("LoadScanAsConfig error: %v", err)
	}
	if len(cfg.ScanAs) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(cfg.ScanAs))
	}
	if cfg.ScanAs[0].Type != catalog.Skills {
		t.Errorf("entry[0].Type = %v, want Skills", cfg.ScanAs[0].Type)
	}
	if cfg.ScanAs[0].Path != "Packs/" {
		t.Errorf("entry[0].Path = %q, want Packs/", cfg.ScanAs[0].Path)
	}
}

func TestLoadScanAsConfig_Missing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg, err := LoadScanAsConfig(root)
	if err != nil {
		t.Fatalf("LoadScanAsConfig should succeed on missing file: %v", err)
	}
	if len(cfg.ScanAs) != 0 {
		t.Errorf("expected empty config for missing file, got %d entries", len(cfg.ScanAs))
	}
}

func TestSaveScanAsConfig_RoundTrip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := &ScanAsConfig{
		ScanAs: []ScanAsEntry{
			{Type: catalog.Skills, Path: "Packs/"},
		},
	}
	if err := SaveScanAsConfig(root, cfg); err != nil {
		t.Fatalf("SaveScanAsConfig error: %v", err)
	}
	loaded, err := LoadScanAsConfig(root)
	if err != nil {
		t.Fatalf("LoadScanAsConfig after save error: %v", err)
	}
	if len(loaded.ScanAs) != 1 || loaded.ScanAs[0].Path != "Packs/" {
		t.Errorf("round-trip failed: %+v", loaded.ScanAs)
	}
}

func TestLoadScanAsConfig_InvalidType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	content := "scan-as:\n  - type: bogus\n    path: Packs/\n"
	os.WriteFile(filepath.Join(root, ".syllago.yaml"), []byte(content), 0644)
	_, err := LoadScanAsConfig(root)
	if err == nil {
		t.Error("expected error for invalid content type in .syllago.yaml")
	}
}

func TestIsValidContentType(t *testing.T) {
	t.Parallel()
	if !IsValidContentType(catalog.Skills) {
		t.Error("Skills should be valid")
	}
	if IsValidContentType("bogus") {
		t.Error("bogus should not be valid")
	}
}

func TestScanAsConfig_ToPathMap(t *testing.T) {
	t.Parallel()
	cfg := &ScanAsConfig{
		ScanAs: []ScanAsEntry{
			{Type: catalog.Skills, Path: "Packs/"},
			{Type: catalog.Agents, Path: "src/agents/"},
		},
	}
	m := cfg.ToPathMap()
	if m["Packs/"] != catalog.Skills {
		t.Errorf("Packs/ should map to Skills")
	}
	if m["src/agents/"] != catalog.Agents {
		t.Errorf("src/agents/ should map to Agents")
	}
}
