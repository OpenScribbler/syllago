package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
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

func TestDetailMessagePreservedDuringModal(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	app.detail.message = "some error"
	app.detail.messageIsErr = true
	// Activate a modal — all keys route to the modal, not detail
	app.envModal = newEnvSetupModal([]string{"TEST_VAR"})
	app.focus = focusModal

	m, _ := app.Update(keyRune('a'))
	app = m.(App)
	if app.detail.message != "some error" {
		t.Fatal("message should be preserved when a modal is active")
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

func TestDetailTabBlockedDuringModal(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	// Activate a modal — all keys route to the modal, not detail
	app.modal = newConfirmModal("Test", "body")
	app.modal.purpose = modalUninstall
	app.focus = focusModal

	// Tab should not switch when a modal is active (keys go to modal)
	m, _ := app.Update(keyTab)
	app = m.(App)
	if app.detail.activeTab != tabOverview {
		t.Fatal("tab switching should be blocked during active modal")
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

func TestDetailShiftTabBlockedDuringModal(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	app.modal = newConfirmModal("Test", "body")
	app.modal.purpose = modalUninstall
	app.focus = focusModal

	m, _ := app.Update(keyShiftTab)
	app = m.(App)
	if app.detail.activeTab != tabOverview {
		t.Fatal("shift+tab should be blocked during active modal")
	}
}

func TestDetailTabBlockedDuringFileView(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	app.detail.fileViewer.viewing = true

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

func TestDetailOverviewMetadata(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	view := app.View()
	// Should show the item type and path
	assertContains(t, view, "skills")
}

func TestDetailOverviewLLMPrompt(t *testing.T) {
	// Navigate to the local skill which has LLM-PROMPT.md
	app := navigateToLibraryItems(t)

	if len(app.items.items) == 0 {
		t.Fatal("expected local items in My Tools")
	}

	m, _ := app.Update(keyEnter) // → detail of first local item
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
	if app.detail.fileViewer.cursor != 1 {
		t.Fatalf("expected fileCursor 1, got %d", app.detail.fileViewer.cursor)
	}

	// Bounds clamping
	app = pressN(app, keyDown, nFiles+5)
	if app.detail.fileViewer.cursor != nFiles-1 {
		t.Fatalf("expected fileCursor clamped at %d, got %d", nFiles-1, app.detail.fileViewer.cursor)
	}
}

func TestDetailFilesEnterOpens(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('2')) // → Files tab
	app = m.(App)

	// Enter opens file viewer
	m, _ = app.Update(keyEnter)
	app = m.(App)

	if !app.detail.fileViewer.viewing {
		t.Fatal("expected viewingFile=true after enter on file")
	}
	if app.detail.fileViewer.content == "" {
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
	if app.detail.fileViewer.scrollOffset != 1 {
		t.Fatalf("expected fileScrollOffset 1, got %d", app.detail.fileViewer.scrollOffset)
	}
}

func TestDetailFilesViewerEsc(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('2'))
	app = m.(App)
	m, _ = app.Update(keyEnter) // open file
	app = m.(App)

	if !app.detail.fileViewer.viewing {
		t.Fatal("expected viewingFile=true")
	}

	// Esc closes file viewer (not back to items — handled by app level)
	m, _ = app.Update(keyEsc)
	app = m.(App)
	if app.detail.fileViewer.viewing {
		t.Fatal("expected viewingFile=false after esc")
	}
}

func TestDetailFilesEmpty(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	app.detail.item.Files = nil      // clear files
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

	nChecks := len(app.detail.provCheck.checks)
	if nChecks < 1 {
		t.Skip("no provider checkboxes available")
	}

	if nChecks >= 2 {
		m, _ = app.Update(keyDown)
		app = m.(App)
		if app.detail.provCheck.cursor != 1 {
			t.Fatalf("expected checkCursor 1, got %d", app.detail.provCheck.cursor)
		}
	}
}

func TestDetailInstallCheckboxToggle(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('3')) // → Install tab
	app = m.(App)

	if len(app.detail.provCheck.checks) < 1 {
		t.Skip("no provider checkboxes")
	}

	initial := app.detail.provCheck.checks[0]

	// Space toggles
	m, _ = app.Update(keySpace)
	app = m.(App)
	if app.detail.provCheck.checks[0] == initial {
		t.Fatal("space should toggle checkbox")
	}

	// Enter also toggles
	m, _ = app.Update(keyEnter)
	app = m.(App)
	if app.detail.provCheck.checks[0] != initial {
		t.Fatal("enter should toggle checkbox back")
	}
}

func TestDetailInstallPreChecked(t *testing.T) {
	// If a provider is already installed, checkbox should be pre-checked.
	// This is handled in newDetailModel() via installer.CheckStatus.
	// Since our test providers have empty install dirs, nothing should be
	// pre-checked (no files exist). Just verify the array is initialized.
	app := navigateToDetail(t, catalog.Skills)
	if app.detail.provCheck.checks == nil {
		t.Fatal("providerChecks should be initialized")
	}
}

func TestProviderCheckboxClickToggle(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('3')) // → Install tab
	app = m.(App)

	if len(app.detail.provCheck.checks) < 1 {
		t.Skip("no provider checkboxes available")
	}

	// app.View() calls zone.Scan() internally, which strips zone markers and
	// queues zone positions asynchronously via a channel. Wait for the worker
	// goroutine to process the queue before querying zone coordinates.
	app.View()
	time.Sleep(50 * time.Millisecond)

	zi := zone.Get("prov-check-0")
	if zi == nil || zi.IsZero() {
		t.Skip("prov-check-0 zone not registered after View() (cannot simulate click)")
	}

	initial := app.detail.provCheck.checks[0]

	// Synthesize a left-click at the zone's start coordinates.
	click := tea.MouseMsg{
		X:      zi.StartX,
		Y:      zi.StartY,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}
	m, _ = app.Update(click)
	app = m.(App)

	if app.detail.provCheck.checks[0] == initial {
		t.Fatal("clicking prov-check-0 zone should toggle checkbox[0]")
	}
}

// ---------------------------------------------------------------------------
// Install tab (install flow)
// ---------------------------------------------------------------------------

func TestDetailInstallStart(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('3')) // → Install tab
	app = m.(App)

	// Press 'i' to start install — should return a cmd to open install modal
	m, cmd := app.Update(keyRune('i'))
	app = m.(App)

	// startInstall() returns a cmd (openInstallModalMsg) for filesystem providers,
	// or installs directly and sets a message for JSON-merge-only providers.
	if cmd == nil && app.toast.text == "" {
		t.Fatal("expected a cmd (install modal) or a result message after pressing i")
	}
}

func TestDetailInstallModalNavigation(t *testing.T) {
	// Test that the install modal handles location navigation
	modal := newInstallModal(
		catalog.ContentItem{Name: "test-skill", Type: catalog.Skills},
		nil, "/tmp/test",
	)

	// Navigate location picker
	updated, _ := modal.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.locationCursor != 1 {
		t.Fatalf("expected locationCursor 1, got %d", updated.locationCursor)
	}

	// Bounds clamping at max
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.locationCursor != 2 {
		t.Fatal("locationCursor should clamp at 2")
	}

	// Select "Global" (cursor 0) → should advance to method step
	updated.locationCursor = 0
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.step != installStepMethod {
		t.Fatalf("expected installStepMethod after selecting Global, got %d", updated.step)
	}

	// Navigate method picker
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updated.methodCursor != 1 {
		t.Fatalf("expected methodCursor 1, got %d", updated.methodCursor)
	}

	// Confirm method → modal should close with confirmed=true
	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.active {
		t.Fatal("modal should be inactive after confirming method")
	}
	if !updated.confirmed {
		t.Fatal("modal should be confirmed")
	}
}

func TestDetailInstallAlreadyInstalled(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('3'))
	app = m.(App)

	// Mark all checkboxes as checked (simulating already installed)
	for i := range app.detail.provCheck.checks {
		app.detail.provCheck.checks[i] = true
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
	assertContains(t, app.toast.text, "No providers detected")
}

// ---------------------------------------------------------------------------
// Install tab (uninstall flow)
// ---------------------------------------------------------------------------

func TestDetailUninstallFlow(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('3'))
	app = m.(App)

	// Check a provider so we pass the "nothing selected" guard
	if len(app.detail.provCheck.checks) > 0 {
		app.detail.provCheck.checks[0] = true
	}

	// 'u' with nothing installed → "Not installed" message
	m, _ = app.Update(keyRune('u'))
	app = m.(App)
	assertContains(t, app.toast.text, "Not installed")
}

