package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// BreadcrumbSegment represents one piece of a breadcrumb trail.
// If ZoneID is non-empty, the segment is wrapped in zone.Mark and rendered as clickable (helpStyle).
// If ZoneID is empty, the segment is rendered as the current location (titleStyle, not clickable).
type BreadcrumbSegment struct {
	Label  string
	ZoneID string // empty = final segment (titleStyle), non-empty = clickable (helpStyle)
}

// renderBreadcrumb renders a clickable breadcrumb trail with " > " separators.
// The last segment with an empty ZoneID is rendered in titleStyle (current location).
// All other segments are rendered in helpStyle and wrapped in zone.Mark for click handling.
//
// Usage:
//
//	renderBreadcrumb(
//	    BreadcrumbSegment{"Home", "crumb-home"},
//	    BreadcrumbSegment{"Skills", ""},          // final = titleStyle
//	)
func renderBreadcrumb(segments ...BreadcrumbSegment) string {
	arrow := helpStyle.Render(" > ")
	var parts []string
	for _, seg := range segments {
		if seg.ZoneID != "" {
			parts = append(parts, zone.Mark(seg.ZoneID, helpStyle.Render(seg.Label)))
		} else {
			parts = append(parts, titleStyle.Render(seg.Label))
		}
	}
	return strings.Join(parts, arrow)
}

// renderStatusMsg renders a transient status message using success or error styling.
// Returns empty string if msg is empty. Prefixes with "Done: " or "Error: "
// for consistent user feedback across all screens.
func renderStatusMsg(msg string, isErr bool) string {
	if msg == "" {
		return ""
	}
	if isErr {
		return errorMsgStyle.Render("Error: " + msg)
	}
	return successMsgStyle.Render("Done: " + msg)
}

// cursorPrefix returns the cursor prefix string and appropriate style for a list item.
// Selected items get "> " prefix with selectedItemStyle.
// Unselected items get "  " prefix with itemStyle.
func cursorPrefix(selected bool) (string, lipgloss.Style) {
	if selected {
		return "> ", selectedItemStyle
	}
	return "  ", itemStyle
}

// renderScrollUp returns a scroll indicator for items above the viewport.
// For list views (isContentView=false), uses "more above".
// For content views (isContentView=true), uses "lines above".
func renderScrollUp(count int, isContentView bool) string {
	if count <= 0 {
		return ""
	}
	word := "more"
	if isContentView {
		word = "lines"
	}
	return helpStyle.Render(fmt.Sprintf("(%d %s above)", count, word))
}

// renderScrollDown returns a scroll indicator for items below the viewport.
// For list views (isContentView=false), uses "more below".
// For content views (isContentView=true), uses "lines below".
func renderScrollDown(count int, isContentView bool) string {
	if count <= 0 {
		return ""
	}
	word := "more"
	if isContentView {
		word = "lines"
	}
	return helpStyle.Render(fmt.Sprintf("(%d %s below)", count, word))
}

// cardScrollRange calculates which card rows to render given viewport constraints.
// Returns (firstRow, rowCount, scrollOffset) where firstRow is the first visible
// row index and rowCount is the number of visible rows.
// cardRowHeight is the estimated lines per card row (including borders and gap).
func cardScrollRange(cursor, totalCards, cols, availableHeight, cardRowHeight, scrollOffset int) (firstRow, visibleRows, newOffset int) {
	totalRows := (totalCards + cols - 1) / cols
	if cardRowHeight < 1 {
		cardRowHeight = 6
	}
	visibleRows = availableHeight / cardRowHeight
	if visibleRows < 1 {
		visibleRows = 1
	}
	if visibleRows >= totalRows {
		return 0, totalRows, 0
	}

	cursorRow := cursor / cols

	// Auto-scroll to keep cursor visible
	newOffset = scrollOffset
	if cursorRow < newOffset {
		newOffset = cursorRow
	}
	if cursorRow >= newOffset+visibleRows {
		newOffset = cursorRow - visibleRows + 1
	}
	if newOffset+visibleRows > totalRows {
		newOffset = totalRows - visibleRows
	}
	if newOffset < 0 {
		newOffset = 0
	}

	return newOffset, visibleRows, newOffset
}

// renderDescriptionBox renders a separator-bounded context box for the currently
// highlighted item. Fixed height prevents layout jitter when switching between items.
// maxLines controls the box height (3 for normal terminals, 1-2 for small).
// width controls the separator line width.
func renderDescriptionBox(text string, width int, maxLines int) string {
	if text == "" || maxLines <= 0 {
		return ""
	}
	var s string
	s += "\n " + helpStyle.Render(strings.Repeat("\u2500", width)) + "\n"
	lines := strings.Split(text, "\n")
	for i := 0; i < maxLines; i++ {
		if i < len(lines) {
			s += " " + helpStyle.Render(lines[i]) + "\n"
		} else {
			s += "\n"
		}
	}
	s += " " + helpStyle.Render(strings.Repeat("\u2500", width)) + "\n"
	return s
}
