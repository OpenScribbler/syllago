package tui_v1

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// navigateToImport creates a test app and navigates to the import screen.
func navigateToImport(t *testing.T) App {
	t.Helper()
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+3) // Add
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenImport)
	return app
}

// navigateToLocalPath navigates to the import screen and selects "Local Path"
// (source cursor 1) to reach stepType.
func navigateToLocalPath(t *testing.T) App {
	t.Helper()
	app := navigateToImport(t)
	app = pressN(app, keyDown, 1) // Local Path
	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepType {
		t.Fatalf("navigateToLocalPath: expected stepType, got %d", app.importer.step)
	}
	return app
}

// ---------------------------------------------------------------------------
// Source selection (step 0)
// ---------------------------------------------------------------------------

func TestImportSourceNavigation(t *testing.T) {
	app := navigateToImport(t)
	if app.importer.step != stepSource {
		t.Fatalf("expected stepSource, got %d", app.importer.step)
	}

	// 4 options: From Provider(0), Local(1), Git(2), Create(3)
	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.importer.sourceCursor != 1 {
		t.Fatalf("expected sourceCursor 1, got %d", app.importer.sourceCursor)
	}

	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.importer.sourceCursor != 2 {
		t.Fatalf("expected sourceCursor 2, got %d", app.importer.sourceCursor)
	}

	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.importer.sourceCursor != 3 {
		t.Fatalf("expected sourceCursor 3, got %d", app.importer.sourceCursor)
	}

	// Bounds clamping
	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.importer.sourceCursor != 3 {
		t.Fatal("sourceCursor should clamp at 3")
	}
}

func TestImportSourceSelectFromProvider(t *testing.T) {
	app := navigateToImport(t)
	// From Provider = cursor 0, just Enter
	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepProviderPick {
		t.Fatalf("expected stepProviderPick after From Provider, got %d", app.importer.step)
	}
}

func TestImportSourceFromProviderNoProviders(t *testing.T) {
	app := navigateToImport(t)
	app.importer.providers = nil // empty provider list

	m, _ := app.Update(keyEnter) // cursor 0 = From Provider
	app = m.(App)
	if app.importer.step != stepSource {
		t.Fatalf("expected to stay on stepSource with no providers, got %d", app.importer.step)
	}
	if app.importer.message == "" || !app.importer.messageIsErr {
		t.Fatal("expected error message about no providers detected")
	}
}

func TestImportSourceSelectLocal(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 1) // Local
	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepType {
		t.Fatalf("expected stepType after local, got %d", app.importer.step)
	}
	if app.importer.isCreate {
		t.Fatal("isCreate should be false for local")
	}
}

func TestImportSourceSelectGit(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 2) // Git
	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepGitURL {
		t.Fatalf("expected stepGitURL after git, got %d", app.importer.step)
	}
}

func TestImportSourceSelectCreate(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 3) // Create
	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepType {
		t.Fatalf("expected stepType after create, got %d", app.importer.step)
	}
	if !app.importer.isCreate {
		t.Fatal("isCreate should be true for create")
	}
}

func TestImportSourceEscBack(t *testing.T) {
	app := navigateToImport(t)
	m, _ := app.Update(keyEsc) // Esc from stepSource at app level → category
	app = m.(App)
	assertScreen(t, app, screenCategory)
}

// ---------------------------------------------------------------------------
// Provider pick (From Provider flow)
// ---------------------------------------------------------------------------

func TestProviderPickNavigation(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepProviderPick
	if len(app.importer.providers) < 2 {
		t.Skip("need at least 2 providers for navigation test")
	}

	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.importer.discoveryProvCursor != 1 {
		t.Fatalf("expected discoveryProvCursor 1, got %d", app.importer.discoveryProvCursor)
	}
}

func TestProviderPickEscBack(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepProviderPick

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepSource {
		t.Fatalf("expected stepSource after esc, got %d", app.importer.step)
	}
}

func TestProviderPickSelectDispatchesDiscovery(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepProviderPick
	if len(app.importer.providers) == 0 {
		t.Skip("no providers available")
	}

	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected async discovery command on provider select")
	}
}

// ---------------------------------------------------------------------------
// Discovery select (multi-select checklist)
// ---------------------------------------------------------------------------

// setupDiscoverySelect creates an app at stepDiscoverySelect with test items.
func setupDiscoverySelect(t *testing.T) App {
	t.Helper()
	app := navigateToImport(t)
	app.importer.step = stepDiscoverySelect
	app.importer.discoveryItems = []add.DiscoveryItem{
		{Name: "rule-one", Type: catalog.Rules, Path: "/tmp/r1", Status: add.StatusNew},
		{Name: "skill-two", Type: catalog.Skills, Path: "/tmp/s2", Status: add.StatusInLibrary},
		{Name: "agent-three", Type: catalog.Agents, Path: "/tmp/a3", Status: add.StatusOutdated},
	}
	// Pre-select: new and outdated selected, in-library deselected
	app.importer.discoverySelected = []bool{true, false, true}
	app.importer.discoveryCursor = 0
	return app
}

func TestDiscoverySelectNavigation(t *testing.T) {
	app := setupDiscoverySelect(t)

	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.importer.discoveryCursor != 1 {
		t.Fatalf("expected cursor 1, got %d", app.importer.discoveryCursor)
	}

	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.importer.discoveryCursor != 2 {
		t.Fatalf("expected cursor 2, got %d", app.importer.discoveryCursor)
	}

	// Clamp at end
	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.importer.discoveryCursor != 2 {
		t.Fatal("cursor should clamp at last item")
	}
}

func TestDiscoverySelectSpaceToggle(t *testing.T) {
	app := setupDiscoverySelect(t)

	// Item 0 is selected (new), toggle off
	m, _ := app.Update(keySpace)
	app = m.(App)
	if app.importer.discoverySelected[0] {
		t.Fatal("item 0 should be deselected after space")
	}

	// Toggle back on
	m, _ = app.Update(keySpace)
	app = m.(App)
	if !app.importer.discoverySelected[0] {
		t.Fatal("item 0 should be selected after second space")
	}
}

func TestDiscoverySelectAll(t *testing.T) {
	app := setupDiscoverySelect(t)

	// Press 'a' to select all
	m, _ := app.Update(keyRune('a'))
	app = m.(App)
	for i, sel := range app.importer.discoverySelected {
		if !sel {
			t.Fatalf("item %d should be selected after 'a'", i)
		}
	}
}

func TestDiscoveryDeselectAll(t *testing.T) {
	app := setupDiscoverySelect(t)

	// Press 'n' to deselect all
	m, _ := app.Update(keyRune('n'))
	app = m.(App)
	for i, sel := range app.importer.discoverySelected {
		if sel {
			t.Fatalf("item %d should be deselected after 'n'", i)
		}
	}
}

func TestDiscoverySelectEscBack(t *testing.T) {
	app := setupDiscoverySelect(t)

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepProviderPick {
		t.Fatalf("expected stepProviderPick after esc, got %d", app.importer.step)
	}
}

func TestDiscoverySelectEnterNoSelection(t *testing.T) {
	app := setupDiscoverySelect(t)
	// Deselect all
	app.importer.discoverySelected = []bool{false, false, false}

	m, _ := app.Update(keyEnter)
	app = m.(App)
	// Should stay on same step with error message
	if app.importer.step != stepDiscoverySelect {
		t.Fatalf("expected to stay on stepDiscoverySelect with no selection, got %d", app.importer.step)
	}
	if app.importer.message == "" || !app.importer.messageIsErr {
		t.Fatal("expected error message about no items selected")
	}
}

func TestDiscoverySelectViewRendering(t *testing.T) {
	app := setupDiscoverySelect(t)
	app.width = 80
	app.height = 30

	view := app.View()
	assertContains(t, view, "rule-one")
	assertContains(t, view, "skill-two")
	assertContains(t, view, "agent-three")
	assertContains(t, view, "Select All")
	assertContains(t, view, "Deselect All")
	assertContains(t, view, "Add Selected")
	assertContains(t, view, "Actions")
}

func TestDiscoveryDoneMsgPreselection(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepProviderPick

	// Simulate discoveryDoneMsg
	msg := discoveryDoneMsg{
		items: []add.DiscoveryItem{
			{Name: "new-item", Status: add.StatusNew},
			{Name: "lib-item", Status: add.StatusInLibrary},
			{Name: "old-item", Status: add.StatusOutdated},
		},
	}
	m, _ := app.Update(msg)
	app = m.(App)

	if app.importer.step != stepDiscoverySelect {
		t.Fatalf("expected stepDiscoverySelect, got %d", app.importer.step)
	}
	// New and outdated should be pre-selected, in-library should not
	expected := []bool{true, false, true}
	for i, sel := range app.importer.discoverySelected {
		if sel != expected[i] {
			t.Fatalf("item %d: expected selected=%v, got %v", i, expected[i], sel)
		}
	}
}

