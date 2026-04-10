package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/tidwall/gjson"
)

// makeItem is a test helper for building ContentItem values concisely.
func makeItem(name string, ct ContentType, library bool, registry string, builtin bool) ContentItem {
	item := ContentItem{Name: name, Type: ct, Library: library, Registry: registry}
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
		valid := []string{"my-skill", "skill_v2", "abc123", "CamelCase", "a", "my-loadout", "_leading", strings.Repeat("x", 100)}
		for _, name := range valid {
			if !IsValidItemName(name) {
				t.Errorf("IsValidItemName(%q) = false, want true", name)
			}
		}
	})

	t.Run("IsValidItemName rejects invalid names", func(t *testing.T) {
		invalid := []string{
			"foo.bar", "a*b", "x#y", "a|b", "", "a b", "a/b",
			"../../.ssh/foo", "../evil", ".hidden", "-leading-dash",
			strings.Repeat("x", 101),
		}
		for _, name := range invalid {
			if IsValidItemName(name) {
				t.Errorf("IsValidItemName(%q) = true, want false", name)
			}
		}
	})

	t.Run("ValidateUserName returns empty for valid names", func(t *testing.T) {
		valid := []string{"my-loadout", "skill_v2", "abc123", "CamelCase", "a"}
		for _, name := range valid {
			if msg := ValidateUserName(name); msg != "" {
				t.Errorf("ValidateUserName(%q) = %q, want empty", name, msg)
			}
		}
	})

	t.Run("ValidateUserName returns specific errors", func(t *testing.T) {
		cases := []struct {
			name string
			want string
		}{
			{"", "name is required"},
			{strings.Repeat("x", 101), "name must be 100 characters or fewer"},
			{"-leading", "name must not start with a dash"},
			{"../evil", "name may only contain letters, numbers, hyphens, and underscores"},
			{"../../.ssh/foo", "name may only contain letters, numbers, hyphens, and underscores"},
			{".hidden", "name may only contain letters, numbers, hyphens, and underscores"},
			{"has spaces", "name may only contain letters, numbers, hyphens, and underscores"},
		}
		for _, tc := range cases {
			got := ValidateUserName(tc.name)
			if got != tc.want {
				t.Errorf("ValidateUserName(%q) = %q, want %q", tc.name, got, tc.want)
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
		if counts[Rules] != 1 {
			t.Errorf("Rules count = %d, want 1", counts[Rules])
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
		if !cat.Items[0].Library {
			t.Error("library item should win")
		}
		if len(cat.Overridden) != 1 {
			t.Fatalf("expected 1 overridden item, got %d", len(cat.Overridden))
		}
		if cat.Overridden[0].Library {
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
		if !cat.Items[0].Library {
			t.Error("library item should win (case-insensitive match)")
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

func TestScanIgnoresLocalDir(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	contentRoot := filepath.Join(projectRoot, "content")

	// Shared item under contentRoot
	writeFile(t, filepath.Join(contentRoot, "skills", "shared-skill", "SKILL.md"),
		"---\nname: Shared Skill\ndescription: A shared skill\n---\n")

	// A local/ directory that used to be scanned — should now be ignored
	writeFile(t, filepath.Join(projectRoot, "local", "skills", "my-local-skill", "SKILL.md"),
		"---\nname: My Local Skill\ndescription: A local skill\n---\n")

	cat, err := Scan(contentRoot, projectRoot)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	skills := cat.ByType(Skills)
	if len(skills) != 1 {
		var names []string
		for _, s := range skills {
			names = append(names, s.Name)
		}
		t.Fatalf("expected 1 skill (local/ is no longer scanned), got %d: %v", len(skills), names)
	}
	if skills[0].Name != "shared-skill" {
		t.Errorf("expected shared-skill, got %q", skills[0].Name)
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

func TestGlobalContentDirOverride(t *testing.T) {
	tmp := t.TempDir()

	original := GlobalContentDirOverride
	GlobalContentDirOverride = tmp
	t.Cleanup(func() { GlobalContentDirOverride = original })

	got := GlobalContentDir()
	if got != tmp {
		t.Errorf("GlobalContentDir() = %q, want %q", got, tmp)
	}
}

func TestScanWithGlobalTagsSource(t *testing.T) {
	tmp := t.TempDir()

	original := GlobalContentDirOverride
	GlobalContentDirOverride = tmp
	t.Cleanup(func() { GlobalContentDirOverride = original })

	// Create a skill under the global content dir override
	skillDir := filepath.Join(tmp, "skills", "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Use empty project dirs so the only discovered item is the global one
	emptyProject := t.TempDir()
	cat, err := ScanWithGlobalAndRegistries(emptyProject, emptyProject, nil)
	if err != nil {
		t.Fatalf("ScanWithGlobalAndRegistries: %v", err)
	}

	for _, item := range cat.Items {
		if item.Name == "test-skill" {
			if item.Source != "global" {
				t.Errorf("global item Source = %q, want %q", item.Source, "global")
			}
			return
		}
	}
	t.Error("test-skill not found in catalog — global content dir was not scanned")
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

func TestValidateRegistryPath(t *testing.T) {
	t.Parallel()

	t.Run("accepts path within registry root", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		sub := filepath.Join(root, "skills", "my-skill")
		os.MkdirAll(sub, 0755)

		if err := validateRegistryPath(sub, root); err != nil {
			t.Errorf("expected no error for path within root, got: %v", err)
		}
	})

	t.Run("accepts registry root itself", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()

		if err := validateRegistryPath(root, root); err != nil {
			t.Errorf("expected no error for root itself, got: %v", err)
		}
	})

	t.Run("rejects symlink escaping registry", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		outside := t.TempDir()

		// Create a symlink inside root that points outside
		link := filepath.Join(root, "evil-link")
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("cannot create symlinks: %v", err)
		}

		err := validateRegistryPath(link, root)
		if err == nil {
			t.Error("expected error for symlink escaping registry, got nil")
		}
		if err != nil && !strings.Contains(err.Error(), "escapes registry boundary") {
			t.Errorf("expected 'escapes registry boundary' error, got: %v", err)
		}
	})

	t.Run("rejects nonexistent path", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()

		err := validateRegistryPath(filepath.Join(root, "does-not-exist"), root)
		if err == nil {
			t.Error("expected error for nonexistent path, got nil")
		}
	})
}

func TestScanSkipsSymlinkEscapingRegistry(t *testing.T) {
	t.Parallel()

	t.Run("universal scan skips symlinked item directory", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		outside := t.TempDir()

		// Create a legitimate skill
		writeFile(t, filepath.Join(root, "skills", "legit-skill", "SKILL.md"),
			"---\nname: Legit\ndescription: A legit skill\n---\n")

		// Create an outside skill directory
		writeFile(t, filepath.Join(outside, "SKILL.md"),
			"---\nname: Evil\ndescription: Escaped skill\n---\n")

		// Symlink inside skills/ pointing outside.
		// os.ReadDir reports symlinks with IsDir()=false, so scanUniversal
		// skips them at the entry filter. This test verifies that the
		// symlinked item is NOT included in results regardless of mechanism.
		link := filepath.Join(root, "skills", "escaped-skill")
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("cannot create symlinks: %v", err)
		}

		cat, err := Scan(root, root)
		if err != nil {
			t.Fatalf("Scan returned error: %v", err)
		}

		skills := cat.ByType(Skills)
		if len(skills) != 1 {
			var names []string
			for _, s := range skills {
				names = append(names, s.Name)
			}
			t.Fatalf("expected 1 skill (escaped should be skipped), got %d: %v", len(skills), names)
		}
		if skills[0].Name != "legit-skill" {
			t.Errorf("expected legit-skill, got %q", skills[0].Name)
		}
	})

	t.Run("provider-specific scan skips symlinked item", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		outside := t.TempDir()

		// Create a legitimate rule
		writeFile(t, filepath.Join(root, "rules", "claude-code", "legit-rule.md"),
			"# Legit Rule\nThis is fine.\n")

		// Create an outside file
		writeFile(t, filepath.Join(outside, "evil.md"),
			"# Evil Rule\nThis escaped.\n")

		// Symlink a file inside the provider dir pointing outside
		link := filepath.Join(root, "rules", "claude-code", "escaped-rule.md")
		if err := os.Symlink(filepath.Join(outside, "evil.md"), link); err != nil {
			t.Skipf("cannot create symlinks: %v", err)
		}

		cat, err := Scan(root, root)
		if err != nil {
			t.Fatalf("Scan returned error: %v", err)
		}

		rules := cat.ByType(Rules)
		if len(rules) != 1 {
			var names []string
			for _, r := range rules {
				names = append(names, r.Name)
			}
			t.Fatalf("expected 1 rule (escaped should be skipped), got %d: %v", len(rules), names)
		}
		if rules[0].Name != "legit-rule.md" {
			t.Errorf("expected legit-rule.md, got %q", rules[0].Name)
		}
	})

	t.Run("index scan skips symlinked item", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		outside := t.TempDir()

		// Create a legit skill referenced by the index
		writeFile(t, filepath.Join(root, "content", "legit-skill", "SKILL.md"),
			"---\nname: Legit\ndescription: A legit skill\n---\n")

		// Create an outside skill
		writeFile(t, filepath.Join(outside, "SKILL.md"),
			"---\nname: Evil\ndescription: Escaped\n---\n")

		// Symlink inside root pointing outside
		link := filepath.Join(root, "content", "evil-skill")
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("cannot create symlinks: %v", err)
		}

		// Write a registry.yaml index referencing both
		writeFile(t, filepath.Join(root, "registry.yaml"),
			"items:\n"+
				"  - name: legit-skill\n"+
				"    type: skills\n"+
				"    path: content/legit-skill\n"+
				"  - name: evil-skill\n"+
				"    type: skills\n"+
				"    path: content/evil-skill\n")

		cat, err := Scan(root, root)
		if err != nil {
			t.Fatalf("Scan returned error: %v", err)
		}

		skills := cat.ByType(Skills)
		if len(skills) != 1 {
			var names []string
			for _, s := range skills {
				names = append(names, s.Name)
			}
			t.Fatalf("expected 1 skill (escaped should be skipped), got %d: %v", len(skills), names)
		}
		if skills[0].Name != "legit-skill" {
			t.Errorf("expected legit-skill, got %q", skills[0].Name)
		}

		// Verify a warning was emitted
		found := false
		for _, w := range cat.Warnings {
			if strings.Contains(w, "evil-skill") && strings.Contains(w, "escapes registry boundary") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected warning about evil-skill, warnings: %v", cat.Warnings)
		}
	})
}

func TestIsValidRegistryName(t *testing.T) {
	valid := []string{"acme/my-tools", "my-tools", "acme/my_tools-v2", "user123/repo-name"}
	for _, name := range valid {
		if !IsValidRegistryName(name) {
			t.Errorf("IsValidRegistryName(%q) = false, want true", name)
		}
	}
	invalid := []string{"", "a/b/c", "a b", "a.b", "a*b"}
	for _, name := range invalid {
		if IsValidRegistryName(name) {
			t.Errorf("IsValidRegistryName(%q) = true, want false", name)
		}
	}
}

func TestScanMCP_NestedConfigExplodes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "mcp", "my-config", "config.json"), `{
		"mcpServers": {
			"server-a": {"command": "npx", "args": ["-y", "@example/server-a"]},
			"server-b": {"url": "https://mcp.example.com/v1"}
		}
	}`)

	cat, err := Scan(root, root)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	mcps := cat.ByType(MCP)
	if len(mcps) != 2 {
		t.Fatalf("expected 2 MCP items, got %d", len(mcps))
	}

	// Build a map for order-independent assertions.
	byName := make(map[string]ContentItem)
	for _, m := range mcps {
		byName[m.Name] = m
	}

	a, ok := byName["server-a"]
	if !ok {
		t.Fatal("missing MCP item 'server-a'")
	}
	if a.ServerKey != "server-a" {
		t.Errorf("server-a ServerKey = %q, want %q", a.ServerKey, "server-a")
	}
	if a.Description != "npx @example/server-a" {
		t.Errorf("server-a Description = %q, want %q", a.Description, "npx @example/server-a")
	}
	// Path should point to the parent directory, not the server key.
	if !strings.HasSuffix(a.Path, filepath.Join("mcp", "my-config")) {
		t.Errorf("server-a Path = %q, expected to end with mcp/my-config", a.Path)
	}

	b, ok := byName["server-b"]
	if !ok {
		t.Fatal("missing MCP item 'server-b'")
	}
	if b.ServerKey != "server-b" {
		t.Errorf("server-b ServerKey = %q, want %q", b.ServerKey, "server-b")
	}
	if b.Description != "https://mcp.example.com/v1" {
		t.Errorf("server-b Description = %q, want %q", b.Description, "https://mcp.example.com/v1")
	}
}

