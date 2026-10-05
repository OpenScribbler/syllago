package loadout

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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

// TestRemove_KeepsInstallsMadeAfterTheApply: on a project's first apply
// there is no installed.json to restore, so remove takes out the loadout's
// entries and keeps one a later install added.
func TestRemove_KeepsInstallsMadeAfterTheApply(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
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
	if slices.Contains(result.RemovedFiles, filepath.Join(projectRoot, ".syllago", "installed.json")) {
		t.Errorf("RemovedFiles lists installed.json: %q", result.RemovedFiles)
	}
	inst, err = installer.LoadInstalled(projectRoot)
	if err != nil || len(inst.Hooks) != 0 || len(inst.MCP) != 1 || inst.MCP[0].Name != "later" {
		t.Errorf("installed.json after remove: got %+v (err %v), want only the later install", inst, err)
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
