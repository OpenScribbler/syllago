package tui

import (
	"os"
	"path/filepath"
	"testing"

	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// Mouse-parity coverage for the Provider-flow Heuristic step. These tests
// must NOT run in parallel — bubblezone's global zone map is a singleton
// (see .claude/rules/tui-testing.md).

// setupProviderHeuristicModelMulti builds a Provider-flow wizard past triage
// with N splittable rules selected. It mirrors setupProviderHeuristicModel
// but allows multiple rows for cursor-movement tests.
func setupProviderHeuristicModelMulti(t *testing.T, n int) *addWizardModel {
	t.Helper()
	tmp := t.TempDir()
	var items []addDiscoveryItem
	names := []string{"CLAUDE", "AGENTS", "CURSOR"}
	for i := 0; i < n; i++ {
		sub := filepath.Join(tmp, names[i])
		if err := os.MkdirAll(sub, 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		path := writeSplittableClaudeMD(t, sub)
		items = append(items, addDiscoveryItem{
			name:       names[i],
			itemType:   catalog.Rules,
			status:     add.StatusNew,
			path:       path,
			underlying: &add.DiscoveryItem{Name: names[i], Type: catalog.Rules, Path: path, Status: add.StatusNew},
		})
	}

	m := testOpenAddWizard(t)
	m = injectDiscoveryResults(t, m, items)
	for i := range m.confirmSelected {
		m.confirmSelected[i] = true
	}
	m.advanceAfterTriage()
	return m
}

// TestAddWizardMouse_ProviderSplitRowClick_MovesCursor verifies that clicking
// a non-focused row moves the cursor without toggling splitChosen — checkbox
// semantics match the keyboard arrow keys.
func TestAddWizardMouse_ProviderSplitRowClick_MovesCursor(t *testing.T) {
	m := setupProviderHeuristicModelMulti(t, 2)
	m.width = 100
	m.height = 30

	if m.heuristicCursor != 0 {
		t.Fatalf("precondition: heuristicCursor=%d want 0", m.heuristicCursor)
	}
	row1Chosen := m.discoveredItems[1].splitChosen

	scanZones(m.View())
	z := zone.Get("add-psplit-row-1")
	if z.IsZero() {
		t.Fatalf("zone add-psplit-row-1 not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.heuristicCursor != 1 {
		t.Errorf("after click row 1, heuristicCursor=%d want 1", m.heuristicCursor)
	}
	if m.discoveredItems[1].splitChosen != row1Chosen {
		t.Errorf("click on non-focused row must not toggle splitChosen")
	}
}

// TestAddWizardMouse_ProviderSplitRowClick_TogglesWhenFocused verifies that
// clicking the focused row toggles splitChosen — matching the [space] key.
func TestAddWizardMouse_ProviderSplitRowClick_TogglesWhenFocused(t *testing.T) {
	m := setupProviderHeuristicModelMulti(t, 1)
	m.width = 100
	m.height = 30

	if !m.discoveredItems[0].splitChosen {
		t.Fatalf("precondition: splitChosen must default to true")
	}

	scanZones(m.View())
	z := zone.Get("add-psplit-row-0")
	if z.IsZero() {
		t.Fatalf("zone add-psplit-row-0 not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.discoveredItems[0].splitChosen {
		t.Errorf("click on focused row must toggle splitChosen off")
	}

	scanZones(m.View())
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if !m.discoveredItems[0].splitChosen {
		t.Errorf("second click must toggle splitChosen back on")
	}
}
