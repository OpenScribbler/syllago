package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// Cross-model state transition invariants. The App composes multiple
// sub-models (library, explorer, gallery) behind the topbar and dispatches
// catalog mutations only to the active one. These tests pin three
// transition boundaries where past regressions have silently stranded a
// sub-model in a stale or inconsistent state:
//
//  1. Install wizard open → close → done must leave the library tab active
//     AND keep library.table.items in sync with a.catalog.Items. A drift
//     here means the user sees stale or empty rows after installing.
//  2. Add wizard open on a non-Library tab → Esc must close the wizard
//     AND leave the caller's active group/tab unchanged. Past wizard
//     rewrites have accidentally reset topbar state to Library on cancel.
//  3. refreshContent() — the central dispatch that R-key rescans and
//     install-done rescans both funnel through — must target ONLY the
//     active tab's sub-model. If it ever fanned out to every sub-model,
//     cursor/selection state on inactive tabs would be silently reset.

// TestCrossModel_InstallFlowPreservesLibraryTabAndSyncsItems walks the full
// install flow (wizard open → wizard close → async install done) and asserts
// the library tab remains active and library.table.items reflects the
// post-rescan catalog. handleInstallDone (actions.go:670) fires
// rescanCatalog which synchronously replaces a.catalog and calls
// refreshContent — if that dispatch is broken, library.table.items will
// diverge from a.catalog.Items.
func TestCrossModel_InstallFlowPreservesLibraryTabAndSyncsItems(t *testing.T) {
	app := testAppWithLibraryItem(t)

	if got := app.topBar.ActiveGroupLabel(); got != "Collections" {
		t.Fatalf("precondition: expected active group Collections, got %q", got)
	}
	if got := app.topBar.ActiveTabLabel(); got != "Library" {
		t.Fatalf("precondition: expected active tab Library, got %q", got)
	}

	m, _ := app.Update(keyRune('i'))
	app = m.(App)
	if app.wizardMode != wizardInstall {
		t.Fatalf("precondition: expected wizardInstall after 'i', got %d", app.wizardMode)
	}

	m, _ = app.Update(installCloseMsg{})
	app = m.(App)
	if app.wizardMode != wizardNone {
		t.Fatalf("precondition: installCloseMsg should close wizard, got wizardMode=%d", app.wizardMode)
	}

	m, _ = app.Update(installDoneMsg{
		itemName:     "my-rule",
		providerName: "Claude Code",
		targetPath:   "/tmp/installed",
	})
	app = m.(App)

	if app.wizardMode != wizardNone {
		t.Errorf("wizardMode should remain wizardNone after installDoneMsg, got %d", app.wizardMode)
	}
	if app.installWizard != nil {
		t.Errorf("installWizard should remain nil after installDoneMsg")
	}
	if got := app.topBar.ActiveGroupLabel(); got != "Collections" {
		t.Errorf("active group should remain Collections after install flow, got %q", got)
	}
	if got := app.topBar.ActiveTabLabel(); got != "Library" {
		t.Errorf("active sub-tab should remain Library after install flow, got %q", got)
	}
	// The central invariant: rescanCatalog → refreshContent keeps library
	// in lockstep with catalog. An earlier dispatch bug broke this by
	// refreshing the explorer even on library tabs.
	if got, want := len(app.library.table.items), len(app.catalog.Items); got != want {
		t.Errorf("library.table.items (%d) out of sync with catalog.Items (%d) after install flow",
			got, want)
	}
}

