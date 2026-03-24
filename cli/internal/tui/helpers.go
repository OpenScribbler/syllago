package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderScrollUp renders a scroll indicator for hidden items above.
func renderScrollUp(count int, isContentView bool) string {
	if count <= 0 {
		return ""
	}
	label := "more above"
	if isContentView {
		label = "lines above"
	}
	return helpStyle.Render(fmt.Sprintf("  (%d %s)", count, label))
}

// renderScrollDown renders a scroll indicator for hidden items below.
func renderScrollDown(count int, isContentView bool) string {
	if count <= 0 {
		return ""
	}
	label := "more below"
	if isContentView {
		label = "lines below"
	}
	return helpStyle.Render(fmt.Sprintf("  (%d %s)", count, label))
}

// cursorPrefix returns the prefix and style for a list item based on selection.
func cursorPrefix(selected bool) (string, lipgloss.Style) {
	if selected {
		return "> ", selectedItemStyle
	}
	return "  ", itemStyle
}

// padToWidth pads a string with spaces to reach the target width.
func padToWidth(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
