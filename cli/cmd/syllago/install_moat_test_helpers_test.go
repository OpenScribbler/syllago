package main

// Test helpers shared by install_moat_*_test.go after the fetch pipeline
// moved into cli/internal/moatinstall and the install path was rewritten
// to clone+tree-hash (bead syllago-cvwj5). The helpers here let cmd/syllago
// integration tests drive runInstallFromRegistry end-to-end without
// spawning git or hitting a real source-repo host.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/moat"
	"github.com/OpenScribbler/syllago/cli/internal/moatinstall"
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

// stubCloneFromFixture replaces moatinstall.CloneRepoFn with one that
// recursively copies fixtureRoot into the production code's destDir,
// mimicking a successful git clone of fixtureRoot.
func stubCloneFromFixture(t *testing.T, fixtureRoot string) {
	t.Helper()
	orig := moatinstall.CloneRepoFn
	moatinstall.CloneRepoFn = func(_ context.Context, _, destDir string) error {
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}
		return os.CopyFS(destDir, os.DirFS(fixtureRoot))
	}
	t.Cleanup(func() { moatinstall.CloneRepoFn = orig })
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
