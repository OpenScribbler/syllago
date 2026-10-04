package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/syllagolock"
)

// A held lock reaches the user as the lock, not as a write failure.
func TestInstallAppend_LockHeldReportsLock(t *testing.T) {
	projectRoot := t.TempDir()
	globalDir := t.TempDir()
	seedLibraryRule(t, globalDir, "claude-code", "foo", "# foo rule body\n")

	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return projectRoot, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })
	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })
	_, _ = output.SetForTest(t)
	holdInstallLock(t)

	installCmd.Flags().Set("to", "claude-code")
	installCmd.Flags().Set("method", "append")
	installCmd.Flags().Set("type", "rules")
	t.Cleanup(func() {
		installCmd.Flags().Set("to", "")
		installCmd.Flags().Set("method", "symlink")
		installCmd.Flags().Set("type", "")
	})

	requireLockedError(t, installCmd.RunE(installCmd, []string{"foo"}))
}

func TestInstallFromRegistry_LockHeldReportsLock(t *testing.T) {
	env := setupIntegrationEnv(t)
	globalDir := t.TempDir()
	withGlobalLibrary(t, globalDir)
	withInstallRecordConfigDir(t)
	output.SetForTest(t)
	t.Setenv("HOME", env.projectRoot)

	fixtureRoot := t.TempDir()
	contentHash := makeRepoFixture(t, fixtureRoot, "skills", "my-skill", map[string]string{
		"SKILL.md": "# from registry\n",
	})
	stubCloneFromFixture(t, fixtureRoot)
	now := time.Date(2026, 8, 22, 14, 0, 0, 0, time.UTC)
	env.syncResultFn = func() (moat.SyncResult, error) {
		return moat.SyncResult{
			ManifestURL: "https://example.com/manifest.json",
			Manifest: &moat.Manifest{Content: []moat.ContentEntry{{
				Name:        "my-skill",
				Type:        "skill",
				ContentHash: contentHash,
				SourceURI:   "https://github.com/example/repo",
				AttestedAt:  now,
			}}},
			IncomingProfile: incomingProfile(),
			Staleness:       moat.StalenessFresh,
		}, nil
	}
	holdInstallLock(t)

	prov := integrationTestProvider()
	err := installFromRegistryForTest(t, context.Background(), &bytes.Buffer{}, &bytes.Buffer{},
		cfgWithPinnedMOATRegistry(t), env.projectRoot, globalDir, "example", "my-skill",
		&prov, installer.MethodSymlink, "", false, installer.ScanOptions{}, now)
	requireLockedError(t, err)
}

// Once the lock is unavailable, the remaining items are skipped without
// waiting out the lock again for each one.
func TestInstallToProvider_LockHeldSkipsRestOnce(t *testing.T) {
	holdInstallLock(t)
	syllagolock.DefaultTimeout = 300 * time.Millisecond
	projectRoot := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	prov := integrationTestProvider()
	var items []catalog.ContentItem
	for _, name := range []string{"a", "b", "c", "d"} {
		dir := filepath.Join(t.TempDir(), name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		items = append(items, catalog.ContentItem{Name: name, Type: catalog.Skills, Path: dir})
	}
	_, _ = output.SetForTest(t)

	start := time.Now()
	result, _ := installToProvider(items, prov, installer.MethodSymlink,
		false, config.NewResolver(nil, ""), prov.Slug, projectRoot, false, installer.ScanOptions{})
	elapsed := time.Since(start)

	if len(result.Skipped) != len(items) || len(result.Installed) != 0 {
		t.Fatalf("result = %+v, want every item skipped", result)
	}
	for _, s := range result.Skipped {
		if !strings.Contains(s.Reason, "another syllago process") {
			t.Errorf("skip reason = %q, want the lock", s.Reason)
		}
	}
	if elapsed > 2*syllagolock.DefaultTimeout {
		t.Errorf("took %v, want one lock wait of %v", elapsed, syllagolock.DefaultTimeout)
	}
}
