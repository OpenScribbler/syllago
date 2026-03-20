package tui

import (
	"os"
	"path/filepath"
	"testing"
)

// navigateToSettings creates a test app and navigates to the settings screen.
func navigateToSettings(t *testing.T) App {
	t.Helper()
	app := testApp(t)
	nTypes := sidebarContentCount()
	app = pressN(app, keyDown, nTypes+5) // Settings
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenSettings)
	return app
}

func TestSettingsNavigation(t *testing.T) {
	app := navigateToSettings(t)

	if app.settings.cursor != 0 {
		t.Fatalf("expected initial cursor 0, got %d", app.settings.cursor)
	}

	// 2 rows: auto-update(0), registry-auto-sync(1)
	m, _ := app.Update(keyDown)
	app = m.(App)
	if app.settings.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", app.settings.cursor)
	}

	// Bounds clamping
	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.settings.cursor != 1 {
		t.Fatal("cursor should clamp at 1")
	}

	m, _ = app.Update(keyUp)
	app = m.(App)
	if app.settings.cursor != 0 {
		t.Fatalf("expected cursor 0 after up, got %d", app.settings.cursor)
	}
}

func TestSettingsAutoUpdateToggle(t *testing.T) {
	app := navigateToSettings(t)

	// Ensure the config directory exists for auto-save
	configDir := filepath.Join(app.catalog.RepoRoot, ".syllago")
	os.MkdirAll(configDir, 0o755)

	// Cursor at 0 = auto-update
	initial := app.settings.cfg.Preferences["autoUpdate"]

	// Enter toggles
	m, _ := app.Update(keyEnter)
	app = m.(App)
	after := app.settings.cfg.Preferences["autoUpdate"]
	if initial == after {
		t.Fatal("enter should toggle auto-update")
	}

	// Space also toggles
	m, _ = app.Update(keySpace)
	app = m.(App)
	afterSpace := app.settings.cfg.Preferences["autoUpdate"]
	if after == afterSpace {
		t.Fatal("space should also toggle auto-update")
	}
}

func TestSettingsAutoSaveOnToggle(t *testing.T) {
	app := navigateToSettings(t)

	// Ensure the config directory exists
	configDir := filepath.Join(app.catalog.RepoRoot, ".syllago")
	os.MkdirAll(configDir, 0o755)

	// Toggle auto-update — should auto-save
	m, _ := app.Update(keyEnter)
	app = m.(App)

	if app.toast.text != "Settings saved" {
		t.Fatalf("expected auto-save message in toast, got %q", app.toast.text)
	}
}

func TestSettingsBackNav(t *testing.T) {
	app := navigateToSettings(t)

	m, _ := app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenCategory)
}

func TestSettingsViewRendering(t *testing.T) {
	app := navigateToSettings(t)
	view := app.View()

	assertContains(t, view, "Settings")
	assertContains(t, view, "Auto-update")
	assertContains(t, view, "Registry auto-sync")
	assertNotContains(t, view, "Providers")
}
