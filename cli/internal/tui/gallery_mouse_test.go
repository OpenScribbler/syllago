package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// Gallery mouse tests — pin the mouse parity rule for the card grid used by
// the Loadouts and Registries sub-tabs. Every zone that appears in
// galleryModel.updateMouse (gallery.go:249) must have a corresponding test
// here so a regression dropping a zone mark fails loudly.
//
// Zone IDs covered:
//   card-{i}            — grid card (single-click select, double-click drill)
//   meta-remove         — metadata bar Remove button
//   meta-edit           — metadata bar Edit button
//   meta-sync           — metadata bar Sync button (Registry tab only)
//
// meta-install and meta-add appear in updateMouse but are emitted by
// metapanel (metapanel.go:177) rather than the gallery itself, so they are
// exercised by app-level tests, not here.

// galleryWithCards builds a galleryModel sized for mouse testing, populated
// with two cards on the given tab label.
func galleryWithCards(t *testing.T, tabLabel string) *galleryModel {
	t.Helper()
	g := newGalleryModel()
	g.SetSize(100, 30)
	cards := []cardData{
		{
			name:     "alpha",
			subtitle: "Target: claude-code",
			desc:     "First card",
			status:   "local",
			counts:   map[string]int{"Skills": 2, "Rules": 1},
			items:    []catalog.ContentItem{{Name: "a-skill", Type: catalog.Skills}},
		},
		{
			name:     "beta",
			subtitle: "Target: cursor",
			desc:     "Second card",
			status:   "local",
			counts:   map[string]int{"Agents": 1},
			items:    []catalog.ContentItem{{Name: "b-agent", Type: catalog.Agents}},
		},
	}
	g.SetCards(cards, tabLabel)
	return &g
}

// mouseScroll produces a scroll-wheel press event at (x,y).
func mouseScroll(x, y int, up bool) tea.MouseMsg {
	btn := tea.MouseButtonWheelDown
	if up {
		btn = tea.MouseButtonWheelUp
	}
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: btn}
}

// --- Card clicks ---

// TestGalleryMouse_CardClickSelects pins card-N zone marks at cards.go:241
// and the selection branch at gallery.go:277. Clicking a non-cursor card
// must move the cursor and emit cardSelectedMsg (not cardDrillMsg).
func TestGalleryMouse_CardClickSelects(t *testing.T) {
	g := galleryWithCards(t, "Loadout")
	scanZones(g.View())

	// Click card index 1 (cursor starts at 0).
	z := zone.Get("card-1")
	if z.IsZero() {
		t.Fatal("zone card-1 should be registered after View()")
	}
	newG, cmd := g.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected command from card-1 click")
	}
	if newG.grid.cursor != 1 {
		t.Errorf("cursor should be 1 after card-1 click, got %d", newG.grid.cursor)
	}
	msg := cmd()
	sel, ok := msg.(cardSelectedMsg)
	if !ok {
		t.Fatalf("expected cardSelectedMsg from click on non-cursor card, got %T", msg)
	}
	if sel.card == nil || sel.card.name != "beta" {
		t.Errorf("cardSelectedMsg should carry beta card, got %+v", sel.card)
	}
}

// TestGalleryMouse_CardDoubleClickDrills pins the double-click branch at
// gallery.go:270. Clicking the already-selected card emits cardDrillMsg.
func TestGalleryMouse_CardDoubleClickDrills(t *testing.T) {
	g := galleryWithCards(t, "Loadout")
	scanZones(g.View())

	// Cursor starts at 0, so clicking card-0 triggers drill-in.
	z := zone.Get("card-0")
	if z.IsZero() {
		t.Fatal("zone card-0 should be registered after View()")
	}
	_, cmd := g.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected command from double-click on card-0")
	}
	drill, ok := cmd().(cardDrillMsg)
	if !ok {
		t.Fatalf("expected cardDrillMsg from click on cursor card, got %T", cmd())
	}
	if drill.card == nil || drill.card.name != "alpha" {
		t.Errorf("cardDrillMsg should carry alpha card, got %+v", drill.card)
	}
}

// TestGalleryMouse_CardClickFocusesGrid verifies the side effect at
// gallery.go:278 — clicking a card sets focus to the grid pane even if the
// sidebar was focused. Mouse parity with Tab toggle focus.
func TestGalleryMouse_CardClickFocusesGrid(t *testing.T) {
	g := galleryWithCards(t, "Loadout")
	g.setFocus(paneSidebar)
	if g.focus != paneSidebar {
		t.Fatal("setup: focus should be sidebar")
	}
	scanZones(g.View())

	z := zone.Get("card-1")
	if z.IsZero() {
		t.Fatal("zone card-1 should be registered")
	}
	newG, _ := g.Update(mouseClick(z.StartX, z.StartY))
	if newG.focus != paneGrid {
		t.Error("card click should focus grid pane even when sidebar was focused")
	}
}

