package add

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// --- DiscoverFromRegistry (0% coverage) ---
//
// DiscoverFromRegistry exercises two scan flavors:
//   - Directory-walk: registry has no registry.yaml; scanRoot finds content
//     under <type>/<item>/ subdirs and ci.Path is the item directory.
//   - Index-based: registry.yaml lists items with explicit paths; ci.Path
//     points at the primary file directly.
// Both flavors are tested below.

func TestDiscoverFromRegistry_DirectoryWalk_NewItem(t *testing.T) {
	t.Parallel()
	cloneDir := t.TempDir()
	globalDir := t.TempDir()

	// Universal type (skills) lives at <root>/skills/<name>/SKILL.md
	skillDir := filepath.Join(cloneDir, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: My Skill\ndescription: Test\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := DiscoverFromRegistry("acme/tools", cloneDir, globalDir)
	if err != nil {
		t.Fatalf("DiscoverFromRegistry: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	got := items[0]
	if got.Name != "my-skill" {
		t.Errorf("Name = %q, want %q", got.Name, "my-skill")
	}
	if got.Type != catalog.Skills {
		t.Errorf("Type = %v, want Skills", got.Type)
	}
	if got.Status != StatusNew {
		t.Errorf("Status = %v, want StatusNew", got.Status)
	}
	// Directory-walk path: SourceDir is the item directory; Path is the primary file.
	if got.SourceDir != skillDir {
		t.Errorf("SourceDir = %q, want %q", got.SourceDir, skillDir)
	}
	if filepath.Base(got.Path) != "SKILL.md" {
		t.Errorf("Path = %q, want it to end in SKILL.md", got.Path)
	}
}

func TestDiscoverFromRegistry_IndexBased_NewItem(t *testing.T) {
	t.Parallel()
	cloneDir := t.TempDir()
	globalDir := t.TempDir()

	// Index-based: registry.yaml lists explicit item paths.
	skillDir := filepath.Join(cloneDir, "skills", "indexed-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: Indexed\ndescription: Indexed skill\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cloneDir, "registry.yaml"), []byte(`name: acme-registry
items:
  - name: indexed-skill
    type: skills
    provider: ""
    path: skills/indexed-skill/SKILL.md
`), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := DiscoverFromRegistry("acme/tools", cloneDir, globalDir)
	if err != nil {
		t.Fatalf("DiscoverFromRegistry: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	got := items[0]
	if got.Name != "indexed-skill" {
		t.Errorf("Name = %q, want %q", got.Name, "indexed-skill")
	}
	// Index-based path: ci.Path is already the primary file, parent != cloneDir,
	// so SourceDir is the parent dir.
	if got.SourceDir != skillDir {
		t.Errorf("SourceDir = %q, want %q", got.SourceDir, skillDir)
	}
	if filepath.Base(got.Path) != "SKILL.md" {
		t.Errorf("Path = %q, want SKILL.md leaf", got.Path)
	}
}

func TestDiscoverFromRegistry_AlreadyInLibrary(t *testing.T) {
	t.Parallel()
	cloneDir := t.TempDir()
	globalDir := t.TempDir()

	content := []byte("---\nname: My Skill\ndescription: Test\n---\nbody")

	// Source in registry
	skillDir := filepath.Join(cloneDir, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), content, 0644); err != nil {
		t.Fatal(err)
	}

	// Same content in library with matching hash
	libSkillDir := filepath.Join(globalDir, "skills", "my-skill")
	if err := os.MkdirAll(libSkillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(libSkillDir, "SKILL.md"), content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Save(libSkillDir, &metadata.Meta{
		ID:         "lib-id",
		Name:       "my-skill",
		SourceHash: sourceHash(content),
	}); err != nil {
		t.Fatal(err)
	}

	items, err := DiscoverFromRegistry("acme/tools", cloneDir, globalDir)
	if err != nil {
		t.Fatalf("DiscoverFromRegistry: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Status != StatusInLibrary {
		t.Errorf("Status = %v, want StatusInLibrary", items[0].Status)
	}
}

func TestDiscoverFromRegistry_OutdatedItem(t *testing.T) {
	t.Parallel()
	cloneDir := t.TempDir()
	globalDir := t.TempDir()

	// Source in registry has new content
	skillDir := filepath.Join(cloneDir, "skills", "my-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: New\ndescription: Updated\n---\nnew body"), 0644); err != nil {
		t.Fatal(err)
	}

	// Library has same name but different stored hash
	libSkillDir := filepath.Join(globalDir, "skills", "my-skill")
	if err := os.MkdirAll(libSkillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Save(libSkillDir, &metadata.Meta{
		ID:         "lib-id",
		Name:       "my-skill",
		SourceHash: "sha256:stalehash",
	}); err != nil {
		t.Fatal(err)
	}

	items, err := DiscoverFromRegistry("acme/tools", cloneDir, globalDir)
	if err != nil {
		t.Fatalf("DiscoverFromRegistry: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Status != StatusOutdated {
		t.Errorf("Status = %v, want StatusOutdated", items[0].Status)
	}
}

func TestDiscoverFromRegistry_EmptyRegistry(t *testing.T) {
	t.Parallel()
	cloneDir := t.TempDir()
	globalDir := t.TempDir()

	items, err := DiscoverFromRegistry("acme/empty", cloneDir, globalDir)
	if err != nil {
		t.Fatalf("DiscoverFromRegistry: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0 from empty registry", len(items))
	}
}

func TestDiscoverFromRegistry_BuildIndexError(t *testing.T) {
	t.Parallel()
	cloneDir := t.TempDir()

	// Make globalDir's skills entry a regular file so os.ReadDir on the type
	// dir fails — exercises the "building library index" error path.
	globalDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(globalDir, "skills"), []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := DiscoverFromRegistry("acme/tools", cloneDir, globalDir)
	if err == nil {
		t.Fatal("expected error from BuildLibraryIndex, got nil")
	}
}

// --- stripAnalyzerMetadata (42.9% coverage — covers happy path) ---

func TestStripAnalyzerMetadata_ZerosAnalyzerFields(t *testing.T) {
	t.Parallel()
	destDir := t.TempDir()

	// Pre-seed metadata with analyzer fields populated (as if a malicious
	// source package set them to influence tier badges).
	original := &metadata.Meta{
		ID:              "test",
		Name:            "my-rule",
		Confidence:      95,
		DetectionSource: "filename",
		DetectionMethod: "exact",
		SourceHash:      "sha256:abc",
	}
	if err := metadata.Save(destDir, original); err != nil {
		t.Fatalf("metadata.Save: %v", err)
	}

	stripAnalyzerMetadata(destDir)

	got, err := metadata.Load(destDir)
	if err != nil {
		t.Fatalf("metadata.Load: %v", err)
	}
	if got == nil {
		t.Fatal("metadata.Load returned nil after strip")
	}
	if got.Confidence != 0 {
		t.Errorf("Confidence = %v, want 0", got.Confidence)
	}
	if got.DetectionSource != "" {
		t.Errorf("DetectionSource = %q, want empty", got.DetectionSource)
	}
	if got.DetectionMethod != "" {
		t.Errorf("DetectionMethod = %q, want empty", got.DetectionMethod)
	}
	// Non-analyzer fields preserved.
	if got.Name != "my-rule" {
		t.Errorf("Name = %q, want preserved %q", got.Name, "my-rule")
	}
	if got.SourceHash != "sha256:abc" {
		t.Errorf("SourceHash = %q, want preserved", got.SourceHash)
	}
}

func TestStripAnalyzerMetadata_NoMetadataIsNoop(t *testing.T) {
	t.Parallel()
	destDir := t.TempDir()

	// No .syllago.yaml present — function should return without error or panic.
	stripAnalyzerMetadata(destDir)

	// Verify: no file was created.
	if _, err := os.Stat(filepath.Join(destDir, ".syllago.yaml")); !os.IsNotExist(err) {
		t.Errorf("expected no .syllago.yaml created, got err = %v", err)
	}
}