func TestDiscoverySelectScrollOverflow(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepDiscoverySelect
	app.importer.height = 20 // small panel height

	// Create 30 items to overflow the viewport
	items := make([]add.DiscoveryItem, 30)
	selected := make([]bool, 30)
	for i := range items {
		items[i] = add.DiscoveryItem{
			Name:   fmt.Sprintf("item-%02d", i),
			Type:   catalog.Rules,
			Path:   fmt.Sprintf("/tmp/item-%02d", i),
			Status: add.StatusNew,
		}
		selected[i] = true
	}
	app.importer.discoveryItems = items
	app.importer.discoverySelected = selected
	app.importer.discoveryCursor = 0
	app.importer.discoveryScrollOffset = 0

	// Buttons should be visible even with overflow
	view := app.View()
	assertContains(t, view, "Select All")
	assertContains(t, view, "Deselect All")
	assertContains(t, view, "Add Selected")
	// Scroll down indicator should appear
	assertContains(t, view, "more below")
	// First item visible
	assertContains(t, view, "item-00")

	// Navigate cursor down past viewport — scroll should follow
	for i := 0; i < 15; i++ {
		m, _ := app.Update(keyDown)
		app = m.(App)
	}
	if app.importer.discoveryCursor != 15 {
		t.Fatalf("expected cursor 15, got %d", app.importer.discoveryCursor)
	}
	if app.importer.discoveryScrollOffset == 0 {
		t.Fatal("expected scrollOffset to advance past 0 after navigating down")
	}

	// Buttons should still be visible after scrolling
	view = app.View()
	assertContains(t, view, "Select All")
	assertContains(t, view, "Add Selected")

	// Home key should jump to top
	m, _ := app.Update(tea.KeyMsg{Type: tea.KeyHome})
	app = m.(App)
	if app.importer.discoveryCursor != 0 {
		t.Fatalf("expected cursor 0 after Home, got %d", app.importer.discoveryCursor)
	}
	if app.importer.discoveryScrollOffset != 0 {
		t.Fatalf("expected scrollOffset 0 after Home, got %d", app.importer.discoveryScrollOffset)
	}

	// End key should jump to last
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnd})
	app = m.(App)
	if app.importer.discoveryCursor != 29 {
		t.Fatalf("expected cursor 29 after End, got %d", app.importer.discoveryCursor)
	}
}

func TestDiscoveryDoneMsgError(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepProviderPick

	msg := discoveryDoneMsg{err: fmt.Errorf("discovery failed")}
	m, _ := app.Update(msg)
	app = m.(App)

	if app.importer.step != stepProviderPick {
		t.Fatalf("expected to stay on stepProviderPick on error, got %d", app.importer.step)
	}
	assertContains(t, app.importer.message, "discovery failed")
}

// ---------------------------------------------------------------------------
// Type selection (step 1)
// ---------------------------------------------------------------------------

func TestImportTypeNavigation(t *testing.T) {
	app := navigateToLocalPath(t)

	nTypes := len(app.importer.types)
	app = pressN(app, keyDown, nTypes+5)
	if app.importer.typeCursor != nTypes-1 {
		t.Fatalf("expected typeCursor clamped at %d, got %d", nTypes-1, app.importer.typeCursor)
	}
}

func TestImportTypeUniversalToBrowse(t *testing.T) {
	app := navigateToLocalPath(t)

	// Skills is universal and at cursor 0
	m, _ := app.Update(keyEnter) // select Skills
	app = m.(App)
	if app.importer.step != stepBrowseStart {
		t.Fatalf("expected stepBrowseStart for universal type, got %d", app.importer.step)
	}
}

func TestImportTypeProviderSpecificToProvider(t *testing.T) {
	app := navigateToLocalPath(t)

	// Find a provider-specific type (e.g., Rules)
	for i, ct := range app.importer.types {
		if !ct.IsUniversal() {
			app = pressN(app, keyDown, i)
			break
		}
	}

	m, _ := app.Update(keyEnter)
	app = m.(App)

	// Should go to stepProvider or show "no provider dirs" message
	if app.importer.step == stepProvider {
		// Good — found provider directories
	} else if app.importer.message != "" {
		// Also acceptable — no provider directories found
	} else {
		t.Fatalf("expected stepProvider or error message, got step %d", app.importer.step)
	}
}

func TestImportTypeCreateToName(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 3) // Create
	m, _ := app.Update(keyEnter)  // → stepType (create flow)
	app = m.(App)

	// Select any type
	m, _ = app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepName {
		t.Fatalf("expected stepName for create flow, got %d", app.importer.step)
	}
}

func TestImportTypeCreateProviderSpecificToProvider(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 3) // Create
	m, _ := app.Update(keyEnter)  // → stepType (create flow)
	app = m.(App)

	// Select Rules (index 3, first provider-specific type)
	app = pressN(app, keyDown, 3)
	m, _ = app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepProvider {
		t.Fatalf("expected stepProvider for create flow with provider-specific type, got %d", app.importer.step)
	}

	// Select a provider, should go to stepName
	m, _ = app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepName {
		t.Fatalf("expected stepName after provider selection in create flow, got %d", app.importer.step)
	}
	if app.importer.providerName == "" {
		t.Fatal("expected providerName to be set after provider selection")
	}
}

func TestImportTypeEscBack(t *testing.T) {
	app := navigateToLocalPath(t)

	m, _ := app.Update(keyEsc) // back to source
	app = m.(App)
	if app.importer.step != stepSource {
		t.Fatalf("expected stepSource after esc, got %d", app.importer.step)
	}
}

// ---------------------------------------------------------------------------
// Browse start (step 3)
// ---------------------------------------------------------------------------

func TestImportBrowseStartOptions(t *testing.T) {
	app := navigateToLocalPath(t)
	m, _ := app.Update(keyEnter) // → stepBrowseStart (Skills)
	app = m.(App)

	if app.importer.step != stepBrowseStart {
		t.Fatalf("expected stepBrowseStart, got %d", app.importer.step)
	}

	// 3 options: cwd(0), home(1), custom(2)
	app = pressN(app, keyDown, 2)
	if app.importer.browseCursor != 2 {
		t.Fatalf("expected browseCursor 2, got %d", app.importer.browseCursor)
	}
}

func TestImportBrowseStartEsc(t *testing.T) {
	app := navigateToLocalPath(t)
	m, _ := app.Update(keyEnter) // → stepBrowseStart
	app = m.(App)

	m, _ = app.Update(keyEsc) // back
	app = m.(App)
	if app.importer.step != stepType {
		t.Fatalf("expected stepType after esc from browseStart, got %d", app.importer.step)
	}
}

// ---------------------------------------------------------------------------
// Path input (step 6)
// ---------------------------------------------------------------------------

func TestImportPathInput(t *testing.T) {
	app := navigateToLocalPath(t)
	m, _ := app.Update(keyEnter) // → stepBrowseStart
	app = m.(App)

	// Select custom path (option 2)
	app = pressN(app, keyDown, 2)
	m, _ = app.Update(keyEnter) // → stepPath
	app = m.(App)

	if app.importer.step != stepPath {
		t.Fatalf("expected stepPath, got %d", app.importer.step)
	}

	// Enter with empty path stays
	m, _ = app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepPath {
		t.Fatalf("expected to stay at stepPath with empty input, got %d", app.importer.step)
	}
}

func TestImportPathInvalid(t *testing.T) {
	app := navigateToLocalPath(t)
	m, _ := app.Update(keyEnter) // → stepBrowseStart
	app = m.(App)
	app = pressN(app, keyDown, 2)
	m, _ = app.Update(keyEnter) // → stepPath
	app = m.(App)

	// Type an invalid path
	app.importer.pathInput.SetValue("/nonexistent/path/xyz")
	m, _ = app.Update(keyEnter)
	app = m.(App)

	if app.importer.message == "" {
		t.Fatal("expected error message for invalid path")
	}
}

func TestImportPathEsc(t *testing.T) {
	app := navigateToLocalPath(t)
	m, _ := app.Update(keyEnter) // → stepBrowseStart
	app = m.(App)
	app = pressN(app, keyDown, 2)
	m, _ = app.Update(keyEnter) // → stepPath
	app = m.(App)

	m, _ = app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepBrowseStart {
		t.Fatalf("expected stepBrowseStart after esc, got %d", app.importer.step)
	}
}

// ---------------------------------------------------------------------------
// Git URL input (step 7)
// ---------------------------------------------------------------------------

func TestImportGitURLEmpty(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 2) // Git
	m, _ := app.Update(keyEnter)  // → stepGitURL
	app = m.(App)

	// Enter with empty URL stays
	m, _ = app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepGitURL {
		t.Fatalf("expected to stay at stepGitURL with empty URL, got %d", app.importer.step)
	}
}

func TestImportGitURLInvalid(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 2)
	m, _ := app.Update(keyEnter) // → stepGitURL
	app = m.(App)

	// Type invalid URL
	app.importer.urlInput.SetValue("not-a-valid-url")
	m, _ = app.Update(keyEnter)
	app = m.(App)

	assertContains(t, app.importer.message, "Invalid URL")
}

