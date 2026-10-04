package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/moatinstall"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/registryops"
)

// registryEnv is one MOAT registry, "example", publishing the skill
// "my-skill" from a local fixture, so no install touches the network.
type registryEnv struct {
	app      App
	home     string
	library  string
	manifest *moat.Manifest
	item     catalog.ContentItem
	prov     provider.Provider
	syncs    []registryops.SyncOpts
	// sync answers each sync; nil means a fresh, verified manifest.
	sync func(ctx context.Context, opts registryops.SyncOpts) (registryops.SyncOutcome, error)
}

func newRegistryEnv(t *testing.T) *registryEnv {
	t.Helper()
	e := &registryEnv{home: t.TempDir(), library: t.TempDir()}
	t.Setenv("HOME", e.home)
	project := t.TempDir()
	origConfig, origLibrary := config.GlobalDirOverride, catalog.GlobalContentDirOverride
	config.GlobalDirOverride, catalog.GlobalContentDirOverride = t.TempDir(), e.library
	origSource, origScratch := moatinstall.SourceCacheDir, moatinstall.CloneScratchDir
	sourceDir, scratchDir := t.TempDir(), t.TempDir()
	moatinstall.SourceCacheDir = func() (string, error) { return sourceDir, nil }
	moatinstall.CloneScratchDir = func() (string, error) { return scratchDir, nil }
	moatinstall.PreviousRootOverride = t.TempDir()
	t.Cleanup(func() {
		config.GlobalDirOverride, catalog.GlobalContentDirOverride = origConfig, origLibrary
		moatinstall.SourceCacheDir, moatinstall.CloneScratchDir = origSource, origScratch
		moatinstall.PreviousRootOverride = ""
	})
	reg := config.Registry{
		Name: "example", URL: "https://example.com/m", Type: config.RegistryTypeMOAT, ManifestURI: "https://example.com/m",
	}
	if err := config.SaveGlobal(&config.Config{Registries: []config.Registry{reg}}); err != nil {
		t.Fatal(err)
	}

	fixture := t.TempDir()
	itemDir := filepath.Join(fixture, "skills", "my-skill")
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemDir, "SKILL.md"), []byte("# my-skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := moat.ContentHash(itemDir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	e.manifest = &moat.Manifest{Content: []moat.ContentEntry{{
		Name: "my-skill", DisplayName: "my-skill", Type: "skill", ContentHash: hash,
		SourceURI: "https://github.com/example/repo", AttestedAt: now,
	}}}

	orig := registryInstallOp
	registryInstallOp = func(minTier moat.TrustTier) *moatinstall.Operation {
		return &moatinstall.Operation{
			MinTier: minTier,
			Clone: func(_ context.Context, _, dest string) error {
				return moat.CopyTree(fixture, dest)
			},
			Sync: func(ctx context.Context, _ string, opts registryops.SyncOpts) (registryops.SyncOutcome, error) {
				e.syncs = append(e.syncs, opts)
				if e.sync != nil {
					return e.sync(ctx, opts)
				}
				return registryops.SyncOutcome{MoatResult: moat.SyncResult{
					Manifest: e.manifest, Staleness: moat.StalenessFresh, FetchedAt: now,
				}}, nil
			},
			Now: func() time.Time { return now },
		}
	}
	t.Cleanup(func() { registryInstallOp = orig })

	e.prov = provider.Provider{
		Name: "Stub", Slug: "stub",
		InstallDir: func(home string, ct catalog.ContentType) string {
			return filepath.Join(home, ".stub", string(ct))
		},
		SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Skills },
	}
	// A synced item: the manifest gave it a trust tier, and its files sit
	// in the content cache, so Path is set though it is not in the Library.
	e.item = catalog.ContentItem{
		Name: "my-skill", Type: catalog.Skills, Registry: "example", Source: "example",
		Path: itemDir, TrustTier: catalog.TrustTierUnsigned,
	}
	e.app = testApp(t)
	e.app.cfg = &config.Config{Registries: []config.Registry{reg}}
	e.app.projectRoot = project
	return e
}

