package loadout

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
)

// TestRemove_RestoresAFileOutsideHome: a loadout applied in a project
// outside the home directory restores the project's config where it was
// and reports that path.
func TestRemove_RestoresAFileOutsideHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	projectRoot := t.TempDir()
	cfgPath := filepath.Join(projectRoot, ".cursor", "mcp.json")
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	const original = `{"mcpServers":{}}`
	os.WriteFile(cfgPath, []byte(original), 0644)
	if _, err := snapshot.Create(projectRoot, "dev", "keep", []string{cfgPath}, nil, nil); err != nil {
		t.Fatalf("snapshot.Create: %v", err)
	}
	os.WriteFile(cfgPath, []byte(`{"mcpServers":{"srv":{}}}`), 0644)

	result, err := Remove(RemoveOptions{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !slices.Equal(result.RestoredFiles, []string{cfgPath}) {
		t.Errorf("RestoredFiles: got %q, want %q", result.RestoredFiles, cfgPath)
	}
	if got, _ := os.ReadFile(cfgPath); string(got) != original {
		t.Errorf("config: got %s, want %s", got, original)
	}
}

// TestRemove_DeletesAFileTheApplyCreated: removing a loadout deletes a
// config its apply created and reports it.
func TestRemove_DeletesAFileTheApplyCreated(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	cfgPath := filepath.Join(projectRoot, ".cursor", "mcp.json")
	if _, err := snapshot.Create(projectRoot, "dev", "keep", []string{cfgPath}, nil, nil); err != nil {
		t.Fatalf("snapshot.Create: %v", err)
	}
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	os.WriteFile(cfgPath, []byte(`{"mcpServers":{"srv":{}}}`), 0644)

	result, err := Remove(RemoveOptions{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if !slices.Equal(result.RemovedFiles, []string{cfgPath}) {
		t.Errorf("RemovedFiles: got %q, want %q", result.RemovedFiles, cfgPath)
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Errorf("created config still there (stat err %v)", err)
	}
}

// TestRemove_KeepsInstallsMadeAfterTheApply: remove takes out the loadout's
// entries and keeps one a later install added, whether or not the project
// had an installed.json before the apply.
func TestRemove_KeepsInstallsMadeAfterTheApply(t *testing.T) {
	t.Parallel()
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "first apply", true: "existing installed.json"}[existing], func(t *testing.T) {
			t.Parallel()
			homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
			want := []string{"later"}
			if existing {
				earlier := &installer.Installed{MCP: []installer.InstalledMCP{{Name: "earlier", Source: "export", Provider: prov.Slug}}}
				if err := installer.SaveInstalled(projectRoot, earlier); err != nil {
					t.Fatalf("SaveInstalled: %v", err)
				}
				want = []string{"earlier", "later"}
			}
			if _, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot}); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			inst, err := installer.LoadInstalled(projectRoot)
			if err != nil || len(inst.Hooks) == 0 {
				t.Fatalf("installed.json after apply: got %+v (err %v), want the loadout's hook", inst, err)
			}
			inst.MCP = append(inst.MCP, installer.InstalledMCP{Name: "later", Source: "export", Provider: prov.Slug})
			if err := installer.SaveInstalled(projectRoot, inst); err != nil {
				t.Fatalf("SaveInstalled: %v", err)
			}

			result, err := Remove(RemoveOptions{ProjectRoot: projectRoot})
			if err != nil {
				t.Fatalf("Remove: %v", err)
			}
			installedPath := filepath.Join(projectRoot, ".syllago", "installed.json")
			if slices.Contains(result.RemovedFiles, installedPath) || slices.Contains(result.RestoredFiles, installedPath) {
				t.Errorf("remove reverted installed.json: restored %q, removed %q", result.RestoredFiles, result.RemovedFiles)
			}
			inst, err = installer.LoadInstalled(projectRoot)
			var got []string
			for _, m := range inst.MCP {
				got = append(got, m.Name)
			}
			if err != nil || len(inst.Hooks) != 0 || !slices.Equal(got, want) {
				t.Errorf("installed.json after remove: hooks %d, MCP %q (err %v), want no hooks and MCP %q", len(inst.Hooks), got, err, want)
			}
		})
	}
}

// TestRemove_ForgetsAnInstallTheRevertTookOut: a hook installed after the
// apply into the settings file the apply created goes when remove deletes
// that file, and so does its record, which would otherwise block a later
// apply or install of the same hook.
func TestRemove_ForgetsAnInstallTheRevertTookOut(t *testing.T) {
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	if _, err := os.Stat(settingsPath); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("settings.json before apply: stat err %v, want none", err)
	}
	if _, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	laterDir := filepath.Join(projectRoot, "content", "hooks", "claude-code", "later-hook")
	os.MkdirAll(laterDir, 0755)
	os.WriteFile(filepath.Join(laterDir, "hook.json"), []byte(`{"spec":"hooks/0.1","hooks":[{"event":"PostToolUse","matcher":".*","handler":{"type":"command","command":"echo later"}}]}`), 0644)
	later := catalog.ContentItem{Name: "later-hook", Type: catalog.Hooks, Provider: "claude-code", Path: laterDir}
	if _, err := installer.Install(later, prov, projectRoot, installer.MethodSymlink, "", installer.ScanOptions{}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	if _, err := Remove(RemoveOptions{ProjectRoot: projectRoot}); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(settingsPath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("settings.json after remove: stat err %v, want deleted", err)
	}
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil || len(inst.Hooks) != 0 {
		t.Errorf("installed.json after remove: got %+v (err %v), want no hooks", inst, err)
	}
}

