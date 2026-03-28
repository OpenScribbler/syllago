package tui

import (
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// checkboxBadgeStyle controls the color of the right-aligned badge on a checkbox item.
type checkboxBadgeStyle int

const (
	badgeStyleNone checkboxBadgeStyle = iota
	badgeStyleDanger
	badgeStyleWarning
	badgeStyleSuccess
	badgeStyleMuted
)

// checkboxItem represents a single item in a multi-select checkbox list.
type checkboxItem struct {
	label       string
	description string
	disabled    bool
	badge       string
	badgeStyle  checkboxBadgeStyle
}

// checkboxList is a reusable multi-select list component with keyboard navigation,
// toggling, select-all/none, and scrolling. All methods use value receivers
// (matching the riskBanner pattern) so copies are returned from Update.
type checkboxList struct {
	items    []checkboxItem
	selected []bool
	cursor   int
	offset   int
	width    int
	height   int
	focused  bool
}

// checkboxDrillInMsg is emitted when the user presses Enter on a checkbox item.
type checkboxDrillInMsg struct {
	index int
}

func newCheckboxList(items []checkboxItem) checkboxList {
	return checkboxList{
		items:    items,
		selected: make([]bool, len(items)),
		cursor:   0,
		offset:   0,
	}
}

// SetSize returns a copy with updated dimensions.
func (c checkboxList) SetSize(w, h int) checkboxList {
	c.width = w
	c.height = h
	return c
}

// SelectedIndices returns the indices where selected[i] is true.
func (c checkboxList) SelectedIndices() []int {
	var indices []int
	for i, sel := range c.selected {
		if sel {
			indices = append(indices, i)
		}
	}
	return indices
}

// Update handles key input for the checkbox list.
func (c checkboxList) Update(msg tea.KeyMsg) (checkboxList, tea.Cmd) {
	if len(c.items) == 0 {
		return c, nil
	}

	switch {
	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		if c.cursor > 0 {
			c.cursor--
			c.adjustOffset()
		}

	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		if c.cursor < len(c.items)-1 {
			c.cursor++
			c.adjustOffset()
		}

	case msg.Type == tea.KeySpace:
		if !c.items[c.cursor].disabled {
			c.selected[c.cursor] = !c.selected[c.cursor]
		}

	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'a':
		// Select all non-disabled
		for i := range c.items {
			if !c.items[i].disabled {
				c.selected[i] = true
			}
		}

	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'n':
		// Deselect all
		for i := range c.selected {
			c.selected[i] = false
		}

	case msg.Type == tea.KeyEnter:
		return c, func() tea.Msg { return checkboxDrillInMsg{index: c.cursor} }

	case msg.Type == tea.KeyPgUp:
		c.cursor -= c.visibleHeight()
		if c.cursor < 0 {
			c.cursor = 0
		}
		c.adjustOffset()

	case msg.Type == tea.KeyPgDown:
		c.cursor += c.visibleHeight()
		if c.cursor >= len(c.items) {
			c.cursor = len(c.items) - 1
		}
		c.adjustOffset()

	case msg.Type == tea.KeyHome:
		c.cursor = 0
		c.adjustOffset()

	case msg.Type == tea.KeyEnd:
		c.cursor = len(c.items) - 1
		c.adjustOffset()
	}

	return c, nil
}

// visibleHeight returns the number of visible rows (clamped to at least 1).
func (c checkboxList) visibleHeight() int {
	if c.height > 0 {
		return c.height
	}
	return len(c.items)
}

// adjustOffset ensures the cursor stays within the visible window.
func (c *checkboxList) adjustOffset() {
	vh := c.visibleHeight()
	if c.cursor < c.offset {
		c.offset = c.cursor
	}
	if c.cursor >= c.offset+vh {
		c.offset = c.cursor - vh + 1
	}
}

// View renders the visible rows of the checkbox list.
func (c checkboxList) View() string {
	if len(c.items) == 0 {
		return mutedStyle.Render("  No items")
	}

	vh := c.visibleHeight()
	end := c.offset + vh
	if end > len(c.items) {
		end = len(c.items)
	}

	var lines []string
	for i := c.offset; i < end; i++ {
		lines = append(lines, c.renderRow(i))
	}

	return strings.Join(lines, "\n")
}

// renderRow renders a single checkbox row.
func (c checkboxList) renderRow(i int) string {
	item := c.items[i]

	// Cursor indicator
	cursor := "  "
	if c.focused && i == c.cursor {
		cursor = "> "
	}

	// Checkbox glyph
	var check string
	if item.disabled {
		check = "[-]"
	} else if c.selected[i] {
		check = "[x]"
	} else {
		check = "[ ]"
	}

	// Label (sanitized)
	label := sanitizeLine(stripAnsi(item.label))

	// Build the left portion
	left := cursor + check + " " + label

	// Badge (right-aligned)
	var badge string
	if item.badge != "" {
		color := badgeColor(item.badgeStyle)
		if color != nil {
			badge = lipgloss.NewStyle().Foreground(color).Render(item.badge)
		} else {
			badge = item.badge
		}
	}

	// Apply disabled styling
	if item.disabled {
		leftStyled := mutedStyle.Render(left)
		if badge != "" {
			badgeStyled := mutedStyle.Render(item.badge)
			return c.alignRow(leftStyled, badgeStyled)
		}
		return leftStyled
	}

	// Cursor highlight
	if c.focused && i == c.cursor {
		leftStyled := lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(left)
		if badge != "" {
			return c.alignRow(leftStyled, badge)
		}
		return leftStyled
	}

	if badge != "" {
		return c.alignRow(left, badge)
	}
	return left
}

// alignRow places left content and a right-aligned badge within the available width.
func (c checkboxList) alignRow(left, badge string) string {
	leftW := lipgloss.Width(left)
	badgeW := lipgloss.Width(badge)
	w := c.width
	if w <= 0 {
		w = 80
	}
	gap := w - leftW - badgeW - 1
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + badge
}

// --- Helpers ---

// ansiRe matches ANSI escape sequences.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripAnsi removes ANSI escape sequences from untrusted content.
func stripAnsi(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// badgeColor maps a badge style to a terminal color.
func badgeColor(style checkboxBadgeStyle) lipgloss.TerminalColor {
	switch style {
	case badgeStyleDanger:
		return dangerColor
	case badgeStyleWarning:
		return warningColor
	case badgeStyleSuccess:
		return successColor
	case badgeStyleMuted:
		return mutedColor
	default:
		return nil
	}
}
