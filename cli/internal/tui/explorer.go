package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// explorerPane tracks which pane is focused within the explorer.
type explorerPane int

const (
	paneItems explorerPane = iota
	panePreview
)

// explorerMode tracks whether we're browsing items or viewing item detail.
type explorerMode int

const (
	explorerBrowse explorerMode = iota // items list + preview
	explorerDetail                     // file tree + preview drill-in
)

// itemSelectedMsg is sent when the selected item changes.
type itemSelectedMsg struct {
	item *catalog.ContentItem
}

// explorerDrillMsg is sent when the user drills into an item from the explorer.
type explorerDrillMsg struct {
	item *catalog.ContentItem
}

// explorerCloseMsg is sent when the user closes the detail view.
type explorerCloseMsg struct{}

// explorerModel is the main content area: items list (left) + preview (right).
// Rendered inside a unified bordered frame with metadata panel at top.
// Supports drill-in detail mode with file tree + preview for a single item.
type explorerModel struct {
	items     itemsModel
	preview   previewModel
	tree      fileTreeModel
	focus     explorerPane
	mode      explorerMode
	width     int
	height    int
	providers []provider.Provider
	repoRoot  string

	// The item currently being viewed in detail
	detailItem *catalog.ContentItem

	// Responsive: at narrow widths, show stacked layout
	stacked bool // true when width < 80
}

func newExplorerModel(items []catalog.ContentItem, mixed bool, providers []provider.Provider, repoRoot string) explorerModel {
	e := explorerModel{
		items:     newItemsModel(items, mixed),
		preview:   newPreviewModel(),
		focus:     paneItems,
		providers: providers,
		repoRoot:  repoRoot,
	}
	e.items.focused = true
	// Auto-select first item
	if len(items) > 0 {
		e.preview.LoadItem(&items[0])
	}
	return e
}

// borderSize is the width/height consumed by a rounded border (1 char each side).
const borderSize = 2

// SetSize recalculates child dimensions, accounting for borders and metadata panel.
func (e *explorerModel) SetSize(width, height int) {
	e.width = width
	e.height = height
	e.stacked = width < 80

	switch e.mode {
	case explorerDetail:
		e.sizeDetailPanes()
	default:
		e.sizeBrowsePanes()
	}
}

// sizeBrowsePanes calculates sizes for browse mode (items + preview).
func (e *explorerModel) sizeBrowsePanes() {
	innerW := max(0, e.width-borderSize)
	// Reserve space for metadata (3 lines) + separator (1) + top/bottom borders (2)
	paneH := max(3, e.height-borderSize-metaBarLines-1)

	if e.stacked {
		e.items.SetSize(innerW, paneH)
		e.preview.SetSize(innerW, paneH)
	} else {
		itemsOuterW := e.itemsWidth()
		itemsInnerW := max(0, itemsOuterW-1) // -1 for vertical divider
		previewInnerW := max(0, innerW-itemsInnerW-1)

		e.items.SetSize(itemsInnerW, paneH)
		e.preview.SetSize(previewInnerW, paneH)
	}
}

// sizeDetailPanes calculates sizes for the detail mode (tree + preview).
func (e *explorerModel) sizeDetailPanes() {
	treeOuterW := e.detailTreeWidth()
	previewOuterW := e.width - treeOuterW
	paneH := max(0, e.height-metaBarTotal)
	innerH := max(0, paneH-borderSize)

	e.tree.SetSize(max(0, treeOuterW-borderSize), innerH)
	e.preview.SetSize(max(0, previewOuterW-borderSize), innerH)
}

// detailTreeWidth returns the outer width of the file tree pane in detail mode.
func (e explorerModel) detailTreeWidth() int {
	if e.width >= 120 {
		return 35
	}
	return max(22, e.width*30/100)
}

// SetItems replaces the item list, reloads preview, and returns to browse mode.
func (e *explorerModel) SetItems(items []catalog.ContentItem, mixed bool) {
	e.items.SetItems(items, mixed)
	e.mode = explorerBrowse
	e.detailItem = nil
	e.setFocus(paneItems)
	if len(items) > 0 {
		e.preview.LoadItem(&items[0])
	} else {
		e.preview.LoadItem(nil)
	}
}

