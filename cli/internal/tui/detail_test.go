package tui

import (
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/catalog"
)

// ---------------------------------------------------------------------------
// Helper: navigate to detail for a specific item type
// ---------------------------------------------------------------------------

// navigateToDetail creates an app and navigates to the detail screen for the
// first item of the given content type. Returns the app positioned on detail.
func navigateToDetail(t *testing.T, ct catalog.ContentType) App {
	t.Helper()
	app := testApp(t)

	// Find the right category row for this type
	types := catalog.AllContentTypes()
	idx := -1
	for i, typ := range types {
		if typ == ct {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("content type %s not found in AllContentTypes", ct)
	}

	app = pressN(app, keyDown, idx)
	m, _ := app.Update(keyEnter) // → items
	app = m.(App)
	assertScreen(t, app, screenItems)

	if len(app.items.items) == 0 {
		t.Fatalf("no items found for type %s", ct)
	}

	m, _ = app.Update(keyEnter) // → detail
	app = m.(App)
	assertScreen(t, app, screenDetail)
	return app
}

// navigateToDetailItem creates an app and navigates to detail for a specific
// item name within a given content type.
func navigateToDetailItem(t *testing.T, ct catalog.ContentType, name string) App {
	t.Helper()
	app := testApp(t)

	types := catalog.AllContentTypes()
	idx := -1
	for i, typ := range types {
		if typ == ct {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("content type %s not found", ct)
	}

	app = pressN(app, keyDown, idx)
	m, _ := app.Update(keyEnter) // → items
	app = m.(App)

	// Find the item and navigate cursor to it
	for i, item := range app.items.items {
		if item.Name == name {
			app = pressN(app, keyDown, i)
			break
		}
	}

	m, _ = app.Update(keyEnter) // → detail
	app = m.(App)
	assertScreen(t, app, screenDetail)
	if app.detail.item.Name != name {
		t.Fatalf("expected detail for %q, got %q", name, app.detail.item.Name)
	}
	return app
}

// ---------------------------------------------------------------------------
// Tab switching
// ---------------------------------------------------------------------------

func TestDetailStatePreservedOnReenter(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)

	// Switch to Files tab
	m, _ := app.Update(keyRune('2'))
	app = m.(App)
	if app.detail.activeTab != tabFiles {
		t.Fatal("expected tabFiles")
	}

	// Navigate back
	m, _ = app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenItems)

	// Re-enter same item
	m, _ = app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenDetail)

	// Tab should be preserved
	if app.detail.activeTab != tabFiles {
		t.Fatalf("expected tabFiles preserved, got %d", app.detail.activeTab)
	}
}

func TestDetailStateClearedOnDifferentItem(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)

	m, _ := app.Update(keyRune('2')) // Files tab
	app = m.(App)

	// Back
	m, _ = app.Update(keyEsc)
	app = m.(App)

	if len(app.items.items) < 2 {
		t.Skip("need at least 2 items")
	}

	// Move to different item
	m, _ = app.Update(keyDown)
	app = m.(App)

	// Enter different item
	m, _ = app.Update(keyEnter)
	app = m.(App)

	// Should NOT preserve previous tab
	if app.detail.activeTab != tabOverview {
		t.Fatalf("expected tabOverview for new item, got %d", app.detail.activeTab)
	}
}

func TestDetailMessageClearsOnKeypress(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	app.detail.message = "test message"
	app.detail.messageIsErr = false

	// Any non-esc key should clear the message
	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.detail.message != "" {
		t.Fatal("expected message to be cleared on keypress")
	}
}

func TestDetailMessagePreservedDuringTextInput(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	app.detail.confirmAction = actionSavePath
	app.detail.message = "some error"
	app.detail.messageIsErr = true

	// During text input, message should NOT be cleared
	m, _ := app.Update(keyRune('a'))
	app = m.(App)
	if app.detail.message != "some error" {
		t.Fatal("message should be preserved during text input")
	}
}

