package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestEditModal_OpenClose(t *testing.T) {
	m := newEditModal()
	if m.active {
		t.Fatal("modal should not be active initially")
	}

	m.Open("Edit: my-skill", "My Skill", "Does cool things", "/path/to/item")
	if !m.active {
		t.Fatal("modal should be active after Open")
	}
	if m.title != "Edit: my-skill" {
		t.Errorf("expected title 'Edit: my-skill', got %q", m.title)
	}
	if m.name != "My Skill" {
		t.Errorf("expected name 'My Skill', got %q", m.name)
	}
	if m.description != "Does cool things" {
		t.Errorf("expected description 'Does cool things', got %q", m.description)
	}
	if m.cursor != 8 {
		t.Errorf("expected cursor at 8, got %d", m.cursor)
	}

	m.Close()
	if m.active {
		t.Fatal("modal should not be active after Close")
	}
	if m.name != "" || m.description != "" {
		t.Errorf("expected empty fields after Close, got name=%q desc=%q", m.name, m.description)
	}
}

func TestEditModal_EscCancels(t *testing.T) {
	m := newEditModal()
	m.Open("Test", "name", "desc", "ctx")

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.active {
		t.Fatal("modal should be inactive after Esc")
	}
	if cmd == nil {
		t.Fatal("expected a command from Esc")
	}
	msg := cmd()
	if _, ok := msg.(editCancelledMsg); !ok {
		t.Fatalf("expected editCancelledMsg, got %T", msg)
	}
}

func TestEditModal_CtrlSSaves(t *testing.T) {
	m := newEditModal()
	m.Open("Test", "my name", "my desc", "/path")

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.active {
		t.Fatal("modal should be inactive after Ctrl+S")
	}
	if cmd == nil {
		t.Fatal("expected a command from Ctrl+S")
	}
	msg := cmd()
	saved, ok := msg.(editSavedMsg)
	if !ok {
		t.Fatalf("expected editSavedMsg, got %T", msg)
	}
	if saved.name != "my name" {
		t.Errorf("expected name 'my name', got %q", saved.name)
	}
	if saved.description != "my desc" {
		t.Errorf("expected description 'my desc', got %q", saved.description)
	}
	if saved.path != "/path" {
		t.Errorf("expected path '/path', got %q", saved.path)
	}
}

func TestEditModal_EnterMovesToNextField(t *testing.T) {
	m := newEditModal()
	m.Open("Test", "name", "desc", "ctx")

	// Focus on name field (default), Enter should move to description
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("Enter in name field should not produce a command")
	}
	if m.focusIdx != 1 {
		t.Errorf("expected focus on description (1), got %d", m.focusIdx)
	}

	// Enter on description should move to Save button
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("Enter in description field should not produce a command")
	}
	if m.focusIdx != 3 {
		t.Errorf("expected focus on Save (3), got %d", m.focusIdx)
	}

	// Enter on Save button should save
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.active {
		t.Fatal("modal should be inactive after Enter on Save")
	}
	if cmd == nil {
		t.Fatal("expected command from Save")
	}
	msg := cmd()
	if _, ok := msg.(editSavedMsg); !ok {
		t.Fatalf("expected editSavedMsg, got %T", msg)
	}
}

func TestEditModal_CancelButtonEnter(t *testing.T) {
	m := newEditModal()
	m.Open("Test", "name", "desc", "ctx")
	m.focusIdx = 2 // Cancel button

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.active {
		t.Fatal("modal should be inactive after Enter on Cancel")
	}
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	if _, ok := msg.(editCancelledMsg); !ok {
		t.Fatalf("expected editCancelledMsg, got %T", msg)
	}
}

func TestEditModal_TypingInNameField(t *testing.T) {
	m := newEditModal()
	m.Open("Test", "", "desc", "ctx")

	for _, ch := range "abc" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if m.name != "abc" {
		t.Errorf("expected name 'abc', got %q", m.name)
	}
	if m.cursor != 3 {
		t.Errorf("expected cursor at 3, got %d", m.cursor)
	}

	// Backspace
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.name != "ab" {
		t.Errorf("expected name 'ab' after backspace, got %q", m.name)
	}
}

func TestEditModal_TypingInDescField(t *testing.T) {
	m := newEditModal()
	m.Open("Test", "name", "", "ctx")
	m.focusIdx = 1 // Description field
	m.cursor = 0

	for _, ch := range "xyz" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if m.description != "xyz" {
		t.Errorf("expected description 'xyz', got %q", m.description)
	}
	// Name should be unchanged
	if m.name != "name" {
		t.Errorf("name should be unchanged, got %q", m.name)
	}
}

