package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// modifiedReason mirrors installcheck.Reason's three Modified variants so the
// TUI modal can vary its copy + default focus without depending on the scan
// package directly (keeps the modal self-contained and test-friendly).
type modifiedReason int

const (
	modifiedReasonEdited modifiedReason = iota
	modifiedReasonMissing
	modifiedReasonUnreadable
)

// installModifiedDecisionMsg carries the user's choice from the Case B
// (D16 Modified state) re-install modal. action is one of
// "drop-record" | "append-fresh" | "keep" and maps 1:1 to --on-modified.
type installModifiedDecisionMsg struct {
	action     string
	targetFile string
}

// installModifiedModal is the D17 Case B modal — the re-install path for a
// rule whose recorded bytes are no longer present at the target. All three
// reasons (edited, missing, unreadable) share the same three options; only
// the copy and default focus differ per D17's reason-varying-default table.
type installModifiedModal struct {
	active     bool
	targetFile string
	reason     modifiedReason
	readErr    string // populated only for modifiedReasonUnreadable
	// focusIdx: 0 = Drop, 1 = Append fresh, 2 = Keep
	focusIdx int
	width    int
}

// newInstallModifiedModal constructs an inactive modal sized wide enough for
// the reason-varying copy (D17 Case B). Chosen empirically so the longest
// line fits on one row at default usableW ≈ 76.
func newInstallModifiedModal() installModifiedModal {
	return installModifiedModal{width: 80}
}

// Open activates the modal and selects the reason-appropriate default focus
// per D17: Drop for edited/missing (the record is provably stale), Keep for
// unreadable (syllago could not observe the file, so don't assert a state).
func (m *installModifiedModal) Open(targetFile string, reason modifiedReason, readErr string) {
	m.active = true
	m.targetFile = targetFile
	m.reason = reason
	m.readErr = readErr
	switch reason {
	case modifiedReasonUnreadable:
		m.focusIdx = 2 // Keep
	default:
		m.focusIdx = 0 // Drop
	}
}

// Close deactivates the modal and clears state.
func (m *installModifiedModal) Close() {
	m.active = false
	m.targetFile = ""
	m.reason = 0
	m.readErr = ""
	m.focusIdx = 0
}

// IsActive reports whether the modal is currently displayed.
func (m installModifiedModal) IsActive() bool { return m.active }

// Update routes keyboard + mouse input. Any selection closes the modal.
func (m installModifiedModal) Update(msg tea.Msg) (installModifiedModal, tea.Cmd) {
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

func (m installModifiedModal) emit(action string) (installModifiedModal, tea.Cmd) {
	target := m.targetFile
	m.Close()
	return m, func() tea.Msg {
		return installModifiedDecisionMsg{action: action, targetFile: target}
	}
}

func (m installModifiedModal) currentAction() string {
	switch m.focusIdx {
	case 0:
		return "drop-record"
	case 1:
		return "append-fresh"
	default:
		return "keep"
	}
}

func (m installModifiedModal) updateKey(msg tea.KeyMsg) (installModifiedModal, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		// Esc is "Skip / Keep" — the safe non-mutating exit per D17 Case B.
		return m.emit("keep")
	case tea.KeyEnter:
		return m.emit(m.currentAction())
	case tea.KeyUp, tea.KeyShiftTab:
		if m.focusIdx > 0 {
			m.focusIdx--
		}
	case tea.KeyDown, tea.KeyTab:
		if m.focusIdx < 2 {
			m.focusIdx++
		}
	case tea.KeyRunes:
		if len(msg.Runes) != 1 {
			return m, nil
		}
		switch msg.Runes[0] {
		case 'k':
			if m.focusIdx > 0 {
				m.focusIdx--
			}
		case 'j':
			if m.focusIdx < 2 {
				m.focusIdx++
			}
		case 'd':
			return m.emit("drop-record")
		case 'a':
			return m.emit("append-fresh")
		}
	}
	return m, nil
}

func (m installModifiedModal) updateMouse(msg tea.MouseMsg) (installModifiedModal, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if zone.Get("inst-modified-drop").InBounds(msg) {
		return m.emit("drop-record")
	}
	if zone.Get("inst-modified-append").InBounds(msg) {
		return m.emit("append-fresh")
	}
	if zone.Get("inst-modified-keep").InBounds(msg) {
		return m.emit("keep")
	}
	return m, nil
}

// View renders the Case B modal. The copy above the options is reason-varying
// and quotes D17's exact text so the non-interactive error message and the
// modal explanation stay in sync.
func (m installModifiedModal) View() string {
	if !m.active {
		return ""
	}
	contentW := m.width - borderSize
	usableW := contentW - 2
	pad := " "

	title := pad + boldStyle.Render("Install record is stale")
	header := pad + "This rule was installed at:"
	targetLine := pad + "  " + truncate(m.targetFile, usableW-2)

	// Reason-varying explanation lines. Strings are the exact D17 copy that
	// tests assert literally — do NOT reword without updating tests + docs.
	// Each line is MaxWidth-constrained as a safety net against narrow
	// terminals (lipgloss.Border would otherwise wrap, breaking layout).
	lineStyle := lipgloss.NewStyle().Foreground(primaryText).MaxWidth(usableW)
	var rawReason []string
	switch m.reason {
	case modifiedReasonEdited:
		rawReason = []string{
			"...but the file no longer contains the version we recorded.",
			"Either the appended block was edited, or the file was rolled back.",
		}
	case modifiedReasonMissing:
		rawReason = []string{
			"...but that file no longer exists. It may have been deleted, renamed, or",
			"the project may have been moved.",
		}
	case modifiedReasonUnreadable:
		err := m.readErr
		if err == "" {
			err = "unknown I/O error"
		}
		rawReason = []string{
			"...but we couldn't read the file: " + truncate(err, usableW-30),
			"Check permissions and whether the path is still accessible, then re-run.",
		}
	}
	reasonLines := make([]string, 0, len(rawReason))
	for _, l := range rawReason {
		reasonLines = append(reasonLines, pad+lineStyle.Render(l))
	}

	// Options are the same set across reasons; per-reason default focus is
	// handled in Open(). Hotkey hints shown match the modal copy in D17.
	dropHotkey := "[Enter]"
	appendHotkey := "[a]"
	keepHotkey := "[Esc]"
	if m.reason == modifiedReasonUnreadable {
		dropHotkey = "[d]"
		appendHotkey = "[a]"
		keepHotkey = "[Enter]"
	}

	options := []string{
		pad + renderModifiedOption(0, m.focusIdx, "Drop stale install record (no file change)", dropHotkey, "inst-modified-drop"),
		pad + renderModifiedOption(1, m.focusIdx, "Append current version as a fresh copy", appendHotkey, "inst-modified-append"),
		pad + renderModifiedOption(2, m.focusIdx, "Keep (leave record + file unchanged)", keepHotkey, "inst-modified-keep"),
	}

	rows := []string{
		title,
		"",
		header,
		targetLine,
	}
	rows = append(rows, reasonLines...)
	rows = append(rows, "")
	rows = append(rows, options...)

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Width(contentW).
		MaxWidth(m.width).
		Render(content)
}

// renderModifiedOption matches renderUpdateOption's layout so the two modals
// share a visual language. Separate function to keep the zone IDs local.
func renderModifiedOption(idx, focusIdx int, label, hotkey, zoneID string) string {
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
