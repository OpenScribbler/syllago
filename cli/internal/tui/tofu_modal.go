package tui

// TOFU (trust-on-first-use) approval modal. Surfaces when a MOAT sync
// observes a wire signing identity that the registry has not yet pinned —
// the user must approve before the orchestrator re-runs sync with
// acceptTOFU=true and persists the profile.
//
// Why a dedicated modal instead of reusing confirmModal: TOFU shows
// structured profile data (issuer, subject, manifest URL) the user must
// see before deciding. Squeezing that through confirmModal's free-form
// `body string` would lose the field labels and the safe-default
// (focus on Reject) the spec calls for.

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// tofuResultMsg reports the user's decision back to App. accepted=true
// triggers a re-run of MOAT sync with acceptTOFU=true (which pins the
// profile); accepted=false dismisses the modal and surfaces a toast.
type tofuResultMsg struct {
	name        string
	accepted    bool
	manifestURL string
}

// tofuFocus identifies the focusable element. Two buttons; default is
// Reject (focusReject) so an accidental Enter does not silently extend
// trust to a registry the user has not actually evaluated.
type tofuFocus int

const (
	tofuFocusReject tofuFocus = iota
	tofuFocusAccept
)

type tofuModal struct {
	active      bool
	name        string
	manifestURL string
	profile     config.SigningProfile
	focusIdx    tofuFocus
	width       int
	height      int
}

func newTOFUModal() tofuModal {
	return tofuModal{}
}

// Open activates the modal with profile details to display. Default focus
// is Reject — accepting trust requires deliberate action.
func (m *tofuModal) Open(name, manifestURL string, profile config.SigningProfile) {
	m.active = true
	m.name = name
	m.manifestURL = manifestURL
	m.profile = profile
	m.focusIdx = tofuFocusReject
}

// Close clears state.
func (m *tofuModal) Close() {
	m.active = false
	m.name = ""
	m.manifestURL = ""
	m.profile = config.SigningProfile{}
	m.focusIdx = tofuFocusReject
}

func (m tofuModal) result(accepted bool) (tofuModal, tea.Cmd) {
	res := tofuResultMsg{
		name:        m.name,
		accepted:    accepted,
		manifestURL: m.manifestURL,
	}
	m.Close()
	return m, func() tea.Msg { return res }
}

func (m tofuModal) Update(msg tea.Msg) (tofuModal, tea.Cmd) {
	if !m.active {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKey(msg)
	case tea.MouseMsg:
		return m.updateMouse(msg)
	}
	return m, nil
}

func (m tofuModal) updateKey(msg tea.KeyMsg) (tofuModal, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		return m.result(false)
	case msg.Type == tea.KeyEnter:
		return m.result(m.focusIdx == tofuFocusAccept)
	case msg.Type == tea.KeyTab,
		msg.Type == tea.KeyRight,
		msg.Type == tea.KeyShiftTab,
		msg.Type == tea.KeyLeft:
		if m.focusIdx == tofuFocusReject {
			m.focusIdx = tofuFocusAccept
		} else {
			m.focusIdx = tofuFocusReject
		}
	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1:
		switch msg.Runes[0] {
		case 'y', 'a':
			return m.result(true)
		case 'n', 'r':
			return m.result(false)
		}
	}
	return m, nil
}

func (m tofuModal) updateMouse(msg tea.MouseMsg) (tofuModal, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if zone.Get("tofu-reject").InBounds(msg) {
		return m.result(false)
	}
	if zone.Get("tofu-accept").InBounds(msg) {
		return m.result(true)
	}
	return m, nil
}

func (m tofuModal) View() string {
	if !m.active {
		return ""
	}

	modalW := min(64, m.width-10)
	if modalW < 40 {
		modalW = 40
	}
	contentW := modalW - borderSize
	usableW := contentW - 2
	pad := " "

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryText).MaxWidth(usableW)
	labelStyle := lipgloss.NewStyle().Foreground(mutedColor).MaxWidth(usableW)
	valueStyle := lipgloss.NewStyle().Foreground(primaryText).MaxWidth(usableW)
	hintStyle := lipgloss.NewStyle().Foreground(mutedColor).Italic(true).MaxWidth(usableW)

	subject := m.profile.Subject
	if subject == "" {
		subject = m.profile.SubjectRegex
	}
	issuer := m.profile.Issuer
	if issuer == "" {
		issuer = m.profile.IssuerRegex
	}

	lines := []string{
		pad + titleStyle.Render(fmt.Sprintf("Trust signing identity for %q?", m.name)),
		"",
		pad + labelStyle.Render("Issuer:"),
		pad + valueStyle.Render(truncate(issuer, usableW)),
		pad + labelStyle.Render("Subject:"),
		pad + valueStyle.Render(truncate(subject, usableW)),
	}
	if m.manifestURL != "" {
		lines = append(lines,
			pad+labelStyle.Render("Manifest:"),
			pad+valueStyle.Render(truncate(m.manifestURL, usableW)),
		)
	}
	lines = append(lines,
		"",
		pad+hintStyle.Render("Accept pins this identity. Future syncs that change it will be rejected."),
		"",
	)

	rejectBtn := m.renderButton("Reject", tofuFocusReject, "tofu-reject", false)
	acceptBtn := m.renderButton("Accept", tofuFocusAccept, "tofu-accept", true)
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, rejectBtn, "  ", acceptBtn)
	buttonsW := lipgloss.Width(buttons)
	buttonPad := max(0, usableW-buttonsW)
	lines = append(lines, pad+strings.Repeat(" ", buttonPad)+buttons)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Width(contentW).
		MaxWidth(modalW).
		Render(content)
}

func (m tofuModal) renderButton(label string, idx tofuFocus, zoneID string, primary bool) string {
	style := lipgloss.NewStyle().Padding(0, 2)
	if m.focusIdx == idx {
		fg := lipgloss.AdaptiveColor{Light: "#FFFCF0", Dark: "#100F0F"}
		bg := accentColor
		if primary {
			bg = successColor
		}
		style = style.Bold(true).Foreground(fg).Background(bg)
	} else {
		style = style.
			Foreground(primaryText).
			Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"})
	}
	return zone.Mark(zoneID, style.Render(label))
}
