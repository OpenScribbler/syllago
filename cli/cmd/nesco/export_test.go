package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/metadata"
	"github.com/OpenScribbler/nesco/cli/internal/output"
	"github.com/OpenScribbler/nesco/cli/internal/provider"
)

// setupExportRepo creates a temp nesco repo with local/ content and
// a skills/ marker directory. Returns the repo root.
func setupExportRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Create the skills/ marker so findContentRepoRoot works.
	os.MkdirAll(filepath.Join(root, "skills"), 0755)

	// Create a skill in local/skills/greeting/
	skillDir := filepath.Join(root, "local", "skills", "greeting")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Greeting Skill\nSays hello.\n"), 0644)
	os.WriteFile(filepath.Join(skillDir, "README.md"), []byte("# greeting\nA greeting skill.\n"), 0644)

	// Create a second skill in local/skills/farewell/
	farewellDir := filepath.Join(root, "local", "skills", "farewell")
	os.MkdirAll(farewellDir, 0755)
	os.WriteFile(filepath.Join(farewellDir, "SKILL.md"), []byte("# Farewell Skill\nSays goodbye.\n"), 0644)

	// Create an MCP item in local/mcp/my-server/
	mcpDir := filepath.Join(root, "local", "mcp", "my-server")
	os.MkdirAll(mcpDir, 0755)
	os.WriteFile(filepath.Join(mcpDir, "README.md"), []byte("# My MCP Server\n"), 0644)

	return root
}

// withFakeRepoRoot overrides findProjectRoot so findContentRepoRoot returns root.
// Returns a cleanup function registered via t.Cleanup.
func withFakeRepoRoot(t *testing.T, root string) {
	t.Helper()
	orig := findProjectRoot
	findProjectRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findProjectRoot = orig })
}

// addTestProvider injects a temporary provider into AllProviders for testing.
// The provider's InstallDir points to installBase/<type>.
// Returns a cleanup function registered via t.Cleanup.
func addTestProvider(t *testing.T, slug, name, installBase string) {
	t.Helper()
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	testProv := provider.Provider{
		Name: name,
		Slug: slug,
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			// Use installBase directly (ignoring homeDir) for test isolation.
			switch ct {
			case catalog.Skills, catalog.Agents, catalog.Prompts, catalog.Apps,
				catalog.Rules, catalog.Commands:
				return filepath.Join(installBase, string(ct))
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			switch ct {
			case catalog.Skills, catalog.Agents, catalog.Prompts, catalog.Apps,
				catalog.Rules, catalog.Commands:
				return true
			}
			return false
		},
	}
	provider.AllProviders = append(provider.AllProviders, testProv)
	t.Cleanup(func() { provider.AllProviders = orig })
}

func TestExportSkillToProvider(t *testing.T) {
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)

	installBase := t.TempDir()
	addTestProvider(t, "test-provider", "Test Provider", installBase)

	stdout, stderr := output.SetForTest(t)

	exportCmd.Flags().Set("to", "test-provider")
	defer exportCmd.Flags().Set("to", "")
	exportCmd.Flags().Set("type", "skills")
	defer exportCmd.Flags().Set("type", "")

	err := exportCmd.RunE(exportCmd, []string{})
	if err != nil {
		t.Fatalf("export failed: %v\nstderr: %s", err, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "Exported greeting to") {
		t.Errorf("expected 'Exported greeting to ...' in output, got: %s", out)
	}
	if !strings.Contains(out, "Exported farewell to") {
		t.Errorf("expected 'Exported farewell to ...' in output, got: %s", out)
	}

	// Verify the file was actually copied.
	skillFile := filepath.Join(installBase, "skills", "greeting", "SKILL.md")
	if _, err := os.Stat(skillFile); err != nil {
		t.Errorf("expected skill file at %s, got error: %v", skillFile, err)
	}
}

func TestExportWithNameFilter(t *testing.T) {
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)

	installBase := t.TempDir()
	addTestProvider(t, "test-provider", "Test Provider", installBase)

	stdout, _ := output.SetForTest(t)

	exportCmd.Flags().Set("to", "test-provider")
	defer exportCmd.Flags().Set("to", "")
	exportCmd.Flags().Set("name", "greet")
	defer exportCmd.Flags().Set("name", "")

	err := exportCmd.RunE(exportCmd, []string{})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "greeting") {
		t.Errorf("expected greeting in output, got: %s", out)
	}
	if strings.Contains(out, "farewell") {
		t.Errorf("name filter should exclude farewell, got: %s", out)
	}
}