func TestDetailTabCycle(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)

	if app.detail.activeTab != tabOverview {
		t.Fatalf("expected initial tab tabOverview, got %d", app.detail.activeTab)
	}

	// Tab cycles: Overview → Files → Install → Overview
	m, _ := app.Update(keyTab)
	app = m.(App)
	if app.detail.activeTab != tabFiles {
		t.Fatalf("expected tabFiles after 1 tab, got %d", app.detail.activeTab)
	}

	m, _ = app.Update(keyTab)
	app = m.(App)
	if app.detail.activeTab != tabInstall {
		t.Fatalf("expected tabInstall after 2 tabs, got %d", app.detail.activeTab)
	}

	m, _ = app.Update(keyTab)
	app = m.(App)
	if app.detail.activeTab != tabOverview {
		t.Fatalf("expected tabOverview after 3 tabs, got %d", app.detail.activeTab)
	}
}

func TestDetailTabShortcuts(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)

	// '2' → Files
	m, _ := app.Update(keyRune('2'))
	app = m.(App)
	if app.detail.activeTab != tabFiles {
		t.Fatalf("expected tabFiles from '2', got %d", app.detail.activeTab)
	}

	// '3' → Install
	m, _ = app.Update(keyRune('3'))
	app = m.(App)
	if app.detail.activeTab != tabInstall {
		t.Fatalf("expected tabInstall from '3', got %d", app.detail.activeTab)
	}

	// '1' → Overview
	m, _ = app.Update(keyRune('1'))
	app = m.(App)
	if app.detail.activeTab != tabOverview {
		t.Fatalf("expected tabOverview from '1', got %d", app.detail.activeTab)
	}
}

func TestDetailTabBlockedDuringAction(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	app.detail.confirmAction = actionUninstall

	// Tab should not switch when an action is active
	m, _ := app.Update(keyTab)
	app = m.(App)
	if app.detail.activeTab != tabOverview {
		t.Fatal("tab switching should be blocked during active action")
	}

	// Number shortcuts should also be blocked
	m, _ = app.Update(keyRune('2'))
	app = m.(App)
	if app.detail.activeTab != tabOverview {
		t.Fatal("tab shortcut should be blocked during active action")
	}
}

func TestDetailShiftTabReverseCycle(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)

	// Start at Overview (0)
	if app.detail.activeTab != tabOverview {
		t.Fatalf("expected tabOverview, got %d", app.detail.activeTab)
	}

	// Shift+Tab: Overview -> Install (wraps backward)
	m, _ := app.Update(keyShiftTab)
	app = m.(App)
	if app.detail.activeTab != tabInstall {
		t.Fatalf("expected tabInstall after shift+tab from Overview, got %d", app.detail.activeTab)
	}

	// Shift+Tab: Install -> Files
	m, _ = app.Update(keyShiftTab)
	app = m.(App)
	if app.detail.activeTab != tabFiles {
		t.Fatalf("expected tabFiles after shift+tab from Install, got %d", app.detail.activeTab)
	}

	// Shift+Tab: Files -> Overview
	m, _ = app.Update(keyShiftTab)
	app = m.(App)
	if app.detail.activeTab != tabOverview {
		t.Fatalf("expected tabOverview after shift+tab from Files, got %d", app.detail.activeTab)
	}
}

func TestDetailShiftTabBlockedDuringAction(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	app.detail.confirmAction = actionUninstall

	m, _ := app.Update(keyShiftTab)
	app = m.(App)
	if app.detail.activeTab != tabOverview {
		t.Fatal("shift+tab should be blocked during active action")
	}
}

func TestDetailTabBlockedDuringFileView(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	app.detail.viewingFile = true

	m, _ := app.Update(keyTab)
	app = m.(App)
	if app.detail.activeTab != tabOverview {
		t.Fatal("tab switching should be blocked while viewing file")
	}
}

// ---------------------------------------------------------------------------
// Overview tab
// ---------------------------------------------------------------------------

func TestDetailOverviewReadme(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	view := app.View()
	// README content should appear (glamour-rendered)
	assertContains(t, view, "Readme body")
}

func TestDetailOverviewNoReadme(t *testing.T) {
	app := navigateToDetail(t, catalog.Agents)
	// Agent has no ReadmeBody set
	view := app.View()
	// Should show fallback text or just no readme content
	// (exact text depends on render, but shouldn't crash)
	_ = view
}

func TestDetailOverviewPromptBody(t *testing.T) {
	app := navigateToDetail(t, catalog.Prompts)
	view := app.View()
	assertContains(t, view, "helpful assistant")
}

