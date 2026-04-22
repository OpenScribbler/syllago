package tui

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/loadout"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

// borderColor for card borders (Flexoki base-200/850).
var borderColor = lipgloss.AdaptiveColor{Light: "#CECDC3", Dark: "#343331"}

// cardData holds the display data for a single gallery card.
type cardData struct {
	name     string
	subtitle string                // "Target: Claude Code" or "Source: /path"
	desc     string                // description from manifest or registry metadata
	counts   map[string]int        // type label -> count (e.g. "Skills": 4)
	status   string                // "local", "registry", etc.
	items    []catalog.ContentItem // items inside this card
	path     string                // directory path for metadata editing

	// Registry-scoped trust aggregate. Non-nil only for cards built from a
	// MOAT-type registry whose manifest was parsed during enrichment. The
	// gallery consumes this for the card trust glyph, sidebar Trust section,
	// and [t]/click routing to the Trust Inspector.
	trust *catalog.RegistryTrust
}

// allContentTypeLabels is the fixed list of content type labels shown on every card.
var allContentTypeLabels = []string{"Agents", "Commands", "Hooks", "MCP Servers", "Rules", "Skills"}

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

// fixedCardHeight is the constant height of every card (border included).
// name(1) + 6 content types + subtitle(1) + border(2) = 10
const fixedCardHeight = 10

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
	visibleRows := max(1, m.height/fixedCardHeight)
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

	visibleRows := max(1, m.height/fixedCardHeight)
	totalRows := (len(m.cards) + m.cols - 1) / m.cols

	startRow := m.offset
	endRow := min(startRow+visibleRows, totalRows)

	var rows []string
	for row := startRow; row < endRow; row++ {
		var rowCards []string
		for col := 0; col < m.cols; col++ {
			idx := row*m.cols + col
			if idx >= len(m.cards) {
				rowCards = append(rowCards, strings.Repeat(" ", cardWidth+borderSize))
				continue
			}
			rowCards = append(rowCards, m.renderCard(idx))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rowCards...))
	}

	result := lipgloss.JoinVertical(lipgloss.Left, rows...)

	resultLines := strings.Split(result, "\n")
	for len(resultLines) < m.height {
		resultLines = append(resultLines, strings.Repeat(" ", m.width))
	}
	if len(resultLines) > m.height {
		resultLines = resultLines[:m.height]
	}
	return strings.Join(resultLines, "\n")
}

// renderCard renders a single card with border. All cards have the same height.
func (m cardGridModel) renderCard(index int) string {
	c := m.cards[index]
	isSelected := index == m.cursor

	innerH := fixedCardHeight - borderSize
	var lines []string

	// Name line — prepend aggregate trust glyph when a MOAT registry has
	// something to report. The glyph is purely visual; clicks on the card
	// body open the contents sidebar (existing behavior), and [t] or the
	// sidebar Trust section (zone-marked separately) open the Trust
	// Inspector.
	glyph := registryTrustGlyph(c.trust)
	nameBudget := cardWidth
	if glyph != "" {
		nameBudget -= 2 // glyph + space
	}
	name := truncate(sanitizeLine(c.name), nameBudget)
	if glyph != "" {
		name = glyph + " " + name
	}
	if isSelected && m.focused {
		lines = append(lines, boldStyle.Render(name))
	} else {
		lines = append(lines, name)
	}

	// Always show all 6 content types in fixed order
	for _, label := range allContentTypeLabels {
		v := c.counts[label]
		lines = append(lines, mutedStyle.Render("  "+itoa(v)+" "+label))
	}

	// Subtitle always at bottom
	if c.subtitle != "" {
		lines = append(lines, mutedStyle.Render(truncate(c.subtitle, cardWidth)))
	}

	for len(lines) < innerH {
		lines = append(lines, "")
	}
	if len(lines) > innerH {
		lines = lines[:innerH]
	}

	content := strings.Join(lines, "\n")

	bc := borderColor
	if isSelected && m.focused {
		bc = accentColor
	}

	rendered := borderedPanel(content, cardWidth, innerH, bc)
	return zone.Mark("card-"+itoa(index), rendered)
}

// --- Card data construction ---

// buildLoadoutCards creates card data from loadout catalog items.
// The full catalog is needed to resolve item references from the manifest.
func buildLoadoutCards(items []catalog.ContentItem, cat *catalog.Catalog) []cardData {
	// Build a lookup: type/name -> ContentItem for resolving manifest refs
	itemLookup := make(map[string]*catalog.ContentItem, len(cat.Items))
	for i := range cat.Items {
		key := string(cat.Items[i].Type) + "/" + cat.Items[i].Name
		itemLookup[key] = &cat.Items[i]
	}

	cards := make([]cardData, 0, len(items))
	for _, item := range items {
		c := cardData{
			name:   itemDisplayName(item),
			status: "local",
			counts: ensureAllTypes(nil),
			path:   item.Path,
		}

		manifestPath := filepath.Join(item.Path, "loadout.yaml")
		if m, err := loadout.Parse(manifestPath); err == nil {
			c.subtitle = "Target: " + providerFullName(m.Provider)
			// Use item.Description which includes .syllago.yaml override from the scanner
			c.desc = sanitizeLine(item.Description)
			raw := make(map[string]int)
			var resolved []catalog.ContentItem
			for ct, refs := range m.RefsByType() {
				raw[ct.Label()] = len(refs)
				for _, ref := range refs {
					key := string(ct) + "/" + ref.Name
					if ci, ok := itemLookup[key]; ok {
						resolved = append(resolved, *ci)
					}
				}
			}
			c.counts = ensureAllTypes(raw)
			c.items = resolved
		}

		if item.Registry != "" {
			c.status = item.Registry
		}

		cards = append(cards, c)
	}
	return cards
}

