package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// UninstallFrom must find a placement wherever Install put it, so an item
// installed under a base directory or a configured path can be removed.
func TestUninstallFrom_FindsInstallLocation(t *testing.T) {
	tests := []struct {
		name     string
		baseDir  func(tmp string) string
		resolver func(tmp string) *config.PathResolver
	}{
		{
			name:    "base dir",
			baseDir: func(tmp string) string { return filepath.Join(tmp, "project") },
		},
		{
			name: "resolver per-type path",
			resolver: func(tmp string) *config.PathResolver {
				return &config.PathResolver{ProviderPaths: map[string]config.ProviderPathConfig{
					"test": {Paths: map[string]string{"rules": filepath.Join(tmp, "custom-rules")}},
				}}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("HOME", tmp)
			repoRoot := filepath.Join(tmp, "repo")
			src := filepath.Join(repoRoot, "rules", "test", "placed")
			if err := os.MkdirAll(src, 0755); err != nil {
				t.Fatal(err)
			}
			item := catalog.ContentItem{Name: "placed", Type: catalog.Rules, Path: src}
			prov := testProvider("test")

			var baseDir string
			var resolver *config.PathResolver
			var pl Placement
			var err error
			if tt.baseDir != nil {
				baseDir = tt.baseDir(tmp)
				pl, err = Install(item, prov, repoRoot, MethodSymlink, baseDir, ScanOptions{})
			} else {
				resolver = tt.resolver(tmp)
				pl, err = InstallWithResolver(item, prov, repoRoot, MethodSymlink, resolver, ScanOptions{})
			}
			if err != nil {
				t.Fatalf("install: %v", err)
			}

			if _, err := Uninstall(item, prov, repoRoot); err == nil {
				t.Fatal("home-default Uninstall succeeded; the placement should be outside the home install dir")
			}
			got, err := UninstallFrom(item, prov, repoRoot, baseDir, resolver)
			if err != nil {
				t.Fatalf("UninstallFrom: %v", err)
			}
			if got.Path != pl.Path {
				t.Errorf("removed %s, want %s", got.Path, pl.Path)
			}
			if _, err := os.Lstat(pl.Path); !os.IsNotExist(err) {
				t.Errorf("%s still exists", pl.Path)
			}
		})
	}
}

// An item whose path names no entry resolves to the install directory
// itself, which UninstallFrom must never delete.
func TestUninstallFrom_RefusesInstallDirItself(t *testing.T) {
	tmp := t.TempDir()
	prov := testProvider("test")
	installDir := prov.InstallDir(tmp, catalog.Rules)
	kept := filepath.Join(installDir, "other-rule")
	if err := os.MkdirAll(kept, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := UninstallFrom(catalog.ContentItem{Name: "", Type: catalog.Rules, Path: ""}, prov, tmp, tmp, nil)

	if err == nil {
		t.Fatal("UninstallFrom succeeded on an item with no path")
	}
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("install dir contents gone: %v", err)
	}
}
