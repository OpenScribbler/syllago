package lifecycle

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

// libraryDest makes a Library item that came from registry and records it
// as installed. It returns the item's destination and its v1 content hash.
func libraryDest(t *testing.T, storePath, name, registry string, pinned bool) (Destination, string) {
	t.Helper()
	item := libraryItem(t, name, "")
	if err := metadata.Save(item.Path, &metadata.Meta{Name: name, SourceType: "registry", SourceRegistry: registry, SourceSHA: "sha1"}); err != nil {
		t.Fatal(err)
	}
	item.Registry = registry
	if err := installstore.RecordInstallMeta(storePath, recordCoord(item), item.Path, installstore.PlacementInput{
		Provider: "alpha", Mechanism: installstore.MechanismSymlink, Path: "/x/alpha/" + name,
	}, installstore.InstallMeta{SourceSHA: "sha1"}, testNow); err != nil {
		t.Fatal(err)
	}
	if pinned {
		if err := installstore.SetPinned(storePath, recordCoord(item), true, testNow); err != nil {
			t.Fatal(err)
		}
	}
	v1, err := installstore.HashContent(item.Path)
	if err != nil {
		t.Fatal(err)
	}
	return Destination{Type: item.Type, Name: name, Path: item.Path}, v1
}

func destCoord(d Destination, registry string) installstore.Coord {
	return installstore.Coord{Registry: registry, Type: string(d.Type), Name: d.Name}
}

// writeV2 is a Write that replaces every approved destination's content and
// records what it saw.
type writeV2 struct {
	saw  []string
	sha  string
	prev string
	err  error
}

func (w *writeV2) write(approved []Destination) ([]Written, error) {
	var out []Written
	for _, d := range approved {
		w.saw = append(w.saw, d.Path)
		if err := os.WriteFile(filepath.Join(d.Path, "rule.md"), []byte("# v2\n"), 0644); err != nil {
			return out, err
		}
		out = append(out, Written{Path: d.Path, SourceSHA: w.sha, PreviousCopy: w.prev})
	}
	return out, w.err
}

// The pin check reads the record of the item already at the destination,
// so a pin set under one registry holds against content from any other.
func TestOverwrite_PinnedNeedsDecision(t *testing.T) {
	storePath := isolate(t)
	d, v1 := libraryDest(t, storePath, "p1", "acme", true)
	w := &writeV2{}

	_, err := scriptedModule(&scriptedPlacer{}).Overwrite(OverwriteRequest{Destinations: []Destination{d}, Write: w.write})
	var dr *DecisionRequired
	if !errors.As(err, &dr) || dr.Kind != OverwritePinned {
		t.Fatalf("err = %v, want an OverwritePinned decision", err)
	}
	pinned, _ := dr.Context.([]PinnedDestination)
	if len(pinned) != 1 || pinned[0].Path != d.Path || pinned[0].SourceSHA != "sha1" {
		t.Errorf("Context = %+v, want %s pinned at sha1", dr.Context, d.Path)
	}
	if len(w.saw) != 0 {
		t.Errorf("Write saw %v, want no call", w.saw)
	}
	rec := loadRecord(t, storePath, destCoord(d, "acme"))
	if rec == nil || rec.ContentHash != v1 || rec.Previous != nil {
		t.Errorf("record = %+v, want it unchanged at %s", rec, v1)
	}
}

func TestOverwrite_SkippedPinNeverReachesWrite(t *testing.T) {
	storePath := isolate(t)
	pinned, v1 := libraryDest(t, storePath, "p2", "acme", true)
	free, _ := libraryDest(t, storePath, "p3", "acme", false)
	w := &writeV2{}

	out, err := scriptedModule(&scriptedPlacer{}).Overwrite(OverwriteRequest{
		Destinations: []Destination{pinned, free},
		Write:        w.write,
		Decisions:    Decisions{Overwrite: map[string]bool{pinned.Path: false}},
	})
	if err != nil || len(out.Failed) != 0 {
		t.Fatalf("err = %v, Failed = %+v", err, out.Failed)
	}
	if len(w.saw) != 1 || w.saw[0] != free.Path {
		t.Errorf("Write saw %v, want only %s", w.saw, free.Path)
	}
	if rec := loadRecord(t, storePath, destCoord(pinned, "acme")); rec == nil || rec.ContentHash != v1 || rec.Previous != nil {
		t.Errorf("pinned record = %+v, want it unchanged", rec)
	}
	if rec := loadRecord(t, storePath, destCoord(free, "acme")); rec == nil || rec.Previous == nil {
		t.Errorf("free record = %+v, want it rotated", rec)
	}
}

