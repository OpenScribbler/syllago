package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// outsideHome points the home directory at a new directory and returns
// another one outside it.
func outsideHome(t *testing.T) (home, outside string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home, t.TempDir()
}

// TestCreate_FileOutsideHomeRestoresInPlace: a project outside the home
// directory, such as one under /mnt/c on WSL, backs up its own config. The
// backup stays inside the snapshot, so the snapshot loads, restores the
// file where it was, and leaves nothing behind once deleted.
func TestCreate_FileOutsideHomeRestoresInPlace(t *testing.T) {
	_, projectRoot := outsideHome(t)
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
// were recorded keys each backup by its path relative to the home directory.
func TestDestination_FallsBackToHome(t *testing.T) {
	t.Parallel()
	// path-literal-ok: Destination only joins paths; nothing is read or written.
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

// TestRestore_RemovesAFileTheApplyCreated: a config that did not exist
// when the snapshot was taken goes away on restore, so a server the apply
// wrote into it does not outlive the loadout. One already gone is fine.
func TestRestore_RemovesAFileTheApplyCreated(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	cfgPath := filepath.Join(projectRoot, ".cursor", "mcp.json")
	gone := filepath.Join(projectRoot, "gone.json")

	snapshotDir, err := Create(projectRoot, "dev", "keep", []string{cfgPath, gone}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	os.WriteFile(cfgPath, []byte(`{"mcpServers":{"srv":{}}}`), 0644)

	manifest, _, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := Restore(snapshotDir, manifest); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := os.Stat(cfgPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("created config still there (stat err %v)", err)
	}
}

// TestCreate_KeysAFileInHomeApartFromOneOutside: a file in the home
// directory whose path matches the key of one outside it keeps its path
// under home as its key, which earlier versions restore correctly, and the
// file outside takes another number, so both restore.
func TestCreate_KeysAFileInHomeApartFromOneOutside(t *testing.T) {
	// Windows and macOS file systems fold case, so a differently cased
	// home path shares the key all the same.
	for _, dir := range []string{"outside-home", "Outside-Home"} {
		t.Run(dir, func(t *testing.T) {
			home, outside := outsideHome(t)
			files := []string{filepath.Join(outside, "settings.json"), filepath.Join(home, dir, "0", "settings.json")}
			for i, f := range files {
				os.MkdirAll(filepath.Dir(f), 0755)
				os.WriteFile(f, []byte(fmt.Sprintf(`{"n":%d}`, i)), 0644)
			}
			snapshotDir, err := Create(outside, "dev", "keep", files, nil, nil)
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			for _, f := range files {
				os.WriteFile(f, []byte(`{"changed":true}`), 0644)
			}

			manifest, _, err := Load(outside)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			keys := manifest.BackedUpFiles
			if len(keys) != 2 || filepath.Join(home, keys[1]) != files[1] || strings.EqualFold(keys[0], keys[1]) {
				t.Errorf("keys: got %q, want the home file under its path in home and the other apart from it", keys)
			}
			if err := Restore(snapshotDir, manifest); err != nil {
				t.Fatalf("Restore: %v", err)
			}
			for i, f := range files {
				if got, want := readString(f), fmt.Sprintf(`{"n":%d}`, i); got != want {
					t.Errorf("%s: got %s, want %s", f, got, want)
				}
			}
		})
	}
}

// TestCreate_KeepsASymlinkToAMissingTarget: a symlink whose target is
// missing reads as missing, but it is the user's, so restore leaves it.
func TestCreate_KeepsASymlinkToAMissingTarget(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	link := filepath.Join(projectRoot, "settings.json")
	if err := os.Symlink(filepath.Join(projectRoot, "missing.json"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	snapshotDir, err := Create(projectRoot, "dev", "keep", []string{link}, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	manifest, _, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := Restore(snapshotDir, manifest); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("symlink gone after restore: %v", err)
	}
}

// TestRestore_RefusesAnUncleanPath: syllago records only clean absolute
// paths, so a relative or unclean one in a manifest fails the restore
// before it changes anything. Each path names victim.json from inside
// dir/sub, the working directory, so a restore that took it would
// overwrite or delete the victim.
func TestRestore_RefusesAnUncleanPath(t *testing.T) {
	tests := []struct {
		name     string
		manifest func(dir string) SnapshotManifest
	}{
		{"relative destination", func(dir string) SnapshotManifest {
			return SnapshotManifest{BackedUpFiles: []string{"home/x"}, Destinations: map[string]string{"home/x": filepath.Join("..", "victim.json")}}
		}},
		{"relative created file", func(dir string) SnapshotManifest {
			return SnapshotManifest{CreatedFiles: []string{filepath.Join("..", "victim.json")}}
		}},
		{"unclean destination", func(dir string) SnapshotManifest {
			return SnapshotManifest{BackedUpFiles: []string{"home/x"}, Destinations: map[string]string{"home/x": dir + "/sub/../victim.json"}}
		}},
		{"unclean created file", func(dir string) SnapshotManifest {
			return SnapshotManifest{CreatedFiles: []string{dir + "/sub/../victim.json"}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err != nil {
				t.Fatal(err)
			}
			t.Chdir(filepath.Join(dir, "sub"))
			victim := filepath.Join(dir, "victim.json")
			if err := os.WriteFile(victim, []byte("theirs"), 0644); err != nil {
				t.Fatal(err)
			}
			snapshotDir := t.TempDir()
			backup := filepath.Join(snapshotDir, "files", "home", "x")
			if err := os.MkdirAll(filepath.Dir(backup), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(backup, []byte("backup"), 0644); err != nil {
				t.Fatal(err)
			}
			m := tt.manifest(dir)
			if err := Restore(snapshotDir, &m); err == nil || !strings.Contains(err.Error(), "not a clean absolute path") {
				t.Errorf("Restore: got %v, want a refusal of the path", err)
			}
			if got, err := os.ReadFile(victim); err != nil || string(got) != "theirs" {
				t.Errorf("victim.json after refusal: %q, %v", got, err)
			}
		})
	}
}

// TestLoad_ReportsASnapshotItCannotRead: a snapshot Load cannot read fails
// the load rather than letting an older snapshot stand in for it.
func TestLoad_ReportsASnapshotItCannotRead(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions enforced")
	}
	projectRoot := t.TempDir()
	for _, name := range []string{"20260101T000000", "20260102T000000"} {
		dir := filepath.Join(snapshotsDir(projectRoot), name)
		os.MkdirAll(dir, 0755)
		data, _ := json.Marshal(SnapshotManifest{LoadoutName: name, Mode: "keep"})
		os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644)
	}
	newest := filepath.Join(snapshotsDir(projectRoot), "20260102T000000")
	os.Chmod(newest, 0)
	t.Cleanup(func() { os.Chmod(newest, 0755) })

	if m, _, err := Load(projectRoot); err == nil || errors.Is(err, ErrNoSnapshot) {
		t.Errorf("Load: got %v (manifest %+v), want a read error", err, m)
	}
}

func readString(path string) string {
	data, _ := os.ReadFile(path)
	return string(data)
}

// TestRestore_AManifestFromBeforeDestinations: earlier versions keyed a file
// outside the home directory as a "../" path, which put its backup beside
// the snapshot rather than in it. Such a snapshot still loads and restores
// the file in place.
func TestRestore_AManifestFromBeforeDestinations(t *testing.T) {
	home, outside := outsideHome(t)
	cfgPath := filepath.Join(outside, "proj", ".cursor", "mcp.json")
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	os.WriteFile(cfgPath, []byte("after"), 0644)
	key, err := filepath.Rel(home, cfgPath)
	if err != nil || !strings.HasPrefix(key, "..") {
		t.Fatalf("Rel: got %q (err %v), want a path out of home", key, err)
	}

	projectRoot := t.TempDir()
	snapDir := filepath.Join(snapshotsDir(projectRoot), "20260101T000000")
	backup := filepath.Join(snapDir, "files", key)
	os.MkdirAll(filepath.Dir(backup), 0755)
	os.WriteFile(backup, []byte("before"), 0644)
	data, _ := json.Marshal(SnapshotManifest{LoadoutName: "dev", Mode: "keep", BackedUpFiles: []string{key}})
	os.WriteFile(filepath.Join(snapDir, "manifest.json"), data, 0644)

	m, dir, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := Restore(dir, m); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := readString(cfgPath); got != "before" {
		t.Errorf("restored %q, want %q", got, "before")
	}
}

// TestLoad_SkipsALeftoverHookSnapshot: earlier versions left a snapshot of
// the settings file after every hook install and uninstall. Load passes
// over them to the loadout's snapshot, and finds none when only they remain.
func TestLoad_SkipsALeftoverHookSnapshot(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	write := func(name, loadout string) {
		dir := filepath.Join(snapshotsDir(projectRoot), name)
		os.MkdirAll(dir, 0755)
		data, _ := json.Marshal(SnapshotManifest{Source: loadout, LoadoutName: loadout, Mode: "keep"})
		os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644)
	}
	write("20260102T000000", "hook-install:guard")
	write("20260103T000000", "hook-uninstall:guard")
	if _, _, err := Load(projectRoot); !errors.Is(err, ErrNoSnapshot) {
		t.Errorf("only hook snapshots: got %v, want ErrNoSnapshot", err)
	}

	write("20260101T000000", "dev")
	m, dir, err := Load(projectRoot)
	if err != nil || m.LoadoutName != "dev" || filepath.Base(dir) != "20260101T000000" {
		t.Errorf("Load: got %+v in %s (err %v), want the dev snapshot", m, dir, err)
	}
}

// TestCreate_KeysAFileInHomeTheWayEarlierVersionsRead: earlier versions
// restore a backup to its key joined to the home directory and ignore
// Destinations, so a file in the home directory keeps its path under it as
// its key, and an older syllago removing this snapshot still puts it back.
func TestCreate_KeysAFileInHomeTheWayEarlierVersionsRead(t *testing.T) {
	home, outside := outsideHome(t)
	path := filepath.Join(home, ".claude", "settings.json")
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte(`{}`), 0644)
	if _, err := Create(outside, "dev", "keep", []string{path}, nil, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}
	manifest, _, err := Load(outside)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(manifest.BackedUpFiles) != 1 || filepath.Join(home, manifest.BackedUpFiles[0]) != path {
		t.Errorf("keys: got %q, want the path under home", manifest.BackedUpFiles)
	}
}
