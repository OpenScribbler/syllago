package tui

// Unit tests for doMOATInstallCmd — the TUI install path for unstaged MOAT
// items (item.Path == "" / item.Source == registry name). The happy path
// requires real network + sigstore fixtures and is exercised by the CLI
// integration tests in cmd/syllago and the moatinstall package's own suite;
// these tests cover the TUI-specific guard rails:
//
//   - missing registry in cfg → friendly error, no panic
//   - missing manifest entry  → friendly error, no panic
//
// Both branches return an installDoneMsg with err set so the App's existing
// toast pipeline surfaces them — callers expect that shape and never receive
// a raw error or a nil msg.

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestDoMOATInstallCmd_RegistryMissingFromConfig(t *testing.T) {
	app := testApp(t)
	app.cfg = &config.Config{} // empty — no registries

	item := catalog.ContentItem{
		Name:     "missing-skill",
		Type:     catalog.Skills,
		Source:   "ghost-registry", // not in cfg
		Registry: "ghost-registry",
	}
	prov := provider.Provider{Name: "Test", Slug: "test"}

	cmd := app.doMOATInstallCmd(installResultMsg{
		item:     item,
		provider: prov,
		method:   installer.MethodSymlink,
		location: "global",
	})
	if cmd == nil {
		t.Fatal("doMOATInstallCmd returned nil tea.Cmd")
	}
	msg, ok := cmd().(installDoneMsg)
	if !ok {
		t.Fatalf("expected installDoneMsg, got %T", cmd())
	}
	if msg.err == nil {
		t.Fatal("expected error for missing registry, got nil")
	}
	if !strings.Contains(msg.err.Error(), "ghost-registry") {
		t.Errorf("error should mention registry name; got %q", msg.err.Error())
	}
}

func TestDoMOATInstallCmd_ManifestEntryNotFound(t *testing.T) {
	app := testApp(t)
	app.cfg = &config.Config{
		Registries: []config.Registry{{
			Name:        "moat-registry",
			ManifestURI: "https://example.com/manifest.json",
		}},
	}
	// Empty manifest — no Content entries.
	app.moatGate = &moat.GateInputs{
		Manifests: map[string]*moat.Manifest{
			"moat-registry": {SchemaVersion: 1, Name: "moat-registry"},
		},
		ManifestURIs: map[string]string{
			"moat-registry": "https://example.com/manifest.json",
		},
		RevSet: moat.NewRevocationSet(),
	}

	item := catalog.ContentItem{
		Name:     "ghost-skill",
		Type:     catalog.Skills,
		Source:   "moat-registry",
		Registry: "moat-registry",
	}
	prov := provider.Provider{Name: "Test", Slug: "test"}

	cmd := app.doMOATInstallCmd(installResultMsg{
		item:     item,
		provider: prov,
		method:   installer.MethodSymlink,
		location: "global",
	})
	msg := cmd().(installDoneMsg)
	if msg.err == nil {
		t.Fatal("expected error for manifest miss, got nil")
	}
	if !strings.Contains(msg.err.Error(), "ghost-skill") {
		t.Errorf("error should mention item name; got %q", msg.err.Error())
	}
}

// TestDoInstallCmd_RoutesUnstagedMOATToMOATPath verifies the dispatcher in
// doInstallCmd recognizes unstaged MOAT items (Path=="" && Source!="") and
// forwards to doMOATInstallCmd instead of the library install path. Without
// this routing, installer.Install would be called with an empty Path and
// fail with a confusing "no such file" error.
func TestDoInstallCmd_RoutesUnstagedMOATToMOATPath(t *testing.T) {
	app := testApp(t)
	app.cfg = &config.Config{} // no registries → MOAT path will report missing registry

	item := catalog.ContentItem{
		Name:     "unstaged-skill",
		Type:     catalog.Skills,
		Source:   "moat-registry",
		Registry: "moat-registry",
		// Path is intentionally empty — this is the unstaged-MOAT discriminator.
	}
	prov := provider.Provider{Name: "Test", Slug: "test"}

	cmd := app.doInstallCmd(installResultMsg{
		item:     item,
		provider: prov,
		method:   installer.MethodSymlink,
		location: "global",
	})
	msg := cmd().(installDoneMsg)
	// The MOAT path's missing-registry error has a distinctive shape; the
	// library install path would have returned an os-level "open : no such
	// file or directory" error instead. Match on the MOAT shape to prove
	// the dispatcher routed correctly.
	if msg.err == nil || !strings.Contains(msg.err.Error(), "registry") {
		t.Fatalf("expected MOAT-path error mentioning registry, got %v", msg.err)
	}
}
