package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func testCatalog() *catalog.Catalog {
	return &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "alpha-skill", Type: catalog.Skills, Source: "team-rules"},
			{Name: "beta-skill", Type: catalog.Skills, Source: "library"},
			{Name: "code-reviewer", Type: catalog.Agents, Source: "team-rules"},
		},
	}
}

func testProviders() []provider.Provider {
	return []provider.Provider{
		{Name: "Claude Code", Slug: "claude-code", Detected: true},
	}
}

func testApp(t *testing.T) App {
	t.Helper()
	app := NewApp(testCatalog(), testProviders(), "v1.0.0", false, nil, &config.Config{}, false, "/tmp/test")
	// Simulate window size
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m.(App)
}

func TestNewApp(t *testing.T) {
	app := NewApp(testCatalog(), testProviders(), "v1.0.0", false, nil, nil, false, "/tmp")
	if app.contentType != catalog.Skills {
		t.Errorf("default content type = %s, want Skills", app.contentType)
	}
	if app.mode != viewExplorer {
		t.Error("default mode should be viewExplorer")
	}
}

func TestAppView(t *testing.T) {
	app := testApp(t)
	view := app.View()

	if !strings.Contains(view, "SYL") {
		t.Error("view should contain SYL logo")
	}
	if !strings.Contains(view, "syllago v1.0.0") {
		t.Error("view should contain version in help bar")
	}
	if !strings.Contains(view, "alpha-skill") {
		t.Error("view should contain catalog items")
	}
}

func TestAppTooSmall(t *testing.T) {
	app := NewApp(testCatalog(), testProviders(), "v1.0.0", false, nil, &config.Config{}, false, "/tmp")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 50, Height: 15})
	view := m.(App).View()
	if !strings.Contains(view, "too small") {
		t.Error("should show 'too small' message at 50x15")
	}
}

func TestAppQuit(t *testing.T) {
	app := testApp(t)
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("pressing q should return a quit command")
	}
}

func TestAppCtrlC(t *testing.T) {
	app := testApp(t)
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("ctrl+c should return a quit command")
	}
}

func TestAppDropdownSelection(t *testing.T) {
	app := testApp(t)

	// Switch to Agents via topBarSelectMsg
	m, _ := app.Update(topBarSelectMsg{category: dropdownContent, item: "Agents"})
	a := m.(App)
	if a.contentType != catalog.Agents {
		t.Errorf("content type = %s, want agents", a.contentType)
	}
	if a.mode != viewExplorer {
		t.Error("mode should remain viewExplorer for content types")
	}
}

func TestAppSearchFlow(t *testing.T) {
	app := testApp(t)

	// Activate search
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	a := m.(App)
	if !a.search.active {
		t.Error("/ should activate search")
	}

	// Filter by query
	m, _ = a.Update(searchQueryMsg{query: "alpha"})
	a = m.(App)
	if len(a.explorer.items.items) != 1 {
		t.Errorf("filter should show 1 item, got %d", len(a.explorer.items.items))
	}

	// Cancel search
	m, _ = a.Update(searchCancelMsg{})
	a = m.(App)
	if len(a.explorer.items.items) != 2 {
		t.Errorf("cancel should restore all items, got %d", len(a.explorer.items.items))
	}
}

func TestAppToastFlow(t *testing.T) {
	app := testApp(t)
	app.toast = showToast("done", toastSuccess)
	app.toast.width = 120

	view := app.View()
	if !strings.Contains(view, "Done:") {
		t.Error("view should contain toast when active")
	}
}

func TestAppMetadata(t *testing.T) {
	app := testApp(t)
	view := app.View()

	// Should show Skills summary (no item selected)
	if !strings.Contains(view, "Skills") {
		t.Error("metadata should show Skills type")
	}
}
