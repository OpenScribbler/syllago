package loadout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// --- BuildManifest tests ---

func TestBuildManifest_Basic(t *testing.T) {
	t.Parallel()
	m := BuildManifest("claude-code", "my-loadout", "A test loadout", nil)

	if m.Kind != "loadout" {
		t.Errorf("Kind = %q, want %q", m.Kind, "loadout")
	}
	if m.Version != 1 {
		t.Errorf("Version = %d, want 1", m.Version)
	}
	if m.Provider != "claude-code" {
		t.Errorf("Provider = %q, want %q", m.Provider, "claude-code")
	}
	if m.Name != "my-loadout" {
		t.Errorf("Name = %q, want %q", m.Name, "my-loadout")
	}
	if m.Description != "A test loadout" {
		t.Errorf("Description = %q, want %q", m.Description, "A test loadout")
	}
}

func TestBuildManifest_AllItems(t *testing.T) {
	t.Parallel()
	items := map[catalog.ContentType][]ItemRef{
		catalog.Rules:    {{Name: "rule-a"}, {Name: "rule-b"}},
		catalog.Hooks:    {{Name: "hook-one"}},
		catalog.Skills:   {{Name: "skill-x"}, {Name: "skill-y"}, {Name: "skill-z"}},
		catalog.Agents:   {{Name: "agent-1"}},
		catalog.MCP:      {{Name: "mcp-server"}},
		catalog.Commands: {{Name: "cmd-foo"}, {Name: "cmd-bar"}},
	}
	m := BuildManifest("cursor", "full-loadout", "All types", items)

	if len(m.Rules) != 2 || m.Rules[0].Name != "rule-a" || m.Rules[1].Name != "rule-b" {
		t.Errorf("Rules = %v, want [rule-a rule-b]", m.Rules)
	}
	if len(m.Hooks) != 1 || m.Hooks[0].Name != "hook-one" {
		t.Errorf("Hooks = %v, want [hook-one]", m.Hooks)
	}
	if len(m.Skills) != 3 {
		t.Errorf("Skills = %v, want 3 entries", m.Skills)
	}
	if len(m.Agents) != 1 || m.Agents[0].Name != "agent-1" {
		t.Errorf("Agents = %v, want [agent-1]", m.Agents)
	}
	if len(m.MCP) != 1 || m.MCP[0].Name != "mcp-server" {
		t.Errorf("MCP = %v, want [mcp-server]", m.MCP)
	}
	if len(m.Commands) != 2 {
		t.Errorf("Commands = %v, want 2 entries", m.Commands)
	}
}

func TestBuildManifest_EmptyItems(t *testing.T) {
	t.Parallel()
	// Empty slices in the map should produce nil fields (yaml omitempty drops them).
	items := map[catalog.ContentType][]ItemRef{
		catalog.Rules:  {},
		catalog.Skills: {},
	}
	m := BuildManifest("claude-code", "empty-loadout", "No items", items)

	if m.Rules != nil {
		t.Errorf("Rules = %v, want nil for empty slice input", m.Rules)
	}
	if m.Skills != nil {
		t.Errorf("Skills = %v, want nil for empty slice input", m.Skills)
	}
	if m.Kind != "loadout" {
		t.Errorf("Kind = %q, want loadout", m.Kind)
	}
}

func TestBuildManifest_NilItems(t *testing.T) {
	t.Parallel()
	m := BuildManifest("claude-code", "nil-loadout", "Nil map", nil)

	if m == nil {
		t.Fatal("BuildManifest returned nil")
	}
	if m.Rules != nil {
		t.Errorf("Rules = %v, want nil", m.Rules)
	}
	if m.Hooks != nil {
		t.Errorf("Hooks = %v, want nil", m.Hooks)
	}
	if m.Skills != nil {
		t.Errorf("Skills = %v, want nil", m.Skills)
	}
	if m.Agents != nil {
		t.Errorf("Agents = %v, want nil", m.Agents)
	}
	if m.MCP != nil {
		t.Errorf("MCP = %v, want nil", m.MCP)
	}
	if m.Commands != nil {
		t.Errorf("Commands = %v, want nil", m.Commands)
	}
}

func TestBuildManifestFromNames(t *testing.T) {
	t.Parallel()
	items := map[catalog.ContentType][]string{
		catalog.Rules: {"rule-a", "rule-b"},
	}
	m := BuildManifestFromNames("claude-code", "test", "desc", items)

	if len(m.Rules) != 2 || m.Rules[0].Name != "rule-a" || m.Rules[1].Name != "rule-b" {
		t.Errorf("Rules = %v, want [rule-a rule-b]", m.Rules)
	}
}

// --- WriteManifest tests ---

