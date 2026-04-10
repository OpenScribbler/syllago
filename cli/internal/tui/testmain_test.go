package tui

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/muesli/termenv"
)

func TestMain(m *testing.M) {
	// Deterministic output — REQUIRED for golden file stability.
	lipgloss.SetColorProfile(termenv.Ascii)
	lipgloss.SetHasDarkBackground(true)
	os.Setenv("NO_COLOR", "1")
	os.Setenv("TERM", "dumb")

	// bubblezone global manager
	zone.NewGlobal()

	// Warmup render to stabilize AdaptiveColor state.
	_ = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#000", Dark: "#fff"}).Render("warmup")

	os.Exit(m.Run())
}
