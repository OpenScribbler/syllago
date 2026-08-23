package registryops

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/regdiff"
)

func TestSyncOneCapturesMOATDiffFromCachedManifest(t *testing.T) {
	root, cacheDir := setupSyncOneDiffTest(t)
	oldManifest := syncOneDiffManifest(t, "2026-04-20T00:00:00Z", []moat.ContentEntry{
		syncOneDiffEntry("kept", "1"),
		syncOneDiffEntry("removed", "2"),
	})
	if err := moat.WriteManifestCache(cacheDir, "example", syncOneDiffManifestBytes(t, oldManifest), []byte(`{"bundle":true}`)); err != nil {
		t.Fatalf("WriteManifestCache: %v", err)
	}

	newManifest := syncOneDiffManifest(t, "2026-04-21T00:00:00Z", []moat.ContentEntry{
		syncOneDiffEntry("added", "3"),
		syncOneDiffEntry("kept", "1"),
	})
	stubSyncOneDiff(t, syncOneDiffResult(t, newManifest, false))

	outcome, err := SyncOne(context.Background(), "example", SyncOpts{
		LockfileRoot: root,
		CacheDir:     cacheDir,
		Now:          syncOneDiffNow(),
	})
	if err != nil {
		t.Fatalf("SyncOne: %v", err)
	}
	if outcome.Diff == nil {
		t.Fatal("SyncOne Diff = nil; want item diff")
	}
	if outcome.Diff.OldRef != "2026-04-20T00:00:00Z" {
		t.Fatalf("OldRef = %q; want cached manifest timestamp", outcome.Diff.OldRef)
	}
	if outcome.Diff.NewRef != "2026-04-21T00:00:00Z" {
		t.Fatalf("NewRef = %q; want fresh manifest timestamp", outcome.Diff.NewRef)
	}
	want := []regdiff.ItemChange{
		{Type: "skill", Name: "added", Kind: regdiff.KindAdded},
		{Type: "skill", Name: "removed", Kind: regdiff.KindRemoved},
	}
	if !reflect.DeepEqual(outcome.Diff.Changes, want) {
		t.Fatalf("Diff.Changes = %#v; want %#v", outcome.Diff.Changes, want)
	}
}

func TestSyncOneCapturesMOATDiffWithNoCachedBaseline(t *testing.T) {
	root, cacheDir := setupSyncOneDiffTest(t)
	newManifest := syncOneDiffManifest(t, "2026-04-21T00:00:00Z", []moat.ContentEntry{
		syncOneDiffEntry("first", "1"),
	})
	stubSyncOneDiff(t, syncOneDiffResult(t, newManifest, false))

	outcome, err := SyncOne(context.Background(), "example", SyncOpts{
		LockfileRoot: root,
		CacheDir:     cacheDir,
		Now:          syncOneDiffNow(),
	})
	if err != nil {
		t.Fatalf("SyncOne: %v", err)
	}
	if outcome.Diff == nil {
		t.Fatal("SyncOne Diff = nil; want first-sync diff")
	}
	if outcome.Diff.OldRef != "" {
		t.Fatalf("OldRef = %q; want empty first-sync baseline", outcome.Diff.OldRef)
	}
	if outcome.Diff.NewRef != "2026-04-21T00:00:00Z" {
		t.Fatalf("NewRef = %q; want fresh manifest timestamp", outcome.Diff.NewRef)
	}
	if len(outcome.Diff.Changes) != 0 {
		t.Fatalf("Changes = %#v; want no first-sync item spam", outcome.Diff.Changes)
	}
}

func TestSyncOneNotModifiedHasNoDiff(t *testing.T) {
	root, cacheDir := setupSyncOneDiffTest(t)
	stubSyncOneDiff(t, syncOneDiffResult(t, nil, true))

	outcome, err := SyncOne(context.Background(), "example", SyncOpts{
		LockfileRoot: root,
		CacheDir:     cacheDir,
		Now:          syncOneDiffNow(),
	})
	if err != nil {
		t.Fatalf("SyncOne: %v", err)
	}
	if outcome.Diff != nil {
		t.Fatalf("Diff = %#v; want nil on 304", outcome.Diff)
	}
}