func TestEditModal_TypingOnlyInTextFields(t *testing.T) {
	m := newEditModal()
	m.Open("Test", "name", "desc", "ctx")
	m.focusIdx = 2 // Cancel button

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if m.name != "name" || m.description != "desc" {
		t.Error("typing on button should not change any field value")
	}
}

func TestEditModal_TabCyclesFocus(t *testing.T) {
	m := newEditModal()
	m.Open("Test", "name", "desc", "ctx")

	if m.focusIdx != 0 {
		t.Fatalf("expected initial focus 0, got %d", m.focusIdx)
	}

	// Tab cycles: 0(name) -> 1(desc) -> 2(cancel) -> 3(save) -> 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusIdx != 1 {
		t.Errorf("expected focus 1, got %d", m.focusIdx)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusIdx != 2 {
		t.Errorf("expected focus 2, got %d", m.focusIdx)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusIdx != 3 {
		t.Errorf("expected focus 3, got %d", m.focusIdx)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusIdx != 0 {
		t.Errorf("expected focus 0, got %d", m.focusIdx)
	}

	// Shift+Tab goes backwards
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.focusIdx != 3 {
		t.Errorf("expected focus 3 after shift-tab, got %d", m.focusIdx)
	}
}

func TestEditModal_CursorMovement(t *testing.T) {
	m := newEditModal()
	m.Open("Test", "hello", "desc", "ctx")

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

	// Right at end doesn't move
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

func TestEditModal_InsertInMiddle(t *testing.T) {
	m := newEditModal()
	m.Open("Test", "hllo", "desc", "ctx")
	m.cursor = 1

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if m.name != "hello" {
		t.Errorf("expected 'hello', got %q", m.name)
	}
	if m.cursor != 2 {
		t.Errorf("expected cursor at 2, got %d", m.cursor)
	}
}

func TestEditModal_DeleteKey(t *testing.T) {
	m := newEditModal()
	m.Open("Test", "hello", "desc", "ctx")
	m.cursor = 0

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	if m.name != "ello" {
		t.Errorf("expected 'ello', got %q", m.name)
	}
}

func TestEditModal_InactiveIgnoresInput(t *testing.T) {
	m := newEditModal()

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("inactive modal should not produce commands")
	}
}

func TestEditModal_ViewContainsBothLabels(t *testing.T) {
	m := newEditModal()
	m.Open("Edit: my-hook", "My Hook", "Runs tests after edits", "ctx")

	view := m.View()
	stripped := ansi.Strip(view)
	if !contains(stripped, "Edit: my-hook") {
		t.Errorf("view should contain title 'Edit: my-hook', got:\n%s", stripped)
	}
	if !contains(stripped, "Display Name") {
		t.Error("view should contain 'Display Name' label")
	}
	if !contains(stripped, "Description") {
		t.Error("view should contain 'Description' label")
	}
	if !contains(stripped, "Cancel") {
		t.Error("view should contain 'Cancel' button")
	}
	if !contains(stripped, "Save") {
		t.Error("view should contain 'Save' button")
	}
}

func TestEditModal_LayoutConsistency(t *testing.T) {
	m := newEditModal()
	m.Open("Edit: my-hook", "wizard-invariant-gate", "Runs after edits", "ctx")

	view := m.View()
	stripped := ansi.Strip(view)
	lines := strings.Split(stripped, "\n")

	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}
	borderW := len([]rune(lines[0]))
	for i, line := range lines {
		runeW := len([]rune(line))
		if runeW != borderW {
			t.Errorf("line %d width %d differs from border width %d: %q", i, runeW, borderW, line)
		}
	}

	// Buttons should appear on the same line
	foundButtons := false
	for _, line := range lines {
		if strings.Contains(line, "Cancel") && strings.Contains(line, "Save") {
			foundButtons = true
		}
	}
	if !foundButtons {
		t.Error("Cancel and Save buttons should be on the same line")
	}
}

func TestEditModal_UpDownNavigatesFields(t *testing.T) {
	m := newEditModal()
	m.Open("Test", "name", "description text", "ctx")

	// Start on name field (0), Down → description (1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.focusIdx != 1 {
		t.Errorf("expected focus on description (1), got %d", m.focusIdx)
	}
	// Cursor preserves column position: was at 4 (end of "name"), so stays at 4 in description

	// Move to end of description field first
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})

	// Type into description to verify it's editable
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}})
	if m.description != "description text!" {
		t.Errorf("expected 'description text!', got %q", m.description)
	}

	// Backspace in description
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.description != "description text" {
		t.Errorf("expected 'description text', got %q", m.description)
	}

	// Up → back to name (0)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.focusIdx != 0 {
		t.Errorf("expected focus on name (0), got %d", m.focusIdx)
	}

	// Down from description → Save button
	m.focusIdx = 1
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.focusIdx != 3 {
		t.Errorf("expected focus on Save (3), got %d", m.focusIdx)
	}

	// Up from buttons → description
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.focusIdx != 1 {
		t.Errorf("expected focus on description (1), got %d", m.focusIdx)
	}
}

