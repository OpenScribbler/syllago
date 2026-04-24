package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
)

// --- Unit tests ---

func TestApp_WindowSizeMsg(t *testing.T) {
	app := NewApp(testCatalog(t), testProviders(), "0.0.0-test", false, nil, testConfig(), false, "", "")
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
	app := NewApp(testCatalog(t), testProviders(), "0.0.0-test", false, nil, testConfig(), false, "", "")
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
	expected := 30 - 6 - app.helpBar.Height()
	if h != expected {
		t.Errorf("expected contentHeight %d at 120 cols, got %d", expected, h)
	}
}

func TestApp_ContentHeightAdjustsOnTabSwitch(t *testing.T) {
	// At 80 cols, Config has fewer hints (1 line) and Content has more (2 lines).
	// Switching between them must resize content models to avoid clipping.
	app := testAppWithItems(t) // 80x30

	// Go to Config (group 3) — fewer hints
	m, cmd := app.Update(keyRune('3'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a3 := m.(App)
	configHelpH := a3.helpBar.Height()
	configContentH := a3.contentHeight()

	// Go to Content (group 2) — more hints
	m, cmd = a3.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a2 := m.(App)
	contentHelpH := a2.helpBar.Height()
	contentContentH := a2.contentHeight()

	// If help bar grew, content height must have shrunk
	if contentHelpH > configHelpH {
		if contentContentH >= configContentH {
			t.Errorf("content height should shrink when help bar grows: config=%d, content=%d (help: %d→%d)",
				configContentH, contentContentH, configHelpH, contentHelpH)
		}
	}

	// The explorer model should have been resized to the new content height
	if a2.explorer.height != contentContentH {
		t.Errorf("explorer height %d should match contentHeight %d after tab switch",
			a2.explorer.height, contentContentH)
	}

	// Total rendered output should fit within terminal height
	view := a2.View()
	lines := strings.Split(view, "\n")
	if len(lines) > 30 {
		t.Errorf("rendered output has %d lines, exceeds terminal height 30", len(lines))
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

	// 'a' on Library tab now opens the add wizard directly
	m, _ := app.Update(keyRune('a'))
	a := m.(App)
	if a.wizardMode != wizardAdd {
		t.Fatal("'a' on Library should open add wizard")
	}

	// 'n' (create) is deferred — should be a no-op
	app2 := testApp(t)
	_, cmd := app2.Update(keyRune('n'))
	if cmd != nil {
		t.Fatal("'n' should be a no-op (create is deferred)")
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

func TestApp_ExplorerSearch(t *testing.T) {
	app := testAppWithItems(t)

	// Switch to Content group (Skills tab)
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)

	// Verify we're on Content tab with explorer
	if !a.isContentTab() {
		t.Fatal("expected Content tab after pressing 2")
	}
	initialLen := a.explorer.items.Len()
	if initialLen == 0 {
		t.Fatal("expected items in explorer")
	}

	// / starts search
	m, _ = a.Update(keyRune('/'))
	a = m.(App)
	if !a.explorer.searching {
		t.Fatal("expected searching mode after /")
	}

	// Type "alpha"
	for _, ch := range "alpha" {
		m, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		a = m.(App)
	}
	if a.explorer.items.Len() >= initialLen {
		t.Errorf("expected filtered items, got %d (same as initial %d)", a.explorer.items.Len(), initialLen)
	}
	if a.explorer.items.Len() == 0 {
		t.Error("expected at least one match for 'alpha'")
	}

	// Enter confirms search
	m, _ = a.Update(keyPress(tea.KeyEnter))
	a = m.(App)
	if a.explorer.searching {
		t.Fatal("expected search mode ended after Enter")
	}
	if a.explorer.searchQuery != "alpha" {
		t.Error("search query should persist after confirm")
	}

	// Esc clears search
	m, _ = a.Update(keyPress(tea.KeyEsc))
	a = m.(App)
	if a.explorer.items.Len() != initialLen {
		t.Errorf("expected all %d items after Esc, got %d", initialLen, a.explorer.items.Len())
	}
}

func TestApp_ExplorerSearchSuppressesGlobalKeys(t *testing.T) {
	app := testAppWithItems(t)

	// Switch to Content tab
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)

	// Start search
	m, _ = a.Update(keyRune('/'))
	a = m.(App)
	if !a.explorer.searching {
		t.Fatal("expected searching mode")
	}

	// Typing 'a' should go to search, not trigger Add
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	a = m.(App)
	if a.explorer.searchQuery != "a" {
		t.Errorf("expected search query 'a', got %q", a.explorer.searchQuery)
	}
	// '1' should go to search, not switch to group 1
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	a = m.(App)
	if a.explorer.searchQuery != "a1" {
		t.Errorf("expected search query 'a1', got %q", a.explorer.searchQuery)
	}
}

func TestApp_ExplorerSearchBarInView(t *testing.T) {
	app := testAppWithItems(t)

	// Switch to Content tab
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)

	// No search bar initially
	view := a.View()
	stripped := ansi.Strip(view)
	if strings.Contains(stripped, "esc cancel") {
		t.Error("search bar hints should not appear before search starts")
	}

	// Start search
	m, _ = a.Update(keyRune('/'))
	a = m.(App)
	view = a.View()
	stripped = ansi.Strip(view)
	if !strings.Contains(stripped, "esc cancel") {
		t.Error("search bar should show 'esc cancel' when searching")
	}
}

func TestApp_GallerySearch(t *testing.T) {
	app := testAppWithItems(t)

	// Switch to Collections > Loadouts (tab past Library)
	m, _ := app.Update(keyPress(tea.KeyTab))
	a := m.(App)
	if !a.isGalleryTab() {
		t.Fatal("expected gallery tab after Tab")
	}

	// Gallery might be empty in test — that's OK, verify search mechanics work
	// / starts search
	m, _ = a.Update(keyRune('/'))
	a = m.(App)
	if !a.gallery.searching {
		t.Fatal("expected gallery searching mode after /")
	}

	// Enter confirms
	m, _ = a.Update(keyPress(tea.KeyEnter))
	a = m.(App)
	if a.gallery.searching {
		t.Fatal("expected gallery search mode ended after Enter")
	}

	// Start search again, then Esc cancels
	m, _ = a.Update(keyRune('/'))
	a = m.(App)
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	a = m.(App)
	if a.gallery.searchQuery != "x" {
		t.Errorf("expected query 'x', got %q", a.gallery.searchQuery)
	}
	m, _ = a.Update(keyPress(tea.KeyEsc))
	a = m.(App)
	if a.gallery.searchQuery != "" {
		t.Error("expected empty query after Esc")
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

func TestApp_ConfigTabSettings(t *testing.T) {
	app := testApp(t)

	m, cmd := app.Update(keyRune('3'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	view := a.View()
	assertContains(t, view, "Configured Registries")
}

func TestApp_RegistriesGallery(t *testing.T) {
	app := testApp(t)

	// Tab from Library to Registries
	m, cmd := app.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	view := a.View()
	// Gallery renders with "No items found" when no registries are configured
	assertContains(t, view, "No items found")
	// Should show gallery hints
	assertContains(t, view, "arrows grid")
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

// --- Modal integration tests ---

func TestApp_EditOpensModal(t *testing.T) {
	app := testAppWithItems(t)

	// Press 'e' to open edit modal
	m, _ := app.Update(keyRune('e'))
	a := m.(App)
	if !a.modal.active {
		t.Fatal("expected modal to be active after 'e'")
	}
	if a.modal.title == "" {
		t.Fatal("expected modal title to be set")
	}
	// Modal name should be pre-filled with the selected item's display name
	if a.modal.name == "" {
		t.Fatal("expected modal name to be pre-filled")
	}
}

func TestApp_ModalCapturesKeys(t *testing.T) {
	app := testAppWithItems(t)

	// Open modal
	m, _ := app.Update(keyRune('e'))
	a := m.(App)

	// 'q' should NOT quit when modal is active (it should be typed into the input)
	m, cmd := a.Update(keyRune('q'))
	if cmd != nil {
		msg := cmd()
		if msg == tea.Quit() {
			t.Fatal("'q' should not quit when modal is active")
		}
	}
	a = m.(App)
	if !a.modal.active {
		t.Fatal("modal should still be active after typing 'q'")
	}
}

func TestApp_ModalEscCancels(t *testing.T) {
	app := testAppWithItems(t)

	// Open modal
	m, _ := app.Update(keyRune('e'))
	a := m.(App)

	// Esc closes modal
	m, cmd := a.Update(keyPress(tea.KeyEsc))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)
	if a.modal.active {
		t.Fatal("modal should be inactive after Esc")
	}
}

func TestApp_ModalViewOverlay(t *testing.T) {
	app := testAppWithItems(t)

	// Open modal
	m, _ := app.Update(keyRune('e'))
	a := m.(App)

	view := a.View()
	stripped := ansi.Strip(view)
	// Modal should be visible in the view
	if !strings.Contains(stripped, "Cancel") {
		t.Error("expected Cancel button visible in modal overlay")
	}
	if !strings.Contains(stripped, "Save") {
		t.Error("expected Save button visible in modal overlay")
	}
}

func TestApp_EditHintVisible(t *testing.T) {
	app := testAppWithItemsSize(t, 120, 40)
	view := app.View()
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "e edit") {
		t.Error("expected 'e edit' hint in Library view")
	}
}

// --- Registry wiring tests ---

// testAppOnRegistries creates a test app navigated to the Registries tab.
// It includes a registry source so the gallery tab has content.
func testAppOnRegistries(t *testing.T) App {
	t.Helper()
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "alpha-skill", Type: catalog.Skills, Source: "my-registry", Registry: "my-registry", Files: []string{"SKILL.md"}},
		},
	}
	app := NewApp(cat, nil, "0.0.0-test", false, []catalog.RegistrySource{
		{Name: "my-registry", Path: "/tmp/fake-registry"},
	}, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	a := m.(App)

	// Navigate to Registries (one Tab from Library)
	m, cmd := a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	return m.(App)
}

func TestApp_RegistryWiring_IsRegistriesTab(t *testing.T) {
	a := testAppOnRegistries(t)

	if !a.isRegistriesTab() {
		t.Fatal("expected isRegistriesTab() == true on Registries tab")
	}

	// Navigate back to Library
	m, cmd := a.Update(keyRune('1'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	if a.isRegistriesTab() {
		t.Fatal("expected isRegistriesTab() == false on Library tab")
	}
}

func TestApp_RegistryWiring_AddOpensModal(t *testing.T) {
	a := testAppOnRegistries(t)

	// Press 'a' to open registry add modal
	m, cmd := a.Update(keyRune('a'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	if !a.registryAdd.active {
		t.Fatal("expected registryAdd.active == true after pressing 'a' on Registries tab")
	}
}

func TestApp_RegistryWiring_AddNoOpOnLibrary(t *testing.T) {
	app := testApp(t)

	// Press 'a' on Library tab
	m, cmd := app.Update(keyRune('a'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)

	if a.registryAdd.active {
		t.Fatal("expected registryAdd.active == false on Library tab")
	}
}

func TestApp_RegistryWiring_AddNoOpOnLoadouts(t *testing.T) {
	app := testApp(t)

	// Navigate to Loadouts (two Tabs from Library)
	m, cmd := app.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	m, cmd = a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	if a.topBar.ActiveTabLabel() != "Loadouts" {
		t.Fatalf("expected Loadouts tab, got %q", a.topBar.ActiveTabLabel())
	}

	// Press 'a'
	m, cmd = a.Update(keyRune('a'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	if a.registryAdd.active {
		t.Fatal("expected registryAdd.active == false on Loadouts tab")
	}
}

func TestApp_RegistryWiring_AddBlockedByOpInProgress(t *testing.T) {
	a := testAppOnRegistries(t)

	// Simulate an in-progress registry operation
	a.registryOpInProgress = true

	// Press 'a'
	m, cmd := a.Update(keyRune('a'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	if a.registryAdd.active {
		t.Fatal("expected registryAdd.active == false when registryOpInProgress is true")
	}
}

func TestApp_RegistryWiring_SyncKeyNoOpOnLibrary(t *testing.T) {
	app := testApp(t)

	// Press 'S' on Library tab — should not panic or crash
	m, _ := app.Update(keyRune('S'))
	a := m.(App)

	// Verify app is still functional (no crash, stays on Library)
	if !a.isLibraryTab() {
		t.Fatal("expected to stay on Library tab after pressing S")
	}
}

func TestApp_RegistryWiring_ModalCapturesKeys(t *testing.T) {
	a := testAppOnRegistries(t)

	// Open registry add modal
	a.registryAdd.Open(nil, testConfig())

	// Press 'q' — should NOT quit, modal should stay open
	m, cmd := a.Update(keyRune('q'))
	if cmd != nil {
		// If cmd produces tea.Quit, that's wrong
		m, _ = m.Update(cmd())
	}
	a = m.(App)
	if !a.registryAdd.active {
		t.Fatal("modal should still be active after pressing 'q'")
	}

	// Press '1' — should NOT switch groups
	m, cmd = a.Update(keyRune('1'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)
	if !a.registryAdd.active {
		t.Fatal("modal should still be active after pressing '1'")
	}

	// Press 'R' — should NOT refresh catalog
	m, cmd = a.Update(keyRune('R'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)
	if !a.registryAdd.active {
		t.Fatal("modal should still be active after pressing 'R'")
	}
}

func TestApp_RegistryWiring_ModalPassesCtrlC(t *testing.T) {
	a := testAppOnRegistries(t)

	// Open registry add modal
	a.registryAdd.Open(nil, testConfig())

	// Press Ctrl+C — should produce tea.Quit
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("Ctrl+C should produce a quit command even when modal is active")
	}
}

func TestApp_RegistryWiring_WindowSizeMsg(t *testing.T) {
	cat := &catalog.Catalog{}
	app := NewApp(cat, nil, "0.0.0-test", false, nil, testConfig(), false, "", "")

	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a := m.(App)

	if a.registryAdd.width != 120 {
		t.Errorf("expected registryAdd.width == 120, got %d", a.registryAdd.width)
	}
	expectedHeight := a.contentHeight()
	if a.registryAdd.height != expectedHeight {
		t.Errorf("expected registryAdd.height == %d, got %d", expectedHeight, a.registryAdd.height)
	}
}

func TestApp_RegistryWiring_OverlayRendered(t *testing.T) {
	a := testAppOnRegistries(t)

	// Open registry add modal
	a.registryAdd.Open(nil, testConfig())

	view := a.View()
	assertContains(t, view, "Add Registry")
}

// --- Registry action handler tests ---

// testAppOnRegistriesWithConfig creates a test app navigated to Registries with a custom config.
func testAppOnRegistriesWithConfig(t *testing.T, cfg *config.Config) App {
	t.Helper()
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "alpha-skill", Type: catalog.Skills, Source: "my-registry", Registry: "my-registry", Files: []string{"SKILL.md"}},
		},
	}
	app := NewApp(cat, nil, "0.0.0-test", false, []catalog.RegistrySource{
		{Name: "my-registry", Path: "/tmp/fake-registry"},
	}, cfg, false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	a := m.(App)

	// Navigate to Registries (one Tab from Library)
	m, cmd := a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	return m.(App)
}

func TestApp_ActionHandler_AddOpensModalWithNames(t *testing.T) {
	cfg := &config.Config{
		Registries: []config.Registry{{Name: "existing-reg", URL: "https://example.com/existing"}},
	}
	a := testAppOnRegistriesWithConfig(t, cfg)

	m, _ := a.handleRegistryAdd()
	a = m.(App)

	if !a.registryAdd.active {
		t.Fatal("expected registryAdd.active == true after handleRegistryAdd()")
	}
	if len(a.registryAdd.existingNames) == 0 {
		t.Fatal("expected existingNames to be populated")
	}
	found := false
	for _, name := range a.registryAdd.existingNames {
		if name == "existing-reg" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected existingNames to contain 'existing-reg', got %v", a.registryAdd.existingNames)
	}
}

func TestApp_ActionHandler_AddResultSetsFlag(t *testing.T) {
	a := testAppOnRegistries(t)

	msg := registryAddMsg{url: "https://github.com/acme/tools", name: "acme/tools"}
	m, _ := a.Update(msg)
	a = m.(App)

	if !a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == true after registryAddMsg")
	}
}

func TestApp_ActionHandler_AddDoneSuccess(t *testing.T) {
	a := testAppOnRegistries(t)
	a.registryOpInProgress = true

	m, _ := a.Update(registryAddDoneMsg{name: "acme/tools"})
	a = m.(App)

	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false after successful registryAddDoneMsg")
	}
}

func TestApp_ActionHandler_AddDoneError(t *testing.T) {
	a := testAppOnRegistries(t)
	a.registryOpInProgress = true

	m, _ := a.Update(registryAddDoneMsg{name: "acme/tools", err: fmt.Errorf("clone failed")})
	a = m.(App)

	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false after failed registryAddDoneMsg (flag cleared even on error)")
	}
}

func TestApp_ActionHandler_SyncWithCard(t *testing.T) {
	a := testAppOnRegistries(t)

	m, cmd := a.handleSync()
	a = m.(App)

	if !a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == true after handleSync() with a selected card")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd from handleSync() when a card is selected")
	}
}

func TestApp_ActionHandler_SyncNilCard(t *testing.T) {
	// Create app with no registry sources (empty gallery)
	app := NewApp(&catalog.Catalog{}, nil, "0.0.0-test", false, nil, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	a := m.(App)

	// Navigate to Registries
	m, cmd := a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	m, cmd = a.handleSync()
	a = m.(App)

	if cmd != nil {
		t.Fatal("expected nil cmd from handleSync() when no cards exist")
	}
	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false when no card is selected")
	}
}

func TestApp_ActionHandler_SyncDoneSuccess(t *testing.T) {
	a := testAppOnRegistries(t)
	a.registryOpInProgress = true

	m, _ := a.Update(registrySyncDoneMsg{name: "my-reg"})
	a = m.(App)

	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false after successful registrySyncDoneMsg")
	}
}

func TestApp_ActionHandler_SyncDoneError(t *testing.T) {
	a := testAppOnRegistries(t)
	a.registryOpInProgress = true

	m, _ := a.Update(registrySyncDoneMsg{name: "my-reg", err: fmt.Errorf("pull failed")})
	a = m.(App)

	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false after failed registrySyncDoneMsg")
	}
}

func TestApp_ActionHandler_RemoveRegistryOpensConfirm(t *testing.T) {
	a := testAppOnRegistries(t)

	m, _ := a.handleRemove()
	a = m.(App)

	if !a.confirm.active {
		t.Fatal("expected confirm.active == true after handleRemove() on Registries tab")
	}
	// The confirm title should reference registry removal.
	view := a.confirm.View()
	stripped := strings.TrimSpace(view)
	if !strings.Contains(stripped, "Remove registry") && !strings.Contains(stripped, "remove") {
		t.Errorf("expected confirm dialog to mention 'Remove registry', got: %s", stripped)
	}
}

func TestApp_ActionHandler_RemoveRegistryBlockedByOp(t *testing.T) {
	a := testAppOnRegistries(t)
	a.registryOpInProgress = true

	m, _ := a.handleRemove()
	a = m.(App)

	if a.confirm.active {
		t.Fatal("expected confirm.active == false when registryOpInProgress is true (remove should be blocked)")
	}
}

func TestApp_ActionHandler_ConfirmRegistryRemoveDispatches(t *testing.T) {
	a := testAppOnRegistries(t)

	msg := confirmResultMsg{
		confirmed: true,
		itemName:  "my-reg",
		item:      catalog.ContentItem{}, // empty path signals registry (not loadout)
	}
	m, _ := a.Update(msg)
	a = m.(App)

	if !a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == true after confirmed registry remove dispatch")
	}
}

func TestApp_ActionHandler_ConfirmLoadoutStillWorks(t *testing.T) {
	// Create a fresh app on Loadouts (not Registries).
	app := NewApp(&catalog.Catalog{}, nil, "0.0.0-test", false, nil, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	a := m.(App)

	// Tab twice to get to Loadouts (Library -> Registries -> Loadouts)
	for i := 0; i < 2; i++ {
		m2, cmd := a.Update(keyTab)
		if cmd != nil {
			m2, _ = m2.Update(cmd())
		}
		a = m2.(App)
	}

	msg := confirmResultMsg{
		confirmed: true,
		item:      catalog.ContentItem{Type: catalog.Loadouts, Path: "/tmp/fake"},
		itemName:  "my-loadout",
	}

	// Should not panic — existing loadout remove logic should still work.
	m2, cmd := a.Update(msg)
	_ = m2.(App)
	// The loadout branch returns a cmd (doSimpleRemoveCmd), so cmd should be non-nil.
	if cmd == nil {
		t.Fatal("expected non-nil cmd from loadout confirm remove (existing logic should still work)")
	}
}

func TestApp_ActionHandler_RemoveDoneSuccess(t *testing.T) {
	a := testAppOnRegistries(t)
	a.registryOpInProgress = true

	m, _ := a.Update(registryRemoveDoneMsg{name: "old-reg"})
	a = m.(App)

	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false after successful registryRemoveDoneMsg")
	}
}

func TestApp_ActionHandler_RemoveDoneError(t *testing.T) {
	a := testAppOnRegistries(t)
	a.registryOpInProgress = true

	m, _ := a.Update(registryRemoveDoneMsg{name: "old-reg", err: fmt.Errorf("permission denied")})
	a = m.(App)

	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false after failed registryRemoveDoneMsg (flag cleared even on error)")
	}
}

// --- Gallery registry buttons tests ---

func TestGallery_RegistryButtons_Visible(t *testing.T) {
	a := testAppOnRegistries(t)
	view := a.View()

	assertContains(t, view, "[a] Add")
	assertContains(t, view, "[S] Sync")
}

func TestGallery_RegistryButtons_NotOnLoadouts(t *testing.T) {
	// Create app with loadout items for the Loadouts tab
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "python-web", DisplayName: "Python-Web", Type: catalog.Loadouts, Source: "project", Files: []string{"loadout.yaml"}},
		},
	}
	app := NewApp(cat, nil, "0.0.0-test", false, nil, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	a := m.(App)

	// Navigate to Loadouts (two Tabs from Library)
	m, cmd := a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	m, cmd = m.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	if a.topBar.ActiveTabLabel() != "Loadouts" {
		t.Fatalf("expected Loadouts tab, got %q", a.topBar.ActiveTabLabel())
	}

	view := a.View()
	assertContains(t, view, "[a] Add loadout")     // loadouts now have their own add button
	assertNotContains(t, view, "[a] Add registry") // registry add should NOT appear on loadouts
	assertNotContains(t, view, "[S] Sync")
}

