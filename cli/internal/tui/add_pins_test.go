package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func TestAddSingleItemOverwriteRotatesInstallRecord(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	storePath := filepath.Join(configDir, "installs.json")
	contentRoot := t.TempDir()
	libraryPath := filepath.Join(contentRoot, string(catalog.Skills), "writer")
	coord, oldHash := seedTUILibraryWriter(t, storePath, libraryPath)
	item := overwriteWriterItem(t)

	result := addSingleItem(item, nil, contentRoot, "", "acme/tools", "private", "", "sha-new")
	if result.status != "updated" {
		t.Fatalf("status = %q err=%v, want updated", result.status, result.err)
	}

	rec := mustLoadTUIInstallRecordStore(t, configDir).Find(coord)
	if rec == nil {
		t.Fatal("install record missing")
	}
	if rec.SourceSHA != "sha-new" {
		t.Fatalf("SourceSHA = %q, want sha-new", rec.SourceSHA)
	}
	if rec.Previous == nil {
		t.Fatal("Previous is nil")
	}
	if rec.Previous.SourceSHA != "sha-old" {
		t.Fatalf("Previous.SourceSHA = %q, want sha-old", rec.Previous.SourceSHA)
	}
	if rec.Previous.ContentHash != oldHash {
		t.Fatalf("Previous.ContentHash = %q, want %q", rec.Previous.ContentHash, oldHash)
	}
}

