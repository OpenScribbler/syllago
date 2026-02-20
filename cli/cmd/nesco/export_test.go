package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
	"github.com/holdenhewett/nesco/cli/internal/output"
	"github.com/holdenhewett/nesco/cli/internal/provider"
)

// setupExportRepo creates a temp nesco repo with my-tools/ content and
// a skills/ marker directory. Returns the repo root.
func setupExportRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Create the skills/ marker so findContentRepoRoot works.
	os.MkdirAll(filepath.Join(root, "skills"), 0755)

	// Create a skill in my-tools/skills/greeting/
	skillDir := filepath.Join(root, "my-tools", "skills", "greeting")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Greeting Skill\nSays hello.\n"), 0644)
	os.WriteFile(filepath.Join(skillDir, "README.md"), []byte("# greeting\nA greeting skill.\n"), 0644)

	// Create a second skill in my-tools/skills/farewell/
	farewellDir := filepath.Join(root, "my-tools", "skills", "farewell")
	os.MkdirAll(farewellDir, 0755)
	os.WriteFile(filepath.Join(farewellDir, "SKILL.md"), []byte("# Farewell Skill\nSays goodbye.\n"), 0644)

	// Create an MCP item in my-tools/mcp/my-server/
	mcpDir := filepath.Join(root, "my-tools", "mcp", "my-server")
	os.MkdirAll(mcpDir, 0755)
	os.WriteFile(filepath.Join(mcpDir, "README.md"), []byte("# My MCP Server\n"), 0644)

	return root
}

// withFakeRepoRoot overrides findSkillsDir so findContentRepoRoot returns root.
// Returns a cleanup function registered via t.Cleanup.
func withFakeRepoRoot(t *testing.T, root string) {
	t.Helper()
	orig := findSkillsDir
	findSkillsDir = func(dir string) (string, error) { return root, nil }
	t.Cleanup(func() { findSkillsDir = orig })
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

func TestExportJSONMergeSentinelSkipped(t *testing.T) {
	root := setupExportRepo(t)
	withFakeRepoRoot(t, root)

	// Claude Code uses JSON merge for MCP. We have an MCP item in my-tools/.
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
