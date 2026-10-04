package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/lifecycle"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/moatinstall"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/regdiff"
)

// syncedInstall is one install of example/my-skill whose sync result the
// test chooses. The registry lives in the global config and the source repo
// is a local fixture, so nothing touches the network.
type syncedInstall struct {
	env       *integrationEnv
	configDir string
	globalDir string
	manifest  *moat.Manifest
	now       time.Time
}

func newSyncedInstall(t *testing.T) *syncedInstall {
	t.Helper()
	s := &syncedInstall{
		env:       setupIntegrationEnv(t),
		globalDir: t.TempDir(),
		configDir: withInstallRecordConfigDir(t),
		now:       time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC),
	}
	withGlobalLibrary(t, s.globalDir)
	output.SetForTest(t)
	t.Setenv("HOME", s.env.projectRoot)

	fixtureRoot := t.TempDir()
	contentHash := makeRepoFixture(t, fixtureRoot, "skills", "my-skill", map[string]string{
		"SKILL.md": "# from registry\n",
	})
	stubCloneFromFixture(t, fixtureRoot)
	s.manifest = syncMOATDriftManifest(t, "2026-10-03T11:00:00Z", []moat.ContentEntry{{
		Name:        "my-skill",
		DisplayName: "my-skill",
		Type:        "skill",
		ContentHash: contentHash,
		SourceURI:   "https://github.com/example/repo",
		AttestedAt:  s.now,
	}})
	return s
}

// freshSync answers the sync with a new manifest and its bytes.
func (s *syncedInstall) freshSync(t *testing.T) moat.SyncResult {
	t.Helper()
	return moat.SyncResult{
		ManifestURL:     "https://example.com/m",
		Manifest:        s.manifest,
		ManifestBytes:   syncMOATDriftManifestBytes(t, s.manifest),
		BundleBytes:     []byte(`{"bundle":true}`),
		IncomingProfile: incomingProfile(),
		Staleness:       moat.StalenessFresh,
		ETag:            `"etag-1"`,
		FetchedAt:       s.now,
	}
}

func (s *syncedInstall) run(t *testing.T) (string, error) {
	t.Helper()
	out := &bytes.Buffer{}
	prov := integrationTestProvider()
	err := installFromRegistryForTest(t, context.Background(), out, &bytes.Buffer{},
		cfgWithPinnedMOATRegistry(t), s.env.projectRoot, s.globalDir, "example", "my-skill",
		&prov, installer.MethodSymlink, "", false, installer.ScanOptions{}, s.now)
	return out.String(), err
}

// Regression: the install saved the merged config into the project's
// .syllago/config.json, copying every global registry into the project, and
// left the global entry without the new ETag.
func TestInstallFromRegistry_SyncSavesGlobalConfig(t *testing.T) {
	s := newSyncedInstall(t)
	s.env.syncResultFn = func() (moat.SyncResult, error) { return s.freshSync(t), nil }

	if _, err := s.run(t); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(config.FilePath(s.env.projectRoot)); !os.IsNotExist(err) {
		t.Errorf("project config written by the install (stat err = %v)", err)
	}
	global, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if len(global.Registries) != 1 || global.Registries[0].ManifestETag != `"etag-1"` {
		t.Errorf("global registries = %+v, want example with the synced ETag", global.Registries)
	}
}

// Regression: a 304 from the registry failed the install, though the
// manifest it confirmed was already cached.
func TestInstallFromRegistry_NotModifiedUsesCachedManifest(t *testing.T) {
	s := newSyncedInstall(t)
	if err := moat.WriteManifestCache(s.configDir, "example",
		syncMOATDriftManifestBytes(t, s.manifest), []byte(`{"bundle":true}`)); err != nil {
		t.Fatalf("seed manifest cache: %v", err)
	}
	s.env.syncResultFn = func() (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL:     "https://example.com/m",
			NotModified:     true,
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
			ETag:            `"etag-1"`,
			FetchedAt:       s.now,
		}, nil
	}

	out, err := s.run(t)
	if err != nil {
		t.Fatalf("install after a 304: %v", err)
	}
	if !strings.Contains(out, "installed example/my-skill") {
		t.Errorf("output = %q, want the install confirmation", out)
	}
}

// Regression: the install synced a new manifest but never cached it, so
// the Library kept showing the registry as it was at the last sync.
func TestInstallFromRegistry_FreshSyncCachesManifest(t *testing.T) {
	s := newSyncedInstall(t)
	s.env.syncResultFn = func() (moat.SyncResult, error) { return s.freshSync(t), nil }

	if _, err := s.run(t); err != nil {
		t.Fatalf("install: %v", err)
	}
	cached, err := regdiff.LoadCachedManifest(s.configDir, "example")
	if err != nil {
		t.Fatalf("LoadCachedManifest: %v", err)
	}
	if cached == nil || len(cached.Content) != 1 || cached.Content[0].Name != "my-skill" {
		t.Errorf("cached manifest = %+v, want the synced one", cached)
	}
}

// Regression: a 304 confirmed a cached manifest whose expiry had passed,
// and the install went ahead instead of exiting 13.
func TestInstallFromRegistry_NotModifiedExpiredCacheExits13(t *testing.T) {
	s := newSyncedInstall(t)
	expired := s.now.Add(-time.Hour)
	s.manifest.Expires = &expired
	if err := moat.WriteManifestCache(s.configDir, "example",
		syncMOATDriftManifestBytes(t, s.manifest), []byte(`{"bundle":true}`)); err != nil {
		t.Fatalf("seed manifest cache: %v", err)
	}
	s.env.syncResultFn = func() (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL:     "https://example.com/m",
			NotModified:     true,
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
			ETag:            `"etag-1"`,
			FetchedAt:       s.now,
		}, nil
	}
	exitCode := withInstallGateStubs(t, false, false)

	out, err := s.run(t)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if *exitCode != moat.ExitMoatManifestStale {
		t.Errorf("exit code = %d, want %d", *exitCode, moat.ExitMoatManifestStale)
	}
	if strings.Contains(out, "installed example/my-skill") {
		t.Errorf("output = %q, want no install from an expired manifest", out)
	}
}

