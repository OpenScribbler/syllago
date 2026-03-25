package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// contentGroup holds items of a single type within a card.
type contentGroup struct {
	typeName string
	items    []catalog.ContentItem
}

// contentsSidebarModel shows grouped items inside the selected card.
type contentsSidebarModel struct {
	groups  []contentGroup
	offset  int
	width   int
	height  int
	focused bool
}

func newContentsSidebarModel() contentsSidebarModel {
	return contentsSidebarModel{}
}

// SetSize updates dimensions.
func (m *contentsSidebarModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetCard updates the sidebar to show items from the given card.
func (m *contentsSidebarModel) SetCard(card *cardData) {
	m.offset = 0
	m.groups = nil
	if card == nil || len(card.items) == 0 {
		return
	}

	// Group items by type
	byType := make(map[string][]catalog.ContentItem)
	var order []string
	for _, item := range card.items {
		label := item.Type.Label()
		if _, exists := byType[label]; !exists {
			order = append(order, label)
		}
		byType[label] = append(byType[label], item)
	}

	for _, label := range order {
		m.groups = append(m.groups, contentGroup{
			typeName: label,
			items:    byType[label],
		})
	}
}

// SetGroups directly sets content groups from a counts map (for loadouts
// where we don't have resolved items, just names from the manifest).
func (m *contentsSidebarModel) SetGroups(groups []contentGroup) {
	m.offset = 0
	m.groups = groups
}

// ScrollUp scrolls the sidebar up one line.
func (m *contentsSidebarModel) ScrollUp() {
	if m.offset > 0 {
		m.offset--
	}
}

// ScrollDown scrolls the sidebar down one line.
func (m *contentsSidebarModel) ScrollDown() {
	total := m.totalLines()
	if m.offset < total-m.height {
		m.offset++
	}
}

// totalLines returns the total number of rendered lines.
func (m contentsSidebarModel) totalLines() int {
	n := 0
	for _, g := range m.groups {
		n++ // header
		n += len(g.items)
	}
	return n
}

// View renders the contents sidebar.
func (m contentsSidebarModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	if len(m.groups) == 0 {
		return lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(mutedColor).
			Render("Select a card")
	}

	// Build all lines first, then slice for scrolling
	var allLines []string
	for _, g := range m.groups {
		header := boldStyle.Render(g.typeName)
		allLines = append(allLines, header)
		for _, item := range g.items {
			name := itemDisplayName(item)
			line := "  " + truncate(sanitizeLine(name), max(0, m.width-2))
			allLines = append(allLines, mutedStyle.Render(line))
		}
	}

	// Apply scroll offset
	start := min(m.offset, len(allLines))
	end := min(start+m.height, len(allLines))
	visible := allLines[start:end]

	// Pad to full height
	for len(visible) < m.height {
		visible = append(visible, strings.Repeat(" ", m.width))
	}

	// Clamp line width
	for i, line := range visible {
		visible[i] = lipgloss.NewStyle().MaxWidth(m.width).Render(line)
		if g := m.width - lipgloss.Width(visible[i]); g > 0 {
			visible[i] += strings.Repeat(" ", g)
		}
	}

	return strings.Join(visible, "\n")
}
