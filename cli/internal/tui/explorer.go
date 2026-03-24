package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// explorerModel combines the items list (left pane) with a content zone (right pane).
// This is the main layout for content types (Skills, Agents, MCP, Rules, Hooks, Commands).
type explorerModel struct {
	items       itemsModel
	preview     previewModel
	contentType catalog.ContentType
	useSplit    bool // true for Skills/Hooks (multi-file), false for others
	focusRight  bool // true = content zone focused, false = items focused
	width       int
	height      int
}

// newExplorerModel creates an explorer layout for the given content type and items.
//
// Why this approach: the explorer owns layout (width splitting, focus routing) while
// delegating rendering to the sub-models (items, preview). This keeps each model
// focused on one job.
//
// Key trade-off: we determine useSplit by content type rather than per-item inspection.
// Skills and Hooks have multi-file directory structures that benefit from a split
// (file tree + preview) layout. Other types are single-file and get a full-width preview.
// For now, since splitModel isn't implemented yet, split types fall back to preview.
func newExplorerModel(items []catalog.ContentItem, ct catalog.ContentType, width, height int) explorerModel {
	useSplit := ct == catalog.Skills || ct == catalog.Hooks

	m := explorerModel{
		items:       newItemsModel(items, ct),
		contentType: ct,
		useSplit:    useSplit,
		width:       width,
		height:      height,
	}

	// Size the sub-models.
	itemsW, contentW := m.splitWidths()
	m.items.width = itemsW
	m.items.height = height
	m.preview.width = contentW
	m.preview.height = height

	// Load preview for the first item if available.
	if len(items) > 0 {
		m.loadPreviewForItem(&items[0])
	}

	return m
}

// splitWidths calculates the width allocation for items (left) and content (right) panes.
// Items list gets ~25% at width >= 100, ~30% at narrower widths. Minimum items width: 20.
// The 1-char border separator is subtracted from the total before splitting.
//
// Gotcha: the border character takes 1 column, so available width is (total - 1).
func (m *explorerModel) splitWidths() (itemsW, contentW int) {
	available := m.width - 1 // 1 char for the vertical border separator
	if available < 20 {
		// Terminal too narrow — give everything to items.
		return available, 0
	}

	var ratio float64
	if m.width >= 100 {
		ratio = 0.25
	} else {
		ratio = 0.30
	}

	itemsW = int(float64(available) * ratio)
	if itemsW < 20 {
		itemsW = 20
	}
	contentW = available - itemsW
	if contentW < 0 {
		contentW = 0
	}
	return itemsW, contentW
}

// loadPreviewForItem loads the primary file content for the given item into the preview model.
func (m *explorerModel) loadPreviewForItem(item *catalog.ContentItem) {
	if item == nil || len(item.Files) == 0 {
		m.preview.setContent("(no files)", "")
		return
	}

	filename := catalog.PrimaryFileName(item.Files, item.Type)
	if filename == "" {
		filename = item.Files[0]
	}

	content, err := catalog.ReadFileContent(item.Path, filename, 0)
	if err != nil {
		// Try reading directly if Path is the file itself (e.g., single-file items).
		data, readErr := os.ReadFile(filepath.Join(item.Path, filename))
		if readErr != nil {
			m.preview.setContent(filename, fmt.Sprintf("(error reading file: %v)", err))
			return
		}
		content = string(data)
	}

	m.preview.setContent(filename, content)
}

// Update handles keyboard input for the explorer layout.
//
// How it works:
// - h/l switch focus between the items list and the content zone
// - When items are focused, up/down navigate the list; selection changes update the preview
// - When the content zone is focused, keys are forwarded to the preview sub-model for scrolling
func (m explorerModel) Update(msg tea.Msg) (explorerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Left):
			m.focusRight = false
			return m, nil
		case key.Matches(msg, keys.Right):
			if m.preview.width > 0 {
				m.focusRight = true
			}
			return m, nil
		}

		if m.focusRight {
			// Forward to preview for scrolling.
			var cmd tea.Cmd
			m.preview, cmd = m.preview.Update(msg)
			return m, cmd
		}

		// Items pane is focused.
		var cmd tea.Cmd
		m.items, cmd = m.items.Update(msg)
		return m, cmd

	case itemSelectedMsg:
		// Cursor moved to a new item — update the content zone.
		m.loadPreviewForItem(&msg.item)
		return m, nil
	}

	return m, nil
}

// View renders the explorer layout: items list on left, content zone on right,
// separated by a vertical border.
func (m explorerModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	leftPane := m.items.View()
	_, contentW := m.splitWidths()

	if contentW <= 0 {
		// Too narrow for a content zone — show items only.
		return leftPane
	}

	rightPane := m.preview.View()

	// Build the border column — a vertical line of │ characters for each row.
	// The border uses a brighter color when focused on the respective side,
	// providing a subtle visual indicator of which pane is active.
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	if m.focusRight {
		borderStyle = borderStyle.Foreground(primaryColor)
	}

	// Join the panes line-by-line with the border separator.
	leftLines := strings.Split(leftPane, "\n")
	rightLines := strings.Split(rightPane, "\n")

	// Pad to equal height.
	for len(leftLines) < m.height {
		leftLines = append(leftLines, "")
	}
	for len(rightLines) < m.height {
		rightLines = append(rightLines, "")
	}

	itemsW, _ := m.splitWidths()

	var b strings.Builder
	for i := 0; i < m.height; i++ {
		left := ""
		if i < len(leftLines) {
			left = leftLines[i]
		}
		right := ""
		if i < len(rightLines) {
			right = rightLines[i]
		}

		// Pad left pane to consistent width.
		leftW := lipgloss.Width(left)
		if leftW < itemsW {
			left += strings.Repeat(" ", itemsW-leftW)
		}

		b.WriteString(left)
		b.WriteString(borderStyle.Render("│"))
		b.WriteString(right)

		if i < m.height-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// resize updates the explorer dimensions and recalculates sub-model sizes.
func (m *explorerModel) resize(width, height int) {
	m.width = width
	m.height = height
	itemsW, contentW := m.splitWidths()
	m.items.width = itemsW
	m.items.height = height
	m.preview.width = contentW
	m.preview.height = height
}
