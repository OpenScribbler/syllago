package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
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

func TestApp_EmptyExplorer(t *testing.T) {
	app := testApp(t)
	view := app.View()
	assertContains(t, view, "No items found")
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

// --- Explorer tests ---

func TestApp_ExplorerShowsItems(t *testing.T) {
	app := testAppWithItems(t)
	view := app.View()
	// Library view shows all items
	assertContains(t, view, "alpha-skill")
	assertContains(t, view, "gamma-rule")
	assertContains(t, view, "delta-agent")
}

func TestApp_ExplorerMixedTypeBadges(t *testing.T) {
	app := testAppWithItems(t)
	view := app.View()
	// Library view shows type badges since it's mixed
	assertContains(t, view, "[skills]")
	assertContains(t, view, "[rules]")
}

func TestApp_ContentTabFilters(t *testing.T) {
	app := testAppWithItems(t)

	// Switch to Content > Skills
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	view := a.View()

	// Should show only skills
	assertContains(t, view, "alpha-skill")
	assertContains(t, view, "beta-skill")
	// Should NOT show non-skills
	assertNotContains(t, view, "gamma-rule")
	assertNotContains(t, view, "delta-agent")
}

func TestApp_ContentTabNoTypeBadge(t *testing.T) {
	app := testAppWithItems(t)

	// Switch to Content > Skills
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	view := a.View()

	// Should NOT show type badges since items are all same type
	assertNotContains(t, view, "[skills]")
}

func TestApp_JKNavigation(t *testing.T) {
	app := testAppWithItems(t)

	// First item should be selected by default
	sel := app.explorer.items.Selected()
	if sel == nil || sel.Name != "alpha-skill" {
		t.Fatalf("expected alpha-skill selected, got %v", sel)
	}

	// j moves cursor down
	m, _ := app.Update(keyRune('j'))
	a := m.(App)
	sel = a.explorer.items.Selected()
	if sel == nil || sel.Name != "beta-skill" {
		t.Fatalf("expected beta-skill after j, got %v", sel)
	}

	// k moves cursor back up
	m, _ = a.Update(keyRune('k'))
	a = m.(App)
	sel = a.explorer.items.Selected()
	if sel == nil || sel.Name != "alpha-skill" {
		t.Fatalf("expected alpha-skill after k, got %v", sel)
	}
}

func TestApp_TabSwitchesFocus(t *testing.T) {
	app := testAppWithItems(t)

	if app.explorer.focus != paneItems {
		t.Fatal("expected initial focus on items pane")
	}

	// Tab switches to preview
	m, _ := app.Update(keyTab)
	a := m.(App)
	if a.explorer.focus != panePreview {
		t.Fatal("expected focus on preview pane after tab")
	}

	// Tab switches back to items
	m, _ = a.Update(keyTab)
	a = m.(App)
	if a.explorer.focus != paneItems {
		t.Fatal("expected focus back on items pane after second tab")
	}
}

func TestApp_ConfigTabPlaceholder(t *testing.T) {
	app := testApp(t)

	// Switch to Config
	m, cmd := app.Update(keyRune('3'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	view := a.View()
	assertContains(t, view, "Settings view coming soon")
}

func TestApp_RegistriesPlaceholder(t *testing.T) {
	app := testApp(t)

	// Navigate to Collections > Registries
	m, cmd := app.Update(keyRune('l'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	view := a.View()
	assertContains(t, view, "Registries view coming soon")
}

func TestApp_CursorWraps(t *testing.T) {
	app := testAppWithItems(t)

	// k from first item should wrap to last
	m, _ := app.Update(keyRune('k'))
	a := m.(App)
	sel := a.explorer.items.Selected()
	if sel == nil || sel.Name != "eta-command" {
		t.Fatalf("expected eta-command after wrap, got %v", sel)
	}
}

func TestApp_HelpHintsContextSensitive(t *testing.T) {
	app := testAppWithItems(t)
	view := app.View()
	assertContains(t, view, "j/k navigate")
	assertContains(t, view, "tab focus")
}

// --- Items search tests ---

func TestItemsModel_Search(t *testing.T) {
	items := []catalog.ContentItem{
		{Name: "alpha-skill", Type: catalog.Skills},
		{Name: "beta-skill", Type: catalog.Skills},
		{Name: "gamma-rule", Type: catalog.Rules},
	}
	m := newItemsModel(items, true)
	m.SetSize(40, 10)

	m.ApplySearch("alpha")
	if m.Len() != 1 {
		t.Errorf("expected 1 match, got %d", m.Len())
	}
	if m.Selected().Name != "alpha-skill" {
		t.Errorf("expected alpha-skill, got %q", m.Selected().Name)
	}

	m.ApplySearch("skill")
	if m.Len() != 2 {
		t.Errorf("expected 2 matches for 'skill', got %d", m.Len())
	}

	m.ClearSearch()
	if m.Len() != 3 {
		t.Errorf("expected 3 items after clear, got %d", m.Len())
	}
}

func TestItemsModel_SearchNoResults(t *testing.T) {
	items := []catalog.ContentItem{
		{Name: "alpha-skill", Type: catalog.Skills},
	}
	m := newItemsModel(items, false)
	m.SetSize(40, 10)

	m.ApplySearch("zzz")
	if m.Len() != 0 {
		t.Errorf("expected 0 matches, got %d", m.Len())
	}
	if m.Selected() != nil {
		t.Error("expected nil selection with no results")
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

func TestGolden_Content_80x30(t *testing.T) {
	app := testApp(t)
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	requireGolden(t, "content-80x30", snapshotApp(t, a))
}

func TestGolden_Explorer_WithItems_80x30(t *testing.T) {
	app := testAppWithItems(t)
	requireGolden(t, "explorer-items-80x30", snapshotApp(t, app))
}

func TestGolden_Explorer_WithItems_120x40(t *testing.T) {
	app := testAppWithItemsSize(t, 120, 40)
	requireGolden(t, "explorer-items-120x40", snapshotApp(t, app))
}

func TestGolden_Explorer_Skills_80x30(t *testing.T) {
	app := testAppWithItems(t)
	// Switch to Content > Skills
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	requireGolden(t, "explorer-skills-80x30", snapshotApp(t, a))
}

// Verify unused catalog import is consumed
var _ = catalog.Skills
