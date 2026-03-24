package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
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
	assertContains(t, view, "Home:")

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
	assertContains(t, view, "Detail:")
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

func TestHelpOverlayViewAllScreens(t *testing.T) {
	// Test that View() renders context-specific content for every screen type.
	screens := []struct {
		s    screen
		want string // expected text in the output
	}{
		{screenCategory, "Home:"},
		{screenItems, "Items:"},
		{screenDetail, "Detail:"},
		{screenLibraryCards, "Library:"},
		{screenLoadoutCards, "Loadouts:"},
		{screenRegistries, "Registries:"},
		{screenImport, "Add:"},
		{screenUpdate, "Update:"},
		{screenSettings, "Settings:"},
		{screenSandbox, "Sandbox:"},
	}
	for _, tt := range screens {
		m := helpOverlayModel{active: true, height: 50}
		got := m.View(tt.s)
		if got == "" {
			t.Errorf("View(%d) returned empty", tt.s)
			continue
		}
		if !containsText(got, tt.want) {
			t.Errorf("View(%d) should contain %q", tt.s, tt.want)
		}
		if !containsText(got, "Keyboard Shortcuts") {
			t.Errorf("View(%d) should contain 'Keyboard Shortcuts' header", tt.s)
		}
	}
}

func TestHelpOverlayViewInactive(t *testing.T) {
	m := helpOverlayModel{active: false}
	if m.View(screenCategory) != "" {
		t.Error("inactive overlay should return empty string")
	}
}

func TestHelpOverlayViewScroll(t *testing.T) {
	// Use a very small height to trigger scrolling
	m := helpOverlayModel{active: true, height: 5, scrollOffset: 2}
	got := m.View(screenDetail)
	// When scrolled down, should show scroll indicator
	if got == "" {
		t.Error("scrolled view should not be empty")
	}
	if !containsText(got, "above") {
		t.Error("scrolled view should show 'above' scroll indicator")
	}
}

func TestHelpOverlayUpdate(t *testing.T) {
	m := helpOverlayModel{active: true, height: 5, scrollOffset: 0}

	// Down should increment scroll
	m.Update(keyDown)
	if m.scrollOffset != 1 {
		t.Errorf("scroll after down = %d, want 1", m.scrollOffset)
	}

	// Up should decrement
	m.Update(keyUp)
	if m.scrollOffset != 0 {
		t.Errorf("scroll after up = %d, want 0", m.scrollOffset)
	}

	// Up at 0 stays at 0
	m.Update(keyUp)
	if m.scrollOffset != 0 {
		t.Errorf("scroll should stay at 0, got %d", m.scrollOffset)
	}

	// PageDown should jump by half height
	m.scrollOffset = 0
	m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.scrollOffset != 2 { // 5/2 = 2
		t.Errorf("scroll after PgDown = %d, want 2", m.scrollOffset)
	}

	// PageUp should jump back
	m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.scrollOffset != 0 {
		t.Errorf("scroll after PgUp = %d, want 0", m.scrollOffset)
	}

	// PageUp at 0 stays at 0
	m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.scrollOffset != 0 {
		t.Errorf("scroll after PgUp at 0 = %d, want 0", m.scrollOffset)
	}

	// Home sets to 0
	m.scrollOffset = 10
	m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.scrollOffset != 0 {
		t.Errorf("scroll after Home = %d, want 0", m.scrollOffset)
	}

	// End sets to large number
	m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.scrollOffset < 1000 {
		t.Errorf("scroll after End = %d, expected large number", m.scrollOffset)
	}

	// Esc closes
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.active {
		t.Error("overlay should be inactive after Esc")
	}
}

// containsText strips ANSI and checks for substring
func containsText(s, sub string) bool {
	return strings.Contains(stripANSI(s), sub)
}

func TestHelpOverlayBlockedDuringModal(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	// When a modal is active, all keys route to the modal — help overlay should not activate
	app.envModal = newEnvSetupModal([]string{"TEST_VAR"})
	app.focus = focusModal

	m, _ := app.Update(keyRune('?'))
	app = m.(App)
	if app.helpOverlay.active {
		t.Fatal("help overlay should not activate when a modal is active")
	}
}
