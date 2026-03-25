package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/tidwall/gjson"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/loadout"
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

	// Type-specific detail for metadata bar line 3 (hooks, MCP, loadouts).
	// Empty for content types with no extra metadata.
	typeDetail string
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
	t.ensureCursorVisible()
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
	t.ensureCursorVisible()
}

// PageUp moves cursor up by one page.
func (t *tableModel) PageUp() {
	vh := t.viewHeight()
	t.cursor = max(0, t.cursor-vh)
	t.offset = max(0, t.offset-vh)
	t.ensureCursorVisible()
}

// PageDown moves cursor down by one page.
func (t *tableModel) PageDown() {
	vh := t.viewHeight()
	t.cursor = min(len(t.items)-1, t.cursor+vh)
	maxOffset := max(0, len(t.items)-vh)
	t.offset = min(maxOffset, t.offset+vh)
	t.ensureCursorVisible()
}

// ensureCursorVisible adjusts offset so the cursor row is not hidden behind
// the "(N more above)" or "(N more below)" scroll indicators. Each indicator
// displaces one data row in the render, so the cursor must not be at the
// displaced position.
func (t *tableModel) ensureCursorVisible() {
	vh := t.viewHeight()
	if vh <= 2 || len(t.items) == 0 {
		return
	}

	// Basic scroll bounds: cursor must be within [offset, offset+vh)
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+vh {
		t.offset = t.cursor - vh + 1
	}

	// "More above" indicator appears when offset > 0, displacing the item
	// at position offset. If cursor == offset, scroll up 1 to make room.
	if t.offset > 0 && t.cursor == t.offset {
		t.offset--
	}

	// "More below" indicator appears when offset+vh < len(items), displacing
	// the item at position offset+vh-1. If cursor is there, scroll down 1.
	if t.offset+vh < len(t.items) && t.cursor == t.offset+vh-1 {
		t.offset++
	}
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

// SortByColumn sorts by the given column. If already sorting by that column,
// reverses direction. Otherwise sorts by the new column keeping current direction.
func (t *tableModel) SortByColumn(col sortColumn) {
	if t.sortCol == col {
		t.sortAsc = !t.sortAsc
	} else {
		t.sortCol = col
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
				strings.Contains(strings.ToLower(item.DisplayName), q) ||
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

	// Clamp to exact height — never render more lines than allocated
	if len(lines) > t.height {
		lines = lines[:t.height]
	}

	return strings.Join(lines, "\n")
}

// renderSearchBar renders the search input line with a background-tinted field.
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

	hints := ""
	if t.searching {
		hints = "  esc cancel · enter confirm"
	} else {
		hints = "  / edit · esc clear"
	}

	// Right side: match count + hints (rendered outside the input field)
	rightText := matchInfo + hints
	rightRendered := mutedStyle.Render(rightText)
	rightW := lipgloss.Width(rightRendered)

	// Input field gets the remaining width
	fieldW := max(10, t.width-rightW-1) // -1 for gap
	fieldContent := prompt + query + cursor

	bg := inputActiveBG
	if !t.searching {
		bg = inputInactiveBG
	}
	fieldContent = truncateLine(fieldContent, fieldW)
	fieldStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(primaryText).
		MaxWidth(fieldW)
	field := fieldStyle.Render(fieldContent)
	if gap := fieldW - lipgloss.Width(field); gap > 0 {
		// Pad inside the background color
		field = lipgloss.NewStyle().Background(bg).Render(field + strings.Repeat(" ", gap))
	}

	return field + " " + rightRendered
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

// renderHeader renders the column header row with clickable zone markers.
func (t tableModel) renderHeader(c colLayout) string {
	row := "   " // prefix space
	row += zone.Mark("col-name", t.headerCell("Name", sortByName, c.name))
	row += " " + zone.Mark("col-type", t.headerCell("Type", sortByType, c.ctype))
	row += " " + zone.Mark("col-scope", t.headerCell("Scope", sortByScope, c.scope))
	row += " " + zone.Mark("col-files", t.headerCell("Files", sortByFiles, c.files))
	row += " " + zone.Mark("col-installed", t.headerCell("Installed", sortByInstalled, c.installed))
	if c.desc > 0 {
		row += " " + zone.Mark("col-desc", t.headerCell("Description", sortByDesc, c.desc))
	}

	// MaxWidth clips without wrapping. Manual pad ensures full-width background.
	rendered := boldStyle.MaxWidth(t.width).Render(row)
	if gap := t.width - lipgloss.Width(rendered); gap > 0 {
		rendered += strings.Repeat(" ", gap)
	}
	return rendered
}

// headerCell renders a column header, fitting the label + sort indicator within colWidth.
func (t tableModel) headerCell(label string, col sortColumn, colWidth int) string {
	indicator := t.sortIndicator(col)
	if indicator == "" {
		return padRight(label, colWidth)
	}
	// Use rune-based widths since indicator contains multi-byte chars (▲/▼)
	indicatorRunes := []rune(indicator)
	labelRunes := []rune(label)
	indicatorW := len(indicatorRunes)
	maxLabel := colWidth - indicatorW
	if maxLabel < 1 {
		return padRight(string(indicatorRunes), colWidth)
	}
	if len(labelRunes) > maxLabel {
		labelRunes = labelRunes[:maxLabel]
	}
	return padRight(string(labelRunes)+indicator, colWidth)
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

	// Hard-clip the assembled row to prevent wrapping (plain text, safe to clip by rune).
	row = truncateLine(row, t.width)

	style := mutedStyle
	if isCursor && t.focused {
		style = selectedRowStyle
	} else if isCursor {
		style = boldStyle
	}
	// MaxWidth clips without wrapping. Manual pad for full-width row background.
	rendered := style.MaxWidth(t.width).Render(row)
	if gap := t.width - lipgloss.Width(rendered); gap > 0 {
		rendered += strings.Repeat(" ", gap)
	}
	return zone.Mark("tbl-"+itoa(index), rendered)
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

// sanitizeLine strips newlines, carriage returns, and tabs from a string
// so it is safe to use as a single-line table cell.
func sanitizeLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.TrimSpace(s)
}

// computeRows pre-computes display strings for items.
func (t tableModel) computeRows(items []catalog.ContentItem) []tableRow {
	rows := make([]tableRow, len(items))
	for i, item := range items {
		rows[i] = tableRow{
			name:        sanitizeLine(itemDisplayName(item)),
			contentType: typeLabel(item.Type),
			scope:       sanitizeLine(item.Source),
			files:       itoa(len(item.Files)),
			sortFiles:   len(item.Files),
			installed:   t.installedTools(item),
			description: sanitizeLine(item.Description),
			typeDetail:  computeTypeDetail(item),
		}
	}
	return rows
}

// computeTypeDetail returns type-specific metadata for hooks, MCP, and loadouts.
// Returns empty string for other content types.
func computeTypeDetail(item catalog.ContentItem) string {
	switch item.Type {
	case catalog.Hooks:
		return computeHookDetail(item)
	case catalog.MCP:
		return computeMCPDetail(item)
	case catalog.Loadouts:
		return computeLoadoutDetail(item)
	}
	return ""
}

// computeHookDetail extracts event, matcher, and handler type from hook.json.
func computeHookDetail(item catalog.ContentItem) string {
	hookPath := filepath.Join(item.Path, "hook.json")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		return ""
	}
	event := gjson.GetBytes(data, "event").String()
	matcher := gjson.GetBytes(data, "matcher").String()
	hookType := gjson.GetBytes(data, "hooks.0.type").String()
	if hookType == "" {
		hookType = "command"
	}

	var parts []string
	if event != "" {
		parts = append(parts, "Event: "+event)
	}
	if matcher != "" {
		parts = append(parts, "Matcher: "+matcher)
	}
	parts = append(parts, "Handler: "+hookType)
	return strings.Join(parts, " · ")
}

// computeMCPDetail extracts server key and command from config.json.
func computeMCPDetail(item catalog.ContentItem) string {
	configPath := filepath.Join(item.Path, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}
	servers := gjson.GetBytes(data, "mcpServers")
	if !servers.Exists() || !servers.IsObject() {
		return ""
	}

	var parts []string
	// Use ServerKey if set, otherwise take first key
	key := item.ServerKey
	if key == "" {
		servers.ForEach(func(k, _ gjson.Result) bool {
			key = k.String()
			return false // stop after first
		})
	}
	if key != "" {
		parts = append(parts, "Server: "+key)
	}
	cmd := gjson.GetBytes(data, "mcpServers."+key+".command").String()
	args := gjson.GetBytes(data, "mcpServers."+key+".args")
	if cmd != "" {
		cmdStr := cmd
		if args.Exists() && args.IsArray() {
			for _, a := range args.Array() {
				cmdStr += " " + a.String()
			}
		}
		parts = append(parts, "Command: "+cmdStr)
	}
	return strings.Join(parts, " · ")
}

// computeLoadoutDetail extracts target provider and item counts from loadout.yaml.
func computeLoadoutDetail(item catalog.ContentItem) string {
	manifest, err := loadout.Parse(filepath.Join(item.Path, "loadout.yaml"))
	if err != nil {
		return ""
	}

	var parts []string
	if manifest.Provider != "" {
		parts = append(parts, "Target: "+manifest.Provider)
	}

	// Build item count summary
	var counts []string
	if n := len(manifest.Skills); n > 0 {
		counts = append(counts, itoa(n)+" skills")
	}
	if n := len(manifest.Rules); n > 0 {
		counts = append(counts, itoa(n)+" rules")
	}
	if n := len(manifest.Hooks); n > 0 {
		counts = append(counts, itoa(n)+" hooks")
	}
	if n := len(manifest.Agents); n > 0 {
		counts = append(counts, itoa(n)+" agents")
	}
	if n := len(manifest.MCP); n > 0 {
		counts = append(counts, itoa(n)+" mcp")
	}
	if n := len(manifest.Commands); n > 0 {
		counts = append(counts, itoa(n)+" commands")
	}
	if len(counts) > 0 {
		parts = append(parts, strings.Join(counts, ", "))
	}
	return strings.Join(parts, " · ")
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
