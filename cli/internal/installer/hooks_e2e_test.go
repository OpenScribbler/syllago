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

// TestInstallHook_E2E_InlineCommand tests the full installHook pipeline
// with an inline command (no script file).
func TestInstallHook_E2E_InlineCommand(t *testing.T) {
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	// Create hook file
	hookDir := filepath.Join(projectRoot, "hooks", "inline-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{"event":"PostToolUse","matcher":"Edit","hooks":[{"type":"command","command":"echo lint"}]}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	item := catalog.ContentItem{
		Name: "inline-hook",
		Type: catalog.Hooks,
		Path: hookDir,
	}

	// Create test provider with temp config dir
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(home, ".syllago-test-e2e-inline-"+filepath.Base(projectRoot))
	os.MkdirAll(configDir, 0755)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	os.WriteFile(filepath.Join(configDir, "settings.json"), []byte("{}"), 0644)

	prov := provider.Provider{
		Name:      "test-provider",
		Slug:      "test",
		ConfigDir: filepath.Base(configDir),
	}

	// Install
	result, err := installHook(item, prov, projectRoot)
	if err != nil {
		t.Fatalf("installHook: %v", err)
	}
	if !strings.Contains(result, "hooks.PostToolUse") {
		t.Errorf("result should mention event, got %q", result)
	}

	// Verify settings.json was updated
	data, _ := os.ReadFile(filepath.Join(configDir, "settings.json"))
	cmd := gjson.GetBytes(data, "hooks.PostToolUse.0.hooks.0.command").String()
	if cmd != "echo lint" {
		t.Errorf("command in settings.json: got %q, want 'echo lint'", cmd)
	}

	// Verify installed.json recorded
	inst, _ := LoadInstalled(projectRoot)
	idx := inst.FindHook("inline-hook", "PostToolUse")
	if idx < 0 {
		t.Fatal("hook not found in installed.json")
	}
}

// TestInstallHook_E2E_WithScript tests the full pipeline when a hook
// references a script file — verifies script is copied and command rewritten.
func TestInstallHook_E2E_WithScript(t *testing.T) {
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	// Create hook with a relative script reference
	hookDir := filepath.Join(projectRoot, "hooks", "script-hook")
	os.MkdirAll(hookDir, 0755)
	os.WriteFile(filepath.Join(hookDir, "lint.sh"), []byte("#!/bin/bash\necho lint"), 0755)
	hookJSON := `{"event":"PostToolUse","matcher":"Edit|Write","hooks":[{"type":"command","command":"./lint.sh --strict"}]}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	item := catalog.ContentItem{
		Name: "script-hook",
		Type: catalog.Hooks,
		Path: hookDir,
	}

	// Create test provider
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(home, ".syllago-test-e2e-script-"+filepath.Base(projectRoot))
	os.MkdirAll(configDir, 0755)
	t.Cleanup(func() {
		os.RemoveAll(configDir)
		// Also clean up the copied scripts
		scriptsDir, _ := hookScriptsDir("script-hook")
		os.RemoveAll(scriptsDir)
	})
	os.WriteFile(filepath.Join(configDir, "settings.json"), []byte("{}"), 0644)

	prov := provider.Provider{
		Name:      "test-provider",
		Slug:      "test",
		ConfigDir: filepath.Base(configDir),
	}

	// Install
	_, err = installHook(item, prov, projectRoot)
	if err != nil {
		t.Fatalf("installHook: %v", err)
	}

	// Verify script was copied to stable location
	scriptsDir, _ := hookScriptsDir("script-hook")
	copiedScript := filepath.Join(scriptsDir, "lint.sh")
	if _, statErr := os.Stat(copiedScript); statErr != nil {
		t.Fatalf("script not copied to %s: %v", copiedScript, statErr)
	}

	// Verify script content was preserved
	copiedData, _ := os.ReadFile(copiedScript)
	if !strings.Contains(string(copiedData), "echo lint") {
		t.Error("copied script content doesn't match original")
	}

	// Verify settings.json has rewritten command path
	data, _ := os.ReadFile(filepath.Join(configDir, "settings.json"))
	cmd := gjson.GetBytes(data, "hooks.PostToolUse.0.hooks.0.command").String()

	// Command should point to the stable copy, not the original
	if !strings.Contains(cmd, ".syllago/hooks/script-hook/lint.sh") {
		t.Errorf("command should point to stable copy, got %q", cmd)
	}
	// Arguments should be preserved
	if !strings.HasSuffix(cmd, " --strict") {
		t.Errorf("command should preserve args, got %q", cmd)
	}

	// Verify installed.json has the rewritten command
	inst, _ := LoadInstalled(projectRoot)
	idx := inst.FindHook("script-hook", "PostToolUse")
	if idx < 0 {
		t.Fatal("hook not found in installed.json")
	}
	if !strings.Contains(inst.Hooks[idx].Command, "lint.sh") {
		t.Errorf("installed.json command: %q", inst.Hooks[idx].Command)
	}
}

// TestInstallHook_E2E_Uninstall tests install then uninstall round-trip.
func TestInstallHook_E2E_Uninstall(t *testing.T) {
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	hookDir := filepath.Join(projectRoot, "hooks", "roundtrip-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{"event":"PreToolUse","matcher":"Bash","hooks":[{"type":"command","command":"echo check"}]}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	item := catalog.ContentItem{
		Name: "roundtrip-hook",
		Type: catalog.Hooks,
		Path: hookDir,
	}

	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".syllago-test-e2e-roundtrip-"+filepath.Base(projectRoot))
	os.MkdirAll(configDir, 0755)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	os.WriteFile(filepath.Join(configDir, "settings.json"), []byte("{}"), 0644)

	prov := provider.Provider{
		Name:      "test-provider",
		Slug:      "test",
		ConfigDir: filepath.Base(configDir),
	}

	// Install
	_, err := installHook(item, prov, projectRoot)
	if err != nil {
		t.Fatalf("installHook: %v", err)
	}

	// Verify hook is in settings.json
	data, _ := os.ReadFile(filepath.Join(configDir, "settings.json"))
	if !gjson.GetBytes(data, "hooks.PreToolUse").Exists() {
		t.Fatal("hook not in settings.json after install")
	}

	// Uninstall
	_, err = uninstallHook(item, prov, projectRoot)
	if err != nil {
		t.Fatalf("uninstallHook: %v", err)
	}

	// Verify hook is removed from settings.json
	data, _ = os.ReadFile(filepath.Join(configDir, "settings.json"))
	arr := gjson.GetBytes(data, "hooks.PreToolUse")
	if arr.Exists() && len(arr.Array()) > 0 {
		t.Error("hook should be removed from settings.json after uninstall")
	}

	// Verify removed from installed.json
	inst, _ := LoadInstalled(projectRoot)
	if inst.FindHook("roundtrip-hook", "PreToolUse") >= 0 {
		t.Error("hook should be removed from installed.json")
	}
}
