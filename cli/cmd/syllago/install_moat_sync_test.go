package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
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
	err := runInstallFromRegistry(context.Background(), out, &bytes.Buffer{},
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
