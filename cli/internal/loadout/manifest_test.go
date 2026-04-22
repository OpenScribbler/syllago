package loadout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
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

	// Provider is now optional — missing provider should succeed
	m, err := Parse(f)
	if err != nil {
		t.Fatalf("unexpected error for missing provider: %v", err)
	}
	if m.Provider != "" {
		t.Errorf("expected empty provider, got %q", m.Provider)
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
		Rules: []ItemRef{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		Hooks: []ItemRef{{Name: "d"}, {Name: "e"}},
	}
	if m.ItemCount() != 5 {
		t.Errorf("expected 5, got %d", m.ItemCount())
	}
}

func TestParse_MissingName(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	f := filepath.Join(tmp, "loadout.yaml")
	os.WriteFile(f, []byte("kind: loadout\nversion: 1\nprovider: claude-code\n"), 0644)

	_, err := Parse(f)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParse_WrongVersion(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	f := filepath.Join(tmp, "loadout.yaml")
	os.WriteFile(f, []byte("kind: loadout\nversion: 99\nprovider: claude-code\nname: test\n"), 0644)

	_, err := Parse(f)
	if err == nil {
		t.Fatal("expected error for wrong version")
	}
}

func TestParse_AllSectionsPopulated(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	f := filepath.Join(tmp, "loadout.yaml")
	content := `kind: loadout
version: 1
provider: claude-code
name: full-loadout
description: all sections
rules:
  - rule-a
hooks:
  - hook-a
skills:
  - skill-a
agents:
  - agent-a
mcp:
  - mcp-a
commands:
  - cmd-a
`
	os.WriteFile(f, []byte(content), 0644)

	m, err := Parse(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.ItemCount() != 6 {
		t.Errorf("expected 6 items, got %d", m.ItemCount())
	}
	refs := m.RefsByType()
	if len(refs) != 6 {
		t.Errorf("expected 6 content types, got %d", len(refs))
	}
}

func TestParse_NonexistentFile(t *testing.T) {
	t.Parallel()
	_, err := Parse("/nonexistent/path/loadout.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	f := filepath.Join(tmp, "loadout.yaml")
	os.WriteFile(f, []byte("{{invalid yaml"), 0644)

	_, err := Parse(f)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParse_InvalidName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		yaml string
	}{
		{"path traversal", "kind: loadout\nversion: 1\nname: ../../evil\n"},
		{"leading dash", "kind: loadout\nversion: 1\nname: -bad\n"},
		{"dots", "kind: loadout\nversion: 1\nname: foo.bar\n"},
		{"spaces", "kind: loadout\nversion: 1\nname: has spaces\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			f := filepath.Join(tmp, "loadout.yaml")
			os.WriteFile(f, []byte(tc.yaml), 0644)
			_, err := Parse(f)
			if err == nil {
				t.Fatalf("expected error for name in %q", tc.name)
			}
		})
	}
}

func TestEffectiveProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest Manifest
		want     []string
	}{
		{
			name:     "providers[] wins over provider",
			manifest: Manifest{Provider: "claude-code", Providers: []string{"gemini-cli", "codex"}},
			want:     []string{"gemini-cli", "codex"},
		},
		{
			name:     "legacy provider field",
			manifest: Manifest{Provider: "claude-code"},
			want:     []string{"claude-code"},
		},
		{
			name:     "multi-provider only",
			manifest: Manifest{Providers: []string{"claude-code", "gemini-cli", "codex", "pi"}},
			want:     []string{"claude-code", "gemini-cli", "codex", "pi"},
		},
		{
			name:     "neither field set returns nil",
			manifest: Manifest{},
			want:     nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.manifest.EffectiveProviders()
			if len(got) != len(tc.want) {
				t.Fatalf("EffectiveProviders() len: got %d, want %d (got %v)", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("EffectiveProviders()[%d]: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestParse_ProvidersField(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	f := filepath.Join(tmp, "loadout.yaml")
	os.WriteFile(f, []byte(`kind: loadout
version: 1
name: multi-provider-starter
description: starter for many providers
providers:
  - claude-code
  - gemini-cli
  - codex
  - pi
skills:
  - code-review
`), 0644)

	m, err := Parse(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Providers) != 4 {
		t.Fatalf("providers count: got %d, want 4", len(m.Providers))
	}
	if m.Provider != "" {
		t.Errorf("provider field should be empty when providers[] is used, got %q", m.Provider)
	}
	if m.Providers[0] != "claude-code" || m.Providers[3] != "pi" {
		t.Errorf("providers: got %v", m.Providers)
	}
	if len(m.Skills) != 1 {
		t.Errorf("skills count: got %d, want 1", len(m.Skills))
	}
}

func TestRefsByType(t *testing.T) {
	t.Parallel()
	m := &Manifest{
		Rules:  []ItemRef{{Name: "rule-a"}},
		Skills: []ItemRef{{Name: "skill-a"}, {Name: "skill-b"}},
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
