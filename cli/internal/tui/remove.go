package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// removeResultMsg carries the user's decision from the Remove modal.
type removeResultMsg struct {
	confirmed bool
	item      catalog.ContentItem
}

// removeModal confirms removing a library item. A remove also uninstalls
// the item from every provider that holds it, so the modal lists them.
type removeModal struct {
	active bool
	width  int
	height int

	item     catalog.ContentItem
	itemName string

	// uninstallFrom names every provider install and link the remove takes
	// out with the item.
	uninstallFrom []string

	focusIdx int // Cancel(0), Remove(1)
}

func newRemoveModal() removeModal {
	return removeModal{}
}

// Open activates the modal for item, listing what the remove uninstalls.
func (m *removeModal) Open(item catalog.ContentItem, uninstallFrom []string) {
	m.active = true
	m.item = item
	m.itemName = item.DisplayName
	if m.itemName == "" {
		m.itemName = item.Name
	}
	m.uninstallFrom = uninstallFrom
	m.focusIdx = 0 // Cancel
}

// Close deactivates the modal and clears state.
func (m *removeModal) Close() {
	m.active = false
	m.item = catalog.ContentItem{}
	m.itemName = ""
	m.uninstallFrom = nil
	m.focusIdx = 0
}

func (m removeModal) result(confirmed bool) (removeModal, tea.Cmd) {
	res := removeResultMsg{confirmed: confirmed, item: m.item}
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
	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'y':
		return m.result(true)
	case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'n':
		return m.result(false)
	case msg.Type == tea.KeyEnter:
		return m.result(m.focusIdx == 1)
	case msg.Type == tea.KeyTab, msg.Type == tea.KeyShiftTab,
		msg.Type == tea.KeyLeft, msg.Type == tea.KeyRight,
		msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && (msg.Runes[0] == 'h' || msg.Runes[0] == 'l'):
		m.focusIdx = 1 - m.focusIdx
	}
	return m, nil
}

func (m removeModal) updateMouse(msg tea.MouseMsg) (removeModal, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if zone.Get("rm-cancel").InBounds(msg) {
		return m.result(false)
	}
	if zone.Get("rm-remove").InBounds(msg) {
		return m.result(true)
	}
	return m, nil
}

// --- View ---

func (m removeModal) View() string {
	if !m.active {
		return ""
	}

	modalW := min(64, m.width-6)
	if modalW < 40 {
		modalW = 40
	}
	contentW := modalW - borderSize
	usableW := contentW - 2
	pad := " "
	text := lipgloss.NewStyle().Foreground(primaryText).MaxWidth(usableW)

	var lines []string
	lines = append(lines, pad+lipgloss.NewStyle().Bold(true).Foreground(primaryText).MaxWidth(usableW).Render(
		fmt.Sprintf("Remove %q?", m.itemName)), "")
	lines = append(lines, pad+text.Render(fmt.Sprintf("This will remove %q from your library.", m.itemName)))
	if len(m.uninstallFrom) > 0 {
		lines = append(lines, "", pad+text.Bold(true).Render("Will uninstall from:"))
		for _, name := range m.uninstallFrom {
			lines = append(lines, pad+"  "+text.Render(name))
		}
	}
	lines = append(lines, "", pad+lipgloss.NewStyle().Foreground(dangerColor).Render("This action cannot be undone."), "")
	lines = append(lines, renderModalButtons(m.focusIdx, usableW, pad, []string{"Remove"},
		buttonDef{"Cancel", "rm-cancel", 0},
		buttonDef{"Remove", "rm-remove", 1},
	))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(dangerColor).
		Width(contentW).
		MaxWidth(modalW).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
