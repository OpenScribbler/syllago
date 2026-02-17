package metadata

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewID(t *testing.T) {
	t.Parallel()
	id := NewID()
	if len(id) != 36 {
		t.Errorf("expected 36-char UUID, got %d: %s", len(id), id)
	}
	// Check uniqueness
	id2 := NewID()
	if id == id2 {
		t.Error("two generated IDs should not be equal")
	}
}

func TestSaveAndLoad(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	itemDir := filepath.Join(dir, "test-item")

	now := time.Now().Truncate(time.Second)
	m := &Meta{
		ID:         NewID(),
		Name:       "test-skill",
		Type:       "skills",
		Author:     "tester",
		Source:     "/some/path",
		ImportedAt: &now,
		ImportedBy: "test-user",
	}

	if err := Save(itemDir, m); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(MetaPath(itemDir)); err != nil {
		t.Fatalf("metadata file not created: %v", err)
	}

	loaded, err := Load(itemDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if loaded.ID != m.ID {
		t.Errorf("ID mismatch: got %s, want %s", loaded.ID, m.ID)
	}
	if loaded.Name != m.Name {
		t.Errorf("Name mismatch: got %s, want %s", loaded.Name, m.Name)
	}
	if loaded.Type != m.Type {
		t.Errorf("Type mismatch: got %s, want %s", loaded.Type, m.Type)
	}
	if loaded.Author != m.Author {
		t.Errorf("Author mismatch: got %s, want %s", loaded.Author, m.Author)
	}
}

func TestLoadNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m, err := Load(dir)
	if err != nil {
		t.Fatalf("Load should return nil error for missing file, got: %v", err)
	}
	if m != nil {
		t.Fatal("Load should return nil Meta for missing file")
	}
}

func TestProviderMetadata(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	m := &Meta{
		ID:   NewID(),
		Name: "hook.json",
		Type: "hooks",
	}

	if err := SaveProvider(dir, "hook.json", m); err != nil {
		t.Fatalf("SaveProvider failed: %v", err)
	}

	loaded, err := LoadProvider(dir, "hook.json")
	if err != nil {
		t.Fatalf("LoadProvider failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadProvider returned nil")
	}
	if loaded.ID != m.ID {
		t.Errorf("ID mismatch: got %s, want %s", loaded.ID, m.ID)
	}
}
