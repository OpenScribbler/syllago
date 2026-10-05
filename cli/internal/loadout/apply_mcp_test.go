package loadout

import (
	"errors"
	"os"
	"path/filepath"
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
