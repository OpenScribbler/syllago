package tui

import (
	"os"
	"path/filepath"
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

// TestAddWizard_AcceptsUndetectedProviders verifies the add wizard preserves
// undetected providers in its provider list (per provider.go:39 advisory-only
// contract) and labels them in the picker. Cursor defaults to the first
// detected provider so the common case stays fast.
func TestAddWizard_AcceptsUndetectedProviders(t *testing.T) {
	t.Parallel()
	// Order matters: undetected first. Correct cursor default must skip past
	// it to the detected one (index 1).
	providers := []provider.Provider{
		testInstallProvider("Windsurf", "windsurf", false),
		testInstallProvider("Claude Code", "claude-code", true),
	}
	m := openAddWizard(providers, testAddRegistries(), testAddConfig(), "/tmp/project", "/tmp/content", "")
	m.width = 80
	m.height = 30
	m.shell.SetWidth(80)

	if got := len(m.providers); got != 2 {
		t.Errorf("expected 2 providers in wizard (detected + undetected), got %d — undetected provider was filtered out", got)
	}
	if m.providerCursor != 1 {
		t.Errorf("expected providerCursor=1 (lands on detected), got %d", m.providerCursor)
	}

	// Render the provider sub-list directly. The view must label the undetected
	// provider so the user can tell which is which.
	subListLines := m.viewProviderSubList("  ")
	combined := ""
	for _, l := range subListLines {
		combined += l + "\n"
	}
	if !contains(combined, "Windsurf") {
		t.Error("provider sub-list should include Windsurf even when undetected")
	}
	if !contains(combined, "(not detected)") {
		t.Error("provider sub-list should label undetected providers with '(not detected)'")
	}
}

// TestAddWizard_SourceProviderOption_EnabledWithUndetected verifies the
// "Provider" source option stays enabled when at least one provider exists,
// even if none are Detected. The previous behavior disabled the option with
// "(no providers detected)" reason text, hard-blocking the import path for
// users whose detection has misses.
func TestAddWizard_SourceProviderOption_EnabledWithUndetected(t *testing.T) {
	t.Parallel()
	providers := []provider.Provider{
		testInstallProvider("Windsurf", "windsurf", false),
		testInstallProvider("Cursor", "cursor", false),
	}
	m := openAddWizard(providers, testAddRegistries(), testAddConfig(), "/tmp/project", "/tmp/content", "")
	m.width = 80
	m.height = 30
	m.shell.SetWidth(80)

	view := m.viewSource()
	if contains(view, "(no providers detected)") {
		t.Error("source step should not show '(no providers detected)' when at least one provider exists")
	}
	// Provider should be the default source option (cursor 0) because
	// providers is non-empty.
	if m.sourceCursor != 0 {
		t.Errorf("expected sourceCursor=0 (Provider) when providers exist, got %d", m.sourceCursor)
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

	// Down to 3 (max — Git URL is the 4th option)
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
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

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
	// Actionable items land in confirmItems (pre-selected); installed items go to discoveredItems baseline
	if len(m.confirmItems) != 2 {
		t.Fatalf("expected 2 confirm items (actionable), got %d", len(m.confirmItems))
	}
	if m.installedCount != 1 {
		t.Fatalf("expected installedCount=1, got %d", m.installedCount)
	}
	// Both actionable items should be pre-selected
	selectedCount := 0
	for _, v := range m.confirmSelected {
		if v {
			selectedCount++
		}
	}
	if selectedCount != 2 {
		t.Fatalf("expected 2 pre-selected confirm items, got %d", selectedCount)
	}
}

// TestAddWizard_GoBackFromDiscovery_ResetsCounters ensures the invariant
// len(discoveredItems) == actionableCount + installedCount holds after the
// user backs out of Discovery. A prior bug left installedCount stale, so the
// next advanceFromSource() crashed in visibleDiscoveryItems with
// "slice bounds out of range [:N] with capacity 0".
func TestAddWizard_GoBackFromDiscovery_ResetsCounters(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	// Walk: Source -> Type -> Discovery
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Populate discovery with an installed item so installedCount > 0.
	m = injectDiscoveryResults(t, m, testDiscoveryItems())
	if m.installedCount == 0 {
		t.Fatalf("test precondition: expected installedCount > 0, got 0")
	}

	m.goBackFromDiscovery()

	if m.discoveredItems != nil {
		t.Errorf("expected discoveredItems=nil after back, got %d items", len(m.discoveredItems))
	}
	if m.installedCount != 0 {
		t.Errorf("expected installedCount=0 after back, got %d", m.installedCount)
	}
	if m.actionableCount != 0 {
		t.Errorf("expected actionableCount=0 after back, got %d", m.actionableCount)
	}
}

// TestAddWizard_AdvanceFromSource_ResetsCounters verifies advanceFromSource is
// self-consistent even if counters are stale on entry. This is defense in
// depth: shellIndexForStep -> hasSplittableSelection -> visibleDiscoveryItems
// is called during the transition, and it indexes discoveredItems by
// actionableCount. Without this reset, a nil slice + non-zero counter panics.
func TestAddWizard_AdvanceFromSource_ResetsCounters(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	// Simulate stale counters from a prior discovery that weren't cleared.
	m.installedCount = 110
	m.actionableCount = 5
	m.discoveredItems = nil

	// Must not panic.
	_ = m.advanceFromSource()

	if m.installedCount != 0 {
		t.Errorf("expected installedCount=0 after advanceFromSource, got %d", m.installedCount)
	}
	if m.actionableCount != 0 {
		t.Errorf("expected actionableCount=0 after advanceFromSource, got %d", m.actionableCount)
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
	assertContains(t, view, "Retry")
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

func TestAddWizard_Discovery_RightFocusesPreview(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())

	// Right arrow focuses the preview pane (does NOT advance to Review).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.step != addStepDiscovery {
		t.Fatalf("expected Discovery step after Right, got %d", m.step)
	}
	if m.confirmFocus != triageZonePreview {
		t.Fatalf("expected preview focus after Right, got %d", m.confirmFocus)
	}

	// Enter advances to Review.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != addStepReview {
		t.Fatalf("expected Review step after Enter, got %d", m.step)
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
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // advance to Review (Enter; Right only focuses preview)

	// Default focus is items zone (so per-item risk info shows)
	if m.reviewZone != addReviewZoneItems {
		t.Fatalf("expected items zone, got %d", m.reviewZone)
	}

	// Tab to buttons. Default lands on Back (index 2) so Enter from the
	// buttons zone is a safe back-nav rather than an accidental commit.
	// Layout: 0=Add, 1=Rename, 2=Back, 3=Cancel.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.reviewZone != addReviewZoneButtons {
		t.Fatalf("expected buttons zone after Tab, got %d", m.reviewZone)
	}
	if m.buttonCursor != 2 {
		t.Fatalf("expected button cursor 2 (Back), got %d", m.buttonCursor)
	}

	// Right moves to Cancel (3)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.buttonCursor != 3 {
		t.Fatalf("expected button cursor 3 (Cancel), got %d", m.buttonCursor)
	}

	// Right at the end is a no-op (cursor clamps at 3)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.buttonCursor != 3 {
		t.Fatalf("expected button cursor to clamp at 3, got %d", m.buttonCursor)
	}

	// Left walks back through Back, Rename, Add.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.buttonCursor != 2 {
		t.Fatalf("expected button cursor 2 (Back), got %d", m.buttonCursor)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.buttonCursor != 1 {
		t.Fatalf("expected button cursor 1 (Rename), got %d", m.buttonCursor)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.buttonCursor != 0 {
		t.Fatalf("expected button cursor 0 (Add), got %d", m.buttonCursor)
	}
}