// Update handles key and mouse input based on current focus.
func (e explorerModel) Update(msg tea.Msg) (explorerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		return e.updateMouse(msg)

	case tea.KeyMsg:
		switch e.mode {
		case explorerDetail:
			return e.updateDetailKeys(msg)
		default:
			return e.updateBrowseKeys(msg)
		}
	}

	return e, nil
}

// updateBrowseKeys handles keys in browse mode (items + preview).
func (e explorerModel) updateBrowseKeys(msg tea.KeyMsg) (explorerModel, tea.Cmd) {
	switch msg.String() {
	// h/l and arrow keys switch pane focus
	case keyLeft, "left":
		if e.focus != paneItems {
			e.setFocus(paneItems)
		}
		return e, nil
	case keyRight, "right":
		if e.focus != panePreview {
			e.setFocus(panePreview)
		}
		return e, nil

	case keySearch:
		// TODO: Phase 3.5 — open search input overlay
		return e, nil
	}

	switch e.focus {
	case paneItems:
		return e.updateItems(msg)
	case panePreview:
		return e.updatePreview(msg)
	}
	return e, nil
}

// updateDetailKeys handles keys in detail mode (file tree + preview).
func (e explorerModel) updateDetailKeys(msg tea.KeyMsg) (explorerModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "x":
		e.mode = explorerBrowse
		e.detailItem = nil
		e.sizeBrowsePanes()
		// Reload preview from selected item
		e.preview.LoadItem(e.items.Selected())
		return e, func() tea.Msg { return explorerCloseMsg{} }

	case keyLeft, "left":
		e.setDetailFocus(paneItems)
		return e, nil
	case keyRight, "right":
		e.setDetailFocus(panePreview)
		return e, nil
	}

	switch e.focus {
	case paneItems:
		return e.updateTreeKeys(msg)
	case panePreview:
		return e.updateDetailPreviewKeys(msg)
	}
	return e, nil
}

// updateTreeKeys handles keys when the file tree is focused in detail mode.
func (e explorerModel) updateTreeKeys(msg tea.KeyMsg) (explorerModel, tea.Cmd) {
	switch msg.String() {
	case keyDown, "down":
		e.tree.CursorDown()
		e.loadSelectedFile()
	case keyUp, "up":
		e.tree.CursorUp()
		e.loadSelectedFile()
	case "pgup", "ctrl+u":
		e.tree.PageUp()
		e.loadSelectedFile()
	case "pgdown", "ctrl+d":
		e.tree.PageDown()
		e.loadSelectedFile()
	case "enter", " ":
		if e.tree.cursor >= 0 && e.tree.cursor < len(e.tree.nodes) {
			if e.tree.nodes[e.tree.cursor].isDir {
				e.tree.ToggleDir()
			} else {
				e.loadSelectedFile()
			}
		}
	}
	return e, nil
}

// updateDetailPreviewKeys handles keys when preview is focused in detail mode.
func (e explorerModel) updateDetailPreviewKeys(msg tea.KeyMsg) (explorerModel, tea.Cmd) {
	switch msg.String() {
	case keyDown, "down":
		e.preview.ScrollDown()
	case keyUp, "up":
		e.preview.ScrollUp()
	case "pgdown", "ctrl+d":
		e.preview.PageDown()
	case "pgup", "ctrl+u":
		e.preview.PageUp()
	}
	return e, nil
}

// updateMouse handles mouse clicks on items and scroll wheel.
func (e explorerModel) updateMouse(msg tea.MouseMsg) (explorerModel, tea.Cmd) {
	if e.mode == explorerDetail {
		return e.updateDetailMouse(msg)
	}
	return e.updateBrowseMouse(msg)
}

