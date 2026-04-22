package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// MOAT trust-surfacing goldens. These lock in the visual output of:
//   - row prefix glyphs (✓ / R / P) in the library table
//   - metapanel Trust line with TrustDescription text
//   - metapanel revocation banner (reason + issuer + details_url)
//   - metapanel private-repo visibility chip
//   - publisher-warn confirm modal (danger border, body text, buttons)
//
// Covered sizes: 60x20 (minimum), 80x30 (default), 120x40 (wide). Every
// surface is captured at all three to prove layout math holds when the
// metapanel grows from 3 → 4 → 5 lines per metaBarLinesFor.

func TestGolden_MOAT_Library_60x20(t *testing.T) {
	app := testAppWithMOATItems(t, 60, 20)
	requireGolden(t, "moat-library-60x20", snapshotApp(t, app))
}

func TestGolden_MOAT_Library_80x30(t *testing.T) {
	app := testAppWithMOATItems(t, 80, 30)
	requireGolden(t, "moat-library-80x30", snapshotApp(t, app))
}

func TestGolden_MOAT_Library_120x40(t *testing.T) {
	app := testAppWithMOATItems(t, 120, 40)
	requireGolden(t, "moat-library-120x40", snapshotApp(t, app))
}

// cursorToMOATRow moves the library cursor down until the selected item's
// Name matches `wantName`. The library sorts its rows, so position-based
// navigation is fragile — search by name and fail fast if not found.
func cursorToMOATRow(t *testing.T, app App, wantName string) App {
	t.Helper()
	for step := 0; step < 10; step++ {
		if sel := app.library.currentMetaItem(); sel != nil && sel.Name == wantName {
			return app
		}
		m, _ := app.Update(keyPress(tea.KeyDown))
		app = m.(App)
	}
	t.Fatalf("could not cursor to row with Name=%q within 10 steps", wantName)
	return app
}

func TestGolden_MOAT_LibraryRevokedSelected_80x30(t *testing.T) {
	app := testAppWithMOATItems(t, 80, 30)
	app = cursorToMOATRow(t, app, "revoked-skill")
	requireGolden(t, "moat-library-revoked-selected-80x30", snapshotApp(t, app))
}

func TestGolden_MOAT_LibraryPrivateSelected_80x30(t *testing.T) {
	app := testAppWithMOATItems(t, 80, 30)
	app = cursorToMOATRow(t, app, "private-skill")
	requireGolden(t, "moat-library-private-selected-80x30", snapshotApp(t, app))
}

func TestGolden_MOAT_LibraryRevokedSelected_120x40(t *testing.T) {
	app := testAppWithMOATItems(t, 120, 40)
	app = cursorToMOATRow(t, app, "revoked-skill")
	requireGolden(t, "moat-library-revoked-selected-120x40", snapshotApp(t, app))
}

// Publisher-warn modal goldens. We inject a publisher-revoked
// installResultMsg directly; this mirrors what the wizard dispatches on
// Install confirm.

func publisherWarnApp(t *testing.T, w, h int) App {
	t.Helper()
	app := testAppWithMOATItems(t, w, h)
	// Dispatch the revoked-skill install; handleInstallResult stashes it
	// and opens the danger-mode confirmModal.
	item := app.catalog.Items[1] // revoked-skill
	m, _ := app.Update(installResultMsg{
		item:        item,
		location:    "global",
		method:      "symlink",
		projectRoot: "",
	})
	a := m.(App)
	if !a.confirm.active || a.pendingInstall == nil {
		t.Fatalf("publisher-warn modal did not open; active=%v pendingInstall=%v", a.confirm.active, a.pendingInstall)
	}
	return a
}

func TestGolden_MOAT_PublisherWarnModal_60x20(t *testing.T) {
	app := publisherWarnApp(t, 60, 20)
	requireGolden(t, "moat-publisher-warn-60x20", snapshotApp(t, app))
}

func TestGolden_MOAT_PublisherWarnModal_80x30(t *testing.T) {
	app := publisherWarnApp(t, 80, 30)
	requireGolden(t, "moat-publisher-warn-80x30", snapshotApp(t, app))
}

func TestGolden_MOAT_PublisherWarnModal_120x40(t *testing.T) {
	app := publisherWarnApp(t, 120, 40)
	requireGolden(t, "moat-publisher-warn-120x40", snapshotApp(t, app))
}

