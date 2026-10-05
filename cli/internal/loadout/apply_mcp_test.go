package loadout

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
)

// setupMCPEnv builds a Cursor loadout holding one MCP item whose
// config.json is config, with Cursor's MCP config in the project holding
// existing.
func setupMCPEnv(t *testing.T, config, existing string) (projectRoot, cfgPath string, manifest *Manifest, cat *catalog.Catalog) {
	t.Helper()
	projectRoot = t.TempDir()
	itemDir := filepath.Join(projectRoot, "content", "mcp", "srv")
	os.MkdirAll(itemDir, 0755)
	os.WriteFile(filepath.Join(itemDir, "config.json"), []byte(config), 0644)
	cfgPath = filepath.Join(projectRoot, ".cursor", "mcp.json")
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	os.WriteFile(cfgPath, []byte(existing), 0644)

	manifest = &Manifest{Kind: "loadout", Version: 1, Provider: "cursor", Name: "mcp-loadout", MCP: []ItemRef{{Name: "srv"}}}
	cat = &catalog.Catalog{
		RepoRoot: projectRoot,
		Items:    []catalog.ContentItem{{Name: "srv", Type: catalog.MCP, Path: itemDir, ServerKey: "srv"}},
	}
	return projectRoot, cfgPath, manifest, cat
}

// TestApply_MCPReportsWhatTheMergeLeavesOut: a loadout's MCP server goes
// through install's merge, so the fields that merge does not write reach
// the apply's warnings, and the record carries the loadout.
func TestApply_MCPReportsWhatTheMergeLeavesOut(t *testing.T) {
	t.Parallel()
	projectRoot, cfgPath, manifest, cat := setupMCPEnv(t, `{"command":"node","cwd":"/srv"}`, `{"mcpServers":{"mine":{"command":"x"}}}`)

	result, err := Apply(manifest, cat, provider.Cursor, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: t.TempDir(), RepoRoot: projectRoot})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := `srv: server "srv": cwd not installed (syllago writes only type, command, args, url, env)`
	if !slices.Contains(result.Warnings, want) {
		t.Errorf("warnings: got %q, want one %q", result.Warnings, want)
	}
	got, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(got), `"srv":{"command":"node"}`) || !strings.Contains(string(got), `"mine"`) {
		t.Errorf("config: got %s, want srv merged beside mine", got)
	}
	if _, err := os.Stat(cfgPath + ".bak"); !os.IsNotExist(err) {
		t.Errorf("apply wrote a backup beside the config (stat err %v)", err)
	}
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil || len(inst.MCP) != 1 || inst.MCP[0].Source != "loadout:mcp-loadout" || inst.MCP[0].ServerKey != "srv" {
		t.Errorf("installed.json: got %+v (err %v), want one srv record from loadout:mcp-loadout", inst, err)
	}
}

// TestApply_MCPPlacesOnlyItsOwnServer: the scanner makes an item of each
// server in a shared config.json, so a loadout naming one server installs
// that server alone.
func TestApply_MCPPlacesOnlyItsOwnServer(t *testing.T) {
	t.Parallel()
	projectRoot, cfgPath, manifest, cat := setupMCPEnv(t, `{"mcpServers":{"srv":{"command":"node"},"other":{"command":"x"}}}`, `{}`)

	if _, err := Apply(manifest, cat, provider.Cursor, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: t.TempDir(), RepoRoot: projectRoot}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(got), `"srv"`) || strings.Contains(string(got), `"other"`) {
		t.Errorf("config: got %s, want srv alone", got)
	}
}

// TestApply_MCPRefusesAServerTheUserDefined: a server of the same name the
// user defined stays. Preview predicts the refusal, so apply stops before it
// takes a snapshot or writes anything.
func TestApply_MCPRefusesAServerTheUserDefined(t *testing.T) {
	t.Parallel()
	const mine = `{"mcpServers":{"srv":{"command":"my-srv"}}}`
	projectRoot, cfgPath, manifest, cat := setupMCPEnv(t, `{"command":"node"}`, mine)
	opts := ApplyOptions{Mode: "preview", ProjectRoot: projectRoot, HomeDir: t.TempDir(), RepoRoot: projectRoot}

	preview, err := Apply(manifest, cat, provider.Cursor, opts)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(preview.Actions) != 1 || preview.Actions[0].Action != "error-conflict" || !strings.Contains(preview.Actions[0].Problem, "not installed by syllago") {
		t.Errorf("preview: got %+v, want an error-conflict naming the user's server", preview.Actions)
	}

	opts.Mode = "keep"
	_, err = Apply(manifest, cat, provider.Cursor, opts)
	if err == nil || !strings.HasPrefix(err.Error(), "conflict: ") || !strings.Contains(err.Error(), "not installed by syllago") {
		t.Fatalf("Apply: got %v, want a conflict refusing the user's server", err)
	}
	if got, _ := os.ReadFile(cfgPath); string(got) != mine {
		t.Errorf("config changed to %s", got)
	}
	if _, _, err := snapshot.Load(projectRoot); !errors.Is(err, snapshot.ErrNoSnapshot) {
		t.Errorf("snapshot: got %v, want none", err)
	}
}

