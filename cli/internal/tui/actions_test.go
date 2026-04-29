package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
	"github.com/OpenScribbler/syllago/cli/internal/registryops"
)

// testAppWithInstalledRule builds an App with one library rule already installed
// into a single provider.
func testAppWithInstalledRule(t *testing.T) (App, catalog.ContentItem, provider.Provider) {
	t.Helper()
	home := t.TempDir()
	itemDir := filepath.Join(home, "lib", "rules", "my-rule")
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatal(err)
	}
	srcFile := filepath.Join(itemDir, "rule.md")
	if err := os.WriteFile(srcFile, []byte("# My Rule"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Install target: a symlink under a fake home provider dir that points at
	// the source. This makes installer.CheckStatus report StatusInstalled.
	prov := provider.Provider{
		Name: "Claude Code", Slug: "claude-code", Detected: true,
		InstallDir: func(home string, ct catalog.ContentType) string {
			switch ct {
			case catalog.Rules:
				return filepath.Join(home, ".claude-code", "rules")
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Rules
		},
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	installDir := prov.InstallDir(home, catalog.Rules)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(itemDir, filepath.Join(installDir, "my-rule")); err != nil {
		t.Fatal(err)
	}

	item := catalog.ContentItem{
		Name:        "my-rule",
		DisplayName: "My Rule",
		Type:        catalog.Rules,
		Path:        itemDir,
		Files:       []string{"rule.md"},
		Library:     true,
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{item}}
	app := NewApp(cat, []provider.Provider{prov}, "0.0.0-test", false, nil, testConfig(), false, "", t.TempDir())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m.(App), item, prov
}

// TestHandleInstall_AcceptsUndetectedProviders verifies that the install wizard
// opens with all providers visible — including undetected ones — and that the
// default cursor lands on the first detected provider. The previous behavior
// hard-blocked the wizard with a "No providers detected" toast when the
// detected list was empty, contradicting the advisory-only contract documented
// at provider.go:39.
func TestHandleInstall_AcceptsUndetectedProviders(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	itemDir := filepath.Join(home, "lib", "rules", "my-rule")
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("# Rule"), 0o644); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{
		Name:    "my-rule",
		Type:    catalog.Rules,
		Path:    itemDir,
		Files:   []string{"rule.md"},
		Library: true,
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{item}}

	// Order matters: undetected first, detected second. A correct implementation
	// must default the cursor to the detected provider (index 1), not index 0.
	undetected := testInstallProvider("Cursor", "cursor", false)
	detected := testInstallProvider("Claude Code", "claude-code", true)

	app := NewApp(cat, []provider.Provider{undetected, detected}, "0.0.0-test", false, nil, testConfig(), false, "", t.TempDir())
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a := m.(App)

	m2, _ := a.handleInstall()
	a2 := m2.(App)

	if a2.wizardMode != wizardInstall {
		t.Fatalf("expected install wizard to open, wizardMode=%v (likely blocked by 'No providers detected' toast)", a2.wizardMode)
	}
	if a2.installWizard == nil {
		t.Fatal("expected installWizard to be non-nil")
	}
	if got := len(a2.installWizard.providers); got != 2 {
		t.Errorf("expected 2 providers in install wizard, got %d (undetected provider was filtered out)", got)
	}
	if a2.installWizard.providerCursor != 1 {
		t.Errorf("expected providerCursor=1 (lands on detected provider), got %d", a2.installWizard.providerCursor)
	}

	// View must label the undetected provider so the user knows.
	view := a2.installWizard.View()
	if !strings.Contains(view, "Cursor") {
		t.Error("install wizard view should list the undetected provider 'Cursor'")
	}
	if !strings.Contains(view, "(not detected)") {
		t.Error("install wizard view should label undetected providers with '(not detected)'")
	}
}

func TestActions_HandleRemoveDone_Success(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	m, cmd := app.handleRemoveDone(removeDoneMsg{
		itemName:        "my-rule",
		uninstalledFrom: []string{"Claude Code"},
	})
	if _, ok := m.(App); !ok {
		t.Fatalf("expected App, got %T", m)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd from success toast + rescan")
	}
}

func TestActions_HandleRemoveDone_Error(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	// Error toasts don't auto-dismiss, so Push returns a nil cmd by design.
	// The behavior we care about is that the toast becomes visible.
	m, _ := app.handleRemoveDone(removeDoneMsg{
		itemName: "bad",
		err:      errors.New("boom"),
	})
	a := m.(App)
	if !a.toast.visible {
		t.Fatal("expected error toast visible after remove failure")
	}
}

func TestActions_HandleUninstallDone_Success(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	_, cmd := app.handleUninstallDone(uninstallDoneMsg{
		itemName:        "my-rule",
		uninstalledFrom: []string{"Claude Code"},
	})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from success path")
	}
}

func TestActions_HandleUninstallDone_Error(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	// Error toasts don't auto-dismiss; assert visibility instead of cmd.
	m, _ := app.handleUninstallDone(uninstallDoneMsg{
		itemName: "my-rule",
		err:      errors.New("uninstall failed"),
	})
	a := m.(App)
	if !a.toast.visible {
		t.Fatal("expected error toast visible after uninstall failure")
	}
}

func TestActions_HandleInstallAllDone_Success(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	_, cmd := app.handleInstallAllDone(installAllDoneMsg{
		itemName: "my-rule",
		count:    3,
	})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from success toast + rescan")
	}
}

