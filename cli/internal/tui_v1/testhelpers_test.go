package tui_v1

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/muesli/termenv"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func init() {
	// Force deterministic terminal state for golden tests.
	// The lipgloss default renderer is created before init() runs, so it may
	// have already detected non-deterministic terminal state from os.Stderr
	// (WSL TTY attachment varies between invocations). We must:
	// 1. Set env vars that termenv checks during lazy detection
	// 2. Replace the renderer's output to prevent terminal queries
	// 3. Explicitly set profile and background to skip sync.Once auto-detection
	os.Setenv("NO_COLOR", "1")
	os.Setenv("TERM", "dumb")
	// Initialize the bubblezone global manager so zone.Mark() calls in View()
	// don't panic during tests (in production this is called in main.go).
	zone.NewGlobal()
	// Replace the renderer's output and pin all detection values.
	r := lipgloss.DefaultRenderer()
	r.SetOutput(termenv.NewOutput(io.Discard, termenv.WithProfile(termenv.Ascii)))
	r.SetColorProfile(termenv.Ascii)
	r.SetHasDarkBackground(true)
}

// ---------------------------------------------------------------------------
// Key press helpers
// ---------------------------------------------------------------------------

var (
	keyUp       = tea.KeyMsg{Type: tea.KeyUp}
	keyDown     = tea.KeyMsg{Type: tea.KeyDown}
	keyLeft     = tea.KeyMsg{Type: tea.KeyLeft}
	keyRight    = tea.KeyMsg{Type: tea.KeyRight}
	keyEnter    = tea.KeyMsg{Type: tea.KeyEnter}
	keyEsc      = tea.KeyMsg{Type: tea.KeyEsc}
	keySpace    = tea.KeyMsg{Type: tea.KeySpace}
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
// a library item. Uses t.TempDir() with real files so the file viewer works.
func testCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	tmp := t.TempDir()

	// Create local directory for library items (simulating global content)
	os.MkdirAll(filepath.Join(tmp, "local", "skills"), 0o755)

	items := []catalog.ContentItem{
		makeSkill(t, tmp, "alpha-skill", "A helpful skill", false),
		makeSkill(t, tmp, "beta-skill", "Another skill", false),
		makeAgent(t, tmp, "test-agent", "An AI agent"),
		makeMCP(t, tmp, "test-mcp", "An MCP server"),
		makeProviderSpecific(t, tmp, "test-rule", catalog.Rules, "claude-code", "A coding rule"),
		makeProviderSpecific(t, tmp, "test-hook", catalog.Hooks, "claude-code", "A hook"),
		makeProviderSpecific(t, tmp, "test-cmd", catalog.Commands, "claude-code", "A command"),
		makeLocalSkill(t, tmp, "local-skill", "A library skill with LLM prompt"),
		makeLoadout(t, tmp, "starter-loadout", "claude-code", "Essential tools for getting started"),
		makeLoadout(t, tmp, "advanced-loadout", "claude-code", "Advanced workflow configuration"),
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
		dir = filepath.Join(root, "local", "skills", name)
	}
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name+"\n"+desc), 0o644)
	os.WriteFile(filepath.Join(dir, "helpers.md"), []byte("# Helpers\nHelper content"), 0o644)
	return catalog.ContentItem{
		Name:        name,
		Description: desc,
		Type:        catalog.Skills,
		Path:        dir,
		Files:       []string{"SKILL.md", "helpers.md"},
		Library:     local,
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
	return catalog.ContentItem{
		Name:        name,
		Description: desc,
		Type:        catalog.MCP,
		Path:        dir,
		Files:       []string{"config.json"},
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

func makeLoadout(t *testing.T, root, name, provSlug, desc string) catalog.ContentItem {
	t.Helper()
	dir := filepath.Join(root, "loadouts", provSlug, name)
	os.MkdirAll(dir, 0o755)
	// Write a valid loadout manifest that references items from the test catalog.
	// The "starter-loadout" references items that exist; "advanced-loadout" keeps it minimal.
	manifest := fmt.Sprintf("kind: loadout\nversion: 1\nname: %s\nprovider: %s\n", name, provSlug)
	if name == "starter-loadout" {
		manifest += "skills:\n  - alpha-skill\n  - beta-skill\nrules:\n  - test-rule\n"
	}
	os.WriteFile(filepath.Join(dir, "loadout.yaml"), []byte(manifest), 0o644)
	return catalog.ContentItem{
		Name:        name,
		Description: desc,
		Type:        catalog.Loadouts,
		Path:        dir,
		Provider:    provSlug,
		Files:       []string{"loadout.yaml"},
	}
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
// Large test catalog (overflow/boundary testing)
// ---------------------------------------------------------------------------

// testCatalogLarge creates a catalog with 85+ items across content types to
// test scroll, truncation, and overflow behavior. Constructs ContentItem
// structs directly (no filesystem I/O) since only list rendering is tested.
func testCatalogLarge(t *testing.T) *catalog.Catalog {
	t.Helper()
	tmp := t.TempDir()

	var items []catalog.ContentItem

	// 50 skills with varying name lengths
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("skill-%03d", i)
		desc := fmt.Sprintf("Description for skill %d", i)

		// Some very long names that need truncation
		if i%10 == 0 {
			name = fmt.Sprintf("extremely-long-skill-name-that-should-be-truncated-in-narrow-terminals-%03d", i)
		}
		// Some very long descriptions (200+ chars)
		if i%7 == 0 {
			desc = strings.Repeat("Long description text. ", 12)
		}
		// Some empty descriptions
		if i%13 == 0 {
			desc = ""
		}
		// Some with special characters in names
		if i == 5 {
			name = "skill-with-dashes-and-123"
		}

		items = append(items, catalog.ContentItem{
			Name:        name,
			Description: desc,
			Type:        catalog.Skills,
			Path:        filepath.Join(tmp, "skills", name),
			Files:       []string{"SKILL.md"},
		})
	}

	// 20 agents
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("agent-%03d", i)
		items = append(items, catalog.ContentItem{
			Name:        name,
			Description: fmt.Sprintf("Agent number %d", i),
			Type:        catalog.Agents,
			Path:        filepath.Join(tmp, "agents", name),
			Files:       []string{"AGENT.md"},
		})
	}

	// 15 MCP configs
	for i := 0; i < 15; i++ {
		name := fmt.Sprintf("mcp-%03d", i)
		items = append(items, catalog.ContentItem{
			Name:        name,
			Description: fmt.Sprintf("MCP server %d", i),
			Type:        catalog.MCP,
			Path:        filepath.Join(tmp, "mcp", name),
			Files:       []string{"config.json"},
		})
	}

	return &catalog.Catalog{
		RepoRoot: tmp,
		Items:    items,
	}
}

// testAppLarge creates a fully-wired App with the large catalog at 80x30.
func testAppLarge(t *testing.T) App {
	t.Helper()
	return testAppLargeSize(t, 80, 30)
}

// testAppLargeSize creates a fully-wired App with the large catalog
// at the specified terminal dimensions.
func testAppLargeSize(t *testing.T, width, height int) App {
	t.Helper()
	cat := testCatalogLarge(t)
	providers := testProviders(t)

	app := NewApp(cat, providers, "1.0.0", false, nil, nil, false, cat.RepoRoot)
	app.width = width
	app.height = height

	contentW := width - sidebarWidth - 1
	if contentW < 20 {
		contentW = 20
	}
	ph := app.panelHeight()
	app.sidebar.height = ph
	app.items.width = contentW
	app.items.height = ph
	app.detail.width = contentW
	app.detail.height = ph
	app.detail.fileViewer.splitView.width = contentW
	app.detail.loadoutContents.splitView.width = contentW
	app.importer.width = contentW
	app.importer.height = ph
	app.updater.width = contentW
	app.updater.height = ph
	app.settings.width = contentW
	app.settings.height = ph
	app.registries.width = contentW
	app.registries.height = ph
	app.sandboxSettings.width = contentW
	app.sandboxSettings.height = ph
	app.createLoadout.width = contentW
	app.createLoadout.height = ph
	app.toast.width = contentW
	return app
}

// testCatalogEmpty creates a catalog with no items (empty registry).
func testCatalogEmpty(t *testing.T) *catalog.Catalog {
	t.Helper()
	return &catalog.Catalog{
		RepoRoot: t.TempDir(),
		Items:    nil,
	}
}

// testAppEmpty creates a fully-wired App with an empty catalog at 80x30.
func testAppEmpty(t *testing.T) App {
	t.Helper()
	return testAppEmptySize(t, 80, 30)
}

func testAppEmptySize(t *testing.T, width, height int) App {
	t.Helper()
	cat := testCatalogEmpty(t)
	providers := testProviders(t)

	app := NewApp(cat, providers, "1.0.0", false, nil, nil, false, cat.RepoRoot)
	app.width = width
	app.height = height

	contentW := width - sidebarWidth - 1
	if contentW < 20 {
		contentW = 20
	}
	ph := app.panelHeight()
	app.sidebar.height = ph
	app.items.width = contentW
	app.items.height = ph
	app.detail.width = contentW
	app.detail.height = ph
	app.detail.fileViewer.splitView.width = contentW
	app.detail.loadoutContents.splitView.width = contentW
	app.importer.width = contentW
	app.importer.height = ph
	app.updater.width = contentW
	app.updater.height = ph
	app.settings.width = contentW
	app.settings.height = ph
	app.registries.width = contentW
	app.registries.height = ph
	app.sandboxSettings.width = contentW
	app.sandboxSettings.height = ph
	app.createLoadout.width = contentW
	app.createLoadout.height = ph
	app.toast.width = contentW
	return app
}

// navigateToLibraryItems navigates to Library items via the card view.
// Presses Enter on the first card (Skills) to get to the items list.
func navigateToLibraryItems(t *testing.T) App {
	t.Helper()
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes) // Library
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenLibraryCards)
	// Press Enter to drill into the first card
	m, _ = app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)
	return app
}

