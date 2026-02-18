package tui

import (
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
)

func TestHelpOverlayToggle(t *testing.T) {
	app := testApp(t)

	// ? activates overlay
	m, _ := app.Update(keyRune('?'))
	app = m.(App)
	if !app.helpOverlay.active {
		t.Fatal("expected help overlay active after ?")
	}

	view := app.View()
	assertContains(t, view, "Keyboard Shortcuts")

	// ? again deactivates
	m, _ = app.Update(keyRune('?'))
	app = m.(App)
	if app.helpOverlay.active {
		t.Fatal("expected help overlay inactive after second ?")
	}
}

func TestHelpOverlayEscCloses(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyRune('?'))
	app = m.(App)

	m, _ = app.Update(keyEsc)
	app = m.(App)
	if app.helpOverlay.active {
		t.Fatal("expected help overlay closed after esc")
	}
}

func TestHelpOverlaySwallowsKeys(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyRune('?'))
	app = m.(App)

	// Down key should not change sidebar cursor
	origCursor := app.sidebar.cursor
	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.sidebar.cursor != origCursor {
		t.Fatal("keys should be swallowed while help overlay is active")
	}
}

func TestHelpOverlayContextSensitive(t *testing.T) {
	// Category context
	app := testApp(t)
	m, _ := app.Update(keyRune('?'))
	app = m.(App)
	view := app.View()
	assertContains(t, view, "Category Screen")

	// Close and navigate to detail
	m, _ = app.Update(keyEsc)
	app = m.(App)
	m, _ = app.Update(keyEnter) // items
	app = m.(App)
	m, _ = app.Update(keyEnter) // detail
	app = m.(App)

	m, _ = app.Update(keyRune('?'))
	app = m.(App)
	view = app.View()
	assertContains(t, view, "Detail Screen")
}

func TestHelpOverlayBlockedDuringSearch(t *testing.T) {
	app := testApp(t)

	// Activate search
	m, _ := app.Update(keyRune('/'))
	app = m.(App)

	// ? should not activate overlay during search
	m, _ = app.Update(keyRune('?'))
	app = m.(App)
	if app.helpOverlay.active {
		t.Fatal("help overlay should not activate during search")
	}
}

func TestHelpOverlayBlockedDuringTextInput(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	app.detail.confirmAction = actionEnvValue

	// ? should not activate during text input
	m, _ := app.Update(keyRune('?'))
	app = m.(App)
	if app.helpOverlay.active {
		t.Fatal("help overlay should not activate during detail text input")
	}
}
