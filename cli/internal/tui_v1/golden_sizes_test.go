// cli/internal/tui/golden_sizes_test.go
//
// Golden tests at multiple terminal sizes to catch breakpoint regressions.
//
// Breakpoints in the TUI:
//
//	60x20  — minimum viable terminal (below this: "Terminal too small" error)
//	80x30  — default (tested in golden_fullapp_test.go)
//	120x40 — medium: card layout visible (height >= 35), two-column cards
//	160x50 — large: ASCII art title visible (height >= 42, content width >= 55)
package tui_v1

import (
	"fmt"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// testSizes defines the additional terminal sizes to test at.
// 80x30 is already covered by golden_fullapp_test.go.
var testSizes = []struct {
	width  int
	height int
	tag    string // short label for golden file names
}{
	{60, 20, "60x20"},
	{120, 40, "120x40"},
	{160, 50, "160x50"},
}

func snapshotAppSize(t *testing.T, width, height int) string {
	t.Helper()
	app := testAppSize(t, width, height)
	return normalizeSnapshot(stripANSI(app.View()))
}

func navigateToDetailSize(t *testing.T, ct catalog.ContentType, width, height int) App {
	t.Helper()
	app := testAppSize(t, width, height)

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

	m, _ = app.Update(keyEnter) // → detail
	app = m.(App)
	assertScreen(t, app, screenDetail)
	return app
}

// --- Category Welcome (most breakpoint-sensitive screen) ---

func TestGoldenSized_CategoryWelcome(t *testing.T) {
	for _, sz := range testSizes {
		t.Run(sz.tag, func(t *testing.T) {
			snap := snapshotAppSize(t, sz.width, sz.height)
			requireGolden(t, fmt.Sprintf("fullapp-category-welcome-%s", sz.tag), snap)
		})
	}
}

// --- Items list (width-dependent column layout) ---

func TestGoldenSized_ItemsSkills(t *testing.T) {
	for _, sz := range testSizes {
		t.Run(sz.tag, func(t *testing.T) {
			app := testAppSize(t, sz.width, sz.height)
			m, _ := app.Update(keyEnter)
			app = m.(App)
			assertScreen(t, app, screenItems)
			requireGolden(t, fmt.Sprintf("fullapp-items-skills-%s", sz.tag),
				normalizeSnapshot(snapshotApp(t, app)))
		})
	}
}

// --- Settings (width affects layout) ---

func TestGoldenSized_Settings(t *testing.T) {
	for _, sz := range testSizes {
		t.Run(sz.tag, func(t *testing.T) {
			app := testAppSize(t, sz.width, sz.height)
			nTypes := sidebarContentCount()
			app = pressN(app, keyDown, nTypes+5)
			m, _ := app.Update(keyEnter)
			app = m.(App)
			assertScreen(t, app, screenSettings)
			requireGolden(t, fmt.Sprintf("fullapp-settings-%s", sz.tag),
				normalizeSnapshot(snapshotApp(t, app)))
		})
	}
}

// --- Library cards (responsive card grid) ---

func TestGoldenSized_LibraryCards(t *testing.T) {
	for _, sz := range testSizes {
		t.Run(sz.tag, func(t *testing.T) {
			app := testAppSize(t, sz.width, sz.height)
			nTypes := sidebarContentCount()
			app = pressN(app, keyDown, nTypes) // Library
			m, _ := app.Update(keyEnter)
			app = m.(App)
			assertScreen(t, app, screenLibraryCards)
			requireGolden(t, fmt.Sprintf("fullapp-library-cards-%s", sz.tag),
				normalizeSnapshot(snapshotApp(t, app)))
		})
	}
}

// --- Loadout cards (responsive card grid) ---

func TestGoldenSized_LoadoutCards(t *testing.T) {
	for _, sz := range testSizes {
		t.Run(sz.tag, func(t *testing.T) {
			app := testAppSize(t, sz.width, sz.height)
			nTypes := sidebarContentCount()
			app = pressN(app, keyDown, nTypes+1) // Loadouts
			m, _ := app.Update(keyEnter)
			app = m.(App)
			assertScreen(t, app, screenLoadoutCards)
			requireGolden(t, fmt.Sprintf("fullapp-loadout-cards-%s", sz.tag),
				normalizeSnapshot(snapshotApp(t, app)))
		})
	}
}

// --- Registries (responsive card grid) ---

func TestGoldenSized_Registries(t *testing.T) {
	for _, sz := range testSizes {
		t.Run(sz.tag, func(t *testing.T) {
			app := testAppSize(t, sz.width, sz.height)
			nTypes := sidebarContentCount()
			app = pressN(app, keyDown, nTypes+2) // Registries
			m, _ := app.Update(keyEnter)
			app = m.(App)
			assertScreen(t, app, screenRegistries)
			requireGolden(t, fmt.Sprintf("fullapp-registries-%s", sz.tag),
				normalizeSnapshot(snapshotApp(t, app)))
		})
	}
}

// --- Import/Add (wizard with text inputs) ---

func TestGoldenSized_Import(t *testing.T) {
	for _, sz := range testSizes {
		t.Run(sz.tag, func(t *testing.T) {
			app := testAppSize(t, sz.width, sz.height)
			nTypes := sidebarContentCount()
			app = pressN(app, keyDown, nTypes+3) // Add
			m, _ := app.Update(keyEnter)
			app = m.(App)
			assertScreen(t, app, screenImport)
			requireGolden(t, fmt.Sprintf("fullapp-import-%s", sz.tag),
				normalizeSnapshot(snapshotApp(t, app)))
		})
	}
}

// --- Update (version info + menu) ---

func TestGoldenSized_Update(t *testing.T) {
	for _, sz := range testSizes {
		t.Run(sz.tag, func(t *testing.T) {
			app := testAppSize(t, sz.width, sz.height)
			nTypes := sidebarContentCount()
			app = pressN(app, keyDown, nTypes+4) // Update
			m, _ := app.Update(keyEnter)
			app = m.(App)
			assertScreen(t, app, screenUpdate)
			requireGolden(t, fmt.Sprintf("fullapp-update-%s", sz.tag),
				normalizeSnapshot(snapshotApp(t, app)))
		})
	}
}

// --- Sandbox (form-like settings) ---

func TestGoldenSized_Sandbox(t *testing.T) {
	for _, sz := range testSizes {
		t.Run(sz.tag, func(t *testing.T) {
			app := testAppSize(t, sz.width, sz.height)
			nTypes := sidebarContentCount()
			app = pressN(app, keyDown, nTypes+6) // Sandbox
			m, _ := app.Update(keyEnter)
			app = m.(App)
			assertScreen(t, app, screenSandbox)
			requireGolden(t, fmt.Sprintf("fullapp-sandbox-%s", sz.tag),
				normalizeSnapshot(snapshotApp(t, app)))
		})
	}
}
