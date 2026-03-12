package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

// splitViewItem represents an entry in the left pane of a split view.
type splitViewItem struct {
	Label    string // display text
	Path     string // absolute path for loading preview
	IsDir    bool   // directories are shown but not previewable
	Indent   int    // nesting level (2 spaces per level)
	Disabled bool   // non-selectable items (e.g. type group headers)
}

// splitViewPane tracks which pane is focused within the split view.
type splitViewPane int

const (
	paneList    splitViewPane = iota // left pane (file tree / item list)
	panePreview                     // right pane (content preview)
)

// splitViewCursorMsg is sent when the cursor moves to a new item.
// The parent should load preview content and call SetPreview().
type splitViewCursorMsg struct {
	index int
	item  splitViewItem
}

// splitViewModel is a reusable two-pane layout component.
// Left pane shows a navigable list, right pane shows a preview of the selected item.
// Falls back to single-pane mode when width is too narrow.
type splitViewModel struct {
	items        []splitViewItem
	cursor       int
	scrollOffset int
	// Right pane
	previewContent string
	previewScroll  int
	// Layout
	focusedPane splitViewPane
	width       int
	height      int
	// Single-pane fallback state
	showingPreview bool // true when Enter opens full-width preview in collapsed mode
	// Zone prefix for unique zone IDs (e.g. "sv-files" or "sv-contents")
	zonePrefix string
}

const splitViewMinWidth = 70 // content width threshold for split vs single-pane

// newSplitView creates a split view with the given items.
func newSplitView(items []splitViewItem, zonePrefix string) splitViewModel {
	return splitViewModel{
		items:      items,
		zonePrefix: zonePrefix,
	}
}

// SetPreview updates the right pane content.
func (m *splitViewModel) SetPreview(content string) {
	m.previewContent = content
	m.previewScroll = 0
}

// SetItems replaces the item list and resets cursor.
func (m *splitViewModel) SetItems(items []splitViewItem) {
	m.items = items
	m.cursor = 0
	m.scrollOffset = 0
	m.previewContent = ""
	m.previewScroll = 0
	m.showingPreview = false
	m.focusedPane = paneList
}

// CursorItem returns the currently selected item, or nil if empty.
func (m splitViewModel) CursorItem() *splitViewItem {
	if m.cursor >= 0 && m.cursor < len(m.items) {
		return &m.items[m.cursor]
	}
	return nil
}

// IsSplit returns true when the layout is in split (two-pane) mode.
func (m splitViewModel) IsSplit() bool {
	return m.width >= splitViewMinWidth
}

// FocusedPane returns which pane is currently focused.
func (m splitViewModel) FocusedPane() splitViewPane {
	return m.focusedPane
}

// leftWidth returns the width of the left pane.
func (m splitViewModel) leftWidth() int {
	if !m.IsSplit() {
		return m.width
	}
	// Adaptive ratio: 40% at 70-90, 35% at 100+
	ratio := 0.40
	if m.width >= 100 {
		ratio = 0.35
	}
	w := int(float64(m.width) * ratio)
	if w < 25 {
		w = 25
	}
	return w
}

// rightWidth returns the width of the right pane (split mode only).
func (m splitViewModel) rightWidth() int {
	return m.width - m.leftWidth() - 1 // -1 for separator
}

// visibleListRows returns how many list rows fit on screen.
func (m splitViewModel) visibleListRows() int {
	rows := m.height - 2 // reserve space for title + bottom margin
	if rows < 1 {
		rows = len(m.items)
	}
	return rows
}

// visiblePreviewRows returns how many preview lines fit on screen.
func (m splitViewModel) visiblePreviewRows() int {
	rows := m.height - 2 // reserve space for title + bottom margin
	if rows < 1 {
		rows = 20
	}
	return rows
}

