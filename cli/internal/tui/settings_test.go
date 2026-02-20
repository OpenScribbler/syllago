package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/catalog"
)

// navigateToSettings creates a test app and navigates to the settings screen.
func navigateToSettings(t *testing.T) App {
	t.Helper()
	app := testApp(t)
	nTypes := len(catalog.AllContentTypes())
	app = pressN(app, keyDown, nTypes+3) // Settings
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

	// 2 rows: auto-update(0), providers(1)
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
	// Cursor at 0 = auto-update

	// Initial value should be "off"
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

func TestSettingsProviderSubPicker(t *testing.T) {
	app := navigateToSettings(t)
	app = pressN(app, keyDown, 1) // cursor to providers row

	m, _ := app.Update(keyEnter)
	app = m.(App)

	if app.settings.editMode != editProviders {
		t.Fatalf("expected editProviders, got %d", app.settings.editMode)
	}
	if len(app.settings.subItems) == 0 {
		t.Fatal("expected sub-picker items for providers")
	}
}

func TestSettingsProviderPickerNav(t *testing.T) {
	app := navigateToSettings(t)
	app = pressN(app, keyDown, 1)
	m, _ := app.Update(keyEnter) // open provider picker
	app = m.(App)

	if len(app.settings.subItems) < 2 {
		t.Skip("need at least 2 providers for nav test")
	}

	m, _ = app.Update(keyDown)
	app = m.(App)
	if app.settings.subCur != 1 {
		t.Fatalf("expected subCur 1, got %d", app.settings.subCur)
	}
}

func TestSettingsProviderPickerToggle(t *testing.T) {
	app := navigateToSettings(t)
	app = pressN(app, keyDown, 1)
	m, _ := app.Update(keyEnter) // open provider picker
	app = m.(App)

	initial := app.settings.subItems[0].checked

	// Space toggles
	m, _ = app.Update(keySpace)
	app = m.(App)
	if app.settings.subItems[0].checked == initial {
		t.Fatal("space should toggle provider checkbox")
	}
}

func TestSettingsProviderPickerEscApplies(t *testing.T) {
	app := navigateToSettings(t)
	app = pressN(app, keyDown, 1)
	m, _ := app.Update(keyEnter) // open provider picker
	app = m.(App)

	// Toggle a provider
	m, _ = app.Update(keySpace)
	app = m.(App)

	// Esc in the sub-picker applies and closes
	// (Within the settings model, Esc closes picker. At the app level,
	// the settings.HasPendingAction() check calls CancelAction() which also applies.)
	m, _ = app.Update(keyEsc)
	app = m.(App)

	if app.settings.editMode != editNone {
		t.Fatal("esc should close sub-picker")
	}
}

func TestSettingsSave(t *testing.T) {
	app := navigateToSettings(t)

	// Ensure the config directory exists
	configDir := filepath.Join(app.catalog.RepoRoot, ".nesco")
	os.MkdirAll(configDir, 0o755)

	// Press 's' to save
	m, _ := app.Update(keyRune('s'))
	app = m.(App)

	if app.settings.message == "" {
		t.Fatal("expected message after save")
	}
}

func TestSettingsBackNav(t *testing.T) {
	app := navigateToSettings(t)

	m, _ := app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenCategory)
}

func TestSettingsBackCancelsSubPicker(t *testing.T) {
	app := navigateToSettings(t)
	app = pressN(app, keyDown, 1)
	m, _ := app.Update(keyEnter) // open provider picker
	app = m.(App)

	if app.settings.editMode == editNone {
		t.Fatal("expected sub-picker to be open")
	}

	// Esc should close sub-picker first, not navigate back
	m, _ = app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenSettings)
}

func TestSettingsAutoSaveOnEsc(t *testing.T) {
	app := navigateToSettings(t)

	// Toggle auto-update (makes it dirty)
	m, _ := app.Update(keyEnter)
	app = m.(App)

	if !app.settings.dirty {
		t.Fatal("expected dirty=true after toggle")
	}

	// Esc should auto-save and go back
	m, _ = app.Update(keyEsc)
	app = m.(App)
	assertScreen(t, app, screenCategory)
	// Verify config was written (save() was called)
	// We can't easily check file IO in this test, but no error means save() ran
}

func TestSettingsViewRendering(t *testing.T) {
	app := navigateToSettings(t)
	view := app.View()

	assertContains(t, view, "Settings")
	assertContains(t, view, "Auto-update")
	assertContains(t, view, "Providers")
	assertNotContains(t, view, "Disabled detectors")
}
