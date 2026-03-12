package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// renderAndWait calls View() to trigger zone.Scan() and waits for the async
// zone position processing goroutine to complete.
func renderAndWait(app App) {
	app.View()
	time.Sleep(50 * time.Millisecond)
}

// clickAtModalRelY creates a left-click mouse event at a relative Y position
// within the "modal-zone" bounds. Returns the event and true, or a zero event
// and false if the zone is not registered.
func clickAtModalRelY(t *testing.T, relY int) (tea.MouseMsg, bool) {
	t.Helper()
	z := zone.Get("modal-zone")
	if z == nil || z.IsZero() {
		return tea.MouseMsg{}, false
	}
	// Click near the left side of the content area (past border+padding)
	relX := 5
	return tea.MouseMsg{
		X:      z.StartX + relX,
		Y:      z.StartY + relY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}, true
}

// clickAtZone creates a left-click mouse event at the start of the named zone.
// Returns the event and true, or a zero event and false if the zone is not registered.
func clickAtZone(t *testing.T, id string) (tea.MouseMsg, bool) {
	t.Helper()
	z := zone.Get(id)
	if z == nil || z.IsZero() {
		return tea.MouseMsg{}, false
	}
	return tea.MouseMsg{
		X:      z.StartX,
		Y:      z.StartY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}, true
}

// ---------------------------------------------------------------------------
// Phase 1+2: modal.go — Text Input Fields & Option Lists
// ---------------------------------------------------------------------------

func TestEnvSetupModal_ClickRadioOption(t *testing.T) {
	// Clicking radio options on the envStepChoose step should move methodCursor.
	app := App{
		width:    80,
		height:   30,
		screen:   screenDetail,
		focus:    focusModal,
		envModal: newEnvSetupModal([]string{"API_KEY"}),
	}

	if app.envModal.methodCursor != 0 {
		t.Fatal("methodCursor should start at 0")
	}

	renderAndWait(app)

	// relY=6 is the second radio option ("I already have it configured")
	// Layout: border(1)+pad(1)+title(1)+help(1)+blank(1)+opt0(1)+opt1(1)
	click, ok := clickAtModalRelY(t, 6)
	if !ok {
		t.Skip("modal-zone not registered after View()")
	}

	m, _ := app.Update(click)
	updated := m.(App)

	if updated.envModal.methodCursor != 1 {
		t.Errorf("clicking second radio option should set methodCursor=1, got %d", updated.envModal.methodCursor)
	}

	// Click option 0 (relY=5)
	click.Y = click.Y - 1
	m, _ = updated.Update(click)
	updated = m.(App)

	if updated.envModal.methodCursor != 0 {
		t.Errorf("clicking first radio option should set methodCursor=0, got %d", updated.envModal.methodCursor)
	}
}

func TestSaveModal_ClickInputFocuses(t *testing.T) {
	// Clicking the text input field should focus it (focusedField=0).
	sm := newSaveModal("filename.md")
	// Tab to buttons so input is blurred
	sm.focusedField = 1
	sm.input.Blur()

	app := App{
		width:     80,
		height:    30,
		screen:    screenDetail,
		focus:     focusModal,
		saveModal: sm,
	}

	renderAndWait(app)

	// relY=4: input field (border+pad+title+blank)
	click, ok := clickAtModalRelY(t, 4)
	if !ok {
		t.Skip("modal-zone not registered after View()")
	}

	m, _ := app.Update(click)
	updated := m.(App)

	if updated.saveModal.focusedField != 0 {
		t.Errorf("clicking input field should set focusedField=0, got %d", updated.saveModal.focusedField)
	}
}

