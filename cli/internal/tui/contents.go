package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

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
	// trust is the MOAT aggregate for the currently-selected registry
	// card. Non-nil triggers the Trust section block; nil hides it. The
	// section is the sole mouse click target for opening the Trust
	// Inspector from the registries tab — the card glyph is intentionally
	// not clickable per the bead's separation of "indicator" from
	// "interaction."
	trust   *catalog.RegistryTrust
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

// SetCard updates the sidebar to show info and items from the given card.
func (m *contentsSidebarModel) SetCard(card *cardData) {
	m.offset = 0
	m.groups = nil
	m.cardName = ""
	m.cardDesc = ""
	m.trust = nil
	if card == nil {
		return
	}

	m.cardName = card.name
	m.cardDesc = card.desc
	m.trust = card.trust

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
	m.trust = nil
	if card != nil {
		m.cardName = card.name
		m.cardDesc = card.desc
		m.trust = card.trust
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
	n += m.trustLines()
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

// trustLines returns the number of lines the Trust section will render.
// Keeps View() and scroll math in sync — both call this helper so adding
// or removing a field in renderTrustSection updates height accounting
// automatically.
func (m contentsSidebarModel) trustLines() int {
	if m.trust == nil {
		return 0
	}
	// Header + 4 data lines + items breakdown + [t] hint + trailing blank.
	// The exact field list is defined in renderTrustSection — keep in sync.
	return 7
}

// descLines returns how many lines the description wraps to.
func (m contentsSidebarModel) descLines() int {
	if m.cardDesc == "" {
		return 0
	}
	maxW := max(10, m.width-2)
	return len(wordWrap(sanitizeLine(m.cardDesc), maxW))
}

// renderTrustSection renders the registry-scoped Trust block. Must produce
// exactly trustLines() rows so scroll math stays correct. Keep the block
// as a single zone-marked region so clicks anywhere inside it (label,
// value, items breakdown) open the Trust Inspector — users don't have to
// hunt for a specific hitbox.
//
// Layout (7 lines):
//
//	Trust                           <- section title, bold
//	Tier: <tier label>              <- status-colored
//	Issuer: <subject or operator>   <- truncated at width
//	Status: <staleness label>       <- danger-colored when Stale/Expired
//	Items: N total · V verified     <- summary counts
//	[t] Inspect trust               <- discoverable affordance
//	<blank spacer>
func (m contentsSidebarModel) renderTrustSection() []string {
	rt := m.trust
	title := boldStyle.Render("Trust")

	tierLabel := rt.Tier.String()
	if tierLabel == "" {
		tierLabel = "Unknown"
	}
	tierStyle := mutedStyle
	switch rt.Tier {
	case catalog.TrustTierSigned, catalog.TrustTierDualAttested:
		tierStyle = lipgloss.NewStyle().Foreground(successColor).Bold(true)
	case catalog.TrustTierUnsigned:
		tierStyle = lipgloss.NewStyle().Foreground(warningColor)
	}
	tierLine := boldStyle.Render("Tier: ") + tierStyle.Render(tierLabel)

	issuerText := rt.Operator
	if issuerText == "" {
		issuerText = rt.Subject
	}
	if issuerText == "" {
		issuerText = "—"
	}
	issuerLine := boldStyle.Render("Issuer: ") + mutedStyle.Render(truncate(sanitizeLine(issuerText), max(0, m.width-9)))

	staleLabel := rt.Staleness
	if staleLabel == "" {
		staleLabel = "Unknown"
	}
	staleStyle := mutedStyle
	if staleLabel != "Fresh" {
		staleStyle = lipgloss.NewStyle().Foreground(dangerColor).Bold(true)
	}
	statusLine := boldStyle.Render("Status: ") + staleStyle.Render(staleLabel)

	itemsSummary := fmt.Sprintf("%d total · %d verified · %d recalled",
		rt.TotalItems, rt.VerifiedItems, rt.RecalledItems)
	itemsLine := boldStyle.Render("Items: ") + mutedStyle.Render(itemsSummary)

	hint := mutedStyle.Render("[t] Inspect trust")

	// Zone-marking every line individually lets click-detection work even
	// when scroll clips the top/bottom of the section. All share the same
	// "registry-trust" id so any hit opens the inspector.
	mark := func(s string) string { return zone.Mark("registry-trust", s) }

	return []string{
		mark(title),
		mark(tierLine),
		mark(issuerLine),
		mark(statusLine),
		mark(itemsLine),
		mark(hint),
		"", // trailing spacer separates Trust from Contents groupings
	}
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

	// Description (may wrap at word boundaries)
	if m.cardDesc != "" {
		desc := sanitizeLine(m.cardDesc)
		maxW := max(10, m.width-2)
		for _, line := range wordWrap(desc, maxW) {
			allLines = append(allLines, mutedStyle.Render(line))
		}
	}

	// Blank line + Contents header
	if m.cardName != "" || m.cardDesc != "" {
		allLines = append(allLines, "")
		allLines = append(allLines, boldStyle.Render("Contents"))
	}

	// Trust section — only for MOAT-type registries. Rendered between
	// Contents and the items list so the trust claim is visible on first
	// render, not hidden behind scrolling. The whole block is zone-marked
	// as a single click target ("registry-trust") that opens the Trust
	// Inspector.
	if m.trust != nil {
		allLines = append(allLines, m.renderTrustSection()...)
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
