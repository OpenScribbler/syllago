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