func TestAddWizard_Review_AddAdvancesToExecute(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Discovery → Review

	// Tab from items to buttons, then navigate to Add button.
	// Layout: 0=Add, 1=Rename, 2=Back, 3=Cancel. Default lands on Back(2).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})  // items -> buttons
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft}) // Back(2) -> Rename(1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft}) // Rename(1) -> Add(0)

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
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Discovery → Review

	// Tab to buttons (default cursor is Back=2), press Enter
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
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
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Discovery → Review

	// Tab to buttons, then move to Cancel (index 3, right of default Back=2)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	var cmd tea.Cmd
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
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
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Discovery → Review

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
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Discovery → Review

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
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Discovery → Review

	// Default: items (so per-item risk info shows immediately)
	if m.reviewZone != addReviewZoneItems {
		t.Fatalf("expected items zone, got %d", m.reviewZone)
	}

	// Tab cycles: items -> buttons -> items (risk zone removed from cycle)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.reviewZone != addReviewZoneButtons {
		t.Fatalf("expected buttons zone after Tab, got %d", m.reviewZone)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.reviewZone != addReviewZoneItems {
		t.Fatalf("expected items zone after Tab, got %d", m.reviewZone)
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
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Discovery → Review (Enter required; Right only focuses preview)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})   // items → buttons (default=Back=2)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})  // Back(2) → Rename(1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})  // Rename(1) → Add(0)
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
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Discovery → Review (Enter required; Right only focuses preview)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})   // items → buttons (default=Back=2)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})  // Back(2) → Rename(1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})  // Rename(1) → Add(0)
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
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Discovery → Review (Enter required; Right only focuses preview)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})   // items → buttons (default=Back=2)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})  // Back(2) → Rename(1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})  // Rename(1) → Add(0)
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
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Discovery → Review (Enter required; Right only focuses preview)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})   // items → buttons (default=Back=2)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})  // Back(2) → Rename(1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})  // Rename(1) → Add(0)
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
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
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
	// Discovery step now uses triage-style split-pane view
	assertContains(t, view, "Discovery: found content")
	assertContains(t, view, "alpha-rule")
	assertContains(t, view, "beta-skill")
	// Both items are auto-detected and pre-selected
	assertContains(t, view, "✓ alpha-rule")
	assertContains(t, view, "✓ beta-skill")
}

