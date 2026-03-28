package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// --- Test helpers ---

func testAddProviders() []provider.Provider {
	return []provider.Provider{
		testInstallProvider("Claude Code", "claude-code", true),
		testInstallProvider("Cursor", "cursor", true),
		testInstallProvider("Windsurf", "windsurf", false), // not detected
	}
}

func testAddRegistries() []catalog.RegistrySource {
	return []catalog.RegistrySource{
		{Name: "test-registry", Path: "/tmp/test-registry"},
	}
}

func testAddConfig() *config.Config {
	return &config.Config{}
}

func testOpenAddWizard(t *testing.T) *addWizardModel {
	t.Helper()
	m := openAddWizard(testAddProviders(), testAddRegistries(), testAddConfig(), "/tmp/project", "/tmp/content", "")
	m.width = 80
	m.height = 30
	m.shell.SetWidth(80)
	return m
}

func testOpenAddWizardPreFiltered(t *testing.T, ct catalog.ContentType) *addWizardModel {
	t.Helper()
	m := openAddWizard(testAddProviders(), testAddRegistries(), testAddConfig(), "/tmp/project", "/tmp/content", ct)
	m.width = 80
	m.height = 30
	m.shell.SetWidth(80)
	return m
}

func testDiscoveryItems() []addDiscoveryItem {
	return []addDiscoveryItem{
		{
			name: "alpha-rule", itemType: catalog.Rules,
			status: add.StatusNew, path: "/tmp/rules/alpha",
			underlying: &add.DiscoveryItem{Name: "alpha-rule", Type: catalog.Rules, Path: "/tmp/rules/alpha", Status: add.StatusNew},
		},
		{
			name: "beta-skill", itemType: catalog.Skills,
			status: add.StatusOutdated, path: "/tmp/skills/beta",
			overwrite:  true,
			underlying: &add.DiscoveryItem{Name: "beta-skill", Type: catalog.Skills, Path: "/tmp/skills/beta", Status: add.StatusOutdated},
		},
		{
			name: "gamma-rule", itemType: catalog.Rules,
			status: add.StatusInLibrary, path: "/tmp/rules/gamma",
			underlying: &add.DiscoveryItem{Name: "gamma-rule", Type: catalog.Rules, Path: "/tmp/rules/gamma", Status: add.StatusInLibrary},
		},
	}
}

// injectDiscoveryResults simulates a discovery completion message.
func injectDiscoveryResults(t *testing.T, m *addWizardModel, items []addDiscoveryItem) *addWizardModel {
	t.Helper()
	m, _ = m.Update(addDiscoveryDoneMsg{
		seq:   m.seq,
		items: items,
	})
	return m
}

// --- Constructor tests ---

func TestAddWizard_Open_5Step(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	if m.step != addStepSource {
		t.Fatalf("expected step Source, got %d", m.step)
	}
	if len(m.shell.steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(m.shell.steps))
	}
	// Providers detected, default cursor = 0 (Provider)
	if m.sourceCursor != 0 {
		t.Fatalf("expected sourceCursor 0, got %d", m.sourceCursor)
	}
}

func TestAddWizard_Open_4Step(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizardPreFiltered(t, catalog.Rules)

	if len(m.shell.steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(m.shell.steps))
	}
	if m.preFilterType != catalog.Rules {
		t.Fatalf("expected preFilterType Rules, got %s", m.preFilterType)
	}
}

func TestAddWizard_Open_NoProviders(t *testing.T) {
	t.Parallel()
	m := openAddWizard(nil, testAddRegistries(), testAddConfig(), "/tmp/project", "/tmp/content", "")
	if m.sourceCursor != 2 {
		t.Fatalf("expected sourceCursor 2 (Local) when no providers, got %d", m.sourceCursor)
	}
}

// --- Source step tests ---

func TestAddWizard_Source_Navigation(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	// Down from 0 to 1
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.sourceCursor != 1 {
		t.Fatalf("expected cursor 1, got %d", m.sourceCursor)
	}

	// Down to 3 (max)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.sourceCursor != 3 {
		t.Fatalf("expected cursor 3, got %d", m.sourceCursor)
	}

	// Down past end: stays at 3
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.sourceCursor != 3 {
		t.Fatalf("expected cursor clamped at 3, got %d", m.sourceCursor)
	}
}

