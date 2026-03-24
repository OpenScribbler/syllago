package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Semantic colors — adaptive for light/dark terminals.
var (
	primaryColor = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#6EE7B7"} // mint
	accentColor  = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"} // viola
	mutedColor   = lipgloss.AdaptiveColor{Light: "#57534E", Dark: "#A8A29E"} // stone
	successColor = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"} // green
	dangerColor  = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FCA5A5"} // red
	warningColor = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FCD34D"} // amber
)

// Structural colors.
var (
	borderColor = lipgloss.AdaptiveColor{Light: "#D4D4D8", Dark: "#3F3F46"}
	selectedBG  = lipgloss.AdaptiveColor{Light: "#D1FAE5", Dark: "#1A3A2A"}
	modalBG     = lipgloss.AdaptiveColor{Light: "#F4F4F5", Dark: "#27272A"}
	primaryText = lipgloss.AdaptiveColor{Light: "#1C1917", Dark: "#FAFAF9"}
)

// Component styles — defined upfront for all phases.
var (
	// Logo
	logoStyle       = lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	accentLogoStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)

	// Buttons (background-color blocks, huh/superfile pattern)
	activeButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#000000"}).
				Background(accentColor).
				Padding(0, 2).
				MarginRight(1)

	inactiveButtonStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Background(lipgloss.AdaptiveColor{Light: "#E4E4E7", Dark: "#3F3F46"}).
				Padding(0, 2).
				MarginRight(1)

	// Selected row (full-width background, gh-dash pattern)
	selectedRowStyle = lipgloss.NewStyle().
				Background(selectedBG).
				Bold(true)

	// Panel borders (border-color only, superfile pattern)
	focusedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentColor)

	unfocusedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(borderColor)

	// Tabs (bold+bg active, faint inactive, gh-dash pattern)
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Background(selectedBG).
			Foreground(primaryText).
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Faint(true).
				Padding(0, 2)

	// Dropdown
	dropdownBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentColor)

	dropdownItemStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				PaddingRight(2)

	dropdownSelectedStyle = lipgloss.NewStyle().
				Background(selectedBG).
				Bold(true).
				PaddingLeft(2).
				PaddingRight(2)

	// Modal
	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentColor).
			Background(modalBG).
			Padding(1, 2).
			Width(56)

	// Toast
	successToastStyle = lipgloss.NewStyle().Foreground(successColor)
	warningToastStyle = lipgloss.NewStyle().Foreground(warningColor)
	errorToastStyle   = lipgloss.NewStyle().Foreground(dangerColor).Bold(true)

	// Help bar
	helpBarStyle = lipgloss.NewStyle().Foreground(mutedColor)
	helpKeyStyle = lipgloss.NewStyle().Foreground(primaryText).Bold(true)
	versionStyle = lipgloss.NewStyle().Foreground(mutedColor).Faint(true)

	// Warning
	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true).
			Align(lipgloss.Center)

	// General text
	mutedStyle   = lipgloss.NewStyle().Foreground(mutedColor)
	primaryStyle = lipgloss.NewStyle().Foreground(primaryColor)
	boldStyle    = lipgloss.NewStyle().Bold(true).Foreground(primaryText)

	// Inline section title: ──Title──────────────────
	sectionTitleStyle = lipgloss.NewStyle().Foreground(primaryColor)
	sectionRuleStyle  = lipgloss.NewStyle().Foreground(mutedColor)
)

// renderSectionTitle renders a divider line: ──Title────────────
func renderSectionTitle(title string, width int) string {
	prefix := sectionRuleStyle.Render("──")
	label := sectionTitleStyle.Render(title)
	used := 2 + lipgloss.Width(title) // "──" + title
	remaining := max(0, width-used-1)
	suffix := sectionRuleStyle.Render("─" + strings.Repeat("─", remaining))
	return prefix + label + suffix
}
