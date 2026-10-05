package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

// TestForgetReverted_DropsOnlyWhatTheRevertTookOut: a record stays while
// its hook or server is still in the file, and while its file is not one
// the revert touched. It goes once a reverted file no longer holds it.
func TestForgetReverted_DropsOnlyWhatTheRevertTookOut(t *testing.T) {
	isolateLegacyRoot(t)
	hook, projectRoot := writeCanonicalHookItem(t, "guard", "before_tool_execute", "shell", "echo hi")
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(settingsPath, []byte(`{}`), 0644)
	overrideHookSettingsPath(t, settingsPath)
	server := writeMCPItem(t, t.TempDir(), "gh")
	cfgPath := filepath.Join(t.TempDir(), "mcp.json")
	os.WriteFile(cfgPath, []byte(`{}`), 0644)
	overrideMCPConfigPaths(t, map[string]string{"claude-code": cfgPath})

	if _, err := installHook(hook, provider.ClaudeCode, projectRoot, ScanOptions{}); err != nil {
		t.Fatalf("installHook: %v", err)
	}
	if _, err := installMCP(server, provider.ClaudeCode, projectRoot); err != nil {
		t.Fatalf("installMCP: %v", err)
	}
	inst, err := LoadInstalled(projectRoot)
	if err != nil || len(inst.Hooks) != 1 || len(inst.MCP) != 1 {
		t.Fatalf("installed.json: got %+v (err %v), want one hook and one server", inst, err)
	}

	ForgetReverted(inst, projectRoot, []string{settingsPath, cfgPath})
	if len(inst.Hooks) != 1 || len(inst.MCP) != 1 {
		t.Errorf("still in their files: got %d hooks and %d servers, want both kept", len(inst.Hooks), len(inst.MCP))
	}

	os.WriteFile(settingsPath, []byte(`{}`), 0644)
	os.Remove(cfgPath)
	ForgetReverted(inst, projectRoot, []string{filepath.Join(t.TempDir(), "other.json")})
	if len(inst.Hooks) != 1 || len(inst.MCP) != 1 {
		t.Errorf("files not reverted: got %d hooks and %d servers, want both kept", len(inst.Hooks), len(inst.MCP))
	}

	ForgetReverted(inst, projectRoot, []string{settingsPath})
	if len(inst.Hooks) != 0 || len(inst.MCP) != 1 {
		t.Errorf("settings reverted: got %d hooks and %d servers, want the server alone", len(inst.Hooks), len(inst.MCP))
	}
	ForgetReverted(inst, projectRoot, []string{cfgPath})
	if len(inst.MCP) != 0 {
		t.Errorf("config deleted: got %+v, want no servers", inst.MCP)
	}
}

// TestForgetReverted_MatchesAFileReachedThroughASymlink: a project applied
// at its real path and removed through a symlink to it names the same
// config two ways, and the record of a server the revert took out still
// goes.
func TestForgetReverted_MatchesAFileReachedThroughASymlink(t *testing.T) {
	isolateLegacyRoot(t)
	server := writeMCPItem(t, t.TempDir(), "gh")
	realDir := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(realDir, alias); err != nil {
		t.Skipf("symlink: %v", err)
	}
	os.WriteFile(filepath.Join(realDir, "mcp.json"), []byte(`{}`), 0644)
	overrideMCPConfigPaths(t, map[string]string{"claude-code": filepath.Join(alias, "mcp.json")})
	projectRoot := t.TempDir()
	if _, err := installMCP(server, provider.ClaudeCode, projectRoot); err != nil {
		t.Fatalf("installMCP: %v", err)
	}
	inst, err := LoadInstalled(projectRoot)
	if err != nil || len(inst.MCP) != 1 {
		t.Fatalf("installed.json: got %+v (err %v), want one server", inst, err)
	}

	os.WriteFile(filepath.Join(realDir, "mcp.json"), []byte(`{}`), 0644)
	ForgetReverted(inst, projectRoot, []string{filepath.Join(realDir, "mcp.json")})
	if len(inst.MCP) != 0 {
		t.Errorf("got %+v, want the server's record gone", inst.MCP)
	}
}