func TestAddWizard_Source_ProviderExpand(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	// Enter on Provider expands sub-list
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.sourceExpanded {
		t.Fatal("expected sourceExpanded after Enter on Provider")
	}
	if m.providerCursor != 0 {
		t.Fatalf("expected providerCursor 0, got %d", m.providerCursor)
	}

	// Down in sub-list
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.providerCursor != 1 {
		t.Fatalf("expected providerCursor 1, got %d", m.providerCursor)
	}

	// Esc collapses sub-list (not close wizard)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.sourceExpanded {
		t.Fatal("expected sourceExpanded false after Esc")
	}
}

func TestAddWizard_Source_ProviderSelectAdvances(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	// Enter to expand, Enter to select
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // expand
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // select provider 0

	if m.source != addSourceProvider {
		t.Fatalf("expected source Provider, got %d", m.source)
	}
	if m.step != addStepType {
		t.Fatalf("expected step Type, got %d", m.step)
	}
}

func TestAddWizard_Source_RegistryExpand(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	// Navigate to Registry (cursor 1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // expand

	if !m.sourceExpanded {
		t.Fatal("expected sourceExpanded for registry")
	}

	// Select registry
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.source != addSourceRegistry {
		t.Fatalf("expected source Registry, got %d", m.source)
	}
}

func TestAddWizard_Source_LocalPathInput(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	// Navigate to Local (cursor 2) and Enter
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.inputActive {
		t.Fatal("expected inputActive for Local")
	}

	// Type a relative path -> should error
	for _, r := range "relative/path" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.sourceErr == "" {
		t.Fatal("expected error for relative path")
	}
}

func TestAddWizard_Source_GitURLValidation(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	// Navigate to Git (cursor 3) and Enter
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.inputActive {
		t.Fatal("expected inputActive for Git")
	}

	// Type ext::malicious
	for _, r := range "ext::malicious" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.sourceErr == "" {
		t.Fatal("expected error for ext:: URL")
	}
	assertContains(t, m.sourceErr, "ext::")
}

func TestAddWizard_Source_EscClosesWizard(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if cmd == nil {
		t.Fatal("expected cmd from Esc on Source")
	}
	msg := cmd()
	if _, ok := msg.(addCloseMsg); !ok {
		t.Fatalf("expected addCloseMsg, got %T", msg)
	}
}

func TestAddWizard_Source_EscFromInputClearsFirst(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	// Go to Local, enter input mode, type something
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	for _, r := range "/tmp" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Esc clears input first (doesn't close wizard)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.pathInput != "" {
		t.Fatalf("expected pathInput cleared, got %q", m.pathInput)
	}
	if m.inputActive {
		t.Fatal("expected inputActive false after Esc with text")
	}
}

func TestAddWizard_Source_PreFilterSkipsType(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizardPreFiltered(t, catalog.Rules)

	// Enter on Provider -> expand -> select -> should go to Discovery (skip Type)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // expand
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // select

	if m.step != addStepDiscovery {
		t.Fatalf("expected step Discovery (skipping Type), got %d", m.step)
	}
}

// --- Type step tests ---

func TestAddWizard_Type_CheckboxToggle(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	// Navigate to Type step
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // expand provider
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // select provider

	if m.step != addStepType {
		t.Fatalf("expected Type step, got %d", m.step)
	}

	// Deselect all with 'n'
	m, _ = m.Update(keyRune('n'))
	if len(m.selectedTypes()) != 0 {
		t.Fatalf("expected 0 selected types after 'n', got %d", len(m.selectedTypes()))
	}

	// Select all with 'a'
	m, _ = m.Update(keyRune('a'))
	if len(m.selectedTypes()) == 0 {
		t.Fatal("expected > 0 selected types after 'a'")
	}
}

func TestAddWizard_Type_EnterWithNoneIsNoop(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	// Get to Type step
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Deselect all
	m, _ = m.Update(keyRune('n'))

	// Enter with 0 selected: stays on Type
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != addStepType {
		t.Fatalf("expected to stay on Type when 0 selected, got step %d", m.step)
	}
}

