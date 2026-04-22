package tui

// Integration tests for the TUI install-gate adapter (bead syllago-u0jna).
//
// These tests construct an App with a hand-built *moat.GateInputs whose
// Manifest + RevocationSet encode the specific condition each MOATGateDecision
// depends on, then drive the install through handleInstallResult /
// handleConfirmResult and assert the resulting App state + confirm modal
// / toast output.
//
// Why integration-level (through Update) rather than unit-level on
// evaluateInstallGate:
//   - The gate's value is the decision-to-UI mapping, not the decision
//     itself (which is already exhaustively tested in installer/moat_gate_test.go).
//   - Feeding the real msg path exercises the App.pendingGateKind /
//     pendingGateRegistryURL stashing, which is what keeps
//     handleConfirmResult coherent across rescan.

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

const (
	testRegistryName = "moat-registry"
	testRegistryURL  = "https://registry.example.com/manifest.json"
	testContentHash  = "sha256:3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b"
)

// gateFixture bundles the inputs a single gate test needs. The zero value
// represents a verified-only manifest with no revocations and no private
// items — subtests flip fields to trigger specific decisions.
type gateFixture struct {
	contentHash      string // defaults to testContentHash
	rekorLogIndex    *int64 // nil = unsigned; set a pointer for signed/dual-attested
	privateRepo      bool
	revocationSource string // "" | RevocationSourceRegistry | RevocationSourcePublisher
	revocationReason string
	lockfileRevoked  bool           // if true, lockfile.RevokedHashes includes contentHash
	minTier          moat.TrustTier // App.moatMinTier override; zero = TrustTierUnsigned
	session          *moat.Session  // nil = fresh NewSession
	signingProfile   *moat.SigningProfile
}

// gateTestApp builds an App with a hand-crafted GateInputs reflecting the
// fixture and returns both the app and the item that will be fed to the
// install handler. The item's Registry matches the fixture manifest so
// evaluateInstallGate finds it.
func gateTestApp(t *testing.T, fx gateFixture) (App, catalog.ContentItem) {
	t.Helper()

	hash := fx.contentHash
	if hash == "" {
		hash = testContentHash
	}

	entry := moat.ContentEntry{
		Name:           "dangerous-skill",
		Type:           "skill",
		ContentHash:    hash,
		AttestedAt:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		RekorLogIndex:  fx.rekorLogIndex,
		PrivateRepo:    fx.privateRepo,
		SigningProfile: fx.signingProfile,
	}
	manifest := &moat.Manifest{
		SchemaVersion: 1,
		ManifestURI:   testRegistryURL,
		Name:          testRegistryName,
		UpdatedAt:     time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC),
		Content:       []moat.ContentEntry{entry},
	}
	if fx.revocationSource != "" {
		manifest.Revocations = []moat.Revocation{{
			ContentHash: hash,
			Reason:      fx.revocationReason,
			DetailsURL:  "https://example.com/recall/" + entry.Name,
			Source:      fx.revocationSource,
		}}
	}

	revSet := moat.NewRevocationSet()
	revSet.AddFromManifest(manifest, testRegistryURL)

	lf := moat.NewLockfile()
	if fx.lockfileRevoked {
		lf.RevokedHashes = append(lf.RevokedHashes, hash)
	}

	session := fx.session
	if session == nil {
		session = moat.NewSession()
	}

	app := testApp(t)
	app.moatGate = &moat.GateInputs{
		RevSet:       revSet,
		Manifests:    map[string]*moat.Manifest{testRegistryName: manifest},
		ManifestURIs: map[string]string{testRegistryName: testRegistryURL},
	}
	app.moatLockfile = lf
	app.moatSession = session
	app.moatMinTier = fx.minTier

	item := catalog.ContentItem{
		Name:     entry.Name,
		Type:     catalog.Skills,
		Path:     "/tmp/fake",
		Registry: testRegistryName,
	}

	return app, item
}

// currentToastText returns the message of the currently visible toast, or ""
// when no toast is visible. Centralized so tests don't reach into the queue
// directly and stay resilient to toastModel internals.
func currentToastText(a App) string {
	entry := a.toast.Current()
	if entry == nil {
		return ""
	}
	return entry.message
}