func TestGallery_RegistryButtons_StillHasRemoveEdit(t *testing.T) {
	a := testAppOnRegistries(t)
	view := a.View()

	assertContains(t, view, "[d] Remove")
	assertContains(t, view, "[e] Edit")
}

func TestApp_RegistryHints_ContainsSync(t *testing.T) {
	a := testAppOnRegistries(t)
	hints := a.currentHints()

	found := false
	for _, h := range hints {
		if h == "S sync" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'S sync' in hints, got %v", hints)
	}
}

func TestApp_RegistryHints_ContainsAdd(t *testing.T) {
	a := testAppOnRegistries(t)
	hints := a.currentHints()

	found := false
	for _, h := range hints {
		if h == "a add" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'a add' in hints, got %v", hints)
	}
}

func TestApp_RegistryHints_NotOnLoadouts(t *testing.T) {
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "python-web", DisplayName: "Python-Web", Type: catalog.Loadouts, Source: "project", Files: []string{"loadout.yaml"}},
		},
	}
	app := NewApp(cat, nil, "0.0.0-test", false, nil, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	a := m.(App)

	// Navigate to Loadouts (two Tabs from Library)
	m, cmd := a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	m, cmd = m.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	if a.topBar.ActiveTabLabel() != "Loadouts" {
		t.Fatalf("expected Loadouts tab, got %q", a.topBar.ActiveTabLabel())
	}

	hints := a.currentHints()
	for _, h := range hints {
		if h == "S sync" {
			t.Error("expected 'S sync' NOT in Loadouts hints")
		}
	}
}

