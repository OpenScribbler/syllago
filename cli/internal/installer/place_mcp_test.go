package installer

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
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
// comments and trailing commas are gone fails the install and stays as it
// was. An empty file holds no servers yet.
func TestInstallMCP_RefusesAConfigItCannotMergeInto(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr bool
	}{
		{"missing comma", `{"theme":"x" "font":"y"}`, true},
		{"truncated", `{"theme":`, true},
		{"trailing commas", "{\"theme\":\"x\",\"tabs\":[1,2,\n],\"s\":\",}\",\n}", false},
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

// TestHookInstallAndUninstall_LeaveNoSnapshot: the settings write is atomic,
// so hook install and uninstall keep no snapshot. One left behind reads as
// an active loadout to every loadout command.
func TestHookInstallAndUninstall_LeaveNoSnapshot(t *testing.T) {
	isolateLegacyRoot(t)
	item, projectRoot := writeCanonicalHookItem(t, "guard", "before_tool_execute", "shell", "echo hi")
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(settingsPath, []byte(`{}`), 0644)
	overrideHookSettingsPath(t, settingsPath)

	if _, err := installHook(item, provider.ClaudeCode, projectRoot, ScanOptions{}); err != nil {
		t.Fatalf("installHook: %v", err)
	}
	if _, _, err := snapshot.Load(projectRoot); !errors.Is(err, snapshot.ErrNoSnapshot) {
		t.Errorf("after install: got %v, want no snapshot", err)
	}
	if _, err := uninstallHook(item, provider.ClaudeCode, projectRoot); err != nil {
		t.Fatalf("uninstallHook: %v", err)
	}
	if _, _, err := snapshot.Load(projectRoot); !errors.Is(err, snapshot.ErrNoSnapshot) {
		t.Errorf("after uninstall: got %v, want no snapshot", err)
	}
}

// TestMCP_ZedSettingsWithATrailingComma: Zed's settings allow a comma
// before a closing brace, so a server in such a file still reads as
// installed and uninstalls.
func TestMCP_ZedSettingsWithATrailingComma(t *testing.T) {
	isolateLegacyRoot(t)
	item := writeMCPItem(t, t.TempDir(), "gh")
	item.ServerKey = "gh"
	cfgPath := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(cfgPath, []byte("{\"theme\":\"x\",\n}"), 0644)
	overrideMCPConfigPaths(t, map[string]string{"zed": cfgPath})
	projectRoot := t.TempDir()

	if _, err := installMCP(item, provider.Zed, projectRoot); err != nil {
		t.Fatalf("installMCP: %v", err)
	}
	data, _ := os.ReadFile(cfgPath)
	end := bytes.LastIndexByte(data, '}')
	os.WriteFile(cfgPath, append(data[:end:end], ",\n}"...), 0644)

	if status, err := mcpStatus(item, provider.Zed, projectRoot); status != StatusInstalled || err != nil {
		t.Errorf("status: got %v (err %v), want installed", status, err)
	}
	if _, err := uninstallMCP(item, provider.Zed, projectRoot); err != nil {
		t.Fatalf("uninstallMCP: %v", err)
	}
	if got, _ := os.ReadFile(cfgPath); strings.Contains(string(got), `"gh"`) {
		t.Errorf("server still configured: %s", got)
	}
}

// TestStripTrailingCommas: a comma before a closing brace or bracket goes,
// and a comma inside a string stays, escaped quotes included.
func TestStripTrailingCommas(t *testing.T) {
	tests := []struct{ in, want string }{
		{"{\"a\":1,\n}", "{\"a\":1\n}"},
		{"[1,2, ]", "[1,2 ]"},
		{`{"s":",}"}`, `{"s":",}"}`},
		{`{"s":"a\",]",}`, `{"s":"a\",]"}`},
		{`{"a":1,"b":2}`, `{"a":1,"b":2}`},
	}
	for _, tt := range tests {
		if got := string(stripTrailingCommas([]byte(tt.in))); got != tt.want {
			t.Errorf("stripTrailingCommas(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestUninstallMCP_RefusesAConfigItCannotEdit: a config that stopped being
// JSON after the install fails the uninstall and stays as it was, rather
// than taking a write that corrupts it further.
func TestUninstallMCP_RefusesAConfigItCannotEdit(t *testing.T) {
	isolateLegacyRoot(t)
	item := writeMCPItem(t, t.TempDir(), "gh")
	cfgPath := filepath.Join(t.TempDir(), "mcp.json")
	os.WriteFile(cfgPath, []byte(`{}`), 0644)
	overrideMCPConfigPaths(t, map[string]string{"cursor": cfgPath})
	projectRoot := t.TempDir()
	if _, err := installMCP(item, provider.Cursor, projectRoot); err != nil {
		t.Fatalf("installMCP: %v", err)
	}
	data, _ := os.ReadFile(cfgPath)
	broken := string(bytes.Replace(data, []byte(`"mcpServers"`), []byte(`"theme":"x" "mcpServers"`), 1))
	os.WriteFile(cfgPath, []byte(broken), 0644)

	if _, err := uninstallMCP(item, provider.Cursor, projectRoot); err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("error: got %v, want invalid JSON", err)
	}
	if got, _ := os.ReadFile(cfgPath); string(got) != broken {
		t.Errorf("config changed to %q", got)
	}
}