func TestImportGitURLEsc(t *testing.T) {
	app := navigateToImport(t)
	app = pressN(app, keyDown, 2)
	m, _ := app.Update(keyEnter) // → stepGitURL
	app = m.(App)

	m, _ = app.Update(keyEsc) // back to source
	app = m.(App)
	if app.importer.step != stepSource {
		t.Fatalf("expected stepSource after esc, got %d", app.importer.step)
	}
}

// ---------------------------------------------------------------------------
// Clone security protections
// ---------------------------------------------------------------------------

func TestImportCloneArgs_SecurityProtections(t *testing.T) {
	t.Parallel()
	args := importCloneArgs("https://github.com/acme/tools.git", "/tmp/clone")

	// Must contain -c core.hooksPath=/dev/null to disable git hooks
	foundHooksPath := false
	for i, a := range args {
		if a == "-c" && i+1 < len(args) && args[i+1] == "core.hooksPath=/dev/null" {
			foundHooksPath = true
			break
		}
	}
	if !foundHooksPath {
		t.Errorf("importCloneArgs missing -c core.hooksPath=/dev/null, got %v", args)
	}

	// Must contain --no-recurse-submodules
	foundNoSubmodules := false
	for _, a := range args {
		if a == "--no-recurse-submodules" {
			foundNoSubmodules = true
			break
		}
	}
	if !foundNoSubmodules {
		t.Errorf("importCloneArgs missing --no-recurse-submodules, got %v", args)
	}

	// Must contain --depth 1 (shallow clone)
	foundDepth := false
	for i, a := range args {
		if a == "--depth" && i+1 < len(args) && args[i+1] == "1" {
			foundDepth = true
			break
		}
	}
	if !foundDepth {
		t.Errorf("importCloneArgs missing --depth 1, got %v", args)
	}

	// The -c flag must come BEFORE clone to be a global git option
	cloneIdx := -1
	hooksIdx := -1
	for i, a := range args {
		if a == "clone" {
			cloneIdx = i
		}
		if a == "core.hooksPath=/dev/null" {
			hooksIdx = i
		}
	}
	if cloneIdx < 0 {
		t.Fatal("clone subcommand not found in args")
	}
	if hooksIdx < 0 {
		t.Fatal("core.hooksPath=/dev/null not found in args")
	}
	if hooksIdx >= cloneIdx {
		t.Errorf("core.hooksPath=/dev/null (index %d) must come before clone (index %d) to be a global git option", hooksIdx, cloneIdx)
	}
}

// ---------------------------------------------------------------------------
// Clone done message handling
// ---------------------------------------------------------------------------

func TestImportCloneDoneSuccess(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepGitURL

	// Simulate successful clone with scanned items
	tmpDir := t.TempDir()
	// Create a scannable skill in the temp dir
	makeSkill(t, tmpDir, "cloned-skill", "From git", false)

	msg := importCloneDoneMsg{err: nil, path: tmpDir}
	m, _ := app.Update(msg)
	app = m.(App)

	if app.importer.step != stepGitPick {
		t.Fatalf("expected stepGitPick after successful clone, got %d", app.importer.step)
	}
	if len(app.importer.clonedItems) == 0 {
		t.Fatal("expected cloned items after scan")
	}
}

func TestImportCloneDoneError(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepGitURL

	msg := importCloneDoneMsg{err: fmt.Errorf("clone failed"), path: ""}
	m, _ := app.Update(msg)
	app = m.(App)

	if app.importer.step != stepGitURL {
		t.Fatalf("expected to stay at stepGitURL on error, got %d", app.importer.step)
	}
	assertContains(t, app.importer.message, "Clone failed")
}

// ---------------------------------------------------------------------------
// Git pick (step 8)
// ---------------------------------------------------------------------------

func TestImportGitPickNavigation(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepGitPick
	app.importer.clonedPath = t.TempDir()
	app.importer.clonedItems = []catalog.ContentItem{
		{Name: "item-1", Type: catalog.Skills},
		{Name: "item-2", Type: catalog.Agents},
	}
	app.importer.pickCursor = 0

	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.importer.pickCursor != 1 {
		t.Fatalf("expected pickCursor 1, got %d", app.importer.pickCursor)
	}
}

func TestImportGitPickSelect(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepGitPick
	app.importer.clonedPath = t.TempDir()
	app.importer.clonedItems = []catalog.ContentItem{
		{Name: "picked-item", Type: catalog.Skills, Path: "/tmp/picked"},
	}

	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepConfirm {
		t.Fatalf("expected stepConfirm, got %d", app.importer.step)
	}
	if app.importer.itemName != "picked-item" {
		t.Fatalf("expected itemName 'picked-item', got %q", app.importer.itemName)
	}
}

func TestImportGitPickEscCleanup(t *testing.T) {
	app := navigateToImport(t)
	tmpDir := t.TempDir()
	app.importer.step = stepGitPick
	app.importer.clonedPath = tmpDir
	app.importer.clonedItems = []catalog.ContentItem{
		{Name: "temp-item", Type: catalog.Skills},
	}

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepGitURL {
		t.Fatalf("expected stepGitURL after esc, got %d", app.importer.step)
	}
	if app.importer.clonedPath != "" {
		t.Fatal("expected clonedPath to be cleaned up")
	}
}

// ---------------------------------------------------------------------------
// Name input (step 10 - create flow)
// ---------------------------------------------------------------------------

func TestImportNameEnter(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepName
	app.importer.contentType = catalog.Skills
	app.importer.isCreate = true
	app.importer.nameInput.SetValue("my-new-skill")

	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepConfirm {
		t.Fatalf("expected stepConfirm, got %d", app.importer.step)
	}
	if app.importer.itemName != "my-new-skill" {
		t.Fatalf("expected itemName 'my-new-skill', got %q", app.importer.itemName)
	}
}

func TestImportNameEmpty(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepName
	app.importer.contentType = catalog.Skills
	app.importer.nameInput.SetValue("")

	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepName {
		t.Fatal("expected to stay at stepName with empty name")
	}
	if app.importer.message != "name is required" {
		t.Errorf("message = %q, want 'name is required'", app.importer.message)
	}
}

func TestImportNameInvalid(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{"path traversal", "../../evil", "name may only contain letters, numbers, hyphens, and underscores"},
		{"leading dash", "-bad", "name must not start with a dash"},
		{"dots", "foo.bar", "name may only contain letters, numbers, hyphens, and underscores"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := navigateToImport(t)
			app.importer.step = stepName
			app.importer.contentType = catalog.Skills
			app.importer.nameInput.SetValue(tc.input)

			m, _ := app.Update(keyEnter)
			app = m.(App)
			if app.importer.step != stepName {
				t.Fatalf("expected to stay at stepName with invalid name %q", tc.input)
			}
			if app.importer.message != tc.wantMsg {
				t.Errorf("message = %q, want %q", app.importer.message, tc.wantMsg)
			}
		})
	}
}

func TestImportNameEsc(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepName
	app.importer.contentType = catalog.Skills

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepType {
		t.Fatalf("expected stepType after esc, got %d", app.importer.step)
	}
}

// ---------------------------------------------------------------------------
// Validate (step 5)
// ---------------------------------------------------------------------------

func TestImportValidateNavigation(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepValidate
	app.importer.validationItems = []validationItem{
		{name: "item-1", included: true},
		{name: "item-2", included: true},
		{name: "item-3", included: true},
	}

	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.importer.validateCursor != 1 {
		t.Fatalf("expected validateCursor 1, got %d", app.importer.validateCursor)
	}
}

func TestImportValidateToggle(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepValidate
	app.importer.validationItems = []validationItem{
		{name: "item-1", included: true},
	}

	m, _ := app.Update(keySpace)
	app = m.(App)
	if app.importer.validationItems[0].included {
		t.Fatal("space should toggle included to false")
	}

	m, _ = app.Update(keySpace)
	app = m.(App)
	if !app.importer.validationItems[0].included {
		t.Fatal("space should toggle included back to true")
	}
}

func TestImportValidateNoneSelected(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepValidate
	app.importer.validationItems = []validationItem{
		{name: "item-1", included: false},
	}

	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertContains(t, app.importer.message, "No items selected")
}

func TestImportValidateSingle(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepValidate
	app.importer.contentType = catalog.Skills
	app.importer.validationItems = []validationItem{
		{path: "/tmp/single-item", name: "single-item", included: true},
	}

	m, _ := app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepConfirm {
		t.Fatalf("expected stepConfirm for single selection, got %d", app.importer.step)
	}
}

func TestImportValidateEsc(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepValidate
	app.importer.validationItems = []validationItem{{path: "/tmp/test", name: "test", included: true}}

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepBrowse {
		t.Fatalf("expected stepBrowse after esc, got %d", app.importer.step)
	}
}

// ---------------------------------------------------------------------------
// Confirm (step 9)
// ---------------------------------------------------------------------------

func TestImportConfirmEnter(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConfirm
	app.importer.sourcePath = app.catalog.Items[0].Path
	app.importer.itemName = "confirm-test"
	app.importer.contentType = catalog.Skills

	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected import command on confirm")
	}
}