func TestDetailOverviewAppProviders(t *testing.T) {
	app := navigateToDetail(t, catalog.Apps)
	view := app.View()
	// App has SupportedProviders: ["claude-code", "cursor"]
	assertContains(t, view, "Claude Code")
}

func TestDetailOverviewMetadata(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	view := app.View()
	// Should show the item type and path
	assertContains(t, view, "skills")
}

func TestDetailOverviewLLMPrompt(t *testing.T) {
	// Navigate to the local skill which has LLM-PROMPT.md
	app := testApp(t)
	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes) // My Tools
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)

	if len(app.items.items) == 0 {
		t.Fatal("expected local items in My Tools")
	}

	m, _ = app.Update(keyEnter) // → detail of first local item
	app = m.(App)
	assertScreen(t, app, screenDetail)

	if app.detail.llmPrompt == "" {
		t.Fatal("expected LLM prompt to be loaded for local item")
	}
}

func TestDetailOverviewScroll(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	initialOffset := app.detail.scrollOffset

	// Scroll down
	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.detail.scrollOffset < initialOffset {
		t.Fatal("expected scroll offset to increase or stay on down")
	}

	// Scroll up
	app.detail.scrollOffset = 5
	m, _ = app.Update(keyUp)
	app = m.(App)
	if app.detail.scrollOffset != 4 {
		t.Fatalf("expected scroll offset 4, got %d", app.detail.scrollOffset)
	}
}

// ---------------------------------------------------------------------------
// Files tab
// ---------------------------------------------------------------------------

func TestDetailFilesNavigation(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	// Switch to files tab
	m, _ := app.Update(keyRune('2'))
	app = m.(App)

	nFiles := len(app.detail.item.Files)
	if nFiles < 2 {
		t.Skipf("need at least 2 files, got %d", nFiles)
	}

	// Navigate down
	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.detail.fileCursor != 1 {
		t.Fatalf("expected fileCursor 1, got %d", app.detail.fileCursor)
	}

	// Bounds clamping
	app = pressN(app, keyDown, nFiles+5)
	if app.detail.fileCursor != nFiles-1 {
		t.Fatalf("expected fileCursor clamped at %d, got %d", nFiles-1, app.detail.fileCursor)
	}
}

func TestDetailFilesEnterOpens(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('2')) // → Files tab
	app = m.(App)

	// Enter opens file viewer
	m, _ = app.Update(keyEnter)
	app = m.(App)

	if !app.detail.viewingFile {
		t.Fatal("expected viewingFile=true after enter on file")
	}
	if app.detail.fileContent == "" {
		t.Fatal("expected fileContent to be loaded")
	}
}

func TestDetailFilesViewerLineNumbers(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('2')) // → Files tab
	app = m.(App)
	m, _ = app.Update(keyEnter) // open file
	app = m.(App)

	view := app.View()
	// File viewer should show line numbers or file content
	assertContains(t, view, "SKILL.md")
}

func TestDetailFilesViewerScroll(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('2'))
	app = m.(App)
	m, _ = app.Update(keyEnter) // open file
	app = m.(App)

	// Scroll down in viewer
	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.detail.fileScrollOffset != 1 {
		t.Fatalf("expected fileScrollOffset 1, got %d", app.detail.fileScrollOffset)
	}
}

func TestDetailFilesViewerEsc(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('2'))
	app = m.(App)
	m, _ = app.Update(keyEnter) // open file
	app = m.(App)

	if !app.detail.viewingFile {
		t.Fatal("expected viewingFile=true")
	}

	// Esc closes file viewer (not back to items — handled by app level)
	m, _ = app.Update(keyEsc)
	app = m.(App)
	if app.detail.viewingFile {
		t.Fatal("expected viewingFile=false after esc")
	}
}

func TestDetailFilesEmpty(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	app.detail.item.Files = nil // clear files
	m, _ := app.Update(keyRune('2')) // → Files tab
	app = m.(App)

	view := app.View()
	// Should not crash with empty files
	_ = view
}

// ---------------------------------------------------------------------------
// Install tab (checkboxes)
// ---------------------------------------------------------------------------

func TestDetailInstallCheckboxNav(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('3')) // → Install tab
	app = m.(App)

	nChecks := len(app.detail.providerChecks)
	if nChecks < 1 {
		t.Skip("no provider checkboxes available")
	}

	if nChecks >= 2 {
		m, _ = app.Update(keyDown)
		app = m.(App)
		if app.detail.checkCursor != 1 {
			t.Fatalf("expected checkCursor 1, got %d", app.detail.checkCursor)
		}
	}
}

