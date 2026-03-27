package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// riskDrillInMsg is emitted when the user presses Enter on a risk item.
type riskDrillInMsg struct {
	risk catalog.RiskIndicator
}

// riskBanner renders a navigable list of risk indicators inside a bordered box.
// Reused by Install Wizard (Phase B), Add Wizard (Phase D), and Loadout Apply (Phase E).
type riskBanner struct {
	risks  []catalog.RiskIndicator
	cursor int // -1 = no focus, 0+ = focused item
	width  int
}

func newRiskBanner(risks []catalog.RiskIndicator, width int) riskBanner {
	cursor := -1
	if len(risks) > 0 {
		cursor = 0
	}
	return riskBanner{risks: risks, cursor: cursor, width: width}
}

// IsEmpty returns true when there are no risk indicators to display.
func (b riskBanner) IsEmpty() bool {
	return len(b.risks) == 0
}

// Update handles keyboard navigation within the risk list.
// Returns an updated banner and an optional command (riskDrillInMsg on Enter).
func (b riskBanner) Update(msg tea.KeyMsg) (riskBanner, tea.Cmd) {
	if b.IsEmpty() {
		return b, nil
	}

	switch {
	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		if b.cursor > 0 {
			b.cursor--
		}

	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		if b.cursor < len(b.risks)-1 {
			b.cursor++
		}

	case msg.Type == tea.KeyEnter:
		if b.cursor >= 0 && b.cursor < len(b.risks) {
			r := b.risks[b.cursor]
			return b, func() tea.Msg { return riskDrillInMsg{risk: r} }
		}
	}

	return b, nil
}

// View renders the risk banner as a bordered box with severity icons.
// Returns "" when empty (zero height contribution).
func (b riskBanner) View() string {
	if b.IsEmpty() {
		return ""
	}

	// Use the width as-is — caller is responsible for passing the correct usable width.
	boxW := b.width
	if boxW < 30 {
		boxW = 30
	}
	contentW := boxW - 2 // subtract border left+right

	titleLine := " " + lipgloss.NewStyle().Bold(true).Foreground(b.borderColor()).Render("Risk Indicators")

	var lines []string
	lines = append(lines, titleLine)
	for i, r := range b.risks {
		line := b.renderRiskLine(r, i, contentW)
		lines = append(lines, zone.Mark(fmt.Sprintf("risk-%d", i), line))
	}

	content := strings.Join(lines, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(b.borderColor()).
		Width(contentW).
		MaxWidth(boxW).
		Render(content)
}

// ViewInline renders risk items as plain lines (no surrounding border).
// The caller is responsible for wrapping these lines in a bordered frame.
// When focused is false, the cursor highlight is dimmed.
func (b riskBanner) ViewInline(width int, focused bool) string {
	if b.IsEmpty() {
		return ""
	}

	var lines []string
	for i, r := range b.risks {
		if !focused && b.cursor == i {
			// Dim the cursor when not focused: show as bold text without background
			line := b.renderRiskLineDim(r, i, width)
			lines = append(lines, zone.Mark(fmt.Sprintf("risk-%d", i), line))
		} else {
			line := b.renderRiskLine(r, i, width)
			lines = append(lines, zone.Mark(fmt.Sprintf("risk-%d", i), line))
		}
	}
	return strings.Join(lines, "\n")
}

// renderRiskLineDim renders a risk item with a dimmed cursor (bold, no background).
func (b riskBanner) renderRiskLineDim(r catalog.RiskIndicator, idx, maxW int) string {
	var icon string
	if r.Level == catalog.RiskHigh {
		icon = lipgloss.NewStyle().Foreground(dangerColor).Render("!!")
	} else {
		icon = lipgloss.NewStyle().Foreground(warningColor).Render("! ")
	}

	prefixW := 4
	availW := maxW - prefixW
	if availW < 10 {
		availW = 10
	}

	text := r.Label
	if r.Description != "" {
		full := r.Label + " — " + r.Description
		if lipgloss.Width(full) > availW {
			maxDesc := availW - lipgloss.Width(r.Label) - lipgloss.Width(" — ") - lipgloss.Width("...")
			if maxDesc > 0 {
				desc := r.Description
				for lipgloss.Width(desc) > maxDesc {
					desc = desc[:len(desc)-1]
				}
				text = r.Label + " — " + desc + "..."
			} else {
				text = r.Label
			}
		} else {
			text = full
		}
	}

	styledText := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryText).
		MaxWidth(availW).
		Render(text)

	return " " + icon + " " + styledText
}

// borderColor returns dangerColor (red) if any risk is RiskHigh, otherwise warningColor (orange).
func (b riskBanner) borderColor() lipgloss.TerminalColor {
	for _, r := range b.risks {
		if r.Level == catalog.RiskHigh {
			return dangerColor
		}
	}
	return warningColor
}

// renderRiskLine renders a single risk item with severity icon and selection highlight.
func (b riskBanner) renderRiskLine(r catalog.RiskIndicator, idx, maxW int) string {
	// Severity icon: "!!" for high (red), "! " for medium (orange).
	var icon string
	if r.Level == catalog.RiskHigh {
		icon = lipgloss.NewStyle().Foreground(dangerColor).Render("!!")
	} else {
		icon = lipgloss.NewStyle().Foreground(warningColor).Render("! ")
	}

	// Build the text portion: " Label — Description"
	// Icon "!!" is 2 rendered chars, then " " prefix = 3 chars before label.
	// We also have a leading " " pad for the whole line = 4 chars total prefix.
	prefixW := 4 // " " + icon(2) + " "
	availW := maxW - prefixW
	if availW < 10 {
		availW = 10
	}

	text := r.Label
	if r.Description != "" {
		full := r.Label + " — " + r.Description
		if lipgloss.Width(full) > availW {
			// Truncate description to fit.
			maxDesc := availW - lipgloss.Width(r.Label) - lipgloss.Width(" — ") - lipgloss.Width("...")
			if maxDesc > 0 {
				desc := r.Description
				for lipgloss.Width(desc) > maxDesc {
					desc = desc[:len(desc)-1]
				}
				text = r.Label + " — " + desc + "..."
			} else {
				// Not enough room even for label + separator; just show label.
				text = r.Label
			}
		} else {
			text = full
		}
	}

	// Apply selection highlight or default style.
	var styledText string
	if b.cursor == idx {
		styledText = lipgloss.NewStyle().
			Bold(true).
			Background(accentColor).
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFCF0", Dark: "#100F0F"}).
			MaxWidth(availW).
			Render(text)
	} else {
		styledText = lipgloss.NewStyle().
			Foreground(primaryText).
			MaxWidth(availW).
			Render(text)
	}

	return " " + icon + " " + styledText
}
