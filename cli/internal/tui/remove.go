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

// removeResultMsg carries the user's decision from the multi-step Remove modal.
type removeResultMsg struct {
	confirmed          bool
	item               catalog.ContentItem
	uninstallProviders []provider.Provider // only the selected providers
}

type removeStep int

const (
	removeStepConfirm   removeStep = iota // Step 1: confirm removal
	removeStepProviders                   // Step 2: select providers to uninstall
	removeStepReview                      // Step 3: review and execute
)

// removeModal is a multi-step overlay for removing library items.
// Step 1: Confirm removal (and ask about providers if installed).
// Step 2: Select providers to uninstall from (conditional).
// Step 3: Review what will happen and confirm.
type removeModal struct {
	active bool
	step   removeStep
	width  int
	height int

	// Item context
	item     catalog.ContentItem
	itemName string

	// Provider data (computed on open)
	installedProviders []provider.Provider // providers where item is installed
	providerChecks     []bool              // parallel array: selected for uninstall

	// Focus index — meaning varies by step (see buttonCount/firstButtonIdx)
	focusIdx int

	// Tracks whether user chose "No" (skip Step 2)
	skippedProviders bool
}

func newRemoveModal() removeModal {
	return removeModal{}
}

// Open activates the modal for the given item.
func (m *removeModal) Open(item catalog.ContentItem, installedProviders []provider.Provider) {
	m.active = true
	m.step = removeStepConfirm
	m.item = item
	m.itemName = item.DisplayName
	if m.itemName == "" {
		m.itemName = item.Name
	}
	m.installedProviders = installedProviders
	m.providerChecks = make([]bool, len(installedProviders)) // all unchecked
	m.focusIdx = 0                                           // Cancel
	m.skippedProviders = false
}

// Close deactivates the modal and clears state.
func (m *removeModal) Close() {
	m.active = false
	m.step = removeStepConfirm
	m.item = catalog.ContentItem{}
	m.itemName = ""
	m.installedProviders = nil
	m.providerChecks = nil
	m.focusIdx = 0
	m.skippedProviders = false
}

func (m removeModal) isInstalled() bool {
	return len(m.installedProviders) > 0
}

// --- Focus helpers per step ---

// Step 1 not installed: Cancel(0), Remove(1)
// Step 1 installed:     Cancel(0), RemoveOnly(1), Yes(2)
// Step 2:               checkbox-0..N-1, Back(N), Done(N+1)
// Step 3:               Cancel(0), Back(1), Remove(2)

func (m removeModal) buttonCount() int {
	switch m.step {
	case removeStepConfirm:
		if m.isInstalled() {
			return 3 // Cancel, Remove Only, Yes
		}
		return 2 // Cancel, Remove
	case removeStepProviders:
		return 2 // Back, Done
	case removeStepReview:
		return 3 // Cancel, Back, Remove
	}
	return 2
}

func (m removeModal) focusCount() int {
	if m.step == removeStepProviders {
		return len(m.installedProviders) + 2 // checkboxes + Back + Done
	}
	return m.buttonCount()
}

func (m removeModal) firstButtonIdx() int {
	if m.step == removeStepProviders {
		return len(m.installedProviders) // first button after checkboxes
	}
	return 0
}

func (m removeModal) lastButtonIdx() int {
	return m.focusCount() - 1
}

func (m removeModal) isButtonFocus() bool {
	return m.focusIdx >= m.firstButtonIdx()
}

func (m removeModal) isCheckboxFocus() bool {
	return m.step == removeStepProviders && m.focusIdx < len(m.installedProviders)
}

// --- Result ---

func (m removeModal) result(confirmed bool) (removeModal, tea.Cmd) {
	var selected []provider.Provider
	if confirmed {
		for i, prov := range m.installedProviders {
			if i < len(m.providerChecks) && m.providerChecks[i] {
				selected = append(selected, prov)
			}
		}
	}
	res := removeResultMsg{
		confirmed:          confirmed,
		item:               m.item,
		uninstallProviders: selected,
	}
	m.Close()
	return m, func() tea.Msg { return res }
}

// --- Update ---

func (m removeModal) Update(msg tea.Msg) (removeModal, tea.Cmd) {
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

func (m removeModal) updateKey(msg tea.KeyMsg) (removeModal, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyEsc:
		return m.result(false)

	// y/n shortcuts only for simple (not-installed) Step 1
	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'y':
		if m.step == removeStepConfirm && !m.isInstalled() {
			return m.result(true)
		}
	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'n':
		if m.step == removeStepConfirm && !m.isInstalled() {
			return m.result(false)
		}

	case msg.Type == tea.KeyEnter:
		return m.handleEnter()

	case msg.Type == tea.KeySpace:
		if m.isCheckboxFocus() {
			m.providerChecks[m.focusIdx] = !m.providerChecks[m.focusIdx]
		}

	case msg.Type == tea.KeyTab:
		m.focusIdx = (m.focusIdx + 1) % m.focusCount()
	case msg.Type == tea.KeyShiftTab:
		m.focusIdx = (m.focusIdx + m.focusCount() - 1) % m.focusCount()

	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'j'):
		if m.focusIdx < m.lastButtonIdx() {
			m.focusIdx++
		}
	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'k'):
		if m.focusIdx > 0 {
			m.focusIdx--
		}

	case msg.Type == tea.KeyLeft || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'h'):
		if m.isButtonFocus() {
			if m.focusIdx == m.firstButtonIdx() {
				m.focusIdx = m.lastButtonIdx()
			} else {
				m.focusIdx--
			}
		}
	case msg.Type == tea.KeyRight || (msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'l'):
		if m.isButtonFocus() {
			if m.focusIdx == m.lastButtonIdx() {
				m.focusIdx = m.firstButtonIdx()
			} else {
				m.focusIdx++
			}
		}
	}
	return m, nil
}

