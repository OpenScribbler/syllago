package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestTextInputModal_OpenClose(t *testing.T) {
	m := newTextInputModal()
	if m.active {
		t.Fatal("modal should not be active initially")
	}

	m.Open("Rename", "old-name", "/path/to/item")
	if !m.active {
		t.Fatal("modal should be active after Open")
	}
	if m.title != "Rename" {
		t.Errorf("expected title 'Rename', got %q", m.title)
	}
	if m.value != "old-name" {
		t.Errorf("expected value 'old-name', got %q", m.value)
	}
	if m.cursor != 8 {
		t.Errorf("expected cursor at 8, got %d", m.cursor)
	}

	m.Close()
	if m.active {
		t.Fatal("modal should not be active after Close")
	}
	if m.value != "" {
		t.Errorf("expected empty value after Close, got %q", m.value)
	}
}

func TestTextInputModal_EscCancels(t *testing.T) {
	m := newTextInputModal()
	m.Open("Test", "hello", "ctx")

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.active {
		t.Fatal("modal should be inactive after Esc")
	}
	if cmd == nil {
		t.Fatal("expected a command from Esc")
	}
	msg := cmd()
	if _, ok := msg.(modalCancelledMsg); !ok {
		t.Fatalf("expected modalCancelledMsg, got %T", msg)
	}
}

func TestTextInputModal_EnterSaves(t *testing.T) {
	m := newTextInputModal()
	m.Open("Test", "my value", "my-context")

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.active {
		t.Fatal("modal should be inactive after Enter")
	}
	if cmd == nil {
		t.Fatal("expected a command from Enter")
	}
	msg := cmd()
	saved, ok := msg.(modalSavedMsg)
	if !ok {
		t.Fatalf("expected modalSavedMsg, got %T", msg)
	}
	if saved.value != "my value" {
		t.Errorf("expected saved value 'my value', got %q", saved.value)
	}
	if saved.context != "my-context" {
		t.Errorf("expected context 'my-context', got %q", saved.context)
	}
}

func TestTextInputModal_Typing(t *testing.T) {
	m := newTextInputModal()
	m.Open("Test", "", "ctx")

	// Type "abc"
	for _, ch := range "abc" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if m.value != "abc" {
		t.Errorf("expected 'abc', got %q", m.value)
	}
	if m.cursor != 3 {
		t.Errorf("expected cursor at 3, got %d", m.cursor)
	}

	// Backspace removes last char
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.value != "ab" {
		t.Errorf("expected 'ab' after backspace, got %q", m.value)
	}
}

func TestTextInputModal_CursorMovement(t *testing.T) {
	m := newTextInputModal()
	m.Open("Test", "hello", "ctx")

	// Left arrow
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.cursor != 4 {
		t.Errorf("expected cursor at 4, got %d", m.cursor)
	}

	// Home goes to start
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}

	// End goes to end
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.cursor != 5 {
		t.Errorf("expected cursor at 5, got %d", m.cursor)
	}

	// Right arrow at end doesn't move
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.cursor != 5 {
		t.Errorf("expected cursor at 5, got %d", m.cursor)
	}

	// Left at 0 doesn't go negative
	m.cursor = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}
}

func TestTextInputModal_TabCyclesFocus(t *testing.T) {
	m := newTextInputModal()
	m.Open("Test", "hello", "ctx")

	if m.focusIdx != 0 {
		t.Fatalf("expected initial focus 0, got %d", m.focusIdx)
	}

	// Tab cycles: 0 -> 1 -> 2 -> 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusIdx != 1 {
		t.Errorf("expected focus 1, got %d", m.focusIdx)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusIdx != 2 {
		t.Errorf("expected focus 2, got %d", m.focusIdx)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusIdx != 0 {
		t.Errorf("expected focus 0, got %d", m.focusIdx)
	}

	// Shift+Tab goes backwards
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.focusIdx != 2 {
		t.Errorf("expected focus 2 after shift-tab, got %d", m.focusIdx)
	}
}

func TestTextInputModal_CancelButtonEnter(t *testing.T) {
	m := newTextInputModal()
	m.Open("Test", "value", "ctx")
	m.focusIdx = 1 // Cancel button

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.active {
		t.Fatal("modal should be inactive after Enter on Cancel")
	}
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	if _, ok := msg.(modalCancelledMsg); !ok {
		t.Fatalf("expected modalCancelledMsg, got %T", msg)
	}
}

func TestTextInputModal_TypingOnlyInTextField(t *testing.T) {
	m := newTextInputModal()
	m.Open("Test", "hello", "ctx")
	m.focusIdx = 1 // Cancel button

	// Typing should not modify value when not focused on text field
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.value != "hello" {
		t.Errorf("typing on Cancel button should not change value, got %q", m.value)
	}
}

func TestTextInputModal_InactiveIgnoresInput(t *testing.T) {
	m := newTextInputModal()

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("inactive modal should not produce commands")
	}
}

func TestTextInputModal_LayoutConsistency(t *testing.T) {
	m := newTextInputModal()
	m.Open("Rename: my-hook", "wizard-invariant-gate", "ctx")

	view := m.View()
	stripped := ansi.Strip(view)
	lines := strings.Split(stripped, "\n")

	// All lines within the border should be the same visual width
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
	borderW := len([]rune(lines[0])) // top border sets the width
	for i, line := range lines {
		runeW := len([]rune(line))
		if runeW != borderW {
			t.Errorf("line %d width %d differs from border width %d: %q", i, runeW, borderW, line)
		}
	}

	// Buttons should appear on the same line
	foundCancel, foundSave := false, false
	for _, line := range lines {
		if strings.Contains(line, "Cancel") && strings.Contains(line, "Save") {
			foundCancel = true
			foundSave = true
		}
	}
	if !foundCancel || !foundSave {
		t.Error("Cancel and Save buttons should be on the same line")
	}

	// Only border lines should have many ─ characters (top + bottom = 2)
	dashLineCount := 0
	for _, line := range lines {
		dashes := strings.Count(line, "─")
		if dashes > 20 {
			dashLineCount++
		}
	}
	if dashLineCount != 2 {
		t.Errorf("expected 2 lines with dashes (borders only), got %d", dashLineCount)
	}
}

func TestTextInputModal_ViewContainsTitle(t *testing.T) {
	m := newTextInputModal()
	m.Open("Rename Hook", "old-name", "ctx")

	view := m.View()
	stripped := ansi.Strip(view)
	if !contains(stripped, "Rename Hook") {
		t.Errorf("view should contain title 'Rename Hook', got:\n%s", stripped)
	}
	if !contains(stripped, "Cancel") {
		t.Errorf("view should contain 'Cancel' button")
	}
	if !contains(stripped, "Save") {
		t.Errorf("view should contain 'Save' button")
	}
}

func TestTextInputModal_InsertInMiddle(t *testing.T) {
	m := newTextInputModal()
	m.Open("Test", "hllo", "ctx")
	m.cursor = 1 // Position after 'h'

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.value != "hello" {
		t.Errorf("expected 'hello', got %q", m.value)
	}
	if m.cursor != 2 {
		t.Errorf("expected cursor at 2, got %d", m.cursor)
	}
}

func TestTextInputModal_DeleteKey(t *testing.T) {
	m := newTextInputModal()
	m.Open("Test", "hello", "ctx")
	m.cursor = 0

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	if m.value != "ello" {
		t.Errorf("expected 'ello', got %q", m.value)
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}
}

func contains(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && len(s) >= len(sub) && indexStr(s, sub) >= 0
}

func indexStr(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
