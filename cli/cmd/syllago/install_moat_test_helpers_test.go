package main

// Test helpers shared by install_moat_*_test.go after the fetch pipeline
// moved into cli/internal/moatinstall and the install path was rewritten
// to clone+tree-hash (bead syllago-cvwj5). The helpers here let cmd/syllago
// integration tests drive runInstallFromRegistry end-to-end without
// spawning git or hitting a real source-repo host.

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/lifecycle"
	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/moatinstall"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// makeRepoFixture creates a fake source-repo tree at
// <root>/<categoryDir>/<name>/<rel> for every (rel, content) in files,
// then returns the spec-correct content_hash for the item subdirectory.
// Mirrors moat-spec.md §"Repository Layout": items live at
// <category>/<name>/, and content_hash covers that subdirectory.
func makeRepoFixture(t *testing.T, root, categoryDir, name string, files map[string]string) string {
	t.Helper()
	itemDir := filepath.Join(root, categoryDir, name)
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatalf("mkdir item: %v", err)
	}
	for rel, content := range files {
		full := filepath.Join(itemDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir parent: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	h, err := moat.ContentHash(itemDir)
	if err != nil {
		t.Fatalf("ContentHash: %v", err)
	}
	return h
}

// stubCloneFromFixture replaces moat.CloneRepoFn with one that
// recursively copies fixtureRoot into the production code's destDir,
// mimicking a successful git clone of fixtureRoot.
func stubCloneFromFixture(t *testing.T, fixtureRoot string) {
	t.Helper()
	orig := moat.CloneRepoFn
	moat.CloneRepoFn = func(_ context.Context, _, destDir string) error {
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}
		return os.CopyFS(destDir, os.DirFS(fixtureRoot))
	}
	t.Cleanup(func() { moat.CloneRepoFn = orig })
}

// stubCloneScratchDir hermetically points clone scratch space at a t.TempDir
// so the moatinstall package never falls back to $TMPDIR during a test.
func stubCloneScratchDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	orig := moatinstall.CloneScratchDir
	moatinstall.CloneScratchDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { moatinstall.CloneScratchDir = orig })
}

// installFromRegistryForTest runs the registry install with the inputs the
// install command derives from its flags: cfg, unless nil, is saved as the global
// config, globalDir becomes the Library, and prov with baseDir becomes the
// one target. A nil prov means the integration test provider, except under
// dryRun, which needs no target.
func installFromRegistryForTest(
	t *testing.T,
	ctx context.Context,
	out, errW io.Writer,
	cfg *config.Config,
	projectRoot, globalDir, registryName, itemName string,
	prov *provider.Provider,
	method installer.InstallMethod,
	baseDir string,
	dryRun bool,
	scan installer.ScanOptions,
	now time.Time,
) error {
	t.Helper()
	if config.GlobalDirOverride == "" {
		config.GlobalDirOverride = t.TempDir()
		t.Cleanup(func() { config.GlobalDirOverride = "" })
	}
	if cfg != nil {
		if err := config.SaveGlobal(cfg); err != nil {
			t.Fatalf("save global config: %v", err)
		}
	}
	withGlobalLibrary(t, globalDir)
	if prov == nil && !dryRun {
		p := integrationTestProvider()
		prov = &p
	}
	var targets []lifecycle.Target
	if prov != nil {
		targets = []lifecycle.Target{{Provider: *prov, BaseDir: baseDir}}
	}
	return runInstallFromRegistry(ctx, out, errW, moatinstall.Request{
		Registry:    registryName,
		Items:       []moatinstall.Item{{Name: itemName}},
		ProjectRoot: projectRoot,
		Targets:     targets,
		Method:      method,
		Scan:        scan,
		DryRun:      dryRun,
	}, now)
}
