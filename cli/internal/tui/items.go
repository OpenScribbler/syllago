package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
	"github.com/holdenhewett/romanesco/cli/internal/installer"
	"github.com/holdenhewett/romanesco/cli/internal/provider"
)

const (
	cursorWidth = 3 // "   " or " > "
	colGap      = 2 // spaces between columns
)

// displayName maps directory-style provider names to full display names.
var displayNames = map[string]string{
	"claude-code": "Claude Code",
	"gemini-cli":  "Gemini CLI",
	"cursor":      "Cursor",
	"windsurf":    "Windsurf",
	"codex":       "Codex",
}

func providerDisplayName(name string) string {
	if d, ok := displayNames[name]; ok {
		return d
	}
	return name
}

// displayName returns the display name for a content item.
// If the item has a DisplayName field set (e.g., from SKILL.md frontmatter),
// use that; otherwise fall back to the directory-based Name.
func displayName(item catalog.ContentItem) string {
	if item.DisplayName != "" {
		return item.DisplayName
	}
	return item.Name
}

// provCell holds both the plain-text (for width measurement) and styled version.
type provCell struct {
	plain  string
	styled string
}

type itemsModel struct {
	contentType catalog.ContentType
	items       []catalog.ContentItem
	providers   []provider.Provider
	repoRoot    string
	cursor      int
	width       int
	height      int
}

func newItemsModel(ct catalog.ContentType, items []catalog.ContentItem, providers []provider.Provider, repoRoot string) itemsModel {
	return itemsModel{
		contentType: ct,
		items:       items,
		providers:   providers,
		repoRoot:    repoRoot,
	}
}

func (m itemsModel) Update(msg tea.Msg) (itemsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case key.Matches(msg, keys.Home):
			m.cursor = 0
		case key.Matches(msg, keys.End):
			if len(m.items) > 0 {
				m.cursor = len(m.items) - 1
			}
		}
	}
	return m, nil
}

// relevantProviders returns detected providers that support the given content type.
func (m itemsModel) relevantProviders() []provider.Provider {
	var relevant []provider.Provider
	for _, p := range m.providers {
		if p.Detected && p.SupportsType(m.contentType) {
			relevant = append(relevant, p)
		}
	}
	return relevant
}

// maxNameLen returns the length of the longest item name (minimum 4 for "Name" header).
func (m itemsModel) maxNameLen() int {
	max := 4
	for _, item := range m.items {
		name := displayName(item)
		if len(name) > max {
			max = len(name)
		}
	}
	return max
}

// termWidth returns the usable terminal width, defaulting to 80.
func (m itemsModel) termWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 80
}

// truncate cuts a string to max length with "..." suffix.
func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// buildProvCell creates the provider column content for a single item.
func (m itemsModel) buildProvCell(item catalog.ContentItem, relevant []provider.Provider) provCell {
	if !m.contentType.IsUniversal() {
		// Provider-specific: show full provider name
		name := providerDisplayName(item.Provider)
		return provCell{plain: name, styled: name}
	}
	// Apps: show supported providers from frontmatter
	if item.Type == catalog.Apps && len(item.SupportedProviders) > 0 {
		var plainParts, styledParts []string
		for _, slug := range item.SupportedProviders {
			name := providerDisplayName(slug)
			plainParts = append(plainParts, name)
			styledParts = append(styledParts, helpStyle.Render(name))
		}
		return provCell{
			plain:  strings.Join(plainParts, ", "),
			styled: strings.Join(styledParts, helpStyle.Render(", ")),
		}
	}
	// Universal: only show providers where the item IS installed
	var plainParts, styledParts []string
	for _, p := range relevant {
		status := installer.CheckStatus(item, p, m.repoRoot)
		if status == installer.StatusInstalled {
			plainParts = append(plainParts, "[ok] "+p.Name)
			styledParts = append(styledParts, installedStyle.Render("[ok]")+" "+p.Name)
		}
	}
	return provCell{
		plain:  strings.Join(plainParts, "  "),
		styled: strings.Join(styledParts, "  "),
	}
}

