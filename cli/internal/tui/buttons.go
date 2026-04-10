package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// buttonDef describes a single button in a modal button row.
type buttonDef struct {
	label   string
	zoneID  string
	focusAt int
}

// renderModalButtons renders a right-aligned row of buttons with focus styling.
// focusIdx is the currently focused button index.
// dangerLabels lists button labels that get danger (red) styling when focused;
// all other focused buttons get accent (purple) styling.
// Unfocused buttons get a neutral gray background.
func renderModalButtons(focusIdx, usableW int, pad string, dangerLabels []string, buttons ...buttonDef) string {
	dangerSet := make(map[string]bool, len(dangerLabels))
	for _, l := range dangerLabels {
		dangerSet[l] = true
	}

	var parts []string
	for _, b := range buttons {
		style := lipgloss.NewStyle().Padding(0, 2)
		if focusIdx == b.focusAt {
			fg := lipgloss.AdaptiveColor{Light: "#FFFCF0", Dark: "#100F0F"}
			bg := accentColor
			if dangerSet[b.label] {
				bg = dangerColor
			}
			style = style.Bold(true).Foreground(fg).Background(bg)
		} else {
			style = style.
				Foreground(primaryText).
				Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"})
		}
		parts = append(parts, zone.Mark(b.zoneID, style.Render(b.label)))
	}

	buttonsStr := strings.Join(parts, "  ")
	buttonsW := lipgloss.Width(buttonsStr)
	buttonPad := max(0, usableW-buttonsW)
	return pad + strings.Repeat(" ", buttonPad) + buttonsStr
}
