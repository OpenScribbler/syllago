package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func TestAppHasSidebarField(t *testing.T) {
	// Verify App struct has a sidebar field by constructing a zero-value App
	// and checking the sidebar field is accessible (compile-time check).
	var a App
	_ = a.sidebar
}

func TestAppViewContainsBreadcrumb(t *testing.T) {
	// App.View() should include a footer with breadcrumb text
	a := App{
		width:  80,
		height: 24,
		screen: screenCategory,
	}
	view := a.View()
	// The default breadcrumb for screenCategory is "syllago"
	if !strings.Contains(view, "syllago") {
		t.Error("App.View() should contain 'syllago' breadcrumb in the footer")
	}
}

func TestTabTogglesFocus(t *testing.T) {
	// Tab key should toggle focus from sidebar to content
	a := App{
		width:  80,
		height: 24,
		screen: screenItems,
		focus:  focusSidebar,
	}
	tabMsg := tea.KeyMsg{Type: tea.KeyTab}
	result, _ := a.Update(tabMsg)
	updated := result.(App)
	if updated.focus != focusContent {
		t.Errorf("Tab should move focus from sidebar to content, got focus=%d", updated.focus)
	}
}

func TestEscFromItemsGoesToCategory(t *testing.T) {
	// Esc from screenItems should go back to category/welcome screen
	a := App{
		width:  80,
		height: 24,
		screen: screenItems,
		focus:  focusContent,
	}
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	result, _ := a.Update(escMsg)
	updated := result.(App)
	if updated.focus != focusSidebar {
		t.Errorf("Esc from screenItems should set focus=focusSidebar, got %d", updated.focus)
	}
	if updated.screen != screenCategory {
		t.Errorf("Esc from screenItems should go to screenCategory, got %d", updated.screen)
	}
}

// TestFirstRunScreenAppearsWhenEmpty verifies that renderContentWelcome shows
// the first-run screen when both catalog and registries are empty.
func TestFirstRunScreenAppearsWhenEmpty(t *testing.T) {
	a := App{
		width:  80,
		height: 30,
		screen: screenCategory,
		catalog: &catalog.Catalog{
			Items: nil,
		},
		registryCfg: &config.Config{
			Registries: nil,
		},
	}
	view := a.renderContentWelcome()
	if !strings.Contains(view, "Welcome to syllago") {
		t.Error("first-run screen should show 'Welcome to syllago' when catalog is empty")
	}
	if !strings.Contains(view, "syllago import") {
		t.Error("first-run screen should show 'syllago import' step")
	}
	if !strings.Contains(view, "syllago registry add") {
		t.Error("first-run screen should show 'syllago registry add' step")
	}
	if !strings.Contains(view, "syllago create") {
		t.Error("first-run screen should show 'syllago create' step")
	}
}

// TestNormalWelcomeScreenWhenContentExists verifies that the normal welcome
// screen (not first-run) appears when there is at least one catalog item.
func TestNormalWelcomeScreenWhenContentExists(t *testing.T) {
	a := testApp(t)
	a.screen = screenCategory
	view := a.renderContentWelcome()
	// Normal welcome should show category cards/list, not the first-run message
	if strings.Contains(view, "Welcome to syllago!") {
		t.Error("normal welcome screen should not show first-run message when content exists")
	}
}

// TestFirstRunScreenWithRegistriesButNoContent verifies that having registries
// (even without catalog items) bypasses the first-run screen. Users who know
// how to add registries don't need the getting-started guide.
func TestFirstRunScreenWithRegistriesButNoContent(t *testing.T) {
	a := App{
		width:  80,
		height: 30,
		screen: screenCategory,
		catalog: &catalog.Catalog{
			Items: nil,
		},
		registryCfg: &config.Config{
			Registries: []config.Registry{
				{Name: "my-reg", URL: "https://example.com/reg.git"},
			},
		},
	}
	view := a.renderContentWelcome()
	// Should NOT show first-run since user has a registry configured
	if strings.Contains(view, "Welcome to syllago!") {
		t.Error("should not show first-run when registries are configured")
	}
}

func TestHiddenItemsFilteredByDefault(t *testing.T) {
	app := testApp(t)

	// Add a hidden item to the catalog
	app.catalog.Items = append(app.catalog.Items, catalog.ContentItem{
		Name:        "hidden-skill",
		Description: "A hidden skill",
		Type:        catalog.Skills,
		Path:        "/tmp/skills/hidden-skill",
		Meta:        &metadata.Meta{Hidden: true},
	})

	// Navigate to Skills
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)

	// The hidden item should not appear in the items list
	for _, item := range app.items.items {
		if item.Name == "hidden-skill" {
			t.Fatal("hidden item should be filtered out by default")
		}
	}

	// But the hidden count should be tracked
	if app.items.hiddenCount == 0 {
		t.Fatal("expected hiddenCount > 0 when hidden items exist")
	}
}

