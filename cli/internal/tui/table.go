package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// sortColumn identifies which column to sort by.
type sortColumn int

const (
	sortByName sortColumn = iota
	sortByType
	sortByScope
	sortByFiles
	sortByInstalled
	sortByDesc
	sortColCount // sentinel for cycling
)

// tableModel renders a full-width scrollable table for the Library view.
type tableModel struct {
	// Source data (unfiltered)
	allItems []catalog.ContentItem
	allRows  []tableRow

	// Displayed data (after search filter + sort)
	items []catalog.ContentItem
	rows  []tableRow

	cursor  int
	offset  int
	width   int
	height  int
	focused bool

	// Search
	searching   bool
	searchQuery string

	// Sort
	sortCol sortColumn
	sortAsc bool

	// Provider data for Installed column
	providers []provider.Provider
	repoRoot  string
}

// tableRow holds pre-computed display strings for a single item.
type tableRow struct {
	name        string
	contentType string
	scope       string
	files       string
	installed   string
	description string
	sortFiles   int // numeric for sorting
}

func newTableModel(items []catalog.ContentItem, provs []provider.Provider, repoRoot string) tableModel {
	t := tableModel{
		providers: provs,
		repoRoot:  repoRoot,
		sortCol:   sortByName,
		sortAsc:   true,
	}
	t.setSourceItems(items)
	return t
}

// setSourceItems sets the underlying data and recomputes display rows.
func (t *tableModel) setSourceItems(items []catalog.ContentItem) {
	t.allItems = items
	t.allRows = t.computeRows(items)
	t.applyFilterAndSort()
}

// SetSize updates table dimensions.
func (t *tableModel) SetSize(width, height int) {
	t.width = width
	t.height = height
}

// SetItems replaces the table data and resets state.
func (t *tableModel) SetItems(items []catalog.ContentItem) {
	t.searchQuery = ""
	t.searching = false
	t.setSourceItems(items)
	t.cursor = 0
	t.offset = 0
}

// Selected returns the currently selected item, or nil if empty.
func (t tableModel) Selected() *catalog.ContentItem {
	if len(t.items) == 0 || t.cursor < 0 || t.cursor >= len(t.items) {
		return nil
	}
	return &t.items[t.cursor]
}

// Len returns displayed item count.
func (t tableModel) Len() int {
	return len(t.items)
}

// CursorUp moves cursor up with wrapping.
func (t *tableModel) CursorUp() {
	if len(t.items) == 0 {
		return
	}
	t.cursor--
	if t.cursor < 0 {
		t.cursor = len(t.items) - 1
		t.offset = max(0, len(t.items)-t.viewHeight())
	}
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
}

// CursorDown moves cursor down with wrapping.
func (t *tableModel) CursorDown() {
	if len(t.items) == 0 {
		return
	}
	t.cursor++
	if t.cursor >= len(t.items) {
		t.cursor = 0
		t.offset = 0
	}
	vh := t.viewHeight()
	if t.cursor >= t.offset+vh {
		t.offset = t.cursor - vh + 1
	}
}

// PageUp moves cursor up by one page.
func (t *tableModel) PageUp() {
	vh := t.viewHeight()
	t.cursor = max(0, t.cursor-vh)
	t.offset = max(0, t.offset-vh)
}

// PageDown moves cursor down by one page.
func (t *tableModel) PageDown() {
	vh := t.viewHeight()
	t.cursor = min(len(t.items)-1, t.cursor+vh)
	maxOffset := max(0, len(t.items)-vh)
	t.offset = min(maxOffset, t.offset+vh)
}

// CycleSort advances to the next sort column.
func (t *tableModel) CycleSort() {
	// Skip Description column if it won't be visible
	for {
		t.sortCol = (t.sortCol + 1) % sortColCount
		if t.sortCol == sortByDesc && t.width < 110+borderSize {
			continue // skip desc at narrow widths
		}
		break
	}
	t.applyFilterAndSort()
}

