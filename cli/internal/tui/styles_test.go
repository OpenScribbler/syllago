package tui

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
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
		"primaryColor": primaryColor,
		"accentColor":  accentColor,
		"mutedColor":   mutedColor,
		"successColor": successColor,
		"dangerColor":  dangerColor,
		"warningColor": warningColor,
	}

	for name, c := range colors {
		if _, ok := c.(lipgloss.AdaptiveColor); !ok {
			t.Errorf("%s should be AdaptiveColor, got %T", name, c)
		}
	}
}

func TestBubblezoneImportable(t *testing.T) {
	// Verify bubblezone is available as a dependency.
	// The real integration test is go build ./... succeeding with the import in main.go.
	// This compile-time check uses the zone package to confirm it's in go.mod.
	_ = zone.NewGlobal
	t.Log("bubblezone package is importable")
}

func TestPanelColorsAreAdaptive(t *testing.T) {
	colors := map[string]lipgloss.TerminalColor{
		"borderColor":      borderColor,
		"selectedBgColor":  selectedBgColor,
		"modalBgColor":     modalBgColor,
		"modalBorderColor": modalBorderColor,
	}
	for name, c := range colors {
		if _, ok := c.(lipgloss.AdaptiveColor); !ok {
			t.Errorf("%s should be AdaptiveColor, got %T", name, c)
		}
	}
}

// contrastRatio calculates the WCAG contrast ratio between two hex colors (#RRGGBB).
// Returns the ratio (e.g. 4.5 for 4.5:1).
func contrastRatio(hex1, hex2 string) float64 {
	l1 := relativeLuminance(hex1)
	l2 := relativeLuminance(hex2)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func relativeLuminance(hex string) float64 {
	hex = strings.TrimPrefix(hex, "#")
	r, _ := strconv.ParseInt(hex[0:2], 16, 64)
	g, _ := strconv.ParseInt(hex[2:4], 16, 64)
	b, _ := strconv.ParseInt(hex[4:6], 16, 64)
	toLinear := func(c int64) float64 {
		s := float64(c) / 255.0
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*toLinear(r) + 0.7152*toLinear(g) + 0.0722*toLinear(b)
}

// TestColorsPassWCAGAA verifies all Nesco palette colors achieve >= 4.5:1
// contrast ratio against their respective terminal background assumptions.
// Light colors are tested against white (#FFFFFF); dark colors against near-black (#18181B).
func TestColorsPassWCAGAA(t *testing.T) {
	const minRatio = 4.5
	lightBg := "#FFFFFF"
	darkBg := "#18181B"

	checks := []struct {
		name     string
		fg       string // foreground hex
		bg       string // background hex
		minRatio float64
	}{
		// Semantic colors (Light = on light terminal, Dark = on dark terminal)
		{"primaryColor light", "#047857", lightBg, minRatio},
		{"primaryColor dark", "#6EE7B7", darkBg, minRatio},
		{"accentColor light", "#6D28D9", lightBg, minRatio},
		{"accentColor dark", "#C4B5FD", darkBg, minRatio},
		{"mutedColor light", "#57534E", lightBg, minRatio},
		{"mutedColor dark", "#A8A29E", darkBg, minRatio},
		{"successColor light", "#15803D", lightBg, minRatio},
		{"successColor dark", "#4ADE80", darkBg, minRatio},
		{"dangerColor light", "#B91C1C", lightBg, minRatio},
		{"dangerColor dark", "#FCA5A5", darkBg, minRatio},
		{"warningColor light", "#B45309", lightBg, minRatio},
		{"warningColor dark", "#FCD34D", darkBg, minRatio},
		// Selected item: foreground against selectedBgColor
		{"selected mint on light bg", "#047857", "#D1FAE5", minRatio},
		{"selected mint on dark bg", "#6EE7B7", "#1A3A2A", minRatio},
	}

	for _, c := range checks {
		ratio := contrastRatio(c.fg, c.bg)
		if ratio < c.minRatio {
			t.Errorf("%s: contrast %.2f:1 < %.1f:1 (fg=%s bg=%s)", c.name, ratio, c.minRatio, c.fg, c.bg)
		}
	}
}
