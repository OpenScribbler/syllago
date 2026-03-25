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

// contentsSidebarModel shows card info and grouped items inside the selected card.
type contentsSidebarModel struct {
	cardName string
	cardDesc string
	groups   []contentGroup
	offset   int
	width    int
	height   int
	focused  bool
}

func newContentsSidebarModel() contentsSidebarModel {
	return contentsSidebarModel{}
}

// SetSize updates dimensions.
func (m *contentsSidebarModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetCard updates the sidebar to show info and items from the given card.
func (m *contentsSidebarModel) SetCard(card *cardData) {
	m.offset = 0
	m.groups = nil
	m.cardName = ""
	m.cardDesc = ""
	if card == nil {
		return
	}

	m.cardName = card.name
	m.cardDesc = card.desc

	if len(card.items) == 0 {
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

// SetGroups sets content groups and card info for loadouts without resolved items.
func (m *contentsSidebarModel) SetGroups(card *cardData, groups []contentGroup) {
	m.offset = 0
	m.groups = groups
	m.cardName = ""
	m.cardDesc = ""
	if card != nil {
		m.cardName = card.name
		m.cardDesc = card.desc
	}
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
	n := m.headerLines()
	for _, g := range m.groups {
		n++ // type header
		n += len(g.items)
	}
	return n
}

// headerLines returns lines used by Name, Description, and Contents header.
func (m contentsSidebarModel) headerLines() int {
	n := 0
	if m.cardName != "" {
		n++ // Name: ...
	}
	if m.cardDesc != "" {
		// Wrap description across multiple lines
		n += m.descLines()
	}
	if m.cardName != "" || m.cardDesc != "" {
		n++ // blank line
		n++ // "Contents" header
	}
	return n
}

// descLines returns how many lines the description wraps to.
func (m contentsSidebarModel) descLines() int {
	if m.cardDesc == "" {
		return 0
	}
	maxW := max(10, m.width-2)
	desc := sanitizeLine(m.cardDesc)
	if len(desc) <= maxW {
		return 1
	}
	return (len(desc) + maxW - 1) / maxW
}

// View renders the contents sidebar.
func (m contentsSidebarModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	if m.cardName == "" && len(m.groups) == 0 {
		return lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(mutedColor).
			Render("Select a card")
	}

	var allLines []string

	// Name
	if m.cardName != "" {
		allLines = append(allLines, boldStyle.Render("Name: ")+mutedStyle.Render(truncate(sanitizeLine(m.cardName), max(0, m.width-6))))
	}

	// Description (may wrap)
	if m.cardDesc != "" {
		desc := sanitizeLine(m.cardDesc)
		maxW := max(10, m.width-2)
		firstLine := truncate(desc, maxW)
		allLines = append(allLines, mutedStyle.Render(firstLine))
		// Wrap remaining
		rest := desc
		if len(rest) > maxW {
			rest = rest[maxW:]
			for len(rest) > 0 {
				chunk := rest
				if len(chunk) > maxW {
					chunk = chunk[:maxW]
				}
				allLines = append(allLines, mutedStyle.Render(chunk))
				rest = rest[len(chunk):]
			}
		}
	}

	// Blank line + Contents header
	if m.cardName != "" || m.cardDesc != "" {
		allLines = append(allLines, "")
		allLines = append(allLines, boldStyle.Render("Contents"))
	}

	// Grouped items
	for _, g := range m.groups {
		allLines = append(allLines, sectionTitleStyle.Render(g.typeName))
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

	for len(visible) < m.height {
		visible = append(visible, strings.Repeat(" ", m.width))
	}

	for i, line := range visible {
		visible[i] = lipgloss.NewStyle().MaxWidth(m.width).Render(line)
		if g := m.width - lipgloss.Width(visible[i]); g > 0 {
			visible[i] += strings.Repeat(" ", g)
		}
	}

	return strings.Join(visible, "\n")
}