// Behavioral spot-checks complementing the goldens — assert substrings
// that MUST be present so a reader knows what the golden is supposed to
// contain even without visual diff.

func TestMOAT_LibraryRowsContainGlyphs(t *testing.T) {
	app := testAppWithMOATItems(t, 120, 40)
	out := snapshotApp(t, app)

	assertContains(t, out, "verified-skill")
	assertContains(t, out, "revoked-skill")
	assertContains(t, out, "private-skill")
	// ✓ must appear for verified row; R for revoked; P for private.
	// Any absence signals trustPrefix() regressed.
	assertContains(t, out, "\u2713")           // ✓ for verified / registry-attested
	assertContains(t, out, "R  revoked-skill") // revoked glyph + 2-space gutter + name
	assertContains(t, out, "P private-skill")  // private glyph adjacent to row name
}

func TestMOAT_RevokedMetapanelShowsCollapsedSummary(t *testing.T) {
	app := testAppWithMOATItems(t, 120, 40)
	app = cursorToMOATRow(t, app, "revoked-skill")
	out := snapshotApp(t, app)

	assertContains(t, out, "Trust:")
	assertContains(t, out, "Revoked")
	// Reason (and all other revocation details) are surfaced by the Trust
	// Inspector modal, not the metapanel — keeping them in the panel
	// would re-introduce the Visibility column floating with reason
	// length. The inspector is discoverable via the [t] help-bar hint.
	assertNotContains(t, out, "key compromise")
	assertNotContains(t, out, "RECALLED")
	assertNotContains(t, out, "ops@example.com")
	assertNotContains(t, out, "example.com/revocation/123")
	assertNotContains(t, out, "[t] Inspect trust")
}

func TestMOAT_RevokedInspectorSurfacesFullDetails(t *testing.T) {
	app := testAppWithMOATItems(t, 120, 40)
	app = cursorToMOATRow(t, app, "revoked-skill")

	// Press [t] to open the Trust Inspector for the revoked row. The
	// library emits libraryTrustInspectMsg via a tea.Cmd, which must be
	// executed and re-dispatched to App.Update so the inspector opens.
	m, cmd := app.Update(keyRune('t'))
	app = m.(App)
	if cmd != nil {
		m, _ = app.Update(cmd())
		app = m.(App)
	}

	if !app.trustInspector.active {
		t.Fatalf("expected trust inspector to open after [t]")
	}

	out := snapshotApp(t, app)
	assertContains(t, out, "Trust: revoked-skill")
	assertContains(t, out, "Revoked")
	assertContains(t, out, "key compromise")
	assertContains(t, out, "ops@example.com")
	assertContains(t, out, "example.com/revocation/123")
}

func TestMOAT_PrivateMetapanelShowsVisibilityChip(t *testing.T) {
	app := testAppWithMOATItems(t, 120, 40)
	app = cursorToMOATRow(t, app, "private-skill")
	out := snapshotApp(t, app)

	assertContains(t, out, "Trust:")
	// Metapanel now shows only the short tier label; the long descriptor
	// is reserved for the Trust Inspector drill-down.
	assertContains(t, out, "Signed")
	assertNotContains(t, out, "Verified (registry-attested)")
	assertContains(t, out, "Visibility:")
	assertContains(t, out, "Private")
}

func TestMOAT_PublisherWarnModalContainsRevocationDetails(t *testing.T) {
	app := publisherWarnApp(t, 120, 40)
	out := snapshotApp(t, app)

	assertContains(t, out, "revoked-skill")
	assertContains(t, out, "publisher has revoked")
	assertContains(t, out, "Reason: key compromise")
	assertContains(t, out, "Revoked by: ops@example.com")
	assertContains(t, out, "Install anyway")
	assertContains(t, out, "Cancel")
}

