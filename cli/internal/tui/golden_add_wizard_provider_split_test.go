package tui

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// setupProviderSplitHeuristicWizard returns a Provider-flow wizard parked on
// the Heuristic step with two splittable rules (CLAUDE.md, AGENTS.md) — both
// flagged splittable and defaulted to splitChosen=true. State is populated
// directly (no filesystem) to keep golden output deterministic.
func setupProviderSplitHeuristicWizard(t *testing.T, w, h int) *addWizardModel {
	t.Helper()
	m := openAddWizard(nil, nil, nil, "/tmp/proj", "/tmp/content", "")
	m.source = addSourceProvider
	m.width = w
	m.height = h

	m.discoveredItems = []addDiscoveryItem{
		{
			name:              "CLAUDE",
			itemType:          catalog.Rules,
			status:            add.StatusNew,
			path:              "/tmp/proj/CLAUDE.md",
			splittable:        true,
			splitSectionCount: 5,
			splitChosen:       true,
		},
		{
			name:              "AGENTS",
			itemType:          catalog.Rules,
			status:            add.StatusNew,
			path:              "/tmp/proj/AGENTS.md",
			splittable:        true,
			splitSectionCount: 3,
			splitChosen:       false,
		},
	}
	m.actionableCount = 2
	m.installedCount = 0
	m.discoveryList = m.buildDiscoveryList()
	m.discoveryList.selected[0] = true
	m.discoveryList.selected[1] = true

	m.shell = newWizardShell("Add", m.buildShellLabels())
	m.shell.SetWidth(w)
	m.shell.SetActive(m.shellIndexForStep(addStepHeuristic))
	m.step = addStepHeuristic
	m.heuristicCursor = 0
	return m
}

func TestGolden_AddProviderSplitHeuristic_60x20(t *testing.T) {
	t.Parallel()
	m := setupProviderSplitHeuristicWizard(t, 60, 20)
	requireGolden(t, "add-provider-split-heuristic-60x20", snapshotAddWizard(t, m))
}

func TestGolden_AddProviderSplitHeuristic_80x30(t *testing.T) {
	t.Parallel()
	m := setupProviderSplitHeuristicWizard(t, 80, 30)
	requireGolden(t, "add-provider-split-heuristic-80x30", snapshotAddWizard(t, m))
}

func TestGolden_AddProviderSplitHeuristic_120x40(t *testing.T) {
	t.Parallel()
	m := setupProviderSplitHeuristicWizard(t, 120, 40)
	requireGolden(t, "add-provider-split-heuristic-120x40", snapshotAddWizard(t, m))
}