func TestImportConfirmEscLocal(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConfirm
	app.importer.contentType = catalog.Skills
	app.importer.isCreate = false
	app.importer.clonedPath = "" // not from git

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepValidate {
		t.Fatalf("expected stepValidate after esc from confirm (local flow), got %d", app.importer.step)
	}
}

func TestImportConfirmEscGit(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConfirm
	app.importer.contentType = catalog.Skills
	app.importer.isCreate = false
	app.importer.clonedPath = "/tmp/fake-clone"

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepGitPick {
		t.Fatalf("expected stepGitPick after esc from confirm (git flow), got %d", app.importer.step)
	}
}

func TestImportConfirmEscCreate(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConfirm
	app.importer.contentType = catalog.Skills
	app.importer.isCreate = true

	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.importer.step != stepName {
		t.Fatalf("expected stepName after esc from confirm (create flow), got %d", app.importer.step)
	}
}

// ---------------------------------------------------------------------------
// Import done message
// ---------------------------------------------------------------------------

func TestImportDoneSingleItem(t *testing.T) {
	app := navigateToImport(t)
	msg := importDoneMsg{name: "imported-item", contentType: catalog.Skills, err: nil}
	m, _ := app.Update(msg)
	app = m.(App)

	// Single item with known type navigates to detail or items list
	// (detail if the item is found in the rescanned catalog, items list otherwise)
	if app.screen != screenDetail && app.screen != screenItems {
		t.Fatalf("expected screenDetail or screenItems, got %d", app.screen)
	}
	assertContains(t, app.toast.text, "imported-item")
}

func TestImportDoneBatch(t *testing.T) {
	app := navigateToImport(t)
	msg := importDoneMsg{name: "item-a, item-b", contentType: catalog.Skills, err: nil}
	m, _ := app.Update(msg)
	app = m.(App)

	// Batch import with known type navigates to items list
	if app.screen != screenItems && app.screen != screenLibraryCards {
		t.Fatalf("expected screenItems or screenLibraryCards, got %d", app.screen)
	}
	assertContains(t, app.toast.text, "item-a, item-b")
}

func TestImportDoneNoContentType(t *testing.T) {
	app := navigateToImport(t)
	msg := importDoneMsg{name: "discovered-items", err: nil}
	m, _ := app.Update(msg)
	app = m.(App)

	// No content type (discovery/mixed) navigates to Library cards
	assertScreen(t, app, screenLibraryCards)
	assertContains(t, app.toast.text, "discovered-items")
}

func TestImportDoneError(t *testing.T) {
	app := navigateToImport(t)
	msg := importDoneMsg{name: "", err: fmt.Errorf("copy failed")}
	m, _ := app.Update(msg)
	app = m.(App)

	assertScreen(t, app, screenImport) // stays on import screen
	assertContains(t, app.importer.message, "Add failed")
}

// ---------------------------------------------------------------------------
// View rendering
// ---------------------------------------------------------------------------

func TestImportViewSource(t *testing.T) {
	app := navigateToImport(t)
	view := app.View()
	assertContains(t, view, "Add")
	assertContains(t, view, "From Provider")
	assertContains(t, view, "Local Path")
	assertContains(t, view, "Git URL")
	assertContains(t, view, "Create New")
}

func TestImportShowsBreadcrumb(t *testing.T) {
	app := navigateToImport(t)
	view := app.View()
	assertContains(t, view, "Home")
	assertContains(t, view, "Add")
}

// ---------------------------------------------------------------------------
// Git URL validation: secure transports only
// ---------------------------------------------------------------------------

func TestIsValidGitURL(t *testing.T) {
	tests := []struct {
		url   string
		valid bool
	}{
		{"https://github.com/user/repo.git", true},
		{"ssh://git@github.com/user/repo.git", true},
		{"git@github.com:user/repo.git", true},
		{"git://github.com/user/repo.git", false},  // insecure
		{"http://github.com/user/repo.git", false}, // insecure
		{"ext::sh -c 'evil'", false},               // command injection
		{"-u flag injection", false},               // argument injection
		{"", false},
		{"not-a-url", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := isValidGitURL(tt.url)
			if got != tt.valid {
				t.Errorf("isValidGitURL(%q) = %v, want %v", tt.url, got, tt.valid)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helper function tests
// ---------------------------------------------------------------------------

func TestCollectRelativeFiles(t *testing.T) {
	tmp := t.TempDir()
	// Create nested structure
	os.MkdirAll(filepath.Join(tmp, "sub"), 0o755)
	os.WriteFile(filepath.Join(tmp, "a.md"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(tmp, "sub", "b.md"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(tmp, ".syllago.yaml"), []byte("meta"), 0o644)
	// Create symlink (should be skipped)
	os.Symlink(filepath.Join(tmp, "a.md"), filepath.Join(tmp, "link.md"))

	files := collectRelativeFiles(tmp)

	// Should contain a.md and sub/b.md, sorted
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(files), files)
	}
	if files[0] != "a.md" {
		t.Errorf("expected files[0]='a.md', got %q", files[0])
	}
	if files[1] != filepath.Join("sub", "b.md") {
		t.Errorf("expected files[1]='sub/b.md', got %q", files[1])
	}
}

func TestBuildConflictInfoDirectories(t *testing.T) {
	existing := t.TempDir()
	source := t.TempDir()

	// A only in existing, B in both, C only in new
	os.WriteFile(filepath.Join(existing, "a.md"), []byte("old-a"), 0o644)
	os.WriteFile(filepath.Join(existing, "b.md"), []byte("old-b"), 0o644)
	os.WriteFile(filepath.Join(source, "b.md"), []byte("new-b"), 0o644)
	os.WriteFile(filepath.Join(source, "c.md"), []byte("new-c"), 0o644)

	m := importModel{}
	ci := m.buildConflictInfo(existing, source, "test-item")

	if len(ci.onlyExisting) != 1 || ci.onlyExisting[0] != "a.md" {
		t.Errorf("expected onlyExisting=[a.md], got %v", ci.onlyExisting)
	}
	if len(ci.inBoth) != 1 || ci.inBoth[0] != "b.md" {
		t.Errorf("expected inBoth=[b.md], got %v", ci.inBoth)
	}
	if len(ci.onlyNew) != 1 || ci.onlyNew[0] != "c.md" {
		t.Errorf("expected onlyNew=[c.md], got %v", ci.onlyNew)
	}
}

func TestBuildConflictInfoSingleFile(t *testing.T) {
	existing := t.TempDir()
	os.WriteFile(filepath.Join(existing, "tool.sh"), []byte("old"), 0o644)

	// Source is a single file (universal type wrapping)
	sourceFile := filepath.Join(t.TempDir(), "tool.sh")
	os.WriteFile(sourceFile, []byte("new"), 0o644)

	m := importModel{}
	ci := m.buildConflictInfo(existing, sourceFile, "tool")

	// tool.sh exists in both
	if len(ci.inBoth) != 1 || ci.inBoth[0] != "tool.sh" {
		t.Errorf("expected inBoth=[tool.sh], got %v", ci.inBoth)
	}
}

func TestBuildConflictInfoCreateFlow(t *testing.T) {
	existing := t.TempDir()
	os.WriteFile(filepath.Join(existing, "SKILL.md"), []byte("content"), 0o644)
	os.WriteFile(filepath.Join(existing, "README.md"), []byte("readme"), 0o644)

	m := importModel{isCreate: true}
	ci := m.buildConflictInfo(existing, "", "test-item")

	// All existing files should be in onlyExisting
	if len(ci.onlyExisting) != 2 {
		t.Errorf("expected 2 onlyExisting, got %d: %v", len(ci.onlyExisting), ci.onlyExisting)
	}
	if len(ci.onlyNew) != 0 {
		t.Errorf("expected empty onlyNew, got %v", ci.onlyNew)
	}
	if len(ci.inBoth) != 0 {
		t.Errorf("expected empty inBoth, got %v", ci.inBoth)
	}
}

// ---------------------------------------------------------------------------
// Single import conflict tests
// ---------------------------------------------------------------------------

func TestConflictDetectionOnConfirm(t *testing.T) {
	// Override global content dir so destination path is predictable in tests
	globalDir := t.TempDir()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })

	app := navigateToImport(t)
	app.importer.step = stepConfirm
	app.importer.contentType = catalog.Skills
	app.importer.isCreate = false

	// Create source
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("content"), 0o644)
	app.importer.sourcePath = srcDir
	app.importer.itemName = "conflict-test"

	// Create the destination in the global library so it conflicts
	dest := filepath.Join(globalDir, "skills", "conflict-test")
	os.MkdirAll(dest, 0o755)
	os.WriteFile(filepath.Join(dest, "SKILL.md"), []byte("existing"), 0o644)

	m, _ := app.Update(keyEnter)
	app = m.(App)

	if app.importer.step != stepConflict {
		t.Fatalf("expected stepConflict, got %d", app.importer.step)
	}
}

func TestNoConflictPassesThrough(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConfirm
	app.importer.contentType = catalog.Skills
	app.importer.isCreate = false
	app.importer.sourcePath = app.catalog.Items[0].Path
	app.importer.itemName = "no-conflict-unique-name"

	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected import command (no conflict), got nil")
	}
}

