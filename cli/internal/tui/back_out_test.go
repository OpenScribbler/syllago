package tui

// Tests for the back-out routing shared between keyQuit and KeyEsc.
//
// Coverage:
//   - handleBackOut returns false when on the landing page (Library browse).
//   - handleBackOut backs out one level from gallery drill-in (library
//     detail → library browse → exit drill-in to gallery).
//   - Esc key triggers handleBackOut and never quits the app.
//   - Esc on the landing page is a no-op (vs. q which quits).

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHandleBackOut_LandingPage_NotHandled(t *testing.T) {
	t.Parallel()
	a := testApp(t)
	handled, _, cmd := a.handleBackOut(false)
	if handled {
		t.Error("landing page (Library browse) should not be handled — caller decides quit-vs-noop")
	}
	if cmd != nil {
		t.Error("no cmd expected on unhandled landing page")
	}
}

func TestHandleBackOut_GalleryDrillIn_LibraryDetail_BacksToBrowse(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	m, _ := app.Update(keyTab) // navigate to Registries (gallery)
	a := m.(App)
	a.galleryDrillIn = true
	a.library.mode = libraryDetail

	handled, model, _ := a.handleBackOut(false)
	if !handled {
		t.Fatal("expected handleBackOut to consume the action")
	}
	a = model.(App)
	if a.library.mode != libraryBrowse {
		t.Errorf("expected libraryBrowse after back-out, got %v", a.library.mode)
	}
	if !a.galleryDrillIn {
		t.Error("should still be in gallery drill-in after backing out of detail")
	}
}

func TestHandleBackOut_GalleryDrillIn_LibraryBrowse_ExitsDrillIn(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	m, _ := app.Update(keyTab)
	a := m.(App)
	a.galleryDrillIn = true
	a.galleryDrillCard = "my-registry"

	handled, model, _ := a.handleBackOut(false)
	if !handled {
		t.Fatal("expected handleBackOut to consume the action")
	}
	a = model.(App)
	if a.galleryDrillIn {
		t.Error("expected galleryDrillIn=false after backing out of registry drill-in")
	}
	if a.galleryDrillCard != "" {
		t.Errorf("expected galleryDrillCard cleared, got %q", a.galleryDrillCard)
	}
}

func TestEsc_GalleryDrillIn_BacksOut(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	m, _ := app.Update(keyTab)
	a := m.(App)
	a.galleryDrillIn = true
	a.galleryDrillCard = "my-registry"

	m, cmd := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if a.galleryDrillIn {
		t.Error("Esc should exit gallery drill-in")
	}
	// Esc should not produce tea.Quit.
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, ok := msg.(tea.QuitMsg); ok {
				t.Error("Esc must never quit the app")
			}
		}
	}
}

func TestEsc_LandingPage_NoOpDoesNotQuit(t *testing.T) {
	t.Parallel()
	a := testApp(t)
	if !a.isLibraryTab() {
		t.Fatal("test precondition: testApp should land on Library")
	}
	if a.library.mode != libraryBrowse {
		t.Fatal("test precondition: Library should start in browse mode")
	}

	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	// Landing-page Esc must not quit the app.
	if cmd != nil {
		if msg := cmd(); msg != nil {
			if _, ok := msg.(tea.QuitMsg); ok {
				t.Error("Esc on landing page must NOT quit (vs. q which does)")
			}
		}
	}
}

func TestQ_LandingPage_StillQuits(t *testing.T) {
	t.Parallel()
	a := testApp(t)
	_, cmd := a.Update(keyRune('q'))
	if cmd == nil {
		t.Fatal("q on landing page should produce a tea.Quit cmd")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("q on landing page should quit; got %T", msg)
	}
}

// Regression: Esc on a non-library tab must NOT switch back to the Library
// landing page — that's `q`'s job. If Esc resets the tab, persisted search
// filters on Content/Registries tabs become impossible to clear with Esc.
func TestEsc_NonLibraryTab_NoDrillIn_DoesNotResetTab(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	m, _ := app.Update(keyTab) // Library → Registries
	a := m.(App)
	if a.isLibraryTab() {
		t.Fatal("test precondition: Tab should leave Library")
	}

	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if a.isLibraryTab() {
		t.Error("Esc on non-library tab must not switch back to Library — that's q's job")
	}
}

func TestQ_NonLibraryTab_NoDrillIn_ResetsTab(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	m, _ := app.Update(keyTab)
	a := m.(App)
	if a.isLibraryTab() {
		t.Fatal("test precondition: Tab should leave Library")
	}

	m, _ = a.Update(keyRune('q'))
	a = m.(App)
	if !a.isLibraryTab() {
		t.Error("q on non-library tab should return to Library landing")
	}
}

func TestQ_GalleryDrillIn_BacksOutSameAsEsc(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	m, _ := app.Update(keyTab)
	a := m.(App)
	a.galleryDrillIn = true

	m, _ = a.Update(keyRune('q'))
	a = m.(App)
	if a.galleryDrillIn {
		t.Error("q in gallery drill-in should back out (parity with Esc)")
	}
}