func TestStatusOneNotModifiedIsUpToDateAndReadOnly(t *testing.T) {
	root, cacheDir := setupSyncOneDiffTest(t)
	stubSyncOneDiff(t, syncOneDiffResult(t, nil, true))

	outcome, err := StatusOne(context.Background(), "example", SyncOpts{
		LockfileRoot: root,
		CacheDir:     cacheDir,
		Now:          syncOneDiffNow(),
	})
	if err != nil {
		t.Fatalf("StatusOne: %v", err)
	}
	if !outcome.UpToDate {
		t.Fatalf("UpToDate = false; want true")
	}
	if outcome.Diff != nil {
		t.Fatalf("Diff = %#v; want nil", outcome.Diff)
	}
	assertStatusOneNoPersistence(t, root, `"old"`)
}

func TestStatusOneFreshManifestDiffsCachedBaselineWithoutPersisting(t *testing.T) {
	root, cacheDir := setupSyncOneDiffTest(t)
	oldManifest := syncOneDiffManifest(t, "2026-04-20T00:00:00Z", []moat.ContentEntry{
		syncOneDiffEntry("kept", "1"),
		syncOneDiffEntry("removed", "2"),
	})
	oldBytes := syncOneDiffManifestBytes(t, oldManifest)
	if err := moat.WriteManifestCache(cacheDir, "example", oldBytes, []byte(`{"bundle":true}`)); err != nil {
		t.Fatalf("WriteManifestCache: %v", err)
	}

	newManifest := syncOneDiffManifest(t, "2026-04-21T00:00:00Z", []moat.ContentEntry{
		syncOneDiffEntry("added", "3"),
		syncOneDiffEntry("kept", "4"),
	})
	stubSyncOneDiff(t, syncOneDiffResult(t, newManifest, false))

	outcome, err := StatusOne(context.Background(), "example", SyncOpts{
		LockfileRoot: root,
		CacheDir:     cacheDir,
		Now:          syncOneDiffNow(),
	})
	if err != nil {
		t.Fatalf("StatusOne: %v", err)
	}
	if outcome.NotSynced || outcome.UpToDate || outcome.ProfileChanged {
		t.Fatalf("unexpected gate flags: %+v", outcome)
	}
	if outcome.Diff == nil {
		t.Fatal("Diff = nil; want cached-baseline diff")
	}
	want := []regdiff.ItemChange{
		{Type: "skill", Name: "added", Kind: regdiff.KindAdded},
		{Type: "skill", Name: "kept", Kind: regdiff.KindModified},
		{Type: "skill", Name: "removed", Kind: regdiff.KindRemoved},
	}
	if !reflect.DeepEqual(outcome.Diff.Changes, want) {
		t.Fatalf("Diff.Changes = %#v; want %#v", outcome.Diff.Changes, want)
	}
	assertStatusOneNoPersistence(t, root, `"old"`)
	manifestPath, err := moat.ManifestCachePath(cacheDir, "example")
	if err != nil {
		t.Fatalf("ManifestCachePath: %v", err)
	}
	gotBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile cached manifest: %v", err)
	}
	if string(gotBytes) != string(oldBytes) {
		t.Fatalf("cached manifest was overwritten:\n got %s\nwant %s", gotBytes, oldBytes)
	}
}

func TestStatusOneProfileChangedWithholdsDiff(t *testing.T) {
	root, cacheDir := setupSyncOneDiffTest(t)
	res := syncOneDiffResult(t, syncOneDiffManifest(t, "2026-04-21T00:00:00Z", []moat.ContentEntry{
		syncOneDiffEntry("added", "3"),
	}), false)
	res.ProfileChanged = true
	stubSyncOneDiff(t, res)

	outcome, err := StatusOne(context.Background(), "example", SyncOpts{
		LockfileRoot: root,
		CacheDir:     cacheDir,
		Now:          syncOneDiffNow(),
	})
	if err != nil {
		t.Fatalf("StatusOne: %v", err)
	}
	if !outcome.ProfileChanged {
		t.Fatalf("ProfileChanged = false; want true")
	}
	if outcome.Diff != nil {
		t.Fatalf("Diff = %#v; want nil", outcome.Diff)
	}
	assertStatusOneNoPersistence(t, root, `"old"`)
}

func TestStatusOneTOFUIsNotSynced(t *testing.T) {
	root, cacheDir := setupSyncOneDiffTest(t)
	res := syncOneDiffResult(t, syncOneDiffManifest(t, "2026-04-21T00:00:00Z", []moat.ContentEntry{
		syncOneDiffEntry("first", "1"),
	}), false)
	res.IsTOFU = true
	stubSyncOneDiff(t, res)

	outcome, err := StatusOne(context.Background(), "example", SyncOpts{
		LockfileRoot: root,
		CacheDir:     cacheDir,
		Now:          syncOneDiffNow(),
	})
	if err != nil {
		t.Fatalf("StatusOne: %v", err)
	}
	if !outcome.NotSynced {
		t.Fatalf("NotSynced = false; want true")
	}
	if outcome.Diff != nil {
		t.Fatalf("Diff = %#v; want nil", outcome.Diff)
	}
	assertStatusOneNoPersistence(t, root, `"old"`)
}

