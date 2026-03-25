package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// previewModel renders a scrollable text preview of a file's content.
type previewModel struct {
	lines    []string // content lines
	fileName string   // displayed in header
	offset   int      // scroll offset (first visible line)
	width    int
	height   int
	focused  bool
}

func newPreviewModel() previewModel {
	return previewModel{}
}

// SetSize updates preview dimensions.
func (p *previewModel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// LoadItem loads the primary file content for a catalog item.
func (p *previewModel) LoadItem(item *catalog.ContentItem) {
	p.offset = 0

	if item == nil {
		p.lines = nil
		p.fileName = ""
		return
	}

	primary := catalog.PrimaryFileName(item.Files, item.Type)
	if primary == "" {
		p.fileName = "(no preview)"
		p.lines = []string{"No previewable file found."}
		return
	}

	p.fileName = primary
	content, err := catalog.ReadFileContent(item.Path, primary, 500)
	if err != nil {
		p.lines = []string{"Error reading file:", err.Error()}
		return
	}

	p.lines = strings.Split(content, "\n")
}

// ScrollUp scrolls the preview up one line.
func (p *previewModel) ScrollUp() {
	if p.offset > 0 {
		p.offset--
	}
}

// ScrollDown scrolls the preview down one line.
func (p *previewModel) ScrollDown() {
	contentHeight := p.height - 1 // 1 for header
	maxOffset := max(0, len(p.lines)-contentHeight)
	if p.offset < maxOffset {
		p.offset++
	}
}

// PageUp scrolls up by one page.
func (p *previewModel) PageUp() {
	contentHeight := p.height - 1
	p.offset = max(0, p.offset-contentHeight)
}

// PageDown scrolls down by one page.
func (p *previewModel) PageDown() {
	contentHeight := p.height - 1
	maxOffset := max(0, len(p.lines)-contentHeight)
	p.offset = min(maxOffset, p.offset+contentHeight)
}

// View renders the preview pane.
func (p previewModel) View() string {
	if p.height <= 0 || p.width <= 0 {
		return ""
	}

	if len(p.lines) == 0 {
		return p.renderEmpty()
	}

	// Header line: ──filename──────
	header := renderSectionTitle(p.fileName, p.width)
	contentHeight := p.height - 1 // 1 for header

	if contentHeight <= 0 {
		return header
	}

	// Calculate scroll indicators
	linesAbove := p.offset
	lastVisible := min(p.offset+contentHeight, len(p.lines))
	linesBelow := max(0, len(p.lines)-lastVisible)
	showAbove := linesAbove > 0
	showBelow := linesBelow > 0

	// Adjust visible content to make room for indicators
	contentStart := p.offset
	contentEnd := lastVisible
	if showAbove {
		contentStart++ // skip one content line for the indicator
	}
	if showBelow && contentEnd > contentStart {
		contentEnd-- // skip one content line for the indicator
	}

	// Render visible lines with line numbers
	visibleLines := make([]string, 0, contentHeight)
	lineNumW := len(fmt.Sprintf("%d", len(p.lines))) // width of largest line number
	if lineNumW < 2 {
		lineNumW = 2
	}

	if showAbove {
		indicator := fmt.Sprintf("(%d more above)", linesAbove)
		visibleLines = append(visibleLines, mutedStyle.Render(indicator))
	}

	for i := contentStart; i < contentEnd; i++ {
		num := mutedStyle.Render(fmt.Sprintf("%*d ", lineNumW, i+1))
		numW := lipgloss.Width(num)
		lineW := p.width - numW
		line := truncateLine(p.lines[i], lineW)
		visibleLines = append(visibleLines, num+line)
	}

	if showBelow {
		indicator := fmt.Sprintf("(%d more below)", linesBelow)
		visibleLines = append(visibleLines, mutedStyle.Render(indicator))
	}

	// Pad remaining height
	for len(visibleLines) < contentHeight {
		visibleLines = append(visibleLines, strings.Repeat(" ", p.width))
	}

	return header + "\n" + strings.Join(visibleLines, "\n")
}

// renderEmpty shows a placeholder when no content is loaded.
func (p previewModel) renderEmpty() string {
	return lipgloss.NewStyle().
		Width(p.width).
		Height(p.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(mutedColor).
		Render("Select an item to preview")
}
