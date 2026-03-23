package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors — adaptive for light/dark terminal themes.
	// Light = color on light terminal backgrounds, Dark = color on dark terminal backgrounds.
	primaryColor = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#6EE7B7"} // Mint
	accentColor  = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"} // Viola
	mutedColor   = lipgloss.AdaptiveColor{Light: "#57534E", Dark: "#A8A29E"} // Stone
	successColor = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"} // Green
	dangerColor  = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FCA5A5"} // Red
	warningColor = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FCD34D"} // Amber

	// Panel and layout colors
	borderColor      = lipgloss.AdaptiveColor{Light: "#D4D4D8", Dark: "#3F3F46"}
	selectedBgColor  = lipgloss.AdaptiveColor{Light: "#D1FAE5", Dark: "#1A3A2A"}
	modalBgColor     = lipgloss.AdaptiveColor{Light: "#F4F4F5", Dark: "#27272A"}
	modalBorderColor = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"} // same as accent

	// Title bar
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	// ASCII art accent (SYL highlight in landing page art)
	artAccentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor)

	// Help bar at bottom
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// List item styles — color only, no padding (layout handled in View)
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

	// Built-in content badge (accent/purple to distinguish from LOCAL/registry)
	builtinStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	// Global content badge (amber — distinct from project and registry)
	globalStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	// Example content badge (dim purple, distinct from builtinStyle)
	exampleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#9D7ACC", Dark: "#A78BFA"})

	// Table header (bold muted — distinct from help text)
	tableHeaderStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Bold(true)

	// Search
	searchPromptStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	// Compatibility matrix (hooks list view)
	compatFullStyle     = lipgloss.NewStyle().Foreground(successColor)
	compatDegradedStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#CA8A04", Dark: "#FDE68A"})
	compatBrokenStyle   = lipgloss.NewStyle().Foreground(warningColor)
	compatNoneStyle     = lipgloss.NewStyle().Foreground(dangerColor)

	versionStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Sidebar panel
	sidebarBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(borderColor).
				BorderRight(true)

	// Action buttons (Install tab)
	buttonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1E1B2E"}).
			Background(lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"}).
			Padding(0, 1)

	buttonDisabledStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Background(lipgloss.AdaptiveColor{Light: "#E4E4E7", Dark: "#3F3F46"}).
				Padding(0, 1)

	// Footer bar
	footerStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(borderColor).
			BorderTop(true).
			Foreground(mutedColor)

	// Detail tabs — single-line with background color for active
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor).
			Background(selectedBgColor).
			Padding(0, 1)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Padding(0, 1)

	// Clickable back link
	backLinkStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Card grid styles (registries, library, loadouts)
	cardNormalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mutedColor).
			Padding(0, 1)

	cardSelectedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentColor).
				Padding(0, 1).
				Bold(true)

	// Action buttons (page-level, below breadcrumb)
	// Format: [hotkey] Label — chip-style with semantic background colors.
	// actionBtnAddStyle — constructive action (install, add, create)
	actionBtnAddStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#052E16"}).
				Background(lipgloss.AdaptiveColor{Light: "#16A34A", Dark: "#4ADE80"}).
				Padding(0, 1)

	// actionBtnRemoveStyle — destructive action (remove, delete)
	actionBtnRemoveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#450A0A"}).
				Background(lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#FCA5A5"}).
				Padding(0, 1)

	// actionBtnUninstallStyle — destructive action (uninstall)
	actionBtnUninstallStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#431407"}).
				Background(lipgloss.AdaptiveColor{Light: "#C2410C", Dark: "#FDBA74"}).
				Padding(0, 1)

	// actionBtnSyncStyle — data synchronization action (sync registries)
	actionBtnSyncStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#2E1065"}).
				Background(lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#C4B5FD"}).
				Padding(0, 1)

	// actionBtnDefaultStyle — neutral action (copy, save, env, share)
	actionBtnDefaultStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#374151", Dark: "#D1D5DB"}).
				Background(lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"}).
				Padding(0, 1)
)
