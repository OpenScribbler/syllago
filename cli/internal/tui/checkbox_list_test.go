package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCheckboxList_Navigation(t *testing.T) {
	t.Parallel()
	items := []checkboxItem{
		{label: "Alpha"},
		{label: "Beta"},
		{label: "Gamma"},
	}
	c := newCheckboxList(items)
	c.focused = true

	// Cursor starts at 0
	if c.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", c.cursor)
	}

	// Down moves to 1
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyDown})
	if c.cursor != 1 {
		t.Fatalf("expected cursor 1 after Down, got %d", c.cursor)
	}

	// Up from 1 goes to 0
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyUp})
	if c.cursor != 0 {
		t.Fatalf("expected cursor 0 after Up, got %d", c.cursor)
	}

	// Up from 0 stays at 0
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyUp})
	if c.cursor != 0 {
		t.Fatalf("expected cursor 0 after Up at top, got %d", c.cursor)
	}

	// Down past end stays at end
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyDown})
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyDown})
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyDown}) // past end
	if c.cursor != 2 {
		t.Fatalf("expected cursor 2 (clamped), got %d", c.cursor)
	}
}

func TestCheckboxList_Toggle(t *testing.T) {
	t.Parallel()
	items := []checkboxItem{
		{label: "Alpha"},
		{label: "Beta"},
	}
	c := newCheckboxList(items)

	// Space on item 0 selects it
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !c.selected[0] {
		t.Fatal("expected item 0 selected after Space")
	}
	indices := c.SelectedIndices()
	if len(indices) != 1 || indices[0] != 0 {
		t.Fatalf("expected SelectedIndices [0], got %v", indices)
	}

	// Space again deselects
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeySpace})
	if c.selected[0] {
		t.Fatal("expected item 0 deselected after second Space")
	}
	indices = c.SelectedIndices()
	if len(indices) != 0 {
		t.Fatalf("expected SelectedIndices [], got %v", indices)
	}
}

func TestCheckboxList_ToggleDisabled(t *testing.T) {
	t.Parallel()
	items := []checkboxItem{
		{label: "Alpha", disabled: true},
	}
	c := newCheckboxList(items)

	// Space on disabled item: stays false
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeySpace})
	if c.selected[0] {
		t.Fatal("expected disabled item to stay unselected")
	}
}

func TestCheckboxList_SelectAll(t *testing.T) {
	t.Parallel()
	items := []checkboxItem{
		{label: "Alpha"},
		{label: "Beta", disabled: true},
		{label: "Gamma"},
	}
	c := newCheckboxList(items)

	// 'a' selects all non-disabled
	c, _ = c.Update(keyRune('a'))
	if !c.selected[0] {
		t.Fatal("expected item 0 selected")
	}
	if c.selected[1] {
		t.Fatal("expected disabled item 1 to stay unselected")
	}
	if !c.selected[2] {
		t.Fatal("expected item 2 selected")
	}
}

func TestCheckboxList_SelectNone(t *testing.T) {
	t.Parallel()
	items := []checkboxItem{
		{label: "Alpha"},
		{label: "Beta"},
		{label: "Gamma"},
	}
	c := newCheckboxList(items)

	// Pre-select all
	c, _ = c.Update(keyRune('a'))

	// 'n' deselects all
	c, _ = c.Update(keyRune('n'))
	for i, sel := range c.selected {
		if sel {
			t.Fatalf("expected item %d deselected after 'n'", i)
		}
	}
}

func TestCheckboxList_DrillIn(t *testing.T) {
	t.Parallel()
	items := []checkboxItem{
		{label: "Alpha"},
		{label: "Beta"},
		{label: "Gamma"},
	}
	c := newCheckboxList(items)

	// Move to cursor 2
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyDown})
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Enter on cursor 2
	var cmd tea.Cmd
	c, cmd = c.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Enter")
	}
	msg := cmd()
	drill, ok := msg.(checkboxDrillInMsg)
	if !ok {
		t.Fatalf("expected checkboxDrillInMsg, got %T", msg)
	}
	if drill.index != 2 {
		t.Fatalf("expected drill index 2, got %d", drill.index)
	}
}

func TestCheckboxList_Scrolling(t *testing.T) {
	t.Parallel()
	items := make([]checkboxItem, 10)
	for i := range items {
		items[i] = checkboxItem{label: "Item " + itoa(i)}
	}
	c := newCheckboxList(items)
	c = c.SetSize(60, 3)

	// Down x5: cursor at 5, offset should adjust
	for i := 0; i < 5; i++ {
		c, _ = c.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if c.cursor != 5 {
		t.Fatalf("expected cursor 5, got %d", c.cursor)
	}
	// offset should be at least cursor - height + 1 = 3
	if c.offset < 3 {
		t.Fatalf("expected offset >= 3, got %d", c.offset)
	}

	// PgDn: jumps by height
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if c.cursor != 8 {
		t.Fatalf("expected cursor 8 after PgDn, got %d", c.cursor)
	}

	// PgUp: jumps back
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if c.cursor != 5 {
		t.Fatalf("expected cursor 5 after PgUp, got %d", c.cursor)
	}
}

func TestCheckboxList_View(t *testing.T) {
	t.Parallel()
	items := []checkboxItem{
		{label: "Alpha"},
		{label: "Beta"},
		{label: "Gamma"},
	}
	c := newCheckboxList(items)
	c.focused = true
	c = c.SetSize(60, 10)

	// Select item 1
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyDown})
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeySpace})

	view := c.View()

	// Item 1 selected should have [x]
	if !strings.Contains(view, "[x]") {
		t.Fatal("expected [x] for selected item")
	}

	// Others should have [ ]
	lines := strings.Split(view, "\n")
	checkCount := 0
	for _, line := range lines {
		if strings.Contains(line, "[ ]") {
			checkCount++
		}
	}
	if checkCount != 2 {
		t.Fatalf("expected 2 lines with [ ], got %d", checkCount)
	}

	// Cursor indicator > on focused row (cursor=1)
	if !strings.Contains(lines[1], ">") {
		t.Fatal("expected > cursor indicator on focused row")
	}
}

func TestCheckboxList_ViewDisabled(t *testing.T) {
	t.Parallel()
	items := []checkboxItem{
		{label: "Alpha"},
		{label: "Beta", disabled: true, badge: "locked", badgeStyle: badgeStyleMuted},
	}
	c := newCheckboxList(items)
	c = c.SetSize(60, 10)

	view := c.View()

	// Disabled item renders [-]
	if !strings.Contains(view, "[-]") {
		t.Fatal("expected [-] for disabled item")
	}
}

func TestCheckboxList_View_SanitizesEscapeCodes(t *testing.T) {
	t.Parallel()
	items := []checkboxItem{
		{label: "test\x1b[1mbold"},
	}
	c := newCheckboxList(items)
	c = c.SetSize(60, 10)

	view := c.View()

	// Should contain "testbold" with escape stripped
	if !strings.Contains(view, "testbold") {
		t.Fatalf("expected sanitized label 'testbold' in view, got: %s", view)
	}
	// Should NOT contain the raw escape
	if strings.Contains(view, "\x1b") {
		t.Fatal("expected ANSI escape to be stripped")
	}
}