// ReverseSort toggles sort direction.
func (t *tableModel) ReverseSort() {
	t.sortAsc = !t.sortAsc
	t.applyFilterAndSort()
}

// StartSearch enters search mode.
func (t *tableModel) StartSearch() {
	t.searching = true
	t.searchQuery = ""
}

// CancelSearch exits search mode and restores all items.
func (t *tableModel) CancelSearch() {
	t.searching = false
	t.searchQuery = ""
	t.applyFilterAndSort()
}

// SearchType adds a character to the search query.
func (t *tableModel) SearchType(ch rune) {
	t.searchQuery += string(ch)
	t.applyFilterAndSort()
}

// SearchBackspace removes the last character from the search query.
func (t *tableModel) SearchBackspace() {
	if len(t.searchQuery) > 0 {
		runes := []rune(t.searchQuery)
		t.searchQuery = string(runes[:len(runes)-1])
		t.applyFilterAndSort()
	}
}

// SearchConfirm exits search mode but keeps the filter active.
func (t *tableModel) SearchConfirm() {
	t.searching = false
	// searchQuery stays applied
}

// applyFilterAndSort rebuilds displayed items from source data.
func (t *tableModel) applyFilterAndSort() {
	// Filter
	if t.searchQuery == "" {
		t.items = make([]catalog.ContentItem, len(t.allItems))
		copy(t.items, t.allItems)
		t.rows = make([]tableRow, len(t.allRows))
		copy(t.rows, t.allRows)
	} else {
		q := strings.ToLower(t.searchQuery)
		t.items = nil
		t.rows = nil
		for i, item := range t.allItems {
			if strings.Contains(strings.ToLower(item.Name), q) ||
				strings.Contains(strings.ToLower(item.Description), q) ||
				strings.Contains(strings.ToLower(string(item.Type)), q) {
				t.items = append(t.items, item)
				t.rows = append(t.rows, t.allRows[i])
			}
		}
	}

	// Sort
	t.sortItems()

	// Clamp cursor
	if t.cursor >= len(t.items) {
		t.cursor = max(0, len(t.items)-1)
	}
	t.offset = 0
}

// sortItems sorts the displayed items+rows by the current sort column.
func (t *tableModel) sortItems() {
	if len(t.items) <= 1 {
		return
	}

	indices := make([]int, len(t.items))
	for i := range indices {
		indices[i] = i
	}

	sort.SliceStable(indices, func(a, b int) bool {
		ra, rb := t.rows[indices[a]], t.rows[indices[b]]
		var less bool
		switch t.sortCol {
		case sortByName:
			less = strings.ToLower(ra.name) < strings.ToLower(rb.name)
		case sortByType:
			less = ra.contentType < rb.contentType
		case sortByScope:
			less = ra.scope < rb.scope
		case sortByFiles:
			less = ra.sortFiles < rb.sortFiles
		case sortByInstalled:
			less = ra.installed < rb.installed
		case sortByDesc:
			less = ra.description < rb.description
		default:
			less = strings.ToLower(ra.name) < strings.ToLower(rb.name)
		}
		if !t.sortAsc {
			less = !less
		}
		return less
	})

	// Reorder items and rows by sorted indices
	sortedItems := make([]catalog.ContentItem, len(t.items))
	sortedRows := make([]tableRow, len(t.rows))
	for i, idx := range indices {
		sortedItems[i] = t.items[idx]
		sortedRows[i] = t.rows[idx]
	}
	t.items = sortedItems
	t.rows = sortedRows
}

// viewHeight returns rows available for data (minus header, minus search bar if active).
func (t tableModel) viewHeight() int {
	h := t.height - 1 // header
	if t.searching || t.searchQuery != "" {
		h-- // search bar
	}
	return max(0, h)
}