func TestApp_RegistryHints_Order(t *testing.T) {
	a := testAppOnRegistries(t)
	hints := a.currentHints()

	addIdx, syncIdx, removeIdx := -1, -1, -1
	for i, h := range hints {
		switch h {
		case "a add":
			addIdx = i
		case "S sync":
			syncIdx = i
		case "d remove":
			removeIdx = i
		}
	}

	if addIdx == -1 {
		t.Fatal("'a add' not found in hints")
	}
	if syncIdx == -1 {
		t.Fatal("'S sync' not found in hints")
	}
	if removeIdx == -1 {
		t.Fatal("'d remove' not found in hints")
	}

	if addIdx >= syncIdx {
		t.Errorf("expected 'a add' (idx %d) before 'S sync' (idx %d)", addIdx, syncIdx)
	}
	if syncIdx >= removeIdx {
		t.Errorf("expected 'S sync' (idx %d) before 'd remove' (idx %d)", syncIdx, removeIdx)
	}
}

// --- Integration tests: Registry management flows ---

func TestIntegration_Registry_AddFlow(t *testing.T) {
	a := testAppOnRegistries(t)

	// Press 'a' to open registry add modal
	m, cmd := a.Update(keyRune('a'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	if !a.registryAdd.active {
		t.Fatal("expected registry add modal to be active after pressing 'a'")
	}

	// Set URL and name fields directly on the modal
	a.registryAdd.urlValue = "https://github.com/acme/tools"
	a.registryAdd.nameValue = "acme/tools"
	a.registryAdd.nameManuallySet = true

	// Set focusIdx to 5 (Add button) and press Enter to submit
	a.registryAdd.focusIdx = 5
	m, cmd = a.Update(keyPress(tea.KeyEnter))
	a = m.(App)

	if a.registryAdd.active {
		t.Fatal("expected registry add modal to be closed after submit")
	}

	// Process the cmd (registryAddMsg) to trigger handleRegistryAddResult
	if cmd != nil {
		m, _ = m.Update(cmd())
		a = m.(App)
	}

	if !a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == true after add flow")
	}
}

func TestIntegration_Registry_AddModalEscape(t *testing.T) {
	a := testAppOnRegistries(t)

	// Open add modal
	m, cmd := a.Update(keyRune('a'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	if !a.registryAdd.active {
		t.Fatal("expected modal to be active")
	}

	// Press Esc
	m, _ = a.Update(keyPress(tea.KeyEsc))
	a = m.(App)

	if a.registryAdd.active {
		t.Fatal("expected modal to be closed after Esc")
	}
	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false after Esc (no op started)")
	}
}

func TestIntegration_Registry_AddValidationError(t *testing.T) {
	a := testAppOnRegistries(t)

	// Open add modal
	m, cmd := a.Update(keyRune('a'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	// Focus on Add button with empty URL
	a.registryAdd.focusIdx = 5
	m, _ = a.Update(keyPress(tea.KeyEnter))
	a = m.(App)

	// Modal should stay open with validation error
	if !a.registryAdd.active {
		t.Fatal("expected modal to stay open on validation error")
	}

	view := a.View()
	assertContains(t, view, "URL is required")
}

func TestIntegration_Registry_SyncFlow(t *testing.T) {
	a := testAppOnRegistries(t)

	// Press 'S' to sync
	m, _ := a.Update(keyRune('S'))
	a = m.(App)

	if !a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == true after pressing S on Registries")
	}
}

func TestIntegration_Registry_RemoveFlow(t *testing.T) {
	a := testAppOnRegistries(t)

	// Press 'd' to remove
	m, _ := a.Update(keyRune('d'))
	a = m.(App)

	if !a.confirm.active {
		t.Fatal("expected confirm modal to open after pressing 'd'")
	}

	stripped := strings.TrimSpace(a.confirm.title)
	if !strings.Contains(stripped, "Remove registry") {
		t.Errorf("expected confirm title to contain 'Remove registry', got %q", stripped)
	}
}

func TestIntegration_Registry_RemoveCancel(t *testing.T) {
	a := testAppOnRegistries(t)

	// Press 'd' to open confirm
	m, _ := a.Update(keyRune('d'))
	a = m.(App)

	if !a.confirm.active {
		t.Fatal("expected confirm modal active")
	}

	// Press 'n' (or Esc) to cancel — confirm modal uses Esc to cancel
	m, cmd := a.Update(keyPress(tea.KeyEsc))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	if a.confirm.active {
		t.Fatal("expected confirm modal closed after cancel")
	}
	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false after cancel")
	}
}

func TestIntegration_Registry_ModalBlocksGlobalKeys(t *testing.T) {
	a := testAppOnRegistries(t)

	// Open add modal
	m, cmd := a.Update(keyRune('a'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	if !a.registryAdd.active {
		t.Fatal("expected modal active")
	}

	// Press various global keys — modal should stay active after each
	globalKeys := []tea.Msg{
		keyRune('1'),
		keyRune('2'),
		keyRune('3'),
		keyRune('q'),
		keyRune('R'),
	}

	for _, key := range globalKeys {
		m, _ = a.Update(key)
		a = m.(App)
		if !a.registryAdd.active {
			t.Fatalf("modal should still be active after pressing %v", key)
		}
	}
}

func TestIntegration_Registry_SyncKeyOnlyOnRegistries(t *testing.T) {
	// Start on Library tab (default)
	a := testAppWithItems(t)

	if a.isRegistriesTab() {
		t.Fatal("expected to NOT be on Registries tab")
	}

	// Press 'S' — should not set registryOpInProgress
	m, _ := a.Update(keyRune('S'))
	a = m.(App)

	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false — S should have no effect on Library tab")
	}
}

func TestIntegration_Registry_AddDoneSuccess(t *testing.T) {
	a := testAppOnRegistries(t)
	a.registryOpInProgress = true

	m, _ := a.Update(registryAddDoneMsg{name: "acme/tools"})
	a = m.(App)

	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false after successful registryAddDoneMsg")
	}
}

func TestIntegration_Registry_AddDoneError(t *testing.T) {
	a := testAppOnRegistries(t)
	a.registryOpInProgress = true

	m, _ := a.Update(registryAddDoneMsg{name: "acme/tools", err: fmt.Errorf("clone failed")})
	a = m.(App)

	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false after failed registryAddDoneMsg")
	}
}

func TestIntegration_Registry_SyncDoneSuccess(t *testing.T) {
	a := testAppOnRegistries(t)
	a.registryOpInProgress = true

	m, _ := a.Update(registrySyncDoneMsg{name: "my-reg"})
	a = m.(App)

	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false after successful registrySyncDoneMsg")
	}
}