func TestActions_HandleInstallAllDone_Error(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	_, cmd := app.handleInstallAllDone(installAllDoneMsg{
		itemName: "my-rule",
		count:    1,
		firstErr: errors.New("partial failure"),
	})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from warning toast")
	}
}

func TestActions_HandleInstallAllDone_EmptyName(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	_, cmd := app.handleInstallAllDone(installAllDoneMsg{count: 2})
	if cmd == nil {
		t.Fatal("expected cmd when itemName empty (default to \"item\")")
	}
}

func TestActions_HandleRemoveResult_NotConfirmed(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	_, cmd := app.handleRemoveResult(removeResultMsg{confirmed: false})
	if cmd != nil {
		t.Errorf("expected nil cmd when confirmed=false, got %v", cmd)
	}
}

func TestActions_HandleRemoveResult_Confirmed(t *testing.T) {
	// Not parallel: testAppWithInstalledRule uses t.Setenv.
	app, item, _ := testAppWithInstalledRule(t)
	_, cmd := app.handleRemoveResult(removeResultMsg{
		confirmed: true,
		item:      item,
	})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from confirmed remove")
	}
	// Execute cmd to run doRemoveCmd closure.
	msg := cmd()
	if _, ok := msg.(removeDoneMsg); !ok {
		t.Errorf("expected removeDoneMsg, got %T", msg)
	}
}