// View renders the table.
func (t tableModel) View() string {
	if t.width <= 0 || t.height <= 0 {
		return ""
	}

	if len(t.allItems) == 0 {
		return t.renderEmpty()
	}

	cols := t.columnWidths()
	lines := make([]string, 0, t.height)

	// Search bar (when searching or query active)
	if t.searching || t.searchQuery != "" {
		lines = append(lines, t.renderSearchBar())
	}

	// Header row
	lines = append(lines, t.renderHeader(cols))

	if len(t.items) == 0 {
		// Search has no results
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("  No matches for %q", t.searchQuery)))
	} else {
		vh := t.viewHeight()
		lastVisible := min(t.offset+vh, len(t.items))

		itemsAbove := t.offset
		itemsBelow := max(0, len(t.items)-lastVisible)
		showAbove := itemsAbove > 0
		showBelow := itemsBelow > 0

		contentStart := t.offset
		contentEnd := lastVisible
		if showAbove {
			contentStart++
		}
		if showBelow && contentEnd > contentStart {
			contentEnd--
		}

		if showAbove {
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("(%d more above)", itemsAbove)))
		}

		for i := contentStart; i < contentEnd; i++ {
			lines = append(lines, t.renderRow(i, cols))
		}

		if showBelow {
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("(%d more below)", itemsBelow)))
		}
	}

	for len(lines) < t.height {
		lines = append(lines, strings.Repeat(" ", t.width))
	}

	return strings.Join(lines, "\n")
}

// renderSearchBar renders the search input line.
func (t tableModel) renderSearchBar() string {
	prompt := "/ "
	query := t.searchQuery
	cursor := ""
	if t.searching {
		cursor = "█"
	}

	matchInfo := ""
	if t.searchQuery != "" {
		matchInfo = fmt.Sprintf("  (%d/%d)", len(t.items), len(t.allItems))
	}

	left := prompt + query + cursor
	right := matchInfo
	if t.searching {
		right += "  esc cancel · enter confirm"
	} else {
		right += "  / edit · esc clear"
	}
	right = mutedStyle.Render(right)

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := max(1, t.width-leftW-rightW)

	return primaryStyle.Render(left) + strings.Repeat(" ", gap) + right
}

// columnWidths computes column widths based on available space.
type colLayout struct {
	name, ctype, scope, files, installed, desc int
}

func (t tableModel) columnWidths() colLayout {
	w := t.width - 4 // prefix (3) + right padding (1)

	ctype := 8
	scope := 12
	files := 5
	installed := 12

	if w >= 100 {
		// Wide: show all columns with description
		ctype = 9
		scope = 12
		installed = 14
		nameW := 22
		fixed := nameW + ctype + scope + files + installed + 5 // 5 gaps
		desc := max(15, w-fixed)
		return colLayout{nameW, ctype, scope, files, installed, desc}
	}

	// Standard: drop description
	fixed := ctype + scope + files + installed + 4 // 4 gaps
	nameW := min(20, max(12, w-fixed))
	return colLayout{nameW, ctype, scope, files, installed, 0}
}

// sortIndicator returns ▲ or ▼ if this column is the sort column, else "".
func (t tableModel) sortIndicator(col sortColumn) string {
	if t.sortCol != col {
		return ""
	}
	if t.sortAsc {
		return " ▲"
	}
	return " ▼"
}

// renderHeader renders the column header row.
func (t tableModel) renderHeader(c colLayout) string {
	row := "   " // prefix space
	row += padRight("Name"+t.sortIndicator(sortByName), c.name)
	row += " " + padRight("Type"+t.sortIndicator(sortByType), c.ctype)
	row += " " + padRight("Scope"+t.sortIndicator(sortByScope), c.scope)
	row += " " + padRight("Files"+t.sortIndicator(sortByFiles), c.files)
	row += " " + padRight("Installed"+t.sortIndicator(sortByInstalled), c.installed)
	if c.desc > 0 {
		row += " " + padRight("Description"+t.sortIndicator(sortByDesc), c.desc)
	}

	return boldStyle.Width(t.width).Render(row)
}

