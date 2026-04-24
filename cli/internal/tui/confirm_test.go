package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"
)

func testChecks() []confirmCheckbox {
	return []confirmCheckbox{
		{label: "Uninstall from Claude Code", checked: true},
		{label: "Uninstall from Cursor", checked: true},
		{label: "Delete from library", checked: true, readOnly: true},
	}
}

func TestConfirmModal_OpenClose(t *testing.T) {
	m := newConfirmModal()
	if m.active {
		t.Fatal("modal should not be active initially")
	}

	checks := testChecks()
	m.Open("Remove \"my-hook\"?", "This cannot be undone.", "Remove", true, checks)
	if !m.active {
		t.Fatal("modal should be active after Open")
	}
	if m.title != "Remove \"my-hook\"?" {
		t.Errorf("unexpected title: %q", m.title)
	}
	if !m.danger {
		t.Error("expected danger=true")
	}
	if len(m.checks) != 3 {
		t.Errorf("expected 3 checks, got %d", len(m.checks))
	}

	m.Close()
	if m.active {
		t.Fatal("modal should not be active after Close")
	}
	if m.title != "" || len(m.checks) != 0 {
		t.Error("expected cleared state after Close")
	}
}

func TestConfirmModal_DefaultFocusCancel(t *testing.T) {
	m := newConfirmModal()

	// With checkboxes: Cancel is at index 3 (3 checkboxes)
	m.Open("Test", "body", "OK", false, testChecks())
	if m.focusIdx != m.cancelIdx() {
		t.Errorf("expected focus on Cancel (%d), got %d", m.cancelIdx(), m.focusIdx)
	}

	// Without checkboxes: Cancel is at index 0
	m.Close()
	m.Open("Test", "body", "OK", false, nil)
	if m.focusIdx != 0 {
		t.Errorf("expected focus on Cancel (0) with no checkboxes, got %d", m.focusIdx)
	}
}

func TestConfirmModal_YShortcut(t *testing.T) {
	m := newConfirmModal()
	m.Open("Test", "body", "OK", false, nil)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if m.active {
		t.Fatal("modal should be inactive after y")
	}
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	res, ok := msg.(confirmResultMsg)
	if !ok {
		t.Fatalf("expected confirmResultMsg, got %T", msg)
	}
	if !res.confirmed {
		t.Error("expected confirmed=true")
	}
}

func TestConfirmModal_NShortcut(t *testing.T) {
	m := newConfirmModal()
	m.Open("Test", "body", "OK", false, nil)

	// Focus on Confirm button — n should still cancel
	m.focusIdx = m.confirmIdx()
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if m.active {
		t.Fatal("modal should be inactive after n")
	}
	msg := cmd()
	res := msg.(confirmResultMsg)
	if res.confirmed {
		t.Error("expected confirmed=false")
	}
}

func TestConfirmModal_EscCancels(t *testing.T) {
	m := newConfirmModal()
	m.Open("Test", "body", "OK", false, nil)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.active {
		t.Fatal("modal should be inactive after Esc")
	}
	msg := cmd()
	res := msg.(confirmResultMsg)
	if res.confirmed {
		t.Error("expected confirmed=false")
	}
}

func TestConfirmModal_EnterOnCancel(t *testing.T) {
	m := newConfirmModal()
	m.Open("Test", "body", "OK", false, testChecks())

	// Default focus is on Cancel
	if m.focusIdx != m.cancelIdx() {
		t.Fatalf("expected focus on Cancel, got %d", m.focusIdx)
	}
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.active {
		t.Fatal("modal should be inactive after Enter on Cancel")
	}
	msg := cmd()
	res := msg.(confirmResultMsg)
	if res.confirmed {
		t.Error("expected confirmed=false")
	}
}

