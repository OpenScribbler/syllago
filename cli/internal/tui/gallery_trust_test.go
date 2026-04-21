package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
)

// Note on mouse tests in this file: App.View() calls zone.Scan() internally,
// so DON'T wrap it with scanZones() — that would double-scan and strip all
// zone markers. Just render once and sleep 20ms to let bubblezone's async
// worker complete, matching the pattern in install_mouse_test.go.

// Covers bead syllago-jc3s7: registry-scoped Trust Inspector wiring.
// These tests pin behaviour for aggregate glyph rules, sidebar trust-section
// layout, [t] key routing, and the click-anywhere zone on the trust block.

func TestRegistryTrustGlyph_States(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		rt   *catalog.RegistryTrust
		want string
	}{
		{"nil pointer → empty", nil, ""},
		{"no verified, no recall, fresh → empty",
			&catalog.RegistryTrust{Staleness: "Fresh"}, ""},
		{"verified items, fresh → check",
			&catalog.RegistryTrust{Staleness: "Fresh", VerifiedItems: 3}, "\u2713"},
		{"recalled wins over verified",
			&catalog.RegistryTrust{Staleness: "Fresh", VerifiedItems: 5, RecalledItems: 1}, "R"},
		{"stale → clock",
			&catalog.RegistryTrust{Staleness: "Stale", VerifiedItems: 2}, "\u23f0"},
		{"expired → clock",
			&catalog.RegistryTrust{Staleness: "Expired"}, "\u23f0"},
		{"missing → clock",
			&catalog.RegistryTrust{Staleness: "Missing"}, "\u23f0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := registryTrustGlyph(tc.rt)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRegistryTrustSummaryFrom_NilReturnsZero(t *testing.T) {
	t.Parallel()
	got := registryTrustSummaryFrom(nil)
	if got != (RegistryTrustSummary{}) {
		t.Errorf("nil input should yield zero summary, got %+v", got)
	}
}

func TestRegistryTrustSummaryFrom_CopiesFields(t *testing.T) {
	t.Parallel()
	fetched := time.Date(2026, 4, 21, 15, 30, 0, 0, time.UTC)
	rt := &catalog.RegistryTrust{
		Name:          "moat-registry",
		Tier:          catalog.TrustTierSigned,
		Issuer:        "https://fulcio.sigstore.dev",
		Subject:       "ops@example.com",
		Operator:      "Example Corp",
		ManifestURI:   "https://example.com/manifest.json",
		FetchedAt:     fetched,
		Staleness:     "Fresh",
		TotalItems:    7,
		VerifiedItems: 4,
		RecalledItems: 1,
		PrivateItems:  2,
	}
	got := registryTrustSummaryFrom(rt)

	if got.Name != rt.Name || got.Tier != rt.Tier || got.Issuer != rt.Issuer ||
		got.Subject != rt.Subject || got.Operator != rt.Operator ||
		got.ManifestURI != rt.ManifestURI || got.Staleness != rt.Staleness ||
		got.TotalItems != rt.TotalItems || got.VerifiedItems != rt.VerifiedItems ||
		got.RecalledItems != rt.RecalledItems || got.PrivateItems != rt.PrivateItems {
		t.Errorf("field mismatch: got %+v, rt %+v", got, rt)
	}
	// FetchedAt is formatted — just assert non-empty and contains the date.
	if !strings.Contains(got.FetchedAt, "2026-04-21") {
		t.Errorf("FetchedAt = %q, want substring 2026-04-21", got.FetchedAt)
	}
}

func TestRegistryTrustSummaryFrom_ZeroTimeOmitsTimestamp(t *testing.T) {
	t.Parallel()
	rt := &catalog.RegistryTrust{Name: "r"}
	got := registryTrustSummaryFrom(rt)
	if got.FetchedAt != "" {
		t.Errorf("zero time should yield empty FetchedAt, got %q", got.FetchedAt)
	}
}

func TestContentsSidebar_TrustLines_AbsentVsPresent(t *testing.T) {
	t.Parallel()
	var m contentsSidebarModel
	if got := m.trustLines(); got != 0 {
		t.Errorf("no trust: lines=%d, want 0", got)
	}
	m.trust = &catalog.RegistryTrust{Name: "r", Staleness: "Fresh"}
	if got := m.trustLines(); got != 7 {
		t.Errorf("with trust: lines=%d, want 7", got)
	}
}

func TestContentsSidebar_RenderTrustSection_Content(t *testing.T) {
	t.Parallel()
	m := contentsSidebarModel{width: 80, height: 20}
	m.trust = &catalog.RegistryTrust{
		Name:          "moat-registry",
		Tier:          catalog.TrustTierSigned,
		Subject:       "ops@example.com",
		Operator:      "Example Corp",
		Staleness:     "Fresh",
		TotalItems:    5,
		VerifiedItems: 3,
		RecalledItems: 0,
	}
	lines := m.renderTrustSection()
	if len(lines) != 7 {
		t.Fatalf("renderTrustSection produced %d lines, want 7", len(lines))
	}
	// Zone markers are invisible characters; use zone-agnostic assertions
	// on the visible labels.
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"Trust", "Tier:", "Issuer:", "Status:", "Items:", "[t] Inspect trust"} {
		if !strings.Contains(joined, want) {
			t.Errorf("trust section missing %q; lines:\n%s", want, joined)
		}
	}
	// Operator preferred over Subject when present.
	if !strings.Contains(joined, "Example Corp") {
		t.Errorf("trust section should prefer Operator %q; got:\n%s", "Example Corp", joined)
	}
	// Items summary must include all three counts.
	if !strings.Contains(joined, "5 total") || !strings.Contains(joined, "3 verified") || !strings.Contains(joined, "0 recalled") {
		t.Errorf("items summary incomplete; got:\n%s", joined)
	}
}

