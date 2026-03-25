package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

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
	app := testAppSize(t, 70, 15)
	view := app.View()
	assertContains(t, view, "Terminal too small")
	assertContains(t, view, "80x20")
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

func TestApp_EmptyLibraryTable(t *testing.T) {
	app := testApp(t)
	view := app.View()
	assertContains(t, view, "No content in library")
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
	// Use a wide enough terminal that hints fit on 1 line for the default tab
	app := testAppSize(t, 120, 30)
	h := app.contentHeight()
	expected := 30 - 5 - app.helpBar.Height()
	if h != expected {
		t.Errorf("expected contentHeight %d at 120 cols, got %d", expected, h)
	}
}

// --- Tab navigation tests ---

func TestApp_GroupSwitchWith123(t *testing.T) {
	app := testApp(t)

	// Press 2 to switch to Content
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
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

func TestApp_SubTabNavTab(t *testing.T) {
	app := testApp(t)

	// Tab moves to next sub-tab (Library -> Registries)
	m, cmd := app.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	if a.topBar.ActiveTabLabel() != "Registries" {
		t.Errorf("expected Registries after Tab, got %q", a.topBar.ActiveTabLabel())
	}

	// Shift+Tab moves back (Registries -> Library)
	m, cmd = a.Update(keyShiftTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)
	if a.topBar.ActiveTabLabel() != "Library" {
		t.Errorf("expected Library after Shift+Tab, got %q", a.topBar.ActiveTabLabel())
	}
}

func TestApp_ActionButtonHotkeys(t *testing.T) {
	app := testApp(t)

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
	assertContains(t, view, "╭")
	assertContains(t, view, "╰")
	assertContains(t, view, "├")
	assertContains(t, view, "┤")
}

// --- Library table tests ---

func TestApp_LibraryTableShowsItems(t *testing.T) {
	app := testAppWithItems(t)
	view := app.View()
	// Library table shows all items with column headers
	assertContains(t, view, "Name")
	assertContains(t, view, "Type")
	assertContains(t, view, "Scope")
	assertContains(t, view, "alpha-skill")
	assertContains(t, view, "gamma-rule")
}

func TestApp_LibraryTableShowsTypes(t *testing.T) {
	app := testAppWithItems(t)
	view := app.View()
	assertContains(t, view, "Skill")
	assertContains(t, view, "Rule")
	assertContains(t, view, "Agent")
}

func TestApp_LibraryJKNavigation(t *testing.T) {
	app := testAppWithItems(t)

	// First item selected by default
	sel := app.library.table.Selected()
	if sel == nil || sel.Name != "alpha-skill" {
		t.Fatalf("expected alpha-skill selected, got %v", sel)
	}

	// j moves down
	m, _ := app.Update(keyRune('j'))
	a := m.(App)
	sel = a.library.table.Selected()
	if sel == nil || sel.Name != "beta-skill" {
		t.Fatalf("expected beta-skill after j, got %v", sel)
	}

	// k moves up
	m, _ = a.Update(keyRune('k'))
	a = m.(App)
	sel = a.library.table.Selected()
	if sel == nil || sel.Name != "alpha-skill" {
		t.Fatalf("expected alpha-skill after k, got %v", sel)
	}
}

func TestApp_LibrarySearch(t *testing.T) {
	app := testAppWithItems(t)

	// / starts search
	m, _ := app.Update(keyRune('/'))
	a := m.(App)
	if !a.library.table.searching {
		t.Fatal("expected searching mode after /")
	}

	// Type "gamma"
	for _, ch := range "gamma" {
		m, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		a = m.(App)
	}
	if a.library.table.Len() != 1 {
		t.Errorf("expected 1 match for 'gamma', got %d", a.library.table.Len())
	}

	// Enter confirms search
	m, _ = a.Update(keyPress(tea.KeyEnter))
	a = m.(App)
	if a.library.table.searching {
		t.Fatal("expected search mode ended after Enter")
	}
	if a.library.table.Len() != 1 {
		t.Error("search filter should persist after confirm")
	}

	// Esc clears search
	m, _ = a.Update(keyPress(tea.KeyEsc))
	a = m.(App)
	if a.library.table.Len() != 7 {
		t.Errorf("expected all 7 items after Esc, got %d", a.library.table.Len())
	}
}

func TestApp_LibrarySort(t *testing.T) {
	app := testAppWithItems(t)

	// Default sort is by name ascending
	sel := app.library.table.Selected()
	if sel == nil || sel.Name != "alpha-skill" {
		t.Fatalf("expected alpha-skill first by default, got %v", sel)
	}

	// s cycles to sort by Type
	m, _ := app.Update(keyRune('s'))
	a := m.(App)
	if a.library.table.sortCol != sortByType {
		t.Errorf("expected sort by type, got %d", a.library.table.sortCol)
	}

	// S reverses sort direction
	m, _ = a.Update(keyRune('S'))
	a = m.(App)
	if a.library.table.sortAsc {
		t.Fatal("expected sort descending after S")
	}
}

func TestApp_LibraryDrillIn(t *testing.T) {
	app := testAppWithItems(t)

	// Enter drills into detail view
	m, _ := app.Update(keyPress(tea.KeyEnter))
	a := m.(App)
	if a.library.mode != libraryDetail {
		t.Fatal("expected libraryDetail mode after Enter")
	}
	if a.library.detailItem == nil || a.library.detailItem.Name != "alpha-skill" {
		t.Fatal("expected alpha-skill as detail item")
	}

	// Esc returns to browse mode
	m, _ = a.Update(keyPress(tea.KeyEsc))
	a = m.(App)
	if a.library.mode != libraryBrowse {
		t.Fatal("expected libraryBrowse mode after Esc")
	}
}

func TestApp_LibraryHintsChange(t *testing.T) {
	app := testAppWithItems(t)
	view := app.View()
	assertContains(t, view, "enter preview")

	// Drill in
	m, _ := app.Update(keyPress(tea.KeyEnter))
	a := m.(App)
	a.helpBar.SetHints(a.currentHints())
	// Rebuild view to check hints
	a2 := testAppWithItemsSize(t, 120, 40)
	m2, _ := a2.Update(keyPress(tea.KeyEnter))
	a2 = m2.(App)
	a2.helpBar.SetHints(a2.currentHints())
	hints := a2.currentHints()
	found := false
	for _, h := range hints {
		if h == "esc close" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'esc close' hint in detail mode, got %v", hints)
	}
}

// --- Content tab explorer tests ---

func TestApp_ContentTabFilters(t *testing.T) {
	app := testAppWithItems(t)

	// Switch to Content > Skills
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	view := a.View()
	assertContains(t, view, "alpha-skill")
	assertContains(t, view, "beta-skill")
	assertNotContains(t, view, "gamma-rule")
	assertNotContains(t, view, "delta-agent")
}

func TestApp_ContentTabNoTypeBadge(t *testing.T) {
	app := testAppWithItems(t)

	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	view := a.View()
	assertNotContains(t, view, "[skills]")
}

func TestApp_ExplorerHLSwitchesFocus(t *testing.T) {
	app := testAppWithItems(t)

	// Switch to Content > Skills (explorer view)
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)

	if a.explorer.focus != paneItems {
		t.Fatal("expected initial focus on items pane")
	}

	// l switches to preview
	m, _ = a.Update(keyRune('l'))
	a = m.(App)
	if a.explorer.focus != panePreview {
		t.Fatal("expected focus on preview pane after l")
	}

	// h switches back
	m, _ = a.Update(keyRune('h'))
	a = m.(App)
	if a.explorer.focus != paneItems {
		t.Fatal("expected focus back on items pane after h")
	}
}

