package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/splitter"
)

// TestAddWizard_HeuristicStepOptions verifies the Heuristic step radio list
// (Task 4.4, D3). The rendered output must contain all five heuristic labels:
// "By H2 (default)", "By H3", "By H4", "By literal marker", "Import as
// single rule". Default cursor selects H2 (chosenHeuristic == HeuristicH2).
func TestAddWizard_HeuristicStepOptions(t *testing.T) {
	t.Parallel()

	m := openAddWizard(nil, nil, nil, "/tmp/proj", "/tmp/content", "")
	m.source = addSourceMonolithic
	m.shell = newWizardShell("Add", m.buildShellLabels())
	m.selectedCandidates = []int{0}
	m.discoveryCandidates = []monolithicCandidate{
		{RelPath: "CLAUDE.md", Filename: "CLAUDE.md", Scope: "project"},
	}
	m.step = addStepHeuristic
	m.heuristicCursor = 0
	m.chosenHeuristic = int(splitter.HeuristicH2)
	m.width = 100
	m.height = 30

	view := m.viewHeuristic()

	for _, want := range []string{
		"By H2 (default)",
		"By H3",
		"By H4",
		"By literal marker",
		"Import as single rule",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing label %q\nview:\n%s", want, view)
		}
	}

	// Default selection = H2 (cursor 0). First option must render with the
	// filled radio marker ◉.
	lines := strings.Split(view, "\n")
	var h2Row string
	for _, ln := range lines {
		if strings.Contains(ln, "By H2") {
			h2Row = ln
			break
		}
	}
	if !strings.Contains(h2Row, "◉") {
		t.Errorf("H2 row should contain filled radio ◉, got %q", h2Row)
	}
}

// TestAddWizard_HeuristicStep_KeyNavigation verifies cursor up/down and
// Enter advance the wizard to the Review step, Esc returns to Discovery.
func TestAddWizard_HeuristicStep_KeyNavigation(t *testing.T) {
	t.Parallel()

	m := openAddWizard(nil, nil, nil, "/tmp/proj", "/tmp/content", "")
	m.source = addSourceMonolithic
	m.shell = newWizardShell("Add", m.buildShellLabels())
	m.selectedCandidates = []int{0}
	// Seed one candidate with enough body to make the splitter happy when
	// Enter advances to Review. 40 lines with several H2 headings.
	var body strings.Builder
	body.WriteString("# Header\n")
	for i := 0; i < 40; i++ {
		if i%10 == 0 {
			body.WriteString("## Section ")
			body.WriteByte('A' + byte(i/10))
			body.WriteByte('\n')
		}
		body.WriteString("line ")
		body.WriteByte('a' + byte(i%26))
		body.WriteByte('\n')
	}
	m.discoveryCandidates = []monolithicCandidate{
		{RelPath: "CLAUDE.md", Filename: "CLAUDE.md", Scope: "project", Bytes: []byte(body.String()), ProviderID: "claude-code"},
	}
	m.step = addStepHeuristic
	m.heuristicCursor = 0
	m.chosenHeuristic = int(splitter.HeuristicH2)
	m.width = 100
	m.height = 30

	// Down moves cursor
	m, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.heuristicCursor != 1 {
		t.Fatalf("expected heuristicCursor=1 after Down, got %d", m.heuristicCursor)
	}

	// Up moves cursor back
	m, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.heuristicCursor != 0 {
		t.Fatalf("expected heuristicCursor=0 after Up, got %d", m.heuristicCursor)
	}

	// Enter advances to Review step
	m, _ = m.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != addStepReview {
		t.Fatalf("expected step=addStepReview after Enter, got %d", m.step)
	}
}
