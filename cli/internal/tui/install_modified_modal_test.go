package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestInstallModifiedModal_Renders exercises all three D17 Case B reasons,
// asserting the reason-varying copy lines and reason-varying default focus
// (Drop for edited/missing; Keep for unreadable).
func TestInstallModifiedModal_Renders(t *testing.T) {
	cases := []struct {
		name          string
		reason        modifiedReason
		readErr       string
		wantContains  string
		defaultAction string
		defaultFocus  int // index of default-focused option
		focusLabel    string
	}{
		{
			name:          "edited",
			reason:        modifiedReasonEdited,
			readErr:       "",
			wantContains:  "...but the file no longer contains the version we recorded.",
			defaultAction: "drop-record",
			defaultFocus:  0,
			focusLabel:    "Drop stale install record",
		},
		{
			name:          "missing",
			reason:        modifiedReasonMissing,
			readErr:       "",
			wantContains:  "...but that file no longer exists.",
			defaultAction: "drop-record",
			defaultFocus:  0,
			focusLabel:    "Drop stale install record",
		},
		{
			name:          "unreadable",
			reason:        modifiedReasonUnreadable,
			readErr:       "permission denied",
			wantContains:  "...but we couldn't read the file:",
			defaultAction: "keep",
			defaultFocus:  2,
			focusLabel:    "Keep (leave record + file unchanged)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newInstallModifiedModal()
			m.Open("/tmp/CLAUDE.md", tc.reason, tc.readErr)

			view := m.View()
			mustContain(t, view, tc.wantContains)
			mustContain(t, view, "Drop stale install record")
			mustContain(t, view, "Append current version as a fresh copy")
			mustContain(t, view, "Keep (leave record + file unchanged)")

			if m.focusIdx != tc.defaultFocus {
				t.Errorf("default focus for %s: got %d, want %d (%s)", tc.name, m.focusIdx, tc.defaultFocus, tc.focusLabel)
			}

			// [Enter] must emit the reason's default action.
			_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil {
				t.Fatalf("Enter should emit a decision cmd")
			}
			msg := cmd()
			d, ok := msg.(installModifiedDecisionMsg)
			if !ok {
				t.Fatalf("expected installModifiedDecisionMsg, got %T", msg)
			}
			if d.action != tc.defaultAction {
				t.Errorf("default Enter action for %s: got %q, want %q", tc.name, d.action, tc.defaultAction)
			}
		})
	}
}
