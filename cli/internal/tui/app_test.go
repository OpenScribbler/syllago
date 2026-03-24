package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Unit tests ---

func TestApp_WindowSizeMsg(t *testing.T) {
	app := NewApp(testCatalog(t), testProviders(), "0.0.0-test", false, nil, testConfig(), false, "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a := m.(App)
	if a.width != 120 || a.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", a.width, a.height)
	}
	if !a.ready {
		t.Error("app should be ready after WindowSizeMsg")
	}
}

func TestApp_NotReadyBeforeWindowSize(t *testing.T) {
	app := NewApp(testCatalog(t), testProviders(), "0.0.0-test", false, nil, testConfig(), false, "")
	view := app.View()
	if view != "" {
		t.Errorf("expected empty view before WindowSizeMsg, got %q", view)
	}
}

func TestApp_TooSmall(t *testing.T) {
	app := testAppSize(t, 50, 15)
	view := app.View()
	assertContains(t, view, "Terminal too small")
	assertContains(t, view, "60x20")
}

func TestApp_QuitOnCtrlC(t *testing.T) {
	app := testApp(t)
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should produce a quit command")
	}
}

func TestApp_QuitOnQ(t *testing.T) {
	app := testApp(t)
	_, cmd := app.Update(keyRune('q'))
	if cmd == nil {
		t.Fatal("q should produce a quit command")
	}
}

func TestApp_EmptyContentGuidance(t *testing.T) {
	app := testApp(t)
	view := app.View()
	assertContains(t, view, "No content")
}

func TestApp_HelpBarVersion(t *testing.T) {
	app := testApp(t)
	view := app.View()
	assertContains(t, view, "syllago v")
}

func TestApp_Branding(t *testing.T) {
	app := testApp(t)
	view := app.View()
	assertContains(t, view, "syl")
	assertContains(t, view, "lago")
}

func TestApp_ContentHeight(t *testing.T) {
	app := testAppSize(t, 80, 30)
	h := app.contentHeight()
	// 30 total - 1 topbar - 1 helpbar = 28
	if h != 28 {
		t.Errorf("expected contentHeight 28, got %d", h)
	}
}

// --- Dropdown integration tests (via App) ---

func TestApp_DropdownOpenClose(t *testing.T) {
	app := testApp(t)

	// Press 1 to open Content dropdown
	m, _ := app.Update(keyRune('1'))
	a := m.(App)
	if !a.topBar.content.isOpen {
		t.Fatal("pressing 1 should open content dropdown")
	}

	// Esc closes it
	m, _ = a.Update(keyEsc)
	a = m.(App)
	if a.topBar.content.isOpen {
		t.Fatal("esc should close the dropdown")
	}
}

func TestApp_DropdownSelectChangesView(t *testing.T) {
	app := testApp(t)

	// Open Content, navigate to Agents, select
	m, _ := app.Update(keyRune('1'))
	m, _ = m.Update(keyDown) // cursor to Agents
	m, cmd := m.Update(keyEnter)

	// Execute the cmd to get the dropdownActiveMsg
	if cmd != nil {
		msg := cmd()
		m, _ = m.Update(msg)
	}

	a := m.(App)
	if a.topBar.content.ActiveLabel() != "Agents" {
		t.Errorf("expected Agents selected, got %q", a.topBar.content.ActiveLabel())
	}
}

func TestApp_DropdownMutualExclusion(t *testing.T) {
	app := testApp(t)

	// Open Collection, select Library
	m, _ := app.Update(keyRune('2'))
	m, cmd := m.Update(keyEnter) // select Library (cursor at 0)
	if cmd != nil {
		msg := cmd()
		m, _ = m.Update(msg)
	}

	a := m.(App)
	if a.topBar.collection.ActiveLabel() != "Library" {
		t.Errorf("expected Library, got %q", a.topBar.collection.ActiveLabel())
	}
	if !a.topBar.content.disabled {
		t.Error("content should be disabled after collection selection")
	}
	if a.topBar.content.ActiveLabel() != "--" {
		t.Errorf("content should be reset to --, got %q", a.topBar.content.ActiveLabel())
	}
}

func TestApp_CtrlCQuitsWithOpenDropdown(t *testing.T) {
	app := testApp(t)

	m, _ := app.Update(keyRune('1'))
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should quit even with dropdown open")
	}
}

func TestApp_TopBarShowsInView(t *testing.T) {
	app := testApp(t)
	view := app.View()
	assertContains(t, view, "Content: Skills")
	assertContains(t, view, "Collection: --")
}

// --- Golden tests ---

func TestGolden_Shell_60x20(t *testing.T) {
	app := testAppSize(t, 60, 20)
	requireGolden(t, "shell-empty-60x20", snapshotApp(t, app))
}

func TestGolden_Shell_80x30(t *testing.T) {
	app := testAppSize(t, 80, 30)
	requireGolden(t, "shell-empty-80x30", snapshotApp(t, app))
}

func TestGolden_Shell_120x40(t *testing.T) {
	app := testAppSize(t, 120, 40)
	requireGolden(t, "shell-empty-120x40", snapshotApp(t, app))
}

func TestGolden_Shell_TooSmall(t *testing.T) {
	app := testAppSize(t, 50, 15)
	requireGolden(t, "shell-toosmall-50x15", snapshotApp(t, app))
}

func TestGolden_Dropdown_Content_80x30(t *testing.T) {
	app := testAppSize(t, 80, 30)
	m, _ := app.Update(keyRune('1'))
	a := m.(App)
	requireGolden(t, "dropdown-content-80x30", snapshotApp(t, a))
}

func TestGolden_Dropdown_Collection_80x30(t *testing.T) {
	app := testAppSize(t, 80, 30)
	m, _ := app.Update(keyRune('2'))
	a := m.(App)
	requireGolden(t, "dropdown-collection-80x30", snapshotApp(t, a))
}