// TestApply_RollbackLetsARetrySucceed: a failure after the MCP writes puts
// the config back as it was, deleting one the apply created, so a retry
// does not find the loadout's own servers and refuse them as the user's.
func TestApply_RollbackLetsARetrySucceed(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a directory the test cannot write to")
	}
	tests := []struct {
		name     string
		existing string
	}{
		{"existing config", `{"mcpServers":{"mine":{"command":"x"}}}`},
		{"absent config", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			projectRoot, cfgPath, manifest, cat := setupMCPEnv(t, `{"mcpServers":{"srv":{"command":"node"},"two":{"command":"y"}}}`, tt.existing)
			cat.Items = append(cat.Items, catalog.ContentItem{Name: "two", Type: catalog.MCP, Path: cat.Items[0].Path, ServerKey: "two"})
			manifest.MCP = append(manifest.MCP, ItemRef{Name: "two"})
			if tt.existing == "" {
				os.Remove(cfgPath)
			}
			// installed.json is saved once, after every placement, so a
			// .syllago the apply cannot write to fails it once both servers
			// are in.
			dir := filepath.Join(projectRoot, ".syllago")
			os.MkdirAll(filepath.Join(dir, "snapshots"), 0755)
			os.Chmod(dir, 0555)
			t.Cleanup(func() { os.Chmod(dir, 0755) })
			opts := ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: t.TempDir(), RepoRoot: projectRoot}

			if _, err := Apply(manifest, cat, provider.Cursor, opts); err == nil || !strings.Contains(err.Error(), "rolled back") {
				t.Fatalf("Apply: got %v, want a rolled-back failure", err)
			}
			got, err := os.ReadFile(cfgPath)
			if tt.existing == "" && !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("rollback left the config the apply created: %s (err %v)", got, err)
			}
			if tt.existing != "" && string(got) != tt.existing {
				t.Errorf("rollback left the config as %s, want %s", got, tt.existing)
			}

			os.Chmod(dir, 0755)
			if _, err := Apply(manifest, cat, provider.Cursor, opts); err != nil {
				t.Fatalf("retry: %v", err)
			}
			got, _ = os.ReadFile(cfgPath)
			if !strings.Contains(string(got), `"srv"`) || !strings.Contains(string(got), `"two"`) {
				t.Errorf("retry: config is %s, want srv and two", got)
			}
		})
	}
}

// TestApply_FailedRollbackKeepsTheSnapshot: a snapshot that did not restore
// holds the only copy of the files the apply changed, so the apply keeps it
// and says so, and loadout remove restores them from it.
func TestApply_FailedRollbackKeepsTheSnapshot(t *testing.T) {
	existing := `{"mcpServers":{"mine":{"command":"x"}}}`
	projectRoot, cfgPath, manifest, cat := setupMCPEnv(t, `{"command":"node"}`, existing)
	dir := filepath.Join(projectRoot, ".syllago")
	os.MkdirAll(filepath.Join(dir, "snapshots"), 0755)
	os.Chmod(dir, 0555)
	t.Cleanup(func() { os.Chmod(dir, 0755) })
	restoreSnapshot = func(string, *snapshot.SnapshotManifest) error { return errors.New("disk full") }
	t.Cleanup(func() { restoreSnapshot = snapshot.Restore })

	_, err := Apply(manifest, cat, provider.Cursor, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: t.TempDir(), RepoRoot: projectRoot})
	if err == nil || !strings.Contains(err.Error(), "rolling back failed: disk full") || !strings.Contains(err.Error(), "syllago loadout remove") {
		t.Fatalf("Apply: got %v, want a failed rollback that names loadout remove", err)
	}
	if _, _, err := snapshot.Load(projectRoot); err != nil {
		t.Fatalf("snapshot.Load after the failed rollback: %v", err)
	}

	os.Chmod(dir, 0755)
	restoreSnapshot = snapshot.Restore
	if _, err := Remove(RemoveOptions{Auto: true, ProjectRoot: projectRoot}); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got, _ := os.ReadFile(cfgPath); string(got) != existing {
		t.Errorf("config after remove: got %s, want %s", got, existing)
	}
}

// TestApply_SkipsAServerInstalledUnderTheLegacyRoot: a server a record
// under the global content dir placed is installed already, so the apply
// skips it as it skips one this project's installed.json records.
func TestApply_SkipsAServerInstalledUnderTheLegacyRoot(t *testing.T) {
	projectRoot, cfgPath, manifest, cat := setupMCPEnv(t, `{"command":"node"}`, `{"mcpServers":{"legacy-srv":{"command":"node"}}}`)
	cat.Items[0].Name, cat.Items[0].ServerKey = "legacy-srv", "legacy-srv"
	manifest.MCP = []ItemRef{{Name: "legacy-srv"}}
	legacyRoot := catalog.GlobalContentDirOverride
	legacy := &installer.Installed{MCP: []installer.InstalledMCP{{Name: "legacy-srv", ServerKey: "legacy-srv", Source: "manual", Provider: "cursor"}}}
	if err := installer.SaveInstalled(legacyRoot, legacy); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}
	t.Cleanup(func() { installer.SaveInstalled(legacyRoot, &installer.Installed{}) })
	before, _ := os.ReadFile(cfgPath)
	opts := ApplyOptions{Mode: "preview", ProjectRoot: projectRoot, HomeDir: t.TempDir(), RepoRoot: projectRoot}

	result, err := Apply(manifest, cat, provider.Cursor, opts)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(result.Actions) != 1 || result.Actions[0].Action != "skip-exists" {
		t.Fatalf("preview actions: got %+v, want one skip-exists", result.Actions)
	}
	opts.Mode = "keep"
	if _, err := Apply(manifest, cat, provider.Cursor, opts); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got, _ := os.ReadFile(cfgPath); string(got) != string(before) {
		t.Errorf("config: got %s, want it unchanged as %s", got, before)
	}
}