func TestWriteManifest_CreatesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	m := BuildManifest("claude-code", "my-loadout", "Test", nil)

	path, err := WriteManifest(m, dir)
	if err != nil {
		t.Fatalf("WriteManifest returned error: %v", err)
	}

	expected := filepath.Join(dir, "my-loadout", "loadout.yaml")
	if path != expected {
		t.Errorf("returned path = %q, want %q", path, expected)
	}

	if _, err := os.Stat(expected); err != nil {
		t.Errorf("file not found at expected path: %v", err)
	}
}

func TestWriteManifest_CreatesDir(t *testing.T) {
	t.Parallel()
	// Pass a destDir that doesn't exist yet — WriteManifest must create it.
	base := t.TempDir()
	destDir := filepath.Join(base, "nested", "path")

	m := BuildManifest("cursor", "my-loadout", "Test", nil)
	_, err := WriteManifest(m, destDir)
	if err != nil {
		t.Fatalf("WriteManifest returned error: %v", err)
	}

	expected := filepath.Join(destDir, "my-loadout", "loadout.yaml")
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("file not created at %q: %v", expected, err)
	}
}

func TestWriteManifest_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	items := map[catalog.ContentType][]ItemRef{
		catalog.Rules:  {{Name: "rule-a"}},
		catalog.Skills: {{Name: "skill-x"}, {Name: "skill-y"}},
	}
	m := BuildManifest("claude-code", "roundtrip-loadout", "Round trip test", items)

	path, err := WriteManifest(m, dir)
	if err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	parsed, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if parsed.Kind != m.Kind {
		t.Errorf("Kind: got %q, want %q", parsed.Kind, m.Kind)
	}
	if parsed.Version != m.Version {
		t.Errorf("Version: got %d, want %d", parsed.Version, m.Version)
	}
	if parsed.Provider != m.Provider {
		t.Errorf("Provider: got %q, want %q", parsed.Provider, m.Provider)
	}
	if parsed.Name != m.Name {
		t.Errorf("Name: got %q, want %q", parsed.Name, m.Name)
	}
	if parsed.Description != m.Description {
		t.Errorf("Description: got %q, want %q", parsed.Description, m.Description)
	}
	if len(parsed.Rules) != len(m.Rules) {
		t.Errorf("Rules len: got %d, want %d", len(parsed.Rules), len(m.Rules))
	}
	if len(parsed.Skills) != len(m.Skills) {
		t.Errorf("Skills len: got %d, want %d", len(parsed.Skills), len(m.Skills))
	}
}

func TestWriteManifest_UnwritablePath(t *testing.T) {
	t.Parallel()
	base := t.TempDir()

	// Make base dir read-only so WriteManifest cannot create subdirs inside it.
	if err := os.Chmod(base, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(base, 0o755) })

	m := BuildManifest("claude-code", "blocked", "Should fail", nil)
	_, err := WriteManifest(m, base)
	if err == nil {
		t.Error("expected error writing to unwritable path, got nil")
	}
}

func TestItemRef_UnmarshalYAML_String(t *testing.T) {
	t.Parallel()
	// Test that ItemRef can unmarshal from a plain string in loadout.yaml
	yamlContent := `kind: loadout
version: 1
name: test
description: test
rules:
  - my-rule
  - other-rule
`
	dir := t.TempDir()
	path := filepath.Join(dir, "loadout.yaml")
	os.WriteFile(path, []byte(yamlContent), 0644)

	m, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Rules) != 2 {
		t.Fatalf("Rules len = %d, want 2", len(m.Rules))
	}
	if m.Rules[0].Name != "my-rule" {
		t.Errorf("Rules[0].Name = %q, want %q", m.Rules[0].Name, "my-rule")
	}
	if m.Rules[1].Name != "other-rule" {
		t.Errorf("Rules[1].Name = %q, want %q", m.Rules[1].Name, "other-rule")
	}
}

func TestItemRef_UnmarshalYAML_Struct(t *testing.T) {
	t.Parallel()
	// Test that ItemRef can unmarshal from a full struct with ID
	yamlContent := `kind: loadout
version: 1
name: test
description: test
rules:
  - name: my-rule
    id: abc-123
  - name: other-rule
`
	dir := t.TempDir()
	path := filepath.Join(dir, "loadout.yaml")
	os.WriteFile(path, []byte(yamlContent), 0644)

	m, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Rules) != 2 {
		t.Fatalf("Rules len = %d, want 2", len(m.Rules))
	}
	if m.Rules[0].Name != "my-rule" || m.Rules[0].ID != "abc-123" {
		t.Errorf("Rules[0] = %+v, want {Name:my-rule ID:abc-123}", m.Rules[0])
	}
	if m.Rules[1].Name != "other-rule" || m.Rules[1].ID != "" {
		t.Errorf("Rules[1] = %+v, want {Name:other-rule ID:}", m.Rules[1])
	}
}
