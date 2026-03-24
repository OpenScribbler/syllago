package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// cardSelectedMsg is emitted when the user presses Enter on a card.
type cardSelectedMsg struct {
	index int
}

// cardData holds the display data for a single card.
type cardData struct {
	title    string
	subtitle string         // e.g. URL for registries
	lines    []string       // summary lines (item counts, target, etc.)
	contents []contentGroup // detailed contents for the gallery sidebar
}

// cardGridModel renders cards in a responsive grid with cursor navigation.
type cardGridModel struct {
	cards        []cardData
	cursor       int
	scrollOffset int // rows scrolled (not cards)
	cols         int // number of columns (calculated from width)
	width        int
	height       int
}

// newCardGridModel creates a card grid from the given card data.
func newCardGridModel(cards []cardData) cardGridModel {
	return cardGridModel{
		cards: cards,
	}
}

// calcCols determines the number of columns based on available width.
// 3 columns at width >= 90, 2 at width >= 45, 1 below 45.
func (m *cardGridModel) calcCols() int {
	switch {
	case m.width >= 90:
		return 3
	case m.width >= 45:
		return 2
	default:
		return 1
	}
}

// cardWidth returns the width available for each card, accounting for gaps.
// Gaps between columns are 1 character wide.
func (m *cardGridModel) cardWidth() int {
	cols := m.cols
	if cols <= 0 {
		cols = 1
	}
	gaps := cols - 1
	w := (m.width - gaps) / cols
	if w < 10 {
		w = 10
	}
	return w
}

// totalRows returns the number of rows needed for all cards.
func (m *cardGridModel) totalRows() int {
	if len(m.cards) == 0 {
		return 0
	}
	cols := m.cols
	if cols <= 0 {
		cols = 1
	}
	return (len(m.cards) + cols - 1) / cols
}

// visibleRows returns how many rows fit in the available height.
// Each card has a fixed rendered height; we estimate based on that.
func (m *cardGridModel) visibleRows() int {
	rh := m.rowHeight()
	if rh <= 0 {
		return 1
	}
	rows := m.height / rh
	if rows < 1 {
		rows = 1
	}
	return rows
}

// rowHeight returns the rendered height of a card row.
// Cards have: 2 border lines + title + separator + lines (minimum 1).
// For multi-column grids we use a fixed height so rows align.
func (m *cardGridModel) rowHeight() int {
	if len(m.cards) == 0 {
		return 4
	}
	// Find max lines across all cards for uniform height.
	maxLines := 1
	for _, c := range m.cards {
		n := len(c.lines)
		if c.subtitle != "" {
			n++ // subtitle takes a line
		}
		if n > maxLines {
			maxLines = n
		}
	}
	// 2 (border top+bottom) + 1 (title) + 1 (separator) + maxLines
	return 2 + 1 + 1 + maxLines
}

// clampCursor ensures cursor is within valid range.
func (m *cardGridModel) clampCursor() {
	if len(m.cards) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.cards) {
		m.cursor = len(m.cards) - 1
	}
}

// clampScroll ensures scroll offset is within valid range and the cursor row is visible.
func (m *cardGridModel) clampScroll() {
	total := m.totalRows()
	visible := m.visibleRows()

	// Can't scroll past the last row.
	maxOffset := total - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}

	// Ensure cursor row is visible.
	cols := m.cols
	if cols <= 0 {
		cols = 1
	}
	cursorRow := m.cursor / cols
	if cursorRow < m.scrollOffset {
		m.scrollOffset = cursorRow
	}
	if cursorRow >= m.scrollOffset+visible {
		m.scrollOffset = cursorRow - visible + 1
	}
}

// Update handles keyboard navigation for the card grid.
func (m cardGridModel) Update(msg tea.Msg) (cardGridModel, tea.Cmd) {
	if len(m.cards) == 0 {
		return m, nil
	}

	m.cols = m.calcCols()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			m.cursor -= m.cols
			m.clampCursor()
			m.clampScroll()

		case key.Matches(msg, keys.Down):
			m.cursor += m.cols
			m.clampCursor()
			m.clampScroll()

		case key.Matches(msg, keys.Left):
			m.cursor--
			m.clampCursor()
			m.clampScroll()

		case key.Matches(msg, keys.Right):
			m.cursor++
			m.clampCursor()
			m.clampScroll()

		case key.Matches(msg, keys.Home):
			m.cursor = 0
			m.clampScroll()

		case key.Matches(msg, keys.End):
			m.cursor = len(m.cards) - 1
			m.clampScroll()

		case key.Matches(msg, keys.Enter):
			return m, func() tea.Msg {
				return cardSelectedMsg{index: m.cursor}
			}
		}
	}

	return m, nil
}

// View renders the card grid.
func (m cardGridModel) View() string {
	if len(m.cards) == 0 {
		return helpStyle.Render("  No cards to display")
	}

	m.cols = m.calcCols()
	cw := m.cardWidth()
	total := m.totalRows()
	visible := m.visibleRows()

	var rows []string

	// Scroll indicators above.
	if m.scrollOffset > 0 {
		above := m.scrollOffset
		rows = append(rows, helpStyle.Render(fmt.Sprintf("  (%d more above)", above)))
	}

	// Render visible rows.
	endRow := m.scrollOffset + visible
	if endRow > total {
		endRow = total
	}
	for row := m.scrollOffset; row < endRow; row++ {
		startIdx := row * m.cols
		endIdx := startIdx + m.cols
		if endIdx > len(m.cards) {
			endIdx = len(m.cards)
		}

		var renderedCards []string
		for i := startIdx; i < endIdx; i++ {
			renderedCards = append(renderedCards, m.renderCard(i, cw))
		}

		rowStr := lipgloss.JoinHorizontal(lipgloss.Top, renderedCards...)
		rows = append(rows, rowStr)
	}

	// Scroll indicators below.
	if endRow < total {
		below := total - endRow
		rows = append(rows, helpStyle.Render(fmt.Sprintf("  (%d more below)", below)))
	}

	return strings.Join(rows, "\n")
}

// renderCard renders a single card at the given index with the specified width.
func (m cardGridModel) renderCard(idx, width int) string {
	card := m.cards[idx]

	// Style: selected gets accent border, others get muted.
	style := cardNormalStyle.Width(width - 2) // -2 for border
	if idx == m.cursor {
		style = cardSelectedStyle.Width(width - 2)
	}

	// Inner content width (card width - borders - padding).
	// cardNormalStyle has Padding(0,1), border is 1 each side.
	innerWidth := width - 4 // 2 border + 2 padding

	var lines []string

	// Title line.
	title := truncateStr(card.title, innerWidth)
	lines = append(lines, labelStyle.Render(title))

	// Separator.
	sepLen := innerWidth
	if sepLen < 1 {
		sepLen = 1
	}
	sep := lipgloss.NewStyle().Foreground(mutedColor).Render(strings.Repeat("─", sepLen))
	lines = append(lines, sep)

	// Subtitle if present.
	if card.subtitle != "" {
		sub := truncateStr(card.subtitle, innerWidth)
		lines = append(lines, helpStyle.Render(sub))
	}

	// Content lines.
	for _, line := range card.lines {
		l := truncateStr(line, innerWidth)
		lines = append(lines, helpStyle.Render(l))
	}

	// For multi-column grids, pad to uniform height.
	if m.cols > 1 {
		targetLines := m.rowHeight() - 2 // subtract border top+bottom
		for len(lines) < targetLines {
			lines = append(lines, "")
		}
	}

	content := strings.Join(lines, "\n")
	return style.Render(content)
}