// confirmModalResult simulates the real modal-close flow. In production the
// confirmModal closes itself (via result()) before emitting confirmResultMsg;
// tests that dispatch the msg directly must replicate that or subsequent
// Update calls will still see a stale confirm.active=true.
func confirmModalResult(t *testing.T, app App, confirmed bool, item catalog.ContentItem) (App, tea.Cmd) {
	t.Helper()
	app.confirm.Close()
	m, cmd := app.Update(confirmResultMsg{confirmed: confirmed, item: item})
	return m.(App), cmd
}

func gateInstallMsg(item catalog.ContentItem) installResultMsg {
	return installResultMsg{
		item:     item,
		provider: provider.Provider{Name: "Claude Code", Slug: "claude-code"},
		location: "global",
		method:   installer.MethodSymlink,
	}
}

// --- Decision coverage ---

func TestInstallGate_ProceedDispatchesImmediately(t *testing.T) {
	app, item := gateTestApp(t, gateFixture{})
	m, cmd := app.Update(gateInstallMsg(item))
	a := m.(App)

	if a.pendingInstall != nil || a.pendingInstallAll != nil {
		t.Error("expected no stashed install for Proceed decision")
	}
	if a.confirm.active {
		t.Error("expected no modal for Proceed decision")
	}
	if cmd == nil {
		t.Error("expected install cmd to dispatch immediately")
	}
}

func TestInstallGate_HardBlockToastsAndAborts(t *testing.T) {
	app, item := gateTestApp(t, gateFixture{
		revocationSource: moat.RevocationSourceRegistry,
		revocationReason: "malicious",
	})
	m, _ := app.Update(gateInstallMsg(item))
	a := m.(App)

	if a.confirm.active {
		t.Error("hard-block must NOT open a modal (no operator override per G-15)")
	}
	if a.pendingInstall != nil {
		t.Error("hard-block must NOT stash an install")
	}
	// Error toasts return nil cmd by design (no auto-dismiss); visible
	// state is what the user actually sees, so assert that instead.
	if !a.toast.visible {
		t.Error("expected toast visible after hard-block")
	}
	if !strings.Contains(currentToastText(a), "Refused") {
		t.Errorf("expected toast to mention refusal; got %q", currentToastText(a))
	}
}

func TestInstallGate_LockfileArchivalRevocationHardBlocks(t *testing.T) {
	// Archival revocation is permanent per G-15 — even with no live
	// revocation in the manifest, the lockfile alone must hard-block.
	app, item := gateTestApp(t, gateFixture{lockfileRevoked: true})
	m, _ := app.Update(gateInstallMsg(item))
	a := m.(App)

	if a.confirm.active {
		t.Error("archival revocation must NOT open a modal")
	}
	if !a.toast.visible {
		t.Error("expected toast visible for archival revocation")
	}
}

func TestInstallGate_PublisherWarnOpensModal(t *testing.T) {
	app, item := gateTestApp(t, gateFixture{
		revocationSource: moat.RevocationSourcePublisher,
		revocationReason: "deprecated",
	})
	m, cmd := app.Update(gateInstallMsg(item))
	a := m.(App)

	if cmd != nil {
		t.Error("expected NO install cmd (install stashed pending confirm)")
	}
	if a.pendingInstall == nil {
		t.Fatal("expected pendingInstall stashed")
	}
	if a.pendingGateKind != gateKindPublisherWarn {
		t.Errorf("pendingGateKind = %v, want publisher-warn", a.pendingGateKind)
	}
	if a.pendingGateRegistryURL != testRegistryURL {
		t.Errorf("pendingGateRegistryURL = %q, want %q", a.pendingGateRegistryURL, testRegistryURL)
	}
	if a.pendingGateContentHash != testContentHash {
		t.Errorf("pendingGateContentHash = %q, want %q", a.pendingGateContentHash, testContentHash)
	}
	if !a.confirm.active {
		t.Error("expected confirm modal active")
	}
	if !a.confirm.danger {
		t.Error("expected danger border for publisher-warn modal")
	}
	if !strings.Contains(a.confirm.body, "deprecated") {
		t.Errorf("body missing live-manifest reason; got %q", a.confirm.body)
	}
}

