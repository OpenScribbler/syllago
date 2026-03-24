package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testCards() []cardData {
	return []cardData{
		{title: "Python-Web", lines: []string{"4 Skills", "2 Rules"}},
		{title: "Go-Backend", subtitle: "https://example.com", lines: []string{"3 Skills"}},
		{title: "React-UI", lines: []string{"1 Agent", "2 Rules"}},
		{title: "DevOps", lines: []string{"5 Hooks"}},
		{title: "Security", lines: []string{"1 Rule"}},
		{title: "ML-Pipeline", lines: []string{"3 Skills", "1 Agent"}},
	}
}

func TestCardGridColumnsAtWidths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		width    int
		wantCols int
	}{
		{"narrow single column", 40, 1},
		{"medium two columns", 60, 2},
		{"wide three columns", 100, 3},
		{"boundary 45 is two columns", 45, 2},
		{"boundary 90 is three columns", 90, 3},
		{"boundary 44 is single column", 44, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := newCardGridModel(testCards())
			m.width = tt.width
			m.height = 40
			got := m.calcCols()
			if got != tt.wantCols {
				t.Fatalf("width=%d: expected %d cols, got %d", tt.width, tt.wantCols, got)
			}
		})
	}
}

func TestCardGridViewRendersCards(t *testing.T) {
	t.Parallel()
	m := newCardGridModel(testCards())
	m.width = 100
	m.height = 80
	m.cols = m.calcCols()

	v := m.View()
	// All card titles should appear.
	for _, c := range m.cards {
		if !strings.Contains(v, c.title) {
			t.Errorf("view missing card title %q", c.title)
		}
	}
}

func TestCardGridSelectedCardStyle(t *testing.T) {
	t.Parallel()
	m := newCardGridModel(testCards())
	m.width = 100
	m.height = 80
	m.cursor = 0
	m.cols = m.calcCols()

	v := m.View()
	// The first card's title should appear in the output.
	if !strings.Contains(v, "Python-Web") {
		t.Error("selected card title missing")
	}
	// The view should contain border characters.
	if !strings.Contains(v, "╭") {
		t.Error("expected rounded border in output")
	}
}

func TestCardGridCursorNavUpDown(t *testing.T) {
	t.Parallel()
	m := newCardGridModel(testCards())
	m.width = 100
	m.height = 80
	m.cols = m.calcCols() // 3 columns

	// Start at 0, move down should go to index 3 (next row).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 3 {
		t.Fatalf("down: expected cursor=3, got %d", m.cursor)
	}

	// Move up should go back to 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Fatalf("up: expected cursor=0, got %d", m.cursor)
	}

	// Move up from 0 should clamp to 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 0 {
		t.Fatalf("up clamp: expected cursor=0, got %d", m.cursor)
	}
}

func TestCardGridCursorNavLeftRight(t *testing.T) {
	t.Parallel()
	m := newCardGridModel(testCards())
	m.width = 100
	m.height = 80
	m.cols = m.calcCols()

	// Right moves by 1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.cursor != 1 {
		t.Fatalf("right: expected cursor=1, got %d", m.cursor)
	}

	// Left moves back.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.cursor != 0 {
		t.Fatalf("left: expected cursor=0, got %d", m.cursor)
	}

	// Left from 0 clamps.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.cursor != 0 {
		t.Fatalf("left clamp: expected cursor=0, got %d", m.cursor)
	}
}

func TestCardGridCursorHomeEnd(t *testing.T) {
	t.Parallel()
	m := newCardGridModel(testCards())
	m.width = 100
	m.height = 80
	m.cols = m.calcCols()

	// End goes to last card.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.cursor != len(m.cards)-1 {
		t.Fatalf("end: expected cursor=%d, got %d", len(m.cards)-1, m.cursor)
	}

	// Home goes to first card.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.cursor != 0 {
		t.Fatalf("home: expected cursor=0, got %d", m.cursor)
	}
}

func TestCardGridEnterEmitsMsg(t *testing.T) {
	t.Parallel()
	m := newCardGridModel(testCards())
	m.width = 100
	m.height = 80
	m.cursor = 2
	m.cols = m.calcCols()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should produce a command")
	}
	msg := cmd()
	sel, ok := msg.(cardSelectedMsg)
	if !ok {
		t.Fatalf("expected cardSelectedMsg, got %T", msg)
	}
	if sel.index != 2 {
		t.Fatalf("expected index=2, got %d", sel.index)
	}
}

func TestCardGridEmptyNoPanic(t *testing.T) {
	t.Parallel()
	m := newCardGridModel(nil)
	m.width = 100
	m.height = 40

	// View should not panic.
	v := m.View()
	if v == "" {
		t.Fatal("empty grid should still render something")
	}

	// Update should not panic.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
}

func TestCardGridScroll(t *testing.T) {
	t.Parallel()

	// Create many cards to force scrolling.
	var cards []cardData
	for i := 0; i < 20; i++ {
		cards = append(cards, cardData{
			title: fmt.Sprintf("Card-%d", i),
			lines: []string{"line1"},
		})
	}

	m := newCardGridModel(cards)
	m.width = 50 // 2 columns
	m.height = 15
	m.cols = m.calcCols()

	// Move to the bottom.
	for i := 0; i < 19; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	}

	if m.cursor != 19 {
		t.Fatalf("expected cursor=19, got %d", m.cursor)
	}

	// Scroll offset should have adjusted.
	if m.scrollOffset <= 0 {
		t.Fatal("expected scrollOffset > 0 after navigating to bottom")
	}

	v := m.View()
	// Last card should be visible.
	if !strings.Contains(v, "Card-19") {
		t.Error("last card should be visible after scrolling")
	}

	// Should show scroll indicator.
	if !strings.Contains(v, "more above") {
		t.Error("expected 'more above' scroll indicator")
	}
}

func TestCardGridSingleColumn(t *testing.T) {
	t.Parallel()
	m := newCardGridModel(testCards())
	m.width = 30 // forces single column
	m.height = 80
	m.cols = m.calcCols()

	if m.cols != 1 {
		t.Fatalf("expected 1 column at width 30, got %d", m.cols)
	}

	// Down should move by 1 card (1 column = 1 card per row).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Fatalf("single col down: expected cursor=1, got %d", m.cursor)
	}

	v := m.View()
	if !strings.Contains(v, "Python-Web") {
		t.Error("first card should be in view")
	}
}

func TestCardGridClampCursorBounds(t *testing.T) {
	t.Parallel()
	m := newCardGridModel(testCards())
	m.width = 100
	m.height = 80
	m.cols = m.calcCols()

	// Set cursor past end, then move -- should clamp.
	m.cursor = 100
	m.clampCursor()
	if m.cursor != len(m.cards)-1 {
		t.Fatalf("clamp high: expected %d, got %d", len(m.cards)-1, m.cursor)
	}

	m.cursor = -5
	m.clampCursor()
	if m.cursor != 0 {
		t.Fatalf("clamp low: expected 0, got %d", m.cursor)
	}
}
