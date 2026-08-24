package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
)

func TestRecordTUIInstallBookkeepingCarriesSourceSHA(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)

	libraryPath := filepath.Join(t.TempDir(), "skills", "writer")
	writeTUITestFile(t, filepath.Join(libraryPath, "SKILL.md"), []byte("# Writer\n"))

	item := catalog.ContentItem{
		Name: "writer",
		Type: catalog.Skills,
		Path: libraryPath,
		Meta: &metadata.Meta{
			SourceType:     "registry",
			SourceRegistry: "acme/tools",
			SourceSHA:      "sha-from-meta",
		},
	}
	placement := installer.Placement{
		Mechanism: installer.MechanismSymlink,
		Path:      filepath.Join(t.TempDir(), "writer"),
	}

	recordTUIInstallBookkeeping(item, "claude-code", placement)

	store := mustLoadTUIInstallRecordStore(t, configDir)
	coord := installstore.Coord{Registry: "acme/tools", Type: string(catalog.Skills), Name: "writer"}
	rec := store.Find(coord)
	if rec == nil {
		t.Fatal("install record missing")
	}
	if rec.SourceSHA != "sha-from-meta" {
		t.Fatalf("SourceSHA = %q, want sha-from-meta", rec.SourceSHA)
	}
}

func TestRecordTUIAddUpdateBookkeepingRotatesExistingRecord(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	storePath := filepath.Join(configDir, "installs.json")
	libraryPath := filepath.Join(t.TempDir(), "skills", "writer")
	writeTUITestFile(t, filepath.Join(libraryPath, "SKILL.md"), []byte("# Writer\n"))

	coord := installstore.Coord{Registry: "acme/tools", Type: string(catalog.Skills), Name: "writer"}
	if err := installstore.RecordInstallMeta(storePath, coord, libraryPath, installstore.PlacementInput{
		Provider:  "claude-code",
		Mechanism: installstore.MechanismSymlink,
		Path:      filepath.Join(t.TempDir(), "writer"),
	}, installstore.InstallMeta{SourceSHA: "sha-old"}, time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("seed RecordInstallMeta: %v", err)
	}
	oldHash := mustHashTUIInstallContent(t, libraryPath)
	writeTUITestFile(t, filepath.Join(libraryPath, "SKILL.md"), []byte("# Writer updated\n"))

	recordTUIAddUpdateBookkeeping("acme/tools", string(catalog.Skills), "writer", "", "sha-new")

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
	if rec.Previous.CopyPath != "" {
		t.Fatalf("Previous.CopyPath = %q, want empty", rec.Previous.CopyPath)
	}
}

func TestRecordTUIAddUpdateBookkeepingMissingRecordDoesNotCreateStore(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	storePath := filepath.Join(configDir, "installs.json")

	recordTUIAddUpdateBookkeeping("acme/tools", string(catalog.Skills), "writer", "", "sha-new")

	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Fatalf("store file exists or stat failed: %v", err)
	}
}

func TestRecordTUIMOATUpdateBookkeepingSetsPreviousCopyPath(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	storePath := filepath.Join(configDir, "installs.json")
	libraryPath := filepath.Join(t.TempDir(), "skills", "writer")
	writeTUITestFile(t, filepath.Join(libraryPath, "SKILL.md"), []byte("# Writer\n"))

	coord := installstore.Coord{Registry: "acme/moat", Type: string(catalog.Skills), Name: "writer"}
	if err := installstore.RecordInstallMeta(storePath, coord, libraryPath, installstore.PlacementInput{
		Provider:  "claude-code",
		Mechanism: installstore.MechanismSymlink,
		Path:      filepath.Join(t.TempDir(), "writer"),
	}, installstore.InstallMeta{}, time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("seed RecordInstallMeta: %v", err)
	}
	writeTUITestFile(t, filepath.Join(libraryPath, "SKILL.md"), []byte("# Writer updated\n"))
	prevCopyPath := filepath.Join(t.TempDir(), "previous", "writer")

	recordTUIMOATUpdateBookkeeping(catalog.ContentItem{
		Name:     "writer",
		Type:     catalog.Skills,
		Path:     libraryPath,
		Registry: "acme/moat",
	}, prevCopyPath)

	rec := mustLoadTUIInstallRecordStore(t, configDir).Find(coord)
	if rec == nil {
		t.Fatal("install record missing")
	}
	if rec.Previous == nil {
		t.Fatal("Previous is nil")
	}
	if rec.Previous.CopyPath != prevCopyPath {
		t.Fatalf("Previous.CopyPath = %q, want %q", rec.Previous.CopyPath, prevCopyPath)
	}
}

func TestAddSingleItemOverwriteRotatesInstallRecord(t *testing.T) {
	configDir := withTUIInstallRecordConfigDir(t)
	storePath := filepath.Join(configDir, "installs.json")
	contentRoot := t.TempDir()
	libraryPath := filepath.Join(contentRoot, string(catalog.Skills), "writer")
	writeTUITestFile(t, filepath.Join(libraryPath, "SKILL.md"), []byte("# Writer\n"))

	coord := installstore.Coord{Registry: "acme/tools", Type: string(catalog.Skills), Name: "writer"}
	if err := installstore.RecordInstallMeta(storePath, coord, libraryPath, installstore.PlacementInput{
		Provider:  "claude-code",
		Mechanism: installstore.MechanismSymlink,
		Path:      filepath.Join(t.TempDir(), "writer"),
	}, installstore.InstallMeta{SourceSHA: "sha-old"}, time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("seed RecordInstallMeta: %v", err)
	}
	oldHash := mustHashTUIInstallContent(t, libraryPath)

	sourcePath := filepath.Join(t.TempDir(), "SKILL.md")
	writeTUITestFile(t, sourcePath, []byte("# Writer updated\n"))
	item := addDiscoveryItem{
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

	result := addSingleItem(item, contentRoot, "acme/tools", "private", "", "sha-new")
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

func TestTUIInstallRecordCoordRegistryFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		item         catalog.ContentItem
		wantRegistry string
	}{
		{
			name: "metadata registry used when item registry empty",
			item: catalog.ContentItem{
				Name: "writer",
				Type: catalog.Skills,
				Meta: &metadata.Meta{SourceType: "registry", SourceRegistry: "acme/tools"},
			},
			wantRegistry: "acme/tools",
		},
		{
			name: "non registry source type leaves registry empty",
			item: catalog.ContentItem{
				Name: "writer",
				Type: catalog.Skills,
				Meta: &metadata.Meta{SourceType: "provider", SourceRegistry: "acme/tools"},
			},
		},
		{
			name: "explicit item registry wins",
			item: catalog.ContentItem{
				Name:     "writer",
				Type:     catalog.Skills,
				Registry: "explicit",
				Meta:     &metadata.Meta{SourceType: "registry", SourceRegistry: "acme/tools"},
			},
			wantRegistry: "explicit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tuiInstallRecordCoord(tt.item)
			if got.Registry != tt.wantRegistry {
				t.Fatalf("Registry = %q, want %q", got.Registry, tt.wantRegistry)
			}
			if got.Type != string(tt.item.Type) || got.Name != tt.item.Name {
				t.Fatalf("Coord = %#v, want type/name from item", got)
			}
		})
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