func TestOverwrite_ApprovedPinRotatesAndStaysPinned(t *testing.T) {
	storePath := isolate(t)
	d, v1 := libraryDest(t, storePath, "p4", "acme", true)
	w := &writeV2{sha: "sha2", prev: "/prev/p4"}

	out, err := scriptedModule(&scriptedPlacer{}).Overwrite(OverwriteRequest{
		Destinations: []Destination{d},
		Write:        w.write,
		Decisions:    Decisions{Overwrite: map[string]bool{d.Path: true}},
	})
	if err != nil || len(out.Failed) != 0 {
		t.Fatalf("err = %v, Failed = %+v", err, out.Failed)
	}
	rec := loadRecord(t, storePath, destCoord(d, "acme"))
	if rec == nil || rec.Previous == nil {
		t.Fatalf("record = %+v, want a previous version", rec)
	}
	if !rec.Pinned || rec.SourceSHA != "sha2" || rec.ContentHash == v1 {
		t.Errorf("record = %+v, want it pinned at sha2 with a new hash", rec)
	}
	if p := rec.Previous; p.ContentHash != v1 || p.SourceSHA != "sha1" || p.CopyPath != "/prev/p4" {
		t.Errorf("Previous = %+v, want v1 %s at sha1 saved to /prev/p4", p, v1)
	}
}

func TestOverwrite_UnreadableStoreWritesNothing(t *testing.T) {
	storePath := isolate(t)
	d, _ := libraryDest(t, storePath, "p5", "acme", false)
	if err := os.WriteFile(storePath, []byte("{broken"), 0644); err != nil {
		t.Fatal(err)
	}
	w := &writeV2{}

	if _, err := scriptedModule(&scriptedPlacer{}).Overwrite(OverwriteRequest{Destinations: []Destination{d}, Write: w.write}); err == nil {
		t.Fatal("want the store error")
	}
	if len(w.saw) != 0 {
		t.Errorf("Write saw %v, want no call", w.saw)
	}
}

func TestOverwrite_UnreadableMetadataWritesNothing(t *testing.T) {
	storePath := isolate(t)
	d, _ := libraryDest(t, storePath, "p6", "acme", true)
	if err := os.WriteFile(metadata.MetaPath(d.Path), []byte("source_registry: [\n"), 0644); err != nil {
		t.Fatal(err)
	}
	w := &writeV2{}

	if _, err := scriptedModule(&scriptedPlacer{}).Overwrite(OverwriteRequest{Destinations: []Destination{d}, Write: w.write}); err == nil {
		t.Fatal("want the metadata error")
	}
	if len(w.saw) != 0 {
		t.Errorf("Write saw %v, want no call", w.saw)
	}
}

func TestOverwrite_LockFailureWritesNothing(t *testing.T) {
	storePath := isolate(t)
	d, _ := libraryDest(t, storePath, "p7", "acme", false)
	m := scriptedModule(&scriptedPlacer{})
	m.lock = func() (func(), error) { return nil, errors.New("busy") }
	w := &writeV2{}

	if _, err := m.Overwrite(OverwriteRequest{Destinations: []Destination{d}, Write: w.write}); err == nil {
		t.Fatal("want the lock error")
	}
	if len(w.saw) != 0 {
		t.Errorf("Write saw %v, want no call", w.saw)
	}
}

// A new item, or one never installed, has no record to rotate.
func TestOverwrite_NoRecordNothingToRotate(t *testing.T) {
	storePath := isolate(t)
	item := libraryItem(t, "p8", "")
	d := Destination{Type: catalog.Rules, Name: "p8", Path: item.Path}
	w := &writeV2{prev: "/prev/p8"}

	out, err := scriptedModule(&scriptedPlacer{}).Overwrite(OverwriteRequest{Destinations: []Destination{d}, Write: w.write})
	if err != nil || len(out.Failed) != 0 {
		t.Fatalf("err = %v, Failed = %+v", err, out.Failed)
	}
	if len(w.saw) != 1 {
		t.Errorf("Write saw %v, want %s", w.saw, d.Path)
	}
	if rec := loadRecord(t, storePath, destCoord(d, "")); rec != nil {
		t.Errorf("record = %+v, want none", rec)
	}
}

// A Write that fails partway still records the destinations it replaced.
func TestOverwrite_WriteErrorRecordsWhatWasWritten(t *testing.T) {
	storePath := isolate(t)
	d, v1 := libraryDest(t, storePath, "p9", "acme", false)
	w := &writeV2{err: errors.New("disk full")}

	if _, err := scriptedModule(&scriptedPlacer{}).Overwrite(OverwriteRequest{Destinations: []Destination{d}, Write: w.write}); err == nil {
		t.Fatal("want the write error")
	}
	if rec := loadRecord(t, storePath, destCoord(d, "acme")); rec == nil || rec.Previous == nil || rec.Previous.ContentHash != v1 {
		t.Errorf("record = %+v, want it rotated from %s", rec, v1)
	}
}

// Content new to the Library cannot be pinned, so an unreadable store does
// not block it.
func TestOverwrite_NewContentSkipsTheStore(t *testing.T) {
	storePath := isolate(t)
	if err := os.WriteFile(storePath, []byte("{broken"), 0644); err != nil {
		t.Fatal(err)
	}
	d := Destination{Type: catalog.Rules, Name: "p10", Path: filepath.Join(t.TempDir(), "p10")}
	var saw []Destination

	_, err := scriptedModule(&scriptedPlacer{}).Overwrite(OverwriteRequest{
		Destinations: []Destination{d},
		Write:        func(approved []Destination) ([]Written, error) { saw = approved; return nil, nil },
	})
	if err != nil || len(saw) != 1 {
		t.Errorf("err = %v, Write saw %v; want %s written", err, saw, d.Path)
	}
}