// Regression: the Add wizard wrote over a pinned library item and the
// record update refused it silently, leaving the pin on new content.
func TestAddSingleItemOverwriteLeavesPinnedItem(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	storePath := filepath.Join(configDir, "installs.json")
	contentRoot := t.TempDir()
	libraryPath := filepath.Join(contentRoot, string(catalog.Skills), "writer")
	coord, oldHash := seedTUILibraryWriter(t, storePath, libraryPath)
	if err := installstore.SetPinned(storePath, coord, true, time.Date(2026, 8, 24, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SetPinned: %v", err)
	}

	// The new content comes from another registry: the pin belongs to the
	// item already in the Library, whatever the incoming source.
	result := addSingleItem(overwriteWriterItem(t), nil, contentRoot, "", "other/tools", "private", "", "sha-new")
	if result.status != "pinned" {
		t.Fatalf("status = %q err=%v, want pinned", result.status, result.err)
	}
	if got := mustHashTUIInstallContent(t, libraryPath); got != oldHash {
		t.Errorf("library content hash = %s, want it unchanged at %s", got, oldHash)
	}
	rec := mustLoadTUIInstallRecordStore(t, configDir).Find(coord)
	if rec == nil || !rec.Pinned || rec.SourceSHA != "sha-old" || rec.Previous != nil {
		t.Errorf("record = %+v, want it pinned and unchanged at sha-old", rec)
	}
}

// Regression: a review-step display name made the pin check look at a
// destination that did not exist while the write landed on the pinned one.
func TestAddSingleItemDisplayNameStillChecksPin(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	storePath := filepath.Join(configDir, "installs.json")
	contentRoot := t.TempDir()
	libraryPath := filepath.Join(contentRoot, string(catalog.Skills), "writer")
	coord, oldHash := seedTUILibraryWriter(t, storePath, libraryPath)
	if err := installstore.SetPinned(storePath, coord, true, time.Date(2026, 8, 24, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SetPinned: %v", err)
	}
	item := overwriteWriterItem(t)
	item.name = "Writer Display"

	result := addSingleItem(item, nil, contentRoot, "", "acme/tools", "private", "", "sha-new")
	if result.status != "pinned" {
		t.Fatalf("status = %q err=%v, want pinned", result.status, result.err)
	}
	if got := mustHashTUIInstallContent(t, libraryPath); got != oldHash {
		t.Errorf("library content hash = %s, want it unchanged at %s", got, oldHash)
	}
}

// Regression: adding a registry item straight to the Library wrote over a
// pinned item added since the catalog loaded.
func TestHandleLibraryAddLeavesPinnedItem(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	storePath := filepath.Join(configDir, "installs.json")
	globalDir := t.TempDir()
	orig := catalog.GlobalContentDirOverride
	catalog.GlobalContentDirOverride = globalDir
	t.Cleanup(func() { catalog.GlobalContentDirOverride = orig })
	libraryPath := filepath.Join(globalDir, string(catalog.Skills), "writer")
	coord, oldHash := seedTUILibraryWriter(t, storePath, libraryPath)
	if err := installstore.SetPinned(storePath, coord, true, time.Date(2026, 8, 24, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SetPinned: %v", err)
	}
	src := filepath.Join(t.TempDir(), "writer")
	writeTUITestFile(t, filepath.Join(src, "SKILL.md"), []byte("# Writer from clone\n"))

	app := testApp(t)
	_, cmd := app.handleLibraryAdd(&catalog.ContentItem{
		Name: "writer", Type: catalog.Skills, Path: src, Files: []string{"SKILL.md"}, Registry: "other/registry",
	}, false)
	if cmd == nil {
		t.Fatal("handleLibraryAdd returned no command")
	}
	msg, ok := cmd().(libraryAddDoneMsg)
	if !ok || msg.err == nil || !strings.Contains(msg.err.Error(), "pinned") {
		t.Fatalf("msg = %+v, want a pinned error", msg)
	}
	if got := mustHashTUIInstallContent(t, libraryPath); got != oldHash {
		t.Errorf("library content hash = %s, want it unchanged at %s", got, oldHash)
	}
}

// Regression: updating an outdated settings hook left its install record
// at the old version, because the hook writer reports it as "added".
func TestAddSingleItemOutdatedHookRotatesInstallRecord(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	storePath := filepath.Join(configDir, "installs.json")
	contentRoot := t.TempDir()
	libraryPath := filepath.Join(contentRoot, string(catalog.Hooks), "claude-code", "my-hook")
	writeTUITestFile(t, filepath.Join(libraryPath, "hook.json"), []byte("{}\n"))
	if err := metadata.Save(libraryPath, &metadata.Meta{Name: "my-hook", SourceType: "registry", SourceRegistry: "acme/tools"}); err != nil {
		t.Fatalf("metadata.Save: %v", err)
	}
	coord := installstore.Coord{Registry: "acme/tools", Type: string(catalog.Hooks), Name: "my-hook"}
	if err := installstore.RecordInstallMeta(storePath, coord, libraryPath, installstore.PlacementInput{
		Provider:  "claude-code",
		Mechanism: installstore.MechanismHookMerge,
		Path:      filepath.Join(t.TempDir(), "settings.json"),
	}, installstore.InstallMeta{SourceSHA: "sha-old"}, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("seed RecordInstallMeta: %v", err)
	}
	hook := converter.HookData{
		Event: "before_tool_execute",
		Hooks: []converter.HookEntry{{Type: "command", Command: "echo updated"}},
	}
	item := addDiscoveryItem{
		name:       "my-hook",
		itemType:   catalog.Hooks,
		overwrite:  true,
		status:     add.StatusOutdated,
		hookData:   &hook,
		underlying: &add.DiscoveryItem{Name: "my-hook", Type: catalog.Hooks, Status: add.StatusOutdated},
	}

	result := addSingleItem(item, nil, contentRoot, "", "acme/tools", "private", "claude-code", "sha-new")
	if result.err != nil {
		t.Fatalf("addSingleItem: status=%q err=%v", result.status, result.err)
	}
	rec := mustLoadTUIInstallRecordStore(t, configDir).Find(coord)
	if rec == nil || rec.SourceSHA != "sha-new" || rec.Previous == nil || rec.Previous.SourceSHA != "sha-old" {
		t.Fatalf("record = %+v, want sha-new with sha-old in Previous", rec)
	}
}

// seedTUILibraryWriter puts a writer skill from acme/tools in the library
// with an install record, and returns the record's coord and content hash.
func seedTUILibraryWriter(t *testing.T, storePath, libraryPath string) (installstore.Coord, string) {
	t.Helper()
	writeTUITestFile(t, filepath.Join(libraryPath, "SKILL.md"), []byte("# Writer\n"))
	if err := metadata.Save(libraryPath, &metadata.Meta{Name: "writer", SourceType: "registry", SourceRegistry: "acme/tools"}); err != nil {
		t.Fatalf("metadata.Save: %v", err)
	}
	coord := installstore.Coord{Registry: "acme/tools", Type: string(catalog.Skills), Name: "writer"}
	if err := installstore.RecordInstallMeta(storePath, coord, libraryPath, installstore.PlacementInput{
		Provider:  "claude-code",
		Mechanism: installstore.MechanismSymlink,
		Path:      filepath.Join(t.TempDir(), "writer"),
	}, installstore.InstallMeta{SourceSHA: "sha-old"}, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("seed RecordInstallMeta: %v", err)
	}
	return coord, mustHashTUIInstallContent(t, libraryPath)
}

// overwriteWriterItem is a newer writer skill the user chose to overwrite.
func overwriteWriterItem(t *testing.T) addDiscoveryItem {
	t.Helper()
	sourcePath := filepath.Join(t.TempDir(), "SKILL.md")
	writeTUITestFile(t, sourcePath, []byte("# Writer updated\n"))
	return addDiscoveryItem{
		name:      "writer",
		itemType:  catalog.Skills,
		overwrite: true,
		underlying: &add.DiscoveryItem{
			Name:   "writer",
			Type:   catalog.Skills,
			Path:   sourcePath,
			Status: add.StatusOutdated,
		},
	}
}

func withTUIInstallRecordConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig := config.GlobalDirOverride
	config.GlobalDirOverride = dir
	t.Cleanup(func() { config.GlobalDirOverride = orig })
	return dir
}

func writeTUITestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

func mustLoadTUIInstallRecordStore(t *testing.T, configDir string) *installstore.Store {
	t.Helper()
	store, err := installstore.Load(filepath.Join(configDir, "installs.json"))
	if err != nil {
		t.Fatalf("Load install store: %v", err)
	}
	return store
}

func mustHashTUIInstallContent(t *testing.T, path string) string {
	t.Helper()
	hash, err := installstore.HashContent(path)
	if err != nil {
		t.Fatalf("HashContent(%s): %v", path, err)
	}
	return hash
}
