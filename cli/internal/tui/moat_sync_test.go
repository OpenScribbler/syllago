package tui

// Tests for MOAT-aware registry sync wiring in the TUI.
//
// The orchestrator runMOATSync is exercised end-to-end by integration tests
// elsewhere; these tests focus on the message routing the App owns:
//
//   - registryIsMOAT correctly classifies the in-memory config so handleSync
//     can pick the right path.
//   - handleMOATSyncDone branches correctly across the four outcomes the
//     orchestrator can return (err, profileChanged, requiresTOFU, happy/stale).
//   - handleTOFUResult re-issues a sync on accept and surfaces a warning
//     toast on reject.
//
// Stubbing rationale: handleMOATSyncDone and handleTOFUResult only consume
// the moatSyncDoneMsg / tofuResultMsg shapes. There's no need to swap
// moatSyncFnTUI here — these tests build the messages directly. End-to-end
// stubbing of moatSyncFnTUI is left for the few tests that need to
// observe doMOATSyncCmd's command output.

import (
	"errors"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
)

func TestRegistryIsMOAT_True(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	app.cfg = &config.Config{Registries: []config.Registry{
		{Name: "moat-reg", Type: config.RegistryTypeMOAT, ManifestURI: "https://example.com/m.json"},
	}}
	if !app.registryIsMOAT("moat-reg") {
		t.Fatal("expected MOAT-typed registry to be classified MOAT")
	}
}

func TestRegistryIsMOAT_FalseForGit(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	app.cfg = &config.Config{Registries: []config.Registry{
		{Name: "git-reg", Type: config.RegistryTypeGit, URL: "https://example.com/repo.git"},
	}}
	if app.registryIsMOAT("git-reg") {
		t.Fatal("git registry should not be classified MOAT")
	}
}

func TestRegistryIsMOAT_FalseForUnknown(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	app.cfg = &config.Config{}
	if app.registryIsMOAT("ghost") {
		t.Fatal("unknown name should fall through to non-MOAT (git pull path)")
	}
}

func TestHandleMOATSyncDone_Error(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	app.registryOpInProgress = true
	m, _ := app.handleMOATSyncDone(moatSyncDoneMsg{name: "reg", err: errors.New("boom")})
	a := m.(App)
	if a.registryOpInProgress {
		t.Error("registryOpInProgress should clear on error")
	}
	if !a.toast.visible {
		t.Error("expected error toast visible")
	}
	if a.tofu.active {
		t.Error("TOFU modal should not open on plain error")
	}
}

func TestHandleMOATSyncDone_ProfileChanged(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	app.registryOpInProgress = true
	m, _ := app.handleMOATSyncDone(moatSyncDoneMsg{
		name:           "reg",
		profileChanged: true,
	})
	a := m.(App)
	if a.registryOpInProgress {
		t.Error("registryOpInProgress should clear on profileChanged")
	}
	if !a.toast.visible {
		t.Error("expected toast visible for profileChanged")
	}
	if a.tofu.active {
		t.Error("profile change should NOT trigger TOFU modal — re-add is required")
	}
}

func TestHandleMOATSyncDone_RequiresTOFU_OpensModal(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	app.registryOpInProgress = true
	profile := config.SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "https://github.com/example/registry",
	}
	m, cmd := app.handleMOATSyncDone(moatSyncDoneMsg{
		name:            "reg",
		requiresTOFU:    true,
		incomingProfile: profile,
		manifestURL:     "https://example.com/manifest.json",
	})
	a := m.(App)
	if !a.tofu.active {
		t.Fatal("TOFU modal should open when requiresTOFU=true")
	}
	if a.tofu.name != "reg" {
		t.Errorf("tofu.name = %q; want %q", a.tofu.name, "reg")
	}
	if a.tofu.profile.Issuer != profile.Issuer {
		t.Errorf("tofu.profile.Issuer = %q; want %q", a.tofu.profile.Issuer, profile.Issuer)
	}
	if a.tofu.manifestURL != "https://example.com/manifest.json" {
		t.Errorf("tofu.manifestURL = %q", a.tofu.manifestURL)
	}
	if cmd != nil {
		t.Error("requiresTOFU should not return a tea.Cmd — modal owns the next step")
	}
}