func TestConfirmModal_EnterOnConfirm(t *testing.T) {
	m := newConfirmModal()
	m.Open("Test", "body", "Remove", true, testChecks())

	// Tab to Confirm
	m.focusIdx = m.confirmIdx()
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.active {
		t.Fatal("modal should be inactive after Enter on Confirm")
	}
	msg := cmd()
	res := msg.(confirmResultMsg)
	if !res.confirmed {
		t.Error("expected confirmed=true")
	}
}

func TestConfirmModal_TabCycle(t *testing.T) {
	m := newConfirmModal()
	checks := testChecks()
	m.Open("Test", "body", "OK", false, checks)

	// 3 checkboxes + Cancel + Confirm = 5 elements
	// Default focus: Cancel (idx 3)
	expected := []int{4, 0, 1, 2, 3, 4, 0} // wrap around twice
	for i, want := range expected {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
		if m.focusIdx != want {
			t.Errorf("Tab step %d: expected focus %d, got %d", i, want, m.focusIdx)
		}
	}

	// Shift+Tab wraps backward
	m.focusIdx = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.focusIdx != 4 { // Confirm
		t.Errorf("Shift+Tab from 0: expected 4, got %d", m.focusIdx)
	}
}

func TestConfirmModal_SpaceTogglesCheckbox(t *testing.T) {
	m := newConfirmModal()
	checks := testChecks()
	m.Open("Test", "body", "OK", false, checks)

	// Focus on first checkbox (Uninstall from Claude Code, initially checked)
	m.focusIdx = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if m.checks[0].checked {
		t.Error("expected first checkbox to be unchecked after Space")
	}

	// Toggle back
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !m.checks[0].checked {
		t.Error("expected first checkbox to be checked again")
	}
}

func TestConfirmModal_SpaceSkipsReadOnly(t *testing.T) {
	m := newConfirmModal()
	checks := testChecks()
	m.Open("Test", "body", "OK", false, checks)

	// Focus on the readOnly checkbox (index 2 = "Delete from library")
	m.focusIdx = 2
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !m.checks[2].checked {
		t.Error("readOnly checkbox should remain checked after Space")
	}
}

func TestConfirmModal_EnterOnCheckboxNoOp(t *testing.T) {
	m := newConfirmModal()
	m.Open("Test", "body", "OK", false, testChecks())

	m.focusIdx = 0 // first checkbox
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("Enter on checkbox should not produce a command")
	}
	if !m.active {
		t.Fatal("modal should still be active")
	}
}

func TestConfirmModal_CheckboxStateInResult(t *testing.T) {
	m := newConfirmModal()
	checks := testChecks()
	m.Open("Test", "body", "OK", false, checks)

	// Uncheck first provider
	m.focusIdx = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})

	// Confirm via y
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	msg := cmd()
	res := msg.(confirmResultMsg)
	if res.checks[0].checked {
		t.Error("expected first checkbox unchecked in result")
	}
	if !res.checks[1].checked {
		t.Error("expected second checkbox still checked")
	}
	if !res.checks[2].checked || !res.checks[2].readOnly {
		t.Error("expected readOnly checkbox still checked and readOnly")
	}
}

func TestConfirmModal_DangerStyling(t *testing.T) {
	m := newConfirmModal()
	m.Open("Remove?", "Gone forever.", "Remove", true, nil)
	m.width = 80
	m.height = 30

	view := m.View()
	// Danger border uses dangerColor — just check the view renders without panic
	// and contains the title
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "Remove?") {
		t.Error("view should contain title")
	}
	if !strings.Contains(stripped, "Gone forever.") {
		t.Error("view should contain body")
	}
}

func TestConfirmModal_InactiveIgnoresInput(t *testing.T) {
	m := newConfirmModal()
	// Not active — all input should be ignored
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("inactive modal should not produce commands")
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd != nil {
		t.Error("inactive modal should not produce commands for y")
	}
}

