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

// View renders the help bar as a single line.
func (h helpBarModel) View() string {
	left := h.renderHints()
	right := versionStyle.Render(h.version)

	rightW := lipgloss.Width(right)
	leftW := lipgloss.Width(left)
	gap := max(0, h.width-leftW-rightW-1)

	return helpBarStyle.Render(left) + strings.Repeat(" ", gap) + right
}

// renderHints joins hints with a separator, dropping low-priority hints
// that don't fit within the available width.
func (h helpBarModel) renderHints() string {
	if len(h.hints) == 0 {
		return ""
	}

	sep := mutedStyle.Render(" · ")
	maxWidth := h.width - lipgloss.Width(h.version) - 4

	var parts []string
	totalWidth := 0
	for _, hint := range h.hints {
		rendered := helpBarStyle.Render(hint)
		w := lipgloss.Width(rendered)
		if totalWidth+w+3 > maxWidth && len(parts) > 0 {
			break
		}
		parts = append(parts, rendered)
		totalWidth += w + 3
	}
	return strings.Join(parts, sep)
}