// adjustScroll keeps the cursor visible within the viewport.
func (m *splitViewModel) adjustScroll() {
	visible := m.visibleListRows()
	if visible <= 0 {
		return
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+visible {
		m.scrollOffset = m.cursor - visible + 1
	}
}

// nextSelectableItem finds the next non-disabled item in the given direction.
func (m splitViewModel) nextSelectableItem(from, dir int) int {
	for i := from + dir; i >= 0 && i < len(m.items); i += dir {
		if !m.items[i].Disabled {
			return i
		}
	}
	return from // no valid item found, stay put
}

// Update handles keyboard and mouse events for the split view.
// Returns the updated model and a tea.Cmd. If the cursor changed,
// the cmd will produce a splitViewCursorMsg.
func (m splitViewModel) Update(msg tea.Msg) (splitViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	return m, nil
}

func (m splitViewModel) handleKey(msg tea.KeyMsg) (splitViewModel, tea.Cmd) {
	// Single-pane preview mode: scroll or exit
	if !m.IsSplit() && m.showingPreview {
		switch {
		case key.Matches(msg, keys.Back):
			m.showingPreview = false
			return m, nil
		case key.Matches(msg, keys.Up):
			if m.previewScroll > 0 {
				m.previewScroll--
			}
			return m, nil
		case key.Matches(msg, keys.Down):
			m.previewScroll++
			return m, nil
		case key.Matches(msg, keys.PageUp):
			page := m.visiblePreviewRows() - 2
			if page < 1 {
				page = 10
			}
			m.previewScroll -= page
			if m.previewScroll < 0 {
				m.previewScroll = 0
			}
			return m, nil
		case key.Matches(msg, keys.PageDown):
			page := m.visiblePreviewRows() - 2
			if page < 1 {
				page = 10
			}
			m.previewScroll += page
			return m, nil
		}
		return m, nil // swallow all other keys in single-pane preview
	}

	// Split mode: preview pane focused
	if m.IsSplit() && m.focusedPane == panePreview {
		switch {
		case key.Matches(msg, keys.Up):
			if m.previewScroll > 0 {
				m.previewScroll--
			}
			return m, nil
		case key.Matches(msg, keys.Down):
			m.previewScroll++
			return m, nil
		case key.Matches(msg, keys.PageUp):
			page := m.visiblePreviewRows() - 2
			if page < 1 {
				page = 10
			}
			m.previewScroll -= page
			if m.previewScroll < 0 {
				m.previewScroll = 0
			}
			return m, nil
		case key.Matches(msg, keys.PageDown):
			page := m.visiblePreviewRows() - 2
			if page < 1 {
				page = 10
			}
			m.previewScroll += page
			return m, nil
		case key.Matches(msg, keys.Left):
			m.focusedPane = paneList
			return m, nil
		}
		return m, nil
	}

	// List pane focused (both split and single-pane modes)
	switch {
	case key.Matches(msg, keys.Up):
		newCursor := m.nextSelectableItem(m.cursor, -1)
		if newCursor != m.cursor {
			m.cursor = newCursor
			m.adjustScroll()
			return m, m.cursorCmd()
		}
		return m, nil

	case key.Matches(msg, keys.Down):
		newCursor := m.nextSelectableItem(m.cursor, 1)
		if newCursor != m.cursor {
			m.cursor = newCursor
			m.adjustScroll()
			return m, m.cursorCmd()
		}
		return m, nil

	case key.Matches(msg, keys.Home):
		newCursor := m.nextSelectableItem(-1, 1) // from before start, going forward
		if newCursor != m.cursor && newCursor >= 0 {
			m.cursor = newCursor
			m.adjustScroll()
			return m, m.cursorCmd()
		}
		return m, nil

	case key.Matches(msg, keys.End):
		newCursor := m.nextSelectableItem(len(m.items), -1) // from after end, going backward
		if newCursor != m.cursor && newCursor < len(m.items) {
			m.cursor = newCursor
			m.adjustScroll()
			return m, m.cursorCmd()
		}
		return m, nil

	case key.Matches(msg, keys.Right):
		if m.IsSplit() {
			m.focusedPane = panePreview
			return m, nil
		}
		return m, nil

	case key.Matches(msg, keys.Enter):
		if !m.IsSplit() && len(m.items) > 0 && !m.items[m.cursor].Disabled && !m.items[m.cursor].IsDir {
			// Single-pane: open full-width preview
			m.showingPreview = true
			m.previewScroll = 0
			return m, nil
		}
		return m, nil
	}

	// 'l' for vim-style right
	if msg.String() == "l" && m.IsSplit() {
		m.focusedPane = panePreview
		return m, nil
	}

	return m, nil
}

func (m splitViewModel) handleMouse(msg tea.MouseMsg) (splitViewModel, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.focusedPane == panePreview || (!m.IsSplit() && m.showingPreview) {
			if m.previewScroll > 0 {
				m.previewScroll--
			}
		} else {
			newCursor := m.nextSelectableItem(m.cursor, -1)
			if newCursor != m.cursor {
				m.cursor = newCursor
				m.adjustScroll()
				return m, m.cursorCmd()
			}
		}
	case tea.MouseButtonWheelDown:
		if m.focusedPane == panePreview || (!m.IsSplit() && m.showingPreview) {
			m.previewScroll++
		} else {
			newCursor := m.nextSelectableItem(m.cursor, 1)
			if newCursor != m.cursor {
				m.cursor = newCursor
				m.adjustScroll()
				return m, m.cursorCmd()
			}
		}
	case tea.MouseButtonLeft:
		if msg.Action == tea.MouseActionRelease {
			return m.handleClick(msg)
		}
	}
	return m, nil
}