func TestContentsSidebar_RenderTrustSection_StaleUsesDangerLabel(t *testing.T) {
	t.Parallel()
	m := contentsSidebarModel{width: 80, height: 20}
	m.trust = &catalog.RegistryTrust{Name: "r", Staleness: "Stale"}
	lines := m.renderTrustSection()
	if !strings.Contains(strings.Join(lines, "\n"), "Stale") {
		t.Errorf("stale registry should render Stale label")
	}
}

// testAppWithMOATRegistry builds an App with a single MOAT registry card so
// [t] and mouse tests can exercise the gallery trust path without reaching
// into enrichment machinery.
func testAppWithMOATRegistry(t *testing.T, w, h int) App {
	t.Helper()
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "alpha-skill", Type: catalog.Skills, Source: "moat-registry",
				Registry: "moat-registry", Files: []string{"SKILL.md"},
				TrustTier: catalog.TrustTierSigned},
		},
		RegistryTrusts: map[string]*catalog.RegistryTrust{
			"moat-registry": {
				Name: "moat-registry", Tier: catalog.TrustTierSigned,
				Operator: "Example Corp", Staleness: "Fresh",
				TotalItems: 1, VerifiedItems: 1,
			},
		},
	}
	regs := []catalog.RegistrySource{{Name: "moat-registry", Path: "/tmp/fake"}}
	app := NewApp(cat, nil, "0.0.0-test", false, regs, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: w, Height: h})
	a := m.(App)
	// Navigate to Collections > Registries (single Tab — Library is [1], Registries next).
	m, cmd := a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	return m.(App)
}

func TestGalleryKeys_TrustOpensInspector(t *testing.T) {
	// Not parallel — inspector state is App-scoped and tests assert it.
	app := testAppWithMOATRegistry(t, 120, 40)

	// Press [t] on the registries tab; should dispatch registryTrustInspectMsg
	// → app handler → trustInspector.active.
	m, cmd := app.Update(keyRune('t'))
	app = m.(App)
	if cmd == nil {
		t.Fatal("expected tea.Cmd from [t]; got nil")
	}
	m, _ = app.Update(cmd())
	app = m.(App)
	if !app.trustInspector.active {
		t.Fatal("trust inspector should be active after [t] on registries tab")
	}
	if app.trustInspector.scope != trustInspectorRegistry {
		t.Errorf("scope = %v, want trustInspectorRegistry", app.trustInspector.scope)
	}
	if !strings.Contains(app.trustInspector.title, "moat-registry") {
		t.Errorf("inspector title missing registry name: %q", app.trustInspector.title)
	}
}

