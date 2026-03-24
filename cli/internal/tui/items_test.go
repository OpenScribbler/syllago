package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func testItems() []catalog.ContentItem {
	return []catalog.ContentItem{
		{Name: "alpha-skill", Type: catalog.Skills, Source: "team-rules"},
		{Name: "beta-skill", Type: catalog.Skills, Source: "library"},
		{Name: "gamma-skill", Type: catalog.Skills, Source: "project"},
		{Name: "delta-skill", Type: catalog.Skills, Source: "my-registry"},
		{Name: "epsilon-skill", Type: catalog.Skills, Source: "global"},
	}
}

func TestItemsModel_Navigation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		key        tea.KeyMsg
		wantCursor int
	}{
		{
			name:       "down moves cursor",
			key:        tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
			wantCursor: 1,
		},
		{
			name:       "up at top stays at 0",
			key:        tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}},
			wantCursor: 0,
		},
		{
			name:       "end goes to last item",
			key:        tea.KeyMsg{Type: tea.KeyEnd},
			wantCursor: 4,
		},
		{
			name:       "home goes to first item",
			key:        tea.KeyMsg{Type: tea.KeyHome},
			wantCursor: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := newItemsModel(testItems(), catalog.Skills)
			m.width = 80
			m.height = 20

			updated, _ := m.Update(tt.key)
			if updated.cursor != tt.wantCursor {
				t.Errorf("cursor = %d, want %d", updated.cursor, tt.wantCursor)
			}
		})
	}
}

func TestItemsModel_ScrollTriggered(t *testing.T) {
	t.Parallel()

	m := newItemsModel(testItems(), catalog.Skills)
	m.width = 80
	m.height = 5 // title(1) + header(1) + 3 visible items

	// Move cursor down past visible area
	for i := 0; i < 4; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	if m.scrollOffset == 0 {
		t.Error("expected scroll offset > 0 when cursor moves past visible area")
	}
	if m.cursor != 4 {
		t.Errorf("cursor = %d, want 4", m.cursor)
	}
}

func TestItemsModel_EmptyList(t *testing.T) {
	t.Parallel()

	m := newItemsModel(nil, catalog.Skills)
	m.width = 80
	m.height = 20

	// Should not panic
	view := m.View()
	if !strings.Contains(view, "Skills (0)") {
		t.Error("empty list should show count of 0")
	}

	// Navigation on empty list should not panic
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Error("navigation on empty list should not emit a command")
	}
	if updated.cursor != 0 {
		t.Errorf("cursor = %d, want 0 on empty list", updated.cursor)
	}
}

func TestItemsModel_SelectedItem(t *testing.T) {
	t.Parallel()

	t.Run("non-empty", func(t *testing.T) {
		t.Parallel()
		m := newItemsModel(testItems(), catalog.Skills)
		item := m.selectedItem()
		if item == nil {
			t.Fatal("selectedItem() returned nil for non-empty list")
		}
		if item.Name != "alpha-skill" {
			t.Errorf("selectedItem().Name = %s, want alpha-skill", item.Name)
		}
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		m := newItemsModel(nil, catalog.Skills)
		if m.selectedItem() != nil {
			t.Error("selectedItem() should return nil for empty list")
		}
	})
}

func TestItemsModel_RenderWide(t *testing.T) {
	t.Parallel()

	m := newItemsModel(testItems(), catalog.Skills)
	m.width = 80
	m.height = 20

	view := m.View()

	// Should show title with count
	if !strings.Contains(view, "Skills (5)") {
		t.Error("view should contain 'Skills (5)' title")
	}

	// Should show column headers
	if !strings.Contains(view, "Name") || !strings.Contains(view, "Source") {
		t.Error("wide view should show Name and Source headers")
	}

	// Should show items with source
	if !strings.Contains(view, "alpha-skill") {
		t.Error("view should contain item names")
	}
	if !strings.Contains(view, "team-rules") {
		t.Error("wide view should show source column")
	}

	// Should have cursor prefix on first item
	if !strings.Contains(view, " > ") {
		t.Error("view should show cursor prefix '>' on selected item")
	}
}

