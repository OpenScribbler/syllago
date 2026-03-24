package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// contentsModel renders a read-only sidebar showing the contents of the
// selected card in the Gallery Grid layout. Items are grouped by type
// (Skills, Rules, etc.) with type headers and indented item names.
type contentsModel struct {
	groups       []contentGroup // items grouped by type
	title        string         // e.g. "Contents (7)"
	scrollOffset int
	width        int
	height       int
}

// contentGroup groups items of a single type for display in the contents sidebar.
type contentGroup struct {
	typeName string   // e.g. "Skills", "Rules"
	items    []string // item names
}

// newContentsModel creates a contents sidebar with the given title and groups.
func newContentsModel(title string, groups []contentGroup) contentsModel {
	return contentsModel{
		title:  title,
		groups: groups,
	}
}

// totalLines returns the total number of renderable lines (group headers + items + blank separators).
func (m contentsModel) totalLines() int {
	n := 0
	for i, g := range m.groups {
		n++ // group header
		n += len(g.items)
		if i < len(m.groups)-1 {
			n++ // blank line separator between groups
		}
	}
	return n
}

// visibleCount returns how many content lines fit in the available height.
// Subtracts 1 for the title row.
func (m contentsModel) visibleCount() int {
	v := m.height - 1
	if v < 0 {
		return 0
	}
	return v
}

// clampScroll ensures the scroll offset stays within valid bounds.
func (m *contentsModel) clampScroll() {
	total := m.totalLines()
	vis := m.visibleCount()
	if vis <= 0 {
		m.scrollOffset = 0
		return
	}
	maxOffset := total - vis
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// Update handles Up/Down scrolling. The contents sidebar is read-only
// (no selection behavior), so only scroll position changes.
func (m contentsModel) Update(msg tea.Msg) (contentsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			m.scrollOffset--
		case key.Matches(msg, keys.Down):
			m.scrollOffset++
		case key.Matches(msg, keys.Home):
			m.scrollOffset = 0
		case key.Matches(msg, keys.End):
			m.scrollOffset = m.totalLines() - m.visibleCount()
		case key.Matches(msg, keys.PageUp):
			m.scrollOffset -= m.visibleCount()
		case key.Matches(msg, keys.PageDown):
			m.scrollOffset += m.visibleCount()
		}

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scrollOffset--
		case tea.MouseButtonWheelDown:
			m.scrollOffset++
		}
	}

	m.clampScroll()
	return m, nil
}

// View renders the contents sidebar with a title, grouped items, and scroll indicators.
func (m contentsModel) View() string {
	var b strings.Builder

	// Title row
	b.WriteString(inlineTitle(m.title, m.width, accentColor))
	b.WriteString("\n")

	// Build all content lines
	var lines []string
	for i, g := range m.groups {
		// Group type header in bold mint
		lines = append(lines, labelStyle.Render(g.typeName))

		// Item names indented
		for _, item := range g.items {
			lines = append(lines, "  "+itemStyle.Render(truncateStr(item, m.width-2)))
		}

		// Blank separator between groups (not after the last)
		if i < len(m.groups)-1 {
			lines = append(lines, "")
		}
	}

	// Apply scroll window
	vis := m.visibleCount()
	if vis <= 0 || len(lines) == 0 {
		return b.String()
	}

	start := m.scrollOffset
	if start < 0 {
		start = 0
	}
	end := start + vis
	if end > len(lines) {
		end = len(lines)
	}

	// Scroll-up indicator
	if start > 0 {
		b.WriteString(helpStyle.Render(fmt.Sprintf("(%d more above)", start)))
		b.WriteString("\n")
		// One fewer visible line because indicator takes a line
		end = start + vis - 1
		if end > len(lines) {
			end = len(lines)
		}
	}

	for i := start; i < end; i++ {
		b.WriteString(lines[i])
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	// Scroll-down indicator
	if end < len(lines) {
		below := len(lines) - end
		b.WriteString("\n")
		b.WriteString(helpStyle.Render(fmt.Sprintf("(%d more below)", below)))
	}

	// Pad remaining height
	rendered := strings.Count(b.String(), "\n") + 1 // +1 for last line without newline
	targetHeight := m.height
	for rendered < targetHeight {
		b.WriteString("\n")
		rendered++
	}

	return lipgloss.NewStyle().Width(m.width).Render(b.String())
}
