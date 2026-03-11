package tui

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// ---------------------------------------------------------------------------
// Items cursor boundary tests
// ---------------------------------------------------------------------------

func TestScrollItems_CursorClampsAtEnd(t *testing.T) {
	app := testAppLarge(t)
	// Navigate to Skills items (50 items)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)

	// Press Down 60 times (more than 50 items)
	app = pressN(app, keyDown, 60)

	want := len(app.items.items) - 1
	if app.items.cursor != want {
		t.Errorf("cursor should clamp at %d, got %d", want, app.items.cursor)
	}
}

func TestScrollItems_CursorClampsAtStart(t *testing.T) {
	app := testAppLarge(t)
	m, _ := app.Update(keyEnter)
	app = m.(App)

	// Press Up at cursor=0 — should stay at 0
	app = pressN(app, keyUp, 5)

	if app.items.cursor != 0 {
		t.Errorf("cursor should stay at 0, got %d", app.items.cursor)
	}
}

func TestScrollItems_HomeEnd(t *testing.T) {
	app := testAppLarge(t)
	m, _ := app.Update(keyEnter)
	app = m.(App)

	// Move down, then Home
	app = pressN(app, keyDown, 20)
	m, _ = app.Update(keyRune('g')) // Home binding
	app = m.(App)
	if app.items.cursor != 0 {
		t.Errorf("Home should set cursor to 0, got %d", app.items.cursor)
	}

	// End
	m, _ = app.Update(keyRune('G')) // End binding
	app = m.(App)
	want := len(app.items.items) - 1
	if app.items.cursor != want {
		t.Errorf("End should set cursor to %d, got %d", want, app.items.cursor)
	}
}

// ---------------------------------------------------------------------------
// Sidebar cursor boundary tests
// ---------------------------------------------------------------------------

func TestScrollSidebar_CursorClampsAtEnd(t *testing.T) {
	app := testApp(t)
	// Press Down many times past the sidebar length
	app = pressN(app, keyDown, 50)

	max := app.sidebar.totalItems() - 1
	if app.sidebar.cursor > max {
		t.Errorf("sidebar cursor should not exceed %d, got %d", max, app.sidebar.cursor)
	}
}

func TestScrollSidebar_CursorClampsAtStart(t *testing.T) {
	app := testApp(t)
	app = pressN(app, keyUp, 5)

	if app.sidebar.cursor != 0 {
		t.Errorf("sidebar cursor should stay at 0, got %d", app.sidebar.cursor)
	}
}

// ---------------------------------------------------------------------------
// Detail scroll behavior
// ---------------------------------------------------------------------------

func TestScrollDetail_TabSwitchResetsScroll(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)

	// Scroll down in overview
	app = pressN(app, keyDown, 5)
	if app.detail.scrollOffset == 0 {
		// If scroll didn't change, that's OK — content may be short.
		// The important thing is the next assertion after tab switch.
	}
	initialScroll := app.detail.scrollOffset

	// Switch to Files tab
	m, _ := app.Update(keyRune('2'))
	app = m.(App)

	if app.detail.scrollOffset != 0 {
		t.Errorf("tab switch should reset scrollOffset to 0, got %d (was %d)",
			app.detail.scrollOffset, initialScroll)
	}
}

func TestScrollDetail_FileViewerBackResetsScroll(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)

	// Switch to Files tab
	m, _ := app.Update(keyRune('2'))
	app = m.(App)

	// Enter a file
	m, _ = app.Update(keyEnter)
	app = m.(App)

	if !app.detail.fileViewer.viewing {
		t.Skip("file viewer not activated — file may not be readable in test")
	}

	// Scroll down in file viewer
	app = pressN(app, keyDown, 3)

	// Press Esc to go back to file list
	m, _ = app.Update(keyEsc)
	app = m.(App)

	if app.detail.fileViewer.scrollOffset != 0 {
		t.Errorf("exiting file viewer should reset scrollOffset to 0, got %d",
			app.detail.fileViewer.scrollOffset)
	}
}

// ---------------------------------------------------------------------------
// Navigation resets
// ---------------------------------------------------------------------------

func TestScrollItems_NavigateAwayResetsViewCursor(t *testing.T) {
	app := testAppLarge(t)
	// Navigate to Skills items
	m, _ := app.Update(keyEnter)
	app = m.(App)

	// Move cursor down
	app = pressN(app, keyDown, 15)
	if app.items.cursor != 15 {
		t.Fatalf("expected cursor at 15, got %d", app.items.cursor)
	}

	// Go back to category (Esc)
	m, _ = app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenCategory)

	// Re-enter Skills
	m, _ = app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)

	// Cursor should be reset to 0 since we re-entered the items list
	if app.items.cursor != 0 {
		t.Errorf("re-entering items should reset cursor to 0, got %d", app.items.cursor)
	}
}
