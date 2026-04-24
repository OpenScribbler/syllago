package tui

import (
	"fmt"
	"testing"

	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/splitter"
)

// Mouse-parity coverage for the monolithic-rule add-wizard path (Task 4.8).
// The zone-scanning tests must NOT run in parallel — bubblezone's global zone
// map is a singleton per the rule in .claude/rules/tui-testing.md.

// TestAddWizardMouse_MonolithicDiscoveryRowClick pins add-mono-disc-row-N.
// Clicking row 0 must toggle row 0 into selectedCandidates (mouse parity with
// the [space] key on the keyboard path).
func TestAddWizardMouse_MonolithicDiscoveryRowClick(t *testing.T) {
	m := openAddWizard(nil, nil, nil, "/tmp/proj", "/tmp/content", "")
	m.source = addSourceMonolithic
	m.shell = newWizardShell("Add", m.buildShellLabels())
	m.step = addStepDiscovery
	m.width = 100
	m.height = 30
	m.discovering = false
	m.discoveryCandidates = []monolithicCandidate{
		{RelPath: "CLAUDE.md", Filename: "CLAUDE.md", Scope: "project", ProviderID: "claude-code"},
		{RelPath: "AGENTS.md", Filename: "AGENTS.md", Scope: "project", ProviderID: "codex"},
	}
	scanZones(m.View())

	z := zone.Get("add-mono-disc-row-0")
	if z.IsZero() {
		t.Fatalf("zone add-mono-disc-row-0 not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if len(m.selectedCandidates) != 1 || m.selectedCandidates[0] != 0 {
		t.Errorf("after click on row 0, selectedCandidates=%v want [0]", m.selectedCandidates)
	}
	if m.discoveryCandidateCurs != 0 {
		t.Errorf("after click on row 0, cursor=%d want 0", m.discoveryCandidateCurs)
	}

	// Click row 1 — both rows should now be selected.
	scanZones(m.View())
	z1 := zone.Get("add-mono-disc-row-1")
	m, _ = m.Update(mouseClick(z1.StartX, z1.StartY))
	if len(m.selectedCandidates) != 2 {
		t.Errorf("after second click, expected 2 selected, got %v", m.selectedCandidates)
	}
}

// TestAddWizardMouse_HeuristicRadioClick pins add-heur-opt-{h2,h3,h4,marker,single}.
// Clicking each option must move the cursor to that position. The keyboard
// equivalent is arrow-down/up; both paths must yield identical cursor values.
func TestAddWizardMouse_HeuristicRadioClick(t *testing.T) {
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

	// Click H3 — expect cursor = 1.
	scanZones(m.View())
	z := zone.Get("add-heur-opt-h3")
	if z.IsZero() {
		t.Fatalf("zone add-heur-opt-h3 not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.heuristicCursor != 1 {
		t.Errorf("after H3 click, heuristicCursor=%d want 1", m.heuristicCursor)
	}

	// Click Marker option — expect cursor = 3.
	scanZones(m.View())
	z = zone.Get("add-heur-opt-marker")
	if z.IsZero() {
		t.Fatalf("zone add-heur-opt-marker not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.heuristicCursor != 3 {
		t.Errorf("after Marker click, heuristicCursor=%d want 3", m.heuristicCursor)
	}

	// Click Single — expect cursor = 4.
	scanZones(m.View())
	z = zone.Get("add-heur-opt-single")
	if z.IsZero() {
		t.Fatalf("zone add-heur-opt-single not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.heuristicCursor != 4 {
		t.Errorf("after Single click, heuristicCursor=%d want 4", m.heuristicCursor)
	}
}

// TestAddWizardMouse_ReviewCandidateClick pins add-mono-review-cand-N. Clicking
// a review row must move reviewCandidateCursor to that row. Keyboard parity:
// arrow-down steps through rows in the same order.
func TestAddWizardMouse_ReviewCandidateClick(t *testing.T) {
	m := openAddWizard(nil, nil, nil, "/tmp/proj", "/tmp/content", "")
	m.source = addSourceMonolithic
	m.shell = newWizardShell("Add", m.buildShellLabels())
	m.discoveryCandidates = []monolithicCandidate{
		{RelPath: "CLAUDE.md", Filename: "CLAUDE.md", Scope: "project", ProviderID: "claude-code"},
	}
	m.selectedCandidates = []int{0}
	m.chosenHeuristic = int(splitter.HeuristicH2)
	m.reviewCandidates = []reviewCandidate{
		{SourceIdx: 0, Candidate: splitter.SplitCandidate{Name: "section-a", Description: "A"}, Accept: true},
		{SourceIdx: 0, Candidate: splitter.SplitCandidate{Name: "section-b", Description: "B"}, Accept: true},
		{SourceIdx: 0, Candidate: splitter.SplitCandidate{Name: "section-c", Description: "C"}, Accept: true},
	}
	m.reviewAccepted = []bool{true, true, true}
	m.reviewRenames = make([]string, 3)
	m.reviewCandidateCursor = 0
	m.step = addStepReview
	m.width = 100
	m.height = 30

	scanZones(m.View())
	z := zone.Get("add-mono-review-cand-2")
	if z.IsZero() {
		t.Fatalf("zone add-mono-review-cand-2 not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.reviewCandidateCursor != 2 {
		t.Errorf("after click on review row 2, cursor=%d want 2", m.reviewCandidateCursor)
	}

	// And sanity: clicking row 1 moves cursor back.
	scanZones(m.View())
	z = zone.Get(fmt.Sprintf("add-mono-review-cand-%d", 1))
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.reviewCandidateCursor != 1 {
		t.Errorf("after click on review row 1, cursor=%d want 1", m.reviewCandidateCursor)
	}
}