// renderRow renders a single data row.
func (t tableModel) renderRow(index int, c colLayout) string {
	r := t.rows[index]
	isCursor := index == t.cursor

	prefix := "   "
	if isCursor {
		prefix = " > "
	}

	row := prefix
	row += padRight(truncate(r.name, c.name), c.name)
	row += " " + padRight(truncate(r.contentType, c.ctype), c.ctype)
	row += " " + padRight(truncate(r.scope, c.scope), c.scope)
	row += " " + padRight(truncate(r.files, c.files), c.files)
	row += " " + padRight(truncate(r.installed, c.installed), c.installed)
	if c.desc > 0 {
		row += " " + truncate(r.description, c.desc)
	}

	if isCursor && t.focused {
		return zone.Mark("tbl-"+itoa(index), selectedRowStyle.Width(t.width).Render(row))
	}
	if isCursor {
		return zone.Mark("tbl-"+itoa(index), boldStyle.Width(t.width).Render(row))
	}
	return zone.Mark("tbl-"+itoa(index), mutedStyle.Width(t.width).Render(row))
}

// renderEmpty shows guidance when no items exist.
func (t tableModel) renderEmpty() string {
	return lipgloss.NewStyle().
		Width(t.width).
		Height(t.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(mutedColor).
		Render("No content in library.\nPress [a] to add your first item.")
}

// computeRows pre-computes display strings for items.
func (t tableModel) computeRows(items []catalog.ContentItem) []tableRow {
	rows := make([]tableRow, len(items))
	for i, item := range items {
		rows[i] = tableRow{
			name:        itemDisplayName(item),
			contentType: typeLabel(item.Type),
			scope:       item.Source,
			files:       itoa(len(item.Files)),
			sortFiles:   len(item.Files),
			installed:   t.installedTools(item),
			description: item.Description,
		}
	}
	return rows
}

// installedTools returns abbreviated provider names where this item is installed.
func (t tableModel) installedTools(item catalog.ContentItem) string {
	var abbrevs []string
	for _, prov := range t.providers {
		if installer.CheckStatus(item, prov, t.repoRoot) == installer.StatusInstalled {
			abbrevs = append(abbrevs, providerAbbrev(prov.Slug))
		}
	}
	if len(abbrevs) == 0 {
		return "--"
	}
	return strings.Join(abbrevs, ",")
}

// providerAbbrev returns a short abbreviation for a provider slug.
func providerAbbrev(slug string) string {
	switch slug {
	case "claude-code":
		return "CC"
	case "gemini-cli":
		return "GC"
	case "cursor":
		return "Cu"
	case "copilot":
		return "Co"
	case "windsurf":
		return "WS"
	case "kiro":
		return "Ki"
	case "cline":
		return "Cl"
	case "roo-code":
		return "RC"
	case "amp":
		return "Am"
	case "opencode":
		return "OC"
	case "zed":
		return "Zd"
	default:
		if len(slug) >= 2 {
			return strings.ToUpper(slug[:2])
		}
		return slug
	}
}

// itemDisplayName returns the best display name for an item.
func itemDisplayName(item catalog.ContentItem) string {
	if item.DisplayName != "" {
		return item.DisplayName
	}
	return item.Name
}

// typeLabel returns a human-readable label for a content type.
func typeLabel(ct catalog.ContentType) string {
	switch ct {
	case catalog.Skills:
		return "Skill"
	case catalog.Agents:
		return "Agent"
	case catalog.MCP:
		return "MCP"
	case catalog.Rules:
		return "Rule"
	case catalog.Hooks:
		return "Hook"
	case catalog.Commands:
		return "Command"
	case catalog.Loadouts:
		return "Loadout"
	default:
		return string(ct)
	}
}

// padRight pads a string with spaces to reach the given width.
func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
