package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installstore"
	"github.com/OpenScribbler/syllago/cli/internal/metadata"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func TestInstallCommandRecordsInstallState(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	configDir := withInstallRecordConfigDir(t)
	withFakeRepoRoot(t, t.TempDir())
	output.SetForTest(t)

	installBase := t.TempDir()
	addTestProviderOpts(t, "record-install-provider", "Record Install Provider", installBase, true)
	resetInstallRecordInstallFlags(t)
	installCmd.Flags().Set("to", "record-install-provider")

	if err := installCmd.RunE(installCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("install command failed: %v", err)
	}

	store := mustLoadInstallRecordStore(t, configDir)
	coord := installstore.Coord{Registry: "", Type: string(catalog.Skills), Name: "my-skill"}
	rec := store.Find(coord)
	if rec == nil {
		t.Fatal("install record missing")
	}
	wantLibraryPath := filepath.Join(globalDir, "skills", "my-skill")
	if rec.LibraryPath != wantLibraryPath {
		t.Fatalf("LibraryPath = %s, want %s", rec.LibraryPath, wantLibraryPath)
	}
	if rec.ContentHash == "" {
		t.Fatal("ContentHash is empty")
	}
	if len(rec.Placements) != 1 {
		t.Fatalf("Placements length = %d, want 1", len(rec.Placements))
	}
	got := rec.Placements[0]
	wantPath := filepath.Join(installBase, "skills", "my-skill")
	if got.Provider != "record-install-provider" || got.Mechanism != installstore.MechanismSymlink || got.Path != wantPath || got.Key != "" {
		t.Fatalf("placement = %#v, want provider record-install-provider symlink path %s", got, wantPath)
	}
}

func TestUninstallCommandRemovesInstallStatePlacement(t *testing.T) {
	globalDir := setupGlobalLibrary(t)
	withGlobalLibrary(t, globalDir)
	configDir := withInstallRecordConfigDir(t)
	withFakeRepoRoot(t, t.TempDir())
	output.SetForTest(t)

	installBase := t.TempDir()
	addTestProviderOpts(t, "record-uninstall-provider", "Record Uninstall Provider", installBase, true)
	targetPath := filepath.Join(installBase, "skills", "my-skill")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll target dir: %v", err)
	}
	libraryPath := filepath.Join(globalDir, "skills", "my-skill")
	if err := os.Symlink(libraryPath, targetPath); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	coord := installstore.Coord{Registry: "", Type: string(catalog.Skills), Name: "my-skill"}
	storePath := filepath.Join(configDir, "installs.json")
	if err := installstore.RecordInstall(storePath, coord, libraryPath, installstore.PlacementInput{
		Provider:  "record-uninstall-provider",
		Mechanism: installstore.MechanismSymlink,
		Path:      targetPath,
	}, time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("seed RecordInstall: %v", err)
	}

	resetInstallRecordUninstallFlags(t)
	uninstallCmd.Flags().Set("from", "record-uninstall-provider")
	uninstallCmd.Flags().Set("force", "true")

	if err := uninstallCmd.RunE(uninstallCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("uninstall command failed: %v", err)
	}

	store := mustLoadInstallRecordStore(t, configDir)
	if rec := store.Find(coord); rec != nil {
		t.Fatalf("record after uninstall = %#v, want pruned record", rec)
	}
}

func TestInstallRecordCoordRegistryFallback(t *testing.T) {
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
			got := installRecordCoord(tt.item)
			if got.Registry != tt.wantRegistry {
				t.Fatalf("Registry = %q, want %q", got.Registry, tt.wantRegistry)
			}
			if got.Type != string(tt.item.Type) || got.Name != tt.item.Name {
				t.Fatalf("Coord = %#v, want type/name from item", got)
			}
		})
	}
}

func withInstallRecordConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig := config.GlobalDirOverride
	config.GlobalDirOverride = dir
	t.Cleanup(func() { config.GlobalDirOverride = orig })
	return dir
}

func resetInstallRecordInstallFlags(t *testing.T) {
	t.Helper()
	installCmd.Flags().Set("to", "")
	installCmd.Flags().Set("to-all", "false")
	installCmd.Flags().Set("type", "")
	installCmd.Flags().Set("method", "symlink")
	installCmd.Flags().Set("all", "false")
	installCmd.Flags().Set("dry-run", "false")
	installCmd.Flags().Set("base-dir", "")
	installCmd.Flags().Set("no-input", "false")
	installCmd.Flags().Set("hook-scanner", "")
	installCmd.Flags().Set("force", "false")
	installCmd.Flags().Set("on-clean", "")
	installCmd.Flags().Set("on-modified", "")
	t.Cleanup(func() {
		installCmd.Flags().Set("to", "")
		installCmd.Flags().Set("to-all", "false")
		installCmd.Flags().Set("type", "")
		installCmd.Flags().Set("method", "symlink")
		installCmd.Flags().Set("all", "false")
		installCmd.Flags().Set("dry-run", "false")
		installCmd.Flags().Set("base-dir", "")
		installCmd.Flags().Set("no-input", "false")
		installCmd.Flags().Set("hook-scanner", "")
		installCmd.Flags().Set("force", "false")
		installCmd.Flags().Set("on-clean", "")
		installCmd.Flags().Set("on-modified", "")
	})
}

func resetInstallRecordUninstallFlags(t *testing.T) {
	t.Helper()
	uninstallCmd.Flags().Set("from", "")
	uninstallCmd.Flags().Set("force", "false")
	uninstallCmd.Flags().Set("dry-run", "false")
	uninstallCmd.Flags().Set("no-input", "false")
	uninstallCmd.Flags().Set("type", "")
	t.Cleanup(func() {
		uninstallCmd.Flags().Set("from", "")
		uninstallCmd.Flags().Set("force", "false")
		uninstallCmd.Flags().Set("dry-run", "false")
		uninstallCmd.Flags().Set("no-input", "false")
		uninstallCmd.Flags().Set("type", "")
	})
}

func mustLoadInstallRecordStore(t *testing.T, configDir string) *installstore.Store {
	t.Helper()
	store, err := installstore.Load(filepath.Join(configDir, "installs.json"))
	if err != nil {
		t.Fatalf("Load install store: %v", err)
	}
	return store
}