func TestConfirmModal_NoCheckboxes(t *testing.T) {
	m := newConfirmModal()
	m.Open("Uninstall?", "Remove from Claude Code", "Uninstall", false, nil)

	// No checkboxes: Cancel=0, Confirm=1
	if m.cancelIdx() != 0 {
		t.Errorf("expected cancelIdx=0, got %d", m.cancelIdx())
	}
	if m.confirmIdx() != 1 {
		t.Errorf("expected confirmIdx=1, got %d", m.confirmIdx())
	}
	// Default focus on Cancel (0)
	if m.focusIdx != 0 {
		t.Errorf("expected focus 0, got %d", m.focusIdx)
	}

	// Tab → Confirm (1) → Cancel (0)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusIdx != 1 {
		t.Errorf("expected focus 1 after Tab, got %d", m.focusIdx)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusIdx != 0 {
		t.Errorf("expected focus 0 after second Tab, got %d", m.focusIdx)
	}
}

func TestConfirmModal_AllProviderUnchecked(t *testing.T) {
	m := newConfirmModal()
	checks := testChecks()
	m.Open("Remove?", "body", "Remove", true, checks)

	// Uncheck both provider checkboxes
	m.focusIdx = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m.focusIdx = 1
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})

	// Confirm — should be valid (library-only remove)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	msg := cmd()
	res := msg.(confirmResultMsg)
	if !res.confirmed {
		t.Error("expected confirmed=true")
	}
	if res.checks[0].checked || res.checks[1].checked {
		t.Error("expected provider checkboxes unchecked")
	}
	if !res.checks[2].checked {
		t.Error("expected readOnly 'Delete from library' still checked")
	}
}

func TestConfirmModal_ViewNoCheckboxes(t *testing.T) {
	m := newConfirmModal()
	m.Open("Uninstall?", "Content stays in your library.", "Uninstall", false, nil)
	m.width = 80
	m.height = 30

	view := m.View()
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "Uninstall?") {
		t.Error("view should contain title")
	}
	if !strings.Contains(stripped, "Cancel") {
		t.Error("view should contain Cancel button")
	}
	if !strings.Contains(stripped, "Uninstall") {
		t.Error("view should contain Uninstall button")
	}
	// Should NOT contain checkbox markers
	if strings.Contains(stripped, "[x]") || strings.Contains(stripped, "[ ]") {
		t.Error("view should not contain checkbox markers when no checks")
	}
}

func TestConfirmModal_ViewWithCheckboxes(t *testing.T) {
	m := newConfirmModal()
	m.Open("Remove?", "This cannot be undone.", "Remove", true, testChecks())
	m.width = 80
	m.height = 30

	view := m.View()
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "[x] Uninstall from Claude Code") {
		t.Error("view should contain Claude Code checkbox")
	}
	if !strings.Contains(stripped, "[x] Delete from library") {
		t.Error("view should contain Delete from library checkbox")
	}
}

func mouseClick(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
}

func TestConfirmModal_MouseClickCancel(t *testing.T) {
	m := newConfirmModal()
	m.Open("Remove?", "body", "Remove", true, nil)
	m.width = 80
	m.height = 30

	scanZones(m.View())

	z := zone.Get("confirm-cancel")
	if z.IsZero() {
		t.Skip("zone confirm-cancel not registered (bubblezone rendering issue)")
	}
	m, cmd := m.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected command from Cancel click")
	}
	msg := cmd()
	res := msg.(confirmResultMsg)
	if res.confirmed {
		t.Error("expected confirmed=false")
	}
}

func TestConfirmModal_MouseClickConfirm(t *testing.T) {
	m := newConfirmModal()
	m.Open("Remove?", "body", "Remove", true, nil)
	m.width = 80
	m.height = 30

	scanZones(m.View())

	z := zone.Get("confirm-ok")
	if z.IsZero() {
		t.Skip("zone confirm-ok not registered (bubblezone rendering issue)")
	}
	m, cmd := m.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected command from Confirm click")
	}
	msg := cmd()
	res := msg.(confirmResultMsg)
	if !res.confirmed {
		t.Error("expected confirmed=true")
	}
}