func TestConflictOverwriteSingle(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.contentType = catalog.Skills
	app.importer.isCreate = false

	// Set up source and destination
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "SKILL.md"), []byte("new"), 0o644)
	app.importer.sourcePath = srcDir
	app.importer.itemName = "overwrite-test"

	dest := filepath.Join(app.catalog.RepoRoot, "local", "skills", "overwrite-test")
	os.MkdirAll(dest, 0o755)
	os.WriteFile(filepath.Join(dest, "SKILL.md"), []byte("old"), 0o644)

	app.importer.conflict = conflictInfo{
		existingPath: dest,
		sourcePath:   srcDir,
		itemName:     "overwrite-test",
		inBoth:       []string{"SKILL.md"},
	}

	// Press 'y' to overwrite
	_, cmd := app.Update(keyRune('y'))
	if cmd == nil {
		t.Fatal("expected overwrite import command, got nil")
	}
}

func TestConflictCancelSingle(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.conflict = conflictInfo{
		existingPath: "/fake/dest",
		sourcePath:   "/fake/src",
		itemName:     "cancel-test",
	}
	// No batch conflicts (single mode)
	app.importer.batchConflicts = nil

	m, _ := app.Update(keyEsc)
	app = m.(App)

	if app.importer.step != stepConfirm {
		t.Fatalf("expected stepConfirm after esc, got %d", app.importer.step)
	}
}

func TestConflictScrolling(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.conflict = conflictInfo{
		itemName: "conflict-item",
		diffText: "--- a/file.md\n+++ b/file.md\n@@ -1,1 +1,1 @@\n-old\n+new",
	}

	// Scroll down
	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.importer.conflict.scrollOffset != 1 {
		t.Fatalf("expected scrollOffset 1, got %d", app.importer.conflict.scrollOffset)
	}

	// Scroll back up
	m, _ = app.Update(keyUp)
	app = m.(App)
	if app.importer.conflict.scrollOffset != 0 {
		t.Fatalf("expected scrollOffset 0, got %d", app.importer.conflict.scrollOffset)
	}

	// Clamp at top
	m, _ = app.Update(keyUp)
	app = m.(App)
	if app.importer.conflict.scrollOffset != 0 {
		t.Fatal("scrollOffset should clamp at 0")
	}
}

// ---------------------------------------------------------------------------
// Batch conflict tests
// ---------------------------------------------------------------------------

func TestBatchConflictDetection(t *testing.T) {
	// Override global content dir so destination paths are predictable in tests
	globalDir := t.TempDir()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })

	app := navigateToImport(t)
	app.importer.step = stepValidate
	app.importer.contentType = catalog.Skills

	// Create 3 source items
	src1 := filepath.Join(t.TempDir(), "item-1")
	src2 := filepath.Join(t.TempDir(), "item-2")
	src3 := filepath.Join(t.TempDir(), "item-3")
	for _, d := range []string{src1, src2, src3} {
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("content"), 0o644)
	}

	app.importer.validationItems = []validationItem{
		{path: src1, name: "item-1", included: true},
		{path: src2, name: "item-2", included: true},
		{path: src3, name: "item-3", included: true},
	}

	// Create destinations in the global library for item-1 and item-3 so they conflict
	for _, name := range []string{"item-1", "item-3"} {
		dest := filepath.Join(globalDir, "skills", name)
		os.MkdirAll(dest, 0o755)
		os.WriteFile(filepath.Join(dest, "SKILL.md"), []byte("existing"), 0o644)
	}

	m, _ := app.Update(keyEnter)
	app = m.(App)

	if app.importer.step != stepConflict {
		t.Fatalf("expected stepConflict, got %d", app.importer.step)
	}
	if len(app.importer.batchConflicts) != 2 {
		t.Fatalf("expected 2 batch conflicts, got %d", len(app.importer.batchConflicts))
	}
}

func TestBatchNoConflicts(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepValidate
	app.importer.contentType = catalog.Skills

	src1 := filepath.Join(t.TempDir(), "unique-1")
	src2 := filepath.Join(t.TempDir(), "unique-2")
	for _, d := range []string{src1, src2} {
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("content"), 0o644)
	}

	app.importer.validationItems = []validationItem{
		{path: src1, name: "unique-1", included: true},
		{path: src2, name: "unique-2", included: true},
	}

	_, cmd := app.Update(keyEnter)
	if cmd == nil {
		t.Fatal("expected batch import command (no conflicts)")
	}
}

func TestBatchConflictStepThrough(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.contentType = catalog.Skills

	src1 := filepath.Join(t.TempDir(), "batch-1")
	src2 := filepath.Join(t.TempDir(), "batch-2")
	for _, d := range []string{src1, src2} {
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("new"), 0o644)
	}

	dest1 := filepath.Join(app.catalog.RepoRoot, "local", "skills", "batch-1")
	dest2 := filepath.Join(app.catalog.RepoRoot, "local", "skills", "batch-2")
	for _, d := range []string{dest1, dest2} {
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("old"), 0o644)
	}

	app.importer.batchConflicts = []string{src1, src2}
	app.importer.batchConflictIdx = 0
	app.importer.batchOverwrite = make(map[string]bool)
	app.importer.selectedPaths = []string{src1, src2}
	app.importer.conflict = app.importer.buildConflictInfo(dest1, src1, "batch-1")

	// Press 'y' on first conflict → overwrite marked, advances
	m, _ := app.Update(keyRune('y'))
	app = m.(App)

	if !app.importer.batchOverwrite[src1] {
		t.Fatal("expected batch-1 marked for overwrite")
	}
	if app.importer.batchConflictIdx != 1 {
		t.Fatalf("expected batchConflictIdx=1, got %d", app.importer.batchConflictIdx)
	}

	// Press 'n' on second conflict → skip, triggers batch import
	_, cmd := app.Update(keyRune('n'))
	if cmd == nil {
		t.Fatal("expected batch import command after resolving all conflicts")
	}
	if app.importer.batchOverwrite[src2] {
		t.Fatal("batch-2 should not be marked for overwrite")
	}
}

func TestBatchConflictAllOverwrite(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.contentType = catalog.Skills

	src1 := filepath.Join(t.TempDir(), "all-ow-1")
	src2 := filepath.Join(t.TempDir(), "all-ow-2")
	for _, d := range []string{src1, src2} {
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "SKILL.md"), []byte("new"), 0o644)
	}

	dest1 := filepath.Join(app.catalog.RepoRoot, "local", "skills", "all-ow-1")
	dest2 := filepath.Join(app.catalog.RepoRoot, "local", "skills", "all-ow-2")
	for _, d := range []string{dest1, dest2} {
		os.MkdirAll(d, 0o755)
	}

	app.importer.batchConflicts = []string{src1, src2}
	app.importer.batchConflictIdx = 0
	app.importer.batchOverwrite = make(map[string]bool)
	app.importer.selectedPaths = []string{src1, src2}
	app.importer.conflict = app.importer.buildConflictInfo(dest1, src1, "all-ow-1")

	// y, y
	m, _ := app.Update(keyRune('y'))
	app = m.(App)
	_, cmd := app.Update(keyRune('y'))

	if !app.importer.batchOverwrite[src1] || !app.importer.batchOverwrite[src2] {
		t.Fatal("expected both paths marked for overwrite")
	}
	if cmd == nil {
		t.Fatal("expected batch import command")
	}
}

func TestBatchConflictAllSkip(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.contentType = catalog.Skills

	src1 := filepath.Join(t.TempDir(), "skip-1")
	src2 := filepath.Join(t.TempDir(), "skip-2")
	for _, d := range []string{src1, src2} {
		os.MkdirAll(d, 0o755)
	}

	dest1 := filepath.Join(app.catalog.RepoRoot, "local", "skills", "skip-1")
	dest2 := filepath.Join(app.catalog.RepoRoot, "local", "skills", "skip-2")
	for _, d := range []string{dest1, dest2} {
		os.MkdirAll(d, 0o755)
	}

	app.importer.batchConflicts = []string{src1, src2}
	app.importer.batchConflictIdx = 0
	app.importer.batchOverwrite = make(map[string]bool)
	app.importer.selectedPaths = []string{src1, src2}
	app.importer.conflict = app.importer.buildConflictInfo(dest1, src1, "skip-1")

	// n, n (Esc also works for skip)
	m, _ := app.Update(keyRune('n'))
	app = m.(App)
	app.Update(keyEsc) // Esc also skips in batch

	if len(app.importer.batchOverwrite) != 0 {
		t.Fatalf("expected empty batchOverwrite, got %v", app.importer.batchOverwrite)
	}
}

// ---------------------------------------------------------------------------
// View rendering tests
// ---------------------------------------------------------------------------

