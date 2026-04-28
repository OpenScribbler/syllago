package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/telemetry"
)

// telemetryConsentDoneMsg is emitted when the user dismisses the modal.
// Enabled reflects the user's choice. App.Update handles persistence and
// transitions back to the normal landing screen.
type telemetryConsentDoneMsg struct {
	Enabled bool
}

// telemetryConsentModal is a full-screen blocking modal shown the first
// time the TUI launches without a recorded telemetry consent decision.
//
// It is intentionally larger than other TUI modals: this is a one-time
// disclosure with content that must be fully readable in place — the user
// should never have to follow a link to know what they are consenting to.
type telemetryConsentModal struct {
	active   bool
	focusIdx int // 0 = "No, thanks", 1 = "Yes, share usage data"

	width  int
	height int
}

func newTelemetryConsentModal() telemetryConsentModal {
	return telemetryConsentModal{focusIdx: 0}
}

// Active reports whether the modal should consume input.
func (m telemetryConsentModal) Active() bool { return m.active }

// Open activates the modal with default focus on the safe choice.
func (m *telemetryConsentModal) Open() {
	m.active = true
	m.focusIdx = 0
}

// Close deactivates the modal.
func (m *telemetryConsentModal) Close() {
	m.active = false
}

// SetSize lets the parent control modal width/height. The modal renders
// at the given width but caps internally to keep readable line lengths.
func (m *telemetryConsentModal) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Update handles keyboard + mouse while active. Returns a
// telemetryConsentDoneMsg when the user makes a choice; the App handles
// persistence (telemetry.RecordConsent) so this component stays presentation-only.
func (m telemetryConsentModal) Update(msg tea.Msg) (telemetryConsentModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "right", "l":
			m.focusIdx = 1
		case "shift+tab", "left", "h":
			m.focusIdx = 0
		case "y", "Y":
			return m.choose(true)
		case "n", "N", "esc":
			return m.choose(false)
		case "enter":
			return m.choose(m.focusIdx == 1)
		}
	case tea.MouseMsg:
		if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
			return m, nil
		}
		if zone.Get("consent-no").InBounds(msg) {
			return m.choose(false)
		}
		if zone.Get("consent-yes").InBounds(msg) {
			return m.choose(true)
		}
	}
	return m, nil
}

func (m telemetryConsentModal) choose(enabled bool) (telemetryConsentModal, tea.Cmd) {
	m.active = false
	return m, func() tea.Msg { return telemetryConsentDoneMsg{Enabled: enabled} }
}

// View renders the consent modal. Designed to dominate the screen until the
// user makes a choice — there is no useful TUI state behind it on first run.
func (m telemetryConsentModal) View() string {
	if !m.active {
		return ""
	}

	innerW := 70
	if m.width > 0 && m.width-4 < innerW {
		innerW = m.width - 4
	}
	if innerW < 40 {
		innerW = 40
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	emphasisStyle := lipgloss.NewStyle().Bold(true).Foreground(warningColor)
	bulletStyle := lipgloss.NewStyle().Foreground(primaryText)
	linkStyle := lipgloss.NewStyle().Foreground(accentColor).Underline(true)
	mutedStyle := lipgloss.NewStyle().Foreground(mutedColor)

	var lines []string
	lines = append(lines, titleStyle.Render("syllago needs your help"))
	lines = append(lines, "")

	lines = append(lines, wrapPlain(telemetry.MaintainerAppeal, innerW)...)
	lines = append(lines, "")
	lines = append(lines, emphasisStyle.Render("Telemetry is OFF by default. Nothing is sent unless you choose Yes."))
	lines = append(lines, "")

	lines = append(lines, titleStyle.Render("What gets collected (only if you opt in):"))
	for _, item := range telemetry.CollectedItems() {
		lines = append(lines, bulletStyle.Render("  • "+item))
	}
	lines = append(lines, "")

	lines = append(lines, titleStyle.Render("Never collected:"))
	for _, item := range telemetry.NeverItems() {
		lines = append(lines, bulletStyle.Render("  • "+item))
	}
	lines = append(lines, "")

	lines = append(lines, mutedStyle.Render("Read the docs: ")+linkStyle.Render(telemetry.DocsURL))
	lines = append(lines, mutedStyle.Render("Read the code: ")+linkStyle.Render(telemetry.CodeURL))
	lines = append(lines, "")
	lines = append(lines, mutedStyle.Render("Change this any time:  syllago telemetry on  /  off  /  reset"))
	lines = append(lines, "")

	lines = append(lines, renderModalButtons(m.focusIdx, innerW, "", nil,
		buttonDef{label: "No, thanks", zoneID: "consent-no", focusAt: 0},
		buttonDef{label: "Yes, share usage data", zoneID: "consent-yes", focusAt: 1},
	))

	body := strings.Join(lines, "\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Padding(1, 2).
		Width(innerW + 4).
		Render(body)

	return zone.Mark("consent-modal", box)
}

// wrapPlain word-wraps s to width columns. Mirrors telemetry.RenderDisclosure
// wrapping but produces a []string for line-based rendering inside the modal.
func wrapPlain(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var out []string
	line := words[0]
	for _, w := range words[1:] {
		if len(line)+1+len(w) > width {
			out = append(out, line)
			line = w
			continue
		}
		line += " " + w
	}
	out = append(out, line)
	return out
}