func TestIntegration_Registry_SyncDoneError(t *testing.T) {
	a := testAppOnRegistries(t)
	a.registryOpInProgress = true

	m, _ := a.Update(registrySyncDoneMsg{name: "my-reg", err: fmt.Errorf("pull failed")})
	a = m.(App)

	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false after failed registrySyncDoneMsg")
	}
}

func TestIntegration_Registry_RemoveDoneSuccess(t *testing.T) {
	a := testAppOnRegistries(t)
	a.registryOpInProgress = true

	m, _ := a.Update(registryRemoveDoneMsg{name: "old-reg"})
	a = m.(App)

	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false after successful registryRemoveDoneMsg")
	}
}

func TestIntegration_Registry_RemoveDoneError(t *testing.T) {
	a := testAppOnRegistries(t)
	a.registryOpInProgress = true

	m, _ := a.Update(registryRemoveDoneMsg{name: "old-reg", err: fmt.Errorf("permission denied")})
	a = m.(App)

	if a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress == false after failed registryRemoveDoneMsg")
	}
}

func TestIntegration_Registry_AddBlockedDuringOp(t *testing.T) {
	a := testAppOnRegistries(t)
	a.registryOpInProgress = true

	// Press 'a' — should NOT open modal
	m, _ := a.Update(keyRune('a'))
	a = m.(App)

	if a.registryAdd.active {
		t.Fatal("expected registry add modal NOT to open when registryOpInProgress is true")
	}
}

