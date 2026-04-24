package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// setupProviderHeuristicModel builds a wizard already merged past triage with
// a single splittable CLAUDE.md rule selected. Shared fixture for the
// provider-flow Heuristic tests.
func setupProviderHeuristicModel(t *testing.T) *addWizardModel {
	t.Helper()
	tmp := t.TempDir()
	claudePath := writeSplittableClaudeMD(t, tmp)

	m := testOpenAddWizard(t)
	items := []addDiscoveryItem{{
		name: "CLAUDE", itemType: catalog.Rules,
		status:     add.StatusNew,
		path:       claudePath,
		underlying: &add.DiscoveryItem{Name: "CLAUDE", Type: catalog.Rules, Path: claudePath, Status: add.StatusNew},
	}}
	m = injectDiscoveryResults(t, m, items)

	// Pre-select and merge as advanceAfterTriage would.
	for i := range m.confirmSelected {
		m.confirmSelected[i] = true
	}
	m.advanceAfterTriage()
	return m
}

func TestProviderHeuristic_EntersOnSplittableSelection(t *testing.T) {
	t.Parallel()
	m := setupProviderHeuristicModel(t)

	if m.step != addStepHeuristic {
		t.Fatalf("expected step Heuristic after advanceAfterTriage, got %d", m.step)
	}
	if !m.hasSplittableSelection() {
		t.Fatal("hasSplittableSelection() must be true for splittable rule")
	}
}

func TestProviderHeuristic_ShellIncludesHeuristicLabel(t *testing.T) {
	t.Parallel()
	m := setupProviderHeuristicModel(t)

	labels := m.buildShellLabels()
	found := false
	for _, l := range labels {
		if l == "Heuristic" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'Heuristic' label in provider-flow shell; got %v", labels)
	}
}

func TestProviderHeuristic_DefaultChosenIsSplit(t *testing.T) {
	t.Parallel()
	m := setupProviderHeuristicModel(t)

	selected := m.selectedItems()
	if len(selected) != 1 {
		t.Fatalf("expected 1 selected item, got %d", len(selected))
	}
	if !selected[0].splitChosen {
		t.Fatal("splitChosen must default to true for auto-detected splittable rule")
	}
}

func TestProviderHeuristic_SpaceTogglesChoice(t *testing.T) {
	t.Parallel()
	m := setupProviderHeuristicModel(t)

	// Space toggles the current cursor row.
	m, _ = m.updateKeyProviderHeuristic(tea.KeyMsg{Type: tea.KeySpace})

	selected := m.selectedItems()
	if selected[0].splitChosen {
		t.Fatal("splitChosen must flip to false after [space]")
	}

	// Another space flips it back.
	m, _ = m.updateKeyProviderHeuristic(tea.KeyMsg{Type: tea.KeySpace})
	selected = m.selectedItems()
	if !selected[0].splitChosen {
		t.Fatal("splitChosen must flip back to true after second [space]")
	}
}

func TestProviderHeuristic_EnterAdvancesToReview(t *testing.T) {
	t.Parallel()
	m := setupProviderHeuristicModel(t)

	m, _ = m.updateKeyProviderHeuristic(tea.KeyMsg{Type: tea.KeyEnter})

	if m.step != addStepReview {
		t.Fatalf("expected step Review after [enter], got %d", m.step)
	}
}

func TestProviderHeuristic_EscReturnsToDiscovery(t *testing.T) {
	t.Parallel()
	m := setupProviderHeuristicModel(t)

	m, _ = m.updateKeyProviderHeuristic(tea.KeyMsg{Type: tea.KeyEsc})

	if m.step != addStepDiscovery {
		t.Fatalf("expected step Discovery after [esc], got %d", m.step)
	}
}

func TestProviderHeuristic_ReviewEscReturnsToHeuristic(t *testing.T) {
	t.Parallel()
	m := setupProviderHeuristicModel(t)

	// Advance to Review
	m, _ = m.updateKeyProviderHeuristic(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != addStepReview {
		t.Fatalf("precondition: expected Review, got %d", m.step)
	}

	// Esc from Review should route back to Heuristic (not Discovery) since
	// splittable items are in play.
	m, _ = m.updateKeyReview(tea.KeyMsg{Type: tea.KeyEsc})

	if m.step != addStepHeuristic {
		t.Fatalf("expected step Heuristic after Review [esc], got %d", m.step)
	}
}

func TestProviderHeuristic_ViewRendersRuleName(t *testing.T) {
	t.Parallel()
	m := setupProviderHeuristicModel(t)

	out := m.viewProviderHeuristic()
	if !strings.Contains(out, "CLAUDE") {
		t.Fatalf("expected rule name in Heuristic view, got:\n%s", out)
	}
	if !strings.Contains(out, "3 sections") {
		t.Fatalf("expected section count in Heuristic view, got:\n%s", out)
	}
}
