package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
)

func TestCategoryNavigation(t *testing.T) {
	app := testApp(t)
	assertScreen(t, app, screenCategory)

	// Sidebar navigation: 8 content types + My Tools + Import + Update + Settings + Registries + Sandbox = 14 rows (indices 0-13)
	totalRows := len(catalog.AllContentTypes()) + 5 // types + My Tools + Import + Update + Settings + Registries + Sandbox
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
	types := catalog.AllContentTypes()
	for i, ct := range types {
		app := testApp(t)
		app = pressN(app, keyDown, i)
		m, _ := app.Update(keyEnter)
		app = m.(App)

		assertScreen(t, app, screenItems)
		if app.items.contentType != ct {
			t.Fatalf("type %s: expected contentType %s, got %s", ct, ct, app.items.contentType)
		}
	}
}

func TestCategorySelectMyTools(t *testing.T) {
	app := testApp(t)
	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes) // My Tools is right after all types
	m, _ := app.Update(keyEnter)
	app = m.(App)

	assertScreen(t, app, screenItems)
	if app.items.contentType != catalog.MyTools {
		t.Fatalf("expected MyTools contentType, got %s", app.items.contentType)
	}
	// Should only contain local items
	for _, item := range app.items.items {
		if !item.Local {
			t.Fatalf("My Tools should only contain local items, found non-local: %s", item.Name)
		}
	}
}

func TestCategorySelectImport(t *testing.T) {
	app := testApp(t)
	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes+1) // Import is types+1
	m, _ := app.Update(keyEnter)
	app = m.(App)

	assertScreen(t, app, screenImport)
	if app.importer.step != stepSource {
		t.Fatalf("expected stepSource, got %d", app.importer.step)
	}
}

func TestCategorySelectUpdate(t *testing.T) {
	app := testApp(t)
	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes+2) // Update is types+2
	m, _ := app.Update(keyEnter)
	app = m.(App)

	assertScreen(t, app, screenUpdate)
	if app.updater.step != stepUpdateMenu {
		t.Fatalf("expected stepUpdateMenu, got %d", app.updater.step)
	}
}

func TestCategorySelectSettings(t *testing.T) {
	app := testApp(t)
	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes+3) // Settings is types+3
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
