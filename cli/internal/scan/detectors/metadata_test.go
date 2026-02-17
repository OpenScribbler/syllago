package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestProjectMetadataWithReadme(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "README.md"), []byte("# My Project\n\nA cool project for testing."), 0644)
	os.WriteFile(filepath.Join(tmp, "LICENSE"), []byte("MIT License\n\nCopyright..."), 0644)
	os.MkdirAll(filepath.Join(tmp, ".github", "workflows"), 0755)

	det := ProjectMetadata{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected metadata section")
	}
	pm, ok := sections[0].(model.ProjectMetadataSection)
	if !ok {
		t.Fatalf("expected ProjectMetadataSection, got %T", sections[0])
	}
	if pm.Description != "A cool project for testing." {
		t.Errorf("description = %q", pm.Description)
	}
	if pm.License != "MIT" {
		t.Errorf("license = %q, want MIT", pm.License)
	}
	if pm.CI != "GitHub Actions" {
		t.Errorf("ci = %q, want GitHub Actions", pm.CI)
	}
}

func TestProjectMetadataEmpty(t *testing.T) {
	t.Parallel()
	det := ProjectMetadata{}
	sections, err := det.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty dir, got %d", len(sections))
	}
}