func setupSyncOneDiffTest(t *testing.T) (root, cacheDir string) {
	t.Helper()
	root = t.TempDir()
	cacheDir = filepath.Join(root, "cache")

	origGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = filepath.Join(root, "global")
	t.Cleanup(func() { config.GlobalDirOverride = origGlobal })

	profile := syncOneDiffConfigProfile()
	cfg := &config.Config{
		Registries: []config.Registry{
			{
				Name:           "example",
				URL:            "https://registry.example.com/manifest.json",
				Type:           config.RegistryTypeMOAT,
				ManifestURI:    "https://registry.example.com/manifest.json",
				SigningProfile: &profile,
				ManifestETag:   `"old"`,
			},
		},
	}
	if err := config.SaveGlobal(cfg); err != nil {
		t.Fatalf("SaveGlobal: %v", err)
	}
	return root, cacheDir
}

func assertStatusOneNoPersistence(t *testing.T, root, wantETag string) {
	t.Helper()
	reloaded, err := config.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal: %v", err)
	}
	if len(reloaded.Registries) != 1 {
		t.Fatalf("loaded %d registries; want 1", len(reloaded.Registries))
	}
	if reloaded.Registries[0].ManifestETag != wantETag {
		t.Fatalf("ManifestETag = %q; want unchanged %q", reloaded.Registries[0].ManifestETag, wantETag)
	}
	if _, err := os.Stat(moat.LockfilePath(root)); !os.IsNotExist(err) {
		t.Fatalf("lockfile exists after StatusOne: %v", err)
	}
}

func stubSyncOneDiff(t *testing.T, result moat.SyncResult) {
	t.Helper()
	orig := SyncOneFn
	SyncOneFn = func(context.Context, *config.Registry, *moat.Lockfile, []byte, *moat.Fetcher, time.Time) (moat.SyncResult, error) {
		return result, nil
	}
	t.Cleanup(func() { SyncOneFn = orig })
}

func syncOneDiffResult(t *testing.T, manifest *moat.Manifest, notModified bool) moat.SyncResult {
	t.Helper()
	res := moat.SyncResult{
		ManifestURL:     "https://registry.example.com/manifest.json",
		BundleURL:       "https://registry.example.com/manifest.json.sigstore",
		ETag:            `"v2"`,
		NotModified:     notModified,
		FetchedAt:       syncOneDiffNow(),
		IncomingProfile: syncOneDiffConfigProfile(),
		Staleness:       moat.StalenessFresh,
	}
	if manifest != nil {
		res.Manifest = manifest
		res.ManifestBytes = syncOneDiffManifestBytes(t, manifest)
		res.BundleBytes = []byte(`{"bundle":true}`)
	}
	return res
}

func syncOneDiffManifest(t *testing.T, updatedAt string, entries []moat.ContentEntry) *moat.Manifest {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		t.Fatalf("parse updated_at: %v", err)
	}
	return &moat.Manifest{
		SchemaVersion:          moat.ManifestSchemaVersion,
		ManifestURI:            "https://registry.example.com/manifest.json",
		Name:                   "Example Registry",
		Operator:               "Example Operator",
		UpdatedAt:              ts,
		RegistrySigningProfile: syncOneDiffMOATProfile(),
		Content:                entries,
		Revocations:            []moat.Revocation{},
	}
}

func syncOneDiffManifestBytes(t *testing.T, manifest *moat.Manifest) []byte {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return data
}

func syncOneDiffEntry(name, digit string) moat.ContentEntry {
	return moat.ContentEntry{
		Name:        name,
		DisplayName: name,
		Type:        "skill",
		ContentHash: "sha256:" + strings.Repeat(digit, 64),
		SourceURI:   "fixture-source",
		AttestedAt:  syncOneDiffNow(),
	}
}

func syncOneDiffConfigProfile() config.SigningProfile {
	return config.SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "repo:example/registry:ref:refs/heads/main",
	}
}

func syncOneDiffMOATProfile() moat.SigningProfile {
	return moat.SigningProfile{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "repo:example/registry:ref:refs/heads/main",
	}
}

func syncOneDiffNow() time.Time {
	return time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
}
