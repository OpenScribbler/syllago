package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestPreviewModel_RendersLineNumbers(t *testing.T) {
	t.Parallel()
	m := newPreviewModel("test.md", "line one\nline two\nline three")
	m.width = 60
	m.height = 10

	view := m.View()
	// Check line numbers appear right-aligned in 3-char field
	if !strings.Contains(view, "  1") {
		t.Error("expected line number 1")
	}
	if !strings.Contains(view, "  2") {
		t.Error("expected line number 2")
	}
	if !strings.Contains(view, "  3") {
		t.Error("expected line number 3")
	}
	// Check content appears
	if !strings.Contains(view, "line one") {
		t.Error("expected 'line one' in output")
	}
	if !strings.Contains(view, "line three") {
		t.Error("expected 'line three' in output")
	}
}

func TestPreviewModel_InlineTitle(t *testing.T) {
	t.Parallel()
	m := newPreviewModel("config.yaml", "key: value")
	m.width = 40
	m.height = 5

	view := m.View()
	if !strings.Contains(view, "Preview: config.yaml") {
		t.Error("expected inline title with filename")
	}
}

func TestPreviewModel_ScrollIndicators(t *testing.T) {
	t.Parallel()
	// Create content that overflows viewport
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "content line"
	}
	m := newPreviewModel("big.txt", strings.Join(lines, "\n"))
	m.width = 40
	m.height = 7 // title + 5 visible lines + room is tight

	// At top: should show "lines below" but not "lines above"
	view := m.View()
	if strings.Contains(view, "lines above") {
		t.Error("should not show 'lines above' at top")
	}
	if !strings.Contains(view, "lines below") {
		t.Error("expected 'lines below' indicator")
	}

	// Scroll down to middle
	m.scrollOffset = 5
	m.clampScroll()
	view = m.View()
	if !strings.Contains(view, "5 lines above") {
		t.Error("expected '5 lines above' indicator")
	}
	if !strings.Contains(view, "lines below") {
		t.Error("expected 'lines below' indicator")
	}
}

func TestPreviewModel_EmptyContent(t *testing.T) {
	t.Parallel()
	m := newPreviewModel("empty.txt", "")
	m.width = 40
	m.height = 10

	view := m.View()
	// Should render title without panicking
	if !strings.Contains(view, "Preview: empty.txt") {
		t.Error("expected title for empty content")
	}
	// Should not contain any line numbers
	if strings.Contains(view, "  1") {
		t.Error("should not have line numbers for empty content")
	}
}

func TestPreviewModel_Truncation(t *testing.T) {
	t.Parallel()
	longLine := strings.Repeat("x", 200)
	m := newPreviewModel("long.txt", longLine)
	m.width = 30
	m.height = 5

	view := m.View()
	// The rendered view width should not exceed model width.
	// Check that the long line was truncated (contains "...")
	if !strings.Contains(view, "...") {
		t.Error("expected truncation ellipsis for long line")
	}
	// Verify no rendered line exceeds width (check raw text lines after title)
	for _, line := range strings.Split(view, "\n") {
		w := lipgloss.Width(line)
		if w > m.width {
			t.Errorf("line exceeds width %d: got %d: %q", m.width, w, line)
		}
	}
}

func TestPreviewModel_ScrollNavigation(t *testing.T) {
	t.Parallel()
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line content"
	}
	m := newPreviewModel("scroll.txt", strings.Join(lines, "\n"))
	m.width = 40
	m.height = 10

	// Scroll down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.scrollOffset != 1 {
		t.Errorf("expected scrollOffset=1 after Down, got %d", m.scrollOffset)
	}

	// Scroll up back to top
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 after Up, got %d", m.scrollOffset)
	}

	// Can't scroll above 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 when already at top, got %d", m.scrollOffset)
	}

	// Home goes to top
	m.scrollOffset = 10
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 after Home, got %d", m.scrollOffset)
	}

	// End goes to bottom
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.scrollOffset == 0 {
		t.Error("expected scrollOffset > 0 after End")
	}
	// After End, scrolling down should not change offset (already at max)
	endOffset := m.scrollOffset
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.scrollOffset != endOffset {
		t.Errorf("expected scrollOffset=%d after Down at end, got %d", endOffset, m.scrollOffset)
	}
}

func TestPreviewModel_SetContent(t *testing.T) {
	t.Parallel()
	m := newPreviewModel("old.txt", "old content")
	m.width = 40
	m.height = 10
	m.scrollOffset = 5

	m.setContent("new.txt", "new line 1\nnew line 2")

	if m.filename != "new.txt" {
		t.Errorf("expected filename=new.txt, got %s", m.filename)
	}
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset reset to 0, got %d", m.scrollOffset)
	}
	if len(m.lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(m.lines))
	}

	view := m.View()
	if !strings.Contains(view, "Preview: new.txt") {
		t.Error("expected updated title")
	}
	if !strings.Contains(view, "new line 1") {
		t.Error("expected new content in view")
	}
}

func TestPreviewModel_ZeroSize(t *testing.T) {
	t.Parallel()
	m := newPreviewModel("test.txt", "content")
	m.width = 0
	m.height = 0

	view := m.View()
	if view != "" {
		t.Errorf("expected empty view for zero size, got %q", view)
	}
}

func TestPreviewModel_PageUpDown(t *testing.T) {
	t.Parallel()
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "line"
	}
	m := newPreviewModel("pages.txt", strings.Join(lines, "\n"))
	m.width = 40
	m.height = 12

	// PageDown should jump by visible lines count
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.scrollOffset == 0 {
		t.Error("expected scrollOffset > 0 after PageDown")
	}
	firstPage := m.scrollOffset

	// PageUp should go back
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.scrollOffset >= firstPage {
		t.Errorf("expected scrollOffset < %d after PageUp, got %d", firstPage, m.scrollOffset)
	}
}

func TestPreviewModel_MouseWheel(t *testing.T) {
	t.Parallel()
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line"
	}
	m := newPreviewModel("mouse.txt", strings.Join(lines, "\n"))
	m.width = 40
	m.height = 10

	// Wheel down
	m, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	if m.scrollOffset != 1 {
		t.Errorf("expected scrollOffset=1 after wheel down, got %d", m.scrollOffset)
	}

	// Wheel up
	m, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	if m.scrollOffset != 0 {
		t.Errorf("expected scrollOffset=0 after wheel up, got %d", m.scrollOffset)
	}
}
