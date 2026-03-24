package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDropdown_OpenClose(t *testing.T) {
	d := newDropdown("test", "Test", []string{"A", "B", "C"})
	d.SetActive(0)

	if d.isOpen {
		t.Fatal("dropdown should start closed")
	}

	d.Open()
	if !d.isOpen {
		t.Fatal("dropdown should be open after Open()")
	}
	if d.cursor != 0 {
		t.Errorf("cursor should be at active (0), got %d", d.cursor)
	}

	d.Close()
	if d.isOpen {
		t.Fatal("dropdown should be closed after Close()")
	}
}

func TestDropdown_KeyNavigation(t *testing.T) {
	d := newDropdown("test", "Test", []string{"A", "B", "C"})
	d.SetActive(0)
	d.Open()

	// j moves down
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if d.cursor != 1 {
		t.Errorf("j should move cursor to 1, got %d", d.cursor)
	}

	// j again
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if d.cursor != 2 {
		t.Errorf("cursor should be 2, got %d", d.cursor)
	}

	// j wraps around
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if d.cursor != 0 {
		t.Errorf("cursor should wrap to 0, got %d", d.cursor)
	}

	// k wraps backward
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if d.cursor != 2 {
		t.Errorf("k from 0 should wrap to 2, got %d", d.cursor)
	}
}

func TestDropdown_EnterSelects(t *testing.T) {
	d := newDropdown("test", "Test", []string{"A", "B", "C"})
	d.SetActive(0)
	d.Open()

	// Move to B and select
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	d, cmd := d.Update(keyEnter)

	if d.isOpen {
		t.Fatal("enter should close the dropdown")
	}
	if d.active != 1 {
		t.Errorf("active should be 1 (B), got %d", d.active)
	}
	if cmd == nil {
		t.Fatal("enter should produce an activeCmd")
	}

	// Verify the command produces a dropdownActiveMsg
	msg := cmd()
	activeMsg, ok := msg.(dropdownActiveMsg)
	if !ok {
		t.Fatalf("expected dropdownActiveMsg, got %T", msg)
	}
	if activeMsg.index != 1 || activeMsg.label != "B" {
		t.Errorf("expected index=1 label=B, got index=%d label=%s", activeMsg.index, activeMsg.label)
	}
}

func TestDropdown_EscCancels(t *testing.T) {
	d := newDropdown("test", "Test", []string{"A", "B", "C"})
	d.SetActive(0)
	d.Open()

	// Move cursor then esc — active should NOT change
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	d, _ = d.Update(keyEsc)

	if d.isOpen {
		t.Fatal("esc should close the dropdown")
	}
	if d.active != 0 {
		t.Errorf("esc should not change active, expected 0 got %d", d.active)
	}
}

func TestDropdown_ActiveLabel(t *testing.T) {
	d := newDropdown("test", "Test", []string{"A", "B", "C"})

	d.Reset()
	if got := d.ActiveLabel(); got != "--" {
		t.Errorf("expected '--' for reset, got %q", got)
	}

	d.SetActive(2)
	if got := d.ActiveLabel(); got != "C" {
		t.Errorf("expected 'C', got %q", got)
	}
}

func TestDropdown_DisabledRendersAsStyle(t *testing.T) {
	d := newDropdown("test", "Test", []string{"A"})
	d.SetActive(0)
	d.disabled = true

	trigger := d.ViewTrigger()
	// Should contain the label text even when disabled
	assertContains(t, trigger, "Test: A")
}

func TestDropdown_ViewMenu(t *testing.T) {
	d := newDropdown("test", "Test", []string{"Alpha", "Beta"})
	d.SetActive(0)
	d.Open()

	menu := d.ViewMenu()
	assertContains(t, menu, "> Alpha")
	assertContains(t, menu, "  Beta")
}

func TestDropdown_ClosedViewMenuEmpty(t *testing.T) {
	d := newDropdown("test", "Test", []string{"Alpha"})
	if menu := d.ViewMenu(); menu != "" {
		t.Errorf("closed dropdown should return empty menu, got %q", menu)
	}
}
