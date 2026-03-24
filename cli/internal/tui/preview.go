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
	maxOffset := max(0, len(p.lines)-p.height)
	if p.offset < maxOffset {
		p.offset++
	}
}

// PageUp scrolls up by one page.
func (p *previewModel) PageUp() {
	p.offset = max(0, p.offset-p.height)
}

// PageDown scrolls down by one page.
func (p *previewModel) PageDown() {
	maxOffset := max(0, len(p.lines)-p.height)
	p.offset = min(maxOffset, p.offset+p.height)
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

	// Render visible lines with line numbers
	visibleLines := make([]string, 0, contentHeight)
	lineNumW := len(fmt.Sprintf("%d", len(p.lines))) // width of largest line number
	if lineNumW < 2 {
		lineNumW = 2
	}

	for i := p.offset; i < p.offset+contentHeight && i < len(p.lines); i++ {
		num := mutedStyle.Render(fmt.Sprintf("%*d ", lineNumW, i+1))
		numW := lipgloss.Width(num)
		lineW := p.width - numW
		line := truncateLine(p.lines[i], lineW)
		visibleLines = append(visibleLines, num+line)
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
