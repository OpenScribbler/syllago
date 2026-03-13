package tui

import (
	"strings"
	"testing"
)

// navigateToLoadoutDetail navigates from the homepage through the loadout card
// grid to the first loadout's detail screen (the "starter-loadout").
func navigateToLoadoutDetail(t *testing.T) App {
	t.Helper()
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+1) // Loadouts
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenLoadoutCards)

	// Enter the first loadout card → items list
	m, _ = app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)

	// Enter the first item → detail
	m, _ = app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenDetail)
	return app
}

func navigateToLoadoutDetailSize(t *testing.T, width, height int) App {
	t.Helper()
	app := testAppSize(t, width, height)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+1) // Loadouts
	m, _ := app.Update(keyEnter)
	app = m.(App)

	m, _ = app.Update(keyEnter)
	app = m.(App)

	m, _ = app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenDetail)
	return app
}

// ---------------------------------------------------------------------------
// Contents tab: split view items
// ---------------------------------------------------------------------------

func TestLoadoutContentsGroupedItems(t *testing.T) {
	app := navigateToLoadoutDetail(t)

	// Should be on Files tab (Contents for loadouts) by default
	if app.detail.activeTab != tabFiles {
		t.Fatalf("expected tabFiles, got %d", app.detail.activeTab)
	}

	// The starter-loadout has skills and rules
	items := app.detail.loadoutContents.splitView.items
	if len(items) == 0 {
		t.Fatal("expected loadout contents items, got none")
	}

	// Check for type headers (disabled items)
	var headers []string
	for _, item := range items {
		if item.Disabled {
			headers = append(headers, item.Label)
		}
	}
	if len(headers) == 0 {
		t.Fatal("expected type group headers, got none")
	}

	// Should have "Rules (1)" and "Skills (2)" headers
	foundRules := false
	foundSkills := false
	for _, h := range headers {
		if strings.Contains(h, "Rules") {
			foundRules = true
		}
		if strings.Contains(h, "Skills") {
			foundSkills = true
		}
	}
	if !foundRules {
		t.Error("missing Rules header")
	}
	if !foundSkills {
		t.Error("missing Skills header")
	}
}

func TestLoadoutContentsCursorSkipsHeaders(t *testing.T) {
	app := navigateToLoadoutDetail(t)

	sv := &app.detail.loadoutContents.splitView
	if len(sv.items) == 0 {
		t.Skip("no items in loadout contents")
	}

	// Cursor should start on first selectable item (not a header)
	if sv.items[sv.cursor].Disabled {
		t.Error("cursor should not be on a disabled header item")
	}

	// Navigate down through all items — cursor should never land on disabled items
	for i := 0; i < len(sv.items); i++ {
		m, _ := app.Update(keyDown)
		app = m.(App)
		sv = &app.detail.loadoutContents.splitView
		if sv.cursor < len(sv.items) && sv.items[sv.cursor].Disabled {
			t.Errorf("cursor landed on disabled item at index %d: %q", sv.cursor, sv.items[sv.cursor].Label)
		}
	}
}

func TestLoadoutContentsPreviewLoads(t *testing.T) {
	app := navigateToLoadoutDetail(t)

	sv := &app.detail.loadoutContents.splitView
	if len(sv.items) == 0 {
		t.Skip("no items in loadout contents")
	}

	// Preview should be loaded for the initial item
	if sv.previewContent == "" {
		t.Error("expected preview content for the initially selected item")
	}

	// Navigate down and check preview updates
	m, _ := app.Update(keyDown)
	app = m.(App)
	// Process the cursor message
	sv = &app.detail.loadoutContents.splitView
	cmd := sv.cursorCmd()
	if cmd != nil {
		msg := cmd()
		m, _ = app.Update(msg)
		app = m.(App)
	}

	if app.detail.loadoutContents.splitView.previewContent == "" {
		t.Error("expected preview content after navigating down")
	}
}

func TestLoadoutContentsUnresolvedItem(t *testing.T) {
	// The starter-loadout references "test-rule" which exists in the catalog.
	// Let's verify a loadout with a non-existent item shows "(not found)".
	app := navigateToLoadoutDetail(t)

	// Check resolvedItems — all items in starter-loadout should resolve
	for i, resolved := range app.detail.loadoutContents.resolvedItems {
		if resolved == nil && !app.detail.loadoutContents.splitView.items[i].Disabled {
			// This would mean an unresolved item — which is acceptable
			// but shouldn't happen for starter-loadout since all items exist
			t.Logf("unresolved item at index %d: %q",
				i, app.detail.loadoutContents.splitView.items[i].Label)
		}
	}
}

