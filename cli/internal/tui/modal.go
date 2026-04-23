package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// editSavedMsg is emitted when the user confirms the edit modal.
type editSavedMsg struct {
	name        string
	description string
	path        string // item directory path (context for saving)
	// context disambiguates who should handle the save:
	//   ""              — library/explorer edit (App writes to .syllago.yaml)
	//   "wizard_rename" — add-wizard review rename (wizard updates in-memory item)
	context string
}

// editCancelledMsg is emitted when the user cancels the edit modal.
type editCancelledMsg struct{}

// editModal is a centered overlay with name and description fields plus Cancel/Save buttons.
// Focus indices: 0 = name field, 1 = description field, 2 = Cancel button, 3 = Save button.
type editModal struct {
	active      bool
	title       string
	name        string
	description string
	path        string // item directory path
	cursor      int    // cursor position within the focused text field
	focusIdx    int    // 0=name, 1=description, 2=Cancel, 3=Save
	// context is propagated into the emitted editSavedMsg so different callers
	// (library edit vs wizard rename) can route the save correctly.
	context string

	width  int
	height int
}

func newEditModal() editModal {
	return editModal{
		width:  56,
		height: 14,
	}
}

// SetWidth adjusts the modal's render width (the default 56 works for a
// centered overlay, but inline placements like the drill-in tree column
// need to clamp to the pane width).
func (m *editModal) SetWidth(w int) {
	if w < 24 {
		w = 24
	}
	m.width = w
}

// Open activates the modal with pre-filled values.
func (m *editModal) Open(title, name, description, path string) {
	m.OpenWithContext(title, name, description, path, "")
}

// OpenWithContext activates the modal and tags the save with a context string
// so the recipient can disambiguate (e.g., "wizard_rename" vs library edit).
func (m *editModal) OpenWithContext(title, name, description, path, context string) {
	m.active = true
	m.title = title
	m.name = name
	m.description = description
	m.path = path
	m.context = context
	m.cursor = len([]rune(name))
	m.focusIdx = 0
}

// Close deactivates the modal and clears state.
func (m *editModal) Close() {
	m.active = false
	m.title = ""
	m.name = ""
	m.description = ""
	m.path = ""
	m.context = ""
	m.cursor = 0
	m.focusIdx = 0
}

// focusedValue returns a pointer to the text value for the currently focused field.
func (m *editModal) focusedValue() *string {
	if m.focusIdx == 1 {
		return &m.description
	}
	return &m.name
}

// isTextField returns true if the current focus is on a text input field.
func (m *editModal) isTextField() bool {
	return m.focusIdx == 0 || m.focusIdx == 1
}

// Update handles input when the modal is active.
func (m editModal) Update(msg tea.Msg) (editModal, tea.Cmd) {
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

func (m editModal) save() (editModal, tea.Cmd) {
	name := m.name
	desc := m.description
	path := m.path
	ctx := m.context
	m.Close()
	return m, func() tea.Msg {
		return editSavedMsg{name: name, description: desc, path: path, context: ctx}
	}
}

func (m editModal) cancel() (editModal, tea.Cmd) {
	m.Close()
	return m, func() tea.Msg { return editCancelledMsg{} }
}

func (m editModal) updateKey(msg tea.KeyMsg) (editModal, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		return m.cancel()

	case tea.KeyCtrlS:
		return m.save()

	case tea.KeyEnter:
		switch m.focusIdx {
		case 0:
			// Name field: move to description field
			m.focusIdx = 1
			m.cursor = len([]rune(m.description))
		case 1:
			// Description field: move to Save button
			m.focusIdx = 3
		case 2:
			return m.cancel()
		case 3:
			return m.save()
		}

	case tea.KeyTab:
		m.focusIdx = (m.focusIdx + 1) % 4
		if m.isTextField() {
			m.cursor = len([]rune(*m.focusedValue()))
		}
	case tea.KeyShiftTab:
		m.focusIdx = (m.focusIdx + 3) % 4
		if m.isTextField() {
			m.cursor = len([]rune(*m.focusedValue()))
		}

	case tea.KeyBackspace:
		if m.isTextField() && m.cursor > 0 {
			val := m.focusedValue()
			runes := []rune(*val)
			*val = string(runes[:m.cursor-1]) + string(runes[m.cursor:])
			m.cursor--
		}

	case tea.KeyDelete:
		if m.isTextField() {
			val := m.focusedValue()
			runes := []rune(*val)
			if m.cursor < len(runes) {
				*val = string(runes[:m.cursor]) + string(runes[m.cursor+1:])
			}
		}

	case tea.KeyUp:
		// Move between fields: description → name, or buttons → description
		if m.focusIdx == 1 {
			m.focusIdx = 0
			m.cursor = min(m.cursor, len([]rune(m.name)))
		} else if m.focusIdx >= 2 {
			m.focusIdx = 1
			m.cursor = len([]rune(m.description))
		}
	case tea.KeyDown:
		// Move between fields: name → description, or description → buttons
		switch m.focusIdx {
		case 0:
			m.focusIdx = 1
			m.cursor = min(m.cursor, len([]rune(m.description)))
		case 1:
			m.focusIdx = 3 // Jump to Save button
		}

	case tea.KeyLeft:
		if m.isTextField() && m.cursor > 0 {
			m.cursor--
		} else if m.focusIdx == 3 {
			m.focusIdx = 2 // Save → Cancel
		}
	case tea.KeyRight:
		if m.isTextField() && m.cursor < len([]rune(*m.focusedValue())) {
			m.cursor++
		} else if m.focusIdx == 2 {
			m.focusIdx = 3 // Cancel → Save
		}

	case tea.KeyHome, tea.KeyCtrlA:
		if m.isTextField() {
			m.cursor = 0
		}
	case tea.KeyEnd, tea.KeyCtrlE:
		if m.isTextField() {
			m.cursor = len([]rune(*m.focusedValue()))
		}

	case tea.KeySpace:
		if m.isTextField() {
			val := m.focusedValue()
			runes := []rune(*val)
			newRunes := make([]rune, 0, len(runes)+1)
			newRunes = append(newRunes, runes[:m.cursor]...)
			newRunes = append(newRunes, ' ')
			newRunes = append(newRunes, runes[m.cursor:]...)
			*val = string(newRunes)
			m.cursor++
		}

	case tea.KeyRunes:
		if m.isTextField() {
			val := m.focusedValue()
			runes := []rune(*val)
			newRunes := make([]rune, 0, len(runes)+len(msg.Runes))
			newRunes = append(newRunes, runes[:m.cursor]...)
			newRunes = append(newRunes, msg.Runes...)
			newRunes = append(newRunes, runes[m.cursor:]...)
			*val = string(newRunes)
			m.cursor += len(msg.Runes)
		}
	}
	return m, nil
}