func TestItemsModel_RenderNarrow(t *testing.T) {
	t.Parallel()

	m := newItemsModel(testItems(), catalog.Skills)
	m.width = 35 // < 40, should hide Source column
	m.height = 20

	view := m.View()

	// Should show Name header but not Source
	if !strings.Contains(view, "Name") {
		t.Error("narrow view should show Name header")
	}

	// Source column header should be absent
	lines := strings.Split(view, "\n")
	headerLine := ""
	if len(lines) > 1 {
		headerLine = lines[1]
	}
	if strings.Contains(headerLine, "Source") {
		t.Error("narrow view (width < 40) should NOT show Source header")
	}
}

func TestItemsModel_SelectionMessage(t *testing.T) {
	t.Parallel()

	m := newItemsModel(testItems(), catalog.Skills)
	m.width = 80
	m.height = 20

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd == nil {
		t.Fatal("expected a command when cursor moves")
	}

	msg := cmd()
	sel, ok := msg.(itemSelectedMsg)
	if !ok {
		t.Fatalf("expected itemSelectedMsg, got %T", msg)
	}
	if sel.index != 1 {
		t.Errorf("itemSelectedMsg.index = %d, want 1", sel.index)
	}
	if sel.item.Name != "beta-skill" {
		t.Errorf("itemSelectedMsg.item.Name = %s, want beta-skill", sel.item.Name)
	}
}

func TestItemsModel_NoMessageWhenCursorUnchanged(t *testing.T) {
	t.Parallel()

	m := newItemsModel(testItems(), catalog.Skills)
	m.width = 80
	m.height = 20

	// Pressing up at top should not change cursor and should not emit
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if cmd != nil {
		t.Error("should not emit a command when cursor doesn't move")
	}
}

func TestItemsModel_PageUpDown(t *testing.T) {
	t.Parallel()

	m := newItemsModel(testItems(), catalog.Skills)
	m.width = 80
	m.height = 5 // visibleCount = 3

	// PageDown should jump by visibleCount
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if updated.cursor != 3 {
		t.Errorf("after PageDown, cursor = %d, want 3", updated.cursor)
	}

	// PageDown again should clamp to last item
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if updated.cursor != 4 {
		t.Errorf("after second PageDown, cursor = %d, want 4 (clamped)", updated.cursor)
	}

	// PageUp should jump back
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if updated.cursor != 1 {
		t.Errorf("after PageUp, cursor = %d, want 1", updated.cursor)
	}
}

func TestItemsModel_MouseClick(t *testing.T) {
	t.Parallel()

	m := newItemsModel(testItems(), catalog.Skills)
	m.width = 80
	m.height = 20

	// Click on row index 1 (Y=3 because title=0, header=1, first item=2, second item=3)
	updated, cmd := m.Update(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
		Y:      3,
	})

	if updated.cursor != 1 {
		t.Errorf("cursor = %d, want 1 after clicking row 1", updated.cursor)
	}
	if cmd == nil {
		t.Error("expected selection message after mouse click")
	}
}

func TestItemsModel_EmptySource(t *testing.T) {
	t.Parallel()

	items := []catalog.ContentItem{
		{Name: "no-source-item", Type: catalog.Skills, Source: ""},
	}
	m := newItemsModel(items, catalog.Skills)
	m.width = 80
	m.height = 20

	view := m.View()
	if !strings.Contains(view, "local") {
		t.Error("items with empty source should display 'local'")
	}
}

func TestItemsModel_ScrollIndicators(t *testing.T) {
	t.Parallel()

	m := newItemsModel(testItems(), catalog.Skills)
	m.width = 80
	m.height = 5 // visibleCount = 3, so 2 items hidden below

	view := m.View()
	if !strings.Contains(view, "more below") {
		t.Error("should show 'more below' indicator when items are hidden below")
	}

	// Scroll to bottom
	for i := 0; i < 4; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	view = m.View()
	if !strings.Contains(view, "more above") {
		t.Error("should show 'more above' indicator when items are hidden above")
	}
}