// updateBrowseMouse handles mouse events in browse mode.
func (e explorerModel) updateBrowseMouse(msg tea.MouseMsg) (explorerModel, tea.Cmd) {
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		// Rename button click
		if zone.Get("meta-rename").InBounds(msg) {
			return e, func() tea.Msg { return libraryRenameMsg{} }
		}

		// Click on a specific item row — select it
		for i := range e.items.items {
			if zone.Get("item-" + itoa(i)).InBounds(msg) {
				if e.items.cursor == i {
					// Second click on same row — drill in
					if item := e.items.Selected(); item != nil {
						e.drillIn(item)
						return e, func() tea.Msg { return explorerDrillMsg{item: item} }
					}
				}
				e.items.cursor = i
				e.setFocus(paneItems)
				e.preview.LoadItem(e.items.Selected())
				return e, e.itemSelectedCmd()
			}
		}

		// Click anywhere in items pane — focus it
		if zone.Get("pane-items").InBounds(msg) {
			e.setFocus(paneItems)
			return e, nil
		}

		// Click anywhere in preview pane — focus it
		if zone.Get("pane-preview").InBounds(msg) {
			e.setFocus(panePreview)
			return e, nil
		}
	}

	// Scroll wheel — scroll whichever pane the mouse is over
	if msg.Action == tea.MouseActionPress {
		onItems := zone.Get("pane-items").InBounds(msg)
		onPreview := zone.Get("pane-preview").InBounds(msg)

		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if onItems {
				e.items.CursorUp()
				e.preview.LoadItem(e.items.Selected())
				return e, e.itemSelectedCmd()
			}
			if onPreview {
				e.preview.ScrollUp()
				return e, nil
			}
		case tea.MouseButtonWheelDown:
			if onItems {
				e.items.CursorDown()
				e.preview.LoadItem(e.items.Selected())
				return e, e.itemSelectedCmd()
			}
			if onPreview {
				e.preview.ScrollDown()
				return e, nil
			}
		}
	}

	return e, nil
}

// updateDetailMouse handles mouse events in detail mode.
func (e explorerModel) updateDetailMouse(msg tea.MouseMsg) (explorerModel, tea.Cmd) {
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		// Rename button click
		if zone.Get("meta-rename").InBounds(msg) {
			return e, func() tea.Msg { return libraryRenameMsg{} }
		}

		// Close button click
		if zone.Get("exp-close").InBounds(msg) {
			e.mode = explorerBrowse
			e.detailItem = nil
			e.sizeBrowsePanes()
			e.preview.LoadItem(e.items.Selected())
			return e, func() tea.Msg { return explorerCloseMsg{} }
		}

		// Click on file tree nodes
		for i := range e.tree.nodes {
			if zone.Get("ftnode-" + itoa(i)).InBounds(msg) {
				e.tree.cursor = i
				e.setDetailFocus(paneItems)
				if e.tree.nodes[i].isDir {
					e.tree.ToggleDir()
				} else {
					e.loadSelectedFile()
				}
				return e, nil
			}
		}
	}

	// Scroll wheel
	if msg.Action == tea.MouseActionPress {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if e.focus == paneItems {
				e.tree.CursorUp()
				e.loadSelectedFile()
			} else {
				e.preview.ScrollUp()
			}
		case tea.MouseButtonWheelDown:
			if e.focus == paneItems {
				e.tree.CursorDown()
				e.loadSelectedFile()
			} else {
				e.preview.ScrollDown()
			}
		}
	}

	return e, nil
}

// updateItems handles keys when items list is focused.
func (e explorerModel) updateItems(msg tea.KeyMsg) (explorerModel, tea.Cmd) {
	switch msg.String() {
	case keyDown, "down":
		e.items.CursorDown()
		e.preview.LoadItem(e.items.Selected())
		return e, e.itemSelectedCmd()
	case keyUp, "up":
		e.items.CursorUp()
		e.preview.LoadItem(e.items.Selected())
		return e, e.itemSelectedCmd()
	case "pgup", "ctrl+u":
		e.items.PageUp()
		e.preview.LoadItem(e.items.Selected())
		return e, e.itemSelectedCmd()
	case "pgdown", "ctrl+d":
		e.items.PageDown()
		e.preview.LoadItem(e.items.Selected())
		return e, e.itemSelectedCmd()
	case "g", "home":
		e.items.cursor = 0
		e.items.offset = 0
		e.preview.LoadItem(e.items.Selected())
		return e, e.itemSelectedCmd()
	case "G", "end":
		if len(e.items.items) > 0 {
			e.items.cursor = len(e.items.items) - 1
			e.items.offset = max(0, len(e.items.items)-e.items.height)
		}
		e.preview.LoadItem(e.items.Selected())
		return e, e.itemSelectedCmd()
	case "enter":
		if item := e.items.Selected(); item != nil {
			if e.stacked {
				e.setFocus(panePreview)
			}
			e.drillIn(item)
			return e, func() tea.Msg { return explorerDrillMsg{item: item} }
		}
		return e, nil
	}
	return e, nil
}

