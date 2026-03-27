package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// wizardShell renders step breadcrumbs for full-screen wizards. It provides a
// bordered topbar showing the wizard title and numbered step labels, with
// completed steps clickable to navigate backwards.
type wizardShell struct {
	title  string   // e.g., "Install"
	steps  []string // step labels: ["Provider", "Location", "Method", "Review"]
	active int      // 0-based index of current step
	width  int      // terminal width
}

func newWizardShell(title string, steps []string) wizardShell {
	return wizardShell{
		title: title,
		steps: steps,
		width: 80,
	}
}

// SetActive sets the current step index.
func (s *wizardShell) SetActive(step int) {
	s.active = step
}

// SetSteps replaces the step labels (for dynamic changes like hooks/MCP showing fewer steps).
func (s *wizardShell) SetSteps(steps []string) {
	s.steps = steps
	if s.active >= len(steps) {
		s.active = len(steps) - 1
	}
}

// SetWidth updates the available width.
func (s *wizardShell) SetWidth(w int) {
	if w < 40 {
		w = 40
	}
	s.width = w
}

// View renders the wizard topbar:
//
//	╭──syllago─── Install ─────────────────────────╮
//	│  [1 Provider]  [2 Location]  [3 Method]      │
//	╰──────────────────────────────────────────────╯
func (s wizardShell) View() string {
	w := s.width
	if w < 40 {
		w = 40
	}
	innerW := w - borderSize // content width inside border chars

	topBorder := s.renderTopBorder(innerW)
	stepRow := s.renderStepRow(innerW)
	bottomBorder := mutedStyle.Render("╰" + strings.Repeat("─", innerW) + "╯")

	return lipgloss.JoinVertical(lipgloss.Left, topBorder, stepRow, bottomBorder)
}

// renderTopBorder renders ╭──syllago─── Title ──...──╮ with colored logo.
func (s wizardShell) renderTopBorder(innerW int) string {
	logo := logoStyle.Render("syl") + accentLogoStyle.Render("lago")
	logoW := lipgloss.Width(logo)

	titleRendered := lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(s.title)
	titleW := lipgloss.Width(titleRendered)

	prefix := mutedStyle.Render("╭──")
	suffix := mutedStyle.Render("──╮")

	// Layout: ╭── logo ─── title space fill ──╮
	// Content area (innerW) = ── (2) + logo + ─── (3) + title + space (1) + fill + ── (2)
	sepAfterLogo := mutedStyle.Render("───")
	spaceAfterTitle := " "

	fill := innerW - 2 - logoW - 3 - titleW - 1 - 2
	if fill < 0 {
		fill = 0
	}

	return prefix + logo + sepAfterLogo + titleRendered + spaceAfterTitle +
		mutedStyle.Render(strings.Repeat("─", fill)) + suffix
}

// renderStepRow renders the step labels with appropriate styling.
func (s wizardShell) renderStepRow(innerW int) string {
	border := mutedStyle.Render("│")
	padding := "  "

	var parts []string
	for i, label := range s.steps {
		num := itoa(i + 1)
		stepLabel := "[" + num + " " + label + "]"

		var rendered string
		switch {
		case i == s.active:
			// Active step: bold + primary
			rendered = lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render(stepLabel)
		case i < s.active:
			// Completed: underlined + primary, clickable zone
			styled := lipgloss.NewStyle().Underline(true).Foreground(primaryColor).Render(stepLabel)
			rendered = zone.Mark("wiz-step-"+itoa(i), styled)
		default:
			// Future: muted
			rendered = mutedStyle.Render(stepLabel)
		}
		parts = append(parts, rendered)
	}

	content := strings.Join(parts, "  ")
	contentW := lipgloss.Width(content)

	// If content doesn't fit, truncate labels to just numbers
	if contentW > innerW-4 { // leave room for padding+borders
		parts = parts[:0]
		for i, label := range s.steps {
			num := itoa(i + 1)
			// Try shorter labels first
			short := label
			if contentW > innerW-4 && len(label) > 3 {
				short = label[:3] + ".."
			}
			stepLabel := "[" + num + " " + short + "]"

			var rendered string
			switch {
			case i == s.active:
				rendered = lipgloss.NewStyle().Bold(true).Foreground(primaryColor).Render(stepLabel)
			case i < s.active:
				styled := lipgloss.NewStyle().Underline(true).Foreground(primaryColor).Render(stepLabel)
				rendered = zone.Mark("wiz-step-"+itoa(i), styled)
			default:
				rendered = mutedStyle.Render(stepLabel)
			}
			parts = append(parts, rendered)
		}
		content = strings.Join(parts, "  ")
	}

	// Pad to fill the row
	row := padding + content
	rowW := lipgloss.Width(row)
	if rowW < innerW {
		row += strings.Repeat(" ", innerW-rowW)
	}

	return border + lipgloss.NewStyle().MaxWidth(innerW).Render(row) + border
}

// HandleClick checks if a completed step was clicked. Returns (step index, true) if so.
// Only completed steps (index < active) have zone marks and are clickable.
func (s wizardShell) HandleClick(msg tea.MouseMsg) (int, bool) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return 0, false
	}

	for i := 0; i < s.active; i++ {
		if zone.Get("wiz-step-" + itoa(i)).InBounds(msg) {
			return i, true
		}
	}
	return 0, false
}
