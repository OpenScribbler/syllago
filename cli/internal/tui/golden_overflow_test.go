package tui

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// ---------------------------------------------------------------------------
// Overflow tests: large dataset (85+ items)
// ---------------------------------------------------------------------------

func TestGoldenOverflow_ItemsSkills(t *testing.T) {
	app := testAppLarge(t)
	// Enter on Skills (first sidebar item) → items screen with 50 items
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)
	requireGolden(t, "fullapp-items-overflow", snapshotApp(t, app))
}

func TestGoldenOverflow_DetailLongDesc(t *testing.T) {
	app := testAppLarge(t)
	// Navigate to items
	m, _ := app.Update(keyEnter)
	app = m.(App)
	// Navigate to first item detail (which has a long name from the factory)
	m, _ = app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenDetail)
	requireGolden(t, "fullapp-detail-overflow", snapshotApp(t, app))
}

func TestGoldenOverflow_Sidebar(t *testing.T) {
	app := testAppLarge(t)
	// Sidebar with all content types visible
	requireGolden(t, "component-sidebar-overflow",
		normalizeSnapshot(stripANSI(app.sidebar.View())))
}

func TestGoldenOverflow_ItemsList(t *testing.T) {
	app := testAppLarge(t)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	requireGolden(t, "component-items-overflow",
		normalizeSnapshot(stripANSI(app.items.View())))
}

func TestGoldenOverflow_ItemsMinTerminal(t *testing.T) {
	app := testAppLargeSize(t, 60, 20)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)
	requireGolden(t, "fullapp-items-overflow-60x20", snapshotApp(t, app))
}

// ---------------------------------------------------------------------------
// Empty state tests: no items
// ---------------------------------------------------------------------------

func TestGoldenEmpty_Items(t *testing.T) {
	app := testAppEmpty(t)
	// Navigate to Skills (first sidebar item) — no items to show
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)
	requireGolden(t, "fullapp-items-empty", snapshotApp(t, app))
}

func TestGoldenEmpty_Category(t *testing.T) {
	app := testAppEmpty(t)
	// Category welcome with all zero counts
	requireGolden(t, "fullapp-category-empty", snapshotApp(t, app))
}

// ---------------------------------------------------------------------------
// Overflow at different sizes
// ---------------------------------------------------------------------------

func TestGoldenOverflow_DetailMinTerminal(t *testing.T) {
	app := testAppLargeSize(t, 60, 20)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	// Navigate to first item
	m, _ = app.Update(keyEnter)
	app = m.(App)
	if app.screen == screenDetail {
		requireGolden(t, "fullapp-detail-overflow-60x20", snapshotApp(t, app))
	}
}

func TestGoldenOverflow_ItemsScrolledDown(t *testing.T) {
	app := testAppLarge(t)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)
	// Scroll down 10 items to test mid-list rendering
	app = pressN(app, keyDown, 10)
	requireGolden(t, "fullapp-items-overflow-scrolled", snapshotApp(t, app))
}

// Verify items with very long names are displayed (truncation tested visually via golden)
func TestGoldenOverflow_LongNameItem(t *testing.T) {
	app := testAppLarge(t)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	// Item at index 10 has the second long name (i%10==0, so index 10)
	app = pressN(app, keyDown, 10)
	m, _ = app.Update(keyEnter)
	app = m.(App)
	if app.screen == screenDetail {
		requireGolden(t, "fullapp-detail-longname", snapshotApp(t, app))
	}
}

// Verify the sidebar still renders correctly with empty catalog counts
func TestGoldenEmpty_Sidebar(t *testing.T) {
	app := testAppEmpty(t)
	requireGolden(t, "component-sidebar-empty",
		normalizeSnapshot(stripANSI(app.sidebar.View())))
}

// Agents overflow (20 items)
func TestGoldenOverflow_ItemsAgents(t *testing.T) {
	app := testAppLarge(t)
	// Navigate down to Agents in sidebar
	types := catalog.AllContentTypes()
	agentIdx := 0
	for i, ct := range types {
		if ct == catalog.Agents {
			agentIdx = i
			break
		}
	}
	// Loadouts doesn't appear in sidebar content section, adjust index
	for _, ct := range types[:agentIdx] {
		if ct == catalog.Loadouts {
			agentIdx--
			break
		}
	}
	app = pressN(app, keyDown, agentIdx)
	m, _ := app.Update(keyEnter)
	app = m.(App)
	assertScreen(t, app, screenItems)
	requireGolden(t, "fullapp-items-agents-overflow", snapshotApp(t, app))
}