func (e *registryEnv) install() installResultMsg {
	return installResultMsg{
		item: e.item, provider: e.prov, location: "global",
		method: installer.MethodSymlink, projectRoot: e.app.projectRoot,
	}
}

// update feeds msg to the app and returns the new app and its command.
func (e *registryEnv) update(msg tea.Msg) tea.Cmd {
	m, cmd := e.app.Update(msg)
	e.app = m.(App)
	return cmd
}

// awaitInstall waits for cmd to report the registry install and feeds the
// report to the app.
func (e *registryEnv) awaitInstall(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	msg, ok := waitInstall(cmd)
	if !ok {
		t.Fatal("the registry install never reported")
	}
	e.update(msg)
}

// waitInstall runs cmd and its batched commands until one reports the
// registry install. The other commands, toast timers among them, are left
// running.
func waitInstall(cmd tea.Cmd) (registryInstallDoneMsg, bool) {
	found := make(chan registryInstallDoneMsg, 1)
	var run func(tea.Cmd)
	run = func(c tea.Cmd) {
		if c == nil {
			return
		}
		switch msg := c().(type) {
		case registryInstallDoneMsg:
			found <- msg
		case tea.BatchMsg:
			for _, sub := range msg {
				go run(sub)
			}
		}
	}
	go run(cmd)
	select {
	case msg := <-found:
		return msg, true
	case <-time.After(10 * time.Second):
		return registryInstallDoneMsg{}, false
	}
}

func (e *registryEnv) placed() bool {
	_, err := os.Lstat(filepath.Join(e.home, ".stub", string(catalog.Skills), "my-skill"))
	return err == nil
}

func (e *registryEnv) inLibrary() bool {
	_, err := os.Stat(filepath.Join(e.library, string(catalog.Skills), "my-skill", "SKILL.md"))
	return err == nil
}

func (e *registryEnv) toast() string {
	var b strings.Builder
	for _, q := range e.app.toast.queue {
		b.WriteString(q.message + "\n")
	}
	return b.String()
}

