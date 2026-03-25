package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// libraryMode tracks whether we're browsing the table or viewing item detail.
type libraryMode int

const (
	libraryBrowse libraryMode = iota // full-width table
	libraryDetail                    // file tree + preview drill-in
)

// libraryDrillMsg is sent when the user drills into an item from the Library table.
type libraryDrillMsg struct {
	item *catalog.ContentItem
}

// libraryCloseMsg is sent when the user closes the detail view.
type libraryCloseMsg struct{}

// libraryModel manages the Library tab: full-width table with drill-in detail view.
type libraryModel struct {
	table   tableModel
	tree    fileTreeModel
	preview previewModel
	mode    libraryMode
	focus   explorerPane // paneItems=tree, panePreview=preview (reuse enum)
	width   int
	height  int

	// The item currently being viewed in detail
	detailItem *catalog.ContentItem
}

func newLibraryModel(items []catalog.ContentItem, provs []provider.Provider, repoRoot string) libraryModel {
	return libraryModel{
		table:   newTableModel(items, provs, repoRoot),
		preview: newPreviewModel(),
		mode:    libraryBrowse,
	}
}

// SetSize updates layout dimensions.
func (l *libraryModel) SetSize(width, height int) {
	l.width = width
	l.height = height

	switch l.mode {
	case libraryBrowse:
		innerH := height - borderSize
		if l.table.Len() > 0 {
			innerH = max(3, innerH-metaBarHeight)
		}
		l.table.SetSize(width-borderSize, innerH)
	case libraryDetail:
		l.sizeDetailPanes()
	}
}

// SetItems replaces the table data and returns to browse mode.
func (l *libraryModel) SetItems(items []catalog.ContentItem) {
	l.table.SetItems(items)
	l.mode = libraryBrowse
	l.detailItem = nil
}

// Update handles input for the current mode.
func (l libraryModel) Update(msg tea.Msg) (libraryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		return l.updateMouse(msg)
	case tea.KeyMsg:
		switch l.mode {
		case libraryBrowse:
			return l.updateBrowse(msg)
		case libraryDetail:
			return l.updateDetail(msg)
		}
	}
	return l, nil
}

// updateBrowse handles keys in table browse mode.
func (l libraryModel) updateBrowse(msg tea.KeyMsg) (libraryModel, tea.Cmd) {
	// When actively searching, route most keys to the search input
	if l.table.searching {
		return l.updateSearch(msg)
	}

	switch msg.String() {
	case keyDown, "down":
		l.table.CursorDown()
	case keyUp, "up":
		l.table.CursorUp()
	case "enter":
		if item := l.table.Selected(); item != nil {
			l.drillIn(item)
			return l, func() tea.Msg { return libraryDrillMsg{item: item} }
		}
	case keySearch:
		l.table.StartSearch()
	case "s":
		l.table.CycleSort()
	case "S":
		l.table.ReverseSort()
	case "esc":
		if l.table.searchQuery != "" {
			l.table.CancelSearch()
		}
	case "pgup", "ctrl+u":
		l.table.PageUp()
	case "pgdown", "ctrl+d":
		l.table.PageDown()
	case "g", "home":
		l.table.cursor = 0
		l.table.offset = 0
	case "G", "end":
		if len(l.table.items) > 0 {
			l.table.cursor = len(l.table.items) - 1
			l.table.offset = max(0, len(l.table.items)-l.table.viewHeight())
		}
	}
	return l, nil
}

// updateSearch handles keys when the search input is active.
func (l libraryModel) updateSearch(msg tea.KeyMsg) (libraryModel, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		l.table.CancelSearch()
	case tea.KeyEnter:
		l.table.SearchConfirm()
	case tea.KeyBackspace:
		l.table.SearchBackspace()
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			l.table.SearchType(r)
		}
	}
	return l, nil
}

// updateDetail handles keys in file tree + preview detail mode.
func (l libraryModel) updateDetail(msg tea.KeyMsg) (libraryModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "x":
		l.mode = libraryBrowse
		l.detailItem = nil
		l.table.SetSize(l.width-borderSize, l.height-borderSize)
		return l, func() tea.Msg { return libraryCloseMsg{} }

	case keyLeft, "left":
		l.setDetailFocus(paneItems)
		return l, nil
	case keyRight, "right":
		l.setDetailFocus(panePreview)
		return l, nil
	}

	switch l.focus {
	case paneItems:
		return l.updateTree(msg)
	case panePreview:
		return l.updatePreviewKeys(msg)
	}
	return l, nil
}