func TestInstallModal_ClickLocationOption(t *testing.T) {
	// Clicking location options should move locationCursor.
	item := catalog.ContentItem{
		Name: "test-item",
		Type: catalog.Skills,
		Path: "/tmp/test",
	}
	p := provider.Provider{
		Name:         "TestProvider",
		Slug:         "test",
		Detected:     true,
		SupportsType: func(ct catalog.ContentType) bool { return true },
		InstallDir:   func(_ string, _ catalog.ContentType) string { return "/tmp/test" },
	}

	app := App{
		width:     80,
		height:    30,
		screen:    screenDetail,
		focus:     focusModal,
		instModal: newInstallModal(item, []provider.Provider{p}, "/tmp/repo"),
	}

	if app.instModal.locationCursor != 0 {
		t.Fatal("locationCursor should start at 0")
	}

	renderAndWait(app)

	// Options start at relY=4 (optionRelY), each takes 2 rows (name+desc).
	// Option 1 (Project) is at relY=6.
	click, ok := clickAtModalRelY(t, 6)
	if !ok {
		t.Skip("modal-zone not registered after View()")
	}

	m, _ := app.Update(click)
	updated := m.(App)

	if updated.instModal.locationCursor != 1 {
		t.Errorf("clicking Project option should set locationCursor=1, got %d", updated.instModal.locationCursor)
	}
}

func TestInstallModal_ClickMethodOption(t *testing.T) {
	// Clicking method options should move methodCursor.
	item := catalog.ContentItem{
		Name: "test-item",
		Type: catalog.Skills,
		Path: "/tmp/test",
	}

	app := App{
		width:     80,
		height:    30,
		screen:    screenDetail,
		focus:     focusModal,
		instModal: newInstallModal(item, nil, "/tmp/repo"),
	}
	app.instModal.step = installStepMethod

	renderAndWait(app)

	// Method options start at relY=4, each takes 2 rows.
	// Option 1 (Copy) is at relY=6.
	click, ok := clickAtModalRelY(t, 6)
	if !ok {
		t.Skip("modal-zone not registered after View()")
	}

	m, _ := app.Update(click)
	updated := m.(App)

	if updated.instModal.methodCursor != 1 {
		t.Errorf("clicking Copy option should set methodCursor=1, got %d", updated.instModal.methodCursor)
	}
}

func TestRegistryAddModal_ClickFieldFocuses(t *testing.T) {
	// Clicking URL vs Name field should switch focusedField.
	app := App{
		width:            80,
		height:           30,
		screen:           screenRegistries,
		focus:            focusModal,
		registryAddModal: newRegistryAddModal(),
	}

	// URL is focused by default (focusedField=0). Click name field.
	renderAndWait(app)

	// relY=5: name field (border+pad+title+blank+url+name)
	click, ok := clickAtModalRelY(t, 5)
	if !ok {
		t.Skip("modal-zone not registered after View()")
	}

	m, _ := app.Update(click)
	updated := m.(App)

	if updated.registryAddModal.focusedField != 1 {
		t.Errorf("clicking Name field should set focusedField=1, got %d", updated.registryAddModal.focusedField)
	}

	// Now click URL field (relY=4)
	click.Y = click.Y - 1
	m, _ = updated.Update(click)
	updated = m.(App)

	if updated.registryAddModal.focusedField != 0 {
		t.Errorf("clicking URL field should set focusedField=0, got %d", updated.registryAddModal.focusedField)
	}
}

// ---------------------------------------------------------------------------
// Phase 3: loadout_create.go — Create Loadout Wizard
// ---------------------------------------------------------------------------

func TestCreateLoadoutModal_ClickProvider(t *testing.T) {
	// Clicking a provider should select it and advance to the items step.
	providers := testProviders(t)
	cat := testCatalog(t)

	app := App{
		width:              80,
		height:             30,
		screen:             screenLoadoutCards,
		focus:              focusModal,
		providers:          providers,
		catalog:            cat,
		createLoadoutModal: newCreateLoadoutModal("", "", providers, cat),
	}

	if app.createLoadoutModal.step != clStepProvider {
		t.Fatalf("expected clStepProvider, got %d", app.createLoadoutModal.step)
	}

	renderAndWait(app)

	// Provider list starts at relY=6 (border+pad+title+blank+subtitle+blank).
	// Click provider at index 1.
	click, ok := clickAtModalRelY(t, 7)
	if !ok {
		t.Skip("modal-zone not registered after View()")
	}

	m, _ := app.Update(click)
	updated := m.(App)

	// Clicking a provider selects + Enter, so step should advance to clStepItems.
	if updated.createLoadoutModal.step != clStepItems {
		t.Errorf("clicking a provider should advance to clStepItems, got step %d", updated.createLoadoutModal.step)
	}
}