func TestExportUnknownProvider(t *testing.T) {
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)

	_, stderr := output.SetForTest(t)

	exportCmd.Flags().Set("to", "nonexistent-provider")
	defer exportCmd.Flags().Set("to", "")

	err := exportCmd.RunE(exportCmd, []string{})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}

	errOut := stderr.String()
	if !strings.Contains(errOut, "unknown provider") {
		t.Errorf("expected 'unknown provider' in error output, got: %s", errOut)
	}
}

func TestExportTypeNotSupported(t *testing.T) {
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)

	installBase := t.TempDir()
	// Add a provider that only supports rules.
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	limitedProv := provider.Provider{
		Name: "Limited",
		Slug: "limited-provider",
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			if ct == catalog.Rules {
				return filepath.Join(installBase, "rules")
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Rules
		},
	}
	provider.AllProviders = append(provider.AllProviders, limitedProv)
	t.Cleanup(func() { provider.AllProviders = orig })

	_, stderr := output.SetForTest(t)

	exportCmd.Flags().Set("to", "limited-provider")
	defer exportCmd.Flags().Set("to", "")
	exportCmd.Flags().Set("type", "skills")
	defer exportCmd.Flags().Set("type", "")

	err := exportCmd.RunE(exportCmd, []string{})
	if err != nil {
		t.Fatalf("export should not error, it should skip: %v", err)
	}

	errOut := stderr.String()
	if !strings.Contains(errOut, "does not support") {
		t.Errorf("expected 'does not support' message, got stderr: %s", errOut)
	}
}

func TestExportDefaultSourceOnlyLocal(t *testing.T) {
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)

	// Add a shared skill at <root>/skills/shared-skill/ (not in local/).
	sharedDir := filepath.Join(root, "skills", "shared-skill")
	os.MkdirAll(sharedDir, 0755)
	os.WriteFile(filepath.Join(sharedDir, "SKILL.md"), []byte("# Shared Skill\nA shared skill.\n"), 0644)

	installBase := t.TempDir()
	addTestProvider(t, "test-provider", "Test Provider", installBase)

	stdout, _ := output.SetForTest(t)

	exportCmd.Flags().Set("to", "test-provider")
	defer exportCmd.Flags().Set("to", "")
	exportCmd.Flags().Set("type", "skills")
	defer exportCmd.Flags().Set("type", "")
	// Don't set --source; it should default to "local".

	err := exportCmd.RunE(exportCmd, []string{})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	out := stdout.String()
	// Should export local items (greeting, farewell).
	if !strings.Contains(out, "greeting") {
		t.Errorf("expected local item 'greeting' in output, got: %s", out)
	}
	// Should NOT export shared items.
	if strings.Contains(out, "shared-skill") {
		t.Errorf("default source should not export shared items, got: %s", out)
	}
}

func TestExportSourceSharedOnly(t *testing.T) {
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)

	// Add a shared skill at <root>/skills/shared-skill/ (not in local/).
	sharedDir := filepath.Join(root, "skills", "shared-skill")
	os.MkdirAll(sharedDir, 0755)
	os.WriteFile(filepath.Join(sharedDir, "SKILL.md"), []byte("# Shared Skill\nA shared skill.\n"), 0644)

	installBase := t.TempDir()
	addTestProvider(t, "test-provider", "Test Provider", installBase)

	stdout, _ := output.SetForTest(t)

	exportCmd.Flags().Set("to", "test-provider")
	defer exportCmd.Flags().Set("to", "")
	exportCmd.Flags().Set("type", "skills")
	defer exportCmd.Flags().Set("type", "")
	exportCmd.Flags().Set("source", "shared")
	defer exportCmd.Flags().Set("source", "local")

	err := exportCmd.RunE(exportCmd, []string{})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	out := stdout.String()
	// Should export shared items.
	if !strings.Contains(out, "shared-skill") {
		t.Errorf("expected shared item 'shared-skill' in output, got: %s", out)
	}
	// Should NOT export local items.
	if strings.Contains(out, "greeting") {
		t.Errorf("source=shared should not export local items, got: %s", out)
	}
}

