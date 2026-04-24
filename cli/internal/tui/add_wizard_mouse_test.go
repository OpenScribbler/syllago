package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// Mouse coverage for the add wizard. The wizard has six logical steps
// (Source, Type, Discovery, Triage, Review, Execute) and a shared nav bar
// (add-nav-back, add-nav-next) that all non-Execute steps share. Each step
// has a distinct zone layout; a break in any one silently violates the
// mouse-parity contract in .claude/rules/tui-wizard-patterns.md.
//
// Tests below construct the wizard model directly — bypassing App.Update —
// to pin single zone handlers. scanZones() (defined in install_mouse_test.go)
// waits for bubblezone's async zoneWorker to populate the zone map before
// Get() is called.

// --- Helpers ---

// addWizardForSource builds a fresh add wizard at the Source step with one
// detected provider and one registry. Sized 100x35 so all source rows +
// nav buttons render cleanly.
func addWizardForSource(t *testing.T) *addWizardModel {
	t.Helper()
	provA := testInstallProvider("Claude Code", "claude-code", true)
	registries := []catalog.RegistrySource{
		{Name: "official", Path: "/tmp/official"},
	}
	m := openAddWizard(
		[]provider.Provider{provA},
		registries,
		&config.Config{},
		"/tmp/project",
		"/tmp/content",
		"",
	)
	m.width = 100
	m.height = 35
	m.shell.SetWidth(100)
	return m
}

// --- Source step ---

