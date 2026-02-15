package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED") // purple
	secondaryColor = lipgloss.Color("#06B6D4") // cyan
	mutedColor     = lipgloss.Color("#6B7280") // gray
	successColor   = lipgloss.Color("#10B981") // green
	dangerColor    = lipgloss.Color("#EF4444") // red
	warningColor   = lipgloss.Color("#F59E0B") // amber

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