func (m splitViewModel) handleClick(msg tea.MouseMsg) (splitViewModel, tea.Cmd) {
	// Check left-pane item clicks
	visible := m.visibleListRows()
	end := m.scrollOffset + visible
	if end > len(m.items) {
		end = len(m.items)
	}
	for j := 0; j < end-m.scrollOffset; j++ {
		zoneID := fmt.Sprintf("%s-item-%d", m.zonePrefix, j)
		if zone.Get(zoneID).InBounds(msg) {
			idx := m.scrollOffset + j
			if idx < len(m.items) && !m.items[idx].Disabled {
				m.cursor = idx
				m.focusedPane = paneList
				return m, m.cursorCmd()
			}
			break
		}
	}
	return m, nil
}

// cursorCmd returns a command that emits a splitViewCursorMsg for the current cursor.
func (m splitViewModel) cursorCmd() tea.Cmd {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	item := m.items[m.cursor]
	idx := m.cursor
	return func() tea.Msg {
		return splitViewCursorMsg{index: idx, item: item}
	}
}

// View renders the split view.
func (m splitViewModel) View() string {
	if len(m.items) == 0 {
		return helpStyle.Render("No files in this item.") + "\n"
	}

	// Single-pane: showing preview
	if !m.IsSplit() && m.showingPreview {
		return m.renderSinglePanePreview()
	}

	// Single-pane: file list only
	if !m.IsSplit() {
		return m.renderList(m.width)
	}

	// Split mode: left + separator + right
	leftW := m.leftWidth()
	rightW := m.rightWidth()

	left := m.renderList(leftW)
	right := m.renderPreview(rightW)

	// Pad each pane to full height for proper alignment
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")

	// Determine actual display height
	displayHeight := m.height
	if displayHeight < 1 {
		displayHeight = max(len(leftLines), len(rightLines))
	}

	// Pad to equal length
	for len(leftLines) < displayHeight {
		leftLines = append(leftLines, strings.Repeat(" ", leftW))
	}
	for len(rightLines) < displayHeight {
		rightLines = append(rightLines, strings.Repeat(" ", rightW))
	}

	// Join line by line with separator
	sep := helpStyle.Render("│")
	var combined strings.Builder
	for i := 0; i < displayHeight; i++ {
		l := leftLines[i]
		r := ""
		if i < len(rightLines) {
			r = rightLines[i]
		}
		combined.WriteString(l)
		combined.WriteString(sep)
		combined.WriteString(r)
		if i < displayHeight-1 {
			combined.WriteString("\n")
		}
	}

	return combined.String()
}

