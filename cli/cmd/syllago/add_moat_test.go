package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/moatinstall"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// moatAddEnv is the MOAT registry "example", whose manifest lists the
// skill "my-skill" from a local fixture, and an empty Library.
type moatAddEnv struct {
	library string
	entry   moat.ContentEntry
	clones  int
}

func setupMOATAdd(t *testing.T) *moatAddEnv {
	t.Helper()
	env := setupIntegrationEnv(t)
	e := &moatAddEnv{library: t.TempDir()}
	withGlobalLibrary(t, e.library)
	withInstallRecordConfigDir(t)
	cfgWithPinnedMOATRegistry(t)
	t.Setenv("HOME", env.projectRoot)
	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return env.projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })
	origPrev := moatinstall.PreviousRootOverride
	moatinstall.PreviousRootOverride = t.TempDir()
	t.Cleanup(func() { moatinstall.PreviousRootOverride = origPrev })

	fixture := t.TempDir()
	hash := makeRepoFixture(t, fixture, "skills", "my-skill", map[string]string{"SKILL.md": "# from registry\n"})
	stubCloneFromFixture(t, fixture)
	stubbed := moat.CloneRepoFn
	moat.CloneRepoFn = func(ctx context.Context, url, dest string) error {
		e.clones++
		return stubbed(ctx, url, dest)
	}

	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	origNow := moatInstallNow
	moatInstallNow = func() time.Time { return now }
	t.Cleanup(func() { moatInstallNow = origNow })
	e.entry = moat.ContentEntry{
		Name: "my-skill", Type: "skill", ContentHash: hash,
		SourceURI: "https://github.com/example/repo", AttestedAt: now,
	}
	env.syncResultFn = func() (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL:     "https://example.com/m",
			Manifest:        &moat.Manifest{Content: []moat.ContentEntry{e.entry}},
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
		}, nil
	}
	return e
}

// add runs `syllago add [args] --from example` with the flags given as
// name=value pairs, and resets the flags after.
func (e *moatAddEnv) add(t *testing.T, args []string, flags ...string) (string, error) {
	t.Helper()
	stdout, _ := output.SetForTest(t)
	flags = append(flags, "from=example")
	for _, f := range flags {
		name, value, _ := strings.Cut(f, "=")
		def := addCmd.Flags().Lookup(name).DefValue
		if err := addCmd.Flags().Set(name, value); err != nil {
			t.Fatal(err)
		}
		defer addCmd.Flags().Set(name, def)
	}
	err := addCmd.RunE(addCmd, args)
	return stdout.String(), err
}

func (e *moatAddEnv) skillDir() string {
	return filepath.Join(e.library, string(catalog.Skills), "my-skill")
}

// Regression: `add --from` refused every MOAT registry as not cloned, and
// the TUI copied MOAT items from the cache without verifying them.
func TestAddFromMOATRegistry_AddsVerifiedItem(t *testing.T) {
	e := setupMOATAdd(t)
	out, err := e.add(t, []string{"skills/my-skill"})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.Contains(out, "added") {
		t.Errorf("output %q, want the item reported added", out)
	}
	meta, err := metadata.Load(e.skillDir())
	if err != nil || meta == nil {
		t.Fatalf("library metadata: %v", err)
	}
	if meta.SourceRegistry != "example" || meta.SourceHash != e.entry.ContentHash {
		t.Errorf("meta source %q hash %q, want example and the manifest's hash", meta.SourceRegistry, meta.SourceHash)
	}
	if _, err := os.Lstat(filepath.Join(os.Getenv("HOME"), ".testprovider")); err == nil {
		t.Error("an add installed to a provider")
	}

	// A second add finds the item up to date and fetches nothing.
	out, err = e.add(t, []string{"skills/my-skill"})
	if err != nil || !strings.Contains(out, "up to date") || e.clones != 1 {
		t.Errorf("second add: err=%v clones=%d output %q, want up to date with no fetch", err, e.clones, out)
	}
}

func TestAddFromMOATRegistry_HashMismatchRefused(t *testing.T) {
	e := setupMOATAdd(t)
	e.entry.ContentHash = "sha256:" + strings.Repeat("0", 64)
	_, err := e.add(t, nil, "all=true")
	if err == nil {
		t.Fatal("add of content that does not match the manifest's hash succeeded")
	}
	if _, statErr := os.Stat(e.skillDir()); statErr == nil {
		t.Error("the unverified item reached the Library")
	}
}

func TestAddFromMOATRegistry_Discovery(t *testing.T) {
	e := setupMOATAdd(t)
	out, err := e.add(t, nil)
	if err != nil || !strings.Contains(out, "my-skill") || !strings.Contains(out, "(new)") {
		t.Fatalf("discovery: err=%v output %q, want my-skill listed as new", err, out)
	}
	if e.clones != 0 {
		t.Errorf("discovery fetched %d items, want none", e.clones)
	}
	if _, err := e.add(t, nil, "all=true"); err != nil {
		t.Fatalf("add --all: %v", err)
	}
	out, _ = e.add(t, nil)
	if !strings.Contains(out, "(in library)") {
		t.Errorf("discovery after add: %q, want my-skill in library", out)
	}
}

func TestAddFromMOATRegistry_ItemNotListed(t *testing.T) {
	e := setupMOATAdd(t)
	_, err := e.add(t, []string{"skills/nope"})
	assertStructuredCode(t, err, output.ErrItemNotFound)
}

func TestAddFromMOATRegistry_OutdatedNeedsForce(t *testing.T) {
	e := setupMOATAdd(t)
	if _, err := e.add(t, nil, "all=true"); err != nil {
		t.Fatalf("first add: %v", err)
	}
	meta, _ := metadata.Load(e.skillDir())
	meta.SourceHash = "sha256:" + strings.Repeat("1", 64)
	if err := metadata.Save(e.skillDir(), meta); err != nil {
		t.Fatal(err)
	}

	out, err := e.add(t, nil, "all=true")
	if err != nil || !strings.Contains(out, "use --force") || e.clones != 1 {
		t.Fatalf("outdated add: err=%v clones=%d output %q, want it skipped unfetched", err, e.clones, out)
	}
	out, err = e.add(t, nil, "all=true", "force=true")
	if err != nil || !strings.Contains(out, "updated") {
		t.Fatalf("forced add: err=%v output %q, want it updated", err, out)
	}
}

func TestAddFromMOATRegistry_TrustedRootRefused(t *testing.T) {
	e := setupMOATAdd(t)
	_, err := e.add(t, nil, "all=true", "trusted-root=/tmp/root.json")
	assertStructuredCode(t, err, output.ErrInputConflict)
}
