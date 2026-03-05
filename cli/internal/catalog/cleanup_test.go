package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func TestCleanupPromotedItems_RequiresNameMatch(t *testing.T) {
	// Note: not parallel — modifies GlobalContentDirOverride global
	root := t.TempDir()
	globalDir := t.TempDir()
	orig := GlobalContentDirOverride
	GlobalContentDirOverride = globalDir
	t.Cleanup(func() { GlobalContentDirOverride = orig })

	// Create shared skill with ID "uuid-123"
	sharedDir := filepath.Join(root, "skills", "shared-tool")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sharedDir, "SKILL.md"), "---\nname: shared-tool\n---\n")
	if err := metadata.Save(sharedDir, &metadata.Meta{
		ID:   "uuid-123",
		Name: "shared-tool",
		Type: "skill",
	}); err != nil {
		t.Fatal(err)
	}

	// Create library item with same ID but DIFFERENT name (ID collision)
	libDir := filepath.Join(globalDir, "skills", "different-name")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(libDir, "SKILL.md"), "---\nname: different-name\n---\n")
	if err := metadata.Save(libDir, &metadata.Meta{
		ID:   "uuid-123",
		Name: "different-name",
		Type: "skill",
	}); err != nil {
		t.Fatal(err)
	}

	cat, err := ScanWithGlobalAndRegistries(root, root, nil)
	if err != nil {
		t.Fatal(err)
	}

	cleaned, err := CleanupPromotedItems(cat)
	if err != nil {
		t.Fatal(err)
	}

	// Should NOT clean up (name mismatch despite same ID)
	if len(cleaned) != 0 {
		t.Errorf("expected 0 items cleaned (name mismatch), got %d", len(cleaned))
	}

	// Library item should still exist
	if _, err := os.Stat(libDir); err != nil {
		t.Error("library item should not have been deleted")
	}
}

func TestCleanupPromotedItems_RequiresTypeMatch(t *testing.T) {
	// Note: not parallel — modifies GlobalContentDirOverride global
	root := t.TempDir()
	globalDir := t.TempDir()
	orig := GlobalContentDirOverride
	GlobalContentDirOverride = globalDir
	t.Cleanup(func() { GlobalContentDirOverride = orig })

	// Create shared skill
	sharedDir := filepath.Join(root, "skills", "tool-name")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sharedDir, "SKILL.md"), "---\nname: tool-name\n---\n")
	if err := metadata.Save(sharedDir, &metadata.Meta{
		ID:   "uuid-456",
		Name: "tool-name",
		Type: "skill",
	}); err != nil {
		t.Fatal(err)
	}

	// Create library agent with same ID and name but different type
	libDir := filepath.Join(globalDir, "agents", "tool-name")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(libDir, "AGENT.md"), "---\nname: tool-name\n---\n")
	if err := metadata.Save(libDir, &metadata.Meta{
		ID:   "uuid-456",
		Name: "tool-name",
		Type: "agent",
	}); err != nil {
		t.Fatal(err)
	}

	cat, err := ScanWithGlobalAndRegistries(root, root, nil)
	if err != nil {
		t.Fatal(err)
	}

	cleaned, err := CleanupPromotedItems(cat)
	if err != nil {
		t.Fatal(err)
	}

	// Should NOT clean up (type mismatch)
	if len(cleaned) != 0 {
		t.Errorf("expected 0 items cleaned (type mismatch), got %d", len(cleaned))
	}

	if _, err := os.Stat(libDir); err != nil {
		t.Error("library item should not have been deleted")
	}
}

func TestCleanupPromotedItems_CleansExactMatch(t *testing.T) {
	// Note: not parallel — modifies GlobalContentDirOverride global
	root := t.TempDir()
	globalDir := t.TempDir()
	orig := GlobalContentDirOverride
	GlobalContentDirOverride = globalDir
	t.Cleanup(func() { GlobalContentDirOverride = orig })

	// Create shared skill
	sharedDir := filepath.Join(root, "skills", "promoted-tool")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sharedDir, "SKILL.md"), "---\nname: promoted-tool\n---\n")
	if err := metadata.Save(sharedDir, &metadata.Meta{
		ID:   "uuid-789",
		Name: "promoted-tool",
		Type: "skill",
	}); err != nil {
		t.Fatal(err)
	}

	// Create library item with matching ID, name, and type
	libDir := filepath.Join(globalDir, "skills", "promoted-tool")
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(libDir, "SKILL.md"), "---\nname: promoted-tool\n---\n")
	if err := metadata.Save(libDir, &metadata.Meta{
		ID:   "uuid-789",
		Name: "promoted-tool",
		Type: "skill",
	}); err != nil {
		t.Fatal(err)
	}

	cat, err := ScanWithGlobalAndRegistries(root, root, nil)
	if err != nil {
		t.Fatal(err)
	}

	cleaned, err := CleanupPromotedItems(cat)
	if err != nil {
		t.Fatal(err)
	}

	// SHOULD clean up (exact match)
	if len(cleaned) != 1 {
		t.Errorf("expected 1 item cleaned, got %d", len(cleaned))
	}

	// Library item should be deleted
	if _, err := os.Stat(libDir); err == nil {
		t.Error("library item should have been deleted")
	}
}
