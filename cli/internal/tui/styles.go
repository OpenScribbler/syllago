package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Syllago brand colors — used ONLY for the logo. Do not use elsewhere.
var (
	logoMint  = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#6EE7B7"}
	logoViola = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"}
)

// Flexoki semantic colors — adaptive for light/dark terminals.
// Light uses -600 values, dark uses -400 values.
// https://stephango.com/flexoki
var (
	primaryColor = lipgloss.AdaptiveColor{Light: "#24837B", Dark: "#3AA99F"} // cyan
	accentColor  = lipgloss.AdaptiveColor{Light: "#5E409D", Dark: "#8B7EC8"} // purple
	mutedColor   = lipgloss.AdaptiveColor{Light: "#6F6E69", Dark: "#878580"} // base-600/500
	successColor = lipgloss.AdaptiveColor{Light: "#66800B", Dark: "#879A39"} // green
	dangerColor  = lipgloss.AdaptiveColor{Light: "#AF3029", Dark: "#D14D41"} // red
	warningColor = lipgloss.AdaptiveColor{Light: "#BC5215", Dark: "#DA702C"} // orange
)

// Flexoki structural colors.
var (
	borderColor = lipgloss.AdaptiveColor{Light: "#CECDC3", Dark: "#343331"} // base-200/850
	selectedBG  = lipgloss.AdaptiveColor{Light: "#E6E4D9", Dark: "#343331"} // base-100/850
	modalBG     = lipgloss.AdaptiveColor{Light: "#F2F0E5", Dark: "#282726"} // base-50/900
	primaryText = lipgloss.AdaptiveColor{Light: "#100F0F", Dark: "#CECDC3"} // black/base-200
)

// Component styles — defined upfront for all phases.
var (
	// Logo — uses syllago brand colors, not Flexoki
	logoStyle       = lipgloss.NewStyle().Bold(true).Foreground(logoMint)
	accentLogoStyle = lipgloss.NewStyle().Bold(true).Foreground(logoViola)

	// Buttons (background-color blocks, huh/superfile pattern)
	activeButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#FFFCF0", Dark: "#100F0F"}). // paper/black
				Background(accentColor).
				Padding(0, 2).
				MarginRight(1)

	inactiveButtonStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Background(lipgloss.AdaptiveColor{Light: "#E6E4D9", Dark: "#343331"}). // base-100/850
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

	// Group tabs — button-like with backgrounds (higher-level navigation)
	activeGroupStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#FFFCF0", Dark: "#100F0F"}). // paper/black
				Background(primaryColor).                                              // cyan
				Padding(0, 2)

	inactiveGroupStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Background(lipgloss.AdaptiveColor{Light: "#E6E4D9", Dark: "#343331"}). // base-100/850
				Padding(0, 2)

	// Sub-tabs — text-only (lower-level navigation within a group)
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor). // cyan
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Faint(true).
				Padding(0, 2)

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

// itoa converts int to string without importing strconv.
func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// renderSectionTitle renders a divider line: ──Title────────────
func renderSectionTitle(title string, width int) string {
	prefix := sectionRuleStyle.Render("──")
	label := sectionTitleStyle.Render(title)
	used := 2 + lipgloss.Width(title) // "──" + title
	remaining := max(0, width-used-1)
	suffix := sectionRuleStyle.Render("─" + strings.Repeat("─", remaining))
	return prefix + label + suffix
}