func (m editModal) updateMouse(msg tea.MouseMsg) (editModal, tea.Cmd) {
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	if zone.Get("modal-cancel").InBounds(msg) {
		return m.cancel()
	}
	if zone.Get("modal-save").InBounds(msg) {
		return m.save()
	}
	if zone.Get("modal-name").InBounds(msg) {
		m.focusIdx = 0
		m.cursor = len([]rune(m.name))
	}
	if zone.Get("modal-desc").InBounds(msg) {
		m.focusIdx = 1
		m.cursor = len([]rune(m.description))
	}

	return m, nil
}

// View renders the modal overlay content (without placement — the app handles centering).
func (m editModal) View() string {
	if !m.active {
		return ""
	}

	contentW := m.width - borderSize
	usableW := contentW - 2
	pad := " "

	// Title
	titleText := lipgloss.NewStyle().Bold(true).Foreground(primaryText).Render(m.title)
	title := pad + titleText

	// Name label + field
	nameLabel := pad + mutedStyle.Render("Display Name")
	nameInput := m.renderField(m.name, 0, usableW, "modal-name")

	// Description label + field
	descLabel := pad + mutedStyle.Render("Description")
	descInput := m.renderField(m.description, 1, usableW, "modal-desc")

	// Microcopy — clarifies where Name and Description actually show up. The
	// wizard_rename context adds "after you add it" since the item isn't in
	// the library yet; the library edit path is already editing a saved item.
	// Wrapped (not truncated) so the full message is visible in narrow modal
	// placements like the drill-in tree column.
	var hint string
	if m.context == "wizard_rename" {
		hint = "Shown in your library after you add this item. The on-disk name is unchanged."
	} else {
		hint = "Shown in your library. The on-disk name is unchanged."
	}
	hintLines := wordWrap(hint, usableW)
	hintRendered := make([]string, len(hintLines))
	for i, hl := range hintLines {
		hintRendered[i] = pad + mutedStyle.Render(hl)
	}

	// Buttons
	cancelBtn := m.renderButton("Cancel", 2, "modal-cancel")
	saveBtn := m.renderButton("Save", 3, "modal-save")
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, cancelBtn, " ", saveBtn)
	buttonsW := lipgloss.Width(buttons)
	buttonPad := max(0, usableW-buttonsW)
	buttonRow := pad + strings.Repeat(" ", buttonPad) + buttons

	rows := []string{
		title,
		"",
		nameLabel,
		nameInput,
		"",
		descLabel,
		descInput,
	}
	rows = append(rows, hintRendered...)
	rows = append(rows, "", buttonRow)
	content := lipgloss.JoinVertical(lipgloss.Left, rows...)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Width(contentW).
		MaxWidth(m.width).
		Render(content)
}

// renderField renders a text input field with background tinting and cursor.
func (m editModal) renderField(value string, fieldIdx, usableW int, zoneID string) string {
	bg := modalFieldInactiveBG
	if m.focusIdx == fieldIdx {
		bg = inputActiveBG
	}
	displayVal := m.renderValueWithCursor(value, fieldIdx, usableW-2)
	style := lipgloss.NewStyle().
		Background(bg).
		Foreground(primaryText).
		Width(usableW).
		Padding(0, 1)
	return zone.Mark(zoneID, " "+style.Render(displayVal)+" ")
}

// renderValueWithCursor renders text with a block cursor when the field is focused.
func (m editModal) renderValueWithCursor(value string, fieldIdx, maxW int) string {
	if m.focusIdx != fieldIdx {
		return truncate(value, maxW)
	}

	runes := []rune(value)
	if m.cursor >= len(runes) {
		return truncate(value+"\u2588", maxW)
	}
	before := string(runes[:m.cursor])
	under := string(runes[m.cursor : m.cursor+1])
	after := string(runes[m.cursor+1:])
	cursorChar := lipgloss.NewStyle().Reverse(true).Render(under)
	return truncate(before+cursorChar+after, maxW)
}

// renderButton renders a button label with focus styling.
func (m editModal) renderButton(label string, idx int, zoneID string) string {
	style := lipgloss.NewStyle().Padding(0, 2)
	if m.focusIdx == idx {
		style = style.
			Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#FFFCF0", Dark: "#100F0F"}).
			Background(accentColor)
	} else {
		style = style.
			Foreground(primaryText).
			Background(lipgloss.AdaptiveColor{Light: "#DAD8CE", Dark: "#403E3C"})
	}
	return zone.Mark(zoneID, style.Render(label))
}