func TestExportToAllRunsMultipleProviders(t *testing.T) {
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)

	// Replace AllProviders with two test providers so we control the output
	// and don't depend on real provider install dirs.
	installBase1 := t.TempDir()
	installBase2 := t.TempDir()

	orig := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{
		{
			Name: "ProviderA",
			Slug: "provider-a",
			InstallDir: func(homeDir string, ct catalog.ContentType) string {
				if ct == catalog.Skills {
					return filepath.Join(installBase1, string(ct))
				}
				return ""
			},
			SupportsType: func(ct catalog.ContentType) bool {
				return ct == catalog.Skills
			},
		},
		{
			Name: "ProviderB",
			Slug: "provider-b",
			InstallDir: func(homeDir string, ct catalog.ContentType) string {
				if ct == catalog.Skills {
					return filepath.Join(installBase2, string(ct))
				}
				return ""
			},
			SupportsType: func(ct catalog.ContentType) bool {
				return ct == catalog.Skills
			},
		},
	}
	t.Cleanup(func() { provider.AllProviders = orig })

	stdout, _ := output.SetForTest(t)

	exportCmd.Flags().Set("to", "all")
	defer exportCmd.Flags().Set("to", "")
	exportCmd.Flags().Set("type", "skills")
	defer exportCmd.Flags().Set("type", "")

	err := exportCmd.RunE(exportCmd, []string{})
	if err != nil {
		t.Fatalf("export --to all failed: %v", err)
	}

	out := stdout.String()

	// Should show headers for both providers.
	if !strings.Contains(out, "ProviderA") {
		t.Errorf("expected ProviderA header in output, got: %s", out)
	}
	if !strings.Contains(out, "ProviderB") {
		t.Errorf("expected ProviderB header in output, got: %s", out)
	}

	// Should show the summary section.
	if !strings.Contains(out, "Export All Summary") {
		t.Errorf("expected summary section in output, got: %s", out)
	}

	// Should have exported skills to both providers.
	greetingA := filepath.Join(installBase1, "skills", "greeting", "SKILL.md")
	if _, err := os.Stat(greetingA); err != nil {
		t.Errorf("expected skill at %s for provider-a, got error: %v", greetingA, err)
	}
	greetingB := filepath.Join(installBase2, "skills", "greeting", "SKILL.md")
	if _, err := os.Stat(greetingB); err != nil {
		t.Errorf("expected skill at %s for provider-b, got error: %v", greetingB, err)
	}
}

func TestExportJSONMergeSentinelSkipped(t *testing.T) {
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)

	// Claude Code uses JSON merge for MCP. We have an MCP item in local/.
	// Export should skip it with a warning.
	_, stderr := output.SetForTest(t)

	exportCmd.Flags().Set("to", "claude-code")
	defer exportCmd.Flags().Set("to", "")
	exportCmd.Flags().Set("type", "mcp")
	defer exportCmd.Flags().Set("type", "")

	err := exportCmd.RunE(exportCmd, []string{})
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	errOut := stderr.String()
	if !strings.Contains(errOut, "JSON merge") {
		t.Errorf("expected JSON merge warning, got stderr: %s", errOut)
	}
}

func TestExportWarnMessage(t *testing.T) {
	tests := []struct {
		name     string
		item     catalog.ContentItem
		wantMsg  string
		wantEmpty bool
	}{
		{
			name: "example content",
			item: catalog.ContentItem{
				Name: "demo-skill",
				Type: catalog.Skills,
				Meta: &metadata.Meta{Tags: []string{"example"}},
			},
			wantMsg: "example content (for reference, not intended for direct use)",
		},
		{
			name: "builtin content",
			item: catalog.ContentItem{
				Name: "nesco-guide",
				Type: catalog.Skills,
				Meta: &metadata.Meta{Tags: []string{"builtin"}},
			},
			wantMsg: "built-in nesco content (may conflict with provider defaults)",
		},
		{
			name: "normal content",
			item: catalog.ContentItem{
				Name: "my-skill",
				Type: catalog.Skills,
			},
			wantEmpty: true,
		},
		{
			name: "content with unrelated tags",
			item: catalog.ContentItem{
				Name: "tagged-skill",
				Type: catalog.Skills,
				Meta: &metadata.Meta{Tags: []string{"productivity", "go"}},
			},
			wantEmpty: true,
		},
		{
			name: "example takes precedence over builtin",
			item: catalog.ContentItem{
				Name: "dual-tagged",
				Type: catalog.Skills,
				Meta: &metadata.Meta{Tags: []string{"example", "builtin"}},
			},
			wantMsg: "example content (for reference, not intended for direct use)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exportWarnMessage(tt.item)
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("exportWarnMessage() = %q, want empty string", got)
				}
				return
			}
			if got != tt.wantMsg {
				t.Errorf("exportWarnMessage() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}