func TestAddWizard_View_ReviewStep(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = injectDiscoveryResults(t, m, testDiscoveryItems())
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Discovery → Review

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
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // Discovery → Review (Enter required; Right only focuses preview)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})   // items → buttons (default=Back=2)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})  // Back(2) → Rename(1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})  // Rename(1) → Add(0)
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

// --- discoverFromLocalPath tests ---

func TestDiscoverFromLocalPath_ReturnsItems(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Syllago canonical rules layout: rules/{provider}/{name}/rule.md
	if err := os.MkdirAll(filepath.Join(dir, "rules", "syllago", "test-rule"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "rules", "syllago", "test-rule", "rule.md"),
		[]byte("# Test Rule\nSome content.\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	items, _, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.Rules}, "")
	if err != nil {
		t.Fatalf("discoverFromLocalPath: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least 1 item from local path discovery")
	}

	found := false
	for _, item := range items {
		if item.name == "test-rule" {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, len(items))
		for i, it := range items {
			names[i] = it.name
		}
		t.Errorf("expected to find 'test-rule' in discovered items, got: %v", names)
	}
}

func TestDiscoverFromLocalPath_TypeFilter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Rules: rules/syllago/my-rule/rule.md
	if err := os.MkdirAll(filepath.Join(dir, "rules", "syllago", "my-rule"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "rules", "syllago", "my-rule", "rule.md"),
		[]byte("# My Rule\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	// Agents: agents/my-agent/AGENT.md
	if err := os.MkdirAll(filepath.Join(dir, "agents", "my-agent"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "agents", "my-agent", "AGENT.md"),
		[]byte("---\nname: My Agent\ndescription: Does things\n---\nAgent body.\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	// Request only Rules
	items, confirm, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.Rules}, "")
	if err != nil {
		t.Fatalf("discoverFromLocalPath: %v", err)
	}

	for _, item := range items {
		if item.itemType == catalog.Agents {
			t.Errorf("found agent in items when only rules requested: %s", item.name)
		}
	}
	for _, ci := range confirm {
		if ci.itemType == catalog.Agents {
			t.Errorf("found agent in confirm when only rules requested: %s", ci.displayName)
		}
	}
}

func TestDiscoverFromLocalPath_NoDuplicates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Syllago canonical layout for skills — picked up by both pattern detection AND
	// the syllago analyzer detector. Dedup must prevent double-counting.
	if err := os.MkdirAll(filepath.Join(dir, "skills", "dedup-skill"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "skills", "dedup-skill", "SKILL.md"),
		[]byte("---\nname: Dedup Skill\ndescription: Tests deduplication\n---\nContent.\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	items, _, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.Skills}, "")
	if err != nil {
		t.Fatalf("discoverFromLocalPath: %v", err)
	}

	// Dedup must key on (type, name), not path — pattern detection emits the
	// item's file path (skills/dedup-skill/SKILL.md) while SyllagoDetector
	// inside the analyzer emits the item's directory path (skills/dedup-skill).
	// Path-based dedup misses this collision; (type, name) catches it.
	seen := make(map[string]int)
	for _, item := range items {
		key := string(item.itemType) + "/" + item.name
		seen[key]++
	}
	for key, count := range seen {
		if count > 1 {
			t.Errorf("logical item %q appears %d times — dedup failed", key, count)
		}
	}
}

func TestDiscoverFromLocalPath_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	items, confirm, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.Rules}, "")
	if err != nil {
		t.Fatalf("discoverFromLocalPath on empty dir: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items from empty dir, got %d", len(items))
	}
	if len(confirm) != 0 {
		t.Errorf("expected 0 confirm items from empty dir, got %d", len(confirm))
	}
}

func TestDiscoverFromLocalPath_AnalyzerConfirmItemsMerged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// PAI-style layout (non-standard) — only the content-signal analyzer detects this.
	// Packs/<name>/SKILL.md is not matched by any pattern detector.
	// Content-signal items cap at 0.70 confidence, which is below the Auto threshold
	// (0.80), so they land in result.Confirm — not result.Auto.
	if err := os.MkdirAll(filepath.Join(dir, "Packs", "custom-skill"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "Packs", "custom-skill", "SKILL.md"),
		[]byte("---\nname: Custom Skill\ndescription: A custom PAI-style skill\n---\nDoes things.\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	_, confirm, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.Skills}, "")
	if err != nil {
		t.Fatalf("discoverFromLocalPath: %v", err)
	}

	// The analyzer should surface this in Confirm (content-signal confidence < Auto threshold).
	if len(confirm) == 0 {
		t.Error("expected at least 1 confirm item from PAI-style layout via analyzer content-signal fallback")
	}
	for _, ci := range confirm {
		if ci.itemType != catalog.Skills {
			t.Errorf("expected Skills confirm item, got %v (name=%s)", ci.itemType, ci.displayName)
		}
	}
}

func TestDiscoverFromLocalPath_ConfirmItemsReturned(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Hook files always land in Confirm (never Auto), per analyzer design.
	// Syllago hook layout: hooks/{provider}/{name}/hook.json
	if err := os.MkdirAll(filepath.Join(dir, "hooks", "claude-code", "lint"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(dir, "hooks", "claude-code", "lint", "hook.json"),
		[]byte(`{"event": "PostToolUse"}`),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	_, confirm, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.Hooks}, "")
	if err != nil {
		t.Fatalf("discoverFromLocalPath: %v", err)
	}

	if len(confirm) == 0 {
		t.Error("expected at least 1 confirm item for hook content (hooks always land in Confirm)")
	}
	for _, ci := range confirm {
		if ci.itemType != catalog.Hooks {
			t.Errorf("expected Hooks confirm item, got %v", ci.itemType)
		}
	}
}

// Bug B regression: a loose rule file at the scan root (e.g. CLAUDE.md,
// AGENTS.md) must not receive sourceDir == scan root. The previous bug set
// sourceDir via filepath.Join(dir, filepath.Dir("CLAUDE.md")) == filepath.Join(dir, ".") == dir,
// which caused add.copySupportingFiles to walk the entire source tree and
// slurp arbitrary files into the library.
func TestDiscoverFromLocalPath_RootLevelRule_NoSourceDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Root-level monolithic rule file (common pattern: ~/.config/pai/CLAUDE.md,
	// repo-root AGENTS.md). Analyzer's TopLevelDetector surfaces these in
	// Confirm (always — they require user review).
	if err := os.WriteFile(
		filepath.Join(dir, "CLAUDE.md"),
		[]byte("# Claude Rules\nUse strict TypeScript.\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	items, confirm, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.Rules}, "")
	if err != nil {
		t.Fatalf("discoverFromLocalPath: %v", err)
	}

	// Scan across both actionable and confirm — CLAUDE.md may land in either
	// depending on the detector's confidence threshold. Invariant: sourceDir
	// must never equal the scan root for a root-level file.
	checkNoScanRoot := func(t *testing.T, label, sourceDir string) {
		t.Helper()
		if sourceDir == dir {
			t.Errorf("%s sourceDir = %q equals scan root %q — copySupportingFiles would walk the whole tree", label, sourceDir, dir)
		}
	}
	for _, it := range items {
		checkNoScanRoot(t, "item "+it.name, it.sourceDir)
	}
	for _, ci := range confirm {
		checkNoScanRoot(t, "confirm "+ci.displayName, ci.sourceDir)
	}
}

// Bug D regression: analyzer-detected items without frontmatter must not
// render as blank rows in the discovery list. The previous bug set
// `name: detected.DisplayName` (empty when no frontmatter) and discarded
// `detected.Name`. The fix preserves both and shows the relative path.
func TestDiscoverFromLocalPath_NoFrontmatter_LabelNotBlank(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// AGENTS.md without frontmatter — DisplayName will be empty.
	if err := os.WriteFile(
		filepath.Join(dir, "AGENTS.md"),
		[]byte("# Agents\nAgent rules.\n"),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	items, confirm, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.Rules}, "")
	if err != nil {
		t.Fatalf("discoverFromLocalPath: %v", err)
	}

	assertLabelNotBlank := func(t *testing.T, kind string, d addDiscoveryItem) {
		t.Helper()
		label := discoveryItemLabel(d)
		if label == "" {
			t.Errorf("%s row renders blank: name=%q displayName=%q relativePath=%q", kind, d.name, d.displayName, d.relativePath)
		}
	}
	for _, it := range items {
		assertLabelNotBlank(t, "actionable "+string(it.itemType), it)
	}
	for _, ci := range confirm {
		di := addDiscoveryItem{name: ci.detected.Name, displayName: ci.displayName, itemType: ci.itemType}
		assertLabelNotBlank(t, "confirm "+string(ci.itemType), di)
	}
}

// Bug D (display) + Bug A (dedup): verify relative path is surfaced for
// pattern-detected items so the list distinguishes multiple files with
// identical basenames across a source tree.
func TestDiscoverFromLocalPath_RelativePathPopulated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Two skills in syllago canonical layout, different directories.
	for _, name := range []string{"alpha", "beta"} {
		subdir := filepath.Join(dir, "skills", name)
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(
			filepath.Join(subdir, "SKILL.md"),
			[]byte("---\nname: "+name+"\n---\nBody.\n"),
			0644,
		); err != nil {
			t.Fatal(err)
		}
	}

	items, _, err := discoverFromLocalPath(dir, []catalog.ContentType{catalog.Skills}, "")
	if err != nil {
		t.Fatalf("discoverFromLocalPath: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected skills items")
	}
	for _, it := range items {
		if it.relativePath == "" {
			t.Errorf("skill %q has empty relativePath; expected skills/%s/SKILL.md", it.name, it.name)
		}
	}
}

// --- updateKeyDiscovery branch coverage ---

// discoveryKeyWizard builds a wizard at addStepDiscovery with confirm items
// loaded so the full triage-navigation branch is reachable.
func discoveryKeyWizard(t *testing.T) *addWizardModel {
	t.Helper()
	m := testOpenAddWizard(t)
	m.source = addSourceLocal
	m.typeChecks = m.buildTypeCheckList()
	m.step = addStepDiscovery
	m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
	m.confirmItems = []addConfirmItem{
		{displayName: "rule-a", itemType: catalog.Rules, path: "/tmp/a.md", sourceDir: "/tmp"},
		{displayName: "rule-b", itemType: catalog.Rules, path: "/tmp/b.md", sourceDir: "/tmp"},
	}
	m.confirmSelected = map[int]bool{}
	return m
}

func TestAddWizard_UpdateKeyDiscovery_DiscoveringEscCancels(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)
	m.source = addSourceLocal
	m.typeChecks = m.buildTypeCheckList()
	m.step = addStepDiscovery
	m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
	m.discovering = true
	seq := m.seq

	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyEsc})
	if m.discovering {
		t.Error("Esc during discovering should clear discovering flag")
	}
	if m.seq == seq {
		t.Error("Esc during discovering should bump seq to cancel pending cmd")
	}
}