// --- Meta bar buttons ---

// TestGalleryMouse_MetaRemove pins meta-remove at gallery.go:497.
func TestGalleryMouse_MetaRemove(t *testing.T) {
	g := galleryWithCards(t, "Loadout")
	scanZones(g.View())

	z := zone.Get("meta-remove")
	if z.IsZero() {
		t.Fatal("zone meta-remove should be registered on gallery metadata bar")
	}
	_, cmd := g.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected command from meta-remove click")
	}
	if _, ok := cmd().(libraryRemoveMsg); !ok {
		t.Errorf("expected libraryRemoveMsg, got %T", cmd())
	}
}

// TestGalleryMouse_MetaEdit pins meta-edit at gallery.go:498.
func TestGalleryMouse_MetaEdit(t *testing.T) {
	g := galleryWithCards(t, "Loadout")
	scanZones(g.View())

	z := zone.Get("meta-edit")
	if z.IsZero() {
		t.Fatal("zone meta-edit should be registered on gallery metadata bar")
	}
	_, cmd := g.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected command from meta-edit click")
	}
	if _, ok := cmd().(libraryEditMsg); !ok {
		t.Errorf("expected libraryEditMsg, got %T", cmd())
	}
}

// TestGalleryMouse_MetaSyncRegistry pins meta-sync at gallery.go:493.
// Sync button renders only when tabLabel=="Registry" (conditional zone mark
// at gallery.go:491-494).
func TestGalleryMouse_MetaSyncRegistry(t *testing.T) {
	g := galleryWithCards(t, "Registry")
	scanZones(g.View())

	z := zone.Get("meta-sync")
	if z.IsZero() {
		t.Fatal("zone meta-sync should be registered on Registry tab metadata bar")
	}
	_, cmd := g.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected command from meta-sync click")
	}
	act, ok := cmd().(actionPressedMsg)
	if !ok {
		t.Fatalf("expected actionPressedMsg, got %T", cmd())
	}
	if act.action != "sync" {
		t.Errorf("expected action=\"sync\", got %q", act.action)
	}
}

// TestGalleryMouse_MetaSyncLoadoutNotRendered verifies the Loadout tab does
// NOT render the Sync button (it's registry-only). Guards against a
// regression that makes the button appear on the wrong tab.
func TestGalleryMouse_MetaSyncLoadoutNotRendered(t *testing.T) {
	g := galleryWithCards(t, "Loadout")
	scanZones(g.View())

	z := zone.Get("meta-sync")
	if !z.IsZero() {
		t.Error("meta-sync zone should NOT be registered on Loadout tab")
	}
}

// --- Scroll wheel ---

// TestGalleryMouse_ScrollWheelGridFocused pins the grid scroll branch at
// gallery.go:290 / 297. Wheel events move the card cursor up/down when the
// grid is focused.
func TestGalleryMouse_ScrollWheelGridFocused(t *testing.T) {
	// Need enough cards that CursorDown has somewhere to go.
	g := newGalleryModel()
	g.SetSize(100, 30)
	cards := []cardData{
		{name: "A", counts: map[string]int{"Skills": 1}},
		{name: "B", counts: map[string]int{"Skills": 1}},
		{name: "C", counts: map[string]int{"Skills": 1}},
		{name: "D", counts: map[string]int{"Skills": 1}},
	}
	g.SetCards(cards, "Loadout")
	if g.focus != paneGrid {
		t.Fatal("setup: focus should start on grid")
	}

	// Scroll down — cursor should move down one row (cols depends on width).
	startCursor := g.grid.cursor
	newG, _ := g.Update(mouseScroll(10, 10, false))
	if newG.grid.cursor == startCursor {
		t.Error("scroll down should advance grid cursor when grid is focused")
	}

	// Scroll up — cursor should move back.
	newG2, _ := newG.Update(mouseScroll(10, 10, true))
	if newG2.grid.cursor != startCursor {
		t.Errorf("scroll up should return grid cursor to %d, got %d",
			startCursor, newG2.grid.cursor)
	}
}

// TestGalleryMouse_ScrollWheelSidebarFocused pins the sidebar scroll branch
// at gallery.go:294 / 301. Wheel events scroll the sidebar when it's
// focused, leaving the grid cursor untouched.
func TestGalleryMouse_ScrollWheelSidebarFocused(t *testing.T) {
	g := galleryWithCards(t, "Loadout")
	g.setFocus(paneSidebar)
	startCursor := g.grid.cursor

	newG, _ := g.Update(mouseScroll(80, 10, false))
	if newG.grid.cursor != startCursor {
		t.Errorf("scroll down on sidebar-focused gallery should not move grid cursor (was %d, got %d)",
			startCursor, newG.grid.cursor)
	}
}