func TestInstallGate_PrivatePromptOpensDistinctModal(t *testing.T) {
	app, item := gateTestApp(t, gateFixture{privateRepo: true})
	m, cmd := app.Update(gateInstallMsg(item))
	a := m.(App)

	if cmd != nil {
		t.Error("expected NO install cmd (stashed pending private confirm)")
	}
	if a.pendingGateKind != gateKindPrivatePrompt {
		t.Errorf("pendingGateKind = %v, want private-prompt", a.pendingGateKind)
	}
	if a.confirm.danger {
		t.Error("private-prompt modal must NOT be in danger mode")
	}
	if !strings.Contains(a.confirm.title, "private") {
		t.Errorf("expected 'private' in title; got %q", a.confirm.title)
	}
}

func TestInstallGate_TierBelowPolicyToastsAndAborts(t *testing.T) {
	// Unsigned entry, policy floor = Signed → gate refuses.
	app, item := gateTestApp(t, gateFixture{minTier: moat.TrustTierSigned})
	m, _ := app.Update(gateInstallMsg(item))
	a := m.(App)

	if a.confirm.active {
		t.Error("tier-below-policy must NOT open a modal (no interactive recovery)")
	}
	// Error toasts return nil cmd by design; assert visibility instead.
	if !a.toast.visible {
		t.Error("expected toast visible after tier refusal")
	}
	toastMsg := currentToastText(a)
	if !strings.Contains(toastMsg, "UNSIGNED") || !strings.Contains(toastMsg, "SIGNED") {
		t.Errorf("expected toast to mention observed vs min tier; got %q", toastMsg)
	}
}

// --- Confirm flow round-trips ---

func TestInstallGate_PublisherWarnConfirmDispatchesAndSuppressesNextTime(t *testing.T) {
	app, item := gateTestApp(t, gateFixture{
		revocationSource: moat.RevocationSourcePublisher,
		revocationReason: "deprecated",
	})
	m, _ := app.Update(gateInstallMsg(item))
	app = m.(App)

	// Confirm the modal.
	a, cmd := confirmModalResult(t, app, true, item)

	if cmd == nil {
		t.Error("expected install cmd after confirm")
	}
	if a.pendingInstall != nil {
		t.Error("expected pendingInstall cleared after confirm")
	}
	if a.pendingGateKind != gateKindNone {
		t.Error("expected pendingGateKind cleared after confirm")
	}

	// Second install of the same item in the same session — modal must NOT
	// reopen (publisher-warn acknowledgement persists per ADR 0007 G-8).
	m, cmd = a.Update(gateInstallMsg(item))
	a = m.(App)
	if a.confirm.active {
		t.Error("expected second install to skip modal (session-memory suppression)")
	}
	if cmd == nil {
		t.Error("expected second install cmd to dispatch immediately")
	}
}

func TestInstallGate_PublisherWarnCancelClearsStash(t *testing.T) {
	app, item := gateTestApp(t, gateFixture{
		revocationSource: moat.RevocationSourcePublisher,
		revocationReason: "deprecated",
	})
	m, _ := app.Update(gateInstallMsg(item))
	app = m.(App)

	a, _ := confirmModalResult(t, app, false, item)

	if a.pendingInstall != nil {
		t.Error("expected pendingInstall cleared after cancel")
	}
	if a.pendingGateKind != gateKindNone {
		t.Error("expected pendingGateKind cleared after cancel")
	}
	if !strings.Contains(currentToastText(a), "recalled") {
		t.Errorf("expected cancel toast to mention 'recalled'; got %q", currentToastText(a))
	}
}

func TestInstallGate_PrivatePromptConfirmDispatchesAndSuppressesNextTime(t *testing.T) {
	app, item := gateTestApp(t, gateFixture{privateRepo: true})
	m, _ := app.Update(gateInstallMsg(item))
	app = m.(App)

	a, cmd := confirmModalResult(t, app, true, item)
	if cmd == nil {
		t.Error("expected install cmd after private-prompt confirm")
	}
	if a.pendingGateKind != gateKindNone {
		t.Error("expected pendingGateKind cleared after confirm")
	}

	// Second install — private-confirm memory keyed distinctly from
	// publisher-warn (G-10 separate key prefix).
	m, _ = a.Update(gateInstallMsg(item))
	a = m.(App)
	if a.confirm.active {
		t.Error("expected second install to skip private-prompt modal")
	}
}