// TestFooterHelpText_CategoryShowsQuit verifies that the home screen footer
// includes "q: quit" (the only screen where quit is shown).
func TestFooterHelpText_CategoryShowsQuit(t *testing.T) {
	a := App{
		width:  80,
		height: 24,
		screen: screenCategory,
	}
	view := a.View()
	if !strings.Contains(view, "q: quit") {
		t.Error("screenCategory footer should contain 'q: quit'")
	}
}

// TestFooterHelpText_ItemsShowsEscBack verifies that the items screen footer
// shows "Esc: back" and does NOT show "q: quit".
func TestFooterHelpText_ItemsShowsEscBack(t *testing.T) {
	a := App{
		width:  80,
		height: 24,
		screen: screenItems,
	}
	view := a.View()
	if strings.Contains(view, "q: quit") {
		t.Error("screenItems footer should NOT contain 'q: quit'")
	}
	if !strings.Contains(view, "Esc: back") {
		t.Error("screenItems footer should contain 'Esc: back'")
	}
}

// TestFooterHelpText_DetailShowsEscBack verifies that the detail screen footer
// shows "esc back" and does NOT show "q: quit".
func TestFooterHelpText_DetailShowsEscBack(t *testing.T) {
	a := App{
		width:  80,
		height: 24,
		screen: screenDetail,
	}
	view := a.View()
	if strings.Contains(view, "q: quit") {
		t.Error("screenDetail footer should NOT contain 'q: quit'")
	}
	if !strings.Contains(view, "esc back") {
		t.Error("screenDetail footer should contain 'esc back'")
	}
}

// TestFooterHelpText_RegistriesShowsEscBack verifies that the registries screen
// footer shows "Esc: back" and does NOT show "q: quit".
func TestFooterHelpText_RegistriesShowsEscBack(t *testing.T) {
	a := App{
		width:  80,
		height: 24,
		screen: screenRegistries,
	}
	view := a.View()
	if strings.Contains(view, "q: quit") {
		t.Error("screenRegistries footer should NOT contain 'q: quit'")
	}
	if !strings.Contains(view, "Esc: back") {
		t.Error("screenRegistries footer should contain 'Esc: back'")
	}
}

func TestHKeyTogglesShowHidden(t *testing.T) {
	app := testApp(t)

	// Add a hidden item to the catalog
	app.catalog.Items = append(app.catalog.Items, catalog.ContentItem{
		Name:        "hidden-skill",
		Description: "A hidden skill",
		Type:        catalog.Skills,
		Path:        "/tmp/skills/hidden-skill",
		Meta:        &metadata.Meta{Hidden: true},
	})

	// Navigate to Skills
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)

	// Count items before toggle
	countBefore := len(app.items.items)

	// Press H to show hidden items
	m, _ = app.Update(keyRune('H'))
	app = m.(App)

	if !app.showHidden {
		t.Fatal("showHidden should be true after pressing H")
	}

	countAfter := len(app.items.items)
	if countAfter <= countBefore {
		t.Fatalf("expected more items after showing hidden (before=%d, after=%d)", countBefore, countAfter)
	}

	// Verify the hidden item is now visible
	found := false
	for _, item := range app.items.items {
		if item.Name == "hidden-skill" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("hidden-skill should appear after pressing H to show hidden items")
	}

	// Press H again to hide
	m, _ = app.Update(keyRune('H'))
	app = m.(App)

	if app.showHidden {
		t.Fatal("showHidden should be false after pressing H again")
	}

	// Hidden item should be filtered out again
	for _, item := range app.items.items {
		if item.Name == "hidden-skill" {
			t.Fatal("hidden item should be filtered out after toggling H off")
		}
	}
}

func TestFirstRunScreen_NoSyllagoToolsReference(t *testing.T) {
	a := testApp(t)
	// Force first-run by clearing catalog items
	a.catalog = &catalog.Catalog{Items: nil}
	a.width = 80
	a.height = 30

	view := a.View()
	stripped := stripANSI(view)
	if strings.Contains(stripped, "syllago-tools") {
		t.Error("first-run screen must not reference 'syllago-tools'")
	}
}

func TestFirstRunScreen_ContainsRegistryCreateStep(t *testing.T) {
	a := testApp(t)
	a.catalog = &catalog.Catalog{Items: nil}
	a.width = 80
	a.height = 30

	view := a.View()
	stripped := stripANSI(view)
	if !strings.Contains(stripped, "registry create") {
		t.Error("first-run screen should show 'registry create' step")
	}
}
