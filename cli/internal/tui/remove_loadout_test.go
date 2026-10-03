package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/output"
	"github.com/OpenScribbler/syllago/cli/internal/syllagolock"
)

// seedLibraryLoadout puts a loadout in the library with an install record
// and returns the item and its record's coord.
func seedLibraryLoadout(t *testing.T, storePath string) (catalog.ContentItem, installstore.Coord) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), string(catalog.Loadouts), "claude-code", "starter")
	writeTUITestFile(t, filepath.Join(dir, "loadout.yaml"), []byte("name: starter\n"))
	coord := installstore.Coord{Type: string(catalog.Loadouts), Name: "starter"}
	if err := installstore.RecordInstallMeta(storePath, coord, dir, installstore.PlacementInput{
		Provider:  "claude-code",
		Mechanism: installstore.MechanismCopy,
		Path:      filepath.Join(t.TempDir(), "starter"),
	}, installstore.InstallMeta{}, time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("seed RecordInstallMeta: %v", err)
	}
	return catalog.ContentItem{Name: "starter", Type: catalog.Loadouts, Path: dir, Library: true}, coord
}

func TestDoSimpleRemoveCmd_RemovesLoadoutAndRecord(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	t.Setenv("HOME", t.TempDir())
	item, coord := seedLibraryLoadout(t, filepath.Join(configDir, "installs.json"))

	msg := testApp(t).doSimpleRemoveCmd(item)().(removeDoneMsg)
	if msg.err != nil {
		t.Fatalf("remove: %v", msg.err)
	}
	if _, err := os.Lstat(item.Path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("loadout still in the library: %v", err)
	}
	if rec := mustLoadTUIInstallRecordStore(t, configDir).Find(coord); rec != nil {
		t.Errorf("record = %+v, want it forgotten", rec)
	}
}

// Regression: removing a loadout from the TUI deleted it without taking the
// install lock.
func TestDoSimpleRemoveCmd_WaitsForInstallLock(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	t.Setenv("HOME", t.TempDir())
	item, coord := seedLibraryLoadout(t, filepath.Join(configDir, "installs.json"))
	origTimeout := syllagolock.DefaultTimeout
	syllagolock.DefaultTimeout = 50 * time.Millisecond
	t.Cleanup(func() { syllagolock.DefaultTimeout = origTimeout })
	release, err := syllagolock.Acquire(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)

	msg := testApp(t).doSimpleRemoveCmd(item)().(removeDoneMsg)
	var se output.StructuredError
	if !errors.As(msg.err, &se) || se.Code != output.ErrSystemLocked {
		t.Fatalf("err = %v, want %s", msg.err, output.ErrSystemLocked)
	}
	if _, err := os.Lstat(item.Path); err != nil {
		t.Errorf("loadout removed while the lock was held: %v", err)
	}
	if mustLoadTUIInstallRecordStore(t, configDir).Find(coord) == nil {
		t.Error("record forgotten while the lock was held")
	}
}