func TestActions_DoSimpleRemoveCmd(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	itemDir := filepath.Join(t.TempDir(), "rules", "my-rule")
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "rule.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}
	item := catalog.ContentItem{Name: "my-rule", Path: itemDir, Library: true}

	cmd := app.doSimpleRemoveCmd(item)
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	done, ok := msg.(removeDoneMsg)
	if !ok {
		t.Fatalf("expected removeDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Errorf("expected nil err from success path, got %v", done.err)
	}
	if _, err := os.Stat(itemDir); !os.IsNotExist(err) {
		t.Errorf("expected item dir removed, got err=%v", err)
	}
}

func TestActions_DoUninstallCmd_NoChecks(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	// No checks means "uninstall from all providers".
	cmd := app.doUninstallCmd(confirmResultMsg{
		item:               catalog.ContentItem{Name: "my-rule"},
		uninstallProviders: nil,
	})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(uninstallDoneMsg); !ok {
		t.Fatalf("expected uninstallDoneMsg, got %T", msg)
	}
}

func TestActions_HandleUninstall_SelectedItemNil(t *testing.T) {
	t.Parallel()
	app := testApp(t) // empty catalog
	_, cmd := app.handleUninstall()
	// No selected item: returns (a, nil).
	if cmd != nil {
		t.Errorf("expected nil cmd on empty catalog, got %v", cmd)
	}
}

func TestActions_HandleUninstall_NoInstalledProviders(t *testing.T) {
	t.Parallel()
	app := testAppWithItems(t)
	_, cmd := app.handleUninstall()
	// Item has no installed providers — should push a "Not installed" warning toast.
	if cmd == nil {
		t.Fatal("expected warning toast cmd")
	}
}

func TestActions_HandleRemove_NoItem(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	_, cmd := app.handleRemove()
	if cmd != nil {
		t.Errorf("expected nil cmd with no selected item, got %v", cmd)
	}
}

func TestActions_HandleRemove_OpensRemoveModal(t *testing.T) {
	app, _, _ := testAppWithInstalledRule(t)
	m, cmd := app.handleRemove()
	_ = cmd
	a := m.(App)
	if !a.remove.active {
		t.Error("remove modal should be open after handleRemove")
	}
}

func TestActions_HandleRemove_SkipsRegistryOnlyItems(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	app.catalog.Items = []catalog.ContentItem{{
		Name: "external", Type: catalog.Rules, Library: false, Registry: "remote",
	}}
	app.refreshContent()
	_, cmd := app.handleRemove()
	if cmd != nil {
		t.Errorf("registry-only item should not trigger remove cmd, got %v", cmd)
	}
}

func TestActions_HandleUninstall_SingleProviderOpensConfirm(t *testing.T) {
	app, _, _ := testAppWithInstalledRule(t)
	m, _ := app.handleUninstall()
	a := m.(App)
	if !a.confirm.active {
		t.Error("confirm modal should be active after handleUninstall with a single installed provider")
	}
	if len(a.confirm.uninstallProviders) != 1 {
		t.Errorf("expected 1 uninstall provider, got %d", len(a.confirm.uninstallProviders))
	}
}

// stubCloneFn swaps the orchestrator's clone seam (registryops.CloneFn) with
// a stub that creates a fake clone dir at registry.CloneDir(name) containing
// registry.yaml with yamlContent. Restores on t.Cleanup.
func stubCloneFn(t *testing.T, yamlContent string) {
	t.Helper()
	orig := registryops.CloneFn
	registryops.CloneFn = func(url, name, ref string) error {
		cloneDir, err := registry.CloneDir(name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(cloneDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(cloneDir, "registry.yaml"), []byte(yamlContent), 0o644)
	}
	t.Cleanup(func() { registryops.CloneFn = orig })
}

// TestDoRegistryAddCmd_AllowlistSetsMOATFields verifies the allowlist path:
// adding the bundled meta-registry URL auto-sets Type=moat + ManifestURI
// without requiring any explicit flags. Regression for the bug where the TUI
// bypassed MOAT auto-detection entirely.
func TestDoRegistryAddCmd_AllowlistSetsMOATFields(t *testing.T) {
	// Not parallel: mutates cloneFn, config.GlobalDirOverride, registry.CacheDirOverride.
	globalDir := t.TempDir()
	cacheDir := t.TempDir()

	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = globalDir
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })

	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	stubCloneFn(t, "name: syllago-meta-registry\nversion: \"1.0\"\n")

	app := testApp(t)
	cmd := app.doRegistryAddCmd(registryAddMsg{
		name: "OpenScribbler/syllago-meta-registry",
		url:  "https://github.com/OpenScribbler/syllago-meta-registry",
	})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	done, ok := msg.(registryAddDoneMsg)
	if !ok {
		t.Fatalf("expected registryAddDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("expected nil err, got %v", done.err)
	}

	cfg, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if len(cfg.Registries) != 1 {
		t.Fatalf("expected 1 registry, got %d", len(cfg.Registries))
	}
	r := cfg.Registries[0]
	if r.Type != config.RegistryTypeMOAT {
		t.Errorf("expected Type=moat from allowlist, got %q", r.Type)
	}
	if r.SigningProfile == nil {
		t.Error("expected non-nil SigningProfile from allowlist")
	}
	if r.ManifestURI == "" {
		t.Error("expected non-empty ManifestURI from allowlist")
	}
}

// TestDoRegistryAddCmd_SelfDeclarationSetsMOATFields verifies the self-declaration
// fallback: a non-allowlisted URL that ships a registry.yaml with manifest_uri
// gets Type=moat + ManifestURI set from that self-declaration.
func TestDoRegistryAddCmd_SelfDeclarationSetsMOATFields(t *testing.T) {
	// Not parallel: mutates cloneFn, config.GlobalDirOverride, registry.CacheDirOverride.
	globalDir := t.TempDir()
	cacheDir := t.TempDir()

	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = globalDir
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })

	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	const wantURI = "https://raw.githubusercontent.com/example/non-allowlisted-registry/moat-registry/registry.json"
	stubCloneFn(t, "name: non-allowlisted-registry\nversion: \"1.0\"\nmanifest_uri: "+wantURI+"\n")

	app := testApp(t)
	cmd := app.doRegistryAddCmd(registryAddMsg{
		name: "example/non-allowlisted-registry",
		url:  "https://github.com/example/non-allowlisted-registry",
	})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	done, ok := msg.(registryAddDoneMsg)
	if !ok {
		t.Fatalf("expected registryAddDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("expected nil err, got %v", done.err)
	}

	cfg, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if len(cfg.Registries) != 1 {
		t.Fatalf("expected 1 registry, got %d", len(cfg.Registries))
	}
	r := cfg.Registries[0]
	if r.Type != config.RegistryTypeMOAT {
		t.Errorf("expected Type=moat from self-declaration, got %q", r.Type)
	}
	if r.ManifestURI != wantURI {
		t.Errorf("expected ManifestURI=%q from self-declaration, got %q", wantURI, r.ManifestURI)
	}
}

