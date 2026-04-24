package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
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

// TestDoRegistryRemoveCmd_RemovesFromLocalConfig is the regression test for the
// bug where doRegistryRemoveCmd only wrote to the global config, leaving a
// registry that lived in a project-local config untouched after removal.
func TestDoRegistryRemoveCmd_RemovesFromLocalConfig(t *testing.T) {
	// Not parallel: mutates global state (registry.CacheDirOverride, config.GlobalDirOverride).
	projectRoot := t.TempDir()
	globalDir := t.TempDir()
	cacheDir := t.TempDir()

	// Redirect global config and registry cache away from the real home.
	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = globalDir
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })

	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	// Write a registry into the project-local config only (not global).
	localCfg := &config.Config{
		Registries: []config.Registry{{Name: "my-reg", URL: "https://example.com/repo.git"}},
	}
	if err := config.Save(projectRoot, localCfg); err != nil {
		t.Fatalf("setup: config.Save: %v", err)
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

	// The registry must be gone from the project-local config.
	result, err := config.Load(projectRoot)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	for _, r := range result.Registries {
		if r.Name == "my-reg" {
			t.Errorf("registry still present in project-local config after removal")
		}
	}
}