func TestScanMCP_NestedSingleServer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "mcp", "single", "config.json"), `{
		"mcpServers": {
			"filesystem": {"command": "npx", "args": ["-y", "@mcp/server-filesystem"]}
		}
	}`)

	cat, err := Scan(root, root)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	mcps := cat.ByType(MCP)
	if len(mcps) != 1 {
		t.Fatalf("expected 1 MCP item, got %d", len(mcps))
	}
	if mcps[0].Name != "filesystem" {
		t.Errorf("Name = %q, want %q", mcps[0].Name, "filesystem")
	}
	if mcps[0].ServerKey != "filesystem" {
		t.Errorf("ServerKey = %q, want %q", mcps[0].ServerKey, "filesystem")
	}
}

func TestScanMCP_FlatConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "mcp", "my-server", "config.json"), `{
		"command": "npx",
		"args": ["-y", "@mcp/server-filesystem"]
	}`)

	cat, err := Scan(root, root)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	mcps := cat.ByType(MCP)
	if len(mcps) != 1 {
		t.Fatalf("expected 1 MCP item, got %d", len(mcps))
	}
	if mcps[0].Name != "my-server" {
		t.Errorf("Name = %q, want %q", mcps[0].Name, "my-server")
	}
	if mcps[0].ServerKey != "my-server" {
		t.Errorf("ServerKey = %q, want %q", mcps[0].ServerKey, "my-server")
	}
	if mcps[0].Description != "npx @mcp/server-filesystem" {
		t.Errorf("Description = %q, want %q", mcps[0].Description, "npx @mcp/server-filesystem")
	}
}