// sidebarContentCount returns the number of content type rows in the sidebar.
// Loadouts is shown in the Collections section, not with content types.
func sidebarContentCount() int {
	count := 0
	for _, ct := range catalog.AllContentTypes() {
		if ct != catalog.Loadouts {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// Test app factory
// ---------------------------------------------------------------------------

// testApp creates a fully-wired App with test catalog, providers,
// and a terminal size of 80x30.
func testApp(t *testing.T) App {
	t.Helper()
	return testAppSize(t, 80, 30)
}

func testAppSize(t *testing.T, width, height int) App {
	t.Helper()
	// Reset zone manager so zone mark IDs are deterministic regardless of
	// test execution order (Go randomizes test order within a package).
	zone.NewGlobal()
	cat := testCatalog(t)
	providers := testProviders(t)

	app := NewApp(cat, providers, "1.0.0", false, nil, nil, false, cat.RepoRoot)
	app.width = width
	app.height = height

	// Mirror WindowSizeMsg propagation so test rendering matches production.
	contentW := width - sidebarWidth - 1
	if contentW < 20 {
		contentW = 20
	}
	ph := app.panelHeight()
	app.sidebar.height = ph
	app.items.width = contentW
	app.items.height = ph
	app.detail.width = contentW
	app.detail.height = ph
	app.detail.fileViewer.splitView.width = contentW
	app.detail.loadoutContents.splitView.width = contentW
	app.importer.width = contentW
	app.importer.height = ph
	app.updater.width = contentW
	app.updater.height = ph
	app.settings.width = contentW
	app.settings.height = ph
	app.registries.width = contentW
	app.registries.height = ph
	app.sandboxSettings.width = contentW
	app.sandboxSettings.height = ph
	app.createLoadout.width = contentW
	app.createLoadout.height = ph
	app.toast.width = contentW
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
