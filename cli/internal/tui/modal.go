package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// modalSavedMsg is emitted when the user confirms the modal input.
type modalSavedMsg struct {
	value   string
	context string
}

// modalCancelledMsg is emitted when the user cancels the modal.
type modalCancelledMsg struct{}

// textInputModal is a centered overlay with a text field, title, and Cancel/Save buttons.
type textInputModal struct {
	active  bool
	title   string
	value   string
	context string
	cursor  int // cursor position within value

	// Focus: 0 = text field, 1 = Cancel button, 2 = Save button
	focusIdx int

	width  int // outer width of the modal box
	height int // outer height of the modal box
}

func newTextInputModal() textInputModal {
	return textInputModal{
		width:  50,
		height: 8,
	}
}

// Open activates the modal with a title and pre-filled value.
func (m *textInputModal) Open(title, value, context string) {
	m.active = true
	m.title = title
	m.value = value
	m.context = context
	m.cursor = len([]rune(value))
	m.focusIdx = 0
}

// Close deactivates the modal.
func (m *textInputModal) Close() {
	m.active = false
	m.value = ""
	m.context = ""
	m.cursor = 0
	m.focusIdx = 0
}

// Update handles input when the modal is active.
func (m textInputModal) Update(msg tea.Msg) (textInputModal, tea.Cmd) {
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

func (m textInputModal) updateKey(msg tea.KeyMsg) (textInputModal, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.Close()
		return m, func() tea.Msg { return modalCancelledMsg{} }

	case tea.KeyEnter:
		if m.focusIdx == 1 {
			// Cancel button focused
			m.Close()
			return m, func() tea.Msg { return modalCancelledMsg{} }
		}
		// Save (text field or Save button)
		val := m.value
		ctx := m.context
		m.Close()
		return m, func() tea.Msg { return modalSavedMsg{value: val, context: ctx} }

	case tea.KeyTab:
		m.focusIdx = (m.focusIdx + 1) % 3
	case tea.KeyShiftTab:
		m.focusIdx = (m.focusIdx + 2) % 3

	case tea.KeyBackspace:
		if m.focusIdx == 0 && m.cursor > 0 {
			runes := []rune(m.value)
			m.value = string(runes[:m.cursor-1]) + string(runes[m.cursor:])
			m.cursor--
		}

	case tea.KeyDelete:
		if m.focusIdx == 0 {
			runes := []rune(m.value)
			if m.cursor < len(runes) {
				m.value = string(runes[:m.cursor]) + string(runes[m.cursor+1:])
			}
		}

	case tea.KeyLeft:
		if m.focusIdx == 0 && m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyRight:
		if m.focusIdx == 0 && m.cursor < len([]rune(m.value)) {
			m.cursor++
		}

	case tea.KeyHome, tea.KeyCtrlA:
		if m.focusIdx == 0 {
			m.cursor = 0
		}
	case tea.KeyEnd, tea.KeyCtrlE:
		if m.focusIdx == 0 {
			m.cursor = len([]rune(m.value))
		}

	case tea.KeyRunes:
		if m.focusIdx == 0 {
			runes := []rune(m.value)
			newRunes := make([]rune, 0, len(runes)+len(msg.Runes))
			newRunes = append(newRunes, runes[:m.cursor]...)
			newRunes = append(newRunes, msg.Runes...)
			newRunes = append(newRunes, runes[m.cursor:]...)
			m.value = string(newRunes)
			m.cursor += len(msg.Runes)
		}
	}
	return m, nil
}

func (m textInputModal) updateMouse(msg tea.MouseMsg) (textInputModal, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	if zone.Get("modal-cancel").InBounds(msg) {
		m.Close()
		return m, func() tea.Msg { return modalCancelledMsg{} }
	}
	if zone.Get("modal-save").InBounds(msg) {
		val := m.value
		ctx := m.context
		m.Close()
		return m, func() tea.Msg { return modalSavedMsg{value: val, context: ctx} }
	}
	if zone.Get("modal-input").InBounds(msg) {
		m.focusIdx = 0
	}

	return m, nil
}

// View renders the modal overlay content (without placement — the app handles centering).
func (m textInputModal) View() string {
	if !m.active {
		return ""
	}

	// Width math: outer = m.width, border = 2, so content area = m.width - 2.
	// We add 1-char padding manually on each side, so usable = contentW - 2.
	contentW := m.width - borderSize // width inside the border
	usableW := contentW - 2          // width inside 1-char manual padding

	pad := " " // manual 1-char padding prefix/suffix

	// Title
	titleText := lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(m.title)
	title := pad + titleText

	// Text input: background-tinted field
	inputBG := inputInactiveBG
	if m.focusIdx == 0 {
		inputBG = inputActiveBG
	}
	displayVal := m.renderValueWithCursor(usableW - 2) // -2 for inner padding
	inputStyle := lipgloss.NewStyle().
		Background(inputBG).
		Foreground(primaryText).
		Width(usableW).
		Padding(0, 1)
	input := zone.Mark("modal-input", pad+inputStyle.Render(displayVal)+pad)

	// Buttons — right-aligned within usable width
	cancelBtn := m.renderButton("Cancel", 1, "modal-cancel")
	saveBtn := m.renderButton("Save", 2, "modal-save")
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, cancelBtn, " ", saveBtn)
	buttonsW := lipgloss.Width(buttons)
	buttonPad := max(0, usableW-buttonsW)
	buttonRow := pad + strings.Repeat(" ", buttonPad) + buttons

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		input,
		"",
		buttonRow,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Width(contentW).
		MaxWidth(m.width).
		Render(content)
}

// renderValueWithCursor renders the text value with a block cursor at the current position.
func (m textInputModal) renderValueWithCursor(maxW int) string {
	if m.focusIdx != 0 {
		// Not focused — show plain text
		return truncate(m.value, maxW)
	}

	runes := []rune(m.value)
	if m.cursor >= len(runes) {
		// Cursor at end — append block cursor
		return truncate(m.value+"█", maxW)
	}
	// Cursor in middle — highlight character under cursor
	before := string(runes[:m.cursor])
	under := string(runes[m.cursor : m.cursor+1])
	after := string(runes[m.cursor+1:])
	cursorChar := lipgloss.NewStyle().Reverse(true).Render(under)
	return truncate(before+cursorChar+after, maxW)
}

// renderButton renders a button label with focus styling.
// All buttons use background+padding for consistent height (no borders).
func (m textInputModal) renderButton(label string, idx int, zoneID string) string {
	style := lipgloss.NewStyle().Padding(0, 2)
	if m.focusIdx == idx {
		style = style.
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFCF0", Dark: "#100F0F"}).
			Background(accentColor)
	} else {
		style = style.
			Foreground(primaryText).
			Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"}) // inactive bg
	}
	return zone.Mark(zoneID, style.Render(label))
}