func TestScanMCP_NoConfigJSON(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// MCP directory with no config.json — just a README.
	writeFile(t, filepath.Join(root, "mcp", "no-config", "README.md"), "# Info")

	cat, err := Scan(root, root)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	mcps := cat.ByType(MCP)
	if len(mcps) != 1 {
		t.Fatalf("expected 1 MCP item, got %d", len(mcps))
	}
	if mcps[0].Name != "no-config" {
		t.Errorf("Name = %q, want %q", mcps[0].Name, "no-config")
	}
	// ServerKey should be set to directory name for flat/missing configs.
	if mcps[0].ServerKey != "no-config" {
		t.Errorf("ServerKey = %q, want %q", mcps[0].ServerKey, "no-config")
	}
	if mcps[0].Description != "" {
		t.Errorf("Description = %q, want empty", mcps[0].Description)
	}
}

func TestScanMCP_InvalidServerName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "mcp", "mixed", "config.json"), `{
		"mcpServers": {
			"valid-server": {"command": "node", "args": ["server.js"]},
			"invalid.name": {"command": "bad"}
		}
	}`)

	cat, err := Scan(root, root)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	mcps := cat.ByType(MCP)
	if len(mcps) != 1 {
		t.Fatalf("expected 1 MCP item (valid only), got %d", len(mcps))
	}
	if mcps[0].Name != "valid-server" {
		t.Errorf("Name = %q, want %q", mcps[0].Name, "valid-server")
	}

	// Should have a warning about the invalid name.
	foundWarning := false
	for _, w := range cat.Warnings {
		if strings.Contains(w, "invalid.name") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Error("expected warning about invalid.name, got none")
	}
}

