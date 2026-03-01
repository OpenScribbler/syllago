package loadout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
)

func TestParse_Valid(t *testing.T) {
	t.Parallel()
	m, err := Parse("testdata/valid-loadout.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Kind != "loadout" {
		t.Errorf("kind: got %q, want loadout", m.Kind)
	}
	if m.Version != 1 {
		t.Errorf("version: got %d, want 1", m.Version)
	}
	if m.Provider != "claude-code" {
		t.Errorf("provider: got %q, want claude-code", m.Provider)
	}
	if m.Name != "test-loadout" {
		t.Errorf("name: got %q, want test-loadout", m.Name)
	}
	if len(m.Rules) != 2 {
		t.Errorf("rules count: got %d, want 2", len(m.Rules))
	}
	if len(m.Hooks) != 2 {
		t.Errorf("hooks count: got %d, want 2", len(m.Hooks))
	}
	if len(m.Skills) != 1 {
		t.Errorf("skills count: got %d, want 1", len(m.Skills))
	}
}

func TestParse_MissingKind(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	f := filepath.Join(tmp, "loadout.yaml")
	os.WriteFile(f, []byte("version: 1\nprovider: claude-code\nname: test\n"), 0644)

	_, err := Parse(f)
	if err == nil {
		t.Fatal("expected error for missing kind")
	}
}

func TestParse_WrongKind(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	f := filepath.Join(tmp, "loadout.yaml")
	os.WriteFile(f, []byte("kind: rules\nversion: 1\nprovider: claude-code\nname: test\n"), 0644)

	_, err := Parse(f)
	if err == nil {
		t.Fatal("expected error for wrong kind")
	}
}

func TestParse_MissingProvider(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	f := filepath.Join(tmp, "loadout.yaml")
	os.WriteFile(f, []byte("kind: loadout\nversion: 1\nname: test\n"), 0644)

	_, err := Parse(f)
	if err == nil {
		t.Fatal("expected error for missing provider")
	}
}

func TestParse_EmptySections(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	f := filepath.Join(tmp, "loadout.yaml")
	os.WriteFile(f, []byte("kind: loadout\nversion: 1\nprovider: claude-code\nname: empty\n"), 0644)

	m, err := Parse(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.ItemCount() != 0 {
		t.Errorf("expected 0 items, got %d", m.ItemCount())
	}
}

func TestItemCount(t *testing.T) {
	t.Parallel()
	m := &Manifest{
		Rules: []string{"a", "b", "c"},
		Hooks: []string{"d", "e"},
	}
	if m.ItemCount() != 5 {
		t.Errorf("expected 5, got %d", m.ItemCount())
	}
}

func TestRefsByType(t *testing.T) {
	t.Parallel()
	m := &Manifest{
		Rules:  []string{"rule-a"},
		Skills: []string{"skill-a", "skill-b"},
	}
	refs := m.RefsByType()
	if len(refs) != 2 {
		t.Errorf("expected 2 types, got %d", len(refs))
	}
	if len(refs[catalog.Rules]) != 1 {
		t.Errorf("expected 1 rule ref, got %d", len(refs[catalog.Rules]))
	}
	if len(refs[catalog.Skills]) != 2 {
		t.Errorf("expected 2 skill refs, got %d", len(refs[catalog.Skills]))
	}
	// Empty sections should not appear
	if _, ok := refs[catalog.Hooks]; ok {
		t.Error("hooks should not be in refs when empty")
	}
}
