package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// navigateToConfig moves the app to the Config group (group index 2).
func navigateToConfig(t *testing.T, app App) App {
	t.Helper()
	m, cmd := app.Update(keyRune('3'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	return m.(App)
}

// navigateToConfigTab moves the app to Config group then presses Right n times.
// Right cycles the Config topbar sub-tabs: 0=Settings, 1=Sandbox, 2=System.
// (Tab in Config group routes to sub-model internal panels, not topbar tabs.)
func navigateToConfigTab(t *testing.T, app App, tabIndex int) App {
	t.Helper()
	app = navigateToConfig(t, app)
	for i := 0; i < tabIndex; i++ {
		m, cmd := app.Update(keyPress(tea.KeyRight))
		if cmd != nil {
			m, _ = m.Update(cmd())
		}
		app = m.(App)
	}
	return app
}

// --- Settings tab goldens ---

func TestGoldenConfig_Settings_60x20(t *testing.T) {
	app := navigateToConfigTab(t, testAppSize(t, 60, 20), 0)
	requireGolden(t, "config-settings-60x20", snapshotApp(t, app))
}

func TestGoldenConfig_Settings_80x30(t *testing.T) {
	app := navigateToConfigTab(t, testAppSize(t, 80, 30), 0)
	requireGolden(t, "config-settings-80x30", snapshotApp(t, app))
}

func TestGoldenConfig_Settings_120x40(t *testing.T) {
	app := navigateToConfigTab(t, testAppSize(t, 120, 40), 0)
	requireGolden(t, "config-settings-120x40", snapshotApp(t, app))
}

// --- Sandbox tab goldens ---

func TestGoldenConfig_Sandbox_60x20(t *testing.T) {
	app := navigateToConfigTab(t, testAppSize(t, 60, 20), 1)
	requireGolden(t, "config-sandbox-60x20", snapshotApp(t, app))
}

func TestGoldenConfig_Sandbox_80x30(t *testing.T) {
	app := navigateToConfigTab(t, testAppSize(t, 80, 30), 1)
	requireGolden(t, "config-sandbox-80x30", snapshotApp(t, app))
}

func TestGoldenConfig_Sandbox_120x40(t *testing.T) {
	app := navigateToConfigTab(t, testAppSize(t, 120, 40), 1)
	requireGolden(t, "config-sandbox-120x40", snapshotApp(t, app))
}

// --- System tab goldens ---

func TestGoldenConfig_System_60x20(t *testing.T) {
	app := navigateToConfigTab(t, testAppSize(t, 60, 20), 2)
	requireGolden(t, "config-system-60x20", snapshotApp(t, app))
}

func TestGoldenConfig_System_80x30(t *testing.T) {
	app := navigateToConfigTab(t, testAppSize(t, 80, 30), 2)
	requireGolden(t, "config-system-80x30", snapshotApp(t, app))
}

func TestGoldenConfig_System_120x40(t *testing.T) {
	app := navigateToConfigTab(t, testAppSize(t, 120, 40), 2)
	requireGolden(t, "config-system-120x40", snapshotApp(t, app))
}

// --- Sandbox add modal golden ---

func TestGoldenConfig_SandboxAddModal_80x30(t *testing.T) {
	app := navigateToConfigTab(t, testAppSize(t, 80, 30), 1)
	// Press 'a' to open add domain modal
	m, _ := app.Update(keyRune('a'))
	app = m.(App)
	requireGolden(t, "config-sandbox-add-modal-80x30", snapshotApp(t, app))
}
