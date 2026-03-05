package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

const (
	cursorWidth = 4 // "  " + "  " or "  " + "> "
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

// hookMatrixCell holds the symbol and styled version for one cell of the compat matrix.
type hookMatrixCell struct {
	plain  string // single char: ✓ ~ ! ✗
	styled string // with lipgloss color applied
}

// hookCompatMatrix is the precomputed 4-column matrix for one hook item.
type hookCompatMatrix [4]hookMatrixCell

// matrixProviders is the fixed order for the compatibility matrix columns.
var matrixProviders = converter.HookProviders() // ["claude-code", "gemini-cli", "copilot-cli", "kiro"]

// matrixHeadersFull are the full column headers (panel width >= 101).
var matrixHeadersFull = []string{"Claude", "Gemini", "Copilot", "Kiro"}

// matrixHeadersAbbr are the abbreviated column headers (panel width < 101).
var matrixHeadersAbbr = []string{"CC", "GC", "Cp", "Ki"}

// compatCellStyle returns the lipgloss style for a compat level.
func compatCellStyle(level converter.CompatLevel) lipgloss.Style {
	switch level {
	case converter.CompatFull:
		return compatFullStyle
	case converter.CompatDegraded:
		return compatDegradedStyle
	case converter.CompatBroken:
		return compatBrokenStyle
	case converter.CompatNone:
		return compatNoneStyle
	}
	return lipgloss.NewStyle()
}

// buildHookMatrix computes the compatibility matrix for a hook item.
func buildHookMatrix(item catalog.ContentItem) hookCompatMatrix {
	var m hookCompatMatrix
	hd, err := converter.LoadHookData(item)
	if err != nil {
		for i := range m {
			m[i] = hookMatrixCell{plain: "?", styled: "?"}
		}
		return m
	}
	for i, slug := range matrixProviders {
		result := converter.AnalyzeHookCompat(hd, slug)
		sym := result.Level.Symbol()
		m[i] = hookMatrixCell{
			plain:  sym,
			styled: compatCellStyle(result.Level).Render(sym),
		}
	}
	return m
}

type itemsModel struct {
	contentType    catalog.ContentType
	items          []catalog.ContentItem
	providers      []provider.Provider
	repoRoot       string
	sourceRegistry string // set when browsing items from a specific registry
	cursor         int
	hiddenCount    int // number of hidden items filtered out
	width          int
	height         int
}

func newItemsModel(ct catalog.ContentType, items []catalog.ContentItem, providers []provider.Provider, repoRoot string) itemsModel {
	// Sort My Tools by type (in display order) so grouped rendering shows contiguous sections
	if ct == catalog.Library && len(items) > 1 {
		typeOrder := make(map[catalog.ContentType]int)
		for idx, t := range catalog.AllContentTypes() {
			typeOrder[t] = idx
		}
		sort.SliceStable(items, func(i, j int) bool {
			return typeOrder[items[i].Type] < typeOrder[items[j].Type]
		})
	}
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
	// Breadcrumb: Home > Category
	home := zone.Mark("crumb-home", helpStyle.Render("Home"))
	arrow := helpStyle.Render(" > ")

	var s string
	switch {
	case m.sourceRegistry != "":
		reg := zone.Mark("crumb-registries", helpStyle.Render("Registries"))
		s = home + arrow + reg + arrow + titleStyle.Render(m.sourceRegistry) + "\n\n"
	case m.contentType == catalog.SearchResults:
		s = home + arrow + titleStyle.Render(fmt.Sprintf("Search Results (%d)", len(m.items))) + "\n\n"
	case m.contentType == catalog.Library:
		s = home + arrow + titleStyle.Render(fmt.Sprintf("Library (%d)", len(m.items))) + "\n\n"
	default:
		s = home + arrow + titleStyle.Render(m.contentType.Label()) + "\n\n"
	}

	if len(m.items) == 0 {
		s += helpStyle.Render("  No items found") + "\n"
		if m.contentType == catalog.Library {
			s += "\n" + helpStyle.Render("  Use Add to add content, or run 'syllago add' from the command line.") + "\n"
		}
		s += "\n" + helpStyle.Render("esc back")
		return s
	}

	relevant := m.relevantProviders()
	isProvSpecific := !m.contentType.IsUniversal()
	showProvCol := isProvSpecific || len(relevant) > 0
	if m.contentType == catalog.Library {
		showProvCol = false // grouped headers replace provider column
	}
	isHooks := m.contentType == catalog.Hooks
	if isHooks {
		showProvCol = false // matrix columns replace provider column
	}
	nameW := m.maxNameLen()
	tw := m.termWidth()

	// For hooks: precompute compat matrices and determine column headers/widths.
	var hookMatrices map[int]hookCompatMatrix
	var matrixColWidths [4]int
	var activeMatrixHeaders []string
	if isHooks {
		hookMatrices = make(map[int]hookCompatMatrix, len(m.items))
		for i, item := range m.items {
			hookMatrices[i] = buildHookMatrix(item)
		}
		// Choose full vs abbreviated headers based on panel width.
		if tw >= 101 {
			activeMatrixHeaders = matrixHeadersFull
		} else {
			activeMatrixHeaders = matrixHeadersAbbr
		}
		for i, hdr := range activeMatrixHeaders {
			matrixColWidths[i] = len(hdr)
		}
	}

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
	if isHooks {
		// Subtract the 4 matrix columns (each colW + colGap)
		for _, colW := range matrixColWidths {
			descW -= colGap + colW
		}
	}
	if descW < 10 {
		descW = 10
	}

	// Table header (skip for Library — group headers replace it)
	if m.contentType != catalog.Library {
		hdr := strings.Repeat(" ", cursorWidth)
		if isHooks {
			hdr += fmt.Sprintf("%-*s  %-*s", nameW, "Name", descW, "Description")
			for i, colHdr := range activeMatrixHeaders {
				hdr += fmt.Sprintf("  %-*s", matrixColWidths[i], colHdr)
			}
		} else if showProvCol {
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
		if isHooks {
			for _, colW := range matrixColWidths {
				sep += "  " + strings.Repeat("─", colW)
			}
		}
		s += helpStyle.Render(sep) + "\n"
	}

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
	showTypeTag := m.contentType == catalog.SearchResults

	// Rows (only render visible items)
	var prevGroupType catalog.ContentType
	for i := offset; i < end; i++ {
		item := m.items[i]

		// Group headers for Library — insert section label when type changes
		if m.contentType == catalog.Library && item.Type != prevGroupType {
			if prevGroupType != "" {
				s += "\n"
			}
			s += labelStyle.Render("  "+item.Type.Label()) + "\n"
			prevGroupType = item.Type
		}
		prefix := "  "
		style := itemStyle
		if i == m.cursor {
			prefix = "> "
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

		// Build description prefix: [EXAMPLE] for examples, [BUILT-IN] for meta-tools, [LOCAL] for local items, [registry-name] for registry items, [G] for global items
		localPrefix := ""
		localPrefixLen := 0
		if item.IsExample() {
			localPrefix = exampleStyle.Render("[EXAMPLE]") + " "
			localPrefixLen = 10 // "[EXAMPLE] "
		} else if item.IsBuiltin() {
			localPrefix = builtinStyle.Render("[BUILT-IN]") + " "
			localPrefixLen = 11 // "[BUILT-IN] "
		} else if item.Library {
			localPrefix = warningStyle.Render("[LIBRARY]") + " "
			localPrefixLen = 10 // "[LIBRARY] "
		} else if item.Registry != "" {
			tag := "[" + item.Registry + "]"
			localPrefix = countStyle.Render(tag) + " "
			localPrefixLen = len(tag) + 1 // tag + space
		} else if item.Source == "global" {
			localPrefix = globalStyle.Render("[GLOBAL]") + " "
			localPrefixLen = 9 // "[GLOBAL] "
		}

		if isHooks {
			paddedDesc := fmt.Sprintf("%-*s", descW, truncate(item.Description, descW-localPrefixLen))
			rowStr := fmt.Sprintf("  %s%s%s  %s%s",
				prefix,
				styledName,
				typeTag,
				localPrefix,
				helpStyle.Render(paddedDesc),
			)
			mat := hookMatrices[i]
			for j, cell := range mat {
				rowStr += fmt.Sprintf("  %-*s", matrixColWidths[j], cell.styled)
			}
			s += zone.Mark(fmt.Sprintf("item-%d", i), rowStr) + "\n"
		} else if showProvCol {
			paddedDesc := fmt.Sprintf("%-*s", descW, truncate(item.Description, descW-localPrefixLen))
			rowStr := fmt.Sprintf("  %s%s%s  %s%s  %s",
				prefix,
				styledName,
				typeTag,
				localPrefix,
				helpStyle.Render(paddedDesc),
				provCells[i].styled,
			)
			s += zone.Mark(fmt.Sprintf("item-%d", i), rowStr) + "\n"
		} else {
			desc := truncate(item.Description, descW-localPrefixLen)
			paddedDesc := fmt.Sprintf("%-*s", descW-localPrefixLen, desc)
			rowStr := fmt.Sprintf("  %s%s%s  %s%s",
				prefix,
				styledName,
				typeTag,
				localPrefix,
				helpStyle.Render(paddedDesc),
			)
			s += zone.Mark(fmt.Sprintf("item-%d", i), rowStr) + "\n"
		}
	}

	// Scroll indicator for items below
	if end < len(m.items) {
		s += helpStyle.Render(fmt.Sprintf("  (%d more below)", len(m.items)-end)) + "\n"
	}

	footer := "up/down navigate • enter detail • esc back • / search"
	if m.hiddenCount > 0 {
		footer += fmt.Sprintf(" • H show %d hidden", m.hiddenCount)
	}
	s += "\n" + helpStyle.Render(footer)
	return s
}

func (m itemsModel) selectedItem() catalog.ContentItem {
	if len(m.items) == 0 {
		return catalog.ContentItem{}
	}
	return m.items[m.cursor]
}