// TestDoRegistryAddCmd_NonMOATStaysUnset verifies a plain git registry (no
// allowlist hit, no manifest_uri in registry.yaml) leaves Type and ManifestURI
// empty — auto-detection should not false-positive.
func TestDoRegistryAddCmd_NonMOATStaysUnset(t *testing.T) {
	// Not parallel: mutates cloneFn, config.GlobalDirOverride, registry.CacheDirOverride.
	globalDir := t.TempDir()
	cacheDir := t.TempDir()

	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = globalDir
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })

	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	stubCloneFn(t, "name: plain-registry\nversion: \"1.0\"\n")

	app := testApp(t)
	cmd := app.doRegistryAddCmd(registryAddMsg{
		name: "example/plain",
		url:  "https://github.com/example/plain",
	})
	msg := cmd()
	done := msg.(registryAddDoneMsg)
	if done.err != nil {
		t.Fatalf("expected nil err, got %v", done.err)
	}

	cfg, _ := config.LoadGlobal()
	if len(cfg.Registries) != 1 {
		t.Fatalf("expected 1 registry, got %d", len(cfg.Registries))
	}
	r := cfg.Registries[0]
	if r.Type != "" {
		t.Errorf("expected empty Type for plain registry, got %q", r.Type)
	}
	if r.ManifestURI != "" {
		t.Errorf("expected empty ManifestURI for plain registry, got %q", r.ManifestURI)
	}
}

// TestDoRegistryRemoveCmd_RemovesFromGlobalConfig verifies the orchestrator
// path: a registry in global config is pruned and the global config is saved.
//
// Pre-S1 the TUI also wrote to project-local configs as a workaround for an
// inconsistency. After S1 (registries-are-global) project-local registries
// can't exist; after S2 (orchestrator) the TUI delegates to
// registryops.RemoveRegistry which only touches global. This test pins the
// new contract.
func TestDoRegistryRemoveCmd_RemovesFromGlobalConfig(t *testing.T) {
	// Not parallel: mutates global state (registry.CacheDirOverride, config.GlobalDirOverride).
	projectRoot := t.TempDir()
	globalDir := t.TempDir()
	cacheDir := t.TempDir()

	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = globalDir
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })

	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	globalCfg := &config.Config{
		Registries: []config.Registry{{Name: "my-reg", URL: "https://example.com/repo.git"}},
	}
	if err := config.SaveGlobal(globalCfg); err != nil {
		t.Fatalf("setup: config.SaveGlobal: %v", err)
	}

	app := NewApp(
		testCatalog(t), testProviders(), "0.0.0-test", false, nil,
		testConfig(), false, projectRoot, projectRoot,
	)
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	a := m.(App)

	cmd := a.doRegistryRemoveCmd("my-reg")
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()
	done, ok := msg.(registryRemoveDoneMsg)
	if !ok {
		t.Fatalf("expected registryRemoveDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("expected nil err, got %v", done.err)
	}

	result, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("config.LoadGlobal: %v", err)
	}
	for _, r := range result.Registries {
		if r.Name == "my-reg" {
			t.Errorf("registry still present in global config after removal")
		}
	}
}

// TestDoRegistryRemoveCmd_FailsLoudOnMismatch is the regression test for the
// silent-success bug where the gallery card's display name (overridden by
// registry.yaml) was used as the operational identity. If the passed name
// matches no config source, we MUST surface an error — not return a clean
// "Removed" message that lies about success.
//
// The original symptom: card displayed "syllago-meta-registry" (from
// registry.yaml), config held "OpenScribbler/syllago-meta-registry". Delete
// passed the display name through, no source matched, registry.Remove ran
// against a non-existent path (no error), config save was a no-op, toast
// said "Removed" — but the registry was still on disk and in config.
func TestDoRegistryRemoveCmd_FailsLoudOnMismatch(t *testing.T) {
	// Not parallel: mutates global state (registry.CacheDirOverride, config.GlobalDirOverride).
	projectRoot := t.TempDir()
	globalDir := t.TempDir()
	cacheDir := t.TempDir()

	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = globalDir
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })

	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	// Global config has the registry under its full identity.
	if err := config.SaveGlobal(&config.Config{
		Registries: []config.Registry{{Name: "owner/full-identity", URL: "https://example.com/repo.git"}},
	}); err != nil {
		t.Fatalf("setup: config.SaveGlobal: %v", err)
	}

	app := NewApp(
		testCatalog(t), testProviders(), "0.0.0-test", false, nil,
		testConfig(), false, projectRoot, projectRoot,
	)
	m, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	a := m.(App)

	// Pass the display-name (no owner prefix) — should NOT silently succeed.
	cmd := a.doRegistryRemoveCmd("full-identity")
	msg := cmd()
	done := msg.(registryRemoveDoneMsg)
	if done.err == nil {
		t.Fatal("expected error for non-matching name, got silent success")
	}
	if !strings.Contains(done.err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got %q", done.err.Error())
	}

	// And the registry must STILL be in config — we did not partial-write.
	loaded, _ := config.LoadGlobal()
	if len(loaded.Registries) != 1 || loaded.Registries[0].Name != "owner/full-identity" {
		t.Errorf("expected registry preserved on mismatch, got %+v", loaded.Registries)
	}
}