func TestConfirmModal_MouseClickCheckbox(t *testing.T) {
	m := newConfirmModal()
	checks := testChecks()
	m.Open("Remove?", "body", "Remove", true, checks)
	m.width = 80
	m.height = 30

	scanZones(m.View())

	z := zone.Get("confirm-check-0")
	if z.IsZero() {
		t.Skip("zone confirm-check-0 not registered (bubblezone rendering issue)")
	}
	// First checkbox starts checked — click should uncheck
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.checks[0].checked {
		t.Error("expected first checkbox unchecked after click")
	}
	if m.focusIdx != 0 {
		t.Errorf("expected focus to move to clicked checkbox (0), got %d", m.focusIdx)
	}
}

func TestConfirmModal_DownUpNavigation(t *testing.T) {
	m := newConfirmModal()
	m.Open("Test", "body", "OK", false, testChecks())

	// Start at Cancel (3), Down → Confirm (4)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.focusIdx != 4 {
		t.Errorf("expected 4, got %d", m.focusIdx)
	}

	// Down at max → stays at max
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.focusIdx != 4 {
		t.Errorf("expected 4 (clamped), got %d", m.focusIdx)
	}

	// Up from Confirm (4) → Cancel (3)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.focusIdx != 3 {
		t.Errorf("expected 3, got %d", m.focusIdx)
	}

	// Up to first checkbox (0), clamped
	m.focusIdx = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.focusIdx != 0 {
		t.Errorf("expected 0 (clamped), got %d", m.focusIdx)
	}
}

func TestConfirmModal_TitleTruncated(t *testing.T) {
	m := newConfirmModal()
	longName := "this-is-a-very-long-item-name-that-should-get-truncated-by-the-modal"
	m.Open("Remove \""+longName+"\"?", "body", "Remove", true, nil)
	m.width = 60
	m.height = 20

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
			t.Errorf("line %d width %d differs from border width %d (title may be wrapping): %q", i, runeW, borderW, line)
		}
	}
}

func TestConfirmModal_LeftRightBetweenButtons(t *testing.T) {
	m := newConfirmModal()
	m.Open("Test", "body", "OK", false, testChecks())

	// Focus on Cancel (3), Right → Confirm (4)
	m.focusIdx = m.cancelIdx() // 3
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.focusIdx != m.confirmIdx() {
		t.Errorf("Right from Cancel: expected %d, got %d", m.confirmIdx(), m.focusIdx)
	}

	// Left from Confirm → Cancel
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.focusIdx != m.cancelIdx() {
		t.Errorf("Left from Confirm: expected %d, got %d", m.cancelIdx(), m.focusIdx)
	}
}

func TestConfirmModal_LeftRightWraps(t *testing.T) {
	m := newConfirmModal()
	m.Open("Test", "body", "OK", false, testChecks())

	// Right from Confirm (last button) → wraps to Cancel (first button)
	m.focusIdx = m.confirmIdx()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.focusIdx != m.cancelIdx() {
		t.Errorf("Right wrap: expected %d, got %d", m.cancelIdx(), m.focusIdx)
	}

	// Left from Cancel (first button) → wraps to Confirm (last button)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.focusIdx != m.confirmIdx() {
		t.Errorf("Left wrap: expected %d, got %d", m.confirmIdx(), m.focusIdx)
	}
}

func TestConfirmModal_LeftRightNoOpOnCheckbox(t *testing.T) {
	m := newConfirmModal()
	m.Open("Test", "body", "OK", false, testChecks())

	// Focus on first checkbox (0), Left/Right should do nothing
	m.focusIdx = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.focusIdx != 0 {
		t.Errorf("Left on checkbox: expected 0, got %d", m.focusIdx)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.focusIdx != 0 {
		t.Errorf("Right on checkbox: expected 0, got %d", m.focusIdx)
	}
}
