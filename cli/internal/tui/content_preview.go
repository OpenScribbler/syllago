package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// previewModel renders full-width file content with line numbers.
// Used for single-file content types (Agents, Rules, MCP Configs, Commands).
type previewModel struct {
	content      string   // raw file content
	lines        []string // split lines for rendering
	filename     string   // displayed in the inline title
	scrollOffset int
	width        int
	height       int
}

// newPreviewModel creates a preview model with the given filename and content.
func newPreviewModel(filename string, content string) previewModel {
	m := previewModel{
		filename: filename,
	}
	m.setContentLines(content)
	return m
}

// setContent updates the displayed filename and content.
func (m *previewModel) setContent(filename, content string) {
	m.filename = filename
	m.setContentLines(content)
	m.scrollOffset = 0
	m.clampScroll()
}

// setContentLines splits raw content into lines and stores both.
func (m *previewModel) setContentLines(content string) {
	m.content = content
	if content == "" {
		m.lines = nil
	} else {
		m.lines = strings.Split(content, "\n")
	}
}

// visibleLines returns how many content lines fit in the viewport.
// Subtracts 1 for the title row, and 1 each for scroll indicators if present.
func (m previewModel) visibleLines() int {
	avail := m.height - 1 // title row
	if avail < 0 {
		return 0
	}
	// Reserve space for scroll indicators when content overflows.
	if m.scrollOffset > 0 {
		avail-- // "lines above" indicator
	}
	if m.scrollOffset+avail < len(m.lines) {
		avail-- // "lines below" indicator
	}
	if avail < 0 {
		return 0
	}
	return avail
}

// clampScroll ensures scrollOffset stays within valid bounds.
func (m *previewModel) clampScroll() {
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	maxOffset := len(m.lines) - m.visibleLines()
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
}

// Update handles keyboard and mouse input for scrolling.
func (m previewModel) Update(msg tea.Msg) (previewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Up):
			m.scrollOffset--
			m.clampScroll()
		case key.Matches(msg, keys.Down):
			m.scrollOffset++
			m.clampScroll()
		case key.Matches(msg, keys.PageUp):
			m.scrollOffset -= m.visibleLines()
			m.clampScroll()
		case key.Matches(msg, keys.PageDown):
			m.scrollOffset += m.visibleLines()
			m.clampScroll()
		case key.Matches(msg, keys.Home):
			m.scrollOffset = 0
		case key.Matches(msg, keys.End):
			maxOffset := len(m.lines) - m.visibleLines()
			if maxOffset < 0 {
				maxOffset = 0
			}
			m.scrollOffset = maxOffset
		}
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.scrollOffset--
			m.clampScroll()
		case tea.MouseButtonWheelDown:
			m.scrollOffset++
			m.clampScroll()
		}
	}
	return m, nil
}

// View renders the preview panel with inline title, line numbers, and content.
func (m previewModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	var b strings.Builder

	// Title row
	title := "Preview: " + m.filename
	b.WriteString(inlineTitle(title, m.width, primaryColor))
	b.WriteString("\n")

	if len(m.lines) == 0 {
		return b.String()
	}

	// Determine visible window
	vis := m.visibleLines()
	end := m.scrollOffset + vis
	if end > len(m.lines) {
		end = len(m.lines)
	}

	// Content width available for line text (after "NNN  " prefix)
	// Line number: 3 chars right-aligned + 2 spaces = 5 chars prefix
	const lineNumWidth = 3
	const gapWidth = 2
	prefixWidth := lineNumWidth + gapWidth
	contentWidth := m.width - prefixWidth
	if contentWidth < 0 {
		contentWidth = 0
	}

	// Scroll-above indicator
	if m.scrollOffset > 0 {
		indicator := fmt.Sprintf("(%d lines above)", m.scrollOffset)
		b.WriteString(helpStyle.Render(indicator))
		b.WriteString("\n")
	}

	// Render visible lines with line numbers
	for i := m.scrollOffset; i < end; i++ {
		lineNum := lineNumStyle.Render(fmt.Sprintf("%*d", lineNumWidth, i+1))
		lineText := m.lines[i]
		if contentWidth > 0 {
			lineText = truncateStr(lineText, contentWidth)
		}
		b.WriteString(lineNum)
		b.WriteString(strings.Repeat(" ", gapWidth))
		b.WriteString(lineText)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	// Scroll-below indicator
	linesBelow := len(m.lines) - end
	if linesBelow > 0 {
		indicator := fmt.Sprintf("(%d lines below)", linesBelow)
		b.WriteString("\n")
		b.WriteString(helpStyle.Render(indicator))
	}

	return b.String()
}
