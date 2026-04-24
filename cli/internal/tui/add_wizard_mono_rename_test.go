package tui

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/splitter"
)

// TestAddWizard_ReviewRename verifies the per-rule rename override (D18) on
// the Review step. Pressing [r] opens the edit modal with the focused
// candidate's slug pre-filled. Saving updates reviewRenames[cursor] so the
// rendered row shows the new slug and writeAcceptedCandidates picks it up.
func TestAddWizard_ReviewRename(t *testing.T) {
	t.Parallel()

	m := openAddWizard(nil, nil, nil, "/tmp/proj", "/tmp/content", "")
	m.source = addSourceMonolithic
	m.shell = newWizardShell("Add", m.buildShellLabels())
	m.discoveryCandidates = []monolithicCandidate{
		{RelPath: "CLAUDE.md", Filename: "CLAUDE.md", Scope: "project", ProviderID: "claude-code"},
	}
	m.selectedCandidates = []int{0}
	m.chosenHeuristic = int(splitter.HeuristicH2)
	// Seed 3 review candidates manually (skip the splitter call to keep the
	// test focused on the rename flow).
	m.reviewCandidates = []reviewCandidate{
		{SourceIdx: 0, Candidate: splitter.SplitCandidate{Name: "section-a"}, Accept: true},
		{SourceIdx: 0, Candidate: splitter.SplitCandidate{Name: "section-b"}, Accept: true},
		{SourceIdx: 0, Candidate: splitter.SplitCandidate{Name: "section-c"}, Accept: true},
	}
	m.reviewAccepted = []bool{true, true, true}
	m.reviewRenames = make([]string, 3)
	m.reviewCandidateCursor = 1
	m.step = addStepReview
	m.width = 100
	m.height = 30

	// Open rename modal for cursor=1.
	m, _ = m.openMonolithicRenameModal()
	if !m.renameModal.active {
		t.Fatalf("expected rename modal active after openMonolithicRenameModal")
	}
	if m.renameModal.context != "wizard_rename" {
		t.Errorf("modal context: got %q want %q", m.renameModal.context, "wizard_rename")
	}
	if m.renameModal.name != "section-b" {
		t.Errorf("modal pre-filled name: got %q want %q", m.renameModal.name, "section-b")
	}

	// Save a new slug.
	m.handleRenameSaved(editSavedMsg{name: "my-custom-slug", context: "wizard_rename"})

	if m.reviewRenames[1] != "my-custom-slug" {
		t.Errorf("reviewRenames[1]: got %q want %q", m.reviewRenames[1], "my-custom-slug")
	}
	// Untouched entries must stay empty.
	if m.reviewRenames[0] != "" || m.reviewRenames[2] != "" {
		t.Errorf("only cursor entry should be renamed, got %v", m.reviewRenames)
	}

	// Rendered row for index 1 must now show the override.
	accepted := m.acceptedReviewCandidates()
	if len(accepted) != 3 {
		t.Fatalf("expected 3 accepted, got %d", len(accepted))
	}
	if accepted[1].Candidate.Name != "my-custom-slug" {
		t.Errorf("acceptedReviewCandidates[1].Name: got %q want %q", accepted[1].Candidate.Name, "my-custom-slug")
	}
	if accepted[1].RenameSlug != "my-custom-slug" {
		t.Errorf("acceptedReviewCandidates[1].RenameSlug: got %q want %q", accepted[1].RenameSlug, "my-custom-slug")
	}
}
