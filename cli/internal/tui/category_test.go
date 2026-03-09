package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func TestCategoryNavigation(t *testing.T) {
	app := testApp(t)
	assertScreen(t, app, screenCategory)

	// Sidebar: 6 content types + Library + Loadouts + Add + Update + Settings + Registries + Sandbox = 13 rows (indices 0-12)
	totalRows := app.sidebar.totalItems() - 1
	if app.sidebar.cursor != 0 {
		t.Fatalf("expected initial cursor 0, got %d", app.sidebar.cursor)
	}

	// Navigate all the way down
	app = pressN(app, keyDown, totalRows)
	if app.sidebar.cursor != totalRows {
		t.Fatalf("expected cursor %d at bottom, got %d", totalRows, app.sidebar.cursor)
	}

	// Bounds clamp: pressing down again stays at bottom
	app = pressN(app, keyDown, 3)
	if app.sidebar.cursor != totalRows {
		t.Fatalf("expected cursor clamped at %d, got %d", totalRows, app.sidebar.cursor)
	}

	// Navigate back up to top
	app = pressN(app, keyUp, totalRows+5)
	if app.sidebar.cursor != 0 {
		t.Fatalf("expected cursor clamped at 0, got %d", app.sidebar.cursor)
	}
}

func TestCategorySelectEachType(t *testing.T) {
	// Sidebar content types (excludes Loadouts which is in Collections section)
	sidebarTypes := func() []catalog.ContentType {
		var types []catalog.ContentType
		for _, ct := range catalog.AllContentTypes() {
			if ct != catalog.Loadouts {
				types = append(types, ct)
			}
		}
		return types
	}()

	for i, ct := range sidebarTypes {
		app := testApp(t)
		app = pressN(app, keyDown, i)
		m, _ := app.Update(keyEnter)
		app = m.(App)

		assertScreen(t, app, screenItems)
		if app.items.contentType != ct {
			t.Fatalf("type %s: expected contentType %s, got %s", ct, ct, app.items.contentType)
		}
	}

	// Test Loadouts separately (in Collections section, after Library)
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+1) // Loadouts
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)
	if app.items.contentType != catalog.Loadouts {
		t.Fatalf("expected Loadouts contentType, got %s", app.items.contentType)
	}
}

func TestCategorySelectLibrary(t *testing.T) {
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes) // Library is right after content types
	m, _ := app.Update(keyEnter)
	app = m.(App)

	// Library now shows a card view first
	assertScreen(t, app, screenLibraryCards)

	// Drill into first card to get items
	m, _ = app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)

	// Should only contain library items
	for _, item := range app.items.items {
		if !item.Library {
			t.Fatalf("Library should only contain library items, found non-library: %s", item.Name)
		}
	}
}

func TestCategorySelectImport(t *testing.T) {
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+2) // Add
	m, _ := app.Update(keyEnter)
	app = m.(App)

	assertScreen(t, app, screenImport)
	if app.importer.step != stepSource {
		t.Fatalf("expected stepSource, got %d", app.importer.step)
	}
}

func TestCategorySelectUpdate(t *testing.T) {
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+3) // Update
	m, _ := app.Update(keyEnter)
	app = m.(App)

	assertScreen(t, app, screenUpdate)
	if app.updater.step != stepUpdateMenu {
		t.Fatalf("expected stepUpdateMenu, got %d", app.updater.step)
	}
}

func TestCategorySelectSettings(t *testing.T) {
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+4) // Settings
	m, _ := app.Update(keyEnter)
	app = m.(App)

	assertScreen(t, app, screenSettings)
}

func TestCategoryCountDisplay(t *testing.T) {
	app := testApp(t)
	view := app.View()

	// The catalog has items of various types — verify counts appear
	for _, ct := range catalog.AllContentTypes() {
		count := app.sidebar.counts[ct]
		if count > 0 {
			// The label should appear in the view
			assertContains(t, view, ct.Label())
		}
	}
}

func TestCategoryQuitFromCategory(t *testing.T) {
	app := testApp(t)

	// q should quit from category
	_, cmd := app.Update(keyRune('q'))
	if cmd == nil {
		t.Fatal("q from category should produce quit command")
	}

	// ctrl+c should also quit
	app2 := testApp(t)
	_, cmd2 := app2.Update(keyCtrlC)
	if cmd2 == nil {
		t.Fatal("ctrl+c from category should produce quit command")
	}
}

func TestCategoryQuitOnlyFromCategory(t *testing.T) {
	app := testApp(t)

	// Navigate to items screen
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)

	// q should NOT quit from items screen
	m, cmd := app.Update(keyRune('q'))
	if cmd != nil {
		// Check it's not a quit command by verifying we're still on items screen
		app = m.(App)
		if app.screen == screenItems {
			// q was consumed but didn't quit — check the cmd
			msg := cmd()
			if _, ok := msg.(tea.QuitMsg); ok {
				t.Fatal("q should not quit from items screen")
			}
		}
	}
}
