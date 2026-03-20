// cli/internal/tui/golden_fullapp_test.go
package tui

import (
	"os"
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

func TestGoldenFullApp_DetailFiles(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	// navigateToDetail lands on the Files tab (now default)
	requireGolden(t, "fullapp-detail-files", snapshotApp(t, app))
}

func TestGoldenFullApp_DetailInstall(t *testing.T) {
	app := navigateToDetail(t, catalog.Skills)
	// Switch to Install tab (key "2")
	m, _ := app.Update(keyRune('2'))
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
	// Skip in CI: sidebar width is non-deterministic (±1 char) due to lipgloss
	// style state leaking between tests. Tracked for fix in a follow-up.
	if os.Getenv("CI") != "" {
		t.Skip("flaky in CI: sidebar width non-deterministic (lipgloss style state leak)")
	}
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
	// Navigate sidebar to Settings: sidebarContentCount() + 5 presses down.
	// 6 content types, then Library (+0), Loadouts (+1), Registries (+2), Add (+3), Update (+4), Settings (+5).
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+5)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenSettings)
	requireGolden(t, "fullapp-settings", snapshotApp(t, app))
}

func TestGoldenFullApp_LibraryCards(t *testing.T) {
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes) // Library
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenLibraryCards)
	requireGolden(t, "fullapp-library-cards", snapshotApp(t, app))
}

func TestGoldenFullApp_LoadoutCards(t *testing.T) {
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+1) // Loadouts
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenLoadoutCards)
	requireGolden(t, "fullapp-loadout-cards", snapshotApp(t, app))
}

func TestGoldenFullApp_Registries(t *testing.T) {
	app := navigateToRegistries(t)
	requireGolden(t, "fullapp-registries", snapshotApp(t, app))
}

func TestGoldenFullApp_Import(t *testing.T) {
	app := navigateToImport(t)
	requireGolden(t, "fullapp-import", snapshotApp(t, app))
}

func TestGoldenFullApp_Update(t *testing.T) {
	app := navigateToUpdate(t)
	requireGolden(t, "fullapp-update", snapshotApp(t, app))
}

func TestGoldenFullApp_Sandbox(t *testing.T) {
	app := navigateToSandbox(t)
	requireGolden(t, "fullapp-sandbox", snapshotApp(t, app))
}