func TestDetailInstallCheckboxToggle(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('3')) // → Install tab
	app = m.(App)

	if len(app.detail.providerChecks) < 1 {
		t.Skip("no provider checkboxes")
	}

	initial := app.detail.providerChecks[0]

	// Space toggles
	m, _ = app.Update(keySpace)
	app = m.(App)
	if app.detail.providerChecks[0] == initial {
		t.Fatal("space should toggle checkbox")
	}

	// Enter also toggles
	m, _ = app.Update(keyEnter)
	app = m.(App)
	if app.detail.providerChecks[0] != initial {
		t.Fatal("enter should toggle checkbox back")
	}
}

func TestDetailInstallPreChecked(t *testing.T) {
	// If a provider is already installed, checkbox should be pre-checked.
	// This is handled in newDetailModel() via installer.CheckStatus.
	// Since our test providers have empty install dirs, nothing should be
	// pre-checked (no files exist). Just verify the array is initialized.
	app := navigateToDetail(t, catalog.Skills)
	if app.detail.providerChecks == nil {
		t.Fatal("providerChecks should be initialized")
	}
}

// ---------------------------------------------------------------------------
// Install tab (install flow)
// ---------------------------------------------------------------------------

func TestDetailInstallStart(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('3')) // → Install tab
	app = m.(App)

	// Press 'i' to start install
	m, _ = app.Update(keyRune('i'))
	app = m.(App)

	// Should enter method picker or complete install (depending on provider type)
	// Either confirmAction changes or message is set
	if app.detail.confirmAction == actionNone && app.detail.message == "" {
		t.Fatal("expected install to either set confirmAction or message")
	}
}

func TestDetailInstallMethodPicker(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('3'))
	app = m.(App)

	// Force method picker state
	app.detail.confirmAction = actionChooseMethod
	app.detail.methodCursor = 0

	// Navigate method picker
	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.detail.methodCursor != 1 {
		t.Fatalf("expected methodCursor 1, got %d", app.detail.methodCursor)
	}

	// Bounds clamping
	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.detail.methodCursor != 1 {
		t.Fatal("methodCursor should clamp at 1")
	}
}

func TestDetailInstallAlreadyInstalled(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('3'))
	app = m.(App)

	// Mark all checkboxes as checked (simulating already installed)
	for i := range app.detail.providerChecks {
		app.detail.providerChecks[i] = true
	}
	// Force startInstall path — but the providers have empty install dirs so
	// CheckStatus won't find them installed. Instead, test with no new installs.
	// Just verify pressing i doesn't crash
	m, _ = app.Update(keyRune('i'))
	app = m.(App)
	// Message should be set
	_ = app.detail.message
}

func TestDetailInstallNoProviders(t *testing.T) {
	app := testApp(t)
	// Create a detail model with no providers
	item := app.catalog.Items[0]
	app.detail = newDetailModel(item, nil, app.catalog.RepoRoot)
	app.detail.width = 80
	app.detail.height = 30
	app.screen = screenDetail
	app.detail.activeTab = tabInstall

	// Press 'i'
	m, _ := app.Update(keyRune('i'))
	app = m.(App)
	assertContains(t, app.detail.message, "No providers detected")
}

// ---------------------------------------------------------------------------
// Install tab (uninstall flow)
// ---------------------------------------------------------------------------

func TestDetailUninstallFlow(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('3'))
	app = m.(App)

	// First 'u' press → confirmation (or "not installed" message)
	m, _ = app.Update(keyRune('u'))
	app = m.(App)

	// Since nothing is installed, should get "Not installed" message
	if app.detail.confirmAction == actionUninstall {
		// Second 'u' confirms
		m, _ = app.Update(keyRune('u'))
		app = m.(App)
		if app.detail.confirmAction != actionNone {
			t.Fatal("expected actionNone after double-press u")
		}
	}
}

func TestDetailUninstallNotInstalled(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('3'))
	app = m.(App)

	m, _ = app.Update(keyRune('u'))
	app = m.(App)

	// No items are installed in test providers
	if app.detail.confirmAction == actionNone {
		assertContains(t, app.detail.message, "Not installed")
	}
}

