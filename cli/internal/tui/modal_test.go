package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestConfirmModalRender(t *testing.T) {
	m := newConfirmModal("Delete item?", "This will remove the item permanently.", "Cancel", "Delete")
	m.termWidth = 80
	m.termHeight = 40

	view := m.View()
	if !strings.Contains(view, "Delete item?") {
		t.Error("confirm modal should render title")
	}
	if !strings.Contains(view, "This will remove the item permanently.") {
		t.Error("confirm modal should render body")
	}
	if !strings.Contains(view, "Cancel") {
		t.Error("confirm modal should render left button")
	}
	if !strings.Contains(view, "Delete") {
		t.Error("confirm modal should render right button")
	}
}

func TestWarningModalRender(t *testing.T) {
	m := newWarningModal("Warning", "Potential risk detected.", "Dismiss", "Proceed")
	m.termWidth = 80
	m.termHeight = 40

	view := m.View()
	if !strings.Contains(view, "Warning") {
		t.Error("warning modal should render title")
	}
	if !strings.Contains(view, "Potential risk detected.") {
		t.Error("warning modal should render body")
	}
	if !strings.Contains(view, "Dismiss") {
		t.Error("warning modal should render left button")
	}
	if !strings.Contains(view, "Proceed") {
		t.Error("warning modal should render right button")
	}
}

func TestWizardModalRender(t *testing.T) {
	opts := []string{"Claude Code", "Gemini CLI", "Cursor"}
	m := newWizardModal("Install alpha-skill", 1, 3, opts)
	m.termWidth = 80
	m.termHeight = 40

	view := m.View()
	if !strings.Contains(view, "Install alpha-skill (1 of 3)") {
		t.Error("wizard modal should render step indicator in title")
	}
	if !strings.Contains(view, "Claude Code") {
		t.Error("wizard modal should render options")
	}
	if !strings.Contains(view, "[ ]") {
		t.Error("wizard modal should render unchecked boxes")
	}
}

func TestInputModalRender(t *testing.T) {
	m := newInputModal("Add Registry", "URL:", "https://github.com/team/rules.git")
	m.termWidth = 80
	m.termHeight = 40

	view := m.View()
	if !strings.Contains(view, "Add Registry") {
		t.Error("input modal should render title")
	}
	if !strings.Contains(view, "URL:") {
		t.Error("input modal should render input label")
	}
	if !strings.Contains(view, "https://github.com/team/rules.git") {
		t.Error("input modal should render input value")
	}
}

func TestButtonCursorSwitchesWithLeftRight(t *testing.T) {
	m := newConfirmModal("Test", "Body", "Cancel", "OK")
	m.termWidth = 80
	m.termHeight = 40

	if m.buttonCursor != 0 {
		t.Error("button cursor should start at 0 (left)")
	}

	// Press Right
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.buttonCursor != 1 {
		t.Errorf("button cursor should be 1 after Right, got %d", m.buttonCursor)
	}

	// Press Left
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.buttonCursor != 0 {
		t.Errorf("button cursor should be 0 after Left, got %d", m.buttonCursor)
	}
}

func TestEscClosesModal(t *testing.T) {
	m := newConfirmModal("Test", "Body", "Cancel", "OK")
	m.termWidth = 80
	m.termHeight = 40

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if m.active {
		t.Error("modal should be inactive after Esc")
	}
	if cmd == nil {
		t.Fatal("Esc should return a command")
	}
	msg := cmd()
	if _, ok := msg.(modalCloseMsg); !ok {
		t.Errorf("Esc should return modalCloseMsg, got %T", msg)
	}
}

func TestEnterConfirmsModal(t *testing.T) {
	m := newConfirmModal("Test", "Body", "Cancel", "OK")
	m.termWidth = 80
	m.termHeight = 40

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.active {
		t.Error("modal should be inactive after Enter")
	}
	if cmd == nil {
		t.Fatal("Enter should return a command")
	}
	msg := cmd()
	if confirm, ok := msg.(modalConfirmMsg); !ok {
		t.Errorf("Enter should return modalConfirmMsg, got %T", msg)
	} else if confirm.modalType != modalConfirm {
		t.Errorf("modalType should be modalConfirm, got %d", confirm.modalType)
	}
}

func TestConfirmModalYesShortcut(t *testing.T) {
	m := newConfirmModal("Test", "Body", "Cancel", "OK")
	m.termWidth = 80
	m.termHeight = 40

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if m.active {
		t.Error("modal should be inactive after 'y'")
	}
	if cmd == nil {
		t.Fatal("'y' should return a command")
	}
	msg := cmd()
	if _, ok := msg.(modalConfirmMsg); !ok {
		t.Errorf("'y' should return modalConfirmMsg, got %T", msg)
	}
}

func TestConfirmModalNoShortcut(t *testing.T) {
	m := newConfirmModal("Test", "Body", "Cancel", "OK")
	m.termWidth = 80
	m.termHeight = 40

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if m.active {
		t.Error("modal should be inactive after 'n'")
	}
	if cmd == nil {
		t.Fatal("'n' should return a command")
	}
	msg := cmd()
	if _, ok := msg.(modalCloseMsg); !ok {
		t.Errorf("'n' should return modalCloseMsg, got %T", msg)
	}
}