func TestApp_ConfigTabPlaceholder(t *testing.T) {
	app := testApp(t)

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

	m, cmd := app.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	view := a.View()
	assertContains(t, view, "Registries view coming soon")
}

func TestApp_CursorWrapsInExplorer(t *testing.T) {
	app := testAppWithItems(t)

	// Switch to Content > Skills (explorer)
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)

	// k from first item should wrap to last
	m, _ = a.Update(keyRune('k'))
	a = m.(App)
	sel := a.explorer.items.Selected()
	if sel == nil || sel.Name != "beta-skill" {
		t.Fatalf("expected beta-skill after wrap, got %v", sel)
	}
}

func TestApp_HelpHintsContextSensitive(t *testing.T) {
	app := testAppWithItemsSize(t, 120, 40)
	view := app.View()
	assertContains(t, view, "navigate")
	assertContains(t, view, "enter preview")
	assertContains(t, view, "tab items")
}

// --- Items scroll indicator tests ---

func TestItemsModel_ScrollIndicators(t *testing.T) {
	items := make([]catalog.ContentItem, 20)
	for i := range items {
		items[i] = catalog.ContentItem{Name: fmt.Sprintf("item-%02d", i), Type: catalog.Skills}
	}
	m := newItemsModel(items, false)
	m.SetSize(30, 5)

	view := m.View()
	stripped := ansi.Strip(view)
	if strings.Contains(stripped, "more above") {
		t.Error("should not show 'above' at top")
	}
	if !strings.Contains(stripped, "more below") {
		t.Error("should show 'below' when items overflow")
	}

	m.offset = 5
	m.cursor = 5
	view = m.View()
	stripped = ansi.Strip(view)
	if !strings.Contains(stripped, "(5 more above)") {
		t.Errorf("expected '(5 more above)', got:\n%s", stripped)
	}
	if !strings.Contains(stripped, "more below") {
		t.Error("should show 'below' indicator")
	}

	m.offset = 15
	m.cursor = 19
	view = m.View()
	stripped = ansi.Strip(view)
	if !strings.Contains(stripped, "more above") {
		t.Error("should show 'above' at bottom")
	}
	if strings.Contains(stripped, "more below") {
		t.Error("should not show 'below' at bottom")
	}
}