func TestDetailUninstallEscCancels(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	app.detail.activeTab = tabInstall
	app.detail.confirmAction = actionUninstall

	// Esc should cancel the confirmation
	m, _ := app.Update(keyEsc)
	app = m.(App)
	if app.detail.confirmAction != actionNone {
		t.Fatal("esc should cancel uninstall confirmation")
	}
}

// ---------------------------------------------------------------------------
// Install tab (prompts)
// ---------------------------------------------------------------------------

func TestDetailPromptCopy(t *testing.T) {
	app := navigateToDetail(t, catalog.Prompts)

	// Press 'c' to copy prompt
	m, _ := app.Update(keyRune('c'))
	app = m.(App)

	// Should set a message (success or clipboard error)
	if app.detail.message == "" {
		t.Fatal("expected message after copy attempt")
	}
}

func TestDetailPromptSavePath(t *testing.T) {
	app := navigateToDetail(t, catalog.Prompts)
	m, _ := app.Update(keyRune('3')) // Install tab
	app = m.(App)

	// Press 's' to start save path flow
	m, _ = app.Update(keyRune('s'))
	app = m.(App)

	if app.detail.confirmAction != actionSavePath {
		t.Fatalf("expected actionSavePath, got %d", app.detail.confirmAction)
	}

	// Type a path and enter → method picker
	m, _ = app.Update(keyEnter)
	app = m.(App)
	if app.detail.confirmAction != actionSaveMethod {
		t.Fatalf("expected actionSaveMethod after enter, got %d", app.detail.confirmAction)
	}

	// Esc from save method → back to none
	m, _ = app.Update(keyEsc)
	app = m.(App)
	if app.detail.confirmAction != actionNone {
		t.Fatalf("expected actionNone after esc from save method, got %d", app.detail.confirmAction)
	}
}

// ---------------------------------------------------------------------------
// Install tab (promote)
// ---------------------------------------------------------------------------

func TestDetailPromoteLocal(t *testing.T) {
	// Navigate to local item
	app := testApp(t)
	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes) // My Tools
	m, _ := app.Update(keyEnter)
	app = m.(App)
	m, _ = app.Update(keyEnter) // → detail of local item
	app = m.(App)
	assertScreen(t, app, screenDetail)

	if !app.detail.item.Local {
		t.Fatal("expected local item")
	}

	// First 'p' → confirmation
	m, _ = app.Update(keyRune('p'))
	app = m.(App)
	if app.detail.confirmAction != actionPromoteConfirm {
		t.Fatalf("expected actionPromoteConfirm, got %d", app.detail.confirmAction)
	}

	// Second 'p' → executes promote (returns command)
	m, cmd := app.Update(keyRune('p'))
	app = m.(App)
	if cmd == nil {
		t.Fatal("expected promote command on double-press")
	}
}

func TestDetailPromoteNonLocal(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)

	if app.detail.item.Local {
		t.Skip("first skill is local, can't test non-local promote blocking")
	}

	// 'p' should do nothing for non-local items
	m, _ := app.Update(keyRune('p'))
	app = m.(App)
	if app.detail.confirmAction == actionPromoteConfirm {
		t.Fatal("promote should not activate for non-local items")
	}
}

func TestDetailPromoteEscCancels(t *testing.T) {
	app := testApp(t)
	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes) // My Tools
	m, _ := app.Update(keyEnter)
	app = m.(App)
	m, _ = app.Update(keyEnter) // → detail
	app = m.(App)
	assertScreen(t, app, screenDetail)

	m, _ = app.Update(keyRune('p')) // first press → confirm
	app = m.(App)

	m, _ = app.Update(keyEsc) // cancel
	app = m.(App)
	if app.detail.confirmAction != actionNone {
		t.Fatal("esc should cancel promote confirmation")
	}
}

// ---------------------------------------------------------------------------
// Back navigation
// ---------------------------------------------------------------------------

func TestDetailBackCancelsPendingAction(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	app.detail.confirmAction = actionChooseMethod

	// Esc should cancel the action, not navigate back
	m, _ := app.Update(keyEsc)
	app = m.(App)

	// Should still be on detail screen
	assertScreen(t, app, screenDetail)
	if app.detail.confirmAction != actionNone {
		t.Fatal("esc should cancel pending action first")
	}

	// Another esc should navigate back to items
	m, _ = app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenItems)
}