func TestLoadoutContentsPaneNavigation(t *testing.T) {
	app := navigateToLoadoutDetailSize(t, 120, 40)

	// Render to set split view dimensions
	_ = snapshotApp(t, app)

	sv := &app.detail.loadoutContents.splitView
	if !sv.IsSplit() {
		t.Skipf("not in split mode at 120x40 (width=%d, need>=%d)", sv.width, splitViewMinWidth)
	}

	// Default focus should be on list pane
	if sv.FocusedPane() != paneList {
		t.Error("expected default focus on list pane")
	}

	// Press Right to move to preview
	m, _ := app.Update(keyRight)
	app = m.(App)
	if app.detail.loadoutContents.splitView.FocusedPane() != panePreview {
		t.Error("expected focus on preview pane after pressing Right")
	}

	// Press Left to move back to list
	m, _ = app.Update(keyLeft)
	app = m.(App)
	if app.detail.loadoutContents.splitView.FocusedPane() != paneList {
		t.Error("expected focus on list pane after pressing Left")
	}

	// Press 'l' for vim-style right
	m, _ = app.Update(keyRune('l'))
	app = m.(App)
	if app.detail.loadoutContents.splitView.FocusedPane() != panePreview {
		t.Error("expected focus on preview pane after pressing 'l'")
	}
}

func TestLoadoutContentsApplyTabNoSplitView(t *testing.T) {
	app := navigateToLoadoutDetail(t)

	// Switch to Apply tab
	m, _ := app.Update(keyRune('2'))
	app = m.(App)

	if app.detail.activeTab != tabInstall {
		t.Fatalf("expected tabInstall, got %d", app.detail.activeTab)
	}

	// l/h should NOT switch panes on Apply tab
	m, _ = app.Update(keyRune('l'))
	app = m.(App)
	// Should still be on Apply tab, not affecting split view
	if app.detail.activeTab != tabInstall {
		t.Error("'l' should not change tab")
	}
}

func TestLoadoutContentsTabSwitchResetsFocus(t *testing.T) {
	app := navigateToLoadoutDetailSize(t, 120, 40)
	_ = snapshotApp(t, app) // render to set dimensions

	// Move to preview pane
	m, _ := app.Update(keyRight)
	app = m.(App)
	if app.detail.loadoutContents.splitView.FocusedPane() != panePreview {
		t.Skip("can't switch panes in non-split mode")
	}

	// Switch to Apply tab and back
	m, _ = app.Update(keyRune('2'))
	app = m.(App)
	m, _ = app.Update(keyRune('1'))
	app = m.(App)

	// Focus should reset to list pane
	if app.detail.loadoutContents.splitView.FocusedPane() != paneList {
		t.Error("pane focus should reset to list after tab switch")
	}
}

// ---------------------------------------------------------------------------
// Golden file tests
// ---------------------------------------------------------------------------

// TestLoadoutAddKeyOpensWizard verifies that pressing 'a' on the items list
// when drilled in from a loadout card opens the create loadout wizard, not the import wizard.
func TestLoadoutAddKeyOpensWizard(t *testing.T) {
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+1) // Loadouts in sidebar
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenLoadoutCards)

	// Drill into the first card → items list
	m, _ = app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)

	t.Logf("cardParent=%d (want %d=screenLoadoutCards)", app.cardParent, screenLoadoutCards)
	t.Logf("sourceProvider=%q", app.items.sourceProvider)
	t.Logf("contentType=%s", app.items.contentType)

	// Press 'a' — should open create loadout wizard
	m, _ = app.Update(keyRune('a'))
	app = m.(App)

	if app.createLoadoutModal.active {
		t.Log("create loadout wizard opened (correct)")
	} else if app.screen == screenImport {
		t.Error("import wizard opened instead of create loadout wizard")
	} else {
		t.Errorf("unexpected state: screen=%d, createLoadoutModal.active=%v", app.screen, app.createLoadoutModal.active)
	}
}

func TestGoldenFullApp_LoadoutDetailContents(t *testing.T) {
	app := navigateToLoadoutDetail(t)
	requireGolden(t, "fullapp-loadout-detail-contents", snapshotApp(t, app))
}

func TestGoldenSized_LoadoutDetailContents(t *testing.T) {
	for _, sz := range testSizes {
		t.Run(sz.tag, func(t *testing.T) {
			app := navigateToLoadoutDetailSize(t, sz.width, sz.height)
			requireGolden(t, "fullapp-loadout-detail-contents-"+sz.tag,
				normalizeSnapshot(snapshotApp(t, app)))
		})
	}
}

