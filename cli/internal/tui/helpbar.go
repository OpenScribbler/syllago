package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// helpBarModel renders a context-sensitive footer with key hints and version.
type helpBarModel struct {
	width   int
	hints   []string // set by parent based on current screen/focus
	version string
}

func newHelpBar(version string) helpBarModel {
	return helpBarModel{
		version: "syllago v" + version,
		hints:   []string{"? help", "ctrl+c quit"},
	}
}

// SetSize updates the help bar width.
func (h *helpBarModel) SetSize(width int) {
	h.width = width
}

// SetHints replaces the current hint list.
func (h *helpBarModel) SetHints(hints []string) {
	h.hints = hints
}

// Height returns the rendered height (1 or 2 lines depending on wrapping).
func (h helpBarModel) Height() int {
	if len(h.hints) == 0 {
		return 1
	}
	_, overflow := h.splitHints()
	if len(overflow) > 0 {
		return 2
	}
	return 1
}

// View renders the help bar, wrapping to 2 lines if hints don't fit.
func (h helpBarModel) View() string {
	right := versionStyle.Render(h.version)
	rightW := lipgloss.Width(right)

	firstLine, secondLine := h.splitHints()

	if len(secondLine) == 0 {
		// Single line: hints left, version right
		left := h.joinHints(firstLine)
		leftW := lipgloss.Width(left)
		gap := max(0, h.width-leftW-rightW-1)
		return helpBarStyle.Render(left) + strings.Repeat(" ", gap) + right
	}

	// Two lines: first line of hints + version, second line of remaining hints
	line1Left := h.joinHints(firstLine)
	line1LeftW := lipgloss.Width(line1Left)
	gap1 := max(0, h.width-line1LeftW-rightW-1)
	line1 := helpBarStyle.Render(line1Left) + strings.Repeat(" ", gap1) + right

	line2 := helpBarStyle.Render(h.joinHints(secondLine))

	return line1 + "\n" + line2
}

// splitHints divides hints into what fits on line 1 and what overflows to line 2.
func (h helpBarModel) splitHints() (line1, line2 []string) {
	if len(h.hints) == 0 {
		return nil, nil
	}

	sep := " · "
	sepW := lipgloss.Width(mutedStyle.Render(sep))
	maxWidth := h.width - lipgloss.Width(h.version) - 4

	totalWidth := 0
	for i, hint := range h.hints {
		rendered := helpBarStyle.Render(hint)
		w := lipgloss.Width(rendered)
		needed := w
		if len(line1) > 0 {
			needed += sepW
		}
		if totalWidth+needed > maxWidth && len(line1) > 0 {
			return line1, h.hints[i:]
		}
		line1 = append(line1, hint)
		totalWidth += needed
	}
	return line1, nil
}

// joinHints renders hints joined by middle dot separators.
func (h helpBarModel) joinHints(hints []string) string {
	sep := mutedStyle.Render(" · ")
	parts := make([]string, len(hints))
	for i, hint := range hints {
		parts[i] = helpBarStyle.Render(hint)
	}
	return strings.Join(parts, sep)
}
