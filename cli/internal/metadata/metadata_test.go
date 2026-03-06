package metadata

import (
	"os"
	"path/filepath"
	"strings"
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
		ID:      NewID(),
		Name:    "test-skill",
		Type:    "skills",
		Author:  "tester",
		Source:  "/some/path",
		AddedAt: &now,
		AddedBy: "test-user",
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

func TestMetaCreatedAt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	m := &Meta{
		ID:        NewID(),
		Name:      "test",
		CreatedAt: &now,
	}
	if err := Save(dir, m); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.CreatedAt == nil {
		t.Fatal("CreatedAt was not persisted")
	}
	if !loaded.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt: got %v, want %v", *loaded.CreatedAt, now)
	}
}

func TestMetaSourceHash(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const hash = "abc123def456"
	m := &Meta{
		ID:         NewID(),
		Name:       "test",
		SourceHash: hash,
	}
	if err := Save(dir, m); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.SourceHash != hash {
		t.Errorf("SourceHash: got %q, want %q", loaded.SourceHash, hash)
	}
}

func TestMetaHiddenField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	itemDir := filepath.Join(dir, "test-item")

	m := &Meta{
		ID:     NewID(),
		Name:   "test-skill",
		Hidden: true,
	}
	if err := Save(itemDir, m); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(itemDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !loaded.Hidden {
		t.Error("Hidden should be true after round-trip")
	}

	// Verify false (zero value) is omitted
	m2 := &Meta{ID: NewID(), Name: "visible"}
	if err := Save(filepath.Join(dir, "item2"), m2); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	data, _ := os.ReadFile(MetaPath(filepath.Join(dir, "item2")))
	if strings.Contains(string(data), "hidden") {
		t.Error("hidden field should be omitted when false")
	}
}
