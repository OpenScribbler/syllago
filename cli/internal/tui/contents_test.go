package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testGroups() []contentGroup {
	return []contentGroup{
		{typeName: "Skills", items: []string{"Refactor-Python", "Py-Doc-Gen", "Django-Patterns"}},
		{typeName: "Rules", items: []string{"Strict-Types", "PEP8-Lint"}},
		{typeName: "Agents", items: []string{"Code-Reviewer"}},
	}
}

func TestContentsModelRendersGroups(t *testing.T) {
	t.Parallel()

	m := newContentsModel("Contents (6)", testGroups())
	m.width = 30
	m.height = 20

	v := m.View()

	// Title should appear
	if !strings.Contains(v, "Contents (6)") {
		t.Error("view should contain title")
	}

	// All group type names should appear
	for _, g := range testGroups() {
		if !strings.Contains(v, g.typeName) {
			t.Errorf("view missing group type %q", g.typeName)
		}
	}

	// All item names should appear
	for _, g := range testGroups() {
		for _, item := range g.items {
			if !strings.Contains(v, item) {
				t.Errorf("view missing item %q", item)
			}
		}
	}
}

func TestContentsModelScrollDown(t *testing.T) {
	t.Parallel()

	// Small height forces scrolling
	m := newContentsModel("Contents (6)", testGroups())
	m.width = 30
	m.height = 5 // title + 4 lines visible

	// Scroll down a few times
	for i := 0; i < 3; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	if m.scrollOffset <= 0 {
		t.Error("scrollOffset should be > 0 after scrolling down")
	}
}

func TestContentsModelScrollUp(t *testing.T) {
	t.Parallel()

	m := newContentsModel("Contents (6)", testGroups())
	m.width = 30
	m.height = 5

	// Scroll down then up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})

	if m.scrollOffset != 1 {
		t.Errorf("expected scrollOffset=1 after down-down-up, got %d", m.scrollOffset)
	}
}

func TestContentsModelScrollClamp(t *testing.T) {
	t.Parallel()

	m := newContentsModel("Contents (6)", testGroups())
	m.width = 30
	m.height = 5

	// Scroll up from the top should clamp to 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.scrollOffset != 0 {
		t.Errorf("scroll up from top: expected 0, got %d", m.scrollOffset)
	}

	// Scroll down past the end should clamp
	for i := 0; i < 50; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	total := m.totalLines()
	vis := m.visibleCount()
	maxOffset := total - vis
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset != maxOffset {
		t.Errorf("scroll past end: expected %d, got %d", maxOffset, m.scrollOffset)
	}
}

func TestContentsModelEmptyGroups(t *testing.T) {
	t.Parallel()

	// No groups — should not panic
	m := newContentsModel("Contents (0)", nil)
	m.width = 30
	m.height = 10

	v := m.View()
	if !strings.Contains(v, "Contents (0)") {
		t.Error("empty contents should still show title")
	}

	// Update should not panic
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
}

func TestContentsModelEmptyGroupItems(t *testing.T) {
	t.Parallel()

	// Group with no items — should not panic
	groups := []contentGroup{
		{typeName: "Skills", items: nil},
		{typeName: "Rules", items: []string{}},
	}
	m := newContentsModel("Contents (0)", groups)
	m.width = 30
	m.height = 10

	v := m.View()
	if !strings.Contains(v, "Skills") {
		t.Error("empty group should still show type name")
	}
}

func TestContentsModelTotalLines(t *testing.T) {
	t.Parallel()

	m := newContentsModel("Contents (6)", testGroups())
	// Skills header + 3 items + blank + Rules header + 2 items + blank + Agents header + 1 item = 10
	want := 1 + 3 + 1 + 1 + 2 + 1 + 1 + 1
	got := m.totalLines()
	if got != want {
		t.Errorf("totalLines = %d, want %d", got, want)
	}
}

func TestContentsModelMouseScroll(t *testing.T) {
	t.Parallel()

	m := newContentsModel("Contents (6)", testGroups())
	m.width = 30
	m.height = 5

	// Mouse wheel down
	m, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	if m.scrollOffset != 1 {
		t.Errorf("wheel down: expected scrollOffset=1, got %d", m.scrollOffset)
	}

	// Mouse wheel up
	m, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	if m.scrollOffset != 0 {
		t.Errorf("wheel up: expected scrollOffset=0, got %d", m.scrollOffset)
	}
}

func TestContentsModelHomeEnd(t *testing.T) {
	t.Parallel()

	m := newContentsModel("Contents (6)", testGroups())
	m.width = 30
	m.height = 5

	// End should scroll to bottom
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	total := m.totalLines()
	vis := m.visibleCount()
	maxOffset := total - vis
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset != maxOffset {
		t.Errorf("end: expected scrollOffset=%d, got %d", maxOffset, m.scrollOffset)
	}

	// Home should scroll to top
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.scrollOffset != 0 {
		t.Errorf("home: expected scrollOffset=0, got %d", m.scrollOffset)
	}
}
