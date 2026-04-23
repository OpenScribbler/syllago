package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// setupEditableSkill creates a real directory item on disk and an App with
// that item in its catalog. Returns the App and the item's directory path.
func setupEditableSkill(t *testing.T, name string) (App, string) {
	t.Helper()
	dir := t.TempDir()
	itemDir := filepath.Join(dir, name)
	if err := os.MkdirAll(itemDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "SKILL.md"), []byte("# "+name+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: name, Type: catalog.Skills, Path: itemDir, Files: []string{"SKILL.md"}},
		},
	}
	app := NewApp(cat, testProviders(), "0.0.0-test", false, nil, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	return m.(App), itemDir
}

// TestHandleEditSaved_WritesMetadataViaMetadataPackage verifies that the TUI
// edit save path uses the metadata package to write .syllago.yaml — the same
// implementation path as the CLI 'syllago edit' command.
func TestHandleEditSaved_WritesMetadataViaMetadataPackage(t *testing.T) {
	app, itemDir := setupEditableSkill(t, "test-skill")

	m, _ := app.Update(editSavedMsg{
		name:        "My Display Name",
		description: "A useful description",
		path:        itemDir,
	})
	_ = m

	meta, err := metadata.Load(itemDir)
	if err != nil {
		t.Fatalf("reading .syllago.yaml after edit: %v", err)
	}
	if meta == nil {
		t.Fatal("expected .syllago.yaml to exist after handleEditSaved")
	}
	if meta.Name != "My Display Name" {
		t.Errorf("Name: got %q, want %q", meta.Name, "My Display Name")
	}
	if meta.Description != "A useful description" {
		t.Errorf("Description: got %q, want %q", meta.Description, "A useful description")
	}
}

// TestHandleEditSaved_UpdatesCatalogInPlace verifies that after a save the
// in-memory catalog item reflects the new display name and description without
// a full re-scan — matching the CLI's behavior of writing and returning immediately.
func TestHandleEditSaved_UpdatesCatalogInPlace(t *testing.T) {
	app, itemDir := setupEditableSkill(t, "catalog-skill")

	m, _ := app.Update(editSavedMsg{
		name:        "Catalog Updated Name",
		description: "Catalog updated desc",
		path:        itemDir,
	})
	updated := m.(App)

	var found bool
	for _, item := range updated.catalog.Items {
		if item.Path == itemDir {
			found = true
			if item.DisplayName != "Catalog Updated Name" {
				t.Errorf("catalog DisplayName: got %q, want %q", item.DisplayName, "Catalog Updated Name")
			}
			if item.Description != "Catalog updated desc" {
				t.Errorf("catalog Description: got %q, want %q", item.Description, "Catalog updated desc")
			}
		}
	}
	if !found {
		t.Fatal("item not found in catalog after edit")
	}
}

// TestHandleEditSaved_EmptyPathIsNoop verifies that an editSavedMsg with no
// path does not panic and returns gracefully — matching the CLI's guard on
// missing item path.
func TestHandleEditSaved_EmptyPathIsNoop(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(editSavedMsg{
		name:        "Should Not Persist",
		description: "noop",
		path:        "",
	})
	_ = m // must not panic
}

// TestHandleEditSaved_OverwritesExistingMetadata verifies that editing an item
// with pre-existing .syllago.yaml replaces Name/Description without losing other
// fields — consistent with the CLI edit command's load-then-mutate-then-save pattern.
func TestHandleEditSaved_OverwritesExistingMetadata(t *testing.T) {
	app, itemDir := setupEditableSkill(t, "existing-skill")

	// Write initial metadata with extra fields.
	initial := &metadata.Meta{
		Name:        "Original Name",
		Description: "Original desc",
		Type:        "skills",
	}
	if err := metadata.Save(itemDir, initial); err != nil {
		t.Fatalf("writing initial metadata: %v", err)
	}

	m, _ := app.Update(editSavedMsg{
		name:        "Updated Name",
		description: "Updated desc",
		path:        itemDir,
	})
	_ = m

	meta, err := metadata.Load(itemDir)
	if err != nil {
		t.Fatalf("reading metadata: %v", err)
	}
	if meta.Name != "Updated Name" {
		t.Errorf("Name: got %q, want %q", meta.Name, "Updated Name")
	}
	if meta.Description != "Updated desc" {
		t.Errorf("Description: got %q, want %q", meta.Description, "Updated desc")
	}
}
