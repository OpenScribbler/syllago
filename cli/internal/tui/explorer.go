package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
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
type explorerModel struct {
	items   itemsModel
	preview previewModel
	focus   explorerPane
	width   int
	height  int

	// Responsive: at narrow widths, show stacked layout
	stacked bool // true when width < 80
}

func newExplorerModel(items []catalog.ContentItem, mixed bool) explorerModel {
	e := explorerModel{
		items:   newItemsModel(items, mixed),
		preview: newPreviewModel(),
		focus:   paneItems,
	}
	e.items.focused = true
	// Auto-select first item
	if len(items) > 0 {
		e.preview.LoadItem(&items[0])
	}
	return e
}

// SetSize recalculates child dimensions.
func (e *explorerModel) SetSize(width, height int) {
	e.width = width
	e.height = height
	e.stacked = width < 80

	if e.stacked {
		e.items.SetSize(width, height)
		e.preview.SetSize(width, height)
	} else {
		itemsW := e.itemsWidth()
		previewW := width - itemsW - 1 // 1 for the vertical separator
		e.items.SetSize(itemsW, height)
		e.preview.SetSize(previewW, height)
	}
}

// SetItems replaces the item list and reloads preview.
func (e *explorerModel) SetItems(items []catalog.ContentItem, mixed bool) {
	e.items.SetItems(items, mixed)
	e.focus = paneItems
	e.items.focused = true
	e.preview.focused = false
	if len(items) > 0 {
		e.preview.LoadItem(&items[0])
	} else {
		e.preview.LoadItem(nil)
	}
}

// Update handles key input based on current focus.
func (e explorerModel) Update(msg tea.Msg) (explorerModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return e, nil
	}

	switch keyMsg.String() {
	case "tab":
		e.toggleFocus()
		return e, nil

	case keySearch:
		// TODO: Phase 3.5 — open search input overlay
		return e, nil
	}

	switch e.focus {
	case paneItems:
		return e.updateItems(keyMsg)
	case panePreview:
		return e.updatePreview(keyMsg)
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
			e.focus = panePreview
			e.items.focused = false
			e.preview.focused = true
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
			// In stacked mode, Esc goes back to items
			e.focus = paneItems
			e.items.focused = true
			e.preview.focused = false
		}
	}
	return e, nil
}

// toggleFocus switches between items and preview panes.
func (e *explorerModel) toggleFocus() {
	switch e.focus {
	case paneItems:
		e.focus = panePreview
		e.items.focused = false
		e.preview.focused = true
	case panePreview:
		e.focus = paneItems
		e.items.focused = true
		e.preview.focused = false
	}
}

// View renders the explorer layout.
func (e explorerModel) View() string {
	if e.width <= 0 || e.height <= 0 {
		return ""
	}

	if e.stacked {
		return e.viewStacked()
	}
	return e.viewSideBySide()
}

// viewSideBySide renders items list | separator | preview.
func (e explorerModel) viewSideBySide() string {
	left := e.items.View()
	right := e.preview.View()

	// Vertical separator
	sep := strings.Repeat("│\n", e.height)
	if len(sep) > 0 {
		sep = sep[:len(sep)-1] // remove trailing newline
	}
	sep = mutedStyle.Render(sep)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right)
}

// viewStacked renders either items or preview based on focus.
func (e explorerModel) viewStacked() string {
	if e.focus == panePreview {
		return e.preview.View()
	}
	return e.items.View()
}

// itemsWidth returns the width allocated to the items list.
func (e explorerModel) itemsWidth() int {
	if e.width >= 120 {
		return 40
	}
	// ~35% of width at 80 cols gives room for names + type badges
	return max(25, e.width*35/100)
}

// itemSelectedCmd creates a command that fires an itemSelectedMsg.
func (e explorerModel) itemSelectedCmd() tea.Cmd {
	item := e.items.Selected()
	return func() tea.Msg {
		return itemSelectedMsg{item: item}
	}
}
