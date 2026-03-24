package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// helpBarModel renders the footer help bar with context-sensitive hints
// on the left and "syllago vX.X.X" pinned to the right.
type helpBarModel struct {
	version string
	width   int
}

// helpHint represents a single key/action pair for the help bar.
type helpHint struct {
	Key    string
	Action string
}

// View renders the help bar. hints are displayed left-to-right, dropping
// lower-priority hints (from the end) when they don't fit.
func (m helpBarModel) View(hints []helpHint) string {
	ruleStyle := lipgloss.NewStyle().Foreground(mutedColor)
	rule := ruleStyle.Render(strings.Repeat("─", m.width))

	versionStr := helpStyle.Render("syllago " + m.version)
	versionWidth := lipgloss.Width(versionStr)

	// Build hint string, dropping from the end if too wide
	separator := helpStyle.Render(" * ")
	sepWidth := lipgloss.Width(separator)
	availWidth := m.width - versionWidth - 4 // padding

	var parts []string
	totalWidth := 0
	for _, h := range hints {
		rendered := helpStyle.Render(h.Key + " " + h.Action)
		w := lipgloss.Width(rendered)
		needed := w
		if len(parts) > 0 {
			needed += sepWidth
		}
		if totalWidth+needed > availWidth {
			break
		}
		parts = append(parts, rendered)
		totalWidth += needed
	}

	hintsStr := strings.Join(parts, separator)

	// Fill gap between hints and version
	gap := m.width - lipgloss.Width(hintsStr) - versionWidth
	if gap < 1 {
		gap = 1
	}

	return rule + "\n" + hintsStr + strings.Repeat(" ", gap) + versionStr
}

// explorerHints returns the default hint set for the Explorer layout.
func explorerHints() []helpHint {
	return []helpHint{
		{"j/k", "navigate"},
		{"h/l", "pane"},
		{"/", "search"},
		{"i", "install"},
		{"?", "help"},
	}
}

// galleryHints returns the default hint set for the Gallery Grid layout.
func galleryHints() []helpHint {
	return []helpHint{
		{"arrows", "navigate"},
		{"enter", "select"},
		{"tab", "grid/contents"},
		{"?", "help"},
	}
}
