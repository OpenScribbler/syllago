package tui

import (
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

// itemSelectedMsg is sent when the selected item changes.
type itemSelectedMsg struct {
	item *catalog.ContentItem
}

// explorerModel is the main content area: items list (left) + preview (right).
// Rendered inside a unified bordered frame with metadata panel at top.
type explorerModel struct {
	items     itemsModel
	preview   previewModel
	focus     explorerPane
	width     int
	height    int
	providers []provider.Provider
	repoRoot  string

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

	innerW := max(0, width-borderSize)
	// Reserve space for metadata (3 lines) + separator (1) + top/bottom borders (2)
	paneH := max(3, height-borderSize-metaBarLines-1)

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

// SetItems replaces the item list and reloads preview.
func (e *explorerModel) SetItems(items []catalog.ContentItem, mixed bool) {
	e.items.SetItems(items, mixed)
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
	}

	return e, nil
}

// updateMouse handles mouse clicks on items and scroll wheel.
func (e explorerModel) updateMouse(msg tea.MouseMsg) (explorerModel, tea.Cmd) {
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		// Rename button click
		if zone.Get("meta-rename").InBounds(msg) {
			return e, func() tea.Msg { return libraryRenameMsg{} }
		}

		// Click on a specific item row — select it
		for i := range e.items.items {
			if zone.Get("item-" + itoa(i)).InBounds(msg) {
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
		if e.stacked && e.items.Selected() != nil {
			e.setFocus(panePreview)
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

// setFocus switches focus to the given pane.
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

	if e.stacked {
		return e.viewStacked()
	}
	return e.viewSideBySide()
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

// renderMetadataContent returns 3 lines of metadata for the selected item.
func (e explorerModel) renderMetadataContent(width int) string {
	item := e.items.Selected()
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
