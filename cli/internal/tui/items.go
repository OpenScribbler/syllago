package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// itemsModel renders a scrollable list of catalog items with cursor navigation.
type itemsModel struct {
	items    []catalog.ContentItem
	cursor   int // selected index
	offset   int // scroll offset (first visible index)
	width    int
	height   int
	focused  bool
	mixed    bool                  // true when showing mixed content types (Library view)
	search   string                // active search query (empty = no filter)
	allItems []catalog.ContentItem // unfiltered items (for search reset)
}

func newItemsModel(items []catalog.ContentItem, mixed bool) itemsModel {
	return itemsModel{
		items:    items,
		allItems: items,
		mixed:    mixed,
	}
}

// SetSize updates the items list dimensions.
func (m *itemsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetItems replaces the item list and resets cursor.
func (m *itemsModel) SetItems(items []catalog.ContentItem, mixed bool) {
	m.items = items
	m.allItems = items
	m.mixed = mixed
	m.cursor = 0
	m.offset = 0
	m.search = ""
}

// Selected returns the currently selected item, or nil if the list is empty.
func (m itemsModel) Selected() *catalog.ContentItem {
	if len(m.items) == 0 {
		return nil
	}
	if m.cursor >= 0 && m.cursor < len(m.items) {
		return &m.items[m.cursor]
	}
	return nil
}

// Len returns the number of visible items.
func (m itemsModel) Len() int {
	return len(m.items)
}

// CursorUp moves the cursor up one item.
func (m *itemsModel) CursorUp() {
	if len(m.items) == 0 {
		return
	}
	m.cursor--
	if m.cursor < 0 {
		m.cursor = len(m.items) - 1 // wrap
		m.offset = max(0, len(m.items)-m.height)
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
}

// CursorDown moves the cursor down one item.
func (m *itemsModel) CursorDown() {
	if len(m.items) == 0 {
		return
	}
	m.cursor++
	if m.cursor >= len(m.items) {
		m.cursor = 0 // wrap
		m.offset = 0
	}
	if m.cursor >= m.offset+m.height {
		m.offset = m.cursor - m.height + 1
	}
}

// PageUp moves cursor up by one page.
func (m *itemsModel) PageUp() {
	m.cursor = max(0, m.cursor-m.height)
	m.offset = max(0, m.offset-m.height)
}

// PageDown moves cursor down by one page.
func (m *itemsModel) PageDown() {
	m.cursor = min(len(m.items)-1, m.cursor+m.height)
	maxOffset := max(0, len(m.items)-m.height)
	m.offset = min(maxOffset, m.offset+m.height)
}

// ApplySearch filters items across name, display name, description, type, and source.
func (m *itemsModel) ApplySearch(query string) {
	m.search = query
	if query == "" {
		m.items = m.allItems
	} else {
		q := strings.ToLower(query)
		filtered := make([]catalog.ContentItem, 0)
		for _, item := range m.allItems {
			if strings.Contains(strings.ToLower(item.Name), q) ||
				strings.Contains(strings.ToLower(item.DisplayName), q) ||
				strings.Contains(strings.ToLower(item.Description), q) ||
				strings.Contains(strings.ToLower(string(item.Type)), q) ||
				strings.Contains(strings.ToLower(item.Source), q) {
				filtered = append(filtered, item)
			}
		}
		m.items = filtered
	}
	m.cursor = 0
	m.offset = 0
}

// ClearSearch removes the search filter and restores all items.
func (m *itemsModel) ClearSearch() {
	m.ApplySearch("")
}

// View renders the items list.
func (m itemsModel) View() string {
	if m.height <= 0 || m.width <= 0 {
		return ""
	}

	if len(m.items) == 0 {
		return m.renderEmpty()
	}

	visibleCount := min(m.height, len(m.items))
	lastVisible := min(m.offset+visibleCount, len(m.items))

	// Scroll indicators
	itemsAbove := m.offset
	itemsBelow := max(0, len(m.items)-lastVisible)
	showAbove := itemsAbove > 0
	showBelow := itemsBelow > 0

	// Adjust visible range to make room for indicators
	contentStart := m.offset
	contentEnd := lastVisible
	if showAbove {
		contentStart++
	}
	if showBelow && contentEnd > contentStart {
		contentEnd--
	}

	lines := make([]string, 0, m.height)

	if showAbove {
		indicator := fmt.Sprintf("(%d more above)", itemsAbove)
		lines = append(lines, mutedStyle.Render(indicator))
	}

	for i := contentStart; i < contentEnd; i++ {
		lines = append(lines, m.renderItem(i))
	}

	if showBelow {
		indicator := fmt.Sprintf("(%d more below)", itemsBelow)
		lines = append(lines, mutedStyle.Render(indicator))
	}

	// Pad remaining height
	for len(lines) < m.height {
		lines = append(lines, strings.Repeat(" ", m.width))
	}

	return strings.Join(lines, "\n")
}

// renderItem renders a single list row.
func (m itemsModel) renderItem(index int) string {
	item := m.items[index]
	isCursor := index == m.cursor

	// Build the display text
	var text string
	if isCursor {
		text = " > "
	} else {
		text = "   "
	}

	name := item.Name
	if item.DisplayName != "" {
		name = item.DisplayName
	}

	// Source column
	source := item.Source
	if source == "" && item.Registry != "" {
		source = item.Registry
	}

	// Type badge for mixed views
	typeBadge := ""
	if m.mixed {
		typeBadge = " " + mutedStyle.Render("["+string(item.Type)+"]")
	}

	// Calculate available width for name and source
	prefixW := 3 // " > " or "   "
	badgeW := 0
	if m.mixed {
		badgeW = lipgloss.Width(typeBadge)
	}

	if source != "" && m.width >= 30 {
		// Two-column: name + source
		sourceMaxW := min(14, m.width/3)
		nameMaxW := m.width - prefixW - sourceMaxW - badgeW - 2 // 2 for gap

		name = truncate(name, nameMaxW)
		source = truncate(source, sourceMaxW)

		gap := max(1, m.width-prefixW-lipgloss.Width(name)-lipgloss.Width(source)-badgeW)
		text += name + strings.Repeat(" ", gap) + mutedStyle.Render(source) + typeBadge
	} else {
		// Single column: name only
		nameMaxW := m.width - prefixW - badgeW
		name = truncate(name, nameMaxW)
		text += name + typeBadge
		// Pad to full width
		textW := lipgloss.Width(text)
		if textW < m.width {
			text += strings.Repeat(" ", m.width-textW)
		}
	}

	var row string
	if isCursor && m.focused {
		row = selectedRowStyle.Width(m.width).Render(text)
	} else if isCursor {
		row = boldStyle.Width(m.width).Render(text)
	} else {
		row = lipgloss.NewStyle().Width(m.width).Render(text)
	}
	return zone.Mark("item-"+itoa(index), row)
}

// renderEmpty shows guidance when no items exist.
func (m itemsModel) renderEmpty() string {
	msg := "No items found."
	if m.search != "" {
		msg = "No matches for \"" + m.search + "\"."
	}
	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(mutedColor).
		Render(msg)
}

// truncate shortens a string to fit within maxWidth, adding "..." if needed.
func truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if len(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return s[:maxWidth]
	}
	return s[:maxWidth-3] + "..."
}

// truncateLine hard-clips a line to maxWidth characters. Handles tabs by
// expanding them to spaces first. No ellipsis — just clip for preview content.
func truncateLine(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	// Expand tabs to 4 spaces for consistent width
	s = strings.ReplaceAll(s, "\t", "    ")
	// Strip any carriage returns
	s = strings.ReplaceAll(s, "\r", "")
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return string(runes)
	}
	return string(runes[:maxWidth])
}
