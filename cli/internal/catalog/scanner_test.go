package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

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

		cat, err := Scan(root)
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

		cat, err := Scan(root)
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

		cat, err := Scan(root)
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
			"foo.bar",          // dot (sjson path separator)
			"skill*",           // asterisk (sjson wildcard)
			"skill#hash",       // hash (sjson modifier)
			"skill|pipe",       // pipe (sjson alternative)
			"mcpServers.evil",  // path injection attempt
		}

		for _, name := range invalidNames {
			writeFile(t, filepath.Join(skillsDir, name, "SKILL.md"), "# Test")
		}

		// Also create a valid skill to verify it still gets discovered
		writeFile(t, filepath.Join(skillsDir, "valid-skill_123", "SKILL.md"), "# Valid")

		cat, err := Scan(root)
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

		cat, err := Scan(root)
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