func TestScanMCP_FilesSharedAcrossExplodedItems(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "mcp", "multi", "config.json"), `{
		"mcpServers": {
			"alpha": {"command": "node", "args": ["a.js"]},
			"beta": {"url": "https://beta.example.com"}
		}
	}`)
	writeFile(t, filepath.Join(root, "mcp", "multi", "README.md"), "# Multi")

	cat, err := Scan(root, root)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	mcps := cat.ByType(MCP)
	if len(mcps) != 2 {
		t.Fatalf("expected 2 MCP items, got %d", len(mcps))
	}

	// Both items should share the same files list.
	for _, m := range mcps {
		if len(m.Files) != 2 {
			t.Errorf("item %q has %d files, want 2", m.Name, len(m.Files))
		}
	}
}

func TestScanMCP_ProviderGroupingDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	// Simulate the layout created by "syllago add mcp --from claude-code":
	// mcp/claude-code/server-a/config.json
	// mcp/claude-code/server-b/config.json
	// No config.json at mcp/claude-code/ level.
	writeFile(t, filepath.Join(root, "mcp", "claude-code", "server-a", "config.json"), `{
		"mcpServers": {
			"server-a": {"command": "npx", "args": ["-y", "@example/a"]}
		}
	}`)
	writeFile(t, filepath.Join(root, "mcp", "claude-code", "server-b", "config.json"), `{
		"mcpServers": {
			"server-b": {"url": "https://mcp.example.com/b"}
		}
	}`)

	cat, err := Scan(root, root)
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	mcps := cat.ByType(MCP)
	if len(mcps) != 2 {
		t.Fatalf("expected 2 individual MCP servers, got %d", len(mcps))
	}

	byName := make(map[string]ContentItem)
	for _, m := range mcps {
		byName[m.Name] = m
	}

	a, ok := byName["server-a"]
	if !ok {
		t.Fatal("missing 'server-a'")
	}
	if a.ServerKey != "server-a" {
		t.Errorf("server-a ServerKey = %q, want %q", a.ServerKey, "server-a")
	}
	if !strings.HasSuffix(a.Path, filepath.Join("claude-code", "server-a")) {
		t.Errorf("server-a Path = %q, expected to end with claude-code/server-a", a.Path)
	}

	b, ok := byName["server-b"]
	if !ok {
		t.Fatal("missing 'server-b'")
	}
	if b.ServerKey != "server-b" {
		t.Errorf("server-b ServerKey = %q, want %q", b.ServerKey, "server-b")
	}
}

func TestMCPServerDescription(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		json string
		want string
	}{
		{"command only", `{"command": "node"}`, "node"},
		{"command with args", `{"command": "npx", "args": ["-y", "@example/server"]}`, "npx @example/server"},
		{"command with only flags", `{"command": "npx", "args": ["-y", "--port"]}`, "npx"},
		{"url only", `{"url": "https://mcp.example.com"}`, "https://mcp.example.com"},
		{"empty", `{}`, ""},
		{"url preferred over empty command", `{"url": "https://x.com", "command": ""}`, "https://x.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			value := gjsonParse(tt.json)
			got := MCPServerDescription(value)
			if got != tt.want {
				t.Errorf("MCPServerDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}

// gjsonParse is a test helper that parses a JSON string into a gjson.Result.
func gjsonParse(s string) gjson.Result {
	return gjson.Parse(s)
}