// updatePreview handles keys when preview is focused.
func (e explorerModel) updatePreview(msg tea.KeyMsg) (explorerModel, tea.Cmd) {
	switch msg.String() {
	case keyDown, "down":
		e.preview.ScrollDown()
	case keyUp, "up":
		e.preview.ScrollUp()
	case "pgdown", "ctrl+d":
		e.preview.PageDown()
	case "pgup", "ctrl+u":
		e.preview.PageUp()
	case "esc":
		if e.stacked {
			e.setFocus(paneItems)
		}
	}
	return e, nil
}

// drillIn enters detail mode for the given item.
func (e *explorerModel) drillIn(item *catalog.ContentItem) {
	e.mode = explorerDetail
	e.detailItem = item
	e.tree = newFileTreeModel(item.Files)
	e.focus = paneItems
	e.tree.focused = true
	e.preview.focused = false
	e.sizeDetailPanes()
	e.loadSelectedFile()
}

// loadSelectedFile loads the file at the tree cursor into the preview.
func (e *explorerModel) loadSelectedFile() {
	if e.detailItem == nil {
		return
	}
	path := e.tree.SelectedPath()
	if path == "" {
		// Directory selected or empty — show primary file
		primary := catalog.PrimaryFileName(e.detailItem.Files, e.detailItem.Type)
		if primary != "" {
			path = primary
		}
	}
	if path == "" {
		e.preview.lines = nil
		e.preview.fileName = ""
		return
	}
	e.preview.fileName = path
	e.preview.offset = 0
	content, err := catalog.ReadFileContent(e.detailItem.Path, path, 500)
	if err != nil {
		e.preview.lines = []string{"Error reading file:", err.Error()}
		return
	}
	e.preview.lines = strings.Split(content, "\n")
}

// setDetailFocus switches focus between tree and preview in detail mode.
func (e *explorerModel) setDetailFocus(pane explorerPane) {
	e.focus = pane
	e.tree.focused = pane == paneItems
	e.preview.focused = pane == panePreview
}

// setFocus switches focus to the given pane (browse mode).
func (e *explorerModel) setFocus(pane explorerPane) {
	e.focus = pane
	e.items.focused = pane == paneItems
	e.preview.focused = pane == panePreview
}

// View renders the explorer layout with bordered panes.
func (e explorerModel) View() string {
	if e.width <= 0 || e.height <= 0 {
		return ""
	}

	switch e.mode {
	case explorerDetail:
		return e.viewDetail()
	default:
		if e.stacked {
			return e.viewStacked()
		}
		return e.viewSideBySide()
	}
}

