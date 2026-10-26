package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/syllagolock"
)

func splitOverwriteItem(t *testing.T) addDiscoveryItem {
	t.Helper()
	path := writeSplittableClaudeMD(t, t.TempDir())
	return addDiscoveryItem{
		name: "CLAUDE", itemType: catalog.Rules, path: path, splittable: true, splitChosen: true, overwrite: true,
		underlying: &add.DiscoveryItem{Name: "CLAUDE", Type: catalog.Rules, Path: path},
	}
}

func seedSplitOverwriteRecord(t *testing.T, configDir, contentRoot, name string, pinned bool) installstore.Record {
	t.Helper()
	dest := filepath.Join(contentRoot, "rules", "claude-code", name)
	writeTUITestFile(t, filepath.Join(dest, "rule.md"), []byte("# Old section\n"))
	writeTUITestFile(t, filepath.Join(dest, ".source", "CLAUDE.md"), []byte("old source\n"))
	if err := metadata.Save(dest, &metadata.Meta{Name: name, SourceType: "registry", SourceRegistry: "old/registry"}); err != nil {
		t.Fatal(err)
	}
	coord := installstore.Coord{Registry: "old/registry", Type: "rules", Name: name}
	storePath := filepath.Join(configDir, "installs.json")
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	if err := installstore.RecordInstallMeta(storePath, coord, dest, installstore.PlacementInput{
		Provider: "claude-code", Mechanism: installstore.MechanismCopy, Path: filepath.Join(t.TempDir(), name),
	}, installstore.InstallMeta{SourceSHA: "sha-old"}, now); err != nil {
		t.Fatal(err)
	}
	if pinned {
		if err := installstore.SetPinned(storePath, coord, true, now); err != nil {
			t.Fatal(err)
		}
	}
	return *mustLoadTUIInstallRecordStore(t, configDir).Find(coord)
}

func TestAddSingleItemSplitOverwriteSkipsPinsAndRotatesRecords(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	contentRoot := t.TempDir()
	held := seedSplitOverwriteRecord(t, configDir, contentRoot, "section-a", true)
	old := seedSplitOverwriteRecord(t, configDir, contentRoot, "section-b", false)
	metaBefore, err := os.ReadFile(metadata.MetaPath(held.LibraryPath))
	if err != nil {
		t.Fatal(err)
	}

	result := addSingleItem(splitOverwriteItem(t), contentRoot, "", "", "", "claude-code", "")
	if result.status != "added" || result.err != nil || !strings.Contains(result.name, "2 sections") {
		t.Fatalf("result = %+v, want two sections added", result)
	}
	if got := mustHashTUIInstallContent(t, held.LibraryPath); got != held.ContentHash {
		t.Error("pinned section content changed")
	}
	if got, _ := os.ReadFile(metadata.MetaPath(held.LibraryPath)); string(got) != string(metaBefore) {
		t.Error("pinned section metadata changed")
	}
	if got, _ := os.ReadFile(filepath.Join(held.LibraryPath, ".source", "CLAUDE.md")); string(got) != "old source\n" {
		t.Error("pinned section source changed")
	}
	store := mustLoadTUIInstallRecordStore(t, configDir)
	if rec := store.Find(held.Coord); !reflect.DeepEqual(rec, &held) {
		t.Errorf("pinned record changed: %+v", rec)
	}
	if rec := store.Find(old.Coord); rec == nil || rec.Previous == nil || rec.Previous.SourceSHA != "sha-old" || rec.Previous.ContentHash != old.ContentHash || rec.ContentHash == old.ContentHash {
		t.Errorf("replaced record = %+v, want old version in Previous", rec)
	}
	if _, err := os.Stat(filepath.Join(contentRoot, "rules", "claude-code", "section-c", "rule.md")); err != nil {
		t.Errorf("new section was not written: %v", err)
	}
}

func TestAddSingleItemSplitOverwriteAllPinned(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	contentRoot := t.TempDir()
	var records []installstore.Record
	for _, name := range []string{"section-a", "section-b", "section-c"} {
		records = append(records, seedSplitOverwriteRecord(t, configDir, contentRoot, name, true))
	}
	result := addSingleItem(splitOverwriteItem(t), contentRoot, "", "", "", "claude-code", "")
	if result.status != "pinned" || result.err != nil {
		t.Errorf("result = %+v, want pinned", result)
	}
	store := mustLoadTUIInstallRecordStore(t, configDir)
	for _, old := range records {
		if got := mustHashTUIInstallContent(t, old.LibraryPath); got != old.ContentHash {
			t.Errorf("pinned %s content changed", old.Coord.Name)
		}
		if rec := store.Find(old.Coord); !reflect.DeepEqual(rec, &old) {
			t.Errorf("pinned record changed: %+v", rec)
		}
	}
}

func TestAddSingleItemSplitOverwriteSameDestinationRotatesOnce(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	contentRoot := t.TempDir()
	old := seedSplitOverwriteRecord(t, configDir, contentRoot, "section-a", false)
	item := splitOverwriteItem(t)
	source, err := os.ReadFile(item.path)
	if err != nil {
		t.Fatal(err)
	}
	source = []byte(strings.NewReplacer("Section B", "Section A", "Section C", "Section A").Replace(string(source)))
	writeTUITestFile(t, item.path, source)

	result := addSingleItem(item, contentRoot, "", "", "", "claude-code", "")
	if result.status != "added" || result.err != nil {
		t.Fatalf("result = %+v, want added", result)
	}
	rec := mustLoadTUIInstallRecordStore(t, configDir).Find(old.Coord)
	if rec == nil || rec.Previous == nil || rec.Previous.SourceSHA != old.SourceSHA || rec.Previous.ContentHash != old.ContentHash {
		t.Errorf("record = %+v, want the original version in Previous", rec)
	}
}

func TestAddSingleItemSplitOverwriteHoldsInstallLock(t *testing.T) {
	withTUIInstallRecordConfigDir(t)
	contentRoot := t.TempDir()
	release, err := syllagolock.Acquire(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	original := syllagolock.DefaultTimeout
	syllagolock.DefaultTimeout = time.Millisecond
	defer func() { syllagolock.DefaultTimeout = original }()

	result := addSingleItem(splitOverwriteItem(t), contentRoot, "", "", "", "claude-code", "")
	if result.status != "error" || result.err == nil {
		t.Errorf("result = %+v, want lock error", result)
	}
	if entries, err := os.ReadDir(contentRoot); err != nil || len(entries) != 0 {
		t.Errorf("Library changed while the install lock was held: %v, %v", entries, err)
	}
}

func TestAddSingleItemOverwriteSurfacesRecordFailure(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	contentRoot := t.TempDir()
	libraryPath := filepath.Join(contentRoot, "skills", "writer")
	seedTUILibraryWriter(t, filepath.Join(configDir, "installs.json"), libraryPath)
	// The content writer can update this directory through a symlink, but
	// the install store refuses to hash one, so rotation fails after the write.
	target := filepath.Join(t.TempDir(), "writer")
	if err := os.Rename(libraryPath, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, libraryPath); err != nil {
		t.Fatal(err)
	}

	result := addSingleItem(overwriteWriterItem(t), contentRoot, "", "acme/tools", "private", "", "sha-new")
	if result.status != "error" || result.err == nil || !strings.Contains(result.err.Error(), "could not record install state") {
		t.Errorf("result = %+v, want record failure", result)
	}
	if got, _ := os.ReadFile(filepath.Join(target, "SKILL.md")); string(got) != "# Writer updated\n" {
		t.Errorf("content = %q, want the completed write left in place", got)
	}
}
