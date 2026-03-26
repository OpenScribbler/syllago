package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// confirmResultMsg carries the user's decision back to App.
type confirmResultMsg struct {
	confirmed          bool
	checks             []confirmCheckbox
	item               catalog.ContentItem
	itemName           string
	uninstallProviders []provider.Provider
}

// confirmCheckbox represents a toggleable checkbox in the confirm modal.
type confirmCheckbox struct {
	label    string
	checked  bool
	readOnly bool // true = always checked, visually distinct, not toggleable
}

// confirmModal is a generic yes/no overlay with optional checkboxes.
// Focus indices: 0..len(checks)-1 = checkboxes, len(checks) = Cancel, len(checks)+1 = Confirm.
type confirmModal struct {
	active       bool
	title        string // e.g., "Remove \"my-hook\"?"
	body         string // context message (multi-line OK)
	danger       bool   // red border when true
	confirmLabel string // confirm button label (e.g., "Remove", "Uninstall")
	checks       []confirmCheckbox
	focusIdx     int
	width        int
	height       int

	// Caller context — passed through to result messages untouched.
	item               catalog.ContentItem
	itemName           string
	uninstallProviders []provider.Provider // only for uninstall flow
}

func newConfirmModal() confirmModal {
	return confirmModal{}
}

// cancelIdx returns the focus index for the Cancel button.
func (m confirmModal) cancelIdx() int { return len(m.checks) }

// confirmIdx returns the focus index for the Confirm button.
func (m confirmModal) confirmIdx() int { return len(m.checks) + 1 }

// focusCount returns the total number of focusable elements.
func (m confirmModal) focusCount() int { return len(m.checks) + 2 }

// isButtonFocus returns true if the current focus is on a button (not a checkbox).
func (m confirmModal) isButtonFocus() bool { return m.focusIdx >= m.cancelIdx() }

// Open activates the modal with the given parameters. Default focus on Cancel (safe default).
func (m *confirmModal) Open(title, body, confirmLabel string, danger bool, checks []confirmCheckbox) {
	m.active = true
	m.title = title
	m.body = body
	m.confirmLabel = confirmLabel
	m.danger = danger
	m.checks = checks
	m.focusIdx = m.cancelIdx()
	m.item = catalog.ContentItem{}
	m.itemName = ""
	m.uninstallProviders = nil
}

// OpenForItem is a convenience that also stores item context for the result message.
func (m *confirmModal) OpenForItem(title, body, confirmLabel string, danger bool, checks []confirmCheckbox, item catalog.ContentItem) {
	m.Open(title, body, confirmLabel, danger, checks)
	m.item = item
	m.itemName = item.Name
}

// Close deactivates the modal and clears state.
func (m *confirmModal) Close() {
	m.active = false
	m.title = ""
	m.body = ""
	m.confirmLabel = ""
	m.danger = false
	m.checks = nil
	m.focusIdx = 0
	m.item = catalog.ContentItem{}
	m.itemName = ""
	m.uninstallProviders = nil
}

func (m confirmModal) result(confirmed bool) (confirmModal, tea.Cmd) {
	res := confirmResultMsg{
		confirmed:          confirmed,
		checks:             m.checks,
		item:               m.item,
		itemName:           m.itemName,
		uninstallProviders: m.uninstallProviders,
	}
	m.Close()
	return m, func() tea.Msg { return res }
}

// Update handles input when the modal is active.
func (m confirmModal) Update(msg tea.Msg) (confirmModal, tea.Cmd) {
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

func (m confirmModal) updateKey(msg tea.KeyMsg) (confirmModal, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		return m.result(false)

	// y/n shortcuts from any focus position
	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'y':
		return m.result(true)
	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'n':
		return m.result(false)

	case msg.Type == tea.KeyEnter:
		switch m.focusIdx {
		case m.cancelIdx():
			return m.result(false)
		case m.confirmIdx():
			return m.result(true)
		}
		// Enter on checkboxes is a no-op

	case msg.Type == tea.KeySpace:
		if m.focusIdx < len(m.checks) && !m.checks[m.focusIdx].readOnly {
			m.checks[m.focusIdx].checked = !m.checks[m.focusIdx].checked
		}

	case msg.Type == tea.KeyTab:
		m.focusIdx = (m.focusIdx + 1) % m.focusCount()

	case msg.Type == tea.KeyShiftTab:
		m.focusIdx = (m.focusIdx + m.focusCount() - 1) % m.focusCount()

	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		if m.focusIdx < m.confirmIdx() {
			m.focusIdx++
		}
	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		if m.focusIdx > 0 {
			m.focusIdx--
		}

	case msg.Type == tea.KeyLeft:
		if m.isButtonFocus() {
			if m.focusIdx == m.cancelIdx() {
				m.focusIdx = m.confirmIdx() // wrap to last button
			} else {
				m.focusIdx--
			}
		}
	case msg.Type == tea.KeyRight:
		if m.isButtonFocus() {
			if m.focusIdx == m.confirmIdx() {
				m.focusIdx = m.cancelIdx() // wrap to first button
			} else {
				m.focusIdx++
			}
		}
	}
	return m, nil
}

