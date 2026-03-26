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
	successColor = lipgloss.AdaptiveColor{Light: "#879A39", Dark: "#879A39"} // green-600/400
	warningColor = lipgloss.AdaptiveColor{Light: "#BC5215", Dark: "#DA702C"} // orange
	dangerColor  = lipgloss.AdaptiveColor{Light: "#AF3029", Dark: "#D14D41"} // red-600/400
)

// Flexoki structural colors.
var (
	selectedBG  = lipgloss.AdaptiveColor{Light: "#E6E4D9", Dark: "#343331"} // base-100/850
	primaryText = lipgloss.AdaptiveColor{Light: "#100F0F", Dark: "#CECDC3"} // black/base-200
)

// Component styles.
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

	// Selected row (full-width background, gh-dash pattern)
	selectedRowStyle = lipgloss.NewStyle().
				Background(selectedBG).
				Bold(true)

	// Group tabs — button-like with backgrounds (higher-level navigation)
	activeGroupStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.AdaptiveColor{Light: "#FFFCF0", Dark: "#100F0F"}). // paper/black
				Background(primaryColor).                                              // cyan
				Padding(0, 2)

	inactiveGroupStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#575653", Dark: "#B7B5AC"}). // base-700/300
				Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"}). // base-150/800
				Padding(0, 2)

	// Sub-tabs — text-only (lower-level navigation within a group)
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor). // cyan
			Padding(0, 2)

	inactiveTabStyle = lipgloss.NewStyle().
				Faint(true).
				Padding(0, 2)

	// Help bar
	helpBarStyle = lipgloss.NewStyle().Foreground(mutedColor)
	versionStyle = lipgloss.NewStyle().Foreground(mutedColor).Faint(true)

	// Warning
	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true).
			Align(lipgloss.Center)

	// Panel borders — focus indicated by border color
	focusedBorderFg = primaryColor

	// General text
	mutedStyle = lipgloss.NewStyle().Foreground(mutedColor)
	boldStyle  = lipgloss.NewStyle().Bold(true).Foreground(primaryText)

	// Inline section title: ──Title──────────────────
	sectionTitleStyle = lipgloss.NewStyle().Foreground(primaryColor)
	sectionRuleStyle  = lipgloss.NewStyle().Foreground(mutedColor)

	// Text input fields — dim background tint
	inputActiveBG   = lipgloss.AdaptiveColor{Light: "#D5EFED", Dark: "#1A3836"} // dim cyan tint
	inputInactiveBG = lipgloss.AdaptiveColor{Light: "#E6E4D9", Dark: "#282726"} // dim grey tint
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

// borderedPanel renders content inside a rounded border with exact dimensions.
// Uses lipgloss Border + Width/MaxWidth + Height/MaxHeight to both pad AND clamp,
// preserving zone markers (which manual string splitting would destroy).
func borderedPanel(content string, innerW, innerH int, fg lipgloss.TerminalColor) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(fg).
		Width(innerW).
		MaxWidth(innerW + borderSize). // +border chars to get exact outer width
		Height(innerH).
		MaxHeight(innerH + borderSize). // +border chars to get exact outer height
		Render(content)
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
