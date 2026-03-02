package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// makeItem is a test helper for building ContentItem values concisely.
func makeItem(name string, ct ContentType, local bool, registry string, builtin bool) ContentItem {
	item := ContentItem{Name: name, Type: ct, Local: local, Registry: registry}
	if builtin {
		item.Meta = &metadata.Meta{Tags: []string{"builtin"}}
	}
	return item
}

// writeFile is a test helper that creates a file (including parent dirs) with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestScan(t *testing.T) {
	t.Parallel()
	t.Run("discovers universal items with frontmatter", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()

		// Create a skill with SKILL.md frontmatter.
		writeFile(t, filepath.Join(root, "skills", "my-skill", "SKILL.md"),
			"---\nname: My Skill\ndescription: Does cool things\n---\nBody text.\n")

		// Create a rule (provider-specific) with a markdown file.
		writeFile(t, filepath.Join(root, "rules", "claude-code", "rule.md"),
			"# Rule\nThis rule does something.\n")

		cat, err := Scan(root, root)
		if err != nil {
			t.Fatalf("Scan returned error: %v", err)
		}

		// Check skill was discovered.
		skills := cat.ByType(Skills)
		if len(skills) != 1 {
			t.Fatalf("expected 1 skill, got %d", len(skills))
		}
		skill := skills[0]
		if skill.Name != "my-skill" {
			t.Errorf("skill Name = %q, want %q", skill.Name, "my-skill")
		}
		if skill.DisplayName != "My Skill" {
			t.Errorf("skill DisplayName = %q, want %q", skill.DisplayName, "My Skill")
		}
		if skill.Description != "Does cool things" {
			t.Errorf("skill Description = %q, want %q", skill.Description, "Does cool things")
		}
		if skill.Type != Skills {
			t.Errorf("skill Type = %q, want %q", skill.Type, Skills)
		}

		// Check rule was discovered.
		rules := cat.ByType(Rules)
		if len(rules) != 1 {
			t.Fatalf("expected 1 rule, got %d", len(rules))
		}
		rule := rules[0]
		if rule.Name != "rule.md" {
			t.Errorf("rule Name = %q, want %q", rule.Name, "rule.md")
		}
		if rule.Provider != "claude-code" {
			t.Errorf("rule Provider = %q, want %q", rule.Provider, "claude-code")
		}
		if rule.Type != Rules {
			t.Errorf("rule Type = %q, want %q", rule.Type, Rules)
		}
	})

	t.Run("empty type directory does not error", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()

		// Create an empty skills directory.
		if err := os.MkdirAll(filepath.Join(root, "skills"), 0755); err != nil {
			t.Fatal(err)
		}

		cat, err := Scan(root, root)
		if err != nil {
			t.Fatalf("Scan returned error: %v", err)
		}
		if len(cat.ByType(Skills)) != 0 {
			t.Errorf("expected 0 skills from empty dir, got %d", len(cat.ByType(Skills)))
		}
	})

	t.Run("missing type directories are skipped", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		// Don't create any type directories at all.

		cat, err := Scan(root, root)
		if err != nil {
			t.Fatalf("Scan returned error: %v", err)
		}
		if len(cat.Items) != 0 {
			t.Errorf("expected 0 items, got %d", len(cat.Items))
		}
		if cat.RepoRoot != root {
			t.Errorf("RepoRoot = %q, want %q", cat.RepoRoot, root)
		}
	})

	t.Run("rejects item names with sjson special characters", func(t *testing.T) {
		root := t.TempDir()

		skillsDir := filepath.Join(root, "skills")

		invalidNames := []string{
			"foo.bar",         // dot (sjson path separator)
			"skill*",          // asterisk (sjson wildcard)
			"skill#hash",      // hash (sjson modifier)
			"skill|pipe",      // pipe (sjson alternative)
			"mcpServers.evil", // path injection attempt
		}

		for _, name := range invalidNames {
			writeFile(t, filepath.Join(skillsDir, name, "SKILL.md"), "# Test")
		}

		// Also create a valid skill to verify it still gets discovered
		writeFile(t, filepath.Join(skillsDir, "valid-skill_123", "SKILL.md"), "# Valid")

		cat, err := Scan(root, root)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}

		// Only the valid skill should be discovered
		skills := cat.ByType(Skills)
		if len(skills) != 1 {
			var names []string
			for _, s := range skills {
				names = append(names, s.Name)
			}
			t.Errorf("expected 1 valid skill, got %d: %v", len(skills), names)
		}
		if len(skills) == 1 && skills[0].Name != "valid-skill_123" {
			t.Errorf("expected valid-skill_123, got %s", skills[0].Name)
		}
	})

	t.Run("IsValidItemName accepts valid names", func(t *testing.T) {
		valid := []string{"my-skill", "skill_v2", "abc123", "CamelCase", "a"}
		for _, name := range valid {
			if !IsValidItemName(name) {
				t.Errorf("IsValidItemName(%q) = false, want true", name)
			}
		}
	})

	t.Run("IsValidItemName rejects invalid names", func(t *testing.T) {
		invalid := []string{"foo.bar", "a*b", "x#y", "a|b", "", "a b", "a/b"}
		for _, name := range invalid {
			if IsValidItemName(name) {
				t.Errorf("IsValidItemName(%q) = true, want false", name)
			}
		}
	})

	t.Run("discovers multiple content types", func(t *testing.T) {
		root := t.TempDir()

		// Skill
		writeFile(t, filepath.Join(root, "skills", "skill-a", "SKILL.md"),
			"---\nname: Skill A\ndescription: First skill\n---\n")

		// Agent
		writeFile(t, filepath.Join(root, "agents", "agent-b", "AGENT.md"),
			"---\nname: Agent B\ndescription: An agent\n---\n")

		// Prompt
		writeFile(t, filepath.Join(root, "prompts", "prompt-c", "PROMPT.md"),
			"---\nname: Prompt C\ndescription: A prompt\n---\nPrompt body here.")

		// Rule (provider-specific)
		writeFile(t, filepath.Join(root, "rules", "gemini-cli", "setup.md"),
			"# Setup\nConfigure gemini.\n")

		cat, err := Scan(root, root)
		if err != nil {
			t.Fatalf("Scan returned error: %v", err)
		}

		counts := cat.CountByType()
		if counts[Skills] != 1 {
			t.Errorf("Skills count = %d, want 1", counts[Skills])
		}
		if counts[Agents] != 1 {
			t.Errorf("Agents count = %d, want 1", counts[Agents])
		}
		if counts[Prompts] != 1 {
			t.Errorf("Prompts count = %d, want 1", counts[Prompts])
		}
		if counts[Rules] != 1 {
			t.Errorf("Rules count = %d, want 1", counts[Rules])
		}

		// Verify prompt body was captured.
		prompts := cat.ByType(Prompts)
		if prompts[0].Body != "Prompt body here." {
			t.Errorf("Prompt Body = %q, want %q", prompts[0].Body, "Prompt body here.")
		}
	})
}

func TestApplyPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("local wins over shared", func(t *testing.T) {
		t.Parallel()
		cat := &Catalog{
			Items: []ContentItem{
				makeItem("my-skill", Skills, false, "", false), // shared
				makeItem("my-skill", Skills, true, "", false),  // local
			},
		}
		applyPrecedence(cat)
		if len(cat.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(cat.Items))
		}
		if !cat.Items[0].Local {
			t.Error("local item should win")
		}
		if len(cat.Overridden) != 1 {
			t.Fatalf("expected 1 overridden item, got %d", len(cat.Overridden))
		}
		if cat.Overridden[0].Local {
			t.Error("shared item should be overridden")
		}
	})

	t.Run("shared wins over registry", func(t *testing.T) {
		t.Parallel()
		cat := &Catalog{
			Items: []ContentItem{
				makeItem("my-skill", Skills, false, "team-reg", false), // registry
				makeItem("my-skill", Skills, false, "", false),         // shared
			},
		}
		applyPrecedence(cat)
		if len(cat.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(cat.Items))
		}
		if cat.Items[0].Registry != "" {
			t.Error("shared item should win over registry")
		}
		if len(cat.Overridden) != 1 || cat.Overridden[0].Registry != "team-reg" {
			t.Error("registry item should be overridden")
		}
	})

	t.Run("registry wins over builtin", func(t *testing.T) {
		t.Parallel()
		cat := &Catalog{
			Items: []ContentItem{
				makeItem("my-skill", Skills, false, "", true),          // built-in
				makeItem("my-skill", Skills, false, "team-reg", false), // registry
			},
		}
		applyPrecedence(cat)
		if len(cat.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(cat.Items))
		}
		if cat.Items[0].Registry != "team-reg" {
			t.Error("registry item should win over built-in")
		}
		if len(cat.Overridden) != 1 || !cat.Overridden[0].IsBuiltin() {
			t.Error("built-in item should be overridden")
		}
	})

	t.Run("different types are not deduplicated", func(t *testing.T) {
		t.Parallel()
		cat := &Catalog{
			Items: []ContentItem{
				makeItem("my-item", Skills, false, "", false),
				makeItem("my-item", Rules, false, "", false),
			},
		}
		applyPrecedence(cat)
		if len(cat.Items) != 2 {
			t.Errorf("expected 2 items (different types), got %d", len(cat.Items))
		}
		if len(cat.Overridden) != 0 {
			t.Errorf("expected 0 overridden items, got %d", len(cat.Overridden))
		}
	})

	t.Run("case-insensitive name matching", func(t *testing.T) {
		t.Parallel()
		cat := &Catalog{
			Items: []ContentItem{
				makeItem("My-Skill", Skills, false, "", false), // shared
				makeItem("my-skill", Skills, true, "", false),  // local wins
			},
		}
		applyPrecedence(cat)
		if len(cat.Items) != 1 {
			t.Fatalf("expected 1 item after case-insensitive dedup, got %d", len(cat.Items))
		}
		if !cat.Items[0].Local {
			t.Error("local item should win (case-insensitive match)")
		}
		if len(cat.Overridden) != 1 {
			t.Fatalf("expected 1 overridden item, got %d", len(cat.Overridden))
		}
	})

	t.Run("no duplicates leaves overridden empty", func(t *testing.T) {
		t.Parallel()
		cat := &Catalog{
			Items: []ContentItem{
				makeItem("skill-a", Skills, false, "", false),
				makeItem("skill-b", Skills, false, "", false),
			},
		}
		applyPrecedence(cat)
		if len(cat.Items) != 2 {
			t.Errorf("expected 2 items, got %d", len(cat.Items))
		}
		if len(cat.Overridden) != 0 {
			t.Errorf("expected 0 overridden items, got %d", len(cat.Overridden))
		}
	})
}

