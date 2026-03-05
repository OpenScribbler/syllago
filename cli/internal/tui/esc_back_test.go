package tui

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// ---------------------------------------------------------------------------
// Navigation helpers for each screen
// ---------------------------------------------------------------------------

func navigateToRegistries(t *testing.T) App {
	t.Helper()
	app := testApp(t)
	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes+4) // Registries is types+4
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenRegistries)
	return app
}

func navigateToSandbox(t *testing.T) App {
	t.Helper()
	app := testApp(t)
	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes+5) // Sandbox is types+5
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenSandbox)
	return app
}

// navigateToItems navigates to the items screen for the first content type (Skills).
func navigateToItems(t *testing.T) App {
	t.Helper()
	app := testApp(t)
	// cursor starts at 0 (Skills), just press Enter
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)
	return app
}

// ---------------------------------------------------------------------------
// Esc-back tests
// ---------------------------------------------------------------------------

// TestEscapeFromRegistries verifies that pressing Esc on the registries screen
// navigates back to the category (home) screen.
func TestEscapeFromRegistries(t *testing.T) {
	app := navigateToRegistries(t)
	m, _ := app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenCategory)
	if app.focus != focusSidebar {
		t.Fatalf("expected focusSidebar after Esc from registries, got %d", app.focus)
	}
}

// TestEscapeFromSettings verifies that pressing Esc on the settings screen
// navigates back to the category (home) screen.
func TestEscapeFromSettings(t *testing.T) {
	app := navigateToSettings(t)
	m, _ := app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenCategory)
	if app.focus != focusSidebar {
		t.Fatalf("expected focusSidebar after Esc from settings, got %d", app.focus)
	}
}

// TestEscapeFromSandbox verifies that pressing Esc on the sandbox screen
// navigates back to the category (home) screen.
func TestEscapeFromSandbox(t *testing.T) {
	app := navigateToSandbox(t)
	m, _ := app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenCategory)
	if app.focus != focusSidebar {
		t.Fatalf("expected focusSidebar after Esc from sandbox, got %d", app.focus)
	}
}

// TestEscapeFromUpdate verifies that pressing Esc on the update screen (at its
// initial menu step) navigates back to the category (home) screen.
func TestEscapeFromUpdate(t *testing.T) {
	app := navigateToUpdate(t)
	// The updater starts at stepUpdateMenu, which allows Esc to go back.
	m, _ := app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenCategory)
	if app.focus != focusSidebar {
		t.Fatalf("expected focusSidebar after Esc from update, got %d", app.focus)
	}
}

// TestEscapeFromImport verifies that pressing Esc on the import screen at its
// initial step (stepSource) navigates back to the category (home) screen.
func TestEscapeFromImport(t *testing.T) {
	app := testApp(t)
	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes+1) // Add is types+1
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenImport)

	// At stepSource, Esc goes back to category.
	m, _ = app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenCategory)
	if app.focus != focusSidebar {
		t.Fatalf("expected focusSidebar after Esc from import, got %d", app.focus)
	}
}

// TestEscapeFromItems verifies that pressing Esc on the items screen navigates
// back to the category (home) screen.
func TestEscapeFromItems(t *testing.T) {
	app := navigateToItems(t)
	m, _ := app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenCategory)
	if app.focus != focusSidebar {
		t.Fatalf("expected focusSidebar after Esc from items, got %d", app.focus)
	}
}

// TestEscapeFromDetail verifies that pressing Esc on the detail screen navigates
// back to the items screen (not directly to category), preserving the items list.
func TestEscapeFromDetail(t *testing.T) {
	// Use Skills — the test catalog always has at least one skill item.
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyEsc)
	app = m.(App)
	// Detail → items (one level at a time, not straight to category)
	assertScreen(t, app, screenItems)
	if app.screen == screenCategory {
		t.Fatal("Esc from detail should go to screenItems, not screenCategory")
	}
}