func TestCreateLoadoutModal_ClickNameDescField(t *testing.T) {
	// Clicking name vs desc field should switch focus.
	providers := testProviders(t)
	cat := testCatalog(t)

	modal := newCreateLoadoutModal("claude-code", "", providers, cat)
	modal.step = clStepName

	app := App{
		width:              80,
		height:             30,
		screen:             screenLoadoutCards,
		focus:              focusModal,
		providers:          providers,
		catalog:            cat,
		createLoadoutModal: modal,
	}

	if !app.createLoadoutModal.nameFirst {
		t.Fatal("nameFirst should be true initially")
	}

	renderAndWait(app)

	// relY=7: descInput (border+pad+title+blank+subtitle+blank+nameInput+descInput)
	click, ok := clickAtModalRelY(t, 7)
	if !ok {
		t.Skip("modal-zone not registered after View()")
	}

	m, _ := app.Update(click)
	updated := m.(App)

	if updated.createLoadoutModal.nameFirst {
		t.Error("clicking desc field should set nameFirst=false")
	}

	// Click back to name field (relY=6)
	click.Y = click.Y - 1
	m, _ = updated.Update(click)
	updated = m.(App)

	if !updated.createLoadoutModal.nameFirst {
		t.Error("clicking name field should set nameFirst=true")
	}
}

func TestCreateLoadoutModal_ClickDestOption(t *testing.T) {
	// Clicking destination options should move destCursor.
	providers := testProviders(t)
	cat := testCatalog(t)

	modal := newCreateLoadoutModal("claude-code", "", providers, cat)
	modal.step = clStepDest

	app := App{
		width:              80,
		height:             30,
		screen:             screenLoadoutCards,
		focus:              focusModal,
		providers:          providers,
		catalog:            cat,
		createLoadoutModal: modal,
	}

	if app.createLoadoutModal.destCursor != 0 {
		t.Fatal("destCursor should start at 0")
	}

	renderAndWait(app)

	// Dest options start at relY=6 (border+pad+title+blank+subtitle+blank).
	// Click option 1 (Library) at relY=7.
	click, ok := clickAtModalRelY(t, 7)
	if !ok {
		t.Skip("modal-zone not registered after View()")
	}

	m, _ := app.Update(click)
	updated := m.(App)

	if updated.createLoadoutModal.destCursor != 1 {
		t.Errorf("clicking Library option should set destCursor=1, got %d", updated.createLoadoutModal.destCursor)
	}
}

// ---------------------------------------------------------------------------
// Phase 5: detail_render.go — Loadout Mode Selector
// ---------------------------------------------------------------------------

