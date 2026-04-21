package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// --- Card grid navigation ---

func TestCardGrid_ArrowNavigation(t *testing.T) {
	cards := []cardData{
		{name: "Card-A", counts: map[string]int{"Skills": 2}},
		{name: "Card-B", counts: map[string]int{"Rules": 3}},
		{name: "Card-C", counts: map[string]int{"Agents": 1}},
	}
	grid := newCardGridModel(cards)
	grid.SetSize(80, 20)

	if grid.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", grid.cursor)
	}

	grid.CursorRight()
	if grid.cursor != 1 {
		t.Errorf("CursorRight: expected 1, got %d", grid.cursor)
	}

	grid.CursorLeft()
	if grid.cursor != 0 {
		t.Errorf("CursorLeft: expected 0, got %d", grid.cursor)
	}

	// Wrap left from 0
	grid.CursorLeft()
	if grid.cursor != 2 {
		t.Errorf("CursorLeft wrap: expected 2, got %d", grid.cursor)
	}

	// Wrap right from last
	grid.CursorRight()
	if grid.cursor != 0 {
		t.Errorf("CursorRight wrap: expected 0, got %d", grid.cursor)
	}
}

func TestCardGrid_UpDownNavigation(t *testing.T) {
	// 4 cards, 2 columns => 2 rows
	cards := []cardData{
		{name: "A", counts: map[string]int{"Skills": 1}},
		{name: "B", counts: map[string]int{"Skills": 1}},
		{name: "C", counts: map[string]int{"Skills": 1}},
		{name: "D", counts: map[string]int{"Skills": 1}},
	}
	grid := newCardGridModel(cards)
	grid.SetSize(80, 30)

	// At 80 width, should be 2 columns
	if grid.cols != 2 {
		t.Fatalf("expected 2 cols, got %d", grid.cols)
	}

	// cursor=0, down should go to 2
	grid.CursorDown()
	if grid.cursor != 2 {
		t.Errorf("CursorDown from 0: expected 2, got %d", grid.cursor)
	}

	// cursor=2, up should go to 0
	grid.CursorUp()
	if grid.cursor != 0 {
		t.Errorf("CursorUp from 2: expected 0, got %d", grid.cursor)
	}
}

func TestCardGrid_EmptyGrid(t *testing.T) {
	grid := newCardGridModel(nil)
	grid.SetSize(80, 20)

	view := grid.View()
	if view == "" {
		t.Error("empty grid should render something")
	}

	// Should not panic
	grid.CursorUp()
	grid.CursorDown()
	grid.CursorLeft()
	grid.CursorRight()

	if grid.Selected() != nil {
		t.Error("Selected should be nil for empty grid")
	}
}

func TestCardGrid_ResponsiveCols(t *testing.T) {
	cards := []cardData{{name: "A", counts: map[string]int{"Skills": 1}}}

	tests := []struct {
		width    int
		wantCols int
	}{
		{120, 3},
		{90, 3},
		{80, 2},
		{60, 2},
		{50, 1},
	}
	for _, tt := range tests {
		grid := newCardGridModel(cards)
		grid.SetSize(tt.width, 20)
		if grid.cols != tt.wantCols {
			t.Errorf("width=%d: expected %d cols, got %d", tt.width, tt.wantCols, grid.cols)
		}
	}
}

// --- Contents sidebar ---

func TestContentsSidebar_SetCard(t *testing.T) {
	card := &cardData{
		name: "Test",
		items: []catalog.ContentItem{
			{Name: "skill-a", Type: catalog.Skills},
			{Name: "skill-b", Type: catalog.Skills},
			{Name: "rule-a", Type: catalog.Rules},
		},
	}

	sidebar := newContentsSidebarModel()
	sidebar.SetSize(30, 20)
	sidebar.SetCard(card)

	if len(sidebar.groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(sidebar.groups))
	}
	if sidebar.groups[0].typeName != "Skills" {
		t.Errorf("first group should be Skills, got %s", sidebar.groups[0].typeName)
	}
	if len(sidebar.groups[0].items) != 2 {
		t.Errorf("Skills group should have 2 items, got %d", len(sidebar.groups[0].items))
	}

	view := sidebar.View()
	if view == "" {
		t.Error("sidebar view should not be empty")
	}
}

func TestContentsSidebar_NilCard(t *testing.T) {
	sidebar := newContentsSidebarModel()
	sidebar.SetSize(30, 20)
	sidebar.SetCard(nil)

	if len(sidebar.groups) != 0 {
		t.Error("nil card should clear groups")
	}
}

// TestContentsSidebar_ScrollUpDown pins ScrollUp (contents.go:87) and
// ScrollDown (contents.go:94). Offset increments/decrements with clamps at 0
// and total-height. Ensures wheel scrolling on the sidebar-focused gallery
// actually moves the offset.
func TestContentsSidebar_ScrollUpDown(t *testing.T) {
	card := &cardData{
		name: "Test",
		items: []catalog.ContentItem{
			{Name: "a", Type: catalog.Skills},
			{Name: "b", Type: catalog.Skills},
			{Name: "c", Type: catalog.Skills},
			{Name: "d", Type: catalog.Skills},
			{Name: "e", Type: catalog.Skills},
			{Name: "f", Type: catalog.Skills},
		},
	}
	sidebar := newContentsSidebarModel()
	sidebar.SetSize(30, 3) // small height forces scrolling
	sidebar.SetCard(card)

	// ScrollUp at offset 0 is a no-op.
	sidebar.ScrollUp()
	if sidebar.offset != 0 {
		t.Errorf("ScrollUp at offset 0 should stay at 0, got %d", sidebar.offset)
	}

	// ScrollDown should increment until it hits total-height.
	before := sidebar.offset
	sidebar.ScrollDown()
	if sidebar.offset <= before {
		t.Errorf("ScrollDown should advance offset from %d, got %d", before, sidebar.offset)
	}

	// ScrollUp should now reverse.
	afterDown := sidebar.offset
	sidebar.ScrollUp()
	if sidebar.offset >= afterDown {
		t.Errorf("ScrollUp should decrement offset from %d, got %d", afterDown, sidebar.offset)
	}
}