// renderList renders the left pane (file tree / item list).
func (m splitViewModel) renderList(width int) string {
	var s strings.Builder

	// Title
	titleStyle := labelStyle
	if m.IsSplit() && m.focusedPane != paneList {
		titleStyle = helpStyle
	}
	s.WriteString(titleStyle.Render("Files"))
	s.WriteString("\n")

	visible := m.visibleListRows()
	end := m.scrollOffset + visible
	if end > len(m.items) {
		end = len(m.items)
	}

	if m.scrollOffset > 0 {
		s.WriteString("  " + renderScrollUp(m.scrollOffset, false) + "\n")
	}

	maxLabelW := width - 6 // 2 leading + 2 cursor prefix + 2 indent margin
	if maxLabelW < 10 {
		maxLabelW = 10
	}

	for i := m.scrollOffset; i < end; i++ {
		item := m.items[i]
		indent := strings.Repeat("  ", item.Indent)

		if item.Disabled {
			line := fmt.Sprintf("  %s%s", indent, helpStyle.Render(item.Label))
			s.WriteString(line + "\n")
			continue
		}

		prefix, style := cursorPrefix(i == m.cursor)
		label := item.Label
		if item.IsDir {
			label += "/"
		}
		if len(label) > maxLabelW {
			label = label[:maxLabelW-3] + "..."
		}
		line := fmt.Sprintf("  %s%s%s", indent, prefix, style.Render(label))
		zoneID := fmt.Sprintf("%s-item-%d", m.zonePrefix, i-m.scrollOffset)
		s.WriteString(zone.Mark(zoneID, line) + "\n")
	}

	if end < len(m.items) {
		s.WriteString("  " + renderScrollDown(len(m.items)-end, false) + "\n")
	}

	return s.String()
}

// renderPreview renders the right pane (content preview).
func (m splitViewModel) renderPreview(width int) string {
	var s strings.Builder

	// Title
	titleStyle := labelStyle
	if m.focusedPane != panePreview {
		titleStyle = helpStyle
	}
	s.WriteString(titleStyle.Render("Preview"))
	s.WriteString("\n")

	if m.previewContent == "" {
		s.WriteString(helpStyle.Render("  (no preview)") + "\n")
		return s.String()
	}

	lines := strings.Split(m.previewContent, "\n")
	visible := m.visiblePreviewRows()

	// Clamp scroll
	maxOffset := len(lines) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := m.previewScroll
	if offset > maxOffset {
		offset = maxOffset
	}

	end := offset + visible
	if end > len(lines) {
		end = len(lines)
	}

	if offset > 0 {
		s.WriteString(renderScrollUp(offset, true) + "\n")
	}

	lineNumW := len(fmt.Sprintf("%d", len(lines)))
	if lineNumW < 4 {
		lineNumW = 4
	}
	maxContentW := width - lineNumW - 2 // line num + space + margin
	if maxContentW < 10 {
		maxContentW = 10
	}

	for i := offset; i < end; i++ {
		lineNum := helpStyle.Render(fmt.Sprintf("%*d ", lineNumW, i+1))
		lineContent := lines[i]
		if len(lineContent) > maxContentW {
			lineContent = lineContent[:maxContentW]
		}
		s.WriteString(lineNum + valueStyle.Render(StripControlChars(lineContent)) + "\n")
	}

	if end < len(lines) {
		s.WriteString(renderScrollDown(len(lines)-end, true) + "\n")
	}

	return s.String()
}

// renderSinglePanePreview renders a full-width preview with a back link (single-pane mode).
func (m splitViewModel) renderSinglePanePreview() string {
	var s strings.Builder

	// Back link and filename
	item := m.CursorItem()
	fileName := ""
	if item != nil {
		fileName = item.Label
	}
	backLink := zone.Mark(fmt.Sprintf("%s-back", m.zonePrefix), backLinkStyle.Render("<- Back to files"))
	s.WriteString(backLink + "  " + labelStyle.Render(fileName) + "\n\n")

	if m.previewContent == "" {
		s.WriteString(helpStyle.Render("  (no content)") + "\n")
		return s.String()
	}

	lines := strings.Split(m.previewContent, "\n")
	visible := m.visiblePreviewRows()

	maxOffset := len(lines) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := m.previewScroll
	if offset > maxOffset {
		offset = maxOffset
	}

	end := offset + visible
	if end > len(lines) {
		end = len(lines)
	}

	if offset > 0 {
		s.WriteString(renderScrollUp(offset, true) + "\n")
	}

	lineNumW := len(fmt.Sprintf("%d", len(lines)))
	if lineNumW < 4 {
		lineNumW = 4
	}

	for i := offset; i < end; i++ {
		lineNum := helpStyle.Render(fmt.Sprintf("%*d ", lineNumW, i+1))
		s.WriteString(lineNum + valueStyle.Render(StripControlChars(lines[i])) + "\n")
	}

	if end < len(lines) {
		s.WriteString(renderScrollDown(len(lines)-end, true) + "\n")
	}

	return s.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
