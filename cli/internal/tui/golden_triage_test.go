package tui

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/analyzer"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// testConfirmItemsUnsorted returns confirm items in arrival order (before type-sort).
// Use this as input to sortConfirmItemsByType to match enterTriage() behavior.
func testConfirmItemsUnsorted() []addConfirmItem {
	return []addConfirmItem{
		{
			displayName: "security-scanner",
			itemType:    catalog.Rules,
			tier:        analyzer.TierHigh,
			path:        "rules/security-scanner.md",
			sourceDir:   "/tmp/test-source/rules",
		},
		{
			displayName: "code-style-guide",
			itemType:    catalog.Skills,
			tier:        analyzer.TierMedium,
			path:        "skills/code-style-guide/SKILL.md",
			sourceDir:   "/tmp/test-source/skills/code-style-guide",
		},
		{
			displayName: "experimental-linter",
			itemType:    catalog.Rules,
			tier:        analyzer.TierLow,
			path:        "rules/experimental-linter.md",
			sourceDir:   "/tmp/test-source/rules",
		},
		{
			displayName: "user-custom-agent",
			itemType:    catalog.Agents,
			tier:        analyzer.TierUser,
			path:        "agents/user-custom-agent/agent.md",
			sourceDir:   "/tmp/test-source/agents/user-custom-agent",
		},
		{
			displayName: "api-docs-helper",
			itemType:    catalog.Skills,
			tier:        analyzer.TierMedium,
			path:        "skills/api-docs-helper/SKILL.md",
			sourceDir:   "/tmp/test-source/skills/api-docs-helper",
		},
	}
}

// setupTriageWizard creates an addWizardModel ready for triage golden tests.
// Items are sorted by type (matching enterTriage() behavior) and High/User
// items are pre-checked; Medium and Low are unchecked.
func setupTriageWizard(t *testing.T, w, h int) *addWizardModel {
	t.Helper()
	m := testOpenAddWizard(t)
	m.width = w
	m.height = h
	m.shell.SetWidth(w)

	// Reproduce enterTriage() sort so golden output matches production.
	rawItems := testConfirmItemsUnsorted()
	rawSelected := map[int]bool{
		0: true, // security-scanner (High) — checked
		3: true, // user-custom-agent (User) — checked
	}
	items, selected := sortConfirmItemsByType(rawItems, rawSelected)

	// Inject sorted confirm items and enable triage step
	m.confirmItems = items
	m.confirmSelected = selected
	m.hasTriageStep = true

	m.confirmCursor = 0
	m.confirmOffset = 0
	m.confirmFocus = triageZoneItems

	// Rebuild shell labels to include Triage step
	m.shell.SetSteps(m.buildShellLabels())
	m.shell.SetActive(m.shellIndexForStep(addStepTriage))

	// Set step to triage (bypassing wizard flow — test-only)
	m.step = addStepTriage

	return m
}

// snapshotWizard captures the wizard view with the same normalization as snapshotApp.
func snapshotWizard(t *testing.T, m *addWizardModel) string {
	t.Helper()
	return normalizeSnapshot(m.View())
}

// assertTriageRender checks the semantic properties of a triage-step render:
// legend, section headers, item names, and correct checkbox state.
// These assertions catch regressions that would be silently blessed by -update-golden.
func assertTriageRender(t *testing.T, view string) {
	t.Helper()
	assertContains(t, view, "Triage: detected content") // wizard step label
	assertContains(t, view, "Match confidence")         // legend line
	// Section headers for each type group present.
	assertContains(t, view, "Skills")
	assertContains(t, view, "Agents")
	assertContains(t, view, "Rules")
	// All five items rendered (80x30 column truncates to "experimental-l...").
	assertContains(t, view, "security-scanner")
	assertContains(t, view, "code-style-guide")
	assertContains(t, view, "user-custom-agent")
	assertContains(t, view, "api-docs-helper")
	// Checked marker (✓) precedes pre-selected items.
	assertContains(t, view, "✓ security-scanner")
	assertContains(t, view, "✓ user-custom-agent")
	// Unchecked items must NOT render the check glyph.
	assertNotContains(t, view, "✓ code-style-guide")
	assertNotContains(t, view, "✓ api-docs-helper")
}

func TestGolden_Triage_80x30(t *testing.T) {
	t.Parallel()
	m := setupTriageWizard(t, 80, 30)
	view := m.View()
	assertTriageRender(t, view)
	requireGolden(t, "triage-80x30", snapshotWizard(t, m))
}

func TestGolden_Triage_120x40(t *testing.T) {
	t.Parallel()
	m := setupTriageWizard(t, 120, 40)
	view := m.View()
	assertTriageRender(t, view)
	requireGolden(t, "triage-120x40", snapshotWizard(t, m))
}
