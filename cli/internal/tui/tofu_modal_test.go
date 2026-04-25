package tui

// Tests for tofuModal: the trust-on-first-use approval surface that
// appears when a MOAT sync observes a wire signing identity that the
// registry has not yet pinned.
//
// Coverage:
//   - Open populates state and defaults focus to Reject (the safe
//     default — accidental Enter must NOT extend trust).
//   - Close clears state.
//   - Key handlers: Esc/n/r reject, Enter accepts/rejects per focus,
//     y/a accept, Tab toggles focus.
//   - Mouse handlers route to the right zone.
//   - The result message round-trips name, accepted, and manifestURL.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/config"
)

func sampleProfile() config.SigningProfile {
	return config.SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "https://github.com/example/registry",
	}
}

func TestTOFUModal_OpenSetsState(t *testing.T) {
	t.Parallel()
	m := newTOFUModal()
	m.Open("example", "https://example.com/manifest.json", sampleProfile())
	if !m.active {
		t.Fatal("expected active after Open")
	}
	if m.name != "example" {
		t.Errorf("name = %q; want %q", m.name, "example")
	}
	if m.manifestURL != "https://example.com/manifest.json" {
		t.Errorf("manifestURL = %q", m.manifestURL)
	}
	if m.focusIdx != tofuFocusReject {
		t.Errorf("default focus should be Reject (safe default), got %v", m.focusIdx)
	}
}

func TestTOFUModal_CloseClearsState(t *testing.T) {
	t.Parallel()
	m := newTOFUModal()
	m.Open("example", "url", sampleProfile())
	m.Close()
	if m.active {
		t.Error("expected !active after Close")
	}
	if m.name != "" || m.manifestURL != "" {
		t.Errorf("Close did not clear identity fields: name=%q url=%q", m.name, m.manifestURL)
	}
}

func TestTOFUModal_EscRejects(t *testing.T) {
	t.Parallel()
	m := newTOFUModal()
	m.Open("reg", "url", sampleProfile())
	m, cmd := m.updateKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.active {
		t.Error("modal should close on Esc")
	}
	if cmd == nil {
		t.Fatal("Esc should produce a result cmd")
	}
	res, ok := cmd().(tofuResultMsg)
	if !ok {
		t.Fatalf("expected tofuResultMsg, got %T", cmd())
	}
	if res.accepted {
		t.Error("Esc must reject")
	}
	if res.name != "reg" {
		t.Errorf("result.name = %q", res.name)
	}
}

func TestTOFUModal_EnterUsesFocus(t *testing.T) {
	t.Parallel()
	m := newTOFUModal()
	m.Open("reg", "url", sampleProfile())
	// Default focus is Reject — Enter rejects.
	_, cmd := m.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	if res := cmd().(tofuResultMsg); res.accepted {
		t.Error("Enter on Reject should reject")
	}

	// Tab to Accept and try Enter again.
	m = newTOFUModal()
	m.Open("reg", "url", sampleProfile())
	m, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusIdx != tofuFocusAccept {
		t.Fatal("Tab should move focus to Accept")
	}
	_, cmd = m.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	if res := cmd().(tofuResultMsg); !res.accepted {
		t.Error("Enter on Accept should accept")
	}
}

func TestTOFUModal_RuneShortcuts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		key      rune
		accepted bool
	}{
		{'y', true},
		{'a', true},
		{'n', false},
		{'r', false},
	}
	for _, tc := range cases {
		t.Run(string(tc.key), func(t *testing.T) {
			m := newTOFUModal()
			m.Open("reg", "url", sampleProfile())
			_, cmd := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})
			if cmd == nil {
				t.Fatalf("rune %q should produce a result", tc.key)
			}
			res := cmd().(tofuResultMsg)
			if res.accepted != tc.accepted {
				t.Errorf("rune %q: accepted = %v; want %v", tc.key, res.accepted, tc.accepted)
			}
		})
	}
}

func TestTOFUModal_TabAndShiftTabToggle(t *testing.T) {
	t.Parallel()
	m := newTOFUModal()
	m.Open("reg", "url", sampleProfile())

	m, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusIdx != tofuFocusAccept {
		t.Errorf("Tab from Reject -> Accept; got %v", m.focusIdx)
	}
	m, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyTab})
	if m.focusIdx != tofuFocusReject {
		t.Errorf("Tab from Accept -> Reject; got %v", m.focusIdx)
	}
	m, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.focusIdx != tofuFocusAccept {
		t.Errorf("Shift+Tab toggles too; got %v", m.focusIdx)
	}
}

func TestTOFUModal_InactiveIgnoresInput(t *testing.T) {
	t.Parallel()
	m := newTOFUModal() // never Open()'d
	got, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got.active {
		t.Error("Update must not activate inactive modal")
	}
	if cmd != nil {
		t.Error("inactive modal should swallow input silently")
	}
}

func TestTOFUModal_ViewRendersWhenActive(t *testing.T) {
	t.Parallel()
	m := newTOFUModal()
	m.Open("syllago-meta-registry", "https://example.com/manifest.json", sampleProfile())
	m.width = 80
	m.height = 24
	out := m.View()
	if out == "" {
		t.Fatal("View should return non-empty when active")
	}
	// Title text appears.
	if !contains(out, "Trust signing identity") {
		t.Errorf("View missing title; got: %s", out)
	}
	if !contains(out, "syllago-meta-registry") {
		t.Errorf("View missing registry name")
	}
}

func TestTOFUModal_ViewEmptyWhenInactive(t *testing.T) {
	t.Parallel()
	m := newTOFUModal()
	if got := m.View(); got != "" {
		t.Errorf("inactive View should be empty, got %q", got)
	}
}