func TestConflictViewDiff(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.conflict = conflictInfo{
		existingPath: "/dest/test",
		sourcePath:   "/src/test",
		itemName:     "test",
		onlyExisting: []string{"old-file.md"},
		onlyNew:      []string{"new-file.md"},
		inBoth:       []string{"shared.md"},
		diffText:     "--- a/old-file.md\n+++ /dev/null\n@@ -1,1 +0,0 @@\n-old content\n\n--- /dev/null\n+++ b/new-file.md\n@@ -0,0 +1,1 @@\n+new content\n\n--- a/shared.md\n+++ b/shared.md\n@@ -1,1 +1,1 @@\n-old shared\n+new shared",
	}

	view := app.importer.viewConflict()

	assertContains(t, view, "Destination already exists")
	assertContains(t, view, "old-file.md")
	assertContains(t, view, "new-file.md")
	assertContains(t, view, "shared.md")
	assertContains(t, view, "-old content")
	assertContains(t, view, "+new content")
	assertContains(t, view, "1 removed")
	assertContains(t, view, "1 added")
	assertContains(t, view, "1 modified")
	// help text is now in footer via helpText(), not inline in the view
	helpText := app.importer.helpText()
	assertContains(t, helpText, "y overwrite")
}

func TestConflictViewBatchHeader(t *testing.T) {
	app := navigateToImport(t)
	app.importer.step = stepConflict
	app.importer.batchConflicts = []string{"/a", "/b", "/c"}
	app.importer.batchConflictIdx = 1
	app.importer.conflict = conflictInfo{
		existingPath: "/dest",
		sourcePath:   "/src",
	}

	label := app.importer.stepLabel()
	if !strings.Contains(label, "2 of 3") {
		t.Fatalf("expected step label to contain '2 of 3', got %q", label)
	}
}

func TestConflictDiffComputation(t *testing.T) {
	existing := t.TempDir()
	source := t.TempDir()

	os.WriteFile(filepath.Join(existing, "keep.md"), []byte("line one.\nline two."), 0o644)
	os.WriteFile(filepath.Join(source, "keep.md"), []byte("line one\nline two."), 0o644) // removed period

	m := importModel{}
	ci := m.buildConflictInfo(existing, source, "diff-test")

	if ci.diffText == "" {
		t.Fatal("expected non-empty diffText for files with differences")
	}
	if !strings.Contains(ci.diffText, "-line one.") {
		t.Error("diffText should contain removed line with period")
	}
	if !strings.Contains(ci.diffText, "+line one") {
		t.Error("diffText should contain added line without period")
	}
	assertContains(t, ci.diffText, "--- a/keep.md")
	assertContains(t, ci.diffText, "+++ b/keep.md")
}

// ---------------------------------------------------------------------------
// Pre-filtered import flows (Bug fixes: provider selection & discovery)
// ---------------------------------------------------------------------------

// navigateToFilteredImport creates a test app and opens the import screen
// pre-filtered to a specific content type (simulating 'a' from Items list).
func navigateToFilteredImport(t *testing.T, ct catalog.ContentType) App {
	t.Helper()
	app := testApp(t)
	app.importer = newImportModelWithFilter(app.providers, app.catalog.RepoRoot, app.projectRoot, ct, "")
	app.importer.width = app.width - sidebarWidth - 1
	app.importer.height = app.panelHeight()
	app.screen = screenImport
	app.focus = focusContent
	return app
}

func TestPreFilteredCreateNewUniversalGoesToName(t *testing.T) {
	app := navigateToFilteredImport(t, catalog.Skills)

	// Source step: select Create New (cursor 3)
	app = pressN(app, keyDown, 3)
	m, _ := app.Update(keyEnter)
	app = m.(App)

	// Universal type should skip provider, go straight to name
	if app.importer.step != stepName {
		t.Fatalf("expected stepName for universal Create New, got %d", app.importer.step)
	}
}

func TestPreFilteredCreateNewProviderSpecificGoesToProvider(t *testing.T) {
	for _, ct := range []catalog.ContentType{catalog.Rules, catalog.Hooks, catalog.Commands} {
		t.Run(string(ct), func(t *testing.T) {
			app := navigateToFilteredImport(t, ct)

			// Source step: select Create New (cursor 3)
			app = pressN(app, keyDown, 3)
			m, _ := app.Update(keyEnter)
			app = m.(App)

			// Provider-specific type should go to provider step (or error if no providers)
			if app.importer.step == stepProvider {
				// Good — found providers
				if len(app.importer.providerNames) == 0 {
					t.Fatal("providerNames should be populated when on stepProvider")
				}
			} else if app.importer.step == stepSource && app.importer.message != "" {
				// Also acceptable — no providers available, returned to source with error
			} else {
				t.Fatalf("expected stepProvider or error, got step %d (msg: %q)", app.importer.step, app.importer.message)
			}
		})
	}
}

func TestPreFilteredLocalPathProviderSpecificGoesToProvider(t *testing.T) {
	for _, ct := range []catalog.ContentType{catalog.Rules, catalog.Hooks, catalog.Commands} {
		t.Run(string(ct), func(t *testing.T) {
			app := navigateToFilteredImport(t, ct)

			// Source step: select Local Path (cursor 1)
			app = pressN(app, keyDown, 1)
			m, _ := app.Update(keyEnter)
			app = m.(App)

			// Provider-specific type should go to provider step (or error if no providers)
			if app.importer.step == stepProvider {
				if len(app.importer.providerNames) == 0 {
					t.Fatal("providerNames should be populated when on stepProvider")
				}
			} else if app.importer.step == stepSource && app.importer.message != "" {
				// No providers available
			} else {
				t.Fatalf("expected stepProvider or error, got step %d (msg: %q)", app.importer.step, app.importer.message)
			}
		})
	}
}

func TestPreFilteredLocalPathUniversalSkipsProvider(t *testing.T) {
	app := navigateToFilteredImport(t, catalog.Skills)

	// Source step: select Local Path (cursor 1)
	app = pressN(app, keyDown, 1)
	m, _ := app.Update(keyEnter)
	app = m.(App)

	// Universal type should skip provider, go to browse start
	if app.importer.step != stepBrowseStart {
		t.Fatalf("expected stepBrowseStart for universal Local Path, got %d", app.importer.step)
	}
}

func TestPreFilteredCreateProviderThenName(t *testing.T) {
	app := navigateToFilteredImport(t, catalog.Commands)

	// Navigate: Create New → provider step
	app = pressN(app, keyDown, 3)
	m, _ := app.Update(keyEnter)
	app = m.(App)

	if app.importer.step != stepProvider {
		t.Skip("no providers available, skipping provider→name test")
	}

	// Select provider → should go to name
	m, _ = app.Update(keyEnter)
	app = m.(App)
	if app.importer.step != stepName {
		t.Fatalf("expected stepName after provider selection, got %d", app.importer.step)
	}
	if app.importer.providerName == "" {
		t.Fatal("expected providerName to be set")
	}
}

func TestDiscoveryFilteredMatchesSelected(t *testing.T) {
	app := navigateToFilteredImport(t, catalog.Skills)

	// Set up the import model to be on provider pick step
	app.importer.step = stepProviderPick

	// Simulate discoveryDoneMsg with mixed types
	msg := discoveryDoneMsg{
		items: []add.DiscoveryItem{
			{Name: "skill-one", Type: catalog.Skills, Status: add.StatusNew},
			{Name: "rule-one", Type: catalog.Rules, Status: add.StatusNew},
			{Name: "skill-two", Type: catalog.Skills, Status: add.StatusInLibrary},
			{Name: "rule-two", Type: catalog.Rules, Status: add.StatusOutdated},
		},
	}
	m, _ := app.Update(msg)
	app = m.(App)

	// Should filter to only Skills (2 items)
	if len(app.importer.discoveryItems) != 2 {
		t.Fatalf("expected 2 filtered discovery items, got %d", len(app.importer.discoveryItems))
	}
	// discoverySelected should match filtered length
	if len(app.importer.discoverySelected) != 2 {
		t.Fatalf("expected discoverySelected length 2, got %d", len(app.importer.discoverySelected))
	}
	// First item (skill-one, StatusNew) should be pre-selected
	if !app.importer.discoverySelected[0] {
		t.Fatal("expected skill-one (StatusNew) to be pre-selected")
	}
	// Second item (skill-two, StatusCurrent) should NOT be pre-selected
	if app.importer.discoverySelected[1] {
		t.Fatal("expected skill-two (StatusCurrent) to NOT be pre-selected")
	}
}

func TestDiscoveryUnfilteredKeepsAll(t *testing.T) {
	// No pre-filter — discovery should keep all items
	app := navigateToImport(t)
	app.importer.step = stepProviderPick

	msg := discoveryDoneMsg{
		items: []add.DiscoveryItem{
			{Name: "skill-one", Type: catalog.Skills, Status: add.StatusNew},
			{Name: "rule-one", Type: catalog.Rules, Status: add.StatusNew},
		},
	}
	m, _ := app.Update(msg)
	app = m.(App)

	if len(app.importer.discoveryItems) != 2 {
		t.Fatalf("expected 2 unfiltered discovery items, got %d", len(app.importer.discoveryItems))
	}
	if len(app.importer.discoverySelected) != 2 {
		t.Fatalf("expected discoverySelected length 2, got %d", len(app.importer.discoverySelected))
	}
}