// buildRegistryCards creates card data from registry sources and catalog.
// MOAT-type registries get a trust aggregate attached so the card can show
// a trust glyph and the sidebar can render a Trust section. Non-MOAT
// registries leave card.trust nil — no trust surfaces are drawn.
func buildRegistryCards(sources []catalog.RegistrySource, cat *catalog.Catalog) []cardData {
	cards := make([]cardData, 0, len(sources))
	for _, src := range sources {
		items := cat.ByRegistry(src.Name)

		raw := make(map[string]int)
		for _, item := range items {
			raw[item.Type.Label()]++
		}

		c := cardData{
			name:     src.Name,
			subtitle: "Source: " + src.Path,
			counts:   ensureAllTypes(raw),
			status:   itoa(len(items)) + " items",
			items:    items,
			path:     src.Path,
		}

		// Load description from registry.yaml manifest
		if manifest, _ := registry.LoadManifestFromDir(src.Path); manifest != nil {
			c.desc = sanitizeLine(manifest.Description)
			if manifest.Name != "" {
				c.name = manifest.Name
			}
		}

		// .syllago.yaml override (survives registry sync since it's untracked by git)
		if meta, _ := metadata.Load(src.Path); meta != nil {
			if meta.Name != "" {
				c.name = meta.Name
			}
			if meta.Description != "" {
				c.desc = sanitizeLine(meta.Description)
			}
		}

		if cat != nil {
			if rt, ok := cat.RegistryTrusts[src.Name]; ok {
				c.trust = rt
			}
		}

		cards = append(cards, c)
	}
	return cards
}

// registryTrustGlyph returns the aggregate health glyph for a registry card.
// Follows the AD-7 Panel C9 collapse rule for the library row badge, adapted
// to registry aggregates:
//
//   - "R" if any item is revoked (hardest signal takes precedence).
//   - "✓" if the registry is Fresh AND at least one item verified.
//   - "⏰" if the registry is Stale, Expired, or Missing (user needs to sync).
//   - ""  for non-MOAT registries or a Fresh MOAT registry with zero
//     verified items (nothing to report).
//
// Kept deliberately narrow — the card glyph is a "is anything wrong here?"
// signal, not a tier display. Full tier/issuer detail lives in the Trust
// Inspector.
func registryTrustGlyph(rt *catalog.RegistryTrust) string {
	if rt == nil {
		return ""
	}
	if rt.RevokedItems > 0 {
		return "R"
	}
	if rt.Staleness != "" && rt.Staleness != "Fresh" {
		return "\u23f0" // clock face — stale/expired/missing
	}
	if rt.VerifiedItems > 0 {
		return "\u2713"
	}
	return ""
}

// ensureAllTypes returns a counts map that has all 6 content type labels,
// filling in 0 for any missing types.
func ensureAllTypes(raw map[string]int) map[string]int {
	result := make(map[string]int, len(allContentTypeLabels))
	for _, label := range allContentTypeLabels {
		result[label] = 0
	}
	for k, v := range raw {
		result[k] = v
	}
	return result
}

// providerFullName maps a provider slug to its full display name.
func providerFullName(slug string) string {
	switch slug {
	case "claude-code":
		return "Claude Code"
	case "gemini-cli":
		return "Gemini CLI"
	case "cursor":
		return "Cursor"
	case "copilot":
		return "Copilot"
	case "windsurf":
		return "Windsurf"
	case "kiro":
		return "Kiro"
	case "cline":
		return "Cline"
	case "roo-code":
		return "Roo Code"
	case "amp":
		return "Amp"
	case "opencode":
		return "OpenCode"
	case "zed":
		return "Zed"
	default:
		return slug
	}
}

// sortedKeys returns map keys in sorted order.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Use the fixed order from allContentTypeLabels when possible
	ordered := make([]string, 0, len(keys))
	for _, label := range allContentTypeLabels {
		if _, ok := m[label]; ok {
			ordered = append(ordered, label)
		}
	}
	// Any extra keys not in the standard list
	for _, k := range keys {
		found := false
		for _, o := range ordered {
			if k == o {
				found = true
				break
			}
		}
		if !found {
			ordered = append(ordered, k)
		}
	}
	return ordered
}