// TestCrossModel_AddWizardEscPreservesContentTab opens the add wizard on the
// Content > Skills tab, cancels with Esc, and verifies the active tab is
// unchanged. The add wizard closes via addCloseMsg which runs rescanCatalog
// (app_update.go:391). A regression where the close handler resets topbar
// state — or where rescanCatalog side-effects flip the active tab — would
// leave the user on Library after cancelling, losing their place.
func TestCrossModel_AddWizardEscPreservesContentTab(t *testing.T) {
	app := testAppWithItems(t)

	// Switch to Content group; it defaults to the first sub-tab (Skills).
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	app = m.(App)
	if got := app.topBar.ActiveGroupLabel(); got != "Content" {
		t.Fatalf("precondition: expected active group Content, got %q", got)
	}
	if got := app.topBar.ActiveTabLabel(); got != "Skills" {
		t.Fatalf("precondition: expected active tab Skills, got %q", got)
	}

	wantGroup := app.topBar.ActiveGroupLabel()
	wantTab := app.topBar.ActiveTabLabel()

	// Open add wizard.
	m, _ = app.Update(keyRune('a'))
	app = m.(App)
	if app.wizardMode != wizardAdd {
		t.Fatalf("precondition: expected wizardAdd after 'a', got %d", app.wizardMode)
	}

	// Esc closes the wizard. The wizard emits addCloseMsg as a tea.Cmd,
	// which the App then processes to clear addWizard + rescan catalog.
	m, cmd = app.Update(keyPress(tea.KeyEsc))
	app = m.(App)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			m, _ = app.Update(msg)
			app = m.(App)
		}
	}

	if app.wizardMode != wizardNone {
		t.Errorf("wizardMode should be wizardNone after Esc, got %d", app.wizardMode)
	}
	if app.addWizard != nil {
		t.Errorf("addWizard should be nil after Esc close")
	}
	if got := app.topBar.ActiveGroupLabel(); got != wantGroup {
		t.Errorf("active group changed across add-wizard cancel: want %q, got %q", wantGroup, got)
	}
	if got := app.topBar.ActiveTabLabel(); got != wantTab {
		t.Errorf("active sub-tab changed across add-wizard cancel: want %q, got %q", wantTab, got)
	}
}

// TestCrossModel_RefreshContentTargetsActiveSubModel verifies that
// refreshContent() — the dispatch point invoked by rescanCatalog (for R key
// and every install/uninstall path) — only pushes fresh items to the
// sub-model backing the active tab. Non-active sub-models must be left
// alone so their cursor/selection state survives background refreshes.
//
// This isolates the dispatch logic by calling refreshContent directly
// after mutating a.catalog.Items. Using the R key path would additionally
// trigger a disk scan that would clobber the mutated catalog before
// dispatch could fan out, defeating the test. The dispatch rule under
// test is exactly the post-scan step of rescanCatalog.
func TestCrossModel_RefreshContentTargetsActiveSubModel(t *testing.T) {
	app := testAppWithItems(t)

	// Baseline: Library is active; library.table.items mirrors catalog.
	if got, want := len(app.library.table.items), len(app.catalog.Items); got != want {
		t.Fatalf("precondition: library.table.items=%d, catalog.Items=%d", got, want)
	}
	libraryBaseline := len(app.library.table.items)

	// Switch to Content > Skills. refreshContent fires via tabChangedMsg
	// handling and populates the explorer with catalog-filtered skills.
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	app = m.(App)
	if got := app.topBar.ActiveTabLabel(); got != "Skills" {
		t.Fatalf("precondition: expected Skills tab active, got %q", got)
	}
	// testCatalogWithItems seeds alpha-skill + beta-skill (type=Skills).
	if got := len(app.explorer.items.items); got != 2 {
		t.Fatalf("precondition: expected 2 skills in explorer after tab switch, got %d", got)
	}

	// Mutate catalog out-of-band: append a new skill. Before refresh,
	// the explorer should NOT see it yet.
	app.catalog.Items = append(app.catalog.Items, catalog.ContentItem{
		Name:   "gamma-skill",
		Type:   catalog.Skills,
		Source: "library",
		Files:  []string{"SKILL.md"},
	})
	if got := len(app.explorer.items.items); got != 2 {
		t.Fatalf("refreshContent fired prematurely: explorer already has %d items before refresh", got)
	}

	// Trigger the dispatch. This is the exact call made by rescanCatalog
	// at app.go:251 after disk scan replaces a.catalog.
	(&app).refreshContent()

	// Active sub-model (explorer) must now reflect the new catalog,
	// filtered to the active tab's content type.
	if got := len(app.explorer.items.items); got != 3 {
		t.Errorf("explorer.items.items should have 3 skills after refreshContent, got %d", got)
	}
	foundGamma := false
	for _, it := range app.explorer.items.items {
		if it.Name == "gamma-skill" {
			foundGamma = true
			break
		}
	}
	if !foundGamma {
		t.Errorf("explorer missing gamma-skill after refreshContent — dispatch did not deliver the mutated catalog")
	}

	// Non-active sub-model (library) must NOT have been touched.
	// Library's cached items reflect whatever was loaded when Library was
	// last active. An over-eager dispatch that updates all sub-models on
	// every refresh would bump this count to match the new catalog.
	if got := len(app.library.table.items); got != libraryBaseline {
		t.Errorf("library.table.items changed to %d despite library tab being inactive (baseline %d) — refreshContent leaked to non-active sub-model",
			got, libraryBaseline)
	}
}
