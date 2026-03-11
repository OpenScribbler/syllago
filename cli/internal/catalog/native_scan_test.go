package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanNativeContent_SyllagoStructure(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte("name: test"), 0644)
	result := ScanNativeContent(dir)
	if !result.HasSyllagoStructure {
		t.Error("expected HasSyllagoStructure=true when registry.yaml present")
	}
}

func TestScanNativeContent_SyllagoContentDirs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "rules"), 0755)
	result := ScanNativeContent(dir)
	if !result.HasSyllagoStructure {
		t.Error("expected HasSyllagoStructure=true when content dir present")
	}
}

func TestScanNativeContent_CursorRules(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".cursorrules"), []byte("# rules"), 0644)
	result := ScanNativeContent(dir)
	if result.HasSyllagoStructure {
		t.Error("should not be syllago structure")
	}
	if len(result.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(result.Providers))
	}
	if result.Providers[0].ProviderSlug != "cursor" {
		t.Errorf("expected cursor, got %s", result.Providers[0].ProviderSlug)
	}
}

func TestScanNativeContent_MultiProvider(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".cursorrules"), []byte("# rules"), 0644)
	os.WriteFile(filepath.Join(dir, ".windsurfrules"), []byte("# rules"), 0644)
	result := ScanNativeContent(dir)
	if len(result.Providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(result.Providers))
	}
}

func TestScanNativeContent_ClaudeCodeDir(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, ".claude", "commands")
	os.MkdirAll(cmdDir, 0755)
	os.WriteFile(filepath.Join(cmdDir, "deploy.md"), []byte("# deploy"), 0644)
	result := ScanNativeContent(dir)
	if result.HasSyllagoStructure {
		t.Error("should not be syllago structure")
	}
	if len(result.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(result.Providers))
	}
	p := result.Providers[0]
	if p.ProviderSlug != "claude-code" {
		t.Errorf("expected claude-code, got %s", p.ProviderSlug)
	}
	files := p.ByType["commands"]
	if len(files) != 1 {
		t.Fatalf("expected 1 command file, got %d", len(files))
	}
}

func TestScanNativeContent_Empty(t *testing.T) {
	dir := t.TempDir()
	result := ScanNativeContent(dir)
	if result.HasSyllagoStructure || len(result.Providers) != 0 {
		t.Error("expected empty result for empty directory")
	}
}
