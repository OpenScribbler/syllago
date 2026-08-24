package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/registry"
)

func TestPinCommand_NonRegistry(t *testing.T) {
	// Set GlobalDirOverride to temp
	tmpDir := t.TempDir()
	prevGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = tmpDir
	t.Cleanup(func() { config.GlobalDirOverride = prevGlobal })

	// Setup global library and withGlobalLibrary
	globalDir := filepath.Join(tmpDir, "content")
	os.MkdirAll(filepath.Join(globalDir, "skills", "non-reg-skill"), 0755)
	os.WriteFile(filepath.Join(globalDir, "skills", "non-reg-skill", "SKILL.md"), []byte("# SKILL\n"), 0644)
	withGlobalLibrary(t, globalDir)

	_, _ = output.SetForTest(t)

	err := pinCmd.RunE(pinCmd, []string{"non-reg-skill"})
	if err == nil {
		t.Fatal("expected error when pinning a non-registry item, got nil")
	}
	if !strings.Contains(err.Error(), "only registry items can be pinned") {
		t.Errorf("expected only registry items can be pinned error, got: %v", err)
	}
}

func TestPinCommand_RegistryAndUnpin(t *testing.T) {
	// Set GlobalDirOverride to temp
	tmpDir := t.TempDir()
	prevGlobal := config.GlobalDirOverride
	config.GlobalDirOverride = tmpDir
	t.Cleanup(func() { config.GlobalDirOverride = prevGlobal })

	// Setup global library and withGlobalLibrary
	globalDir := filepath.Join(tmpDir, "content")
	os.MkdirAll(filepath.Join(globalDir, "skills", "my-reg-skill"), 0755)
	os.WriteFile(filepath.Join(globalDir, "skills", "my-reg-skill", "SKILL.md"), []byte("# SKILL\n"), 0644)
	os.WriteFile(filepath.Join(globalDir, "skills", "my-reg-skill", ".syllago.yaml"), []byte("source_type: registry\nsource_registry: my-reg\n"), 0644)
	withGlobalLibrary(t, globalDir)

	stdout, _ := output.SetForTest(t)

	// Seeding the install record first
	path, err := installstore.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	// Make sure the store dir exists
	os.MkdirAll(filepath.Dir(path), 0755)
	store, err := installstore.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	coord := installstore.Coord{Registry: "my-reg", Type: "skills", Name: "my-reg-skill"}
	store.Records = append(store.Records, installstore.Record{
		Coord:       coord,
		ContentHash: "fakehash",
		SourceSHA:   "abcdef1234567890",
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	})
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Run pinCmd
	stdout.Reset()
	if err := pinCmd.RunE(pinCmd, []string{"my-reg-skill"}); err != nil {
		t.Fatalf("pinCmd failed: %v", err)
	}

	gotOut := stdout.String()
	expectedPinOut := "Pinned skills/my-reg-skill — holding at abcdef123456\n"
	if gotOut != expectedPinOut {
		t.Errorf("stdout = %q; want %q", gotOut, expectedPinOut)
	}

	// Verify in store
	store, _ = installstore.Load(path)
	rec := store.Find(coord)
	if rec == nil {
		t.Fatal("expected record to be found in store")
	}
	if !rec.Pinned {
		t.Error("expected rec.Pinned to be true")
	}

	// Run unpinCmd
	stdout.Reset()
	if err := unpinCmd.RunE(unpinCmd, []string{"my-reg-skill"}); err != nil {
		t.Fatalf("unpinCmd failed: %v", err)
	}

	gotUnpinOut := stdout.String()
	expectedUnpinOut := "Unpinned skills/my-reg-skill\n"
	if gotUnpinOut != expectedUnpinOut {
		t.Errorf("stdout = %q; want %q", gotUnpinOut, expectedUnpinOut)
	}

	// Verify in store
	store, _ = installstore.Load(path)
	rec = store.Find(coord)
	if rec == nil {
		t.Fatal("expected record to be found in store")
	}
	if rec.Pinned {
		t.Error("expected rec.Pinned to be false")
	}
}

func TestPreOverwritePinGuard_Add(t *testing.T) {
	const regName = "test-org/test-registry"

	cacheDir := t.TempDir()
	origCache := registry.CacheDirOverride
	registry.CacheDirOverride = cacheDir
	t.Cleanup(func() { registry.CacheDirOverride = origCache })

	setupRegistryClone(t, cacheDir, regName)

	root := setupProjectWithRegistry(t, regName)
	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	globalDir := t.TempDir()
	origGlobal := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = origGlobal })

	// Seed the install store in the temp global dir (so DefaultPath points to it)
	tmpDir := t.TempDir()
	prevGlobalDir := config.GlobalDirOverride
	config.GlobalDirOverride = tmpDir
	t.Cleanup(func() { config.GlobalDirOverride = prevGlobalDir })

	path, err := installstore.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	os.MkdirAll(filepath.Dir(path), 0755)
	store, err := installstore.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	coord := installstore.Coord{Registry: regName, Type: "skills", Name: "canary-skill"}
	store.Records = append(store.Records, installstore.Record{
		Coord:       coord,
		ContentHash: "fakehash",
		SourceSHA:   "abcdef1234567890",
		Pinned:      true,
		PinnedAt:    time.Now(),
	})
	if err := store.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, stderr := output.SetForTest(t)

	addCmd.Flags().Set("from", regName)
	addCmd.Flags().Set("force", "false")
	addCmd.Flags().Set("dry-run", "false")
	t.Cleanup(func() {
		addCmd.Flags().Set("from", "")
		addCmd.Flags().Set("force", "false")
		addCmd.Flags().Set("dry-run", "false")
	})

	err = addCmd.RunE(addCmd, []string{"skills/canary-skill"})
	if err == nil {
		t.Fatal("expected ErrInstallConflict when adding a pinned item, got nil")
	}

	expectedWarning := "  pinned skills/canary-skill — holding at abcdef123456; unpin to update: syllago unpin canary-skill"
	if !strings.Contains(stderr.String(), expectedWarning) {
		t.Errorf("expected warning about skipping add in stderr, got:\n%s", stderr.String())
	}
}

func TestAddFrozenFlagValidation(t *testing.T) {
	root := t.TempDir()
	origRoot := findProjectRoot
	findProjectRoot = func() (string, error) { return root, nil }
	t.Cleanup(func() { findProjectRoot = origRoot })

	_, _ = output.SetForTest(t)

	t.Cleanup(func() {
		addCmd.Flags().Set("from", "")
		addCmd.Flags().Set("frozen", "false")
		addCmd.Flags().Set("install", "false")
		addCmd.Flags().Set("to", "")
	})

	// --frozen without --install/--to must error, not silently drop the pin.
	addCmd.Flags().Set("from", "claude-code")
	addCmd.Flags().Set("frozen", "true")
	err := addCmd.RunE(addCmd, []string{"skills/foo"})
	if err == nil || !strings.Contains(err.Error(), "--frozen requires --install and --to") {
		t.Errorf("expected frozen-requires-install error, got: %v", err)
	}

	// --frozen on a registry add must error: that branch never chains an install.
	addCmd.Flags().Set("install", "true")
	addCmd.Flags().Set("to", "claude-code")
	addCmd.Flags().Set("from", "some-org/some-registry")
	err = addCmd.RunE(addCmd, []string{"skills/foo"})
	if err == nil || !strings.Contains(err.Error(), "not supported when adding from a registry") {
		t.Errorf("expected registry frozen error, got: %v", err)
	}
}
