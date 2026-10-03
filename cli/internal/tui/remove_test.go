package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

func testItem() catalog.ContentItem {
	return catalog.ContentItem{
		Name:        "my-hook",
		DisplayName: "My Hook",
		Type:        catalog.Hooks,
		Path:        "/tmp/test/hooks/my-hook",
		Library:     true,
	}
}

func openRemoveModal(uninstallFrom []string) removeModal {
	m := newRemoveModal()
	m.Open(testItem(), uninstallFrom)
	m.width = 80
	m.height = 30
	return m
}

// requireRemoveResult runs cmd and checks the decision it carries.
func requireRemoveResult(t *testing.T, cmd tea.Cmd, confirmed bool) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command")
	}
	res, ok := cmd().(removeResultMsg)
	if !ok {
		t.Fatalf("expected removeResultMsg, got %T", cmd())
	}
	if res.confirmed != confirmed {
		t.Errorf("confirmed = %v, want %v", res.confirmed, confirmed)
	}
	if confirmed && res.item.Name != "my-hook" {
		t.Errorf("item = %q, want my-hook", res.item.Name)
	}
}

func TestRemoveModal_OpenClose(t *testing.T) {
	m := newRemoveModal()
	if m.active {
		t.Fatal("should not be active initially")
	}

	m.Open(testItem(), []string{"Claude Code"})
	if !m.active || m.itemName != "My Hook" || m.focusIdx != 0 {
		t.Fatalf("after Open: active=%v itemName=%q focusIdx=%d", m.active, m.itemName, m.focusIdx)
	}

	m.Close()
	if m.active || m.itemName != "" || m.uninstallFrom != nil {
		t.Error("Close should clear state")
	}
}

func TestRemoveModal_NotInstalledView(t *testing.T) {
	view := ansi.Strip(openRemoveModal(nil).View())
	if strings.Contains(view, "Will uninstall from") {
		t.Error("should not list uninstalls when nothing is installed")
	}
	for _, want := range []string{`Remove "My Hook"?`, "This action cannot be undone.", "Cancel", "Remove"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestRemoveModal_InstalledViewListsUninstalls(t *testing.T) {
	view := ansi.Strip(openRemoveModal([]string{"Claude Code", "Cursor (old-name)"}).View())
	for _, want := range []string{"Will uninstall from:", "Claude Code", "Cursor (old-name)"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestRemoveModal_EnterUsesFocus(t *testing.T) {
	m := openRemoveModal([]string{"Claude Code"})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	requireRemoveResult(t, cmd, false) // Cancel has focus on open

	m = openRemoveModal([]string{"Claude Code"})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	requireRemoveResult(t, cmd, true)
}

func TestRemoveModal_Shortcuts(t *testing.T) {
	tests := []struct {
		name      string
		key       tea.KeyMsg
		confirmed bool
	}{
		{"y", keyRune('y'), true},
		{"n", keyRune('n'), false},
		{"esc", tea.KeyMsg{Type: tea.KeyEsc}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, cmd := openRemoveModal([]string{"Claude Code"}).Update(tt.key)
			requireRemoveResult(t, cmd, tt.confirmed)
			if m.active {
				t.Error("modal should close")
			}
		})
	}
}

func TestRemoveModal_ArrowsToggleFocus(t *testing.T) {
	m := openRemoveModal(nil)
	for _, k := range []tea.KeyMsg{{Type: tea.KeyRight}, {Type: tea.KeyLeft}, keyRune('l'), keyRune('h'), {Type: tea.KeyShiftTab}} {
		before := m.focusIdx
		m, _ = m.Update(k)
		if m.focusIdx != 1-before {
			t.Fatalf("%v: focusIdx = %d, want %d", k, m.focusIdx, 1-before)
		}
	}
}

func TestRemoveModal_InactiveIgnoresInput(t *testing.T) {
	m := newRemoveModal()
	if _, cmd := m.Update(keyRune('y')); cmd != nil {
		t.Error("inactive modal should ignore input")
	}
}

// --- Mouse tests (sequential — bubblezone singleton) ---

func TestRemoveModal_MouseButtons(t *testing.T) {
	for _, tt := range []struct {
		zoneID    string
		confirmed bool
	}{
		{"rm-cancel", false},
		{"rm-remove", true},
	} {
		m := openRemoveModal([]string{"Claude Code"})
		scanZones(m.View())
		z := zone.Get(tt.zoneID)
		if z.IsZero() {
			t.Fatalf("zone %s not registered", tt.zoneID)
		}
		_, cmd := m.Update(mouseClick(z.StartX, z.StartY))
		requireRemoveResult(t, cmd, tt.confirmed)
	}
}

func TestRemoveModal_MouseNonLeftIgnored(t *testing.T) {
	m := openRemoveModal([]string{"Claude Code"})
	msg := tea.MouseMsg{X: 10, Y: 10, Action: tea.MouseActionPress, Button: tea.MouseButtonRight}
	updated, cmd := m.Update(msg)
	if cmd != nil || !updated.active {
		t.Error("right-click should neither emit a command nor close the modal")
	}
}