// TestMOAT_LibraryDetail_TrustKey_OpensInspector is a regression test for a
// bug where [t] on library-detail mode (file-tree + preview drilled-in view)
// was unrouted — updateDetail dispatched straight to the focused pane's key
// handler, so the Trust Inspector was unreachable via keyboard once you had
// drilled in. The fix routes [t] at the mode level before pane dispatch,
// matching the browse-mode handler (library.go:156) and the detail-mode
// mouse handler (library.go:345).
func TestMOAT_LibraryDetail_TrustKey_OpensInspector(t *testing.T) {
	app := testAppWithMOATItems(t, 120, 40)
	app = cursorToMOATRow(t, app, "revoked-skill")

	// Drill in with Enter — this is how the user reaches library-detail.
	m, cmd := app.Update(keyPress(tea.KeyEnter))
	app = m.(App)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			m, _ = app.Update(msg)
			app = m.(App)
		}
	}
	if app.library.mode != libraryDetail {
		t.Fatalf("precondition: expected library.mode=libraryDetail after Enter, got %d", app.library.mode)
	}

	// Press [t]; updateDetail emits libraryTrustInspectMsg via a tea.Cmd,
	// which App.Update dispatches to trustInspector.OpenForItem.
	m, cmd = app.Update(keyRune('t'))
	app = m.(App)
	if cmd != nil {
		m, _ = app.Update(cmd())
		app = m.(App)
	}

	if !app.trustInspector.active {
		t.Fatalf("expected trust inspector active after [t] on library-detail")
	}
	out := snapshotApp(t, app)
	// Inspector surfaces the full revocation breakdown — detail-mode [t] must
	// open the SAME inspector as browse-mode [t], carrying detailItem as the
	// subject.
	assertContains(t, out, "Trust: revoked-skill")
	assertContains(t, out, "key compromise")
}

// TestMOAT_ContentTab_TrustKey_OpensInspector is a regression test for the
// content tabs (Skills/Commands/Rules). Before the fix, [t] was only handled
// by the library model — the explorer (used by content tabs) had no route,
// so pressing [t] while browsing Skills did nothing. The fix adds an
// explorerTrustInspectMsg parallel to libraryTrustInspectMsg and dispatches
// from both updateBrowseKeys and updateDetailKeys.
func TestMOAT_ContentTab_TrustKey_OpensInspector(t *testing.T) {
	app := testAppWithMOATItems(t, 120, 40)

	// Switch to Content group (key '2'). It defaults to Skills sub-tab,
	// which contains all four MOAT fixture items.
	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	app = m.(App)
	if got := app.topBar.ActiveGroupLabel(); got != "Content" {
		t.Fatalf("precondition: expected active group Content, got %q", got)
	}

	// Press [t] on whichever item is currently focused. The first row
	// under Skills is one of the MOAT items; we don't care which — the
	// assertion is that SOME inspector opens for an item.
	m, cmd = app.Update(keyRune('t'))
	app = m.(App)
	if cmd != nil {
		m, _ = app.Update(cmd())
		app = m.(App)
	}

	if !app.trustInspector.active {
		t.Fatalf("expected trust inspector active after [t] on Content > Skills")
	}
	if app.trustInspector.scope != trustInspectorItem {
		t.Errorf("expected inspector scope=trustInspectorItem, got %v", app.trustInspector.scope)
	}
}

// TestMOAT_ContentTab_Detail_TrustKey_OpensInspector covers the parallel
// library-detail bug in the explorer: once drilled into an item on a content
// tab, [t] must still open the Trust Inspector regardless of whether the
// file tree or preview pane is focused.
func TestMOAT_ContentTab_Detail_TrustKey_OpensInspector(t *testing.T) {
	app := testAppWithMOATItems(t, 120, 40)

	m, cmd := app.Update(keyRune('2'))
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	app = m.(App)

	// Drill into the focused Content-tab row.
	m, cmd = app.Update(keyPress(tea.KeyEnter))
	app = m.(App)
	if cmd != nil {
		if msg := cmd(); msg != nil {
			m, _ = app.Update(msg)
			app = m.(App)
		}
	}
	if app.explorer.mode != explorerDetail {
		t.Fatalf("precondition: expected explorer.mode=explorerDetail after Enter, got %d", app.explorer.mode)
	}

	m, cmd = app.Update(keyRune('t'))
	app = m.(App)
	if cmd != nil {
		m, _ = app.Update(cmd())
		app = m.(App)
	}

	if !app.trustInspector.active {
		t.Fatalf("expected trust inspector active after [t] on explorer-detail")
	}
}

// Guard against TrustTier constant removal by referencing every value.
// If a future refactor drops one, this block fails to compile.
var _ = []catalog.TrustTier{
	catalog.TrustTierUnknown,
	catalog.TrustTierUnsigned,
	catalog.TrustTierSigned,
	catalog.TrustTierDualAttested,
}
