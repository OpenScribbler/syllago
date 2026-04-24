package tui

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/splitter"
)

// TestAddWizard_ReviewRendersCandidatesPerSource seeds two monolithic sources.
// The first splits cleanly into three H2 candidates; the second is short
// enough to trigger splitter's too_few_h2 skip-split signal. The Review view
// must show one group per source, with the skip-split source rendered as a
// single row labeled "will import as single rule (too few H2 headings)" (D4).
func TestAddWizard_ReviewRendersCandidatesPerSource(t *testing.T) {
	t.Parallel()

	// Source 1 — splits into 3 H2 candidates. Must be >= 30 lines AND have
	// >= 3 H2 headings to avoid the splitter's skip-split heuristics.
	var b1 strings.Builder
	b1.WriteString("# Title\n\n")
	for i := 1; i <= 3; i++ {
		b1.WriteString("## Section ")
		b1.WriteByte('A' + byte(i-1))
		b1.WriteString("\n\n")
		for j := 0; j < 12; j++ {
			b1.WriteString("body line ")
			b1.WriteByte('a' + byte(j))
			b1.WriteByte('\n')
		}
		b1.WriteString("\n")
	}
	src1 := b1.String()

	// Source 2 — has only one H2 but enough lines to pass the 30-line floor,
	// so skip-split fires with reason too_few_h2.
	var b2 strings.Builder
	b2.WriteString("# Title\n\n## Only One\n\n")
	for i := 0; i < 40; i++ {
		b2.WriteString("line ")
		b2.WriteByte('a' + byte(i%26))
		b2.WriteByte('\n')
	}
	src2 := b2.String()

	m := openAddWizard(nil, nil, nil, "/tmp/proj", "/tmp/content", "")
	m.source = addSourceMonolithic
	m.shell = newWizardShell("Add", m.buildShellLabels())
	m.discoveryCandidates = []monolithicCandidate{
		{RelPath: "CLAUDE.md", Filename: "CLAUDE.md", Scope: "project", Bytes: []byte(src1), ProviderID: "claude-code"},
		{RelPath: "AGENTS.md", Filename: "AGENTS.md", Scope: "project", Bytes: []byte(src2), ProviderID: "codex"},
	}
	m.selectedCandidates = []int{0, 1}
	m.chosenHeuristic = int(splitter.HeuristicH2)
	m.reviewCandidates = buildReviewCandidates(m.discoveryCandidates, m.selectedCandidates, m.chosenHeuristic, "")
	m.reviewAccepted = make([]bool, len(m.reviewCandidates))
	m.reviewRenames = make([]string, len(m.reviewCandidates))
	for i := range m.reviewAccepted {
		m.reviewAccepted[i] = true
	}
	m.step = addStepReview
	m.width = 120
	m.height = 40

	view := m.viewMonolithicReview()

	// Both source headers should render.
	if !strings.Contains(view, "CLAUDE.md") {
		t.Errorf("expected CLAUDE.md group header in view, got:\n%s", view)
	}
	if !strings.Contains(view, "AGENTS.md") {
		t.Errorf("expected AGENTS.md group header in view, got:\n%s", view)
	}

	// AGENTS.md should show the skip-split label with the human reason.
	if !strings.Contains(view, "single") {
		t.Errorf("expected skip-split label for AGENTS.md, got:\n%s", view)
	}
	if !strings.Contains(view, "too few H2 headings") {
		t.Errorf("expected skip reason 'too few H2 headings' in view, got:\n%s", view)
	}

	// CLAUDE.md should indicate 3 candidates in its group header (short form).
	if !strings.Contains(view, "3 cands") {
		t.Errorf("expected CLAUDE.md group to render '3 cands' header, got:\n%s", view)
	}

	// Count candidates produced — 3 for source 1 (split) + 1 for source 2
	// (skip-split emits a single whole-file row).
	if got, want := len(m.reviewCandidates), 4; got != want {
		t.Errorf("expected %d review candidates (3 split + 1 skip), got %d", want, got)
	}
}
