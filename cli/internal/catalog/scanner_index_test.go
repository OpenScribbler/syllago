package catalog

import (
	"path/filepath"
	"testing"
)

// TestScanFromIndex_Skills verifies that a registry.yaml with items causes
// scanRoot to use the index rather than walking directories, and that skill
// metadata (DisplayName, Description) is extracted from SKILL.md frontmatter.
func TestScanFromIndex_Skills(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Create a native skill directory structure.
	skillDir := filepath.Join(root, "skills", "my-skill")
	writeFile(t, filepath.Join(skillDir, "SKILL.md"),
		"---\nname: My Skill\ndescription: Does cool things\n---\nBody text.\n")

	// Write registry.yaml with an items list pointing at the skill.
	writeFile(t, filepath.Join(root, "registry.yaml"), `name: test-registry
items:
  - name: my-skill
    type: skills
    provider: ""
    path: skills/my-skill
`)

	cat := &Catalog{}
	if err := scanRoot(cat, root, false); err != nil {
		t.Fatalf("scanRoot returned error: %v", err)
	}

	skills := cat.ByType(Skills)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	s := skills[0]
	if s.Name != "my-skill" {
		t.Errorf("Name = %q, want %q", s.Name, "my-skill")
	}
	if s.DisplayName != "My Skill" {
		t.Errorf("DisplayName = %q, want %q", s.DisplayName, "My Skill")
	}
	if s.Description != "Does cool things" {
		t.Errorf("Description = %q, want %q", s.Description, "Does cool things")
	}
	if s.Type != Skills {
		t.Errorf("Type = %q, want %q", s.Type, Skills)
	}
	if len(s.Files) == 0 {
		t.Error("Files should be non-empty for a directory item")
	}
}

// TestScanFromIndex_SingleFileAgent verifies that a single-file agent (a .md
// file rather than a directory) is correctly parsed from the index.
func TestScanFromIndex_SingleFileAgent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Single-file agent — not a directory.
	agentPath := filepath.Join(root, "agents", "my-agent.md")
	writeFile(t, agentPath,
		"---\nname: My Agent\ndescription: An autonomous agent\n---\nAgent body.\n")

	writeFile(t, filepath.Join(root, "registry.yaml"), `name: test-registry
items:
  - name: my-agent
    type: agents
    provider: ""
    path: agents/my-agent.md
`)

	cat := &Catalog{}
	if err := scanRoot(cat, root, false); err != nil {
		t.Fatalf("scanRoot returned error: %v", err)
	}

	agents := cat.ByType(Agents)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	a := agents[0]
	if a.Name != "my-agent" {
		t.Errorf("Name = %q, want %q", a.Name, "my-agent")
	}
	if a.DisplayName != "My Agent" {
		t.Errorf("DisplayName = %q, want %q", a.DisplayName, "My Agent")
	}
	if a.Description != "An autonomous agent" {
		t.Errorf("Description = %q, want %q", a.Description, "An autonomous agent")
	}
	// Single-file items get Files = [basename].
	if len(a.Files) != 1 || a.Files[0] != "my-agent.md" {
		t.Errorf("Files = %v, want [my-agent.md]", a.Files)
	}
}

// TestScanFromIndex_MissingPath verifies that an index entry whose path does
// not exist on disk produces a warning (not an error) and is skipped.
func TestScanFromIndex_MissingPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// No files on disk — path in index doesn't exist.
	writeFile(t, filepath.Join(root, "registry.yaml"), `name: test-registry
items:
  - name: ghost-skill
    type: skills
    provider: ""
    path: skills/ghost-skill
`)

	cat := &Catalog{}
	if err := scanRoot(cat, root, false); err != nil {
		t.Fatalf("scanRoot returned error: %v", err)
	}

	if len(cat.Items) != 0 {
		t.Errorf("expected 0 items (missing path skipped), got %d", len(cat.Items))
	}
	if len(cat.Warnings) == 0 {
		t.Error("expected at least one warning for missing path, got none")
	}
}

// TestScanFromIndex_FallbackToNativeLayout verifies that a registry.yaml
// without an items list causes scanRoot to fall back to the native directory
// walk rather than using index-based scanning.
func TestScanFromIndex_FallbackToNativeLayout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// registry.yaml with no items — should trigger fallback to native walk.
	writeFile(t, filepath.Join(root, "registry.yaml"), "name: test-registry\n")

	// Create a skill in the native layout.
	writeFile(t, filepath.Join(root, "skills", "native-skill", "SKILL.md"),
		"---\nname: Native Skill\ndescription: Found by walk\n---\n")

	cat := &Catalog{}
	if err := scanRoot(cat, root, false); err != nil {
		t.Fatalf("scanRoot returned error: %v", err)
	}

	skills := cat.ByType(Skills)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill via native walk, got %d", len(skills))
	}
	if skills[0].Name != "native-skill" {
		t.Errorf("Name = %q, want %q", skills[0].Name, "native-skill")
	}
}

// TestScanFromIndex_LibraryFlag verifies that the local flag is forwarded
// correctly to ContentItem.Library.
func TestScanFromIndex_LibraryFlag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	skillDir := filepath.Join(root, "skills", "lib-skill")
	writeFile(t, filepath.Join(skillDir, "SKILL.md"), "# lib-skill\n")

	writeFile(t, filepath.Join(root, "registry.yaml"), `name: test-registry
items:
  - name: lib-skill
    type: skills
    provider: ""
    path: skills/lib-skill
`)

	cat := &Catalog{}
	// Pass local=true to simulate a library scan.
	if err := scanRoot(cat, root, true); err != nil {
		t.Fatalf("scanRoot returned error: %v", err)
	}

	skills := cat.ByType(Skills)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if !skills[0].Library {
		t.Error("Library = false, want true when local=true")
	}
}

