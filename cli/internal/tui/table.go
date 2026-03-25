package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// tableModel renders a full-width scrollable table for the Library view.
type tableModel struct {
	items   []catalog.ContentItem
	rows    []tableRow // pre-computed display data
	cursor  int
	offset  int
	width   int
	height  int
	focused bool

	// Provider data for Tools/Installed columns
	providers []provider.Provider
	repoRoot  string
}

// tableRow holds pre-computed display strings for a single item.
type tableRow struct {
	name        string
	contentType string
	scope       string
	files       string
	tools       string
	installed   string
	description string
}

func newTableModel(items []catalog.ContentItem, provs []provider.Provider, repoRoot string) tableModel {
	t := tableModel{
		items:     items,
		providers: provs,
		repoRoot:  repoRoot,
	}
	t.rows = t.computeRows()
	return t
}

// SetSize updates table dimensions.
func (t *tableModel) SetSize(width, height int) {
	t.width = width
	t.height = height
}

// SetItems replaces the table data.
func (t *tableModel) SetItems(items []catalog.ContentItem) {
	t.items = items
	t.rows = t.computeRows()
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

// Len returns item count.
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

// viewHeight returns the number of rows available for data (minus header).
func (t tableModel) viewHeight() int {
	return max(0, t.height-1) // 1 for header row
}

// View renders the table.
func (t tableModel) View() string {
	if t.width <= 0 || t.height <= 0 {
		return ""
	}

	if len(t.items) == 0 {
		return t.renderEmpty()
	}

	cols := t.columnWidths()
	lines := make([]string, 0, t.height)

	// Header row
	lines = append(lines, t.renderHeader(cols))

	vh := t.viewHeight()
	lastVisible := min(t.offset+vh, len(t.items))

	// Scroll indicators
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

	// Pad remaining height
	for len(lines) < t.height {
		lines = append(lines, strings.Repeat(" ", t.width))
	}

	return strings.Join(lines, "\n")
}

// columnWidths computes column widths based on available space.
type colLayout struct {
	name, ctype, scope, files, tools, installed, desc int
}

func (t tableModel) columnWidths() colLayout {
	w := t.width - 4 // prefix (3) + right padding (1)

	// Give non-name columns reasonable fixed sizes first, then name gets capped remainder
	ctype := 8
	scope := 12
	files := 5
	tools := 10
	installed := 10

	if w >= 110 {
		// Wide: show all columns with description
		ctype = 9
		scope = 12
		tools = 12
		installed = 10
		nameW := 22
		fixed := nameW + ctype + scope + files + tools + installed + 6 // 6 gaps
		desc := max(15, w-fixed)
		return colLayout{nameW, ctype, scope, files, tools, installed, desc}
	}

	// Standard: drop description, cap name at 20
	fixed := ctype + scope + files + tools + installed + 5 // 5 gaps
	nameW := min(20, max(12, w-fixed))
	return colLayout{nameW, ctype, scope, files, tools, installed, 0}
}

// renderHeader renders the column header row.
func (t tableModel) renderHeader(c colLayout) string {
	row := "   " // prefix space
	row += padRight("Name", c.name)
	row += " " + padRight("Type", c.ctype)
	row += " " + padRight("Scope", c.scope)
	row += " " + padRight("Files", c.files)
	row += " " + padRight("Tools", c.tools)
	row += " " + padRight("Inst.", c.installed)
	if c.desc > 0 {
		row += " " + padRight("Description", c.desc)
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

	// Build plain text row — no per-column styling so selectedRowStyle background
	// can span the entire row without being overridden by column foreground colors.
	row := prefix
	row += padRight(truncate(r.name, c.name), c.name)
	row += " " + padRight(truncate(r.contentType, c.ctype), c.ctype)
	row += " " + padRight(truncate(r.scope, c.scope), c.scope)
	row += " " + padRight(truncate(r.files, c.files), c.files)
	row += " " + padRight(truncate(r.tools, c.tools), c.tools)
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
	// Non-selected rows use muted style for a subtler look
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

// computeRows pre-computes display strings for all items.
func (t tableModel) computeRows() []tableRow {
	rows := make([]tableRow, len(t.items))
	for i, item := range t.items {
		rows[i] = tableRow{
			name:        itemDisplayName(item),
			contentType: typeLabel(item.Type),
			scope:       item.Source,
			files:       itoa(len(item.Files)),
			tools:       t.supportedTools(item),
			installed:   t.installedTools(item),
			description: item.Description,
		}
	}
	return rows
}

// supportedTools returns abbreviated provider names that support this item's type.
func (t tableModel) supportedTools(item catalog.ContentItem) string {
	var abbrevs []string
	for _, prov := range t.providers {
		if prov.SupportsType(item.Type) {
			abbrevs = append(abbrevs, providerAbbrev(prov.Slug))
		}
	}
	return strings.Join(abbrevs, ",")
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
