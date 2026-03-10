package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/acme/my-tools.git", "acme/my-tools"},
		{"https://github.com/acme/my-tools", "acme/my-tools"},
		{"https://github.com/acme/my-tools/", "acme/my-tools"},
		{"git@github.com:acme/my-tools.git", "acme/my-tools"},
		{"git@github.com:acme/my-tools", "acme/my-tools"},
		{"git@github.com:acme/my_tools.git", "acme/my_tools"},
		{"https://example.com/my-tools.git", "my-tools"},
		{"https://example.com/my-tools", "my-tools"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := NameFromURL(tt.url); got != tt.want {
				t.Errorf("NameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestExpandAlias_KnownAliasTableIsEmpty(t *testing.T) {
	if len(KnownAliases) != 0 {
		t.Errorf("KnownAliases should be empty, got %d entries: %v", len(KnownAliases), KnownAliases)
	}
}

func TestExpandAlias_FullURL_NotExpanded(t *testing.T) {
	input := "https://github.com/acme/tools.git"
	url, expanded := ExpandAlias(input)
	if expanded {
		t.Fatal("expected expanded=false for full URL")
	}
	if url != input {
		t.Errorf("url = %q, want %q", url, input)
	}
}

func TestExpandAlias_UnknownShortName_NotExpanded(t *testing.T) {
	input := "some-random-name"
	url, expanded := ExpandAlias(input)
	if expanded {
		t.Fatal("expected expanded=false for unknown short name")
	}
	if url != input {
		t.Errorf("url = %q, want %q", url, input)
	}
}

func TestExpandAlias_SSHURL_NotExpanded(t *testing.T) {
	input := "git@github.com:acme/tools.git"
	url, expanded := ExpandAlias(input)
	if expanded {
		t.Fatal("expected expanded=false for SSH URL (contains ':')")
	}
	if url != input {
		t.Errorf("url = %q, want %q", url, input)
	}
}

func TestLoadManifest_Missing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir() // no registry.yaml
	m, err := loadManifestFromDir(dir)
	if err != nil {
		t.Fatalf("loadManifestFromDir: %v", err)
	}
	if m != nil {
		t.Errorf("expected nil manifest for missing file, got %+v", m)
	}
}

func TestLoadManifest_Valid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := "name: my-registry\ndescription: Test registry\nversion: \"1.0.0\"\n"
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	m, err := loadManifestFromDir(dir)
	if err != nil {
		t.Fatalf("loadManifestFromDir: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if m.Name != "my-registry" {
		t.Errorf("Name = %q, want %q", m.Name, "my-registry")
	}
	if m.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", m.Version, "1.0.0")
	}
	if m.Description != "Test registry" {
		t.Errorf("Description = %q, want %q", m.Description, "Test registry")
	}
}

func TestLoadManifest_InvalidYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(":\n  - bad: [yaml"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := loadManifestFromDir(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoadManifest_AllFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `name: full-registry
description: A full registry
maintainers:
  - alice
  - bob
version: "2.1.0"
min_syllago_version: "0.5.0"
`
	if err := os.WriteFile(filepath.Join(dir, "registry.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	m, err := loadManifestFromDir(dir)
	if err != nil {
		t.Fatalf("loadManifestFromDir: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manifest")
	}
	if len(m.Maintainers) != 2 {
		t.Errorf("Maintainers len = %d, want 2", len(m.Maintainers))
	}
	if m.MinSyllagoVersion != "0.5.0" {
		t.Errorf("MinSyllagoVersion = %q, want %q", m.MinSyllagoVersion, "0.5.0")
	}
}