func TestInstallGate_PrivatePromptCancelToastDistinguishesFromPublisher(t *testing.T) {
	app, item := gateTestApp(t, gateFixture{privateRepo: true})
	m, _ := app.Update(gateInstallMsg(item))
	app = m.(App)

	a, _ := confirmModalResult(t, app, false, item)

	if !strings.Contains(currentToastText(a), "private") {
		t.Errorf("expected private-source cancel toast to mention 'private'; got %q", currentToastText(a))
	}
}

// --- Install-to-all parity ---

func TestInstallGate_InstallAll_PublisherWarnGates(t *testing.T) {
	app, item := gateTestApp(t, gateFixture{
		revocationSource: moat.RevocationSourcePublisher,
		revocationReason: "key compromise",
	})
	msg := installAllResultMsg{
		item: item,
		providers: []provider.Provider{
			{Name: "Claude Code", Slug: "claude-code"},
			{Name: "Gemini CLI", Slug: "gemini-cli"},
		},
	}
	m, cmd := app.Update(msg)
	a := m.(App)

	if cmd != nil {
		t.Error("expected no install-all cmd (stashed pending confirm)")
	}
	if a.pendingInstallAll == nil {
		t.Fatal("expected pendingInstallAll stashed")
	}
	if a.pendingInstall != nil {
		t.Error("expected pendingInstall nil when pendingInstallAll set")
	}
	if a.pendingGateKind != gateKindPublisherWarn {
		t.Errorf("pendingGateKind = %v, want publisher-warn", a.pendingGateKind)
	}
}

func TestInstallGate_InstallAll_ConfirmDispatchesBatch(t *testing.T) {
	app, item := gateTestApp(t, gateFixture{
		revocationSource: moat.RevocationSourcePublisher,
	})
	msg := installAllResultMsg{
		item:      item,
		providers: []provider.Provider{{Name: "Claude Code", Slug: "claude-code"}},
	}
	m, _ := app.Update(msg)
	app = m.(App)

	a, cmd := confirmModalResult(t, app, true, item)
	if cmd == nil {
		t.Error("expected batch install cmd on confirm")
	}
	if a.pendingInstallAll != nil {
		t.Error("expected pendingInstallAll cleared")
	}
}

// --- Non-MOAT items bypass the gate entirely ---

func TestInstallGate_NonMOATItemProceedsDirectly(t *testing.T) {
	// App has GateInputs configured, but item has no Registry — gate is
	// not applicable, install should proceed on the legacy path.
	app, _ := gateTestApp(t, gateFixture{})
	nonMOATItem := catalog.ContentItem{
		Name: "local-only",
		Type: catalog.Skills,
		Path: "/tmp/fake",
	}
	m, cmd := app.Update(gateInstallMsg(nonMOATItem))
	a := m.(App)

	if a.confirm.active {
		t.Error("expected no modal for non-MOAT item")
	}
	if cmd == nil {
		t.Error("expected install cmd for non-MOAT item")
	}
}

func TestInstallGate_ItemWithUnknownRegistryProceedsDirectly(t *testing.T) {
	// Registry name does not match any manifest in GateInputs — gate
	// bypass path (e.g., item from a non-MOAT registry or a stale catalog).
	app, _ := gateTestApp(t, gateFixture{})
	ghostItem := catalog.ContentItem{
		Name:     "ghost",
		Type:     catalog.Skills,
		Path:     "/tmp/fake",
		Registry: "unknown-registry",
	}
	m, cmd := app.Update(gateInstallMsg(ghostItem))
	a := m.(App)

	if a.confirm.active {
		t.Error("expected no modal for unknown registry")
	}
	if cmd == nil {
		t.Error("expected install cmd for unknown registry")
	}
}

// --- Window-size sanity (defense against test-helper drift) ---

func TestInstallGate_GateStateSurvivesWindowResize(t *testing.T) {
	app, item := gateTestApp(t, gateFixture{
		revocationSource: moat.RevocationSourcePublisher,
	})
	m, _ := app.Update(gateInstallMsg(item))
	app = m.(App)
	if !app.confirm.active {
		t.Fatal("setup: expected modal active")
	}
	m, _ = app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a := m.(App)
	if !a.confirm.active {
		t.Error("expected modal to remain active across resize")
	}
	if a.pendingInstall == nil {
		t.Error("expected pendingInstall preserved across resize")
	}
}