func TestAddWizard_Type_EscGoesBackToSource(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.step != addStepSource {
		t.Fatalf("expected Source step after Esc, got %d", m.step)
	}
}

// --- Discovery step tests ---

func TestAddWizard_Discovery_StaleSeqDropped(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	// Get to Discovery
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // expand
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // select provider
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // advance from Type (all selected)

	// Save current seq
	currentSeq := m.seq

	// Inject stale result (wrong seq)
	m, _ = m.Update(addDiscoveryDoneMsg{
		seq:   currentSeq - 1,
		items: testDiscoveryItems(),
	})

	// Should still be discovering (stale msg dropped)
	if !m.discovering {
		t.Fatal("expected still discovering after stale msg")
	}
}

func TestAddWizard_Discovery_ResultsPopulate(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Inject valid results
	m = injectDiscoveryResults(t, m, testDiscoveryItems())

	if m.discovering {
		t.Fatal("expected discovering=false after results")
	}
	if len(m.discoveredItems) != 3 {
		t.Fatalf("expected 3 discovered items, got %d", len(m.discoveredItems))
	}

	// Check pre-selection: New+Outdated checked, InLibrary unchecked
	sel := m.discoveryList.SelectedIndices()
	if len(sel) != 2 {
		t.Fatalf("expected 2 pre-selected, got %d: %v", len(sel), sel)
	}
}

func TestAddWizard_Discovery_ErrorRendersRetry(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Inject error
	m, _ = m.Update(addDiscoveryDoneMsg{
		seq: m.seq,
		err: &testError{"scan failed"},
	})

	if m.discoveryErr == "" {
		t.Fatal("expected discoveryErr set")
	}

	view := m.View()
	assertContains(t, view, "scan failed")
	assertContains(t, view, "[r] Retry")
}

func TestAddWizard_Discovery_EscDuringScan(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// During scan, Esc goes back
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.step != addStepType {
		t.Fatalf("expected Type step after Esc during scan, got %d", m.step)
	}
}

func TestAddWizard_Discovery_RightAdvancesToReview(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())

	// Right arrow advances
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.step != addStepReview {
		t.Fatalf("expected Review step, got %d", m.step)
	}
}

// --- Review step tests ---

func TestAddWizard_Review_ButtonNavigation(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight}) // advance to Review

	// Default focus is buttons, cursor on Back (1)
	if m.reviewZone != addReviewZoneButtons {
		t.Fatalf("expected buttons zone, got %d", m.reviewZone)
	}
	if m.buttonCursor != 1 {
		t.Fatalf("expected button cursor 1 (Back), got %d", m.buttonCursor)
	}

	// Right moves to Add (2)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.buttonCursor != 2 {
		t.Fatalf("expected button cursor 2, got %d", m.buttonCursor)
	}

	// Left back to Back (1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.buttonCursor != 1 {
		t.Fatalf("expected button cursor 1, got %d", m.buttonCursor)
	}
}

func TestAddWizard_Review_AddAdvancesToExecute(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Navigate to Add button
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight}) // cursor 2

	// Press Enter to Add
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.step != addStepExecute {
		t.Fatalf("expected Execute step, got %d", m.step)
	}
	if !m.reviewAcknowledged {
		t.Fatal("expected reviewAcknowledged=true")
	}
}

func TestAddWizard_Review_BackGoesToDiscovery(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Default cursor is on Back (1), press Enter
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.step != addStepDiscovery {
		t.Fatalf("expected Discovery step after Back, got %d", m.step)
	}
}

func TestAddWizard_Review_CancelClosesWizard(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Move to Cancel (0)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Cancel")
	}
	msg := cmd()
	if _, ok := msg.(addCloseMsg); !ok {
		t.Fatalf("expected addCloseMsg, got %T", msg)
	}
}

func TestAddWizard_Review_EscGoesBackToDiscovery(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.step != addStepDiscovery {
		t.Fatalf("expected Discovery step after Esc from Review, got %d", m.step)
	}
}

func TestAddWizard_Review_ConflictsDetected(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	// beta-skill is outdated -> should be in conflicts
	if len(m.conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(m.conflicts))
	}
}