func TestGalleryKeys_TrustIsNoopForNonMOAT(t *testing.T) {
	// Registry card without trust pointer — [t] must not dispatch.
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "alpha", Type: catalog.Skills, Source: "git-registry",
				Registry: "git-registry", Files: []string{"SKILL.md"}},
		},
		// No RegistryTrusts entry.
	}
	regs := []catalog.RegistrySource{{Name: "git-registry", Path: "/tmp/fake"}}
	app := NewApp(cat, nil, "0.0.0-test", false, regs, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a := m.(App)
	m, cmd := a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)

	m, cmd = a.Update(keyRune('t'))
	a = m.(App)
	if cmd != nil {
		// The [t] handler returns nil when trust is absent. If a cmd came
		// through, it must not be the trust inspect message.
		if msg := cmd(); msg != nil {
			if _, isTrust := msg.(registryTrustInspectMsg); isTrust {
				t.Fatal("non-MOAT registry should not emit registryTrustInspectMsg")
			}
		}
	}
	if a.trustInspector.active {
		t.Error("inspector should stay closed for non-MOAT registry")
	}
}

// Golden tests pin the visual output of the MOAT-enriched Registries gallery
// at the three standard sizes. Covers: card glyph, preview panel Trust
// section, items breakdown. Built on testAppWithMOATRegistry so fixtures stay
// colocated with the behavior tests above.

func TestGolden_MOAT_RegistriesGallery_60x20(t *testing.T) {
	app := testAppWithMOATRegistry(t, 60, 20)
	requireGolden(t, "moat-registries-60x20", snapshotApp(t, app))
}

func TestGolden_MOAT_RegistriesGallery_80x30(t *testing.T) {
	app := testAppWithMOATRegistry(t, 80, 30)
	requireGolden(t, "moat-registries-80x30", snapshotApp(t, app))
}

func TestGolden_MOAT_RegistriesGallery_120x40(t *testing.T) {
	app := testAppWithMOATRegistry(t, 120, 40)
	requireGolden(t, "moat-registries-120x40", snapshotApp(t, app))
}

// MOAT registries gallery with a recalled item — exercises the R glyph path.
func TestGolden_MOAT_RegistriesGallery_Recalled_120x40(t *testing.T) {
	cat := &catalog.Catalog{
		Items: []catalog.ContentItem{
			{Name: "alpha-skill", Type: catalog.Skills, Source: "moat-registry",
				Registry: "moat-registry", Files: []string{"SKILL.md"},
				TrustTier: catalog.TrustTierSigned},
			{Name: "bad-skill", Type: catalog.Skills, Source: "moat-registry",
				Registry: "moat-registry", Files: []string{"SKILL.md"},
				TrustTier: catalog.TrustTierSigned, Recalled: true,
				RecallSource: "publisher", RecallReason: "key compromise"},
		},
		RegistryTrusts: map[string]*catalog.RegistryTrust{
			"moat-registry": {
				Name: "moat-registry", Tier: catalog.TrustTierSigned,
				Operator: "Example Corp", Staleness: "Fresh",
				TotalItems: 2, VerifiedItems: 1, RecalledItems: 1,
			},
		},
	}
	regs := []catalog.RegistrySource{{Name: "moat-registry", Path: "/tmp/fake"}}
	app := NewApp(cat, nil, "0.0.0-test", false, regs, testConfig(), false, "", "")
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a := m.(App)
	m, cmd := a.Update(keyTab)
	if cmd != nil {
		m, _ = m.Update(cmd())
	}
	a = m.(App)
	requireGolden(t, "moat-registries-recalled-120x40", snapshotApp(t, a))
}

func TestGalleryMouse_TrustZoneOpensInspector(t *testing.T) {
	// Not parallel — zone.Scan() uses a singleton manager (see tui-testing.md).
	app := testAppWithMOATRegistry(t, 120, 40)

	// App.View() invokes zone.Scan() internally — calling scanZones() again
	// would strip the markers. Just render once and wait for the async scan
	// worker to finish (see install_mouse_test.go scanZones() for the rationale).
	_ = app.View()
	time.Sleep(20 * time.Millisecond)

	rt := zone.Get("registry-trust")
	if rt == nil || rt.IsZero() {
		t.Fatal("registry-trust zone not registered after App.View render")
	}
	m, cmd := app.Update(mouseClick(rt.StartX, rt.StartY))
	app = m.(App)
	if cmd == nil {
		t.Fatal("click on registry-trust zone produced no cmd")
	}
	m, _ = app.Update(cmd())
	app = m.(App)
	if !app.trustInspector.active {
		t.Error("clicking trust section should open inspector")
	}
}
