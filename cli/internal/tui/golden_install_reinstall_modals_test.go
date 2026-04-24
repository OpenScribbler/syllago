package tui

import (
	"testing"
)

// TestGoldenInstallUpdateModal_Sizes pins the Clean-state (D17 Case A) modal
// rendering at the three standard sizes. Regenerate with -update-golden
// whenever the copy, hotkeys, or border style change.
func TestGoldenInstallUpdateModal_Sizes(t *testing.T) {
	cases := []struct {
		name string
		w    int
	}{
		{"install-update-modal-60x20", 60},
		{"install-update-modal-80x30", 80},
		{"install-update-modal-120x40", 120},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newInstallUpdateModal()
			m.Open(
				"/home/user/CLAUDE.md",
				"sha256:abc123def456789a",
				"sha256:fff111222333444b",
			)
			m.width = tc.w
			got := normalizeSnapshot(m.View())
			requireGolden(t, tc.name, got)
		})
	}
}

// TestGoldenInstallModifiedModal_Reasons_Sizes pins the Modified-state (D17
// Case B) modal rendering per reason × per size. Unreadable uses a fixed
// "permission denied" readErr so the golden is deterministic.
func TestGoldenInstallModifiedModal_Reasons_Sizes(t *testing.T) {
	type variant struct {
		name    string
		reason  modifiedReason
		readErr string
	}
	variants := []variant{
		{"edited", modifiedReasonEdited, ""},
		{"missing", modifiedReasonMissing, ""},
		{"unreadable", modifiedReasonUnreadable, "permission denied"},
	}
	sizes := []int{60, 80, 120}

	for _, v := range variants {
		for _, w := range sizes {
			name := "install-modified-modal-" + v.name
			switch w {
			case 60:
				name += "-60x20"
			case 80:
				name += "-80x30"
			case 120:
				name += "-120x40"
			}
			t.Run(name, func(t *testing.T) {
				m := newInstallModifiedModal()
				m.Open("/home/user/CLAUDE.md", v.reason, v.readErr)
				m.width = w
				got := normalizeSnapshot(m.View())
				requireGolden(t, name, got)
			})
		}
	}
}
