package installer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// TestPlaceMCP_LeavesInstalledAndBackupToTheCaller: a caller placing
// several items keeps installed.json and its own backup of the config, so
// PlaceMCP records into the caller's inst under the caller's source and
// writes neither file. Install writes both.
func TestPlaceMCP_LeavesInstalledAndBackupToTheCaller(t *testing.T) {
	isolateLegacyRoot(t)
	item := writeMCPItem(t, t.TempDir(), "gh")

	cfgPath := filepath.Join(t.TempDir(), "mcp.json")
	os.WriteFile(cfgPath, []byte(`{}`), 0644)
	overrideMCPConfigPaths(t, map[string]string{"cursor": cfgPath})
	projectRoot := t.TempDir()
	inst := &Installed{}

	if _, err := PlaceMCP(item, provider.Cursor, projectRoot, inst, "loadout:dev"); err != nil {
		t.Fatalf("PlaceMCP: %v", err)
	}
	if len(inst.MCP) != 1 || inst.MCP[0].Source != "loadout:dev" || inst.MCP[0].Provider != "cursor" {
		t.Errorf("record: got %+v, want one cursor record from loadout:dev", inst.MCP)
	}
	if _, err := os.Stat(installedPath(projectRoot)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("PlaceMCP wrote installed.json (stat err %v)", err)
	}
	if _, err := os.Stat(cfgPath + ".bak"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("PlaceMCP wrote a backup (stat err %v)", err)
	}

	os.WriteFile(cfgPath, []byte(`{}`), 0644)
	if _, err := installMCP(item, provider.Cursor, projectRoot); err != nil {
		t.Fatalf("installMCP: %v", err)
	}
	saved, err := LoadInstalled(projectRoot)
	if err != nil || len(saved.MCP) != 1 || saved.MCP[0].Source != "export" {
		t.Errorf("installed.json after install: got %+v (err %v), want one record from export", saved, err)
	}
	if _, err := os.Stat(cfgPath + ".bak"); err != nil {
		t.Errorf("install wrote no backup: %v", err)
	}
}

// TestInstallMCP_RefusesAConfigItCannotMergeInto: the merge writes into
// invalid JSON without complaint, so a config that is not JSON once its
// comments are gone fails the install and stays as it was. An empty file
// holds no servers yet.
func TestInstallMCP_RefusesAConfigItCannotMergeInto(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr bool
	}{
		{"trailing comma", `{"theme":"x",}`, true},
		{"truncated", `{"theme":`, true},
		{"empty", "", false},
		{"blank", " \n", false},
		{"comments", "// mine\n{\"theme\":\"x\"}", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isolateLegacyRoot(t)
			item := writeMCPItem(t, t.TempDir(), "gh")
			cfgPath := filepath.Join(t.TempDir(), "mcp.json")
			os.WriteFile(cfgPath, []byte(tt.config), 0644)
			overrideMCPConfigPaths(t, map[string]string{"cursor": cfgPath})

			_, err := installMCP(item, provider.Cursor, t.TempDir())
			got, _ := os.ReadFile(cfgPath)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
					t.Errorf("error: got %v, want invalid JSON", err)
				}
				if string(got) != tt.config {
					t.Errorf("config changed to %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("installMCP: %v", err)
			}
			if !strings.Contains(string(got), `"gh":{"command":"node"}`) {
				t.Errorf("server not merged: %s", got)
			}
		})
	}
}

// TestInstallMCP_RefusesAServerTheUserDefined: a server of the same name
// that syllago did not install stays, and the error says how to make room
// for the item rather than offering a flag that does not override it.
func TestInstallMCP_RefusesAServerTheUserDefined(t *testing.T) {
	isolateLegacyRoot(t)
	item := writeMCPItem(t, t.TempDir(), "gh")
	cfgPath := filepath.Join(t.TempDir(), "mcp.json")
	const mine = `{"mcpServers":{"gh":{"command":"my-gh"}}}`
	os.WriteFile(cfgPath, []byte(mine), 0644)
	overrideMCPConfigPaths(t, map[string]string{"cursor": cfgPath})

	_, err := installMCP(item, provider.Cursor, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "rename or remove it there first") {
		t.Errorf("error: got %v, want the rename-or-remove hint", err)
	}
	if got, _ := os.ReadFile(cfgPath); string(got) != mine {
		t.Errorf("config changed to %s", got)
	}
}
