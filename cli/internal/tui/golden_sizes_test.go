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
package tui

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

// --- Detail overview (width affects content rendering) ---

func TestGoldenSized_DetailOverview(t *testing.T) {
	for _, sz := range testSizes {
		t.Run(sz.tag, func(t *testing.T) {
			app := navigateToDetailSize(t, catalog.Skills, sz.width, sz.height)
			requireGolden(t, fmt.Sprintf("fullapp-detail-overview-%s", sz.tag),
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
			app = pressN(app, keyDown, nTypes+4)
			m, _ := app.Update(keyEnter)
			app = m.(App)
			assertScreen(t, app, screenSettings)
			requireGolden(t, fmt.Sprintf("fullapp-settings-%s", sz.tag),
				normalizeSnapshot(snapshotApp(t, app)))
		})
	}
}