// Regression: a sync that verified an expired manifest and then failed to
// save it reported the save failure, exit 1, instead of exit 13.
func TestInstallFromRegistry_ExpiredManifestSaveFailureExits13(t *testing.T) {
	s := newSyncedInstall(t)
	expired := s.now.Add(-time.Hour)
	s.manifest.Expires = &expired
	s.env.syncResultFn = func() (moat.SyncResult, error) {
		r := s.freshSync(t)
		r.Staleness = moat.StalenessExpired
		return r, nil
	}
	exitCode := withInstallGateStubs(t, false, false)
	cfgWithPinnedMOATRegistry(t)
	if err := os.Chmod(s.configDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(s.configDir, 0o755) })

	out := &bytes.Buffer{}
	prov := integrationTestProvider()
	// A nil config keeps the one saved above, which the read-only config
	// directory would refuse to save again.
	err := installFromRegistryForTest(t, context.Background(), out, &bytes.Buffer{},
		nil, s.env.projectRoot, s.globalDir, "example", "my-skill",
		&prov, installer.MethodSymlink, "", false, installer.ScanOptions{}, s.now)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if *exitCode != moat.ExitMoatManifestStale {
		t.Errorf("exit code = %d, want %d", *exitCode, moat.ExitMoatManifestStale)
	}
}

// --to-all with <registry>/<item> installs the registry item to every
// detected provider through the same operation as --to.
func TestInstallToAll_RegistryItem(t *testing.T) {
	s := newSyncedInstall(t)
	s.env.syncResultFn = func() (moat.SyncResult, error) { return s.freshSync(t), nil }
	cfgWithPinnedMOATRegistry(t)
	withFakeRepoRoot(t, s.env.projectRoot)
	origNow := moatInstallNow
	moatInstallNow = func() time.Time { return s.now }
	t.Cleanup(func() { moatInstallNow = origNow })
	prov := integrationTestProvider()
	prov.Detect = func(string) bool { return true }
	origProviders := append([]provider.Provider(nil), provider.AllProviders...)
	provider.AllProviders = []provider.Provider{prov}
	t.Cleanup(func() { provider.AllProviders = origProviders })
	stdout, _ := output.SetForTest(t)
	resetInstallRecordInstallFlags(t)
	installCmd.Flags().Set("to-all", "true")
	installCmd.Flags().Set("no-input", "true")
	t.Cleanup(func() { resetInstallRecordInstallFlags(t); installCmd.Flags().Set("no-input", "false") })

	installCmd.SetContext(context.Background())
	if err := installCmd.RunE(installCmd, []string{"example/my-skill"}); err != nil {
		t.Fatalf("install --to-all: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "installed example/my-skill") || !strings.Contains(got, ".testprovider") {
		t.Errorf("output = %q, want the registry install confirmation", stdout.String())
	}
}

func TestInstallFromRegistry_NoTargetsIsInputError(t *testing.T) {
	err := runInstallFromRegistry(context.Background(), &bytes.Buffer{}, &bytes.Buffer{},
		moatinstall.Request{Registry: "example", Items: []moatinstall.Item{{Name: "my-skill"}}}, time.Now())
	var se output.StructuredError
	if !errors.As(err, &se) || se.Code != output.ErrInputMissing {
		t.Fatalf("err = %v, want %s", err, output.ErrInputMissing)
	}
}

// Regression: a declined publisher warning was recorded and the install
// retried, so a second sync whose manifest dropped the warning installed it.
func TestInstallFromRegistry_DeclineStopsWithoutRetry(t *testing.T) {
	s := newSyncedInstall(t)
	s.manifest.Revocations = []moat.Revocation{{
		ContentHash: s.manifest.Content[0].ContentHash,
		Reason:      "deprecated",
		Source:      "publisher",
	}}
	syncs := 0
	s.env.syncResultFn = func() (moat.SyncResult, error) {
		syncs++
		return s.freshSync(t), nil
	}
	withInstallGateStubs(t, true, false)

	out, err := s.run(t)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if syncs != 1 || strings.Contains(out, "installed") {
		t.Errorf("syncs = %d, output = %q; want one sync and no install", syncs, out)
	}
}

// Regression: a failure on one target hid the targets that did install.
func TestReportRegistryInstall_ReportsCompletedBeforeFailure(t *testing.T) {
	output.SetForTest(t)
	failure := errors.New("second target failed")
	out := &bytes.Buffer{}
	res := moatinstall.Result{Items: []moatinstall.ItemResult{{
		Name:    "my-skill",
		Entry:   &moat.ContentEntry{Name: "my-skill"},
		Install: lifecycle.Outcome{Completed: []lifecycle.Step{{Placement: installer.Placement{Path: "/x"}}}},
		Err:     failure,
	}}}

	err := reportRegistryInstall(out, &bytes.Buffer{}, moatinstall.Request{Registry: "example"}, res)
	if !errors.Is(err, failure) {
		t.Errorf("err = %v, want the target failure", err)
	}
	if !strings.Contains(out.String(), "installed example/my-skill") {
		t.Errorf("output = %q, want the completed install reported", out.String())
	}
}