func TestIntegration_Registry_SyncBlockedDuringOp(t *testing.T) {
	a := testAppOnRegistries(t)

	// First verify we're on registries and have a card
	if !a.isRegistriesTab() {
		t.Fatal("expected to be on Registries tab")
	}

	// Set flag before pressing S
	a.registryOpInProgress = true

	// Press 'S' — should not start another sync
	beforeFlag := a.registryOpInProgress
	m, _ := a.Update(keyRune('S'))
	a = m.(App)

	// Flag should still be true (unchanged — no new op started)
	if !a.registryOpInProgress {
		t.Fatal("expected registryOpInProgress to remain true")
	}
	if a.registryOpInProgress != beforeFlag {
		t.Fatal("expected no state change when sync blocked by in-progress op")
	}
}

func TestIntegration_Registry_RemoveBlockedDuringOp(t *testing.T) {
	a := testAppOnRegistries(t)
	a.registryOpInProgress = true

	// Press 'd' — should NOT open confirm modal
	m, _ := a.Update(keyRune('d'))
	a = m.(App)

	if a.confirm.active {
		t.Fatal("expected confirm modal NOT to open when registryOpInProgress is true")
	}
}

// --- Golden tests: Registry add modal ---

func TestGolden_RegistryAddModal_Git_80x30(t *testing.T) {
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "alpha-skill", Type: catalog.Skills, Source: "my-registry", Registry: "my-registry", Files: []string{"SKILL.md"}},
		},
	}
	app := NewApp(cat, nil, "0.0.0-test", false, []catalog.RegistrySource{
		{Name: "my-registry", Path: "/tmp/fake-registry"},
	}, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	a := m.(App)

	// Navigate to Registries
	m, cmd := a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	// Open add modal (default: git mode)
	m, cmd = a.Update(keyRune('a'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	requireGolden(t, "registry-add-modal-git-80x30", snapshotApp(t, a))
}

func TestGolden_RegistryAddModal_Local_80x30(t *testing.T) {
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "alpha-skill", Type: catalog.Skills, Source: "my-registry", Registry: "my-registry", Files: []string{"SKILL.md"}},
		},
	}
	app := NewApp(cat, nil, "0.0.0-test", false, []catalog.RegistrySource{
		{Name: "my-registry", Path: "/tmp/fake-registry"},
	}, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	a := m.(App)

	// Navigate to Registries
	m, cmd := a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	// Open modal
	m, cmd = a.Update(keyRune('a'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	// Switch to local mode: Shift+Tab to radio (idx 0), then Space
	m, _ = a.Update(keyShiftTab)
	a = m.(App)
	m, _ = a.Update(keyPress(tea.KeySpace))
	a = m.(App)

	requireGolden(t, "registry-add-modal-local-80x30", snapshotApp(t, a))
}

func TestGolden_RegistryAddModal_Error_80x30(t *testing.T) {
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "alpha-skill", Type: catalog.Skills, Source: "my-registry", Registry: "my-registry", Files: []string{"SKILL.md"}},
		},
	}
	app := NewApp(cat, nil, "0.0.0-test", false, []catalog.RegistrySource{
		{Name: "my-registry", Path: "/tmp/fake-registry"},
	}, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	a := m.(App)

	// Navigate to Registries
	m, cmd := a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	// Open modal
	m, cmd = a.Update(keyRune('a'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	// Navigate to Add button and press Enter (empty URL triggers error)
	// Tab through: URL(1) → Name(2) → Branch(3) → Cancel(4) → Add(5)
	for i := 0; i < 4; i++ {
		m, _ = a.Update(keyTab)
		a = m.(App)
	}
	m, _ = a.Update(keyPress(tea.KeyEnter))
	a = m.(App)

	requireGolden(t, "registry-add-modal-error-80x30", snapshotApp(t, a))
}

func TestGolden_RegistryAddModal_Buttons_80x30(t *testing.T) {
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "alpha-skill", Type: catalog.Skills, Source: "my-registry", Registry: "my-registry", Files: []string{"SKILL.md"}},
		},
	}
	app := NewApp(cat, nil, "0.0.0-test", false, []catalog.RegistrySource{
		{Name: "my-registry", Path: "/tmp/fake-registry"},
	}, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	a := m.(App)

	// Navigate to Registries (no modal) to show [a] Add [S] Sync buttons
	m, cmd := a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	// Semantic assertions — this golden exists to pin the Registries-tab
	// action-button layout. The test is worthless if the buttons don't
	// actually appear, so assert each label directly.
	view := a.View()
	assertContains(t, view, "[a] Add registry") // Add button (active-tab context)
	assertContains(t, view, "[S] Sync")         // Sync button (selected-registry action)
	assertContains(t, view, "[d] Remove")       // Remove button
	assertContains(t, view, "[e] Edit")         // Edit button
	assertContains(t, view, "my-registry")      // the selected registry itself

	requireGolden(t, "gallery-registries-buttons-80x30", snapshotApp(t, a))
}