// TestScanFromIndex_ProviderSpecificRule verifies that provider-specific items
// (rules with a provider field) are scanned correctly from the index.
func TestScanFromIndex_ProviderSpecificRule(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	ruleDir := filepath.Join(root, "rules", "claude-code", "my-rule")
	writeFile(t, filepath.Join(ruleDir, "rule.md"), "# Rule\nThis rule enforces style.\n")

	writeFile(t, filepath.Join(root, "registry.yaml"), `name: test-registry
items:
  - name: my-rule
    type: rules
    provider: claude-code
    path: rules/claude-code/my-rule
`)

	cat := &Catalog{}
	if err := scanRoot(cat, root, false); err != nil {
		t.Fatalf("scanRoot returned error: %v", err)
	}

	rules := cat.ByType(Rules)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	r := rules[0]
	if r.Name != "my-rule" {
		t.Errorf("Name = %q, want %q", r.Name, "my-rule")
	}
	if r.Provider != "claude-code" {
		t.Errorf("Provider = %q, want %q", r.Provider, "claude-code")
	}
	if r.Description == "" {
		t.Error("Description should be extracted from rule.md")
	}
}

// TestScanFromIndex_NoRegistryYaml verifies that directories without a
// registry.yaml still use the native layout walk (no regression).
func TestScanFromIndex_NoRegistryYaml(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// No registry.yaml at all — pure native layout.
	writeFile(t, filepath.Join(root, "skills", "plain-skill", "SKILL.md"),
		"---\nname: Plain Skill\ndescription: Via walk\n---\n")

	cat := &Catalog{}
	if err := scanRoot(cat, root, false); err != nil {
		t.Fatalf("scanRoot returned error: %v", err)
	}

	skills := cat.ByType(Skills)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "plain-skill" {
		t.Errorf("Name = %q, want %q", skills[0].Name, "plain-skill")
	}
}

// TestScanFromIndex_MultipleItems verifies that multiple items of different
// types are all picked up from a single index.
func TestScanFromIndex_MultipleItems(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	writeFile(t, filepath.Join(root, "skills", "skill-a", "SKILL.md"),
		"---\nname: Skill A\ndescription: First\n---\n")
	writeFile(t, filepath.Join(root, "agents", "agent-b", "AGENT.md"),
		"---\nname: Agent B\ndescription: Second\n---\n")

	writeFile(t, filepath.Join(root, "registry.yaml"), `name: test-registry
items:
  - name: skill-a
    type: skills
    provider: ""
    path: skills/skill-a
  - name: agent-b
    type: agents
    provider: ""
    path: agents/agent-b
`)

	cat := &Catalog{}
	if err := scanRoot(cat, root, false); err != nil {
		t.Fatalf("scanRoot returned error: %v", err)
	}

	if len(cat.ByType(Skills)) != 1 {
		t.Errorf("expected 1 skill, got %d", len(cat.ByType(Skills)))
	}
	if len(cat.ByType(Agents)) != 1 {
		t.Errorf("expected 1 agent, got %d", len(cat.ByType(Agents)))
	}
	if len(cat.Items) != 2 {
		t.Errorf("expected 2 total items, got %d", len(cat.Items))
	}
}

// TestScanFromIndex_EmptyRegistryYamlNoItems verifies a registry.yaml that
// parses successfully but has zero items triggers the native walk (not the index path).
func TestScanFromIndex_EmptyRegistryYamlNoItems(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// registry.yaml with explicit empty items list.
	writeFile(t, filepath.Join(root, "registry.yaml"), "name: test-registry\nitems: []\n")

	// Native layout skill — should be found via walk fallback.
	writeFile(t, filepath.Join(root, "skills", "walk-skill", "SKILL.md"),
		"---\nname: Walk Skill\ndescription: By walk\n---\n")

	cat := &Catalog{}
	if err := scanRoot(cat, root, false); err != nil {
		t.Fatalf("scanRoot returned error: %v", err)
	}

	skills := cat.ByType(Skills)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill via native walk, got %d", len(skills))
	}
	if skills[0].Name != "walk-skill" {
		t.Errorf("Name = %q, want %q", skills[0].Name, "walk-skill")
	}
}

// TestScanRegistriesOnly_WithIndex exercises ScanRegistriesOnly using a
// registry that has a manifest index, verifying Registry field is tagged.
func TestScanRegistriesOnly_WithIndex(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	skillDir := filepath.Join(root, "skills", "reg-skill")
	writeFile(t, filepath.Join(skillDir, "SKILL.md"),
		"---\nname: Registry Skill\ndescription: From registry index\n---\n")

	writeFile(t, filepath.Join(root, "registry.yaml"), `name: acme-registry
items:
  - name: reg-skill
    type: skills
    provider: ""
    path: skills/reg-skill
`)

	sources := []RegistrySource{{Name: "acme/tools", Path: root}}
	cat, err := ScanRegistriesOnly(sources)
	if err != nil {
		t.Fatalf("ScanRegistriesOnly error: %v", err)
	}

	skills := cat.ByType(Skills)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Registry != "acme/tools" {
		t.Errorf("Registry = %q, want %q", skills[0].Registry, "acme/tools")
	}
}