// TestRemove_KeepsTheSnapshotWhenInstalledJSONCannotBeSaved: with no backup
// of installed.json, saving it is the only way the loadout's records go, so
// a failed save fails the remove and keeps the snapshot for another try.
func TestRemove_KeepsTheSnapshotWhenInstalledJSONCannotBeSaved(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a directory the test cannot write to")
	}
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
	if _, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", Method: installer.MethodCopy, ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	before, _, err := snapshot.Load(projectRoot)
	if err != nil || len(before.Symlinks) == 0 {
		t.Fatalf("snapshot after apply: %+v, %v; want a placed copy", before, err)
	}
	syllagoDir := filepath.Join(projectRoot, ".syllago")
	os.Chmod(syllagoDir, 0555)
	t.Cleanup(func() { os.Chmod(syllagoDir, 0755) })

	if _, err := Remove(RemoveOptions{ProjectRoot: projectRoot}); err == nil {
		t.Fatal("Remove succeeded with installed.json unwritable")
	}
	after, _, err := snapshot.Load(projectRoot)
	if err != nil {
		t.Fatalf("snapshot after the failed remove: %v", err)
	}
	// The copies are deleted, so they leave the snapshot, and the retry
	// cannot delete what someone puts at their paths in between.
	if len(after.Symlinks) != 0 {
		t.Errorf("Symlinks after the failed remove: got %+v, want none", after.Symlinks)
	}
	theirs := filepath.Join(before.Symlinks[0].Path, "theirs.md")
	if err := os.MkdirAll(filepath.Dir(theirs), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(theirs, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	os.Chmod(syllagoDir, 0755)
	if _, err := Remove(RemoveOptions{ProjectRoot: projectRoot}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if _, err := os.Stat(theirs); err != nil {
		t.Errorf("retry deleted what appeared at a removed path: %v", err)
	}
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil || len(inst.Hooks) != 0 || len(inst.Symlinks) != 0 {
		t.Errorf("installed.json after retry: got %+v (err %v), want no loadout records", inst, err)
	}
}

// TestRemove_LeavesInstalledJSONAnEarlierSnapshotBackedUp: snapshots from
// earlier versions list installed.json among their backups. Remove cleans
// it rather than restoring it, so a record added after the apply stays.
func TestRemove_LeavesInstalledJSONAnEarlierSnapshotBackedUp(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	if err := installer.SaveInstalled(projectRoot, &installer.Installed{}); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}
	installedPath := filepath.Join(projectRoot, ".syllago", "installed.json")
	if _, err := snapshot.Create(projectRoot, "dev", "keep", []string{installedPath}, nil, nil); err != nil {
		t.Fatalf("snapshot.Create: %v", err)
	}
	if err := installer.SaveInstalled(projectRoot, &installer.Installed{MCP: []installer.InstalledMCP{
		{Name: "srv", Source: "loadout:dev", Provider: "cursor"},
		{Name: "later", Source: "export", Provider: "cursor"},
	}}); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}

	result, err := Remove(RemoveOptions{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if slices.Contains(result.RestoredFiles, installedPath) {
		t.Errorf("RestoredFiles lists installed.json: %q", result.RestoredFiles)
	}
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil || len(inst.MCP) != 1 || inst.MCP[0].Name != "later" {
		t.Errorf("installed.json after remove: got %+v (err %v), want only the later install", inst, err)
	}
}

// TestRemove_SkipsInstalledJSONInAManifestFromBeforeDestinations: an earlier
// version's manifest keys installed.json by its path under home and records
// no destination. Remove skips it even when the project is reached through
// a symlink, so a record added after the apply stays.
func TestRemove_SkipsInstalledJSONInAManifestFromBeforeDestinations(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	projectRoot := filepath.Join(home, "proj")
	alias := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(projectRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(projectRoot, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	snapshotDir := filepath.Join(projectRoot, ".syllago", "snapshots", "20260101T000000")
	key := filepath.Join("proj", ".syllago", "installed.json")
	os.MkdirAll(filepath.Dir(filepath.Join(snapshotDir, "files", key)), 0755)
	os.WriteFile(filepath.Join(snapshotDir, "files", key), []byte(`{}`), 0644)
	legacy := fmt.Sprintf(`{"loadoutName":"dev","mode":"keep","backedUpFiles":[%q]}`, filepath.ToSlash(key))
	os.WriteFile(filepath.Join(snapshotDir, "manifest.json"), []byte(legacy), 0644)
	if err := installer.SaveInstalled(projectRoot, &installer.Installed{MCP: []installer.InstalledMCP{
		{Name: "srv", Source: "loadout:dev", Provider: "cursor"},
		{Name: "later", Source: "export", Provider: "cursor"},
	}}); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}

	result, err := Remove(RemoveOptions{ProjectRoot: alias})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(result.RestoredFiles) != 0 {
		t.Errorf("RestoredFiles: got %q, want none", result.RestoredFiles)
	}
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil || len(inst.MCP) != 1 || inst.MCP[0].Name != "later" {
		t.Errorf("installed.json after remove: got %+v (err %v), want only the later install", inst, err)
	}
}