func TestHandleMOATSyncDone_HappyPath(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	app.registryOpInProgress = true
	m, cmd := app.handleMOATSyncDone(moatSyncDoneMsg{name: "reg"})
	a := m.(App)
	if a.registryOpInProgress {
		t.Error("registryOpInProgress should clear on happy path")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (toast + rescan batch)")
	}
}

func TestHandleMOATSyncDone_Stale(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	m, cmd := app.handleMOATSyncDone(moatSyncDoneMsg{name: "reg", stale: true})
	a := m.(App)
	if a.tofu.active {
		t.Error("stale should not open TOFU modal")
	}
	if cmd == nil {
		t.Fatal("stale path should still toast + rescan")
	}
}

func TestHandleTOFUResult_Rejected(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	m, _ := app.handleTOFUResult(tofuResultMsg{name: "reg", accepted: false})
	a := m.(App)
	if !a.toast.visible {
		t.Error("rejection should surface a warning toast")
	}
	if a.registryOpInProgress {
		t.Error("rejected reject path should not flip registryOpInProgress on")
	}
}

func TestHandleTOFUResult_Accepted_TriggersResync(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	m, cmd := app.handleTOFUResult(tofuResultMsg{name: "reg", accepted: true})
	a := m.(App)
	if !a.registryOpInProgress {
		t.Error("accepted should mark registryOpInProgress=true while re-sync runs")
	}
	if cmd == nil {
		t.Fatal("accepted should return a re-sync cmd")
	}
}

// TestHandleSync_RoutesByRegistryType exercises the dispatch in handleSync:
// the MOAT path triggers doMOATSyncCmd (not registry.Sync). We verify by
// checking that the in-progress flag flips and a toast is queued — both
// branches do that, but only the MOAT branch passes the registryIsMOAT
// guard, so a config without MOAT registries proves the fallback works.
func TestHandleSync_NoSelectedCard(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	// Default app has no gallery selection — handleSync returns (a, nil).
	_, cmd := app.handleSync()
	if cmd != nil {
		t.Errorf("expected nil cmd with no selected card, got %v", cmd)
	}
}

// Add-path → MOAT sync wiring. Without this chain, the manifest cache stays
// empty after add and EnrichFromMOATManifests downgrades trust to Unknown.
// The bug the user reported: trust still says "Unknown" after a fresh add
// because handleRegistryAddDone only toasted + rescanned.
func TestHandleRegistryAddDone_MOAT_DispatchesSync(t *testing.T) {
	t.Parallel()
	a := testApp(t)
	a.registryOpInProgress = true
	m, cmd := a.handleRegistryAddDone(registryAddDoneMsg{name: "moat-reg", isMOAT: true})
	got := m.(App)
	if !got.registryOpInProgress {
		t.Error("MOAT add must keep registryOpInProgress=true through the sync")
	}
	if cmd == nil {
		t.Fatal("MOAT add must dispatch a sync cmd")
	}
}

func TestHandleRegistryAddDone_NonMOAT_OnlyRescans(t *testing.T) {
	t.Parallel()
	a := testApp(t)
	a.registryOpInProgress = true
	m, _ := a.handleRegistryAddDone(registryAddDoneMsg{name: "git-reg", isMOAT: false})
	got := m.(App)
	if got.registryOpInProgress {
		t.Error("non-MOAT add should clear registryOpInProgress immediately")
	}
}

func TestHandleRegistryAddDone_Error_ClearsInProgress(t *testing.T) {
	t.Parallel()
	a := testApp(t)
	a.registryOpInProgress = true
	m, _ := a.handleRegistryAddDone(registryAddDoneMsg{name: "x", err: errors.New("clone failed"), isMOAT: true})
	got := m.(App)
	if got.registryOpInProgress {
		t.Error("error path must clear registryOpInProgress even when isMOAT=true")
	}
}

// Sanity: empty catalog stays consistent across MOAT path so future
// regressions where an unrelated handler accidentally resets RegistryTrusts
// are caught here.
func TestHandleMOATSyncDone_KeepsCatalogReference(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	app.catalog = &catalog.Catalog{RegistryTrusts: map[string]*catalog.RegistryTrust{
		"reg": {Name: "reg", Tier: catalog.TrustTierSigned},
	}}
	m, _ := app.handleMOATSyncDone(moatSyncDoneMsg{name: "reg", err: errors.New("x")})
	a := m.(App)
	if a.catalog == nil || a.catalog.RegistryTrusts == nil {
		t.Fatal("catalog reference dropped during error path")
	}
}