// --- Gallery model ---

func TestGallery_TabTogglesFocus(t *testing.T) {
	g := newGalleryModel()
	g.SetSize(80, 25)
	cards := []cardData{
		{name: "A", counts: map[string]int{"Skills": 1}},
	}
	g.SetCards(cards, "Loadout")

	if g.focus != paneGrid {
		t.Fatal("initial focus should be grid")
	}

	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyTab})
	if g.focus != paneSidebar {
		t.Error("Tab should switch to sidebar")
	}

	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyTab})
	if g.focus != paneGrid {
		t.Error("Tab should switch back to grid")
	}
}

func TestGallery_EnterDrillsIn(t *testing.T) {
	g := newGalleryModel()
	g.SetSize(80, 25)
	cards := []cardData{
		{name: "A", counts: map[string]int{"Skills": 1}, items: []catalog.ContentItem{{Name: "a"}}},
	}
	g.SetCards(cards, "Loadout")

	g, cmd := g.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	if _, ok := msg.(cardDrillMsg); !ok {
		t.Errorf("expected cardDrillMsg, got %T", msg)
	}
}

func TestGallery_ViewRenders(t *testing.T) {
	g := newGalleryModel()
	g.SetSize(80, 25)
	cards := []cardData{
		{name: "Python-Web", counts: map[string]int{"Skills": 4, "Rules": 2}, subtitle: "Target: CC", status: "local"},
		{name: "React-Frontend", counts: map[string]int{"Skills": 6, "MCP Servers": 2}, subtitle: "Target: Cu", status: "local"},
	}
	g.SetCards(cards, "Loadout")

	view := g.View()
	if view == "" {
		t.Fatal("gallery view should not be empty")
	}
}

// --- App integration ---

func TestApp_LoadoutsShowsGallery(t *testing.T) {
	app := testApp(t)

	// Navigate to Collections > Loadouts (Tab Tab from Library)
	m, cmd := app.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	m, cmd = m.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a := m.(App)
	view := a.View()

	// Should show gallery (No items found since empty catalog)
	assertContains(t, view, "No items found")
	assertContains(t, view, "arrows grid")
}

// --- Golden files ---

func TestGoldenGallery_80x30(t *testing.T) {
	// Create a catalog with loadout items to populate the gallery
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "python-web", DisplayName: "Python-Web", Type: catalog.Loadouts, Source: "project", Files: []string{"loadout.yaml"}},
		},
	}
	app := NewApp(cat, nil, "0.0.0-test", false, nil, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	a := m.(App)

	// Navigate to Collections > Loadouts (Tab Tab)
	m, cmd := a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	m, cmd = m.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	// Semantic assertions — pin behaviour independently of the golden so
	// that a buggy render blessed via -update-golden still fails this test.
	view := a.View()
	assertContains(t, view, "Python-Web")      // loadout display name from catalog
	assertContains(t, view, "[a] Add loadout") // Loadouts-tab action button
	assertContains(t, view, "Loadout")         // card subtitle / type label
	assertNotContains(t, view, "Add registry") // not on Registries sub-tab

	requireGolden(t, "gallery-80x30", snapshotApp(t, a))
}

func TestGoldenGallery_120x40(t *testing.T) {
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "python-web", DisplayName: "Python-Web", Type: catalog.Loadouts, Source: "project", Files: []string{"loadout.yaml"}},
			{Name: "alpha-skill", Type: catalog.Skills, Source: "my-registry", Registry: "my-registry", Files: []string{"SKILL.md"}, Description: "A helpful skill"},
			{Name: "beta-rule", Type: catalog.Rules, Source: "my-registry", Registry: "my-registry", Files: []string{"rule.md"}},
		},
	}
	app := NewApp(cat, nil, "0.0.0-test", false, []catalog.RegistrySource{
		{Name: "my-registry", Path: "/tmp/fake-registry"},
	}, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a := m.(App)

	// Navigate to Collections > Registries (Tab)
	m, cmd := a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	// Semantic assertions for Registries gallery at 120x40 — both registry
	// items must appear in the contents sidebar, and the Registries-specific
	// action button must be present.
	view := a.View()
	assertContains(t, view, "my-registry")      // registry name from source
	assertContains(t, view, "alpha-skill")      // Skills item in registry
	assertContains(t, view, "beta-rule")        // Rules item in registry
	assertContains(t, view, "[a] Add registry") // Registries-tab action button
	assertNotContains(t, view, "Add loadout")   // not on Loadouts sub-tab

	requireGolden(t, "gallery-registries-120x40", snapshotApp(t, a))
}
