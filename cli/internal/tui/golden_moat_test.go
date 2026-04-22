package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
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

func TestGolden_MOAT_LibraryRecalledSelected_80x30(t *testing.T) {
	app := testAppWithMOATItems(t, 80, 30)
	app = cursorToMOATRow(t, app, "recalled-skill")
	requireGolden(t, "moat-library-recalled-selected-80x30", snapshotApp(t, app))
}

func TestGolden_MOAT_LibraryPrivateSelected_80x30(t *testing.T) {
	app := testAppWithMOATItems(t, 80, 30)
	app = cursorToMOATRow(t, app, "private-skill")
	requireGolden(t, "moat-library-private-selected-80x30", snapshotApp(t, app))
}

func TestGolden_MOAT_LibraryRecalledSelected_120x40(t *testing.T) {
	app := testAppWithMOATItems(t, 120, 40)
	app = cursorToMOATRow(t, app, "recalled-skill")
	requireGolden(t, "moat-library-recalled-selected-120x40", snapshotApp(t, app))
}

// Publisher-warn modal goldens. We inject a publisher-revoked
// installResultMsg directly; this mirrors what the wizard dispatches on
// Install confirm.

func publisherWarnApp(t *testing.T, w, h int) App {
	t.Helper()
	app := testAppWithMOATItems(t, w, h)

	// The gate is now the source of truth for publisher-warn branching.
	// Build a minimal GateInputs whose manifest lists the recalled-skill
	// under a publisher-source revocation — this makes PreInstallCheck
	// return MOATGatePublisherWarn and the handler opens the modal.
	const fakeHash = "sha256:" +
		"recalledskill0000000000000000000000000000000000000000000000000000"
	const registryName = "moat-registry"
	const registryURL = "https://moat-registry.example.com/manifest.json"
	manifest := &moat.Manifest{
		SchemaVersion: 1,
		ManifestURI:   registryURL,
		Name:          registryName,
		UpdatedAt:     time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC),
		Content: []moat.ContentEntry{{
			Name:        "recalled-skill",
			Type:        "skill",
			ContentHash: fakeHash,
			AttestedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}},
		Revocations: []moat.Revocation{{
			ContentHash: fakeHash,
			Reason:      "key compromise",
			DetailsURL:  "https://example.com/recall/123",
			Source:      moat.RevocationSourcePublisher,
		}},
	}
	revSet := moat.NewRevocationSet()
	revSet.AddFromManifest(manifest, registryURL)
	app.moatGate = &moat.GateInputs{
		RevSet:       revSet,
		Manifests:    map[string]*moat.Manifest{registryName: manifest},
		ManifestURIs: map[string]string{registryName: registryURL},
	}
	app.moatLockfile = moat.NewLockfile()

	// Dispatch the recalled-skill install; handleInstallResult stashes it
	// and opens the danger-mode confirmModal.
	item := app.catalog.Items[1] // recalled-skill
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
	assertContains(t, out, "recalled-skill")
	assertContains(t, out, "private-skill")
	// ✓ must appear for verified row; R for recalled; P for private.
	// Any absence signals trustPrefix() regressed.
	assertContains(t, out, "\u2713")            // ✓ for verified / registry-attested
	assertContains(t, out, "R  recalled-skill") // recalled glyph + 2-space gutter + name
	assertContains(t, out, "P private-skill")   // private glyph adjacent to row name
}

func TestMOAT_RecalledMetapanelShowsCollapsedSummary(t *testing.T) {
	app := testAppWithMOATItems(t, 120, 40)
	app = cursorToMOATRow(t, app, "recalled-skill")
	out := snapshotApp(t, app)

	assertContains(t, out, "Trust:")
	assertContains(t, out, "Recalled")
	assertContains(t, out, "key compromise")
	// Collapsed layout — banner text stays out; details live in the inspector.
	assertNotContains(t, out, "RECALLED")
	assertNotContains(t, out, "ops@example.com")
	assertNotContains(t, out, "example.com/recall/123")
	// Discoverable affordance so users know how to see full recall details.
	assertContains(t, out, "[t] Inspect trust")
}

func TestMOAT_RecalledInspectorSurfacesFullDetails(t *testing.T) {
	app := testAppWithMOATItems(t, 120, 40)
	app = cursorToMOATRow(t, app, "recalled-skill")

	// Press [t] to open the Trust Inspector for the recalled row. The
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
	assertContains(t, out, "Trust: recalled-skill")
	assertContains(t, out, "Recalled")
	assertContains(t, out, "key compromise")
	assertContains(t, out, "ops@example.com")
	assertContains(t, out, "example.com/recall/123")
}

func TestMOAT_PrivateMetapanelShowsVisibilityChip(t *testing.T) {
	app := testAppWithMOATItems(t, 120, 40)
	app = cursorToMOATRow(t, app, "private-skill")
	out := snapshotApp(t, app)

	assertContains(t, out, "Trust:")
	assertContains(t, out, "Verified (registry-attested)")
	assertContains(t, out, "Visibility:")
	assertContains(t, out, "Private")
}

func TestMOAT_PublisherWarnModalContainsRevocationDetails(t *testing.T) {
	app := publisherWarnApp(t, 120, 40)
	out := snapshotApp(t, app)

	assertContains(t, out, "recalled-skill")
	assertContains(t, out, "publisher has revoked")
	assertContains(t, out, "Reason: key compromise")
	assertContains(t, out, "Issued by: ops@example.com")
	assertContains(t, out, "Install anyway")
	assertContains(t, out, "Cancel")
}

// Guard against TrustTier constant removal by referencing every value.
// If a future refactor drops one, this block fails to compile.
var _ = []catalog.TrustTier{
	catalog.TrustTierUnknown,
	catalog.TrustTierUnsigned,
	catalog.TrustTierSigned,
	catalog.TrustTierDualAttested,
}