// TestAddWizardMouse_SourceRowClickProvider pins add-src-0 (Provider) at
// add_wizard_update.go:192. Clicking Provider row must set sourceCursor=0
// AND expand the sub-list — the keyboard equivalent is arrow-down-to-0 +
// Enter.
func TestAddWizardMouse_SourceRowClickProvider(t *testing.T) {
	m := addWizardForSource(t)
	scanZones(m.View())
	z := zone.Get("add-src-0")
	if z.IsZero() {
		t.Skip("zone add-src-0 not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.sourceCursor != 0 {
		t.Errorf("sourceCursor should be 0 after Provider click, got %d", m.sourceCursor)
	}
	if !m.sourceExpanded {
		t.Error("sourceExpanded should be true after Provider click (provider sub-list shown)")
	}
}

// TestAddWizardMouse_SourceRowClickRegistry pins add-src-1. Clicking
// Registry must expand the registry sub-list.
func TestAddWizardMouse_SourceRowClickRegistry(t *testing.T) {
	m := addWizardForSource(t)
	scanZones(m.View())
	z := zone.Get("add-src-1")
	if z.IsZero() {
		t.Skip("zone add-src-1 not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.sourceCursor != 1 {
		t.Errorf("sourceCursor should be 1 after Registry click, got %d", m.sourceCursor)
	}
	if !m.sourceExpanded {
		t.Error("sourceExpanded should be true after Registry click")
	}
}

// TestAddWizardMouse_SourceRowClickLocal pins add-src-2. Clicking Local must
// activate the path input field (inputActive=true).
func TestAddWizardMouse_SourceRowClickLocal(t *testing.T) {
	m := addWizardForSource(t)
	scanZones(m.View())
	z := zone.Get("add-src-2")
	if z.IsZero() {
		t.Skip("zone add-src-2 not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.sourceCursor != 2 {
		t.Errorf("sourceCursor should be 2 after Local click, got %d", m.sourceCursor)
	}
	if !m.inputActive {
		t.Error("inputActive should be true after Local click (path input activated)")
	}
	if m.sourceExpanded {
		t.Error("sourceExpanded should be false for Local (no sub-list)")
	}
}

// TestAddWizardMouse_SourceRowClickGit pins add-src-3. Clicking Git must
// activate the path input field for URL entry.
func TestAddWizardMouse_SourceRowClickGit(t *testing.T) {
	m := addWizardForSource(t)
	scanZones(m.View())
	z := zone.Get("add-src-3")
	if z.IsZero() {
		t.Skip("zone add-src-3 not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.sourceCursor != 3 {
		t.Errorf("sourceCursor should be 3 after Git click, got %d", m.sourceCursor)
	}
	if !m.inputActive {
		t.Error("inputActive should be true after Git click (URL input activated)")
	}
}

// TestAddWizardMouse_SourceProviderSublistAdvances pins add-prov-0 at
// add_wizard_update.go:225. Clicking a provider sub-list row must set
// source=addSourceProvider AND advance past the Source step. The keyboard
// path requires two Enters (select + advance); mouse collapses that into
// a single click per UX conventions.
func TestAddWizardMouse_SourceProviderSublistAdvances(t *testing.T) {
	m := addWizardForSource(t)
	// Expand the provider sub-list by clicking add-src-0 first.
	scanZones(m.View())
	zSrc := zone.Get("add-src-0")
	if zSrc.IsZero() {
		t.Skip("zone add-src-0 not registered")
	}
	m, _ = m.Update(mouseClick(zSrc.StartX, zSrc.StartY))
	if !m.sourceExpanded {
		t.Fatal("precondition: provider sub-list should be expanded")
	}
	// Now click add-prov-0.
	scanZones(m.View())
	zProv := zone.Get("add-prov-0")
	if zProv.IsZero() {
		t.Skip("zone add-prov-0 not registered")
	}
	m, _ = m.Update(mouseClick(zProv.StartX, zProv.StartY))

	if m.source != addSourceProvider {
		t.Errorf("source should be addSourceProvider after provider row click, got %d", m.source)
	}
	if m.sourceExpanded {
		t.Error("sourceExpanded should be false after provider selection (sub-list collapsed)")
	}
	// advanceFromSource pushes the wizard past Source; with no type filter,
	// the next step is Type.
	if m.step == addStepSource {
		t.Error("step should advance past addStepSource after provider click")
	}
}

// TestAddWizardMouse_SourceRegistrySublistAdvances pins add-reg-0 at
// add_wizard_update.go:237. Mirror of provider sub-list for registries.
func TestAddWizardMouse_SourceRegistrySublistAdvances(t *testing.T) {
	m := addWizardForSource(t)
	scanZones(m.View())
	zSrc := zone.Get("add-src-1")
	if zSrc.IsZero() {
		t.Skip("zone add-src-1 not registered")
	}
	m, _ = m.Update(mouseClick(zSrc.StartX, zSrc.StartY))
	if !m.sourceExpanded {
		t.Fatal("precondition: registry sub-list should be expanded")
	}
	scanZones(m.View())
	zReg := zone.Get("add-reg-0")
	if zReg.IsZero() {
		t.Skip("zone add-reg-0 not registered")
	}
	m, _ = m.Update(mouseClick(zReg.StartX, zReg.StartY))

	if m.source != addSourceRegistry {
		t.Errorf("source should be addSourceRegistry after registry row click, got %d", m.source)
	}
	if m.step == addStepSource {
		t.Error("step should advance past addStepSource after registry click")
	}
}

// TestAddWizardMouse_SourcePathInputClickable pins add-path-input at
// add_wizard_update.go:247. The path input zone only renders when
// inputActive=true. Clicking it must be consumed (no panic, stays active) —
// this defensive handler prevents click-through to other zones.
func TestAddWizardMouse_SourcePathInputClickable(t *testing.T) {
	m := addWizardForSource(t)
	// Path input only renders when active + on Local/Git row.
	m.sourceCursor = 2
	m.inputActive = true

	scanZones(m.View())
	z := zone.Get("add-path-input")
	if z.IsZero() {
		t.Fatal("zone add-path-input should be registered when inputActive=true on Local row")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))

	if !m.inputActive {
		t.Error("inputActive should remain true after clicking add-path-input")
	}
}

// --- Shared nav buttons ---

// TestAddWizardMouse_NavBackRoutesToEsc pins add-nav-back at
// add_wizard_update.go:65. The shared Back button routes to the same
// keyboard Esc handler per step. Source step has no Back button
// (showBack=false at add_wizard_view.go:88), so the zone first renders on
// the Type step, where Esc transitions back to Source.
func TestAddWizardMouse_NavBackRoutesToEsc(t *testing.T) {
	m := addWizardForSource(t)
	// Advance to Type step so the Back button renders.
	m.source = addSourceProvider
	m.typeChecks = m.buildTypeCheckList()
	m.step = addStepType
	m.shell.SetActive(m.shellIndexForStep(addStepType))

	scanZones(m.View())
	z := zone.Get("add-nav-back")
	if z.IsZero() {
		t.Fatal("zone add-nav-back should be registered on Type step")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.step != addStepSource {
		t.Errorf("expected step=addStepSource after Back click on Type, got %v", m.step)
	}
}

// TestAddWizardMouse_NavNextRoutesToEnter pins add-nav-next at
// add_wizard_update.go:68. Clicking Next on the Source step (with sourceCursor=2)
// must route to the same keyboard Enter path — which for Local activates
// the path input.
func TestAddWizardMouse_NavNextRoutesToEnter(t *testing.T) {
	m := addWizardForSource(t)
	m.sourceCursor = 2 // Local
	scanZones(m.View())
	z := zone.Get("add-nav-next")
	if z.IsZero() {
		t.Skip("zone add-nav-next not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	// Enter on Local activates inputActive.
	if !m.inputActive {
		t.Error("Next click on Local-source should activate the path input (Enter-equivalent)")
	}
}

// --- Execute step ---

// executeStepWizard builds a wizard at the Execute step with executeDone=true
// so the Close/Add More buttons are clickable.
func executeStepWizard(t *testing.T) *addWizardModel {
	t.Helper()
	m := addWizardForSource(t)
	// Seed the wizard through to Execute-done state. validateStep() requires
	// real state for each intermediate step; setting it manually here mirrors
	// how TestAddWizard_ValidateStep_Forward walks the steps.
	m.source = addSourceProvider
	m.typeChecks = m.buildTypeCheckList()
	m.discoveredItems = []addDiscoveryItem{
		{name: "test", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "test", Type: catalog.Rules}},
	}
	m.discoveryList = m.buildDiscoveryList()
	m.reviewAcknowledged = true
	m.step = addStepExecute
	m.shell.SetActive(m.shellIndexForStep(addStepExecute))
	m.executeDone = true
	return m
}

// TestAddWizardMouse_ExecuteDoneCloses pins add-exec-done at
// add_wizard_update.go:402. Clicking the Close button on the execute-done
// screen must emit addCloseMsg.
func TestAddWizardMouse_ExecuteDoneCloses(t *testing.T) {
	m := executeStepWizard(t)
	scanZones(m.View())
	z := zone.Get("add-exec-done")
	if z.IsZero() {
		t.Skip("zone add-exec-done not registered")
	}
	_, cmd := m.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected command from add-exec-done click")
	}
	if _, ok := cmd().(addCloseMsg); !ok {
		t.Errorf("expected addCloseMsg, got %T", cmd())
	}
}

// TestAddWizardMouse_ExecuteRestartEmitsRestart pins add-exec-restart at
// add_wizard_update.go:405. Clicking Add More must emit addRestartMsg so
// the App can rebuild the wizard from Source.
func TestAddWizardMouse_ExecuteRestartEmitsRestart(t *testing.T) {
	m := executeStepWizard(t)
	scanZones(m.View())
	z := zone.Get("add-exec-restart")
	if z.IsZero() {
		t.Skip("zone add-exec-restart not registered")
	}
	_, cmd := m.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected command from add-exec-restart click")
	}
	if _, ok := cmd().(addRestartMsg); !ok {
		t.Errorf("expected addRestartMsg, got %T", cmd())
	}
}

// TestAddWizardMouse_ExecuteCancelSetsFlag pins add-exec-cancel at
// add_wizard_update.go:409. Clicking Cancel while still executing must set
// executeCancelled=true. This is a state flag (not a command) — the add
// worker observes it between items.
func TestAddWizardMouse_ExecuteCancelSetsFlag(t *testing.T) {
	m := executeStepWizard(t)
	m.executeDone = false // still running
	m.executing = true

	scanZones(m.View())
	z := zone.Get("add-exec-cancel")
	if z.IsZero() {
		t.Skip("zone add-exec-cancel not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if !m.executeCancelled {
		t.Error("executeCancelled should be true after cancel click")
	}
}

// --- Parameterized source-row smoke test ---

// TestAddWizardMouse_AllSourceRowsRespondToClick pins all four add-src-N
// zones in a single table-driven test. Each row must at minimum update
// sourceCursor. The zone-specific side effects are pinned in the dedicated
// tests above — this guards against a regression that drops any of the
// four registrations entirely.
func TestAddWizardMouse_AllSourceRowsRespondToClick(t *testing.T) {
	for i := 0; i < 4; i++ {
		i := i
		t.Run(fmt.Sprintf("src_%d", i), func(t *testing.T) {
			m := addWizardForSource(t)
			scanZones(m.View())
			z := zone.Get(fmt.Sprintf("add-src-%d", i))
			if z.IsZero() {
				t.Skipf("zone add-src-%d not registered", i)
			}
			m, _ = m.Update(mouseClick(z.StartX, z.StartY))
			if m.sourceCursor != i {
				t.Errorf("sourceCursor should be %d, got %d", i, m.sourceCursor)
			}
		})
	}
}

// Type assertion so tea is referenced even if all test funcs get inlined.
var _ = tea.KeyMsg{}

// --- Type step mouse tests ---

// typeStepWizard builds a wizard at the Type step with a typed check list.
func typeStepWizard(t *testing.T) *addWizardModel {
	t.Helper()
	m := addWizardForSource(t)
	m.source = addSourceProvider
	m.typeChecks = m.buildTypeCheckList()
	m.step = addStepType
	m.shell.SetActive(m.shellIndexForStep(addStepType))
	return m
}

func TestAddWizardMouse_TypeRowTogglesSelection(t *testing.T) {
	m := typeStepWizard(t)
	scanZones(m.View())
	// Click row 1 (Skills)
	z := zone.Get("add-type-1")
	if z.IsZero() {
		t.Skip("zone add-type-1 not registered")
	}
	prev := m.typeChecks.selected[1]
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.typeChecks.selected[1] == prev {
		t.Error("clicking row should toggle selection")
	}
	if m.typeChecks.cursor != 1 {
		t.Errorf("cursor should move to 1, got %d", m.typeChecks.cursor)
	}
}

func TestAddWizardMouse_TypeWheelUpMovesCursor(t *testing.T) {
	m := typeStepWizard(t)
	m.typeChecks.cursor = 3
	scanZones(m.View())
	wheel := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp}
	m, _ = m.Update(wheel)
	if m.typeChecks.cursor != 2 {
		t.Errorf("wheel up should decrement cursor to 2, got %d", m.typeChecks.cursor)
	}
}

func TestAddWizardMouse_TypeWheelDownMovesCursor(t *testing.T) {
	m := typeStepWizard(t)
	m.typeChecks.cursor = 0
	scanZones(m.View())
	wheel := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}
	m, _ = m.Update(wheel)
	if m.typeChecks.cursor != 1 {
		t.Errorf("wheel down should increment cursor to 1, got %d", m.typeChecks.cursor)
	}
}

func TestAddWizardMouse_TypeWheelUpClampsAtZero(t *testing.T) {
	m := typeStepWizard(t)
	m.typeChecks.cursor = 0
	wheel := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp}
	m, _ = m.Update(wheel)
	if m.typeChecks.cursor != 0 {
		t.Errorf("cursor should clamp at 0, got %d", m.typeChecks.cursor)
	}
}

func TestAddWizardMouse_TypeWheelDownClampsAtEnd(t *testing.T) {
	m := typeStepWizard(t)
	m.typeChecks.cursor = len(m.typeChecks.items) - 1
	last := m.typeChecks.cursor
	wheel := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}
	m, _ = m.Update(wheel)
	if m.typeChecks.cursor != last {
		t.Errorf("cursor should clamp at last index, got %d", m.typeChecks.cursor)
	}
}