// viewSideBySide renders a unified frame: metadata + ├──┬──┤ + items│preview.
func (e explorerModel) viewSideBySide() string {
	innerW := e.width - borderSize
	itemsOuterW := e.itemsWidth()
	itemsInnerW := max(0, itemsOuterW-1) // -1 for vertical divider
	previewInnerW := max(0, innerW-itemsInnerW-1)
	paneH := max(3, e.height-borderSize-metaBarLines-1)

	// Compute metadata for selected item
	metaContent := e.renderMetadataContent(innerW)

	// Build separator with T-junction
	separator := sectionRuleStyle.Render("├" + strings.Repeat("─", itemsInnerW) + "┬" + strings.Repeat("─", previewInnerW) + "┤")

	// Get pane content
	itemsLines := strings.Split(e.items.View(), "\n")
	previewLines := strings.Split(e.preview.View(), "\n")
	for len(itemsLines) < paneH {
		itemsLines = append(itemsLines, strings.Repeat(" ", itemsInnerW))
	}
	for len(previewLines) < paneH {
		previewLines = append(previewLines, strings.Repeat(" ", previewInnerW))
	}

	border := sectionRuleStyle.Render
	topBorder := border("╭" + strings.Repeat("─", innerW) + "╮")
	bottomBorder := border("╰" + strings.Repeat("─", itemsInnerW) + "┴" + strings.Repeat("─", previewInnerW) + "╯")

	wrapLine := func(s string, w int) string {
		s = lipgloss.NewStyle().MaxWidth(w).Render(s)
		if g := w - lipgloss.Width(s); g > 0 {
			s += strings.Repeat(" ", g)
		}
		return s
	}

	var lines []string
	lines = append(lines, topBorder)
	for _, ml := range strings.Split(metaContent, "\n") {
		lines = append(lines, border("│")+wrapLine(ml, innerW)+border("│"))
	}
	lines = append(lines, separator)
	for i := 0; i < paneH; i++ {
		il := ""
		if i < len(itemsLines) {
			il = itemsLines[i]
		}
		pl := ""
		if i < len(previewLines) {
			pl = previewLines[i]
		}
		lines = append(lines, border("│")+wrapLine(il, itemsInnerW)+border("│")+wrapLine(pl, previewInnerW)+border("│"))
	}
	lines = append(lines, bottomBorder)

	return strings.Join(lines, "\n")
}

// viewDetail renders a unified frame: metadata + ├──┬──┤ + tree│preview (drill-in mode).
func (e explorerModel) viewDetail() string {
	innerW := e.width - borderSize
	totalInnerH := e.height - borderSize

	// Metadata gets metaBarLines, separator gets 1, panes get the rest
	paneH := max(3, totalInnerH-metaBarLines-1)

	treeOuterW := e.detailTreeWidth()
	treeInnerW := max(0, treeOuterW-1) // -1 for the vertical divider
	previewInnerW := max(0, innerW-treeInnerW-1)

	metaContent := e.renderMetadataContent(innerW)

	// Build separator with T-junction: ├──────┬──────────────────┤
	sepLeft := strings.Repeat("─", treeInnerW)
	sepRight := strings.Repeat("─", previewInnerW)
	separator := sectionRuleStyle.Render("├" + sepLeft + "┬" + sepRight + "┤")

	// Build tree content
	treeContentH := max(0, paneH)
	closeBtn := zone.Mark("exp-close", mutedStyle.Render("[x] Close"))
	treeViewH := max(0, treeContentH-1) // -1 for close button
	e.tree.SetSize(treeInnerW, treeViewH)
	treeLines := strings.Split(e.tree.View(), "\n")
	for len(treeLines) < treeViewH {
		treeLines = append(treeLines, strings.Repeat(" ", treeInnerW))
	}
	if len(treeLines) > treeViewH {
		treeLines = treeLines[:treeViewH]
	}
	treeLines = append(treeLines, closeBtn)

	// Build preview content
	fileCount := ""
	if e.detailItem != nil {
		fileCount = fmt.Sprintf(" (%d files)", len(e.detailItem.Files))
	}
	previewHeader := renderSectionTitle(e.preview.fileName+fileCount, previewInnerW)
	previewViewH := max(0, paneH-1) // -1 for header
	e.preview.SetSize(previewInnerW, previewViewH)
	previewBody := e.renderDetailPreviewBody(previewViewH, previewInnerW)
	previewLines := []string{previewHeader}
	previewLines = append(previewLines, strings.Split(previewBody, "\n")...)
	for len(previewLines) < paneH {
		previewLines = append(previewLines, strings.Repeat(" ", previewInnerW))
	}
	if len(previewLines) > paneH {
		previewLines = previewLines[:paneH]
	}

	// Assemble the frame
	border := sectionRuleStyle.Render
	topBorder := border("╭" + strings.Repeat("─", innerW) + "╮")
	bottomLeft := strings.Repeat("─", treeInnerW)
	bottomRight := strings.Repeat("─", previewInnerW)
	bottomBorder := border("╰" + bottomLeft + "┴" + bottomRight + "╯")

	wrapLine := func(s string, w int) string {
		s = lipgloss.NewStyle().MaxWidth(w).Render(s)
		if g := w - lipgloss.Width(s); g > 0 {
			s += strings.Repeat(" ", g)
		}
		return s
	}

	var lines []string
	lines = append(lines, topBorder)
	for _, ml := range strings.Split(metaContent, "\n") {
		lines = append(lines, border("│")+wrapLine(ml, innerW)+border("│"))
	}
	lines = append(lines, separator)
	for i := 0; i < paneH; i++ {
		tl := ""
		if i < len(treeLines) {
			tl = treeLines[i]
		}
		pl := ""
		if i < len(previewLines) {
			pl = previewLines[i]
		}
		lines = append(lines, border("│")+wrapLine(tl, treeInnerW)+border("│")+wrapLine(pl, previewInnerW)+border("│"))
	}
	lines = append(lines, bottomBorder)

	return strings.Join(lines, "\n")
}