// Verify unused catalog import is consumed
var _ = catalog.Skills

// --- Add Wizard Integration Tests ---

func TestApp_AddKeyOpensWizardOnLibrary(t *testing.T) {
	app := testAppWithItems(t)

	m, _ := app.Update(keyRune('a'))
	a := m.(App)

	if a.wizardMode != wizardAdd {
		t.Fatal("expected wizardAdd mode after [a] on Library tab")
	}
	if a.addWizard == nil {
		t.Fatal("expected addWizard not nil")
	}
	if a.addWizard.preFilterType != "" {
		t.Fatalf("expected no preFilterType on Library tab, got %s", a.addWizard.preFilterType)
	}

	view := a.View()
	assertContains(t, view, "Where is the content?")
}

func TestApp_AddKeyOpensWizardOnContentTab(t *testing.T) {
	app := testAppWithItems(t)

	// Switch to Content group (2), then tab to get to a content tab
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)

	// Press 'a' to open add wizard with preFilterType
	m, _ = a.Update(keyRune('a'))
	a = m.(App)

	if a.wizardMode != wizardAdd {
		t.Fatal("expected wizardAdd mode on Content tab")
	}
	if a.addWizard == nil {
		t.Fatal("expected addWizard not nil")
	}
	// Content tab defaults to first sub-tab which should set a preFilterType
	if a.addWizard.preFilterType == "" {
		t.Fatal("expected preFilterType set on Content tab")
	}
}