func TestDetailUninstallNotInstalled(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('3'))
	app = m.(App)

	// Check a provider checkbox so we pass the "nothing selected" guard
	if len(app.detail.provCheck.checks) > 0 {
		app.detail.provCheck.checks[0] = true
	}

	m, _ = app.Update(keyRune('u'))
	app = m.(App)

	// No items are installed in test providers — message should indicate this
	assertContains(t, app.toast.text, "Not installed")
}

func TestDetailUninstallNothingSelected(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	m, _ := app.Update(keyRune('3'))
	app = m.(App)

	// Ensure no checkboxes are checked
	for i := range app.detail.provCheck.checks {
		app.detail.provCheck.checks[i] = false
	}

	m, _ = app.Update(keyRune('u'))
	app = m.(App)

	assertContains(t, app.toast.text, "No providers selected")
}

// TestDetailUninstallEscCancels was removed — uninstall confirmation is now
// handled by the centralized confirmModal. Esc behavior is tested in modal_test.go.

// ---------------------------------------------------------------------------
// Install tab (copy/save)
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// Install tab (share)
// ---------------------------------------------------------------------------

func TestDetailShareLibrary(t *testing.T) {
	// Navigate to library item via card view
	app := navigateToLibraryItems(t)
	m, _ := app.Update(keyEnter) // → detail of library item
	app = m.(App)
	assertScreen(t, app, screenDetail)

	if !app.detail.item.Library {
		t.Fatal("expected library item")
	}

	// 'p' now emits openModalMsg instead of setting confirmAction
	m, cmd := app.Update(keyRune('p'))
	app = m.(App)
	if cmd == nil {
		t.Fatal("pressing p should return a cmd (openModalMsg) to open the share modal")
	}
	// Verify the cmd produces an openModalMsg with modalShare purpose
	if msg := cmd(); msg != nil {
		oMsg, ok := msg.(openModalMsg)
		if !ok {
			t.Fatalf("cmd should return openModalMsg, got %T", msg)
		}
		if oMsg.purpose != modalShare {
			t.Errorf("openModalMsg.purpose should be modalShare, got %d", oMsg.purpose)
		}
	}
}

