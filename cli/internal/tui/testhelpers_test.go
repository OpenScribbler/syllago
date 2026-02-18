package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/model"
	"github.com/holdenhewett/romanesco/cli/internal/provider"
	"github.com/holdenhewett/romanesco/cli/internal/scan"
)

func init() {
	// Disable ANSI color output for deterministic test assertions.
	// All charmbracelet libraries honor the NO_COLOR standard.
	os.Setenv("NO_COLOR", "1")
	// Initialize the bubblezone global manager so zone.Mark() calls in View()
	// don't panic during tests (in production this is called in main.go).
	zone.NewGlobal()
}

// ---------------------------------------------------------------------------
// Key press helpers
// ---------------------------------------------------------------------------

var (
	keyUp    = tea.KeyMsg{Type: tea.KeyUp}
	keyDown  = tea.KeyMsg{Type: tea.KeyDown}
	keyEnter = tea.KeyMsg{Type: tea.KeyEnter}
	keyEsc   = tea.KeyMsg{Type: tea.KeyEsc}
	keySpace = tea.KeyMsg{Type: tea.KeySpace}
	keyTab      = tea.KeyMsg{Type: tea.KeyTab}
	keyShiftTab = tea.KeyMsg{Type: tea.KeyShiftTab}
	keyCtrlC    = tea.KeyMsg{Type: tea.KeyCtrlC}
)

func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// pressN sends a key message to the app n times and returns the updated app.
func pressN(app App, msg tea.KeyMsg, n int) App {
	for i := 0; i < n; i++ {
		m, _ := app.Update(msg)
		app = m.(App)
	}
	return app
}

// ---------------------------------------------------------------------------
// Test catalog
// ---------------------------------------------------------------------------

// testCatalog creates a catalog with items covering all 8 content types plus
// a local item. Uses t.TempDir() with real files so the file viewer works.
func testCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	tmp := t.TempDir()

	// Create my-tools directory for local items
	os.MkdirAll(filepath.Join(tmp, "my-tools", "skills"), 0o755)

	items := []catalog.ContentItem{
		makeSkill(t, tmp, "alpha-skill", "A helpful skill", false),
		makeSkill(t, tmp, "beta-skill", "Another skill", false),
		makeAgent(t, tmp, "test-agent", "An AI agent"),
		makePrompt(t, tmp, "test-prompt", "A useful prompt"),
		makeMCP(t, tmp, "test-mcp", "An MCP server"),
		makeApp(t, tmp, "test-app", "An app with install.sh"),
		makeProviderSpecific(t, tmp, "test-rule", catalog.Rules, "claude-code", "A coding rule"),
		makeProviderSpecific(t, tmp, "test-hook", catalog.Hooks, "claude-code", "A hook"),
		makeProviderSpecific(t, tmp, "test-cmd", catalog.Commands, "claude-code", "A command"),
		makeLocalSkill(t, tmp, "local-skill", "A local skill with LLM prompt"),
	}

	return &catalog.Catalog{
		RepoRoot: tmp,
		Items:    items,
	}
}

func makeSkill(t *testing.T, root, name, desc string, local bool) catalog.ContentItem {
	t.Helper()
	dir := filepath.Join(root, "skills", name)
	if local {
		dir = filepath.Join(root, "my-tools", "skills", name)
	}
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name+"\n"+desc), 0o644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# "+name+"\n\nReadme body for "+name), 0o644)
	return catalog.ContentItem{
		Name:        name,
		Description: desc,
		Type:        catalog.Skills,
		Path:        dir,
		ReadmeBody:  "# " + name + "\n\nReadme body for " + name,
		Files:       []string{"SKILL.md", "README.md"},
		Local:       local,
	}
}

func makeAgent(t *testing.T, root, name, desc string) catalog.ContentItem {
	t.Helper()
	dir := filepath.Join(root, "agents", name)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "AGENT.md"), []byte("# "+name+"\n"+desc), 0o644)
	return catalog.ContentItem{
		Name:        name,
		Description: desc,
		Type:        catalog.Agents,
		Path:        dir,
		Files:       []string{"AGENT.md"},
	}
}

func makePrompt(t *testing.T, root, name, desc string) catalog.ContentItem {
	t.Helper()
	dir := filepath.Join(root, "prompts", name)
	os.MkdirAll(dir, 0o755)
	body := "You are a helpful assistant.\nDo the thing."
	os.WriteFile(filepath.Join(dir, "prompt.md"), []byte(body), 0o644)
	return catalog.ContentItem{
		Name:        name,
		Description: desc,
		Type:        catalog.Prompts,
		Path:        dir,
		Body:        body,
		Files:       []string{"prompt.md"},
	}
}

func makeMCP(t *testing.T, root, name, desc string) catalog.ContentItem {
	t.Helper()
	dir := filepath.Join(root, "mcp", name)
	os.MkdirAll(dir, 0o755)
	// config.json with env vars for env setup wizard tests
	configJSON := `{
  "type": "stdio",
  "command": "npx",
  "args": ["-y", "@test/mcp-server"],
  "env": {
    "TEST_API_KEY": "",
    "TEST_SECRET": ""
  }
}`
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(configJSON), 0o644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# "+name), 0o644)
	return catalog.ContentItem{
		Name:        name,
		Description: desc,
		Type:        catalog.MCP,
		Path:        dir,
		ReadmeBody:  "# " + name,
		Files:       []string{"config.json", "README.md"},
	}
}

