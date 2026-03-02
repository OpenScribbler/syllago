package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestScaffold_CreatesExpectedDirectories(t *testing.T) {
	tmp := t.TempDir()
	err := Scaffold(tmp, "my-registry", "Our team rules")
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	for _, ct := range catalog.AllContentTypes() {
		path := filepath.Join(tmp, "my-registry", string(ct))
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			t.Errorf("expected directory %s to exist", path)
		}
		// Each directory should contain a .gitkeep
		gitkeep := filepath.Join(path, ".gitkeep")
		if _, statErr := os.Stat(gitkeep); os.IsNotExist(statErr) {
			t.Errorf("expected .gitkeep in %s", path)
		}
	}
}

func TestScaffold_CreatesRegistryYAML(t *testing.T) {
	tmp := t.TempDir()
	err := Scaffold(tmp, "test-reg", "A test registry")
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "test-reg", "registry.yaml"))
	if err != nil {
		t.Fatalf("reading registry.yaml: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "test-reg") {
		t.Error("registry.yaml should contain the registry name")
	}
	if !strings.Contains(content, "A test registry") {
		t.Error("registry.yaml should contain the description")
	}
	if !strings.Contains(content, "0.1.0") {
		t.Error("registry.yaml should contain the version")
	}

	// Verify it can be loaded by the existing manifest loader.
	manifest, loadErr := loadManifestFromDir(filepath.Join(tmp, "test-reg"))
	if loadErr != nil {
		t.Fatalf("loadManifestFromDir: %v", loadErr)
	}
	if manifest.Name != "test-reg" {
		t.Errorf("manifest name = %q, want %q", manifest.Name, "test-reg")
	}
}

func TestScaffold_DefaultDescription(t *testing.T) {
	tmp := t.TempDir()
	err := Scaffold(tmp, "team-rules", "")
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	manifest, loadErr := loadManifestFromDir(filepath.Join(tmp, "team-rules"))
	if loadErr != nil {
		t.Fatalf("loadManifestFromDir: %v", loadErr)
	}
	if manifest.Description != "team-rules registry" {
		t.Errorf("default description = %q, want %q", manifest.Description, "team-rules registry")
	}
}

func TestScaffold_ErrorsOnInvalidName(t *testing.T) {
	tmp := t.TempDir()
	cases := []string{"invalid name!", "has spaces", "with.dots", "../traversal"}
	for _, name := range cases {
		err := Scaffold(tmp, name, "")
		if err == nil {
			t.Errorf("Scaffold(%q) should return an error", name)
		}
	}
}

func TestScaffold_ErrorsIfDirectoryAlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "existing-reg"), 0755)
	err := Scaffold(tmp, "existing-reg", "")
	if err == nil {
		t.Error("Scaffold should error if directory already exists")
	}
}

func TestScaffold_CreatesREADME(t *testing.T) {
	tmp := t.TempDir()
	err := Scaffold(tmp, "readme-test", "Check the readme")
	if err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "readme-test", "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# readme-test") {
		t.Error("README should contain the registry name as heading")
	}
	if !strings.Contains(content, "Check the readme") {
		t.Error("README should contain the description")
	}
}
