package tui

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/loadout"
)

// borderColor for card borders (Flexoki base-200/850).
var borderColor = lipgloss.AdaptiveColor{Light: "#CECDC3", Dark: "#343331"}

// cardData holds the display data for a single gallery card.
type cardData struct {
	name     string
	subtitle string                // target provider (loadouts) or URL (registries)
	desc     string                // description from manifest or registry metadata
	counts   map[string]int        // type label -> count (e.g. "Skills": 4)
	status   string                // "local", "registry", etc.
	items    []catalog.ContentItem // items inside this card
}

// cardGridModel renders a responsive grid of cards with cursor navigation.
type cardGridModel struct {
	cards   []cardData
	cursor  int // selected card index
	cols    int // cards per row (responsive)
	offset  int // scroll offset in rows
	width   int
	height  int
	focused bool
}

func newCardGridModel(cards []cardData) cardGridModel {
	return cardGridModel{
		cards:   cards,
		focused: true,
	}
}

// cardWidth is the fixed inner width of each card (excluding border).
const cardWidth = 26

// cardRenderHeight returns the height of a single rendered card including borders.
func cardRenderHeight(c cardData) int {
	// name + each count line + subtitle + border top/bottom
	lines := 1 // name
	lines += len(c.counts)
	if c.subtitle != "" {
		lines++
	}
	return lines + borderSize // +2 for border
}

// maxCardHeight returns the tallest card height in the set.
func maxCardHeight(cards []cardData) int {
	h := 0
	for _, c := range cards {
		if ch := cardRenderHeight(c); ch > h {
			h = ch
		}
	}
	if h == 0 {
		h = 5 // minimum
	}
	return h
}

// SetSize updates grid dimensions and recomputes column count.
func (m *cardGridModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.cols = m.computeCols()
}

// computeCols calculates columns based on available width.
func (m cardGridModel) computeCols() int {
	outerCardW := cardWidth + borderSize + 2 // card + border + gap
	if m.width >= 3*outerCardW {
		return 3
	}
	if m.width >= 2*outerCardW {
		return 2
	}
	return 1
}

// Selected returns the currently selected card, or nil if empty.
func (m cardGridModel) Selected() *cardData {
	if len(m.cards) == 0 || m.cursor < 0 || m.cursor >= len(m.cards) {
		return nil
	}
	return &m.cards[m.cursor]
}

// CursorUp moves the cursor up one row.
func (m *cardGridModel) CursorUp() {
	if len(m.cards) == 0 {
		return
	}
	m.cursor -= m.cols
	if m.cursor < 0 {
		// Wrap to last row, same column
		lastRow := (len(m.cards) - 1) / m.cols
		m.cursor += (lastRow + 1) * m.cols
		if m.cursor >= len(m.cards) {
			m.cursor = len(m.cards) - 1
		}
	}
	m.scrollToCursor()
}

// CursorDown moves the cursor down one row.
func (m *cardGridModel) CursorDown() {
	if len(m.cards) == 0 {
		return
	}
	m.cursor += m.cols
	if m.cursor >= len(m.cards) {
		// Wrap to first row, same column
		col := (m.cursor - m.cols) % m.cols
		m.cursor = col
		if m.cursor >= len(m.cards) {
			m.cursor = 0
		}
	}
	m.scrollToCursor()
}

// CursorLeft moves the cursor left one card.
func (m *cardGridModel) CursorLeft() {
	if len(m.cards) == 0 {
		return
	}
	m.cursor--
	if m.cursor < 0 {
		m.cursor = len(m.cards) - 1
	}
	m.scrollToCursor()
}

// CursorRight moves the cursor right one card.
func (m *cardGridModel) CursorRight() {
	if len(m.cards) == 0 {
		return
	}
	m.cursor++
	if m.cursor >= len(m.cards) {
		m.cursor = 0
	}
	m.scrollToCursor()
}

// scrollToCursor ensures the cursor row is visible.
func (m *cardGridModel) scrollToCursor() {
	if len(m.cards) == 0 || m.cols == 0 {
		return
	}
	ch := maxCardHeight(m.cards)
	if ch == 0 {
		ch = 5
	}
	visibleRows := max(1, m.height/ch)
	cursorRow := m.cursor / m.cols
	if cursorRow < m.offset {
		m.offset = cursorRow
	}
	if cursorRow >= m.offset+visibleRows {
		m.offset = cursorRow - visibleRows + 1
	}
}