func TestItemsModel_NoIndicatorsWhenFits(t *testing.T) {
	items := []catalog.ContentItem{
		{Name: "a", Type: catalog.Skills},
		{Name: "b", Type: catalog.Skills},
	}
	m := newItemsModel(items, false)
	m.SetSize(30, 10)

	view := m.View()
	stripped := ansi.Strip(view)
	if strings.Contains(stripped, "more above") || strings.Contains(stripped, "more below") {
		t.Error("should not show indicators when all items fit")
	}
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

// --- Preview scroll indicator tests ---

func TestPreview_ScrollIndicators(t *testing.T) {
	p := newPreviewModel()
	p.SetSize(40, 7)

	p.lines = make([]string, 20)
	for i := range p.lines {
		p.lines[i] = fmt.Sprintf("line %d content", i+1)
	}
	p.fileName = "test.md"

	view := p.View()
	stripped := ansi.Strip(view)
	if strings.Contains(stripped, "more above") {
		t.Error("should not show 'above' indicator at top")
	}
	if !strings.Contains(stripped, "more below") {
		t.Error("should show 'below' indicator when content overflows")
	}

	p.offset = 5
	view = p.View()
	stripped = ansi.Strip(view)
	if !strings.Contains(stripped, "more above") {
		t.Error("should show 'above' indicator when scrolled down")
	}
	if !strings.Contains(stripped, "more below") {
		t.Error("should show 'below' indicator when more content exists")
	}
	if !strings.Contains(stripped, "(5 more above)") {
		t.Errorf("expected '(5 more above)', got:\n%s", stripped)
	}

	p.offset = 14
	view = p.View()
	stripped = ansi.Strip(view)
	if !strings.Contains(stripped, "more above") {
		t.Error("should show 'above' indicator at bottom")
	}
	if strings.Contains(stripped, "more below") {
		t.Error("should not show 'below' indicator at bottom")
	}
}

func TestPreview_NoIndicatorsWhenContentFits(t *testing.T) {
	p := newPreviewModel()
	p.SetSize(40, 10)

	p.lines = []string{"line 1", "line 2", "line 3"}
	p.fileName = "short.md"

	view := p.View()
	stripped := ansi.Strip(view)
	if strings.Contains(stripped, "more above") || strings.Contains(stripped, "more below") {
		t.Error("should not show scroll indicators when all content fits")
	}
}

// --- Golden tests ---

func TestGolden_Shell_80x30(t *testing.T) {
	app := testAppSize(t, 80, 30)
	requireGolden(t, "shell-empty-80x30", snapshotApp(t, app))
}

func TestGolden_Shell_120x40(t *testing.T) {
	app := testAppSize(t, 120, 40)
	requireGolden(t, "shell-empty-120x40", snapshotApp(t, app))
}

func TestGolden_Shell_TooSmall(t *testing.T) {
	app := testAppSize(t, 70, 15)
	requireGolden(t, "shell-toosmall-70x15", snapshotApp(t, app))
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

func TestGolden_Library_WithItems_80x30(t *testing.T) {
	app := testAppWithItems(t)
	requireGolden(t, "library-table-80x30", snapshotApp(t, app))
}

func TestGolden_Library_WithItems_120x40(t *testing.T) {
	app := testAppWithItemsSize(t, 120, 40)
	requireGolden(t, "library-table-120x40", snapshotApp(t, app))
}

func TestGolden_Explorer_Skills_80x30(t *testing.T) {
	app := testAppWithItems(t)
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	requireGolden(t, "explorer-skills-80x30", snapshotApp(t, a))
}

// Verify unused catalog import is consumed
var _ = catalog.Skills