// ---------------------------------------------------------------------------
// hasTextInput tests
// ---------------------------------------------------------------------------

func TestHasTextInput(t *testing.T) {
	textSteps := []importStep{stepGitURL, stepPath, stepName}
	nonTextSteps := []importStep{
		stepSource, stepType, stepProvider, stepBrowseStart,
		stepBrowse, stepValidate, stepGitPick, stepConfirm,
		stepConflict, stepHookSelect, stepProviderPick, stepDiscoverySelect,
	}

	for _, step := range textSteps {
		m := importModel{step: step}
		if !m.hasTextInput() {
			t.Errorf("hasTextInput() = false for step %d, want true", step)
		}
	}
	for _, step := range nonTextSteps {
		m := importModel{step: step}
		if m.hasTextInput() {
			t.Errorf("hasTextInput() = true for step %d, want false", step)
		}
	}
}

// ---------------------------------------------------------------------------
// viewValidate tests
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// View() step rendering tests
// ---------------------------------------------------------------------------

func TestImportView_StepSource(t *testing.T) {
	m := importModel{step: stepSource, sourceCursor: 0, width: 80, height: 30}
	got := stripANSI(m.View())
	if !strings.Contains(got, "From Provider") {
		t.Error("stepSource view should contain 'From Provider'")
	}
	if !strings.Contains(got, "Git URL") {
		t.Error("stepSource view should contain 'Git URL'")
	}
	if !strings.Contains(got, "Create New") {
		t.Error("stepSource view should contain 'Create New'")
	}
}

func TestImportView_StepType(t *testing.T) {
	m := importModel{
		step:       stepType,
		types:      []catalog.ContentType{catalog.Rules, catalog.Skills},
		typeCursor: 0,
		width:      80,
		height:     30,
	}
	got := stripANSI(m.View())
	if !strings.Contains(got, "content type") {
		t.Error("stepType view should contain 'content type'")
	}
}

func TestImportView_StepProvider(t *testing.T) {
	m := importModel{
		step:          stepProvider,
		contentType:   catalog.Rules,
		providerNames: []string{"Claude Code", "Gemini CLI"},
		provCursor:    0,
		width:         80,
		height:        30,
	}
	got := stripANSI(m.View())
	if !strings.Contains(got, "Claude Code") {
		t.Error("stepProvider view should contain 'Claude Code'")
	}
}

func TestImportView_StepBrowseStart(t *testing.T) {
	m := importModel{
		step:         stepBrowseStart,
		browseCursor: 0,
		width:        80,
		height:       30,
	}
	got := stripANSI(m.View())
	if !strings.Contains(got, "browse") {
		t.Error("stepBrowseStart view should contain 'browse'")
	}
	if !strings.Contains(got, "Current directory") {
		t.Error("stepBrowseStart view should contain 'Current directory'")
	}
}

func TestImportView_StepPath(t *testing.T) {
	m := importModel{step: stepPath, width: 80, height: 30}
	got := stripANSI(m.View())
	if !strings.Contains(got, "starting path") {
		t.Error("stepPath view should contain 'starting path'")
	}
}

func TestImportView_StepConfirmCreate(t *testing.T) {
	m := importModel{
		step:        stepConfirm,
		contentType: catalog.Skills,
		itemName:    "my-skill",
		isCreate:    true,
		width:       80,
		height:      30,
	}
	got := stripANSI(m.View())
	if !strings.Contains(got, "creation") {
		t.Error("stepConfirm create view should contain 'creation'")
	}
	if !strings.Contains(got, "my-skill") {
		t.Error("stepConfirm create view should show item name")
	}
}

func TestImportView_StepGitURL(t *testing.T) {
	m := importModel{step: stepGitURL, width: 80, height: 30}
	got := stripANSI(m.View())
	if !strings.Contains(got, "git repository URL") {
		t.Error("stepGitURL view should contain 'git repository URL'")
	}
}

func TestImportView_StepGitPick(t *testing.T) {
	m := importModel{
		step:       stepGitPick,
		pickCursor: 0,
		clonedItems: []catalog.ContentItem{
			{Name: "item-a", Type: catalog.Rules},
			{Name: "item-b", Type: catalog.Skills},
		},
		width:  80,
		height: 30,
	}
	got := stripANSI(m.View())
	if !strings.Contains(got, "item-a") {
		t.Error("stepGitPick view should show items")
	}
}

func TestImportView_StepName(t *testing.T) {
	m := importModel{
		step:        stepName,
		contentType: catalog.Skills,
		width:       80,
		height:      30,
	}
	got := stripANSI(m.View())
	if !strings.Contains(got, "name") || !strings.Contains(got, "Skill") {
		t.Error("stepName view should contain item name prompt with content type")
	}
}

func TestImportView_StepConfirm(t *testing.T) {
	m := importModel{
		step:        stepConfirm,
		contentType: catalog.Rules,
		itemName:    "my-rule",
		sourcePath:  "/tmp/source",
		width:       80,
		height:      30,
	}
	got := stripANSI(m.View())
	if !strings.Contains(got, "my-rule") {
		t.Error("stepConfirm view should show item name")
	}
}

func TestImportView_StepHookSelect(t *testing.T) {
	m := importModel{
		step:             stepHookSelect,
		hookCandidates:   []converter.HookData{{Event: "before_tool_execute"}, {Event: "after_tool_execute"}},
		hookNames:        []string{"pre-commit", "post-build"},
		hookSelected:     []bool{true, false},
		hookSelectCursor: 0,
		width:            80,
		height:           30,
	}
	got := stripANSI(m.View())
	if !strings.Contains(got, "pre-commit") {
		t.Error("stepHookSelect view should show hook names")
	}
}

func TestImportView_StepProviderPick(t *testing.T) {
	m := importModel{
		step: stepProviderPick,
		providers: []provider.Provider{
			{Name: "Claude Code", Slug: "claude-code"},
			{Name: "Gemini CLI", Slug: "gemini-cli"},
		},
		discoveryProvCursor: 0,
		width:               80,
		height:              30,
	}
	got := stripANSI(m.View())
	if !strings.Contains(got, "Claude Code") {
		t.Error("stepProviderPick view should show provider names")
	}
}

func TestImportView_StepConflict(t *testing.T) {
	m := importModel{
		step: stepConflict,
		conflict: conflictInfo{
			existingPath: "/tmp/rules/my-rule",
			onlyNew:      []string{"new.md"},
			diffText:     "+new content\n-old content",
		},
		width:  80,
		height: 30,
	}
	got := stripANSI(m.View())
	if !strings.Contains(got, "already exists") {
		t.Error("stepConflict view should show conflict warning")
	}
}

func TestImportView_StepValidate(t *testing.T) {
	m := importModel{
		step: stepValidate,
		validationItems: []validationItem{
			{name: "my-rule", detection: "Rule", included: true},
		},
		width:  80,
		height: 30,
	}
	got := stripANSI(m.View())
	if !strings.Contains(got, "my-rule") {
		t.Error("stepValidate view should show items")
	}
}

func TestImportView_StepDiscoverySelect(t *testing.T) {
	m := importModel{
		step: stepDiscoverySelect,
		discoveryItems: []add.DiscoveryItem{
			{Name: "discovered-rule", Type: catalog.Rules},
		},
		discoverySelected: []bool{true},
		discoveryCursor:   0,
		discoveryProvider: provider.Provider{Name: "Claude Code"},
		width:             80,
		height:            30,
	}
	got := stripANSI(m.View())
	if !strings.Contains(got, "discovered-rule") {
		t.Error("stepDiscoverySelect view should show discovered items")
	}
}

// ---------------------------------------------------------------------------
// viewConflict and renderDiffLine tests
// ---------------------------------------------------------------------------

func TestViewConflict_NoDiff(t *testing.T) {
	m := importModel{
		step: stepConflict,
		conflict: conflictInfo{
			existingPath: "/home/user/rules/my-rule",
		},
		width:  80,
		height: 30,
	}
	got := stripANSI(m.viewConflict())
	if !strings.Contains(got, "already exists") {
		t.Error("should contain 'already exists'")
	}
	if !strings.Contains(got, "no differences") {
		t.Error("should show 'no differences' when diffText is empty")
	}
}