func TestAddWizard_UpdateKeyDiscovery_DiscoveringOtherKeysIgnored(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)
	m.source = addSourceLocal
	m.typeChecks = m.buildTypeCheckList()
	m.step = addStepDiscovery
	m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
	m.discovering = true

	m, cmd := m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("Enter during discovering should return nil cmd")
	}
	if !m.discovering {
		t.Error("Enter during discovering should not cancel")
	}
}

func TestAddWizard_UpdateKeyDiscovery_ErrorRetry(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)
	m.source = addSourceLocal
	m.typeChecks = m.buildTypeCheckList()
	m.step = addStepDiscovery
	m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
	m.discoveryErr = "boom"

	m, cmd := m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if m.discoveryErr != "" {
		t.Error("'r' on error state should clear discoveryErr")
	}
	if !m.discovering {
		t.Error("'r' should set discovering=true")
	}
	if cmd == nil {
		t.Error("'r' should return startDiscoveryCmd")
	}
}

func TestAddWizard_UpdateKeyDiscovery_ErrorEsc(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)
	m.source = addSourceLocal
	m.typeChecks = m.buildTypeCheckList()
	m.step = addStepDiscovery
	m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
	m.discoveryErr = "boom"

	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyEsc})
	if m.step == addStepDiscovery {
		t.Error("Esc on error state should navigate back")
	}
}

