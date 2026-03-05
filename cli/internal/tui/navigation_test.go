package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestItemsNavigation(t *testing.T) {
	items := []catalog.ContentItem{
		{Name: "item-a", Type: catalog.Skills, Description: "First"},
		{Name: "item-b", Type: catalog.Skills, Description: "Second"},
		{Name: "item-c", Type: catalog.Skills, Description: "Third"},
	}

	m := newItemsModel(catalog.Skills, items, nil, "/tmp")
	m.width = 80
	m.height = 30

	if m.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", m.cursor)
	}

	// Press Down twice
	down := tea.KeyMsg{Type: tea.KeyDown}
	m, _ = m.Update(down)
	if m.cursor != 1 {
		t.Fatalf("after Down, expected cursor 1, got %d", m.cursor)
	}
	m, _ = m.Update(down)
	if m.cursor != 2 {
		t.Fatalf("after 2nd Down, expected cursor 2, got %d", m.cursor)
	}

	// Down at end stays
	m, _ = m.Update(down)
	if m.cursor != 2 {
		t.Fatalf("at end, expected cursor 2, got %d", m.cursor)
	}

	// Up works
	up := tea.KeyMsg{Type: tea.KeyUp}
	m, _ = m.Update(up)
	if m.cursor != 1 {
		t.Fatalf("after Up, expected cursor 1, got %d", m.cursor)
	}

	// View renders > at cursor position
	view := m.View()
	lines := strings.Split(view, "\n")
	foundCursor := false
	for _, line := range lines {
		if strings.Contains(line, ">") && strings.Contains(line, "item-b") {
			foundCursor = true
			break
		}
	}
	if !foundCursor {
		t.Fatal("expected > next to item-b (cursor=1)")
	}
}

func TestAppFullNavigation(t *testing.T) {
	cat := &catalog.Catalog{
		RepoRoot: "/tmp",
		Items: []catalog.ContentItem{
			{Name: "item-a", Type: catalog.Skills, Description: "First", Path: "/tmp/a"},
			{Name: "item-b", Type: catalog.Skills, Description: "Second", Path: "/tmp/b"},
		},
	}
	providers := []provider.Provider{}

	app := NewApp(cat, providers, "", false, nil, nil, false, "")
	app.width = 80
	app.height = 30

	// Category → Items via Enter
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	model, _ := app.Update(enter)
	app = model.(App)

	if app.screen != screenItems {
		t.Fatalf("expected screenItems, got %d", app.screen)
	}
	if len(app.items.items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(app.items.items))
	}

	// Navigate items with Down
	down := tea.KeyMsg{Type: tea.KeyDown}
	model, _ = app.Update(down)
	app = model.(App)
	if app.items.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", app.items.cursor)
	}

	// Enter detail
	model, _ = app.Update(enter)
	app = model.(App)
	if app.screen != screenDetail {
		t.Fatalf("expected screenDetail, got %d", app.screen)
	}

	// Esc goes back to items
	esc := tea.KeyMsg{Type: tea.KeyEsc}
	model, _ = app.Update(esc)
	app = model.(App)
	if app.screen != screenItems {
		t.Fatalf("expected screenItems after Esc, got %d", app.screen)
	}

	// Esc from items: new behavior is focus returns to sidebar, screen stays as items
	model, _ = app.Update(esc)
	app = model.(App)
	if app.focus != focusSidebar {
		t.Fatalf("expected focusSidebar after 2nd Esc (from items), got focus=%d", app.focus)
	}

	// ctrl+c quits from any screen
	ctrlC := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := app.Update(ctrlC)
	if cmd == nil {
		t.Fatal("ctrl+c should produce a quit command")
	}
}

func TestImportBrowseFlow(t *testing.T) {
	// Create a test catalog and temp directory with a valid skill
	tmp := t.TempDir()
	skillDir := filepath.Join(tmp, "skills", "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: test-skill\ndescription: A test\n---\n"), 0644)

	// Create local dir
	os.MkdirAll(filepath.Join(tmp, "local", "skills"), 0755)

	cat := &catalog.Catalog{RepoRoot: tmp, Items: nil}
	providers := []provider.Provider{}

	app := NewApp(cat, providers, "", false, nil, nil, false, "")
	app.width = 80
	app.height = 30

	// Navigate to Add (types + Library + Add = types+1)
	down := tea.KeyMsg{Type: tea.KeyDown}
	enter := tea.KeyMsg{Type: tea.KeyEnter}

	// Move cursor to Import option
	for i := 0; i < len(catalog.AllContentTypes())+1; i++ {
		model, _ := app.Update(down)
		app = model.(App)
	}
	model, _ := app.Update(enter)
	app = model.(App)

	if app.screen != screenImport {
		t.Fatalf("expected screenImport, got %d", app.screen)
	}

	// Source: pick Local Path (cursor=0, just Enter)
	model, _ = app.Update(enter)
	app = model.(App)
	if app.importer.step != stepType {
		t.Fatalf("expected stepType, got %d", app.importer.step)
	}

	// Type: pick Skills (cursor=0, just Enter)
	model, _ = app.Update(enter)
	app = model.(App)
	if app.importer.step != stepBrowseStart {
		t.Fatalf("expected stepBrowseStart, got %d", app.importer.step)
	}
}

func TestTooSmallMessageNoUnicode(t *testing.T) {
	app := testApp(t)
	app.tooSmall = true
	app.width = 40 // force width below the 60-column threshold

	view := app.View()
	assertContains(t, view, "Terminal too small")
	// Should use ASCII "x" not Unicode "×" for dimensions
	assertContains(t, view, "60x20")
	assertNotContains(t, view, "×")
}
