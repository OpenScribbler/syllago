package tui

// Acceptance tests: the Add wizard's split-rule import respects pins and
// rotates the install record of each section it replaces. These tests set
// package globals, so they do not run in parallel.

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
)

var accSplitSections = []string{"heldrule", "freshrule", "otherrule"}

// accSplitSource is a CLAUDE.md that the H2 splitter cuts into the three
// accSplitSections.
func accSplitSource(version string) string {
	var b strings.Builder
	b.WriteString("# Project rules\n\n")
	for _, name := range accSplitSections {
		b.WriteString("## " + name + "\n\n")
		for i := 0; i < 9; i++ {
			b.WriteString("- " + name + " guidance " + version + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func accSplitItem(path string) addDiscoveryItem {
	return addDiscoveryItem{
		name:        "CLAUDE.md",
		itemType:    catalog.Rules,
		path:        path,
		splittable:  true,
		splitChosen: true,
		underlying:  &add.DiscoveryItem{Name: "CLAUDE.md", Type: catalog.Rules, Path: path},
	}
}

// accSeedSplit imports the three sections at v1, records an install for
// each at sha-old (pinning those in pinned), and points the source at v2.
func accSeedSplit(t *testing.T, pinned map[string]bool) (configDir, contentRoot, source string, hashes map[string]string) {
	t.Helper()
	configDir = withTUIInstallRecordConfigDir(t)
	storePath := filepath.Join(configDir, "installs.json")
	contentRoot = t.TempDir()
	source = filepath.Join(t.TempDir(), "CLAUDE.md")
	writeTUITestFile(t, source, []byte(accSplitSource("v1")))
	if r := addSingleItem(accSplitItem(source), contentRoot, "", "", "", "claude-code", ""); r.err != nil {
		t.Fatalf("seed split import: status %q err %v", r.status, r.err)
	}
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	hashes = map[string]string{}
	for _, name := range accSplitSections {
		path := accSplitPath(contentRoot, name)
		coord := installstore.Coord{Type: string(catalog.Rules), Name: name}
		if err := installstore.RecordInstallMeta(storePath, coord, path, installstore.PlacementInput{
			Provider: "claude-code", Mechanism: installstore.MechanismSymlink, Path: filepath.Join(t.TempDir(), name),
		}, installstore.InstallMeta{SourceSHA: "sha-old"}, now); err != nil {
			t.Fatalf("RecordInstallMeta %s: %v", name, err)
		}
		if pinned[name] {
			if err := installstore.SetPinned(storePath, coord, true, now); err != nil {
				t.Fatalf("SetPinned %s: %v", name, err)
			}
		}
		hashes[name] = mustHashTUIInstallContent(t, path)
	}
	writeTUITestFile(t, source, []byte(accSplitSource("v2")))
	return configDir, contentRoot, source, hashes
}

func accSplitPath(contentRoot, name string) string {
	return filepath.Join(contentRoot, string(catalog.Rules), "claude-code", name)
}

func accSplitSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		files[rel] = string(data)
		return nil
	})
	if err != nil || len(files) == 0 {
		t.Fatalf("snapshot %s: %d files, err %v", dir, len(files), err)
	}
	return files
}

func accSplitAssertHeld(t *testing.T, configDir, contentRoot, name string, before map[string]string, oldHash string) {
	t.Helper()
	after := accSplitSnapshot(t, accSplitPath(contentRoot, name))
	if len(after) != len(before) {
		t.Errorf("pinned %s: %d files, want %d unchanged", name, len(after), len(before))
	}
	for rel, data := range before {
		if after[rel] != data {
			t.Errorf("pinned %s: %s changed", name, rel)
		}
	}
	rec := mustLoadTUIInstallRecordStore(t, configDir).Find(installstore.Coord{Type: string(catalog.Rules), Name: name})
	if rec == nil || !rec.Pinned || rec.Previous != nil || rec.ContentHash != oldHash || rec.SourceSHA != "sha-old" {
		t.Errorf("pinned %s: record = %+v, want it pinned and unchanged", name, rec)
	}
}

func accSplitAssertRotated(t *testing.T, configDir, contentRoot, name, oldHash string) {
	t.Helper()
	if got := mustHashTUIInstallContent(t, accSplitPath(contentRoot, name)); got == oldHash {
		t.Fatalf("%s: Library content unchanged, want it replaced", name)
	}
	rec := mustLoadTUIInstallRecordStore(t, configDir).Find(installstore.Coord{Type: string(catalog.Rules), Name: name})
	if rec == nil {
		t.Fatalf("%s: install record gone", name)
	}
	if rec.Previous == nil {
		t.Fatalf("%s: record has no Previous; the replaced version was not kept", name)
	}
	if rec.Previous.ContentHash != oldHash || rec.Previous.SourceSHA != "sha-old" {
		t.Errorf("%s: Previous = %+v, want content hash %s at sha-old", name, rec.Previous, oldHash)
	}
	if rec.ContentHash == oldHash {
		t.Errorf("%s: record content hash still %s, want it moved to the new content", name, oldHash)
	}
}

func TestAcceptance_SplitImport_AllSectionsPinned(t *testing.T) {
	pinned := map[string]bool{"heldrule": true, "freshrule": true, "otherrule": true}
	configDir, contentRoot, source, hashes := accSeedSplit(t, pinned)
	before := map[string]map[string]string{}
	for _, name := range accSplitSections {
		before[name] = accSplitSnapshot(t, accSplitPath(contentRoot, name))
	}

	result := addSingleItem(accSplitItem(source), contentRoot, "", "", "", "claude-code", "")
	if result.status != "pinned" {
		t.Errorf("status = %q err=%v, want pinned", result.status, result.err)
	}
	for _, name := range accSplitSections {
		accSplitAssertHeld(t, configDir, contentRoot, name, before[name], hashes[name])
	}
}

func TestAcceptance_SplitImport_SomeSectionsPinned(t *testing.T) {
	configDir, contentRoot, source, hashes := accSeedSplit(t, map[string]bool{"heldrule": true})
	before := accSplitSnapshot(t, accSplitPath(contentRoot, "heldrule"))

	addSingleItem(accSplitItem(source), contentRoot, "", "", "", "claude-code", "")
	accSplitAssertHeld(t, configDir, contentRoot, "heldrule", before, hashes["heldrule"])
	accSplitAssertRotated(t, configDir, contentRoot, "freshrule", hashes["freshrule"])
	accSplitAssertRotated(t, configDir, contentRoot, "otherrule", hashes["otherrule"])
}

func TestAcceptance_SplitImport_UnpinnedReplaceRotates(t *testing.T) {
	configDir, contentRoot, source, hashes := accSeedSplit(t, nil)

	result := addSingleItem(accSplitItem(source), contentRoot, "", "", "", "claude-code", "")
	if result.err != nil {
		t.Fatalf("split import over unpinned sections: status %q err %v", result.status, result.err)
	}
	for _, name := range accSplitSections {
		accSplitAssertRotated(t, configDir, contentRoot, name, hashes[name])
	}
}
