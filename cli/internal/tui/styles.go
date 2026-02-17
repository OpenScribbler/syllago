package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors — adaptive for light/dark terminal themes.
	// Light = color on dark backgrounds, Dark = color on light backgrounds.
	primaryColor = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"}
	secondaryColor = lipgloss.AdaptiveColor{Light: "#06B6D4", Dark: "#22D3EE"}
	mutedColor     = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	successColor   = lipgloss.AdaptiveColor{Light: "#10B981", Dark: "#34D399"}
	dangerColor    = lipgloss.AdaptiveColor{Light: "#EF4444", Dark: "#F87171"}
	warningColor   = lipgloss.AdaptiveColor{Light: "#F59E0B", Dark: "#FBBF24"}

	// Title bar
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	// Help bar at bottom
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// List item styles — color only, no padding (layout handled in View)
	itemStyle = lipgloss.NewStyle()

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(secondaryColor).
				Background(lipgloss.AdaptiveColor{
				Light: "#1E293B", // dark blue-gray for dark terminals
				Dark:  "#E2E8F0", // light gray for light terminals
			}).
			Bold(true)

	// Status indicators
	installedStyle = lipgloss.NewStyle().
			Foreground(successColor)

	notInstalledStyle = lipgloss.NewStyle().
				Foreground(mutedColor)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	// Detail view
	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#374151", Dark: "#E5E7EB"})

	// Messages
	successMsgStyle = lipgloss.NewStyle().
			Foreground(successColor)

	errorMsgStyle = lipgloss.NewStyle().
			Foreground(dangerColor)

	// Category count badge
	countStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Search
	searchPromptStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	// Update notification
	updateBannerStyle = lipgloss.NewStyle().
				Foreground(warningColor)

	versionStyle = lipgloss.NewStyle().
			Foreground(mutedColor)
)