// updateTree handles keys when file tree is focused.
func (l libraryModel) updateTree(msg tea.KeyMsg) (libraryModel, tea.Cmd) {
	switch msg.String() {
	case keyDown, "down":
		l.tree.CursorDown()
		l.loadSelectedFile()
	case keyUp, "up":
		l.tree.CursorUp()
		l.loadSelectedFile()
	case "pgup", "ctrl+u":
		l.tree.PageUp()
		l.loadSelectedFile()
	case "pgdown", "ctrl+d":
		l.tree.PageDown()
		l.loadSelectedFile()
	case "enter", " ":
		if l.tree.cursor >= 0 && l.tree.cursor < len(l.tree.nodes) {
			if l.tree.nodes[l.tree.cursor].isDir {
				l.tree.ToggleDir()
			} else {
				l.loadSelectedFile()
			}
		}
	}
	return l, nil
}

// updatePreviewKeys handles keys when preview is focused.
func (l libraryModel) updatePreviewKeys(msg tea.KeyMsg) (libraryModel, tea.Cmd) {
	switch msg.String() {
	case keyDown, "down":
		l.preview.ScrollDown()
	case keyUp, "up":
		l.preview.ScrollUp()
	case "pgdown", "ctrl+d":
		l.preview.PageDown()
	case "pgup", "ctrl+u":
		l.preview.PageUp()
	}
	return l, nil
}

// updateMouse handles mouse events.
func (l libraryModel) updateMouse(msg tea.MouseMsg) (libraryModel, tea.Cmd) {
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		switch l.mode {
		case libraryBrowse:
			// Column header clicks for sorting
			colZones := []struct {
				id  string
				col sortColumn
			}{
				{"col-name", sortByName},
				{"col-type", sortByType},
				{"col-scope", sortByScope},
				{"col-files", sortByFiles},
				{"col-installed", sortByInstalled},
				{"col-desc", sortByDesc},
			}
			for _, cz := range colZones {
				if zone.Get(cz.id).InBounds(msg) {
					l.table.SortByColumn(cz.col)
					return l, nil
				}
			}

			// Row clicks
			for i := range l.table.items {
				if zone.Get("tbl-" + itoa(i)).InBounds(msg) {
					l.table.cursor = i
					return l, nil
				}
			}
		case libraryDetail:
			// Click on file tree nodes
			for i := range l.tree.nodes {
				if zone.Get("ftnode-" + itoa(i)).InBounds(msg) {
					l.tree.cursor = i
					l.setDetailFocus(paneItems)
					if l.tree.nodes[i].isDir {
						l.tree.ToggleDir()
					} else {
						l.loadSelectedFile()
					}
					return l, nil
				}
			}
			// Click on pane areas for focus
			if zone.Get("lib-tree").InBounds(msg) {
				l.setDetailFocus(paneItems)
				return l, nil
			}
			if zone.Get("lib-preview").InBounds(msg) {
				l.setDetailFocus(panePreview)
				return l, nil
			}
		}
	}

	// Scroll wheel
	if msg.Action == tea.MouseActionPress {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if l.mode == libraryBrowse {
				l.table.CursorUp()
			} else if l.focus == paneItems {
				l.tree.CursorUp()
				l.loadSelectedFile()
			} else {
				l.preview.ScrollUp()
			}
		case tea.MouseButtonWheelDown:
			if l.mode == libraryBrowse {
				l.table.CursorDown()
			} else if l.focus == paneItems {
				l.tree.CursorDown()
				l.loadSelectedFile()
			} else {
				l.preview.ScrollDown()
			}
		}
	}

	return l, nil
}

// drillIn enters detail mode for the given item.
func (l *libraryModel) drillIn(item *catalog.ContentItem) {
	l.mode = libraryDetail
	l.detailItem = item
	l.tree = newFileTreeModel(item.Files)
	l.focus = paneItems
	l.tree.focused = true
	l.preview.focused = false
	l.sizeDetailPanes()
	l.loadSelectedFile()
}