func TestApp_AddKeyOnRegistriesDoesNotOpenWizard(t *testing.T) {
	// Registries tab should still use registry add modal, not add wizard
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "test-loadout", Type: catalog.Loadouts, Source: "library"},
		},
	}
	app := NewApp(cat, nil, "0.0.0-test", false, []catalog.RegistrySource{
		{Name: "my-registry", Path: "/tmp/fake-registry"},
	}, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	a := m.(App)

	// Navigate to Registries tab
	m, cmd := a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	// Press 'a' — should open registry add modal, not add wizard
	m, _ = a.Update(keyRune('a'))
	a = m.(App)

	if a.wizardMode == wizardAdd {
		t.Fatal("should not open add wizard on Registries tab")
	}
}

func TestApp_AddWizardEscCloses(t *testing.T) {
	app := testAppWithItems(t)

	// Open add wizard
	m, _ := app.Update(keyRune('a'))
	a := m.(App)

	if a.wizardMode != wizardAdd {
		t.Fatal("expected wizardAdd mode")
	}

	// Esc closes wizard
	m, cmd := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	if a.wizardMode != wizardNone {
		t.Fatal("expected wizardNone after Esc")
	}
	if a.addWizard != nil {
		t.Fatal("expected addWizard nil after close")
	}
}

func TestApp_AddWizardSuppressesGroupKeys(t *testing.T) {
	app := testAppWithItems(t)

	// Open add wizard
	m, _ := app.Update(keyRune('a'))
	a := m.(App)

	// Press '1' — should be suppressed, not switch groups
	m, _ = a.Update(keyRune('1'))
	a = m.(App)

	if a.wizardMode != wizardAdd {
		t.Fatal("expected to still be in add wizard after '1'")
	}
}

func TestApp_AddWizardWindowResize(t *testing.T) {
	app := testAppWithItems(t)

	m, _ := app.Update(keyRune('a'))
	a := m.(App)

	// Resize
	m, _ = a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a = m.(App)

	if a.addWizard.width != 120 {
		t.Fatalf("expected wizard width 120 after resize, got %d", a.addWizard.width)
	}
}

// --- Coverage boost: pure helpers + Init ---

func TestApp_Init_NoTelemetryNotice_ReturnsNil(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	app.telemetryNotice = false
	if cmd := app.Init(); cmd != nil {
		t.Errorf("expected nil cmd when telemetryNotice=false, got %v", cmd)
	}
}

func TestApp_Init_WithTelemetryNotice_ReturnsCmd(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	app.telemetryNotice = true
	cmd := app.Init()
	if cmd == nil {
		t.Fatal("expected non-nil cmd when telemetryNotice=true")
	}
	msg := cmd()
	if _, ok := msg.(telemetryNoticeMsg); !ok {
		t.Errorf("expected telemetryNoticeMsg, got %T", msg)
	}
}

func TestApp_TabToContentType(t *testing.T) {
	t.Parallel()
	cases := map[string]catalog.ContentType{
		"Skills":   catalog.Skills,
		"Agents":   catalog.Agents,
		"MCP":      catalog.MCP,
		"Rules":    catalog.Rules,
		"Hooks":    catalog.Hooks,
		"Commands": catalog.Commands,
		"Unknown":  catalog.ContentType(""),
	}
	for tab, want := range cases {
		if got := tabToContentType(tab); got != want {
			t.Errorf("tabToContentType(%q) = %q, want %q", tab, got, want)
		}
	}
}

func TestApp_RenderPlaceholder(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	out := ansi.Strip(app.renderPlaceholder("Hello"))
	if !strings.Contains(out, "Hello") {
		t.Errorf("expected 'Hello' in placeholder, got %q", out)
	}
}

func TestApp_HandleBreadcrumbClick_NoOpOnLandingPage(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	// On Library browse mode with index=0, breadcrumb click should be a no-op.
	m, cmd := app.handleBreadcrumbClick(breadcrumbClickMsg{index: 0})
	if _, ok := m.(App); !ok {
		t.Fatalf("expected App return, got %T", m)
	}
	if cmd != nil {
		t.Errorf("expected nil cmd on landing page, got %v", cmd)
	}
}

func TestApp_HandleBreadcrumbClick_LibraryDetail_IndexZero_NoOp(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	app.library.mode = libraryDetail
	_, cmd := app.handleBreadcrumbClick(breadcrumbClickMsg{index: 0})
	if cmd != nil {
		t.Errorf("expected nil cmd (already at that depth), got %v", cmd)
	}
}

func TestApp_HandleBreadcrumbClick_GalleryDrillIn_IndexZero_ExitsDetail(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	// Set up Registries tab (gallery) with drill-in + library in detail mode
	m, _ := app.Update(keyTab)
	a := m.(App)
	a.galleryDrillIn = true
	a.library.mode = libraryDetail
	m, cmd := a.handleBreadcrumbClick(breadcrumbClickMsg{index: 0})
	_ = cmd
	a = m.(App)
	// Clicking card-name breadcrumb (index=0) while in detail should revert to browse.
	if a.library.mode != libraryBrowse {
		t.Errorf("expected libraryBrowse after breadcrumb index=0 click, got %v", a.library.mode)
	}
}

func TestApp_HandleBreadcrumbClick_GalleryDrillIn_IndexNonZero_NoOp(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	m, _ := app.Update(keyTab)
	a := m.(App)
	a.galleryDrillIn = true
	_, cmd := a.handleBreadcrumbClick(breadcrumbClickMsg{index: 1})
	if cmd != nil {
		t.Errorf("expected nil cmd when index>=1 in drill-in, got %v", cmd)
	}
}

func TestApp_HandleBreadcrumbClick_ExplorerDetail_NoOp(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	// Switch to Content group (explorer) and set explorer mode to detail.
	m, _ := app.Update(keyRune('2'))
	a := m.(App)
	a.explorer.mode = explorerDetail
	_, cmd := a.handleBreadcrumbClick(breadcrumbClickMsg{index: 0})
	if cmd != nil {
		t.Errorf("expected nil cmd on explorer detail breadcrumb index=0, got %v", cmd)
	}
}