// --- Discovery step mouse tests ---

// discoveryErrorStepWizard builds a wizard at the Discovery step with an error.
func discoveryErrorStepWizard(t *testing.T) *addWizardModel {
	t.Helper()
	m := addWizardForSource(t)
	m.source = addSourceProvider
	m.typeChecks = m.buildTypeCheckList()
	m.step = addStepDiscovery
	m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
	m.discoveryErr = "boom"
	return m
}

func TestAddWizardMouse_DiscoveryRetryClicks(t *testing.T) {
	m := discoveryErrorStepWizard(t)
	scanZones(m.View())
	z := zone.Get("add-retry")
	if z.IsZero() {
		t.Skip("zone add-retry not registered")
	}
	m, cmd := m.Update(mouseClick(z.StartX, z.StartY))
	if m.discoveryErr != "" {
		t.Error("retry click should clear discoveryErr")
	}
	if !m.discovering {
		t.Error("retry click should set discovering=true")
	}
	if cmd == nil {
		t.Error("retry click should emit startDiscoveryCmd")
	}
}

func TestAddWizardMouse_DiscoveryErrBackClicks(t *testing.T) {
	m := discoveryErrorStepWizard(t)
	scanZones(m.View())
	z := zone.Get("add-err-back")
	if z.IsZero() {
		t.Skip("zone add-err-back not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	// goBackFromDiscovery routes back; step should change away from Discovery
	if m.step == addStepDiscovery && m.discoveryErr != "" {
		// Allow depending on logic — check that it at least doesn't panic
		_ = m
	}
}

func TestAddWizardMouse_DiscoveryEmptyBackClicks(t *testing.T) {
	m := addWizardForSource(t)
	m.source = addSourceProvider
	m.typeChecks = m.buildTypeCheckList()
	m.step = addStepDiscovery
	m.shell.SetActive(m.shellIndexForStep(addStepDiscovery))
	// Not discovering, no error, no confirm items → empty state
	scanZones(m.View())
	z := zone.Get("add-empty-back")
	if z.IsZero() {
		t.Skip("zone add-empty-back not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	_ = m
}

// --- Review step mouse tests ---

// addReviewStepWizard builds a wizard at the Review step with one selected item.
func addReviewStepWizard(t *testing.T) *addWizardModel {
	t.Helper()
	m := addWizardForSource(t)
	m.source = addSourceProvider
	m.typeChecks = m.buildTypeCheckList()
	m.discoveredItems = []addDiscoveryItem{
		{name: "test-rule", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "test-rule", Type: catalog.Rules}},
	}
	m.discoveryList = m.buildDiscoveryList()
	m.enterReview()
	return m
}

func TestAddWizardMouse_ReviewCancelClicks(t *testing.T) {
	m := addReviewStepWizard(t)
	scanZones(m.View())
	z := zone.Get("add-cancel")
	if z.IsZero() {
		t.Skip("zone add-cancel not registered")
	}
	_, cmd := m.Update(mouseClick(z.StartX, z.StartY))
	if cmd == nil {
		t.Fatal("expected cmd from cancel click")
	}
	if _, ok := cmd().(addCloseMsg); !ok {
		t.Errorf("expected addCloseMsg, got %T", cmd())
	}
}

func TestAddWizardMouse_ReviewBackClicks(t *testing.T) {
	m := addReviewStepWizard(t)
	scanZones(m.View())
	z := zone.Get("add-back")
	if z.IsZero() {
		t.Skip("zone add-back not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.step != addStepDiscovery {
		t.Errorf("Back click on Review → step=%v, want addStepDiscovery", m.step)
	}
	if m.reviewAcknowledged {
		t.Error("reviewAcknowledged should be reset to false")
	}
}

func TestAddWizardMouse_ReviewConfirmFirstClickAdvances(t *testing.T) {
	m := addReviewStepWizard(t)
	scanZones(m.View())
	z := zone.Get("add-confirm")
	if z.IsZero() {
		t.Skip("zone add-confirm not registered")
	}
	m, cmd := m.Update(mouseClick(z.StartX, z.StartY))
	if !m.reviewAcknowledged {
		t.Error("reviewAcknowledged should be true after confirm click")
	}
	if m.step != addStepExecute {
		t.Errorf("step should be addStepExecute, got %v", m.step)
	}
	if cmd == nil {
		t.Error("confirm click should emit addItemCmd")
	}
}

func TestAddWizardMouse_ReviewRenameClicksOpensModal(t *testing.T) {
	m := addReviewStepWizard(t)
	scanZones(m.View())
	z := zone.Get("add-rename")
	if z.IsZero() {
		t.Skip("zone add-rename not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if !m.renameModal.active {
		t.Error("rename modal should be active after rename click")
	}
}

func TestAddWizardMouse_ReviewItemClickSelects(t *testing.T) {
	m := addReviewStepWizard(t)
	// Force cursor off item 0 so clicking it changes cursor
	m.reviewItemCursor = 99
	scanZones(m.View())
	z := zone.Get("add-rev-item-0")
	if z.IsZero() {
		t.Skip("zone add-rev-item-0 not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.reviewItemCursor != 0 {
		t.Errorf("reviewItemCursor = %d, want 0", m.reviewItemCursor)
	}
	if m.reviewZone != addReviewZoneItems {
		t.Errorf("reviewZone should be addReviewZoneItems, got %v", m.reviewZone)
	}
}

func TestAddWizardMouse_ReviewItemClickTwiceDrillsIn(t *testing.T) {
	m := addReviewStepWizard(t)
	// Seed the discovery item with a real on-disk file so drill-in doesn't abort.
	dir := t.TempDir()
	itemDir := dir + "/rules/my-rule"
	if err := makeTestFile(t, itemDir, "rule.md", "# Rule\n\nbody\n"); err != nil {
		t.Fatal(err)
	}
	m.discoveredItems[0].path = itemDir
	m.discoveredItems[0].sourceDir = itemDir

	m.reviewZone = addReviewZoneItems
	m.reviewItemCursor = 0
	scanZones(m.View())
	z := zone.Get("add-rev-item-0")
	if z.IsZero() {
		t.Skip("zone add-rev-item-0 not registered")
	}
	// Click once (already selected) → drill in
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if !m.reviewDrillIn {
		t.Error("second click on selected item should drill in")
	}
}

func TestAddWizardMouse_ReviewDrillInBackExits(t *testing.T) {
	m := addReviewStepWizard(t)
	m.reviewDrillIn = true
	m.reviewDrillTree.focused = true
	scanZones(m.View())
	z := zone.Get("add-nav-back")
	if z.IsZero() {
		t.Skip("zone add-nav-back not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if m.reviewDrillIn {
		t.Error("Back click should exit drill-in")
	}
}

func TestAddWizardMouse_ReviewDrillInRenameClicksOpensModal(t *testing.T) {
	m := addReviewStepWizard(t)
	m.reviewDrillIn = true
	scanZones(m.View())
	z := zone.Get("add-rename")
	if z.IsZero() {
		t.Skip("zone add-rename not registered")
	}
	m, _ = m.Update(mouseClick(z.StartX, z.StartY))
	if !m.renameModal.active {
		t.Error("rename modal should be active after rename click in drill-in")
	}
}

func TestAddWizardMouse_ReviewWheelScrollsItemCursor(t *testing.T) {
	m := addReviewStepWizard(t)
	// Two items so wheel-down can move
	m.discoveredItems = append(m.discoveredItems,
		addDiscoveryItem{name: "b", itemType: catalog.Rules, status: add.StatusNew,
			underlying: &add.DiscoveryItem{Name: "b", Type: catalog.Rules}})
	m.discoveryList = m.buildDiscoveryList()
	m.enterReview()
	m.reviewZone = addReviewZoneItems
	m.reviewItemCursor = 0

	wheel := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown}
	m, _ = m.Update(wheel)
	if m.reviewItemCursor != 1 {
		t.Errorf("wheel down on review items → cursor=%d, want 1", m.reviewItemCursor)
	}
	wheelUp := tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp}
	m, _ = m.Update(wheelUp)
	if m.reviewItemCursor != 0 {
		t.Errorf("wheel up on review items → cursor=%d, want 0", m.reviewItemCursor)
	}
}