func TestOverridesFor(t *testing.T) {
	t.Parallel()

	t.Run("returns losers for given name and type", func(t *testing.T) {
		t.Parallel()
		cat := &Catalog{
			Items: []ContentItem{
				makeItem("my-skill", Skills, true, "", false), // local wins
			},
			Overridden: []ContentItem{
				makeItem("my-skill", Skills, false, "", false),         // shared lost
				makeItem("my-skill", Skills, false, "team-reg", false), // registry lost
			},
		}
		got := cat.OverridesFor("my-skill", Skills)
		if len(got) != 2 {
			t.Errorf("expected 2 overrides, got %d", len(got))
		}
	})

	t.Run("different type does not match", func(t *testing.T) {
		t.Parallel()
		cat := &Catalog{
			Overridden: []ContentItem{
				makeItem("my-item", Rules, false, "", false),
			},
		}
		got := cat.OverridesFor("my-item", Skills)
		if len(got) != 0 {
			t.Errorf("expected 0 overrides for different type, got %d", len(got))
		}
	})

	t.Run("case-insensitive lookup", func(t *testing.T) {
		t.Parallel()
		cat := &Catalog{
			Overridden: []ContentItem{
				makeItem("My-Skill", Skills, false, "", false),
			},
		}
		got := cat.OverridesFor("my-skill", Skills)
		if len(got) != 1 {
			t.Errorf("expected 1 override (case-insensitive), got %d", len(got))
		}
	})
}

func TestScanLocalRootSeparate(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	contentRoot := filepath.Join(projectRoot, "content")

	// Shared item under contentRoot
	writeFile(t, filepath.Join(contentRoot, "skills", "shared-skill", "SKILL.md"),
		"---\nname: Shared Skill\ndescription: A shared skill\n---\n")

	// Local item under projectRoot/local/ (NOT under contentRoot/local/)
	writeFile(t, filepath.Join(projectRoot, "local", "skills", "my-local-skill", "SKILL.md"),
		"---\nname: My Local Skill\ndescription: A local skill\n---\n")

	cat, err := Scan(contentRoot, projectRoot)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	skills := cat.ByType(Skills)
	if len(skills) != 2 {
		var names []string
		for _, s := range skills {
			names = append(names, s.Name)
		}
		t.Fatalf("expected 2 skills (1 shared + 1 local), got %d: %v", len(skills), names)
	}

	// Find the local item and verify it's marked Local
	var foundLocal bool
	for _, s := range skills {
		if s.Name == "my-local-skill" {
			if !s.Local {
				t.Error("my-local-skill should be marked Local=true")
			}
			foundLocal = true
		}
	}
	if !foundLocal {
		t.Error("local skill was not discovered")
	}
}

func TestGlobalContentDir_ContainsExpectedPath(t *testing.T) {
	dir := GlobalContentDir()
	if dir == "" {
		t.Skip("cannot determine home directory")
	}
	if !strings.HasSuffix(dir, filepath.Join(".syllago", "content")) {
		t.Errorf("GlobalContentDir() = %q, want suffix .syllago/content", dir)
	}
}

func TestScanWithGlobalAndRegistries_TagsProjectItems(t *testing.T) {
	// Create a project with a skill
	projectDir := t.TempDir()
	skillDir := filepath.Join(projectDir, "skills", "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# test-skill"), 0644)

	cat, err := ScanWithGlobalAndRegistries(projectDir, projectDir, nil)
	if err != nil {
		t.Fatalf("ScanWithGlobalAndRegistries: %v", err)
	}

	for _, item := range cat.Items {
		if item.Name == "test-skill" {
			if item.Source != "project" {
				t.Errorf("project item should have Source='project', got %q", item.Source)
			}
			return
		}
	}
	t.Error("test-skill not found in catalog")
}
