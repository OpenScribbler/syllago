package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestSelectedItemHasBackground(t *testing.T) {
	bg := selectedItemStyle.GetBackground()
	if bg == nil {
		t.Fatal("selectedItemStyle should have a background color set")
	}
	// Should be AdaptiveColor for theme support
	if _, ok := bg.(lipgloss.AdaptiveColor); !ok {
		t.Errorf("selectedItemStyle background should be AdaptiveColor, got %T", bg)
	}
}

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