// loadSelectedFile loads the file at the tree cursor into the preview.
func (l *libraryModel) loadSelectedFile() {
	if l.detailItem == nil {
		return
	}
	path := l.tree.SelectedPath()
	if path == "" {
		// Directory selected or empty — show primary file
		primary := catalog.PrimaryFileName(l.detailItem.Files, l.detailItem.Type)
		if primary != "" {
			path = primary
		}
	}
	if path == "" {
		l.preview.lines = nil
		l.preview.fileName = ""
		return
	}
	l.preview.fileName = path
	l.preview.offset = 0
	content, err := catalog.ReadFileContent(l.detailItem.Path, path, 500)
	if err != nil {
		l.preview.lines = []string{"Error reading file:", err.Error()}
		return
	}
	l.preview.lines = strings.Split(content, "\n")
}

// sizeDetailPanes calculates sizes for the detail mode (tree + preview).
func (l *libraryModel) sizeDetailPanes() {
	treeOuterW := l.detailTreeWidth()
	previewOuterW := l.width - treeOuterW
	innerH := max(0, l.height-borderSize)

	l.tree.SetSize(max(0, treeOuterW-borderSize), innerH)
	l.preview.SetSize(max(0, previewOuterW-borderSize), innerH)
}

// setDetailFocus switches focus between tree and preview in detail mode.
func (l *libraryModel) setDetailFocus(pane explorerPane) {
	l.focus = pane
	l.tree.focused = pane == paneItems
	l.preview.focused = pane == panePreview
}

// detailTreeWidth returns the outer width of the file tree pane.
func (l libraryModel) detailTreeWidth() int {
	if l.width >= 120 {
		return 35
	}
	return max(22, l.width*30/100)
}

// View renders the Library view based on current mode.
func (l libraryModel) View() string {
	if l.width <= 0 || l.height <= 0 {
		return ""
	}

	switch l.mode {
	case libraryDetail:
		return l.viewDetail()
	default:
		return l.viewBrowse()
	}
}

// metaBarHeight is the number of lines reserved for the metadata bar below the table.
const metaBarHeight = 3

// viewBrowse renders the full-width table with a metadata bar below.
func (l libraryModel) viewBrowse() string {
	l.table.focused = true
	innerW := l.width - borderSize
	innerH := l.height - borderSize

	// Reserve space for metadata bar when there are items
	tableH := innerH
	if l.table.Len() > 0 {
		tableH = max(3, innerH-metaBarHeight)
	}
	l.table.SetSize(innerW, tableH)

	content := l.table.View()
	if l.table.Len() > 0 {
		content += "\n" + l.renderMetadataBar(innerW, innerH-tableH)
	}
	return borderedPanel(content, innerW, innerH, focusedBorderFg)
}

