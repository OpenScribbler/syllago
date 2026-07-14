package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/tidwall/gjson"
)

// TestHookSettingsPath_Crush verifies crush hooks merge into the unified
// crush.json (XDG global config), not a settings.json.
func TestHookSettingsPath_Crush(t *testing.T) {
	got, err := hookSettingsPath(provider.Crush)
	if err != nil {
		t.Fatalf("hookSettingsPath: %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join(".config", "crush", "crush.json")) {
		t.Errorf("expected path ending in .config/crush/crush.json, got %q", got)
	}
}

// overrideHookSettingsPath points hookSettingsPath at a fixed file for the
// duration of the test. Not parallel-safe (mutates a package global).
func overrideHookSettingsPath(t *testing.T, path string) {
	t.Helper()
	orig := hookSettingsPath
	hookSettingsPath = func(prov provider.Provider) (string, error) { return path, nil }
	t.Cleanup(func() { hookSettingsPath = orig })
}

// TestInstallHook_E2E_Crush runs the full install/uninstall pipeline against
// crush's flat hook entry shape ({command, matcher, timeout} — seconds, no
// nested hooks array).
func TestInstallHook_E2E_Crush(t *testing.T) {
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	hookDir := filepath.Join(projectRoot, "hooks", "crush-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{"spec":"hooks/0.1","hooks":[{"name":"lint-guard","event":"before_tool_execute","matcher":"shell","handler":{"type":"command","command":"echo lint","timeout":5}}]}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	item := catalog.ContentItem{
		Name: "crush-hook",
		Type: catalog.Hooks,
		Path: hookDir,
	}

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "crush.json")
	os.WriteFile(configPath, []byte(`{"mcp":{"existing":{"command":"keep-me"}}}`), 0644)
	overrideHookSettingsPath(t, configPath)

	result, err := installHook(item, provider.Crush, projectRoot)
	if err != nil {
		t.Fatalf("installHook: %v", err)
	}
	if !strings.Contains(result, "hooks.PreToolUse") {
		t.Errorf("result should mention the native event, got %q", result)
	}

	data, _ := os.ReadFile(configPath)
	entry := gjson.GetBytes(data, "hooks.PreToolUse.0")
	if !entry.Exists() {
		t.Fatalf("expected hooks.PreToolUse.0 in crush.json, got: %s", data)
	}
	if got := entry.Get("command").String(); got != "echo lint" {
		t.Errorf("command: got %q, want 'echo lint'", got)
	}
	if entry.Get("hooks").Exists() {
		t.Errorf("crush entry must be flat, found nested hooks array: %s", entry.Raw)
	}
	// Canonical matcher tool names must translate to crush's native names
	// (Codex review finding on PR #505).
	if got := entry.Get("matcher").String(); got != "bash" {
		t.Errorf("matcher: got %q, want %q (canonical shell -> crush bash)", got, "bash")
	}
	// The manifest's optional hook name maps to crush's display name field.
	if got := entry.Get("name").String(); got != "lint-guard" {
		t.Errorf("name: got %q, want %q", got, "lint-guard")
	}
	// Canonical timeout is seconds; crush reads seconds — value unchanged.
	if got := entry.Get("timeout").Int(); got != 5 {
		t.Errorf("timeout: got %d, want 5", got)
	}
	// Sibling config keys must survive the merge.
	if gjson.GetBytes(data, "mcp.existing.command").String() != "keep-me" {
		t.Error("existing crush.json content was clobbered by hook install")
	}

	// installed.json tracks the hook with the extracted command.
	inst, _ := LoadInstalled(projectRoot)
	idx := inst.FindHook("crush-hook", "PreToolUse")
	if idx < 0 {
		t.Fatal("hook not found in installed.json")
	}
	if inst.Hooks[idx].Command != "echo lint" {
		t.Errorf("tracked command: got %q, want 'echo lint'", inst.Hooks[idx].Command)
	}

	// Status sees it as installed.
	if status := checkHookStatus(item, provider.Crush, projectRoot); status != StatusInstalled {
		t.Errorf("expected StatusInstalled, got %v", status)
	}

	// Uninstall removes the entry and the tracking record.
	if _, err := uninstallHook(item, provider.Crush, projectRoot); err != nil {
		t.Fatalf("uninstallHook: %v", err)
	}
	data, _ = os.ReadFile(configPath)
	if gjson.GetBytes(data, "hooks.PreToolUse.0").Exists() {
		t.Errorf("hook entry should be removed, got: %s", data)
	}
	if gjson.GetBytes(data, "mcp.existing.command").String() != "keep-me" {
		t.Error("existing crush.json content was clobbered by hook uninstall")
	}
	inst, _ = LoadInstalled(projectRoot)
	if inst.FindHook("crush-hook", "PreToolUse") >= 0 {
		t.Error("hook should be removed from installed.json")
	}
}

// TestInstallHook_Crush_RejectsUnsupportedEvent: crush fires hooks only on
// PreToolUse; installing any other event would write dead config that crush
// never reads (Codex review finding on PR #505).
func TestInstallHook_Crush_RejectsUnsupportedEvent(t *testing.T) {
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	hookDir := filepath.Join(projectRoot, "hooks", "session-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{"spec":"hooks/0.1","hooks":[{"event":"session_start","handler":{"type":"command","command":"echo hi"}}]}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	item := catalog.ContentItem{
		Name: "session-hook",
		Type: catalog.Hooks,
		Path: hookDir,
	}

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "crush.json")
	os.WriteFile(configPath, []byte(`{}`), 0644)
	overrideHookSettingsPath(t, configPath)

	if _, err := installHook(item, provider.Crush, projectRoot); err == nil {
		t.Fatal("expected error installing a session_start hook to crush")
	}
	data, _ := os.ReadFile(configPath)
	if gjson.GetBytes(data, "hooks").Exists() {
		t.Errorf("no hook should have been written, got: %s", data)
	}
}

// TestInstallHook_Crush_RejectsNonCommand: crush hooks are shell commands
// only; a non-command handler cannot be flattened into a crush entry.
func TestInstallHook_Crush_RejectsNonCommand(t *testing.T) {
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	hookDir := filepath.Join(projectRoot, "hooks", "prompt-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{"spec":"hooks/0.1","hooks":[{"event":"before_tool_execute","handler":{"type":"prompt","prompt":"Is this safe?"}}]}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	item := catalog.ContentItem{
		Name: "prompt-hook",
		Type: catalog.Hooks,
		Path: hookDir,
	}

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "crush.json")
	os.WriteFile(configPath, []byte(`{}`), 0644)
	overrideHookSettingsPath(t, configPath)

	if _, err := installHook(item, provider.Crush, projectRoot); err == nil {
		t.Fatal("expected error installing a prompt hook to crush")
	}
	data, _ := os.ReadFile(configPath)
	if gjson.GetBytes(data, "hooks").Exists() {
		t.Errorf("no hook should have been written, got: %s", data)
	}
}