func TestViewConflict_WithDiff(t *testing.T) {
	m := importModel{
		step: stepConflict,
		conflict: conflictInfo{
			existingPath: "/home/user/rules/my-rule",
			onlyExisting: []string{"old.md"},
			onlyNew:      []string{"new.md"},
			inBoth:       []string{"shared.md"},
			diffText:     "--- a/shared.md\n+++ b/shared.md\n@@ -1,3 +1,3 @@\n-old line\n+new line\n same line",
		},
		width:  80,
		height: 30,
	}
	got := stripANSI(m.viewConflict())
	if !strings.Contains(got, "1 removed") {
		t.Error("should show removed count")
	}
	if !strings.Contains(got, "1 added") {
		t.Error("should show added count")
	}
	if !strings.Contains(got, "1 modified") {
		t.Error("should show modified count")
	}
	if !strings.Contains(got, "old line") {
		t.Error("should show diff content")
	}
}

func TestRenderDiffLine(t *testing.T) {
	// Basic prefix coloring
	got := renderDiffLine("+added line", 0)
	if !strings.Contains(got, "added line") {
		t.Error("should contain the line text")
	}

	got = renderDiffLine("-removed line", 0)
	if !strings.Contains(got, "removed line") {
		t.Error("should contain the line text")
	}

	got = renderDiffLine("@@ -1,3 +1,3 @@", 0)
	if !strings.Contains(got, "@@") {
		t.Error("should contain hunk header")
	}

	got = renderDiffLine("--- a/file.txt", 0)
	if !strings.Contains(got, "file.txt") {
		t.Error("should contain file path")
	}

	// Horizontal offset
	got = renderDiffLine("+abcdefgh", 3)
	if !strings.Contains(got, "defgh") {
		t.Error("horizontal offset should skip first chars")
	}

	// Offset beyond line length — result should be empty or a styled empty string
	got = renderDiffLine("+abc", 10)
	_ = got // valid: may be empty or styled empty
}

// ---------------------------------------------------------------------------
// cwd / homeDir tests
// ---------------------------------------------------------------------------

func TestCwd(t *testing.T) {
	got := cwd()
	if got == "" || got == "(unknown)" {
		t.Error("cwd() should return a valid directory")
	}
}

func TestHomeDir(t *testing.T) {
	got := homeDir()
	if got == "" || got == "(unknown)" {
		t.Error("homeDir() should return a valid directory")
	}
}

func TestViewValidate_EmptyItems(t *testing.T) {
	m := importModel{
		step:            stepValidate,
		validationItems: nil,
	}
	got := stripANSI(m.viewValidate())
	if !strings.Contains(got, "0 of 0 items") {
		t.Error("empty validation should show '0 of 0 items'")
	}
}

// ---------------------------------------------------------------------------
// sourceLabel tests
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// stepLabel tests
// ---------------------------------------------------------------------------

func TestStepLabel(t *testing.T) {
	tests := []struct {
		step importStep
		want string
	}{
		{stepSource, "Step 1 of 4: Source"},
		{stepType, "Step 2 of 4: Content Type"},
		{stepProvider, "Step 2b of 4: Provider"},
		{stepBrowseStart, "Step 3 of 4: Browse"},
		{stepBrowse, "Step 3 of 4: Browse"},
		{stepPath, "Step 3 of 4: Browse"},
		{stepValidate, "Step 3b of 4: Review"},
		{stepGitURL, "Step 2 of 3: Repository URL"},
		{stepGitPick, "Step 3 of 3: Select Item"},
		{stepName, "Step 2 of 3: Name"},
		{stepConfirm, "Confirm"},
		{stepConflict, "Conflict"},
		{stepHookSelect, "Step 4 of 4: Select Hooks"},
		{stepProviderPick, "Select Provider"},
		{stepDiscoverySelect, "Select Items to Add"},
	}
	for _, tt := range tests {
		m := importModel{step: tt.step}
		got := m.stepLabel()
		if got != tt.want {
			t.Errorf("stepLabel() at step %d = %q, want %q", tt.step, got, tt.want)
		}
	}
}

func TestStepLabel_BatchConflict(t *testing.T) {
	m := importModel{
		step:             stepConflict,
		batchConflicts:   []string{"a", "b", "c"},
		batchConflictIdx: 1,
	}
	got := m.stepLabel()
	if got != "Conflict 2 of 3" {
		t.Errorf("stepLabel() = %q, want %q", got, "Conflict 2 of 3")
	}
}

// ---------------------------------------------------------------------------
// helpText tests
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// breadcrumb tests
// ---------------------------------------------------------------------------

func TestImportBreadcrumb(t *testing.T) {
	tests := []struct {
		step importStep
		want string // substring expected in breadcrumb
	}{
		{stepSource, "Add"},
		{stepType, "Content Type"},
		{stepProvider, "Provider"},
		{stepBrowseStart, "Browse"},
		{stepBrowse, "Browse"},
		{stepPath, "Browse"},
		{stepValidate, "Review"},
		{stepGitURL, "Repository URL"},
		{stepGitPick, "Select Item"},
		{stepName, "Name"},
		{stepConfirm, "Confirm"},
		{stepConflict, "Conflict"},
		{stepHookSelect, "Select Hooks"},
		{stepProviderPick, "Select Provider"},
		{stepDiscoverySelect, "Select Items"},
	}
	for _, tt := range tests {
		m := importModel{step: tt.step}
		got := stripANSI(m.breadcrumb())
		if !strings.Contains(got, tt.want) {
			t.Errorf("breadcrumb() at step %d = %q, should contain %q", tt.step, got, tt.want)
		}
		// All breadcrumbs should contain "Home"
		if !strings.Contains(got, "Home") {
			t.Errorf("breadcrumb() at step %d should contain 'Home'", tt.step)
		}
	}
}

func TestImportHelpText(t *testing.T) {
	tests := []struct {
		step importStep
		want string // substring expected in help text
	}{
		{stepSource, "navigate"},
		{stepType, "navigate"},
		{stepProvider, "navigate"},
		{stepBrowseStart, "navigate"},
		{stepGitPick, "navigate"},
		{stepPath, "open browser"},
		{stepGitURL, "clone"},
		{stepName, "confirm"},
		{stepValidate, "toggle"},
		{stepHookSelect, "toggle"},
		{stepProviderPick, "navigate"},
		{stepDiscoverySelect, "toggle"},
	}
	for _, tt := range tests {
		m := importModel{step: tt.step}
		got := m.helpText()
		if !strings.Contains(got, tt.want) {
			t.Errorf("helpText() at step %d = %q, should contain %q", tt.step, got, tt.want)
		}
	}
}

func TestImportHelpText_ConfirmCreate(t *testing.T) {
	m := importModel{step: stepConfirm, isCreate: true}
	got := m.helpText()
	if !strings.Contains(got, "create") {
		t.Errorf("helpText() for create confirm = %q, should contain 'create'", got)
	}
}

func TestImportHelpText_ConfirmAdd(t *testing.T) {
	m := importModel{step: stepConfirm, isCreate: false}
	got := m.helpText()
	if !strings.Contains(got, "add") {
		t.Errorf("helpText() for add confirm = %q, should contain 'add'", got)
	}
}

func TestImportHelpText_ConflictBatch(t *testing.T) {
	m := importModel{step: stepConflict, batchConflicts: []string{"a"}}
	got := m.helpText()
	if !strings.Contains(got, "skip") {
		t.Errorf("helpText() for batch conflict = %q, should contain 'skip'", got)
	}
}

func TestImportHelpText_ConflictSingle(t *testing.T) {
	m := importModel{step: stepConflict}
	got := m.helpText()
	if !strings.Contains(got, "cancel") {
		t.Errorf("helpText() for single conflict = %q, should contain 'cancel'", got)
	}
}

func TestSourceLabel(t *testing.T) {
	tests := []struct {
		cursor int
		want   string
	}{
		{0, "From Provider"},
		{1, "Local Path"},
		{2, "Git URL"},
		{3, "Create New"},
		{99, "Source"},
	}
	for _, tt := range tests {
		m := importModel{sourceCursor: tt.cursor}
		got := m.sourceLabel()
		if got != tt.want {
			t.Errorf("sourceLabel() with cursor=%d = %q, want %q", tt.cursor, got, tt.want)
		}
	}
}

func TestViewValidate_WithItems(t *testing.T) {
	m := importModel{
		step:           stepValidate,
		validateCursor: 0,
		validationItems: []validationItem{
			{name: "rule-alpha", detection: "Rule detected", included: true},
			{name: "bad-file", detection: "", isWarning: true, included: false},
			{name: "skill-beta", detection: "Skill detected", description: "A skill", included: true},
		},
	}
	got := stripANSI(m.viewValidate())

	// Cursor indicator on first item
	if !strings.Contains(got, ">") {
		t.Error("should contain cursor indicator '>'")
	}
	// Item names
	if !strings.Contains(got, "rule-alpha") {
		t.Error("should contain item name 'rule-alpha'")
	}
	if !strings.Contains(got, "bad-file") {
		t.Error("should contain item name 'bad-file'")
	}
	// Warning indicator
	if !strings.Contains(got, "No recognized content") {
		t.Error("warning item should show 'No recognized content'")
	}
	// Description
	if !strings.Contains(got, "A skill") {
		t.Error("should contain item description")
	}
	// Count
	if !strings.Contains(got, "2 of 3 items") {
		t.Error("should show '2 of 3 items will be added'")
	}
}
