package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// setupExportRepo creates a temp syllago repo with shared content and
// a skills/ marker directory. Returns the repo root.
func setupExportRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Create a shared skill in skills/greeting/
	skillDir := filepath.Join(root, "skills", "greeting")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Greeting Skill\nSays hello.\n"), 0644)
	os.WriteFile(filepath.Join(skillDir, "README.md"), []byte("# greeting\nA greeting skill.\n"), 0644)

	// Create a second skill in skills/farewell/
	farewellDir := filepath.Join(root, "skills", "farewell")
	os.MkdirAll(farewellDir, 0755)
	os.WriteFile(filepath.Join(farewellDir, "SKILL.md"), []byte("# Farewell Skill\nSays goodbye.\n"), 0644)

	// Create an MCP item in mcp/my-server/
	mcpDir := filepath.Join(root, "mcp", "my-server")
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
	addTestProviderOpts(t, slug, name, installBase, false)
}

// addTestProviderOpts is like addTestProvider but allows setting the Detected flag.
func addTestProviderOpts(t *testing.T, slug, name, installBase string, detected bool) {
	t.Helper()
	orig := append([]provider.Provider(nil), provider.AllProviders...)
	testProv := provider.Provider{
		Name:     name,
		Slug:     slug,
		Detected: detected,
		Detect:   func(string) bool { return detected },
		InstallDir: func(homeDir string, ct catalog.ContentType) string {
			// Use installBase directly (ignoring homeDir) for test isolation.
			switch ct {
			case catalog.Skills, catalog.Agents,
				catalog.Rules, catalog.Commands:
				return filepath.Join(installBase, string(ct))
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			switch ct {
			case catalog.Skills, catalog.Agents,
				catalog.Rules, catalog.Commands:
				return true
			}
			return false
		},
	}
	provider.AllProviders = append(provider.AllProviders, testProv)
	t.Cleanup(func() { provider.AllProviders = orig })
}
