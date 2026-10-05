package snapshot

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// outsideHome returns a directory outside the home directory, skipping the
// test when the temp directory sits under it.
func outsideHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("getting home dir: %v", err)
	}
	if rel, err := filepath.Rel(home, dir); err == nil && filepath.IsLocal(rel) {
		t.Skip("temp dir is under the home directory")
	}
	return dir
}

// TestCreate_FileOutsideHomeRestoresInPlace: a project outside the home
// directory, such as one under /mnt/c on WSL, backs up its own config. The
// backup stays inside the snapshot, so the snapshot loads, restores the
// file where it was, and leaves nothing behind once deleted.
func TestCreate_FileOutsideHomeRestoresInPlace(t *testing.T) {
	t.Parallel()
	projectRoot := outsideHome(t)
	cfgPath := filepath.Join(projectRoot, ".cursor", "mcp.json")
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	const original = `{"mcpServers":{}}`
	os.WriteFile(cfgPath, []byte(original), 0644)

	snapshotDir, err := Create(projectRoot, "dev", "keep", []string{cfgPath}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	os.WriteFile(cfgPath, []byte(`{"mcpServers":{"srv":{}}}`), 0644)

	manifest, loadedDir, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loadedDir != snapshotDir {
		t.Fatalf("Load: got %s, want %s", loadedDir, snapshotDir)
	}
	if err := Restore(snapshotDir, manifest); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got, _ := os.ReadFile(cfgPath); string(got) != original {
		t.Errorf("config: got %s, want %s", got, original)
	}

	if err := Delete(snapshotDir); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	left, _ := os.ReadDir(snapshotsDir(projectRoot))
	if len(left) != 0 {
		t.Errorf("snapshots dir after Delete holds %d entries, want none", len(left))
	}
}

// TestDestination_FallsBackToHome: a manifest written before destinations
// were recorded backs up only files under the home directory.
func TestDestination_FallsBackToHome(t *testing.T) {
	t.Parallel()
	home := filepath.Join(string(filepath.Separator), "home", "u")
	m := &SnapshotManifest{Destinations: map[string]string{"outside-home/0/mcp.json": "/proj/.cursor/mcp.json"}}
	if got := m.Destination(home, "outside-home/0/mcp.json"); got != "/proj/.cursor/mcp.json" {
		t.Errorf("recorded: got %s", got)
	}
	if got, want := m.Destination(home, ".claude/settings.json"), filepath.Join(home, ".claude", "settings.json"); got != want {
		t.Errorf("unrecorded: got %s, want %s", got, want)
	}
}

// TestLoad_SkipsADirectoryWithoutAManifest: earlier versions left backups
// of files outside the home directory beside the snapshots, under a name
// such as "tmp" or "mnt" that sorts after every timestamp. Load reads the
// snapshot rather than that directory, and finds none once only it is left.
func TestLoad_SkipsADirectoryWithoutAManifest(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	stray := filepath.Join(snapshotsDir(projectRoot), "tmp", "proj", ".cursor")
	os.MkdirAll(stray, 0755)
	os.WriteFile(filepath.Join(stray, "mcp.json"), []byte(`{}`), 0644)

	if _, _, err := Load(projectRoot); !errors.Is(err, ErrNoSnapshot) {
		t.Errorf("only the stray dir: got %v, want ErrNoSnapshot", err)
	}

	snapshotDir := filepath.Join(snapshotsDir(projectRoot), "20260101T000000")
	os.MkdirAll(snapshotDir, 0755)
	data, _ := json.Marshal(SnapshotManifest{LoadoutName: "dev", Mode: "keep"})
	os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), data, 0644)

	manifest, loadedDir, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loadedDir != snapshotDir || manifest.LoadoutName != "dev" {
		t.Errorf("Load: got %s (%s), want %s", loadedDir, manifest.LoadoutName, snapshotDir)
	}
}
