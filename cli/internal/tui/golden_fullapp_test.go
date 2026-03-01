// cli/internal/tui/golden_fullapp_test.go
package tui

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// snapshotApp renders the full app view, strips ANSI, and normalizes non-deterministic
// content (temp paths, trailing whitespace) so golden files are stable across runs.
func snapshotApp(t *testing.T, app App) string {
	t.Helper()
	return normalizeSnapshot(stripANSI(app.View()))
}

func TestGoldenFullApp_CategoryWelcome(t *testing.T) {
	app := testApp(t)
	// testApp starts on screenCategory with focusSidebar — no navigation needed.
	requireGolden(t, "fullapp-category-welcome", snapshotApp(t, app))
}

func TestGoldenFullApp_ItemsSkills(t *testing.T) {
	app := testApp(t)
	// Enter on Skills (first sidebar item, cursor=0) → items screen
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)
	requireGolden(t, "fullapp-items-skills", snapshotApp(t, app))
}

func TestGoldenFullApp_DetailOverview(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	// navigateToDetail lands on the first skill's overview tab
	requireGolden(t, "fullapp-detail-overview", snapshotApp(t, app))
}

func TestGoldenFullApp_DetailFiles(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	// Switch to Files tab (key "2")
	m, _ := app.Update(keyRune('2'))
	app = m.(App)
	requireGolden(t, "fullapp-detail-files", snapshotApp(t, app))
}

func TestGoldenFullApp_DetailInstall(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	// Switch to Install tab (key "3")
	m, _ := app.Update(keyRune('3'))
	app = m.(App)
	requireGolden(t, "fullapp-detail-install", snapshotApp(t, app))
}

func TestGoldenFullApp_SearchResults(t *testing.T) {
	app := testApp(t)
	// Activate search
	m, _ := app.Update(keyRune('/'))
	app = m.(App)
	// Type "alpha" one rune at a time
	for _, r := range "alpha" {
		m, _ = app.Update(keyRune(r))
		app = m.(App)
	}
	// Submit search → items screen with filtered results
	m, _ = app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)
	requireGolden(t, "fullapp-search-results", snapshotApp(t, app))
}

func TestGoldenFullApp_Modal(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	// Force-inject an openModalMsg directly rather than relying on install state.
	// This guarantees the modal is always active for the snapshot — no t.Skip.
	m, _ := app.Update(openModalMsg{
		title: "Confirm Uninstall",
		body:  "Remove alpha-skill from all providers?",
	})
	app = m.(App)
	if !app.modal.active {
		t.Fatalf("modal was not activated after openModalMsg — check openModalMsg handling in App.Update")
	}
	requireGolden(t, "fullapp-modal", snapshotApp(t, app))
}

func TestGoldenFullApp_Settings(t *testing.T) {
	app := testApp(t)
	// Navigate sidebar to Settings: len(AllContentTypes()) + 3 presses down.
	// AllContentTypes() has 9 types, then My Tools (+1), Import (+2), Update (+3), Settings (+4).
	// Index of Settings = 9 + 3 = 12 (0-based), so 12 down presses from cursor=0.
	nTypes := 9 // catalog.AllContentTypes() length
	app = pressN(app, keyDown, nTypes+3)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenSettings)
	requireGolden(t, "fullapp-settings", snapshotApp(t, app))
}
