package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Colors — adaptive for light/dark terminal themes.
var (
	primaryColor = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#6EE7B7"} // Mint (content)
	accentColor  = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"} // Viola (collections)
	mutedColor   = lipgloss.AdaptiveColor{Light: "#57534E", Dark: "#A8A29E"} // Stone
	successColor = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"} // Green
	dangerColor  = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FCA5A5"} // Red
	warningColor = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FCD34D"} // Amber

	// Panel and layout colors
	borderColor      = lipgloss.AdaptiveColor{Light: "#D4D4D8", Dark: "#3F3F46"}
	selectedBgColor  = lipgloss.AdaptiveColor{Light: "#D1FAE5", Dark: "#1A3A2A"}
	modalBgColor     = lipgloss.AdaptiveColor{Light: "#F4F4F5", Dark: "#27272A"}
	modalBorderColor = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"}
)

// Named styles — all styles defined here, never inline.
var (
	// Title and text styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#374151", Dark: "#E5E7EB"})

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	countStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Selection / items
	itemStyle = lipgloss.NewStyle()

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Background(selectedBgColor).
				Bold(true)

	// Status indicators
	installedStyle = lipgloss.NewStyle().
			Foreground(successColor)

	notInstalledStyle = lipgloss.NewStyle().
				Foreground(mutedColor)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	// Messages
	successMsgStyle = lipgloss.NewStyle().
			Foreground(successColor)

	errorMsgStyle = lipgloss.NewStyle().
			Foreground(dangerColor)

	// Top bar
	topBarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor)

	logoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	activeDropdownStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	inactiveDropdownStyle = lipgloss.NewStyle().
				Foreground(mutedColor)

	collectionDropdownStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true)

	// Dropdown menu
	dropdownBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(borderColor)

	dropdownItemStyle = lipgloss.NewStyle()

	dropdownSelectedStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true)

	// Metadata bar
	metadataStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(borderColor)

	metaNameStyle = lipgloss.NewStyle().
			Bold(true)

	metaActionStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Content zone
	splitBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(borderColor)

	lineNumStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Cards
	cardNormalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mutedColor).
			Padding(0, 1)

	cardSelectedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentColor).
				Padding(0, 1).
				Bold(true)

	// Modals
	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(modalBorderColor).
			Background(modalBgColor).
			Padding(1, 2)

	buttonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(accentColor).
			Padding(0, 2)

	buttonDisabledStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Background(lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#3F3F46"}).
				Padding(0, 2)

	// Action button styles (semantic)
	actionBtnAddStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Padding(0, 1)

	actionBtnNewStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Padding(0, 1)
)

// modalWidth is the fixed width for all modals.
const modalWidth = 56

// inlineTitle renders a section title with horizontal rules.
// Format: ──{Title}──{fill remaining width with ─}
// The title text uses the given style; rules use mutedColor.
func inlineTitle(title string, width int, titleColor lipgloss.AdaptiveColor) string {
	prefix := "──"
	suffix := "──"
	titleRendered := lipgloss.NewStyle().Foreground(titleColor).Render(title)
	ruleStyle := lipgloss.NewStyle().Foreground(mutedColor)

	// Calculate fill width: total - prefix(2) - title len - suffix(2)
	titleLen := lipgloss.Width(title)
	fillLen := width - 2 - titleLen - 2
	if fillLen < 0 {
		fillLen = 0
	}

	return ruleStyle.Render(prefix) + titleRendered + ruleStyle.Render(suffix+strings.Repeat("─", fillLen))
}

// truncateStr truncates text to maxWidth, adding "..." if needed.
func truncateStr(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= maxWidth {
		return text
	}
	if maxWidth <= 3 {
		return text[:maxWidth]
	}
	// Simple byte-based truncation; sufficient for ASCII content
	if len(text) > maxWidth-3 {
		return text[:maxWidth-3] + "..."
	}
	return text
}