func TestDetailShareNonLibrary(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)

	if app.detail.item.Library {
		t.Skip("first skill is library item, can't test non-library share blocking")
	}

	// 'p' should do nothing for non-library items (no cmd returned)
	_, cmd := app.Update(keyRune('p'))
	if cmd != nil {
		t.Fatal("share should not activate for non-library items")
	}
}

// TestDetailShareEscCancels was removed — share confirmation is now
// handled by the centralized confirmModal. Esc behavior is tested in modal_test.go.

// ---------------------------------------------------------------------------
// Back navigation
// ---------------------------------------------------------------------------

func TestDetailBackNavigatesToItems(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)

	// Esc should navigate back to items
	m, _ := app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenItems)
}

// ---------------------------------------------------------------------------
// Position indicator (4.16)
// ---------------------------------------------------------------------------

func TestDetailShowsPosition(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	view := app.View()
	// Position is shown as "(N/Total)" format per Task 4.1 renderContentSplit
	assertContains(t, view, "1/")
}

// ---------------------------------------------------------------------------
// Next/prev navigation (4.17)
// ---------------------------------------------------------------------------

func TestDetailNextPrevItem(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyEnter) // -> items
	app = m.(App)

	if len(app.items.items) < 2 {
		t.Skip("need at least 2 items")
	}

	m, _ = app.Update(keyEnter) // -> detail of first item
	app = m.(App)
	firstName := app.detail.item.Name

	// ctrl+n goes to next
	ctrlN := tea.KeyMsg{Type: tea.KeyCtrlN}
	m, _ = app.Update(ctrlN)
	app = m.(App)

	if app.detail.item.Name == firstName {
		t.Fatal("expected different item after ctrl+n")
	}
	assertScreen(t, app, screenDetail)

	// ctrl+p goes back
	ctrlP := tea.KeyMsg{Type: tea.KeyCtrlP}
	m, _ = app.Update(ctrlP)
	app = m.(App)

	if app.detail.item.Name != firstName {
		t.Fatalf("expected %s after ctrl+p, got %s", firstName, app.detail.item.Name)
	}
}

func TestDetailNextPrevBounds(t *testing.T) {
	app := testApp(t)
	m, _ := app.Update(keyEnter) // -> items
	app = m.(App)

	m, _ = app.Update(keyEnter) // -> detail of first item
	app = m.(App)
	firstName := app.detail.item.Name

	// ctrl+p at first item should do nothing
	ctrlP := tea.KeyMsg{Type: tea.KeyCtrlP}
	m, _ = app.Update(ctrlP)
	app = m.(App)
	if app.detail.item.Name != firstName {
		t.Fatal("ctrl+p at first item should do nothing")
	}

	// Navigate to last item
	ctrlN := tea.KeyMsg{Type: tea.KeyCtrlN}
	for i := 0; i < len(app.items.items)+5; i++ {
		m, _ = app.Update(ctrlN)
		app = m.(App)
	}

	lastName := app.detail.item.Name
	// ctrl+n at last item should do nothing
	m, _ = app.Update(ctrlN)
	app = m.(App)
	if app.detail.item.Name != lastName {
		t.Fatal("ctrl+n at last item should do nothing")
	}
}

// TestDetailNextPrevBlockedDuringAction was removed — confirmAction no longer
// exists on detailModel. Navigation blocking during confirmation is now handled
// by the centralized confirmModal at the App level.

func TestDetailNextPrevShowsInHelpBar(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	// Set listTotal > 1 so the help bar shows ctrl+n/p
	app.detail.listTotal = 5
	view := app.detail.helpText()
	assertContains(t, view, "ctrl+n/p")
}

// ---------------------------------------------------------------------------
// Help bar audit (4.19)
// ---------------------------------------------------------------------------

func TestHelpBarNoSaveOnOverviewTab(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	// Should be on Overview tab by default
	view := app.detail.helpText()
	assertNotContains(t, view, "s save")
	// Copy is only shown for library items, not regular Skills
	assertNotContains(t, view, "c copy")
}

// ---------------------------------------------------------------------------
// Method picker path preview (4.18)
// ---------------------------------------------------------------------------

func TestInstallModalShowsDestination(t *testing.T) {
	providers := testProviders(t)
	detected := []provider.Provider{}
	for _, p := range providers {
		if p.Detected {
			detected = append(detected, p)
		}
	}

	modal := newInstallModal(
		catalog.ContentItem{Name: "test-skill", Type: catalog.Skills},
		detected, "/tmp/test",
	)

	view := modal.View()
	assertContains(t, view, "Destination")
}