func (m removeModal) handleEnter() (removeModal, tea.Cmd) {
	switch m.step {
	case removeStepConfirm:
		if m.isInstalled() {
			// 3 buttons: Cancel(0), Remove Only(1), Yes(2)
			switch m.focusIdx {
			case 0:
				return m.result(false)
			case 1: // Remove Only → skip to review
				m.skippedProviders = true
				m.step = removeStepReview
				m.focusIdx = 0
			case 2: // Yes → provider selection
				m.step = removeStepProviders
				m.focusIdx = 0
			}
		} else {
			// 2 buttons: Cancel(0), Remove(1)
			switch m.focusIdx {
			case 0:
				return m.result(false)
			case 1:
				return m.result(true)
			}
		}

	case removeStepProviders:
		backIdx := len(m.installedProviders)
		doneIdx := len(m.installedProviders) + 1
		switch m.focusIdx {
		case backIdx:
			m.step = removeStepConfirm
			m.focusIdx = 0
		case doneIdx:
			m.step = removeStepReview
			m.focusIdx = 0
		}
		// Enter on checkboxes is a no-op

	case removeStepReview:
		// 3 buttons: Cancel(0), Back(1), Remove(2)
		switch m.focusIdx {
		case 0:
			return m.result(false)
		case 1: // Back
			if m.skippedProviders {
				m.step = removeStepConfirm
			} else {
				m.step = removeStepProviders
			}
			m.focusIdx = 0
		case 2:
			return m.result(true)
		}
	}
	return m, nil
}

func (m removeModal) updateMouse(msg tea.MouseMsg) (removeModal, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	// Check button zones per step
	switch m.step {
	case removeStepConfirm:
		if zone.Get("rm-cancel").InBounds(msg) {
			return m.result(false)
		}
		if m.isInstalled() {
			if zone.Get("rm-remove-only").InBounds(msg) {
				m.skippedProviders = true
				m.step = removeStepReview
				m.focusIdx = 0
				return m, nil
			}
			if zone.Get("rm-yes").InBounds(msg) {
				m.step = removeStepProviders
				m.focusIdx = 0
				return m, nil
			}
		} else {
			if zone.Get("rm-remove").InBounds(msg) {
				return m.result(true)
			}
		}
	case removeStepProviders:
		if zone.Get("rm-back").InBounds(msg) {
			m.step = removeStepConfirm
			m.focusIdx = 0
			return m, nil
		}
		if zone.Get("rm-done").InBounds(msg) {
			m.step = removeStepReview
			m.focusIdx = 0
			return m, nil
		}
		for i := range m.installedProviders {
			if zone.Get(fmt.Sprintf("rm-prov-%d", i)).InBounds(msg) {
				m.providerChecks[i] = !m.providerChecks[i]
				m.focusIdx = i
				return m, nil
			}
		}
	case removeStepReview:
		if zone.Get("rm-cancel").InBounds(msg) {
			return m.result(false)
		}
		if zone.Get("rm-back").InBounds(msg) {
			if m.skippedProviders {
				m.step = removeStepConfirm
			} else {
				m.step = removeStepProviders
			}
			m.focusIdx = 0
			return m, nil
		}
		if zone.Get("rm-remove").InBounds(msg) {
			return m.result(true)
		}
	}
	return m, nil
}

// --- View ---