// renderDetailPreviewBody renders preview content lines for the detail view (no header).
func (e explorerModel) renderDetailPreviewBody(height, width int) string {
	if len(e.preview.lines) == 0 {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(mutedColor).
			Render("Select a file to preview")
	}

	linesAbove := e.preview.offset
	lastVisible := min(e.preview.offset+height, len(e.preview.lines))
	linesBelow := max(0, len(e.preview.lines)-lastVisible)
	showAbove := linesAbove > 0
	showBelow := linesBelow > 0

	contentStart := e.preview.offset
	contentEnd := lastVisible
	if showAbove {
		contentStart++
	}
	if showBelow && contentEnd > contentStart {
		contentEnd--
	}

	lineNumW := len(fmt.Sprintf("%d", len(e.preview.lines)))
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
		line := truncateLine(e.preview.lines[i], lineW)
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

// renderMetadataContent returns 3 lines of metadata for the selected/detail item.
func (e explorerModel) renderMetadataContent(width int) string {
	var item *catalog.ContentItem
	if e.mode == explorerDetail {
		item = e.detailItem
	} else {
		item = e.items.Selected()
	}
	if item == nil {
		return renderMetaPanel(nil, metaPanelData{}, width)
	}
	data := computeMetaPanelData(*item, e.providers, e.repoRoot)
	return renderMetaPanel(item, data, width)
}

// viewStacked renders metadata + single bordered pane based on focus.
func (e explorerModel) viewStacked() string {
	innerW := e.width - borderSize
	paneH := max(3, e.height-borderSize-metaBarLines-1)

	metaContent := e.renderMetadataContent(innerW)

	paneContent := e.items.View()
	if e.focus == panePreview {
		paneContent = e.preview.View()
	}
	paneLines := strings.Split(paneContent, "\n")
	for len(paneLines) < paneH {
		paneLines = append(paneLines, strings.Repeat(" ", innerW))
	}

	border := sectionRuleStyle.Render
	topBorder := border("╭" + strings.Repeat("─", innerW) + "╮")
	separator := border("├" + strings.Repeat("─", innerW) + "┤")
	bottomBorder := border("╰" + strings.Repeat("─", innerW) + "╯")

	wrapLine := func(s string, w int) string {
		s = lipgloss.NewStyle().MaxWidth(w).Render(s)
		if g := w - lipgloss.Width(s); g > 0 {
			s += strings.Repeat(" ", g)
		}
		return s
	}

	var lines []string
	lines = append(lines, topBorder)
	for _, ml := range strings.Split(metaContent, "\n") {
		lines = append(lines, border("│")+wrapLine(ml, innerW)+border("│"))
	}
	lines = append(lines, separator)
	for _, pl := range paneLines {
		lines = append(lines, border("│")+wrapLine(pl, innerW)+border("│"))
	}
	lines = append(lines, bottomBorder)

	return strings.Join(lines, "\n")
}

// itemsWidth returns the OUTER width (including border) allocated to the items list.
func (e explorerModel) itemsWidth() int {
	if e.width >= 120 {
		return 42 // 40 inner + 2 border
	}
	// ~35% of width at 80 cols
	return max(27, e.width*35/100)
}

// itemSelectedCmd creates a command that fires an itemSelectedMsg.
func (e explorerModel) itemSelectedCmd() tea.Cmd {
	item := e.items.Selected()
	return func() tea.Msg {
		return itemSelectedMsg{item: item}
	}
}