// viewDetail renders file tree + preview with bordered panes.
func (l libraryModel) viewDetail() string {
	treeOuterW := l.detailTreeWidth()
	previewOuterW := l.width - treeOuterW

	treeFg := unfocusedBorderFg
	previewFg := unfocusedBorderFg
	if l.focus == paneItems {
		treeFg = focusedBorderFg
	} else {
		previewFg = focusedBorderFg
	}

	innerH := max(0, l.height-borderSize)
	treeInnerW := max(0, treeOuterW-borderSize)
	previewInnerW := max(0, previewOuterW-borderSize)

	// Tree content: item name header + file tree + close button
	itemName := ""
	if l.detailItem != nil {
		itemName = itemDisplayName(*l.detailItem)
	}
	treeHeader := boldStyle.Render(truncate(itemName, treeInnerW))
	closeBtn := zone.Mark("lib-close", mutedStyle.Render("[x] Close"))

	treeContentH := max(0, innerH-2) // header + footer
	l.tree.SetSize(treeInnerW, treeContentH)
	treeContent := treeHeader + "\n" + l.tree.View() + "\n" + closeBtn

	left := zone.Mark("lib-tree",
		borderedPanel(treeContent, treeInnerW, innerH, treeFg))

	// Preview pane with file count indicator
	fileCount := ""
	if l.detailItem != nil {
		fileCount = fmt.Sprintf(" (%d files)", len(l.detailItem.Files))
	}
	previewHeader := renderSectionTitle(l.preview.fileName+fileCount, previewInnerW)
	previewContentH := max(0, innerH-1)
	l.preview.SetSize(previewInnerW, previewContentH)

	previewBody := l.renderPreviewBody(previewContentH, previewInnerW)
	previewContent := previewHeader + "\n" + previewBody

	right := zone.Mark("lib-preview",
		borderedPanel(previewContent, previewInnerW, innerH, previewFg))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// renderMetadataBar renders a detail panel for the currently selected item.
// Shows: name, type, provider, file count, installed providers, and path/description.
func (l libraryModel) renderMetadataBar(width, height int) string {
	item := l.table.Selected()
	if item == nil || height < 2 {
		return strings.Repeat(" ", width)
	}

	// Line 1: separator
	sep := sectionRuleStyle.Render(strings.Repeat("─", width))

	// Build tag-style detail chips
	name := boldStyle.Render(itemDisplayName(*item))
	typeLbl := mutedStyle.Render(typeLabel(item.Type))
	provLbl := ""
	if item.Provider != "" {
		provLbl = mutedStyle.Render(item.Provider)
	}
	filesLbl := mutedStyle.Render(fmt.Sprintf("%d files", len(item.Files)))

	row := l.table.rows[l.table.cursor]
	installedLbl := ""
	if row.installed != "--" {
		installedLbl = lipgloss.NewStyle().Foreground(primaryColor).Render(row.installed)
	}

	dot := mutedStyle.Render(" · ")
	line1 := " " + name + dot + typeLbl
	if provLbl != "" {
		line1 += dot + provLbl
	}
	line1 += dot + filesLbl
	if installedLbl != "" {
		line1 += dot + installedLbl
	}

	// Line 2: path and description
	var parts []string
	if item.Path != "" {
		path := item.Path
		if home, err := homeDir(); err == nil && strings.HasPrefix(path, home) {
			path = "~" + path[len(home):]
		}
		parts = append(parts, mutedStyle.Render(truncate(path, width-2)))
	}
	if item.Description != "" && width > 40 {
		parts = append(parts, mutedStyle.Render(truncate(item.Description, width-2)))
	}
	line2 := " "
	if len(parts) > 0 {
		line2 += strings.Join(parts, dot)
	}

	result := sep + "\n" + truncateLine(line1, width) + "\n" + truncateLine(line2, width)

	// Pad remaining height
	usedLines := 3
	for usedLines < height {
		result += "\n" + strings.Repeat(" ", width)
		usedLines++
	}
	return result
}

// homeDir returns the user's home directory path, cached for rendering.
func homeDir() (string, error) {
	return os.UserHomeDir()
}

// renderPreviewBody renders just the preview content lines (no header).
func (l libraryModel) renderPreviewBody(height, width int) string {
	if len(l.preview.lines) == 0 {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(mutedColor).
			Render("Select a file to preview")
	}

	linesAbove := l.preview.offset
	lastVisible := min(l.preview.offset+height, len(l.preview.lines))
	linesBelow := max(0, len(l.preview.lines)-lastVisible)
	showAbove := linesAbove > 0
	showBelow := linesBelow > 0

	contentStart := l.preview.offset
	contentEnd := lastVisible
	if showAbove {
		contentStart++
	}
	if showBelow && contentEnd > contentStart {
		contentEnd--
	}

	lineNumW := len(fmt.Sprintf("%d", len(l.preview.lines)))
	if lineNumW < 2 {
		lineNumW = 2
	}

	lines := make([]string, 0, height)

	if showAbove {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("(%d more above)", linesAbove)))
	}

	for i := contentStart; i < contentEnd; i++ {
		num := mutedStyle.Render(fmt.Sprintf("%*d ", lineNumW, i+1))
		numW := lipgloss.Width(num)
		lineW := width - numW
		line := truncateLine(l.preview.lines[i], lineW)
		lines = append(lines, num+line)
	}

	if showBelow {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("(%d more below)", linesBelow)))
	}

	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}