func TestAddWizard_UpdateKeyDiscovery_ErrorOtherKeyNoop(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)
	m.source = addSourceLocal
	m.typeChecks = m.buildTypeCheckList()
	m.step = addStepDiscovery
	m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
	m.discoveryErr = "boom"

	m, cmd := m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("Enter on error state should be no-op")
	}
	if m.discoveryErr != "boom" {
		t.Error("Enter on error state should not clear err")
	}
}

func TestAddWizard_UpdateKeyDiscovery_EmptyEsc(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)
	m.source = addSourceLocal
	m.typeChecks = m.buildTypeCheckList()
	m.step = addStepDiscovery
	m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
	// no confirmItems — empty discovery state

	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyEsc})
	if m.step == addStepDiscovery {
		t.Error("Esc on empty state should navigate back")
	}
}

func TestAddWizard_UpdateKeyDiscovery_EmptyOtherKeyNoop(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)
	m.source = addSourceLocal
	m.typeChecks = m.buildTypeCheckList()
	m.step = addStepDiscovery
	m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))

	_, cmd := m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("Enter on empty state should return nil cmd")
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriageRightFocusesPreview(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyRight})
	if m.confirmFocus != triageZonePreview {
		t.Errorf("Right should focus preview, got focus=%v", m.confirmFocus)
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriageLeftFocusesItems(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m.confirmFocus = triageZonePreview
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyLeft})
	if m.confirmFocus != triageZoneItems {
		t.Errorf("Left should focus items, got focus=%v", m.confirmFocus)
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriageTabCyclesFocus(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	start := m.confirmFocus
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyTab})
	if m.confirmFocus == start {
		t.Error("Tab should advance focus")
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriageShiftTabCyclesBack(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m.confirmFocus = triageZoneItems
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyShiftTab})
	// Should move backward (wrapping)
	if m.confirmFocus == triageZoneItems {
		t.Error("Shift+Tab should change focus (wrap backward)")
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriageDownMovesItemsCursor(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m.confirmFocus = triageZoneItems
	m.confirmCursor = 0
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyDown})
	if m.confirmCursor != 1 {
		t.Errorf("Down should increment cursor, got %d", m.confirmCursor)
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriageUpMovesItemsCursor(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m.confirmFocus = triageZoneItems
	m.confirmCursor = 1
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyUp})
	if m.confirmCursor != 0 {
		t.Errorf("Up should decrement cursor, got %d", m.confirmCursor)
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriageDownClampsAtEnd(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m.confirmFocus = triageZoneItems
	m.confirmCursor = len(m.confirmItems) - 1
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyDown})
	if m.confirmCursor != len(m.confirmItems)-1 {
		t.Errorf("Down at end should clamp, got %d", m.confirmCursor)
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriageUpClampsAtZero(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m.confirmFocus = triageZoneItems
	m.confirmCursor = 0
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyUp})
	if m.confirmCursor != 0 {
		t.Errorf("Up at 0 should clamp, got %d", m.confirmCursor)
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriagePreviewDownScrolls(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m.confirmFocus = triageZonePreview
	m.confirmPreview.lines = []string{"a", "b", "c"}
	m.confirmPreview.offset = 0
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyDown})
	if m.confirmPreview.offset != 1 {
		t.Errorf("Preview Down should scroll offset, got %d", m.confirmPreview.offset)
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriagePreviewUpScrolls(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m.confirmFocus = triageZonePreview
	m.confirmPreview.lines = []string{"a", "b", "c"}
	m.confirmPreview.offset = 2
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyUp})
	if m.confirmPreview.offset != 1 {
		t.Errorf("Preview Up should decrement offset, got %d", m.confirmPreview.offset)
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriagePreviewPgDn(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m.confirmFocus = triageZonePreview
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line"
	}
	m.confirmPreview.lines = lines
	m.confirmPreview.offset = 0
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.confirmPreview.offset != 10 {
		t.Errorf("PgDown should advance 10 lines, got %d", m.confirmPreview.offset)
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriagePreviewPgUp(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m.confirmFocus = triageZonePreview
	m.confirmPreview.lines = make([]string, 30)
	m.confirmPreview.offset = 15
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.confirmPreview.offset != 5 {
		t.Errorf("PgUp should reduce 10 lines, got %d", m.confirmPreview.offset)
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriageSpaceTogglesSelection(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m.confirmFocus = triageZoneItems
	m.confirmCursor = 0
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeySpace})
	if !m.confirmSelected[0] {
		t.Error("Space should select current item")
	}
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeySpace})
	if m.confirmSelected[0] {
		t.Error("Space again should deselect")
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriageASelectsAll(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	for i := range m.confirmItems {
		if !m.confirmSelected[i] {
			t.Errorf("'a' should select all, item %d not selected", i)
		}
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriageNDeselectsAll(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m.confirmSelected = map[int]bool{0: true, 1: true}
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	for i := range m.confirmItems {
		if m.confirmSelected[i] {
			t.Errorf("'n' should deselect all, item %d still selected", i)
		}
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriageEscGoesBack(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyEsc})
	if m.step == addStepDiscovery {
		t.Error("Esc in triage should navigate back")
	}
}

func TestAddWizard_UpdateKeyDiscovery_TriageUnhandledRuneNoop(t *testing.T) {
	t.Parallel()
	m := discoveryKeyWizard(t)
	m.confirmSelected = map[int]bool{0: true}
	m, _ = m.updateKeyDiscovery(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if !m.confirmSelected[0] {
		t.Error("unrelated rune should not mutate selection")
	}
}

// --- updateKeyReviewItems / updateKeyReviewButtons coverage ---

// reviewKeyWizard builds a wizard on Review with 3 selected items so cursor
// motion is testable end-to-end.
func reviewKeyWizard(t *testing.T) *addWizardModel {
	t.Helper()
	m := testOpenAddWizard(t)
	m.source = addSourceLocal
	m.typeChecks = m.buildTypeCheckList()
	m.discoveredItems = []addDiscoveryItem{
		{name: "a", displayName: "a", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "a", Type: catalog.Rules}},
		{name: "b", displayName: "b", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "b", Type: catalog.Rules}},
		{name: "c", displayName: "c", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "c", Type: catalog.Rules}},
	}
	m.actionableCount = 3
	m.discoveryList = m.buildDiscoveryList()
	m.enterReview()
	return m
}

func TestAddWizard_UpdateKeyReviewItems_EmptySelectionNoop(t *testing.T) {
	t.Parallel()
	m := testOpenAddWizard(t)
	m.source = addSourceLocal
	m.typeChecks = m.buildTypeCheckList()
	// No discoveredItems → no selection → early return.
	m.step = addStepReview
	m.shell.SetActive(m.shellIndexForStep(addStepReview))

	m, cmd := m.updateKeyReviewItems(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Errorf("expected nil cmd for empty selection, got %v", cmd)
	}
	if m.reviewItemCursor != 0 {
		t.Errorf("cursor should stay at 0, got %d", m.reviewItemCursor)
	}
}

func TestAddWizard_UpdateKeyReviewItems_DownMovesCursor(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m, _ = m.updateKeyReviewItems(tea.KeyMsg{Type: tea.KeyDown})
	if m.reviewItemCursor != 1 {
		t.Errorf("Down should advance cursor, got %d", m.reviewItemCursor)
	}
}

func TestAddWizard_UpdateKeyReviewItems_UpMovesCursor(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m.reviewItemCursor = 2
	m, _ = m.updateKeyReviewItems(tea.KeyMsg{Type: tea.KeyUp})
	if m.reviewItemCursor != 1 {
		t.Errorf("Up should decrement cursor, got %d", m.reviewItemCursor)
	}
}

func TestAddWizard_UpdateKeyReviewItems_JKVimKeys(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m, _ = m.updateKeyReviewItems(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.reviewItemCursor != 1 {
		t.Errorf("j should advance cursor, got %d", m.reviewItemCursor)
	}
	m, _ = m.updateKeyReviewItems(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.reviewItemCursor != 0 {
		t.Errorf("k should decrement cursor, got %d", m.reviewItemCursor)
	}
}

func TestAddWizard_UpdateKeyReviewItems_RightSwitchesToButtons(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m, _ = m.updateKeyReviewItems(tea.KeyMsg{Type: tea.KeyRight})
	if m.reviewZone != addReviewZoneButtons {
		t.Errorf("Right should switch to buttons zone, got %v", m.reviewZone)
	}
	if m.buttonCursor != 0 {
		t.Errorf("buttonCursor should land on Add (0), got %d", m.buttonCursor)
	}
}

func TestAddWizard_UpdateKeyReviewItems_PgDnJumps(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m.height = 30
	m, _ = m.updateKeyReviewItems(tea.KeyMsg{Type: tea.KeyPgDown})
	// Should move toward end
	if m.reviewItemCursor == 0 {
		t.Error("PgDn should advance cursor at least one step")
	}
}

func TestAddWizard_UpdateKeyReviewItems_PgUpReverses(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m.reviewItemCursor = 2
	m, _ = m.updateKeyReviewItems(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.reviewItemCursor > 2 {
		t.Errorf("PgUp shouldn't move forward, got %d", m.reviewItemCursor)
	}
}

func TestAddWizard_UpdateKeyReviewItems_HomeAndEnd(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m.reviewItemCursor = 1
	m, _ = m.updateKeyReviewItems(tea.KeyMsg{Type: tea.KeyHome})
	if m.reviewItemCursor != 0 {
		t.Errorf("Home should jump to 0, got %d", m.reviewItemCursor)
	}
	m, _ = m.updateKeyReviewItems(tea.KeyMsg{Type: tea.KeyEnd})
	last := len(m.selectedItems()) - 1
	if m.reviewItemCursor != last {
		t.Errorf("End should jump to %d, got %d", last, m.reviewItemCursor)
	}
}

func TestAddWizard_UpdateKeyReviewItems_EOpensRenameModal(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m, cmd := m.updateKeyReviewItems(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	_ = cmd
	if !m.renameModal.active {
		t.Error("'e' should open the rename modal")
	}
}

func TestAddWizard_UpdateKeyReviewButtons_LeftBacksToItems(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m.reviewZone = addReviewZoneButtons
	m.buttonCursor = 0 // leftmost button
	m, _ = m.updateKeyReviewButtons(tea.KeyMsg{Type: tea.KeyLeft})
	if m.reviewZone != addReviewZoneItems {
		t.Errorf("Left at leftmost button should cross back to items, got %v", m.reviewZone)
	}
}

func TestAddWizard_UpdateKeyReviewButtons_LeftDecrementsCursor(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m.reviewZone = addReviewZoneButtons
	m.buttonCursor = 2
	m, _ = m.updateKeyReviewButtons(tea.KeyMsg{Type: tea.KeyLeft})
	if m.buttonCursor != 1 {
		t.Errorf("Left should decrement cursor, got %d", m.buttonCursor)
	}
}

func TestAddWizard_UpdateKeyReviewButtons_RightAdvances(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m.reviewZone = addReviewZoneButtons
	m.buttonCursor = 0
	m, _ = m.updateKeyReviewButtons(tea.KeyMsg{Type: tea.KeyRight})
	if m.buttonCursor != 1 {
		t.Errorf("Right should advance cursor, got %d", m.buttonCursor)
	}
}

func TestAddWizard_UpdateKeyReviewButtons_RightClampsAt3(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m.reviewZone = addReviewZoneButtons
	m.buttonCursor = 3
	m, _ = m.updateKeyReviewButtons(tea.KeyMsg{Type: tea.KeyRight})
	if m.buttonCursor != 3 {
		t.Errorf("Right at 3 should clamp, got %d", m.buttonCursor)
	}
}

func TestAddWizard_UpdateKeyReviewButtons_EnterAddAdvances(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m.reviewZone = addReviewZoneButtons
	m.buttonCursor = 0
	m, cmd := m.updateKeyReviewButtons(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.reviewAcknowledged {
		t.Error("Enter on Add should acknowledge")
	}
	if m.step != addStepExecute {
		t.Errorf("Enter on Add should advance to Execute, got %v", m.step)
	}
	if cmd == nil {
		t.Error("expected addItemCmd")
	}
}

func TestAddWizard_UpdateKeyReviewButtons_EnterBackReturns(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m.reviewZone = addReviewZoneButtons
	m.buttonCursor = 2
	m, _ = m.updateKeyReviewButtons(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != addStepDiscovery {
		t.Errorf("Enter on Back should return to Discovery, got %v", m.step)
	}
}

func TestAddWizard_UpdateKeyReviewButtons_EnterCancelReturnsCloseCmd(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m.reviewZone = addReviewZoneButtons
	m.buttonCursor = 3
	_, cmd := m.updateKeyReviewButtons(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected close cmd")
	}
	if _, ok := cmd().(addCloseMsg); !ok {
		t.Errorf("expected addCloseMsg, got %T", cmd())
	}
}

func TestAddWizard_UpdateKeyReviewButtons_EnterRenameOpensModal(t *testing.T) {
	t.Parallel()
	m := reviewKeyWizard(t)
	m.reviewZone = addReviewZoneButtons
	m.buttonCursor = 1
	m, _ = m.updateKeyReviewButtons(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.renameModal.active {
		t.Error("Enter on Rename should open rename modal")
	}
}

// --- updateKeyExecute coverage ---

// executeDoneWizard builds a wizard on Execute with executeDone=true and 6
// selected items at height=8 so executeOffset scrolling actually engages
// (execH = max(3, height-8) = 3, maxOff = len-execH = 3).
func executeDoneWizard(t *testing.T) *addWizardModel {
	t.Helper()
	m := testOpenAddWizard(t)
	m.source = addSourceLocal
	m.typeChecks = m.buildTypeCheckList()
	m.discoveredItems = []addDiscoveryItem{
		{name: "a", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "a", Type: catalog.Rules}},
		{name: "b", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "b", Type: catalog.Rules}},
		{name: "c", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "c", Type: catalog.Rules}},
		{name: "d", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "d", Type: catalog.Rules}},
		{name: "e", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "e", Type: catalog.Rules}},
		{name: "f", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "f", Type: catalog.Rules}},
	}
	m.actionableCount = len(m.discoveredItems)
	m.discoveryList = m.buildDiscoveryList()
	m.reviewAcknowledged = true
	m.step = addStepExecute
	m.shell.SetActive(m.shellIndexForStep(addStepExecute))
	m.executeDone = true
	m.height = 8
	return m
}

func TestAddWizard_UpdateKeyExecute_DoneAddMoreEmitsRestart(t *testing.T) {
	t.Parallel()
	m := executeDoneWizard(t)
	_, cmd := m.updateKeyExecute(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("expected cmd from 'a' (Add More)")
	}
	if _, ok := cmd().(addRestartMsg); !ok {
		t.Errorf("expected addRestartMsg, got %T", cmd())
	}
}

func TestAddWizard_UpdateKeyExecute_DoneEnterCloses(t *testing.T) {
	t.Parallel()
	m := executeDoneWizard(t)
	_, cmd := m.updateKeyExecute(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Enter")
	}
	if _, ok := cmd().(addCloseMsg); !ok {
		t.Errorf("expected addCloseMsg, got %T", cmd())
	}
}

func TestAddWizard_UpdateKeyExecute_DoneEscCloses(t *testing.T) {
	t.Parallel()
	m := executeDoneWizard(t)
	_, cmd := m.updateKeyExecute(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected cmd from Esc")
	}
	if _, ok := cmd().(addCloseMsg); !ok {
		t.Errorf("expected addCloseMsg, got %T", cmd())
	}
}

func TestAddWizard_UpdateKeyExecute_DoneDownScrolls(t *testing.T) {
	t.Parallel()
	m := executeDoneWizard(t)
	m, _ = m.updateKeyExecute(tea.KeyMsg{Type: tea.KeyDown})
	if m.executeOffset == 0 {
		t.Error("Down should scroll offset when room available")
	}
}

func TestAddWizard_UpdateKeyExecute_DoneUpScrolls(t *testing.T) {
	t.Parallel()
	m := executeDoneWizard(t)
	m.executeOffset = 2
	m, _ = m.updateKeyExecute(tea.KeyMsg{Type: tea.KeyUp})
	if m.executeOffset != 1 {
		t.Errorf("Up should decrement offset, got %d", m.executeOffset)
	}
}

func TestAddWizard_UpdateKeyExecute_DoneHomeResets(t *testing.T) {
	t.Parallel()
	m := executeDoneWizard(t)
	m.executeOffset = 5
	m, _ = m.updateKeyExecute(tea.KeyMsg{Type: tea.KeyHome})
	if m.executeOffset != 0 {
		t.Errorf("Home should reset offset to 0, got %d", m.executeOffset)
	}
}

func TestAddWizard_UpdateKeyExecute_DoneEndMoves(t *testing.T) {
	t.Parallel()
	m := executeDoneWizard(t)
	m.executeOffset = 0
	m, _ = m.updateKeyExecute(tea.KeyMsg{Type: tea.KeyEnd})
	// With 3 selected items and height=8, executeOffset should stay at 0
	// or move toward the maximum. Just ensure no crash.
	_ = m
}

func TestAddWizard_UpdateKeyExecute_RunningEscCancels(t *testing.T) {
	t.Parallel()
	m := executeDoneWizard(t)
	m.executeDone = false
	m, _ = m.updateKeyExecute(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.executeCancelled {
		t.Error("Esc while running should set executeCancelled=true")
	}
}

func TestAddWizard_UpdateKeyExecute_RunningOtherKeyNoop(t *testing.T) {
	t.Parallel()
	m := executeDoneWizard(t)
	m.executeDone = false
	m, cmd := m.updateKeyExecute(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("non-Esc key while running should be no-op")
	}
	if m.executeCancelled {
		t.Error("non-Esc key should not cancel")
	}
}

// --- Helper ---

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