func TestYNShortcutsIgnoredForNonConfirmModals(t *testing.T) {
	m := newWarningModal("Warn", "Body", "Dismiss", "OK")
	m.termWidth = 80
	m.termHeight = 40

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if !m2.active {
		t.Error("'y' should not close a warning modal")
	}
	if cmd != nil {
		t.Error("'y' should not produce a command for warning modal")
	}
}

func TestScrollIndicatorsAppear(t *testing.T) {
	// Create a body with many lines to force scrolling
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line content"
	}
	body := strings.Join(lines, "\n")

	m := newConfirmModal("Scroll Test", body, "Cancel", "OK")
	m.termWidth = 80
	m.termHeight = 20 // small terminal to force overflow

	view := m.View()
	// At offset 0, there should be no "above" indicator but there should be "below"
	if strings.Contains(view, "more above") {
		t.Error("should not show 'more above' at offset 0")
	}
	if !strings.Contains(view, "more below") {
		t.Error("should show 'more below' when body overflows")
	}

	// Scroll down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	view = m.View()
	if !strings.Contains(view, "more above") {
		t.Error("should show 'more above' after scrolling down")
	}
}

func TestRenderButtons(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		left   string
		right  string
	}{
		{"left active", 0, "Cancel", "OK"},
		{"right active", 1, "Cancel", "OK"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderButtons(tt.left, tt.right, tt.cursor, 50)
			if !strings.Contains(result, tt.left) {
				t.Errorf("should contain left button %q", tt.left)
			}
			if !strings.Contains(result, tt.right) {
				t.Errorf("should contain right button %q", tt.right)
			}
		})
	}
}

func TestWizardSpaceTogglesCheckbox(t *testing.T) {
	opts := []string{"Option A", "Option B"}
	m := newWizardModal("Test", 1, 2, opts)
	m.termWidth = 80
	m.termHeight = 40

	// Toggle first option
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if !m.selected[0] {
		t.Error("space should toggle option 0 to selected")
	}

	// Toggle again to deselect
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if m.selected[0] {
		t.Error("space should toggle option 0 back to unselected")
	}
}

func TestWizardUpDownNavigatesOptions(t *testing.T) {
	opts := []string{"A", "B", "C"}
	m := newWizardModal("Test", 1, 1, opts)
	m.termWidth = 80
	m.termHeight = 40

	if m.optionCursor != 0 {
		t.Error("option cursor should start at 0")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.optionCursor != 1 {
		t.Errorf("option cursor should be 1 after Down, got %d", m.optionCursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.optionCursor != 2 {
		t.Errorf("option cursor should be 2 after second Down, got %d", m.optionCursor)
	}

	// Should not go past end
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.optionCursor != 2 {
		t.Errorf("option cursor should stay at 2, got %d", m.optionCursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.optionCursor != 1 {
		t.Errorf("option cursor should be 1 after Up, got %d", m.optionCursor)
	}
}

func TestInactiveModalReturnsEmpty(t *testing.T) {
	m := newConfirmModal("Test", "Body", "Cancel", "OK")
	m.active = false

	view := m.View()
	if view != "" {
		t.Error("inactive modal should return empty string")
	}
}

func TestInactiveModalIgnoresInput(t *testing.T) {
	m := newConfirmModal("Test", "Body", "Cancel", "OK")
	m.active = false

	m2, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("inactive modal should not return a command")
	}
	if m2.active {
		t.Error("inactive modal should stay inactive")
	}
}

func TestCopyReturnsModalCopyMsg(t *testing.T) {
	m := newConfirmModal("Test", "Some body text", "Cancel", "OK")
	m.termWidth = 80
	m.termHeight = 40

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd == nil {
		t.Fatal("'c' should return a command")
	}
	msg := cmd()
	copyMsg, ok := msg.(modalCopyMsg)
	if !ok {
		t.Fatalf("'c' should return modalCopyMsg, got %T", msg)
	}
	if copyMsg.text != "Some body text" {
		t.Errorf("copy text should be body, got %q", copyMsg.text)
	}
}

func TestConfirmModalEnterReturnsSelections(t *testing.T) {
	opts := []string{"A", "B"}
	m := newWizardModal("Test", 1, 1, opts)
	m.termWidth = 80
	m.termHeight = 40

	// Select first option, move to second
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should return a command")
	}
	msg := cmd()
	confirm, ok := msg.(modalConfirmMsg)
	if !ok {
		t.Fatalf("should return modalConfirmMsg, got %T", msg)
	}
	if !confirm.selections[0] {
		t.Error("option 0 should be selected in confirm msg")
	}
	if confirm.selections[1] {
		t.Error("option 1 should not be selected in confirm msg")
	}
	if confirm.optionIndex != 1 {
		t.Errorf("optionIndex should be 1 (cursor position), got %d", confirm.optionIndex)
	}
}
