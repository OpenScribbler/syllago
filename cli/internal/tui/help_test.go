package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestHelpOverlay_ToggleOnOff(t *testing.T) {
	h := newHelpOverlay()
	if h.active {
		t.Fatal("help should not be active initially")
	}

	h.Toggle()
	if !h.active {
		t.Fatal("help should be active after Toggle")
	}

	h.Toggle()
	if h.active {
		t.Fatal("help should be inactive after second Toggle")
	}
}

func TestHelpOverlay_EscCloses(t *testing.T) {
	h := newHelpOverlay()
	h.SetSize(80, 30)
	h.active = true

	h, _ = h.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if h.active {
		t.Fatal("help should be inactive after Esc")
	}
}

func TestHelpOverlay_QuestionMarkCloses(t *testing.T) {
	h := newHelpOverlay()
	h.SetSize(80, 30)
	h.active = true

	h, _ = h.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if h.active {
		t.Fatal("help should be inactive after ?")
	}
}

func TestHelpOverlay_InactiveIgnoresInput(t *testing.T) {
	h := newHelpOverlay()
	h.SetSize(80, 30)

	h, cmd := h.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("inactive help should not produce commands")
	}
	if h.active {
		t.Fatal("help should remain inactive")
	}
}

func TestHelpOverlay_ViewContainsSections(t *testing.T) {
	h := newHelpOverlay()
	h.SetSize(100, 40)
	h.active = true

	view := h.View()
	stripped := ansi.Strip(view)

	for _, section := range []string{"Navigation", "Actions", "File Tree", "Gallery"} {
		if !strings.Contains(stripped, section) {
			t.Errorf("help view should contain section %q", section)
		}
	}

	for _, key := range []string{"Esc", "Enter", "Search"} {
		if !strings.Contains(stripped, key) {
			t.Errorf("help view should contain key %q", key)
		}
	}
}

func TestHelpOverlay_ViewContainsCloseButton(t *testing.T) {
	h := newHelpOverlay()
	h.SetSize(100, 40)
	h.active = true

	view := h.View()
	stripped := ansi.Strip(view)
	if !strings.Contains(stripped, "[Esc] Close") {
		t.Error("help view should contain '[Esc] Close' button")
	}
}

func TestHelpOverlay_InactiveViewEmpty(t *testing.T) {
	h := newHelpOverlay()
	h.SetSize(80, 30)

	if h.View() != "" {
		t.Error("inactive help should return empty view")
	}
}

func TestApp_HelpOverlayToggle(t *testing.T) {
	app := testAppWithItems(t)

	// Press '?' to open help
	m, _ := app.Update(keyRune('?'))
	a := m.(App)
	if !a.help.active {
		t.Fatal("expected help overlay to be active after '?'")
	}

	// '?' again to close
	m, _ = a.Update(keyRune('?'))
	a = m.(App)
	if a.help.active {
		t.Fatal("expected help overlay to be inactive after second '?'")
	}
}

func TestApp_HelpOverlayCapturesKeys(t *testing.T) {
	app := testAppWithItems(t)

	// Open help
	m, _ := app.Update(keyRune('?'))
	a := m.(App)

	// 'q' should not quit when help is active
	m, cmd := a.Update(keyRune('q'))
	if cmd != nil {
		msg := cmd()
		if msg == tea.Quit() {
			t.Fatal("'q' should not quit when help overlay is active")
		}
	}
	a = m.(App)
	if !a.help.active {
		t.Fatal("help overlay should still be active after typing 'q'")
	}

	// Esc closes
	m, _ = a.Update(keyPress(tea.KeyEsc))
	a = m.(App)
	if a.help.active {
		t.Fatal("help should be inactive after Esc")
	}
}

func TestApp_HelpToggleMsg(t *testing.T) {
	app := testAppWithItems(t)

	// Simulate helpToggleMsg (from clicking [?] in topbar corner)
	m, _ := app.Update(helpToggleMsg{})
	a := m.(App)
	if !a.help.active {
		t.Fatal("expected help overlay to be active after helpToggleMsg")
	}

	// Second toggle closes
	m, _ = a.Update(helpToggleMsg{})
	a = m.(App)
	if a.help.active {
		t.Fatal("expected help overlay to be inactive after second helpToggleMsg")
	}
}