func TestApp_IsFilePath(t *testing.T) {
	t.Parallel()
	// Path with extension on nonexistent file → treated as file.
	if !isFilePath("/nonexistent/foo.md") {
		t.Error("expected true for path with extension")
	}
	// Path with no extension on nonexistent path → false.
	if isFilePath("/nonexistent/foobar") {
		t.Error("expected false for path without extension")
	}
}

func TestApp_HandleCatalogReady_Error_QueuesErrorToast(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	// Reset toast state so Push returns a non-nil show cmd deterministically.
	app.toast.visible = false
	app.toast.queue = nil
	m, _ := app.handleCatalogReady(catalogReadyMsg{err: fmt.Errorf("boom")})
	updated := m.(App)
	// Error toasts don't auto-dismiss, so cmd is intentionally nil — verify via queue.
	if len(updated.toast.queue) != 1 {
		t.Fatalf("expected 1 queued toast, got %d", len(updated.toast.queue))
	}
	if updated.toast.queue[0].level != toastError {
		t.Errorf("expected toastError level, got %v", updated.toast.queue[0].level)
	}
	if !strings.Contains(updated.toast.queue[0].message, "boom") {
		t.Errorf("expected error message to contain 'boom', got %q", updated.toast.queue[0].message)
	}
}

func TestApp_HandleCatalogReady_Success_RefreshesAndToasts(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	app.toast.visible = false
	app.toast.queue = nil

	newCatalog := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "post-rescan", Type: catalog.Skills, Source: "library", Files: []string{"SKILL.md"}},
		},
	}
	result := &moat.ScanResult{
		Catalog: newCatalog,
		Config:  &config.Config{},
	}
	m, cmd := app.handleCatalogReady(catalogReadyMsg{result: result})
	updated := m.(App)
	if updated.catalog == nil || len(updated.catalog.Items) != 1 {
		t.Fatalf("expected catalog swapped in with 1 item, got %+v", updated.catalog)
	}
	if updated.catalog.Items[0].Name != "post-rescan" {
		t.Errorf("expected post-rescan item, got %q", updated.catalog.Items[0].Name)
	}
	if updated.galleryDrillIn {
		t.Error("handleCatalogReady should clear galleryDrillIn")
	}
	if cmd == nil {
		t.Error("expected toast cmd on success")
	}
	if len(updated.toast.queue) == 0 {
		t.Error("expected success toast queued")
	}
}

// --- routeKeyConfig / routeMouse coverage ---

func TestApp_RouteKeyConfig_Sandbox(t *testing.T) {
	app := testApp(t)
	// Navigate to Config > Sandbox (group 3 -> Right once)
	m, _ := app.Update(keyRune('3'))
	a := m.(App)
	m, _ = a.Update(keyPress(tea.KeyRight))
	a = m.(App)
	if a.topBar.ActiveTabLabel() != "Sandbox" {
		t.Fatalf("expected Sandbox tab, got %q", a.topBar.ActiveTabLabel())
	}
	// Send a key that sandbox will consume (covers the switch case)
	m, _ = a.Update(keyRune('x'))
	if _, ok := m.(App); !ok {
		t.Fatalf("expected App, got %T", m)
	}
}

func TestApp_RouteKeyConfig_System(t *testing.T) {
	app := testApp(t)
	// Navigate to Config > System (group 3 -> Right twice)
	m, _ := app.Update(keyRune('3'))
	a := m.(App)
	m, _ = a.Update(keyPress(tea.KeyRight))
	a = m.(App)
	m, _ = a.Update(keyPress(tea.KeyRight))
	a = m.(App)
	if a.topBar.ActiveTabLabel() != "System" {
		t.Fatalf("expected System tab, got %q", a.topBar.ActiveTabLabel())
	}
	// Send a key — System should consume it without crashing
	m, _ = a.Update(keyRune('x'))
	if _, ok := m.(App); !ok {
		t.Fatalf("expected App, got %T", m)
	}
}

func TestApp_RouteMouse_ConfigSettings(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyRune('3'))
	a := m.(App)
	click := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	m, _ = a.Update(click)
	if _, ok := m.(App); !ok {
		t.Fatalf("expected App, got %T", m)
	}
}

func TestApp_RouteMouse_ConfigSandbox(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyRune('3'))
	a := m.(App)
	m, _ = a.Update(keyTab)
	a = m.(App)
	click := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	m, _ = a.Update(click)
	if _, ok := m.(App); !ok {
		t.Fatalf("expected App, got %T", m)
	}
}

func TestApp_RouteMouse_ConfigSystem(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyRune('3'))
	a := m.(App)
	m, _ = a.Update(keyTab)
	a = m.(App)
	m, _ = a.Update(keyTab)
	a = m.(App)
	click := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	m, _ = a.Update(click)
	if _, ok := m.(App); !ok {
		t.Fatalf("expected App, got %T", m)
	}
}

func TestApp_RouteMouse_LibraryTab(t *testing.T) {
	app := testAppWithItems(t)
	click := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	m, _ := app.Update(click)
	if _, ok := m.(App); !ok {
		t.Fatalf("expected App, got %T", m)
	}
}

func TestApp_RouteMouse_GalleryTab(t *testing.T) {
	app := testApp(t)
	// Move to Registries (gallery)
	m, _ := app.Update(keyTab)
	a := m.(App)
	if !a.isGalleryTab() {
		t.Fatal("expected isGalleryTab after tab")
	}
	click := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	m, _ = a.Update(click)
	if _, ok := m.(App); !ok {
		t.Fatalf("expected App, got %T", m)
	}
}

func TestApp_RouteMouse_ExplorerTab(t *testing.T) {
	app := testApp(t)
	// Group 2 is Content → first tab is Skills (explorer)
	m, _ := app.Update(keyRune('2'))
	a := m.(App)
	click := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	m, _ = a.Update(click)
	if _, ok := m.(App); !ok {
		t.Fatalf("expected App, got %T", m)
	}
}

func TestApp_RouteMouse_GalleryDrillIn(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyTab)
	a := m.(App)
	a.galleryDrillIn = true
	click := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	m, _ = a.Update(click)
	if _, ok := m.(App); !ok {
		t.Fatalf("expected App, got %T", m)
	}
}