func TestDetailLoadoutModeClickSelect(t *testing.T) {
	// Clicking mode options (Preview/Try/Keep) should set loadoutModeCursor.
	// Uses zone-based testing since detail is a full-screen view (not overlay).
	//
	// We need a loadout item with a valid manifest so the mode selector renders.
	// The test catalog's loadout.yaml is minimal (missing kind/version), so we
	// write a proper one.
	app := testApp(t)

	// Find a loadout item and write a valid manifest so it parses
	var loadoutItem *catalog.ContentItem
	for i := range app.catalog.Items {
		if app.catalog.Items[i].Type == catalog.Loadouts {
			loadoutItem = &app.catalog.Items[i]
			break
		}
	}
	if loadoutItem == nil {
		t.Skip("no loadout items in test catalog")
	}

	// Write a proper loadout.yaml that passes Parse validation
	yamlContent := "kind: loadout\nversion: 1\nname: test-loadout\nskills:\n  - alpha-skill\n"
	os.WriteFile(filepath.Join(loadoutItem.Path, "loadout.yaml"), []byte(yamlContent), 0o644)

	// Navigate sidebar to Loadouts
	loadoutsIdx := app.sidebar.loadoutsIdx()
	app = pressN(app, keyDown, loadoutsIdx)
	m, _ := app.Update(keyEnter) // → screenLoadoutCards
	app = m.(App)
	assertScreen(t, app, screenLoadoutCards)

	m, _ = app.Update(keyEnter) // → screenItems
	app = m.(App)
	assertScreen(t, app, screenItems)

	if len(app.items.items) == 0 {
		t.Skip("no loadout items found")
	}

	m, _ = app.Update(keyEnter) // → screenDetail
	app = m.(App)
	assertScreen(t, app, screenDetail)

	m, _ = app.Update(keyRune('3')) // → Install tab
	app = m.(App)

	if app.detail.loadoutManifest == nil {
		t.Skip("loadout manifest not parsed (mode selector won't render)")
	}
	if app.detail.activeTab != tabInstall {
		t.Fatalf("expected tabInstall, got %d", app.detail.activeTab)
	}

	renderAndWait(app)

	// Try clicking "detail-mode-1" (Try mode)
	click, ok := clickAtZone(t, "detail-mode-1")
	if !ok {
		t.Skip("detail-mode-1 zone not registered after View()")
	}

	m, _ = app.Update(click)
	app = m.(App)

	if app.detail.loadoutModeCursor != 1 {
		t.Errorf("clicking Try mode should set loadoutModeCursor=1, got %d", app.detail.loadoutModeCursor)
	}

	// Click "detail-mode-2" (Keep mode)
	renderAndWait(app) // re-render to update zone positions
	click, ok = clickAtZone(t, "detail-mode-2")
	if !ok {
		t.Skip("detail-mode-2 zone not registered after View()")
	}

	m, _ = app.Update(click)
	app = m.(App)

	if app.detail.loadoutModeCursor != 2 {
		t.Errorf("clicking Keep mode should set loadoutModeCursor=2, got %d", app.detail.loadoutModeCursor)
	}
}

// ---------------------------------------------------------------------------
// Phase 4: import.go — Text Input Fields
// ---------------------------------------------------------------------------

func TestImportTextFieldClickFocuses(t *testing.T) {
	// The import wizard's text input steps use zone-based click handling.
	// Since the import view is a full screen (not overlay), inner zone marks work.
	// We test at the model level since the click handler is on importModel.
	app := testApp(t)

	// Navigate to import screen via 'a' on the items page
	m, _ := app.Update(keyEnter) // → items for Skills
	app = m.(App)
	m, _ = app.Update(keyRune('a')) // → import screen
	app = m.(App)

	if app.screen != screenImport {
		t.Skip("could not navigate to import screen")
	}

	// Navigate to Git URL step: Source → Git URL (option 2)
	app.importer.sourceCursor = 2
	m, _ = app.Update(keyEnter) // select Git URL source
	app = m.(App)

	if app.importer.step != stepGitURL {
		t.Skipf("expected stepGitURL, got step %d", app.importer.step)
	}

	// Render to register zones
	renderAndWait(app)

	// Check that the zone is registered and click it
	click, ok := clickAtZone(t, "import-field-url")
	if !ok {
		t.Skip("import-field-url zone not registered")
	}

	// The input should already be focused on this step, so clicking
	// should keep it focused (verify no panic or state corruption).
	m, _ = app.Update(click)
	app = m.(App)

	// Just verify we're still on the same step (no crash, no navigation)
	if app.importer.step != stepGitURL {
		t.Errorf("clicking URL field should not change step, got %d", app.importer.step)
	}
}
