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
	assertContains(t, view, "syllago")
}

func TestApp_ContentHeight(t *testing.T) {
	app := testAppSize(t, 80, 30)
	h := app.contentHeight()
	// 30 total - 5 topbar - 1 helpbar = 24
	if h != 24 {
		t.Errorf("expected contentHeight 24, got %d", h)
	}
}

// --- Tab navigation tests ---

func TestApp_GroupSwitchWith123(t *testing.T) {
	app := testApp(t)

	// Press 2 to switch to Content
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd()) // process tabChangedMsg
	}
	a := m.(App)
	if a.topBar.ActiveGroupLabel() != "Content" {
		t.Errorf("expected Content, got %q", a.topBar.ActiveGroupLabel())
	}
	if a.topBar.ActiveTabLabel() != "Skills" {
		t.Errorf("expected Skills as first tab, got %q", a.topBar.ActiveTabLabel())
	}

	// Press 3 to switch to Config
	m, cmd = a.Update(keyRune('3'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)
	if a.topBar.ActiveGroupLabel() != "Config" {
		t.Errorf("expected Config, got %q", a.topBar.ActiveGroupLabel())
	}
}

func TestApp_SubTabNavHL(t *testing.T) {
	app := testApp(t)

	// l moves to next sub-tab (Library -> Registries)
	m, cmd := app.Update(keyRune('l'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	if a.topBar.ActiveTabLabel() != "Registries" {
		t.Errorf("expected Registries after l, got %q", a.topBar.ActiveTabLabel())
	}

	// h moves back (Registries -> Library)
	m, cmd = a.Update(keyRune('h'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)
	if a.topBar.ActiveTabLabel() != "Library" {
		t.Errorf("expected Library after h, got %q", a.topBar.ActiveTabLabel())
	}
}

func TestApp_ActionButtonHotkeys(t *testing.T) {
	app := testApp(t)

	// 'a' should fire an add action
	_, cmd := app.Update(keyRune('a'))
	if cmd == nil {
		t.Fatal("'a' should produce an action command")
	}
	msg := cmd()
	action, ok := msg.(actionPressedMsg)
	if !ok {
		t.Fatalf("expected actionPressedMsg, got %T", msg)
	}
	if action.action != "add" {
		t.Errorf("expected action=add, got %q", action.action)
	}
	if action.tab != "Library" {
		t.Errorf("expected tab=Library, got %q", action.tab)
	}

	// 'n' should fire a create action
	_, cmd = app.Update(keyRune('n'))
	if cmd == nil {
		t.Fatal("'n' should produce an action command")
	}
	msg = cmd()
	action = msg.(actionPressedMsg)
	if action.action != "create" {
		t.Errorf("expected action=create, got %q", action.action)
	}
}

func TestApp_TopBarShowsInView(t *testing.T) {
	app := testApp(t)
	view := app.View()
	assertContains(t, view, "Collections")
	assertContains(t, view, "Content")
	assertContains(t, view, "Config")
	assertContains(t, view, "Library")
}

func TestApp_TopBarBorder(t *testing.T) {
	app := testApp(t)
	view := app.View()
	// Should have rounded corners
	assertContains(t, view, "╭")
	assertContains(t, view, "╰")
	// Should have separator
	assertContains(t, view, "├")
	assertContains(t, view, "┤")
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

func TestGolden_Content_80x30(t *testing.T) {
	app := testApp(t)
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	requireGolden(t, "content-80x30", snapshotApp(t, a))
}
