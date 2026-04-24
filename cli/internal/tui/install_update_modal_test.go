package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestInstallUpdateModal_Renders verifies the Case A (Clean state) modal
// renders the D17-exact copy, includes both option labels, and defaults focus
// to Replace so [Enter] triggers the update action (not Skip).
func TestInstallUpdateModal_Renders(t *testing.T) {
	m := newInstallUpdateModal()
	m.Open(
		"/tmp/CLAUDE.md",
		"sha256:abc123abcdef",
		"sha256:def456abcdef",
	)

	view := m.View()
	if view == "" {
		t.Fatalf("View() returned empty string on active modal")
	}

	mustContain(t, view, "This rule is already installed at:")
	mustContain(t, view, "Replace with current version")
	mustContain(t, view, "Skip (leave file unchanged)")
	// Short hashes should appear so the user can distinguish versions.
	mustContain(t, view, "abc123")
	mustContain(t, view, "def456")

	// Enter on default focus must produce a replace decision.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("Enter on default-focused modal should produce a decision cmd")
	}
	msg := cmd()
	decision, ok := msg.(installUpdateDecisionMsg)
	if !ok {
		t.Fatalf("expected installUpdateDecisionMsg, got %T", msg)
	}
	if decision.action != "replace" {
		t.Errorf("default Enter action: got %q, want %q", decision.action, "replace")
	}
}

// TestInstallUpdateModal_EscEmitsSkip verifies Esc on the Clean-state modal
// produces a skip decision so the file is never mutated.
func TestInstallUpdateModal_EscEmitsSkip(t *testing.T) {
	m := newInstallUpdateModal()
	m.Open("/tmp/CLAUDE.md", "sha256:abc123", "sha256:def456")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatalf("Esc should produce a decision cmd")
	}
	msg := cmd()
	decision, ok := msg.(installUpdateDecisionMsg)
	if !ok {
		t.Fatalf("expected installUpdateDecisionMsg, got %T", msg)
	}
	if decision.action != "skip" {
		t.Errorf("Esc action: got %q, want %q", decision.action, "skip")
	}
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected view to contain %q; view:\n%s", needle, haystack)
	}
}
