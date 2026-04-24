package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestAddWizard_DiscoveryMultiSelect exercises the monolithic-rule discovery
// step (D2, D18). Three mock candidates are seeded on the Discovery step with
// source set to addSourceMonolithic. Two spacebar presses at different cursor
// positions must leave both rows in selectedCandidates and render each with
// the selected checkbox mark. Tests the keyboard toggle behavior — the mouse
// click equivalent is covered in add_wizard_mouse_test.go (Task 4.8).
func TestAddWizard_DiscoveryMultiSelect(t *testing.T) {
	t.Parallel()

	m := openAddWizard(nil, nil, nil, "/tmp/proj", "/tmp/content", "")
	m.source = addSourceMonolithic
	m.shell = newWizardShell("Add", m.buildShellLabels())
	m.step = addStepDiscovery
	m.width = 100
	m.height = 30
	m.discovering = false
	m.discoveryCandidates = []monolithicCandidate{
		{RelPath: "CLAUDE.md", Filename: "CLAUDE.md", Scope: "project", Lines: 40, H2Count: 5, ProviderID: "claude-code"},
		{RelPath: "AGENTS.md", Filename: "AGENTS.md", Scope: "project", Lines: 30, H2Count: 3, ProviderID: "codex"},
		{RelPath: "GEMINI.md", Filename: "GEMINI.md", Scope: "global", Lines: 50, H2Count: 6, ProviderID: "gemini-cli"},
	}
	m.discoveryCandidateCurs = 0

	// Press space at row 0
	m, _ = m.updateKey(tea.KeyMsg{Type: tea.KeySpace})

	// Move cursor to row 2 and press space
	m.discoveryCandidateCurs = 2
	m, _ = m.updateKey(tea.KeyMsg{Type: tea.KeySpace})

	if len(m.selectedCandidates) != 2 {
		t.Fatalf("expected 2 selected candidates, got %d (%v)", len(m.selectedCandidates), m.selectedCandidates)
	}
	gotSel := map[int]bool{}
	for _, idx := range m.selectedCandidates {
		gotSel[idx] = true
	}
	if !gotSel[0] {
		t.Errorf("expected row 0 to be selected")
	}
	if !gotSel[2] {
		t.Errorf("expected row 2 to be selected")
	}

	// Render the rows and verify the selected mark shows up for rows 0 and 2
	rows := m.renderMonolithicDiscoveryRows(m.width)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rendered rows, got %d", len(rows))
	}
	if !strings.Contains(rows[0], "[x]") {
		t.Errorf("row 0 should show [x] mark, got %q", rows[0])
	}
	if strings.Contains(rows[1], "[x]") {
		t.Errorf("row 1 should NOT show [x] mark, got %q", rows[1])
	}
	if !strings.Contains(rows[2], "[x]") {
		t.Errorf("row 2 should show [x] mark, got %q", rows[2])
	}
}