// Regression: the install wizard refused a synced MOAT item, because its
// files were cached and so it no longer looked unstaged.
func TestIsMOATRegistryItem(t *testing.T) {
	synced := catalog.ContentItem{Registry: "example", Path: "/cache/x", TrustTier: catalog.TrustTierUnsigned}
	unstaged := catalog.ContentItem{Registry: "example", Source: "example"}
	gitItem := catalog.ContentItem{Registry: "git-reg", Path: "/clone/x"}
	library := synced
	library.Library = true
	for _, tc := range []struct {
		name string
		item catalog.ContentItem
		want bool
	}{
		{"synced", synced, true},
		{"unstaged", unstaged, true},
		{"git registry", gitItem, false},
		{"in the Library", library, false},
	} {
		if got := isMOATRegistryItem(&tc.item); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestRegistryInstall_StagesThenInstalls(t *testing.T) {
	e := newRegistryEnv(t)
	m, cmd := e.app.handleInstallResult(e.install())
	e.app = m.(App)
	e.awaitInstall(t, cmd)

	if !e.inLibrary() || !e.placed() {
		t.Fatalf("Library=%v placed=%v, want both; toasts:\n%s", e.inLibrary(), e.placed(), e.toast())
	}
	if e.app.registryInstallCancel != nil {
		t.Error("the finished install left its cancel set")
	}
}

// Regression: install-all had no registry path and installed a registry
// item from its cache without verifying it or adding it to the Library.
func TestRegistryInstall_InstallAllStagesThenInstalls(t *testing.T) {
	e := newRegistryEnv(t)
	m, cmd := e.app.handleInstallAllResult(installAllResultMsg{
		item: e.item, providers: []provider.Provider{e.prov}, projectRoot: e.app.projectRoot,
	})
	e.app = m.(App)
	e.awaitInstall(t, cmd)

	if !e.inLibrary() || !e.placed() {
		t.Fatalf("Library=%v placed=%v, want both; toasts:\n%s", e.inLibrary(), e.placed(), e.toast())
	}
}

func TestRegistryInstall_TOFUAcceptedInstalls(t *testing.T) {
	e := newRegistryEnv(t)
	e.sync = func(_ context.Context, opts registryops.SyncOpts) (registryops.SyncOutcome, error) {
		out := registryops.SyncOutcome{MoatResult: moat.SyncResult{Manifest: e.manifest, Staleness: moat.StalenessFresh}}
		out.GateTOFUNeeded = !opts.AcceptTOFU
		return out, nil
	}
	m, cmd := e.app.handleInstallResult(e.install())
	e.app = m.(App)
	e.awaitInstall(t, cmd)
	if !e.app.tofu.Active() || e.app.pendingRegistryDecision == nil {
		t.Fatalf("TOFU modal active=%v, want it open; toasts:\n%s", e.app.tofu.Active(), e.toast())
	}

	e.app.tofu.active = false
	e.awaitInstall(t, e.update(tofuResultMsg{name: "example", accepted: true}))
	if !e.syncs[len(e.syncs)-1].AcceptTOFU || !e.placed() {
		t.Fatalf("AcceptTOFU=%v placed=%v, want both", e.syncs[len(e.syncs)-1].AcceptTOFU, e.placed())
	}
}

func TestRegistryInstall_PublisherWarning(t *testing.T) {
	for _, confirmed := range []bool{true, false} {
		e := newRegistryEnv(t)
		e.manifest.Revocations = []moat.Revocation{{
			ContentHash: e.manifest.Content[0].ContentHash, Reason: "deprecated", Source: "publisher",
		}}
		m, cmd := e.app.handleInstallResult(e.install())
		e.app = m.(App)
		e.awaitInstall(t, cmd)
		if !e.app.confirm.active || e.app.pendingRegistryDecision == nil {
			t.Fatalf("confirm modal active=%v, want it open; toasts:\n%s", e.app.confirm.active, e.toast())
		}

		e.app.confirm.active = false
		cmd = e.update(confirmResultMsg{confirmed: confirmed, item: e.item})
		if !confirmed {
			if len(e.syncs) != 1 || e.inLibrary() {
				t.Errorf("declined: syncs=%d Library=%v, want no second run", len(e.syncs), e.inLibrary())
			}
			continue
		}
		e.awaitInstall(t, cmd)
		if !e.placed() {
			t.Errorf("confirmed: not placed; toasts:\n%s", e.toast())
		}
	}
}

func TestRegistryInstall_EscCancels(t *testing.T) {
	e := newRegistryEnv(t)
	started := make(chan struct{})
	e.sync = func(ctx context.Context, _ registryops.SyncOpts) (registryops.SyncOutcome, error) {
		close(started)
		<-ctx.Done()
		return registryops.SyncOutcome{}, ctx.Err()
	}
	m, cmd := e.app.handleInstallResult(e.install())
	e.app = m.(App)
	type report struct {
		msg registryInstallDoneMsg
		ok  bool
	}
	done := make(chan report, 1)
	go func() {
		msg, ok := waitInstall(cmd)
		done <- report{msg, ok}
	}()
	<-started
	e.update(keyPress(tea.KeyEsc))
	r := <-done
	if !r.ok {
		t.Fatal("the registry install never reported")
	}
	e.update(r.msg)

	if e.app.registryInstallCancel != nil || e.inLibrary() {
		t.Errorf("cancel set=%v Library=%v after Esc", e.app.registryInstallCancel != nil, e.inLibrary())
	}
	if !strings.Contains(e.toast(), "Install cancelled") {
		t.Errorf("toasts:\n%s\nwant the cancel reported", e.toast())
	}
}

func TestRegistryInstall_OneAtATime(t *testing.T) {
	e := newRegistryEnv(t)
	e.app.registryInstallCancel = func() {}
	m, _ := e.app.handleInstallResult(e.install())
	e.app = m.(App)
	if !strings.Contains(e.toast(), "already running") || len(e.syncs) != 0 {
		t.Errorf("syncs=%d toasts:\n%s\nwant the second install refused", len(e.syncs), e.toast())
	}
}
