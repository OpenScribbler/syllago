package tui

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/analyzer"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// testConfirmItems returns a set of addConfirmItems with varied tiers for triage golden tests.
func testConfirmItems() []addConfirmItem {
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
// High and User items are pre-checked; Medium and Low are unchecked.
func setupTriageWizard(t *testing.T, w, h int) *addWizardModel {
	t.Helper()
	m := testOpenAddWizard(t)
	m.width = w
	m.height = h
	m.shell.SetWidth(w)

	// Inject confirm items and enable triage step
	m.confirmItems = testConfirmItems()
	m.hasTriageStep = true

	// Pre-populate confirmSelected: High(0) and User(3) checked, Medium(1,4) and Low(2) unchecked
	m.confirmSelected = map[int]bool{
		0: true,  // security-scanner (High) — checked
		1: false, // code-style-guide (Medium) — unchecked
		2: false, // experimental-linter (Low) — unchecked
		3: true,  // user-custom-agent (User) — checked
		4: false, // api-docs-helper (Medium) — unchecked
	}
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

func TestGolden_Triage_80x30(t *testing.T) {
	t.Parallel()
	m := setupTriageWizard(t, 80, 30)
	requireGolden(t, "triage-80x30", snapshotWizard(t, m))
}

func TestGolden_Triage_120x40(t *testing.T) {
	t.Parallel()
	m := setupTriageWizard(t, 120, 40)
	requireGolden(t, "triage-120x40", snapshotWizard(t, m))
}