func (m confirmModal) updateMouse(msg tea.MouseMsg) (confirmModal, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	if zone.Get("confirm-cancel").InBounds(msg) {
		return m.result(false)
	}
	if zone.Get("confirm-ok").InBounds(msg) {
		return m.result(true)
	}
	for i := range m.checks {
		if zone.Get(fmt.Sprintf("confirm-check-%d", i)).InBounds(msg) {
			if !m.checks[i].readOnly {
				m.checks[i].checked = !m.checks[i].checked
			}
			m.focusIdx = i
			return m, nil
		}
	}
	return m, nil
}

// View renders the modal overlay content (without placement — the app handles centering).
func (m confirmModal) View() string {
	if !m.active {
		return ""
	}

	modalW := min(54, m.width-10)
	if modalW < 34 {
		modalW = 34
	}
	contentW := modalW - borderSize
	usableW := contentW - 2
	pad := " "

	// Title — constrained to prevent wrapping past modal border
	titleText := lipgloss.NewStyle().Bold(true).Foreground(primaryText).MaxWidth(usableW).Render(m.title)
	title := pad + titleText

	// Body
	var lines []string
	lines = append(lines, title, "")
	for _, line := range strings.Split(m.body, "\n") {
		lines = append(lines, pad+lipgloss.NewStyle().Foreground(primaryText).MaxWidth(usableW).Render(line))
	}

	// Checkboxes
	if len(m.checks) > 0 {
		lines = append(lines, "")
		for i, c := range m.checks {
			mark := "[ ]"
			if c.checked {
				mark = "[x]"
			}
			style := lipgloss.NewStyle().Foreground(primaryText)
			if c.readOnly {
				style = style.Foreground(mutedColor)
			}
			if m.focusIdx == i {
				style = style.Bold(true).Foreground(accentColor)
			}
			checkLine := pad + style.Render(mark+" "+c.label)
			lines = append(lines, zone.Mark(fmt.Sprintf("confirm-check-%d", i), checkLine))
		}
	}

	// Buttons
	lines = append(lines, "")
	cancelBtn := m.renderConfirmButton("Cancel", m.cancelIdx(), "confirm-cancel", false)
	confirmBtn := m.renderConfirmButton(m.confirmLabel, m.confirmIdx(), "confirm-ok", m.danger)
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, cancelBtn, "  ", confirmBtn)
	buttonsW := lipgloss.Width(buttons)
	buttonPad := max(0, usableW-buttonsW)
	lines = append(lines, pad+strings.Repeat(" ", buttonPad)+buttons)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	borderColor := accentColor
	if m.danger {
		borderColor = dangerColor
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(contentW).
		MaxWidth(modalW).
		Render(content)
}

// renderConfirmButton renders a button with focus styling.
func (m confirmModal) renderConfirmButton(label string, idx int, zoneID string, danger bool) string {
	style := lipgloss.NewStyle().Padding(0, 2)
	if m.focusIdx == idx {
		fg := lipgloss.AdaptiveColor{Light: "#FFFCF0", Dark: "#100F0F"}
		bg := accentColor
		if danger && idx == m.confirmIdx() {
			bg = dangerColor
		}
		style = style.Bold(true).Foreground(fg).Background(bg)
	} else {
		style = style.
			Foreground(primaryText).
			Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"})
	}
	return zone.Mark(zoneID, style.Render(label))
}