// View renders the card grid.
func (m cardGridModel) View() string {
	if len(m.cards) == 0 {
		return lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(mutedColor).
			Render("No items found")
	}

	ch := maxCardHeight(m.cards)
	visibleRows := max(1, m.height/ch)
	totalRows := (len(m.cards) + m.cols - 1) / m.cols

	startRow := m.offset
	endRow := min(startRow+visibleRows, totalRows)

	var rows []string
	for row := startRow; row < endRow; row++ {
		var rowCards []string
		for col := 0; col < m.cols; col++ {
			idx := row*m.cols + col
			if idx >= len(m.cards) {
				// Empty cell — pad to card width
				rowCards = append(rowCards, strings.Repeat(" ", cardWidth+borderSize))
				continue
			}
			rowCards = append(rowCards, m.renderCard(idx, ch))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rowCards...))
	}

	result := lipgloss.JoinVertical(lipgloss.Left, rows...)

	// Pad height
	resultLines := strings.Split(result, "\n")
	for len(resultLines) < m.height {
		resultLines = append(resultLines, strings.Repeat(" ", m.width))
	}
	if len(resultLines) > m.height {
		resultLines = resultLines[:m.height]
	}
	return strings.Join(resultLines, "\n")
}

// renderCard renders a single card with border.
func (m cardGridModel) renderCard(index, targetHeight int) string {
	c := m.cards[index]
	isSelected := index == m.cursor

	innerH := targetHeight - borderSize // subtract border
	var lines []string

	// Name line
	name := truncate(sanitizeLine(c.name), cardWidth)
	if isSelected && m.focused {
		lines = append(lines, boldStyle.Render(name))
	} else {
		lines = append(lines, name)
	}

	// Count lines (sorted by type name for consistency)
	countKeys := sortedKeys(c.counts)
	for _, k := range countKeys {
		v := c.counts[k]
		lines = append(lines, mutedStyle.Render("  "+itoa(v)+" "+k))
	}

	// Subtitle line
	if c.subtitle != "" {
		lines = append(lines, mutedStyle.Render(truncate(c.subtitle, cardWidth)))
	}

	// Pad to target height
	for len(lines) < innerH {
		lines = append(lines, "")
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}

	content := strings.Join(lines, "\n")

	// Border color
	bc := borderColor
	if isSelected && m.focused {
		bc = accentColor
	}

	rendered := borderedPanel(content, cardWidth, innerH, bc)
	return zone.Mark("card-"+itoa(index), rendered)
}

// --- Card data construction ---

// buildLoadoutCards creates card data from loadout catalog items.
func buildLoadoutCards(items []catalog.ContentItem) []cardData {
	cards := make([]cardData, 0, len(items))
	for _, item := range items {
		c := cardData{
			name:   itemDisplayName(item),
			status: "local",
			items:  nil, // will be populated if we have the full catalog
		}

		// Try to parse the manifest for richer data
		manifestPath := filepath.Join(item.Path, "loadout.yaml")
		if m, err := loadout.Parse(manifestPath); err == nil {
			c.subtitle = "Target: " + providerAbbrev(m.Provider)
			c.desc = sanitizeLine(m.Description)
			c.counts = make(map[string]int)
			for ct, refs := range m.RefsByType() {
				c.counts[ct.Label()] = len(refs)
			}
		} else {
			// Fallback: count files
			c.subtitle = "Loadout"
			c.counts = map[string]int{"Files": len(item.Files)}
		}

		if item.Registry != "" {
			c.status = item.Registry
		}

		cards = append(cards, c)
	}
	return cards
}

// buildRegistryCards creates card data from registry sources and catalog.
func buildRegistryCards(sources []catalog.RegistrySource, cat *catalog.Catalog) []cardData {
	cards := make([]cardData, 0, len(sources))
	for _, src := range sources {
		items := cat.ByRegistry(src.Name)

		counts := make(map[string]int)
		for _, item := range items {
			counts[item.Type.Label()]++
		}

		c := cardData{
			name:     src.Name,
			subtitle: src.Path,
			counts:   counts,
			status:   itoa(len(items)) + " items",
			items:    items,
		}
		cards = append(cards, c)
	}
	return cards
}

// sortedKeys returns map keys in sorted order.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