func TestAddWizard_Review_TabCyclesZones(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Add risks to test items
	items := testDiscoveryItems()
	items[0].risks = []catalog.RiskIndicator{{Label: "test risk", Level: catalog.RiskMedium}}
	m = injectDiscoveryResults(t, m, items)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Default: buttons
	if m.reviewZone != addReviewZoneButtons {
		t.Fatalf("expected buttons zone, got %d", m.reviewZone)
	}

	// Tab cycles: buttons -> risks -> items -> buttons
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.reviewZone != addReviewZoneRisks {
		t.Fatalf("expected risks zone after Tab, got %d", m.reviewZone)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.reviewZone != addReviewZoneItems {
		t.Fatalf("expected items zone after Tab, got %d", m.reviewZone)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.reviewZone != addReviewZoneButtons {
		t.Fatalf("expected buttons zone after Tab, got %d", m.reviewZone)
	}
}

// --- Execute step tests ---

func TestAddWizard_Execute_ItemDoneProgresses(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Now in Execute step
	if m.step != addStepExecute {
		t.Fatalf("expected Execute step, got %d", m.step)
	}

	// Simulate first item done
	m, _ = m.Update(addExecItemDoneMsg{
		seq:    m.seq,
		index:  0,
		result: addExecResult{name: "alpha-rule", status: "added"},
	})

	if m.executeCurrent != 1 {
		t.Fatalf("expected executeCurrent 1, got %d", m.executeCurrent)
	}
}

func TestAddWizard_Execute_AllDone(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	selected := m.selectedItems()
	for i := range selected {
		m, _ = m.Update(addExecItemDoneMsg{
			seq:    m.seq,
			index:  i,
			result: addExecResult{name: selected[i].name, status: "added"},
		})
	}

	if !m.executeDone {
		t.Fatal("expected executeDone=true")
	}

	view := m.View()
	assertContains(t, view, "Done!")
	assertContains(t, view, "Go to Library")
}

func TestAddWizard_Execute_EscCancelsRemaining(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Cancel before any complete
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.executeCancelled {
		t.Fatal("expected executeCancelled=true after Esc")
	}
}

func TestAddWizard_Execute_EnterOnDoneCloses(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Complete all items
	selected := m.selectedItems()
	for i := range selected {
		m, _ = m.Update(addExecItemDoneMsg{
			seq:    m.seq,
			index:  i,
			result: addExecResult{name: selected[i].name, status: "added"},
		})
	}

	// Enter on done screen
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Enter on done")
	}
	msg := cmd()
	if _, ok := msg.(addCloseMsg); !ok {
		t.Fatalf("expected addCloseMsg, got %T", msg)
	}
}

// --- View tests ---

func TestAddWizard_View_SourceStep(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)
	view := m.View()

	assertContains(t, view, "Where is the content?")
	assertContains(t, view, "Provider")
	assertContains(t, view, "Registry")
	assertContains(t, view, "Local Path")
	assertContains(t, view, "Git URL")
	assertContains(t, view, "Add") // shell title
}

func TestAddWizard_View_TypeStep(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	view := m.View()
	assertContains(t, view, "What type of content?")
	assertContains(t, view, "Rules")
	assertContains(t, view, "Skills")
}

func TestAddWizard_View_DiscoveryLoading(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	view := m.View()
	assertContains(t, view, "Discovering content...")
}

func TestAddWizard_View_DiscoveryResults(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())

	view := m.View()
	assertContains(t, view, "Found 3 items")
	assertContains(t, view, "2 selected")
}

func TestAddWizard_View_ReviewStep(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	view := m.View()
	assertContains(t, view, "Adding 2 items")
	assertContains(t, view, "Cancel")
	assertContains(t, view, "Back")
}

func TestAddWizard_View_ExecuteStep(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	view := m.View()
	assertContains(t, view, "Adding items")
}

func TestAddWizard_View_ShellIndex4Step(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizardPreFiltered(t, catalog.Rules)

	view := m.View()
	assertContains(t, view, "[1 Source]")
	assertContains(t, view, "[2 Discovery]")
	assertNotContains(t, view, "Type")
}

func TestAddWizard_View_ShellIndex5Step(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	view := m.View()
	assertContains(t, view, "[1 Source]")
	assertContains(t, view, "[2 Type]")
	assertContains(t, view, "[3 Discovery]")
}

// --- Helper ---

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
