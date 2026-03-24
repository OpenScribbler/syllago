package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// itemSelectedMsg is emitted when the cursor moves to a new item.
type itemSelectedMsg struct {
	index int
	item  catalog.ContentItem
}

// itemsModel renders the items list (left pane in Explorer layout).
// It shows a scrollable list of catalog items with name and source columns.
type itemsModel struct {
	items        []catalog.ContentItem
	contentType  catalog.ContentType
	cursor       int
	scrollOffset int
	width        int
	height       int
}

// newItemsModel creates an items list for the given content type.
func newItemsModel(items []catalog.ContentItem, ct catalog.ContentType) itemsModel {
	return itemsModel{
		items:       items,
		contentType: ct,
	}
}

// selectedItem returns a pointer to the currently selected item, or nil if the list is empty.
func (m *itemsModel) selectedItem() *catalog.ContentItem {
	if len(m.items) == 0 {
		return nil
	}
	return &m.items[m.cursor]
}

// visibleCount returns how many items can be displayed given the current height.
// Subtracts 2 lines for the title and column header rows.
func (m *itemsModel) visibleCount() int {
	v := m.height - 2
	if v < 0 {
		return 0
	}
	return v
}

// clampCursor ensures the cursor stays within valid bounds.
func (m *itemsModel) clampCursor() {
	if m.cursor < 0 {
		m.cursor = 0
	}
	if len(m.items) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.items) {
		m.cursor = len(m.items) - 1
	}
}

// clampScroll ensures the scroll offset keeps the cursor visible.
func (m *itemsModel) clampScroll() {
	vis := m.visibleCount()
	if vis <= 0 {
		m.scrollOffset = 0
		return
	}
	// Ensure cursor is within the visible window
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+vis {
		m.scrollOffset = m.cursor - vis + 1
	}
	// Don't scroll past the end
	maxOffset := len(m.items) - vis
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

// Update handles keyboard and mouse input for the items list.
func (m itemsModel) Update(msg tea.Msg) (itemsModel, tea.Cmd) {
	if len(m.items) == 0 {
		return m, nil
	}

	prevCursor := m.cursor

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			m.cursor--
		case key.Matches(msg, keys.Down):
			m.cursor++
		case key.Matches(msg, keys.Home):
			m.cursor = 0
		case key.Matches(msg, keys.End):
			m.cursor = len(m.items) - 1
		case key.Matches(msg, keys.PageUp):
			m.cursor -= m.visibleCount()
		case key.Matches(msg, keys.PageDown):
			m.cursor += m.visibleCount()
		}

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scrollOffset--
			if m.scrollOffset < 0 {
				m.scrollOffset = 0
			}
			return m, nil
		case tea.MouseButtonWheelDown:
			maxOffset := len(m.items) - m.visibleCount()
			if maxOffset < 0 {
				maxOffset = 0
			}
			m.scrollOffset++
			if m.scrollOffset > maxOffset {
				m.scrollOffset = maxOffset
			}
			return m, nil
		case tea.MouseButtonLeft:
			if msg.Action != tea.MouseActionRelease {
				return m, nil
			}
			// Row 0 = title, Row 1 = header, Row 2+ = items
			clickedRow := msg.Y - 2
			if clickedRow >= 0 && clickedRow < m.visibleCount() {
				idx := m.scrollOffset + clickedRow
				if idx >= 0 && idx < len(m.items) {
					m.cursor = idx
				}
			}
		}
	}

	m.clampCursor()
	m.clampScroll()

	if m.cursor != prevCursor {
		return m, func() tea.Msg {
			return itemSelectedMsg{index: m.cursor, item: m.items[m.cursor]}
		}
	}

	return m, nil
}

// View renders the items list with title, column headers, and scrollable item rows.
func (m itemsModel) View() string {
	var b strings.Builder

	// 1. Inline title: ──Skills (5)──────────
	title := fmt.Sprintf("%s (%d)", m.contentType.Label(), len(m.items))
	b.WriteString(inlineTitle(title, m.width, primaryColor))
	b.WriteString("\n")

	// 2. Column headers
	showSource := m.width >= 40
	if showSource {
		nameCol := "Name"
		sourceCol := "Source"
		nameW := m.width*2/3 - 3 // prefix "   " is 3 chars
		if nameW < 4 {
			nameW = 4
		}
		header := "   " + padRight(nameCol, nameW) + sourceCol
		b.WriteString(helpStyle.Render(truncateStr(header, m.width)))
	} else {
		b.WriteString(helpStyle.Render("   Name"))
	}
	b.WriteString("\n")

	// 3. Item rows
	vis := m.visibleCount()
	if vis <= 0 || len(m.items) == 0 {
		return b.String()
	}

	end := m.scrollOffset + vis
	if end > len(m.items) {
		end = len(m.items)
	}

	for i := m.scrollOffset; i < end; i++ {
		item := m.items[i]
		isSelected := i == m.cursor

		var prefix string
		var style lipgloss.Style
		if isSelected {
			prefix = " > "
			style = selectedItemStyle
		} else {
			prefix = "   "
			style = itemStyle
		}

		name := item.Name
		source := item.Source
		if source == "" {
			source = "local"
		}

		if showSource {
			nameW := m.width*2/3 - 3 // match header alignment
			if nameW < 4 {
				nameW = 4
			}
			sourceW := m.width - nameW - 3
			if sourceW < 0 {
				sourceW = 0
			}
			row := prefix + style.Render(padRight(truncateStr(name, nameW), nameW)+truncateStr(source, sourceW))
			b.WriteString(row)
		} else {
			nameW := m.width - 3
			if nameW < 1 {
				nameW = 1
			}
			row := prefix + style.Render(truncateStr(name, nameW))
			b.WriteString(row)
		}

		if i < end-1 {
			b.WriteString("\n")
		}
	}

	// Scroll indicators
	if m.scrollOffset > 0 {
		above := m.scrollOffset
		b.WriteString("\n")
		b.WriteString(helpStyle.Render(fmt.Sprintf("   (%d more above)", above)))
	}
	if end < len(m.items) {
		below := len(m.items) - end
		b.WriteString("\n")
		b.WriteString(helpStyle.Render(fmt.Sprintf("   (%d more below)", below)))
	}

	return b.String()
}

// padRight pads a string with spaces to reach the desired width.
func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