func (m itemsModel) View() string {
	var s string
	switch m.contentType {
	case catalog.SearchResults:
		s = helpStyle.Render("nesco >") + " " + titleStyle.Render(fmt.Sprintf("Search Results (%d)", len(m.items))) + "\n"
	case catalog.MyTools:
		s = helpStyle.Render("nesco >") + " " + titleStyle.Render(fmt.Sprintf("My Tools (%d)", len(m.items))) + "\n"
	default:
		s = helpStyle.Render("nesco >") + " " + titleStyle.Render(m.contentType.Label()) + "\n"
	}

	if len(m.items) == 0 {
		s += helpStyle.Render("  No items found") + "\n"
		if m.contentType == catalog.MyTools {
			s += "\n" + helpStyle.Render("  Use Import to add content, or run 'nesco add' from the command line.") + "\n"
		}
		s += "\n" + helpStyle.Render("esc back")
		return s
	}

	relevant := m.relevantProviders()
	isProvSpecific := !m.contentType.IsUniversal()
	showProvCol := isProvSpecific || len(relevant) > 0
	nameW := m.maxNameLen()
	tw := m.termWidth()

	// Precompute provider column for each item and measure max width
	provCells := make([]provCell, len(m.items))
	maxProvW := 0
	if showProvCol {
		for i, item := range m.items {
			provCells[i] = m.buildProvCell(item, relevant)
			if len(provCells[i].plain) > maxProvW {
				maxProvW = len(provCells[i].plain)
			}
		}
		if maxProvW < 8 { // minimum width for "Provider" header
			maxProvW = 8
		}
	}

	// Description fills remaining terminal width
	descW := tw - cursorWidth - nameW - colGap
	if showProvCol {
		descW -= colGap + maxProvW
	}
	if descW < 10 {
		descW = 10
	}

	// Table header
	hdr := strings.Repeat(" ", cursorWidth)
	if showProvCol {
		hdr += fmt.Sprintf("%-*s  %-*s  %s", nameW, "Name", descW, "Description", "Provider")
	} else {
		hdr += fmt.Sprintf("%-*s  %s", nameW, "Name", "Description")
	}
	s += tableHeaderStyle.Render(hdr) + "\n"

	// Separator
	sep := strings.Repeat(" ", cursorWidth)
	sep += strings.Repeat("─", nameW) + "  " + strings.Repeat("─", descW)
	if showProvCol {
		sep += "  " + strings.Repeat("─", maxProvW)
	}
	s += helpStyle.Render(sep) + "\n"

	// Calculate viewport: show only items that fit in terminal height
	// Header takes ~3 lines (title + header row + separator), footer takes ~2 lines (blank + help)
	visibleRows := m.height - 5
	if visibleRows < 1 {
		visibleRows = len(m.items) // fallback: show all if height unknown
	}

	// Calculate scroll offset based on cursor position
	offset := 0
	if m.cursor >= visibleRows {
		offset = m.cursor - visibleRows + 1
	}
	end := offset + visibleRows
	if end > len(m.items) {
		end = len(m.items)
	}

	// Scroll indicators
	if offset > 0 {
		s += helpStyle.Render(fmt.Sprintf("  (%d more above)", offset)) + "\n"
	}

	// Whether to show a type tag per item (for mixed-type views)
	showTypeTag := m.contentType == catalog.SearchResults || m.contentType == catalog.MyTools

	// Rows (only render visible items)
	for i := offset; i < end; i++ {
		item := m.items[i]
		prefix := "   "
		style := itemStyle
		if i == m.cursor {
			prefix = " > "
			style = selectedItemStyle
		}

		name := displayName(item)
		paddedName := fmt.Sprintf("%-*s", nameW, truncate(name, nameW))
		styledName := style.Render(paddedName)

		// Add type tag for mixed-type views
		typeTag := ""
		if showTypeTag {
			typeTag = " " + countStyle.Render("("+item.Type.Label()+")")
		}

		// Build description with LOCAL prefix for local items
		localPrefix := ""
		localPrefixLen := 0
		if item.Local {
			localPrefix = warningStyle.Render("[LOCAL]") + " "
			localPrefixLen = 8 // "[LOCAL] "
		}

		if showProvCol {
			paddedDesc := fmt.Sprintf("%-*s", descW, truncate(item.Description, descW-localPrefixLen))
			rowStr := fmt.Sprintf("%s%s%s  %s%s  %s\n",
				prefix,
				styledName,
				typeTag,
				localPrefix,
				helpStyle.Render(paddedDesc),
				provCells[i].styled,
			)
			s += zone.Mark(fmt.Sprintf("item-%d", i), rowStr)
		} else {
			desc := truncate(item.Description, descW-localPrefixLen)
			paddedDesc := fmt.Sprintf("%-*s", descW-localPrefixLen, desc)
			rowStr := fmt.Sprintf("%s%s%s  %s%s\n",
				prefix,
				styledName,
				typeTag,
				localPrefix,
				helpStyle.Render(paddedDesc),
			)
			s += zone.Mark(fmt.Sprintf("item-%d", i), rowStr)
		}
	}

	// Scroll indicator for items below
	if end < len(m.items) {
		s += helpStyle.Render(fmt.Sprintf("  (%d more below)", len(m.items)-end)) + "\n"
	}

	s += "\n" + helpStyle.Render("up/down navigate • enter detail • esc back • / search")
	return s
}

func (m itemsModel) selectedItem() catalog.ContentItem {
	if len(m.items) == 0 {
		return catalog.ContentItem{}
	}
	return m.items[m.cursor]
}
