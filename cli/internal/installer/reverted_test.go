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
// goes, also once the revert has deleted a config the apply created.
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

	os.Remove(filepath.Join(realDir, "mcp.json"))
	ForgetReverted(inst, projectRoot, []string{filepath.Join(realDir, "mcp.json")})
	if len(inst.MCP) != 0 {
		t.Errorf("got %+v, want the server's record gone", inst.MCP)
	}
}

// TestForgetReverted_RecordsWithoutAProvider: a record written before
// records named their provider could be any provider's. It goes once a
// reverted file no longer holds it and no other provider's file does, and
// stays while one does.
func TestForgetReverted_RecordsWithoutAProvider(t *testing.T) {
	isolateLegacyRoot(t)
	server := writeMCPItem(t, t.TempDir(), "gh")
	dir := t.TempDir()
	claudeCfg, cursorCfg := filepath.Join(dir, "claude.json"), filepath.Join(dir, "cursor.json")
	os.WriteFile(claudeCfg, []byte(`{}`), 0644)
	os.WriteFile(cursorCfg, []byte(`{}`), 0644)
	overrideMCPConfigPaths(t, map[string]string{"claude-code": claudeCfg, "cursor": cursorCfg})
	hook, projectRoot := writeCanonicalHookItem(t, "guard", "before_tool_execute", "shell", "echo hi")
	hookPaths := hookTestPaths(t)
	overrideHookSettingsPaths(t, hookPaths)

	if _, err := installMCP(server, provider.Cursor, projectRoot); err != nil {
		t.Fatalf("installMCP: %v", err)
	}
	if _, err := installHook(hook, provider.ClaudeCode, projectRoot, ScanOptions{}); err != nil {
		t.Fatalf("installHook: %v", err)
	}
	inst, err := LoadInstalled(projectRoot)
	if err != nil || len(inst.MCP) != 1 || len(inst.Hooks) != 1 {
		t.Fatalf("installed.json: got %+v (err %v), want one server and one hook", inst, err)
	}
	inst.MCP[0].Provider, inst.Hooks[0].Provider = "", ""

	ForgetReverted(inst, projectRoot, []string{claudeCfg, hookPaths["claude-code"]})
	if len(inst.MCP) != 1 || len(inst.Hooks) != 1 {
		t.Errorf("still held: got %d servers and %d hooks, want both kept", len(inst.MCP), len(inst.Hooks))
	}

	os.WriteFile(cursorCfg, []byte(`{}`), 0644)
	os.WriteFile(hookPaths["claude-code"], []byte(`{}`), 0644)
	ForgetReverted(inst, projectRoot, []string{filepath.Join(dir, "other.json")})
	if len(inst.MCP) != 1 || len(inst.Hooks) != 1 {
		t.Errorf("nothing reverted: got %d servers and %d hooks, want both kept", len(inst.MCP), len(inst.Hooks))
	}

	ForgetReverted(inst, projectRoot, []string{claudeCfg, hookPaths["cursor"]})
	if len(inst.MCP) != 0 || len(inst.Hooks) != 0 {
		t.Errorf("held nowhere: got %+v and %+v, want both gone", inst.MCP, inst.Hooks)
	}
}

// TestForgetReverted_KeepsAHookFromV0_14: v0.14.0 recorded a hook by the
// hash of its matcher group's JSON rather than its identity, so a record of
// that kind stays while the restored settings still hold the group.
func TestForgetReverted_KeepsAHookFromV0_14(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	overrideHookSettingsPaths(t, map[string]string{"claude-code": settingsPath})
	group := `{"matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}`
	os.WriteFile(settingsPath, []byte(`{"hooks":{"PreToolUse":[`+group+`]}}`), 0644)
	inst := &Installed{Hooks: []InstalledHook{{Name: "guard", Event: "PreToolUse", GroupHash: computeGroupHash([]byte(group)), Source: "export"}}}

	ForgetReverted(inst, t.TempDir(), []string{settingsPath})
	if len(inst.Hooks) != 1 {
		t.Fatalf("group still in settings: got %+v, want the record kept", inst.Hooks)
	}
	os.WriteFile(settingsPath, []byte(`{}`), 0644)
	ForgetReverted(inst, t.TempDir(), []string{settingsPath})
	if len(inst.Hooks) != 0 {
		t.Errorf("group gone: got %+v, want the record gone", inst.Hooks)
	}
}

// TestForgetReverted_MatchesAV0_14HookUnderItsOwnEvent: the v0.14.0 hash
// leaves out the event, so the same group under another event does not
// hold the record.
func TestForgetReverted_MatchesAV0_14HookUnderItsOwnEvent(t *testing.T) {
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	overrideHookSettingsPaths(t, map[string]string{"claude-code": settingsPath})
	group := `{"matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}`
	os.WriteFile(settingsPath, []byte(`{"hooks":{"PreToolUse":[`+group+`]}}`), 0644)
	inst := &Installed{Hooks: []InstalledHook{{Name: "guard", Event: "PostToolUse", GroupHash: computeGroupHash([]byte(group)), Source: "export"}}}

	ForgetReverted(inst, t.TempDir(), []string{settingsPath})
	if len(inst.Hooks) != 0 {
		t.Errorf("group only under PreToolUse: got %+v, want the PostToolUse record gone", inst.Hooks)
	}
}