func (m removeModal) View() string {
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

	var body string
	switch m.step {
	case removeStepConfirm:
		body = m.viewConfirm(usableW, pad)
	case removeStepProviders:
		body = m.viewProviders(usableW, pad)
	case removeStepReview:
		body = m.viewReview(usableW, pad)
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(dangerColor).
		Width(contentW).
		MaxWidth(modalW).
		Render(body)
}

func (m removeModal) viewConfirm(usableW int, pad string) string {
	title := pad + lipgloss.NewStyle().Bold(true).Foreground(primaryText).MaxWidth(usableW).Render(
		fmt.Sprintf("Remove %q?", m.itemName))

	bodyText := fmt.Sprintf("This will remove %q from your library.", m.itemName)
	warning := lipgloss.NewStyle().Foreground(dangerColor).Render("This action cannot be undone.")

	var lines []string
	lines = append(lines, title, "")
	lines = append(lines, pad+lipgloss.NewStyle().Foreground(primaryText).MaxWidth(usableW).Render(bodyText))
	lines = append(lines, "")
	lines = append(lines, pad+warning)

	if m.isInstalled() {
		lines = append(lines, "")
		provMsg := fmt.Sprintf("This content is installed in %d provider(s).", len(m.installedProviders))
		lines = append(lines, pad+lipgloss.NewStyle().Foreground(primaryText).Render(provMsg))
		lines = append(lines, pad+lipgloss.NewStyle().Foreground(primaryText).Render("Also uninstall from those providers?"))
		lines = append(lines, "")
		lines = append(lines, m.renderButtons(usableW, pad,
			buttonDef{"Cancel", "rm-cancel", 0},
			buttonDef{"Remove only", "rm-remove-only", 1},
			buttonDef{"Remove and uninstall", "rm-yes", 2},
		))
	} else {
		lines = append(lines, "")
		lines = append(lines, m.renderButtons(usableW, pad,
			buttonDef{"Cancel", "rm-cancel", 0},
			buttonDef{"Remove", "rm-remove", 1},
		))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m removeModal) viewProviders(usableW int, pad string) string {
	title := pad + lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render("Uninstall from providers")

	var lines []string
	lines = append(lines, title, "")
	lines = append(lines, pad+lipgloss.NewStyle().Foreground(primaryText).Render("Select providers to uninstall from:"))
	lines = append(lines, "")

	for i, prov := range m.installedProviders {
		mark := "[ ]"
		if i < len(m.providerChecks) && m.providerChecks[i] {
			mark = "[x]"
		}
		style := lipgloss.NewStyle().Foreground(primaryText)
		if m.focusIdx == i {
			style = style.Bold(true).Foreground(accentColor)
		}
		checkLine := pad + style.Render(mark+" "+prov.Name)
		lines = append(lines, zone.Mark(fmt.Sprintf("rm-prov-%d", i), checkLine))
	}

	backIdx := len(m.installedProviders)
	doneIdx := len(m.installedProviders) + 1
	lines = append(lines, "")
	lines = append(lines, m.renderButtons(usableW, pad,
		buttonDef{"Back", "rm-back", backIdx},
		buttonDef{"Next", "rm-done", doneIdx},
	))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m removeModal) viewReview(usableW int, pad string) string {
	title := pad + lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render("Review")

	var lines []string
	lines = append(lines, title, "")
	lines = append(lines, pad+lipgloss.NewStyle().Foreground(primaryText).Render(
		fmt.Sprintf("Will remove %q from library.", m.itemName)))

	// "Will uninstall from" — only if providers selected
	var selected, remaining []string
	for i, prov := range m.installedProviders {
		if i < len(m.providerChecks) && m.providerChecks[i] {
			selected = append(selected, prov.Name)
		} else {
			remaining = append(remaining, prov.Name)
		}
	}

	if len(selected) > 0 {
		lines = append(lines, "")
		lines = append(lines, pad+lipgloss.NewStyle().Foreground(primaryText).Bold(true).Render("Will uninstall from:"))
		for _, name := range selected {
			lines = append(lines, pad+"  "+lipgloss.NewStyle().Foreground(primaryText).Render(name))
		}
	}

	// "Still installed in" — when installed but not all selected
	if len(remaining) > 0 {
		lines = append(lines, "")
		lines = append(lines, pad+lipgloss.NewStyle().Foreground(warningColor).Bold(true).Render("Still installed in:"))
		for _, name := range remaining {
			lines = append(lines, pad+"  "+lipgloss.NewStyle().Foreground(warningColor).Render(name))
		}
	}

	lines = append(lines, "")
	lines = append(lines, m.renderButtons(usableW, pad,
		buttonDef{"Cancel", "rm-cancel", 0},
		buttonDef{"Back", "rm-back", 1},
		buttonDef{"Remove", "rm-remove", 2},
	))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// --- Button rendering ---

type buttonDef struct {
	label   string
	zoneID  string
	focusAt int
}

func (m removeModal) renderButtons(usableW int, pad string, buttons ...buttonDef) string {
	var parts []string
	for _, b := range buttons {
		style := lipgloss.NewStyle().Padding(0, 2)
		if m.focusIdx == b.focusAt {
			fg := lipgloss.AdaptiveColor{Light: "#FFFCF0", Dark: "#100F0F"}
			bg := accentColor
			// Danger style for the final "Remove" button
			if b.label == "Remove" {
				bg = dangerColor
			}
			style = style.Bold(true).Foreground(fg).Background(bg)
		} else {
			style = style.
				Foreground(primaryText).
				Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"})
		}
		parts = append(parts, zone.Mark(b.zoneID, style.Render(b.label)))
	}

	buttons_str := strings.Join(parts, "  ")
	buttonsW := lipgloss.Width(buttons_str)
	buttonPad := max(0, usableW-buttonsW)
	return pad + strings.Repeat(" ", buttonPad) + buttons_str
}
