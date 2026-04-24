package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// installUpdateDecisionMsg carries the user's choice from the Case A
// (D16 Clean state) re-install modal. action is one of "replace" | "skip"
// and maps 1:1 to the --on-clean flag values.
type installUpdateDecisionMsg struct {
	action     string // "replace" | "skip"
	targetFile string
}

// installUpdateModal is the D17 Case A modal — the re-install path for a
// rule that is already installed at the clean state. Two options, default
// focus Replace (the common intent when the user invoked install).
type installUpdateModal struct {
	active            bool
	targetFile        string
	recordedShortHash string
	newShortHash      string
	// focusIdx: 0 = Replace, 1 = Skip
	focusIdx int
	width    int
}

// newInstallUpdateModal constructs an inactive modal with the standard
// modalWidth so centered overlay code can compose it without reconfiguration.
func newInstallUpdateModal() installUpdateModal {
	return installUpdateModal{width: 56}
}

// Open activates the modal with the context needed for D17's Clean-state copy.
// Default focus is Replace per D17 Case A.
func (m *installUpdateModal) Open(targetFile, recordedShortHash, newShortHash string) {
	m.active = true
	m.targetFile = targetFile
	m.recordedShortHash = recordedShortHash
	m.newShortHash = newShortHash
	m.focusIdx = 0 // Replace
}

// Close deactivates the modal and clears state.
func (m *installUpdateModal) Close() {
	m.active = false
	m.targetFile = ""
	m.recordedShortHash = ""
	m.newShortHash = ""
	m.focusIdx = 0
}

// IsActive reports whether the modal is currently displayed.
func (m installUpdateModal) IsActive() bool { return m.active }

// Update routes keyboard and mouse input to the appropriate handler. Emitting
// an installUpdateDecisionMsg closes the modal so the App's message handler
// can dispatch the execute step.
func (m installUpdateModal) Update(msg tea.Msg) (installUpdateModal, tea.Cmd) {
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

func (m installUpdateModal) emit(action string) (installUpdateModal, tea.Cmd) {
	target := m.targetFile
	m.Close()
	return m, func() tea.Msg {
		return installUpdateDecisionMsg{action: action, targetFile: target}
	}
}

func (m installUpdateModal) updateKey(msg tea.KeyMsg) (installUpdateModal, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		return m.emit("skip")
	case tea.KeyEnter:
		if m.focusIdx == 0 {
			return m.emit("replace")
		}
		return m.emit("skip")
	case tea.KeyUp, tea.KeyShiftTab:
		if m.focusIdx > 0 {
			m.focusIdx--
		}
	case tea.KeyDown, tea.KeyTab:
		if m.focusIdx < 1 {
			m.focusIdx++
		}
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 'k':
				if m.focusIdx > 0 {
					m.focusIdx--
				}
			case 'j':
				if m.focusIdx < 1 {
					m.focusIdx++
				}
			}
		}
	}
	return m, nil
}

func (m installUpdateModal) updateMouse(msg tea.MouseMsg) (installUpdateModal, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if zone.Get("inst-update-replace").InBounds(msg) {
		return m.emit("replace")
	}
	if zone.Get("inst-update-skip").InBounds(msg) {
		return m.emit("skip")
	}
	return m, nil
}

// View renders the modal box. The App composes this into an overlay.
func (m installUpdateModal) View() string {
	if !m.active {
		return ""
	}

	contentW := m.width - borderSize
	usableW := contentW - 2
	pad := " "

	title := pad + boldStyle.Render("Already installed")

	headline := pad + "This rule is already installed at:"
	targetLine := pad + "  " + truncate(m.targetFile, usableW-2) + " (version " + shortHashLabel(m.recordedShortHash) + ")"
	currentLine := pad + "Current library version is " + shortHashLabel(m.newShortHash) + "."

	replaceLabel := "Replace with current version"
	skipLabel := "Skip (leave file unchanged)"

	options := []string{
		pad + renderUpdateOption(0, m.focusIdx, replaceLabel, "[Enter]", "inst-update-replace"),
		pad + renderUpdateOption(1, m.focusIdx, skipLabel, "[Esc]", "inst-update-skip"),
	}

	rows := []string{
		title,
		"",
		headline,
		targetLine,
		currentLine,
		"",
	}
	rows = append(rows, options...)
	content := lipgloss.JoinVertical(lipgloss.Left, rows...)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Width(contentW).
		MaxWidth(m.width).
		Render(content)
}

// renderUpdateOption renders a single radio-style option row with the hotkey
// hint right-aligned. Focused rows get the accent highlight.
func renderUpdateOption(idx, focusIdx int, label, hotkey, zoneID string) string {
	marker := "  "
	labelStyle := lipgloss.NewStyle().Foreground(primaryText)
	hotkeyStyle := mutedStyle
	if idx == focusIdx {
		marker = "> "
		labelStyle = labelStyle.Bold(true).Foreground(accentColor)
		hotkeyStyle = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	}
	row := marker + labelStyle.Render(label) + "  " + hotkeyStyle.Render(hotkey)
	return zone.Mark(zoneID, row)
}

// shortHashLabel renders an 8-char prefix of a canonical "<algo>:<hex>" hash.
// Falls back to the raw value when shorter than 8 hex chars.
func shortHashLabel(h string) string {
	// Split off the algo prefix (everything through ':') for display.
	algoLen := strings.Index(h, ":")
	if algoLen < 0 {
		return truncate(h, 12)
	}
	body := h[algoLen+1:]
	if len(body) > 8 {
		body = body[:8]
	}
	return fmt.Sprintf("%s:%s", h[:algoLen], body)
}