func makeApp(t *testing.T, root, name, desc string) catalog.ContentItem {
	t.Helper()
	dir := filepath.Join(root, "apps", name)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "install.sh"), []byte("#!/bin/sh\necho installed"), 0o755)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# "+name+"\n\nApp readme"), 0o644)
	return catalog.ContentItem{
		Name:               name,
		Description:        desc,
		Type:               catalog.Apps,
		Path:               dir,
		Body:               "# " + name + "\n\nApp readme",
		ReadmeBody:         "# " + name + "\n\nApp readme",
		SupportedProviders: []string{"claude-code", "cursor"},
		Files:              []string{"install.sh", "README.md"},
	}
}

func makeProviderSpecific(t *testing.T, root, name string, ct catalog.ContentType, provSlug, desc string) catalog.ContentItem {
	t.Helper()
	dir := filepath.Join(root, string(ct), provSlug, name)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, name+".md"), []byte("# "+name+"\n"+desc), 0o644)
	return catalog.ContentItem{
		Name:        name,
		Description: desc,
		Type:        ct,
		Path:        dir,
		Provider:    provSlug,
		Files:       []string{name + ".md"},
	}
}

func makeLocalSkill(t *testing.T, root, name, desc string) catalog.ContentItem {
	t.Helper()
	item := makeSkill(t, root, name, desc, true)
	// Add LLM-PROMPT.md for local items
	os.WriteFile(filepath.Join(item.Path, "LLM-PROMPT.md"),
		[]byte("Describe this skill for an LLM."), 0o644)
	item.Files = append(item.Files, "LLM-PROMPT.md")
	return item
}

// ---------------------------------------------------------------------------
// Test providers
// ---------------------------------------------------------------------------

// testProviders creates 2 providers:
//   - Claude Code: detected, supports all types
//   - Cursor: not detected, supports Skills + Rules only
func testProviders(t *testing.T) []provider.Provider {
	t.Helper()
	ccDir := t.TempDir()
	cursorDir := t.TempDir()

	return []provider.Provider{
		{
			Name:      "Claude Code",
			Slug:      "claude-code",
			Detected:  true,
			ConfigDir: ccDir,
			InstallDir: func(homeDir string, ct catalog.ContentType) string {
				return filepath.Join(ccDir, string(ct))
			},
			SupportsType: func(ct catalog.ContentType) bool { return true },
		},
		{
			Name:      "Cursor",
			Slug:      "cursor",
			Detected:  false,
			ConfigDir: cursorDir,
			InstallDir: func(homeDir string, ct catalog.ContentType) string {
				return filepath.Join(cursorDir, string(ct))
			},
			SupportsType: func(ct catalog.ContentType) bool {
				return ct == catalog.Skills || ct == catalog.Rules
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Test detectors
// ---------------------------------------------------------------------------

// mockDetector implements scan.Detector for settings tests.
type mockDetector struct {
	name string
}

func (d mockDetector) Name() string {
	return d.name
}

func (d mockDetector) Detect(root string) ([]model.Section, error) {
	return nil, nil
}

// Verify mockDetector implements scan.Detector at compile time.
var _ scan.Detector = mockDetector{}

// testDetectors returns 2 mock detectors for settings screen tests.
func testDetectors() []scan.Detector {
	return []scan.Detector{
		mockDetector{name: "go-detector"},
		mockDetector{name: "node-detector"},
	}
}

// ---------------------------------------------------------------------------
// Test app factory
// ---------------------------------------------------------------------------

// testApp creates a fully-wired App with test catalog, providers, detectors,
// and a terminal size of 80x30.
func testApp(t *testing.T) App {
	t.Helper()
	cat := testCatalog(t)
	providers := testProviders(t)
	detectors := testDetectors()

	app := NewApp(cat, providers, detectors, "1.0.0", false)
	app.width = 80
	app.height = 30
	// Propagate dimensions to sub-models that need them
	app.items.width = 80
	app.items.height = 30
	app.detail.width = 80
	app.detail.height = 30
	app.settings.width = 80
	app.settings.height = 30
	app.importer.width = 80
	app.importer.height = 30
	app.updater.width = 80
	app.updater.height = 30
	return app
}

// ---------------------------------------------------------------------------
// Assertion helpers
// ---------------------------------------------------------------------------

// assertScreen checks that the app is on the expected screen.
func assertScreen(t *testing.T, app App, expected screen) {
	t.Helper()
	if app.screen != expected {
		t.Fatalf("expected screen %d, got %d", expected, app.screen)
	}
}

// assertContains fails if s does not contain substr.
func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("expected output to contain %q, but it didn't.\nGot:\n%s", substr, s)
	}
}

// assertNotContains fails if s contains substr.
func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Fatalf("expected output NOT to contain %q, but it did.\nGot:\n%s", substr, s)
	}
}
