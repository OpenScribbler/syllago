package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestColorsAreAdaptive(t *testing.T) {
	colors := map[string]lipgloss.TerminalColor{
		"primaryColor":   primaryColor,
		"secondaryColor": secondaryColor,
		"mutedColor":     mutedColor,
		"successColor":   successColor,
		"dangerColor":    dangerColor,
		"warningColor":   warningColor,
	}

	for name, c := range colors {
		if _, ok := c.(lipgloss.AdaptiveColor); !ok {
			t.Errorf("%s should be AdaptiveColor, got %T", name, c)
		}
	}
}