func TestEditModal_LeftRightOnButtons(t *testing.T) {
	m := newEditModal()
	m.Open("Test", "name", "desc", "ctx")
	m.focusIdx = 2 // Cancel button

	// Right → Save
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.focusIdx != 3 {
		t.Errorf("expected Save (3), got %d", m.focusIdx)
	}

	// Left → Cancel
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.focusIdx != 2 {
		t.Errorf("expected Cancel (2), got %d", m.focusIdx)
	}
}

func TestEditModal_FullEditFlow(t *testing.T) {
	// Simulate the exact user flow: open → down to description → edit → save
	m := newEditModal()
	m.Open("Edit: my-skill", "My Skill", "Old description", "/path")

	// Down arrow to description field
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.focusIdx != 1 {
		t.Fatal("expected description field focus after Down")
	}

	// Select all (Home then shift-delete style: go to start, delete forward)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	for range len("Old description") {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	}
	if m.description != "" {
		t.Errorf("expected empty description after deleting all, got %q", m.description)
	}

	// Type new description
	for _, ch := range "New description" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if m.description != "New description" {
		t.Errorf("expected 'New description', got %q", m.description)
	}

	// Ctrl+S to save
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.active {
		t.Fatal("modal should be inactive after save")
	}
	msg := cmd()
	saved, ok := msg.(editSavedMsg)
	if !ok {
		t.Fatalf("expected editSavedMsg, got %T", msg)
	}
	if saved.name != "My Skill" {
		t.Errorf("name should be unchanged: got %q", saved.name)
	}
	if saved.description != "New description" {
		t.Errorf("expected 'New description', got %q", saved.description)
	}
}

func TestEditModal_SpacesInFields(t *testing.T) {
	m := newEditModal()
	m.Open("Test", "", "", "ctx")

	// Type "hello world" with a space in the name field
	for _, ch := range "hello" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	for _, ch := range "world" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if m.name != "hello world" {
		t.Errorf("expected 'hello world', got %q", m.name)
	}

	// Move to description field and type with spaces
	m.focusIdx = 1
	m.cursor = 0
	for _, ch := range "foo" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	for _, ch := range "bar" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if m.description != "foo bar" {
		t.Errorf("expected 'foo bar', got %q", m.description)
	}

	// Space on button fields should NOT modify text
	m.focusIdx = 2
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.name != "hello world" || m.description != "foo bar" {
		t.Error("space on button should not modify fields")
	}
}

func TestApp_EditDescriptionViaAllNavigationMethods(t *testing.T) {
	app := testAppWithItems(t)

	// Method 1: Tab to description
	m, _ := app.Update(keyRune('e'))
	a := m.(App)
	m, _ = a.Update(keyPress(tea.KeyTab))
	a = m.(App)
	if a.modal.focusIdx != 1 {
		t.Fatalf("Tab: expected focus on description (1), got %d", a.modal.focusIdx)
	}
	m, _ = a.Update(keyRune('x'))
	a = m.(App)
	if !strings.Contains(a.modal.description, "x") {
		t.Errorf("Tab then type: expected 'x' in description, got %q", a.modal.description)
	}
	a.modal.Close()

	// Method 2: Enter advances to description
	a.modal.Open("Test", "name", "original", "/path")
	m, _ = a.Update(keyPress(tea.KeyEnter))
	a = m.(App)
	if a.modal.focusIdx != 1 {
		t.Fatalf("Enter: expected focus on description (1), got %d", a.modal.focusIdx)
	}
	m, _ = a.Update(keyRune('Z'))
	a = m.(App)
	if !strings.Contains(a.modal.description, "Z") {
		t.Errorf("Enter then type: expected 'Z' in description %q", a.modal.description)
	}
	a.modal.Close()

	// Method 3: Down arrow to description
	a.modal.Open("Test", "name", "original", "/path")
	m, _ = a.Update(keyPress(tea.KeyDown))
	a = m.(App)
	if a.modal.focusIdx != 1 {
		t.Fatalf("Down: expected focus on description (1), got %d", a.modal.focusIdx)
	}
	m, _ = a.Update(keyRune('W'))
	a = m.(App)
	if !strings.Contains(a.modal.description, "W") {
		t.Errorf("Down then type: expected 'W' in description %q", a.modal.description)
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
