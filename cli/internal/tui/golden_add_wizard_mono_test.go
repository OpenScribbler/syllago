package tui

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/splitter"
)

// snapshotAddWizard captures the add wizard view (View() runs renderShell +
// step view) with temp-dir paths normalized to <TESTDIR>.
func snapshotAddWizard(t *testing.T, m *addWizardModel) string {
	t.Helper()
	return normalizeSnapshot(m.View())
}

// setupMonoDiscoveryWizard returns a wizard parked on the Discovery step with
// three mocked monolithic candidates (one flagged in-library). Deterministic
// output — no filesystem calls.
func setupMonoDiscoveryWizard(t *testing.T, w, h int) *addWizardModel {
	t.Helper()
	m := openAddWizard(nil, nil, nil, "/tmp/proj", "/tmp/content", "")
	m.source = addSourceMonolithic
	m.shell = newWizardShell("Add", m.buildShellLabels())
	m.shell.SetWidth(w)
	m.width = w
	m.height = h
	m.step = addStepDiscovery
	m.discovering = false
	m.discoveryCandidates = []monolithicCandidate{
		{RelPath: "CLAUDE.md", Filename: "CLAUDE.md", Scope: "project", Lines: 42, H2Count: 5, ProviderID: "claude-code"},
		{RelPath: "AGENTS.md", Filename: "AGENTS.md", Scope: "project", Lines: 30, H2Count: 3, ProviderID: "codex", InLibrary: true},
		{RelPath: "GEMINI.md", Filename: "GEMINI.md", Scope: "global", Lines: 50, H2Count: 6, ProviderID: "gemini-cli"},
	}
	return m
}

// setupMonoHeuristicWizard returns a wizard parked on the Heuristic step with
// one selected candidate and cursor on H2 (default).
func setupMonoHeuristicWizard(t *testing.T, w, h int) *addWizardModel {
	t.Helper()
	m := setupMonoDiscoveryWizard(t, w, h)
	m.selectedCandidates = []int{0}
	m.step = addStepHeuristic
	m.heuristicCursor = 0
	m.chosenHeuristic = int(splitter.HeuristicH2)
	return m
}

// setupMonoReviewWizard returns a wizard parked on Review with three seeded
// review candidates (all accepted) plus one skip-split row. Deterministic
// naming keeps golden output stable.
func setupMonoReviewWizard(t *testing.T, w, h int) *addWizardModel {
	t.Helper()
	m := setupMonoHeuristicWizard(t, w, h)
	m.step = addStepReview
	m.reviewCandidates = []reviewCandidate{
		{SourceIdx: 0, Candidate: splitter.SplitCandidate{Name: "section-a", Description: "Section A"}, Accept: true},
		{SourceIdx: 0, Candidate: splitter.SplitCandidate{Name: "section-b", Description: "Section B"}, Accept: true},
		{SourceIdx: 0, Candidate: splitter.SplitCandidate{Name: "section-c", Description: "Section C"}, Accept: true},
	}
	m.reviewAccepted = []bool{true, true, true}
	m.reviewRenames = make([]string, 3)
	m.reviewCandidateCursor = 0
	return m
}

// --- Discovery step goldens ---

func TestGolden_AddMonoDiscovery_60x20(t *testing.T) {
	t.Parallel()
	m := setupMonoDiscoveryWizard(t, 60, 20)
	requireGolden(t, "add-mono-discovery-60x20", snapshotAddWizard(t, m))
}

func TestGolden_AddMonoDiscovery_80x30(t *testing.T) {
	t.Parallel()
	m := setupMonoDiscoveryWizard(t, 80, 30)
	requireGolden(t, "add-mono-discovery-80x30", snapshotAddWizard(t, m))
}

func TestGolden_AddMonoDiscovery_120x40(t *testing.T) {
	t.Parallel()
	m := setupMonoDiscoveryWizard(t, 120, 40)
	requireGolden(t, "add-mono-discovery-120x40", snapshotAddWizard(t, m))
}

// --- Heuristic step goldens ---

func TestGolden_AddMonoHeuristic_60x20(t *testing.T) {
	t.Parallel()
	m := setupMonoHeuristicWizard(t, 60, 20)
	requireGolden(t, "add-mono-heuristic-60x20", snapshotAddWizard(t, m))
}

func TestGolden_AddMonoHeuristic_80x30(t *testing.T) {
	t.Parallel()
	m := setupMonoHeuristicWizard(t, 80, 30)
	requireGolden(t, "add-mono-heuristic-80x30", snapshotAddWizard(t, m))
}

func TestGolden_AddMonoHeuristic_120x40(t *testing.T) {
	t.Parallel()
	m := setupMonoHeuristicWizard(t, 120, 40)
	requireGolden(t, "add-mono-heuristic-120x40", snapshotAddWizard(t, m))
}

// --- Review step goldens ---

func TestGolden_AddMonoReview_60x20(t *testing.T) {
	t.Parallel()
	m := setupMonoReviewWizard(t, 60, 20)
	requireGolden(t, "add-mono-review-60x20", snapshotAddWizard(t, m))
}

func TestGolden_AddMonoReview_80x30(t *testing.T) {
	t.Parallel()
	m := setupMonoReviewWizard(t, 80, 30)
	requireGolden(t, "add-mono-review-80x30", snapshotAddWizard(t, m))
}

func TestGolden_AddMonoReview_120x40(t *testing.T) {
	t.Parallel()
	m := setupMonoReviewWizard(t, 120, 40)
	requireGolden(t, "add-mono-review-120x40", snapshotAddWizard(t, m))
}
