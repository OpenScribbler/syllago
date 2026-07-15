package loadout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/tidwall/gjson"
)

// setupTestEnv creates a minimal test environment with a catalog, manifest,
// and provider that exercise symlink + hook apply paths.
func setupTestEnv(t *testing.T) (homeDir string, projectRoot string, manifest *Manifest, cat *catalog.Catalog, prov provider.Provider) {
	t.Helper()
	homeDir = t.TempDir()
	projectRoot = t.TempDir()

	// Create .syllago dir
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	// Create provider directories
	rulesDir := filepath.Join(homeDir, ".claude", "rules")
	os.MkdirAll(rulesDir, 0755)
	os.MkdirAll(filepath.Join(homeDir, ".claude"), 0755)

	// Create a rule source
	ruleDir := filepath.Join(projectRoot, "content", "rules", "claude-code", "my-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# My Rule\nDo things."), 0644)

	// Create a hook source
	hookDir := filepath.Join(projectRoot, "content", "hooks", "claude-code", "my-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{
  "spec": "hooks/0.1",
  "hooks": [
    {
      "event": "PostToolUse",
      "matcher": ".*",
      "handler": {"type": "command", "command": "echo test"}
    }
  ]
}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	manifest = &Manifest{
		Kind:     "loadout",
		Version:  1,
		Provider: "claude-code",
		Name:     "test-loadout",
		Rules:    []ItemRef{{Name: "my-rule"}},
		Hooks:    []ItemRef{{Name: "my-hook"}},
	}

	cat = &catalog.Catalog{
		RepoRoot: projectRoot,
		Items: []catalog.ContentItem{
			{Name: "my-rule", Type: catalog.Rules, Provider: "claude-code", Path: ruleDir},
			{Name: "my-hook", Type: catalog.Hooks, Provider: "claude-code", Path: hookDir},
		},
	}

	prov = provider.Provider{
		Name:      "Claude Code",
		Slug:      "claude-code",
		ConfigDir: ".claude",
		InstallDir: func(home string, ct catalog.ContentType) string {
			switch ct {
			case catalog.Rules:
				return filepath.Join(home, ".claude", "rules")
			case catalog.Hooks:
				return "__json_merge__"
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			switch ct {
			case catalog.Rules, catalog.Hooks:
				return true
			}
			return false
		},
	}

	return
}

func TestApply_PreviewMode(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)

	opts := ApplyOptions{
		Mode:        "preview",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	result, err := Apply(manifest, cat, prov, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Preview should not create any files
	if result.SnapshotDir != "" {
		t.Error("preview mode should not create a snapshot")
	}

	// Should have planned actions
	if len(result.Actions) == 0 {
		t.Error("expected planned actions")
	}

	// Check that no symlinks were created
	rulesDir := filepath.Join(homeDir, ".claude", "rules")
	entries, _ := os.ReadDir(rulesDir)
	for _, e := range entries {
		t.Errorf("unexpected file in rules dir during preview: %s", e.Name())
	}

	// Check that settings.json was not created/modified
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	if _, err := os.Stat(settingsPath); err == nil {
		t.Error("settings.json should not exist after preview")
	}
}

func TestApply_KeepMode_CreatesSymlinks(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)

	// Only rules for this test (skip hooks to keep it focused)
	manifest.Hooks = nil
	cat.Items = cat.Items[:1] // only the rule

	opts := ApplyOptions{
		Mode:        "keep",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	result, err := Apply(manifest, cat, prov, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SnapshotDir == "" {
		t.Error("expected snapshot dir for keep mode")
	}

	// Verify symlink was created
	targetPath := filepath.Join(homeDir, ".claude", "rules", "my-rule")
	info, err := os.Lstat(targetPath)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular file")
	}
}

func TestApply_KeepMode_MergesHooks(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)

	// Only hooks for this test
	manifest.Rules = nil
	cat.Items = cat.Items[1:] // only the hook

	opts := ApplyOptions{
		Mode:        "keep",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	result, err := Apply(manifest, cat, prov, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SnapshotDir == "" {
		t.Error("expected snapshot dir")
	}

	// Verify settings.json has the hook
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}

	hooksArray := gjson.GetBytes(data, "hooks.PostToolUse")
	if !hooksArray.Exists() {
		t.Fatal("hooks.PostToolUse not found in settings.json")
	}
	if !hooksArray.IsArray() || len(hooksArray.Array()) == 0 {
		t.Fatal("hooks.PostToolUse should be a non-empty array")
	}
}

// TestApply_KeepMode_TranslatesCanonicalHook verifies applyHook translates
// canonical event names AND matcher tool names to provider-native before the
// settings merge (syllago-9qgwt, loadout path). Library hook.json stores
// canonical names ("before_tool_execute", "shell"); merging them verbatim
// writes config the provider never reads (wrong key) or never matches
// (wrong tool name regex).
func TestApply_KeepMode_TranslatesCanonicalHook(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)

	// Replace the fixture hook with a fully canonical one.
	hookDir := filepath.Join(projectRoot, "content", "hooks", "claude-code", "my-hook")
	hookJSON := `{
  "spec": "hooks/0.1",
  "hooks": [
    {
      "event": "before_tool_execute",
      "matcher": "shell",
      "handler": {"type": "command", "command": "echo guard"}
    }
  ]
}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	manifest.Rules = nil
	cat.Items = cat.Items[1:] // only the hook

	opts := ApplyOptions{
		Mode:        "keep",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	if _, err := Apply(manifest, cat, prov, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}

	entry := gjson.GetBytes(data, "hooks.PreToolUse.0")
	if !entry.Exists() {
		t.Fatalf("expected hook under native key hooks.PreToolUse, got: %s", data)
	}
	if got := entry.Get("matcher").String(); got != "Bash" {
		t.Errorf("matcher: got %q, want %q (canonical shell -> claude-code Bash)", got, "Bash")
	}
	if gjson.GetBytes(data, "hooks.before_tool_execute").Exists() {
		t.Errorf("canonical event key must not appear in settings, got: %s", data)
	}
}

func TestApply_TryMode_InjectsSessionEndHook(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)

	// Only rules to keep test simple (SessionEnd hook is injected regardless of content types)
	manifest.Hooks = nil
	cat.Items = cat.Items[:1]

	opts := ApplyOptions{
		Mode:        "try",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	result, err := Apply(manifest, cat, prov, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.AutoRevertArmed {
		t.Error("claude-code supports session_end, so AutoRevertArmed should be true")
	}

	// Verify settings.json has the SessionEnd hook
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}

	sessionEnd := gjson.GetBytes(data, "hooks.SessionEnd")
	if !sessionEnd.Exists() {
		t.Fatal("hooks.SessionEnd not found in settings.json")
	}
	if !sessionEnd.IsArray() || len(sessionEnd.Array()) == 0 {
		t.Fatal("hooks.SessionEnd should be a non-empty array")
	}

	// Check the command
	cmd := sessionEnd.Array()[0].Get("hooks.0.command").String()
	if cmd != "syllago loadout remove --auto" {
		t.Errorf("expected auto-remove command, got %q", cmd)
	}
}

func TestApply_ConflictAborts(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)

	// Only rules
	manifest.Hooks = nil
	cat.Items = cat.Items[:1]

	// Create a regular file at the target to cause a conflict
	targetPath := filepath.Join(homeDir, ".claude", "rules", "my-rule")
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	os.WriteFile(targetPath, []byte("existing content"), 0644)

	opts := ApplyOptions{
		Mode:        "keep",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	_, err := Apply(manifest, cat, prov, opts)
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestApply_ResolveFails(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, _, _, prov := setupTestEnv(t)

	// Manifest references something not in catalog
	manifest := &Manifest{
		Provider: "claude-code",
		Name:     "bad-loadout",
		Rules:    []ItemRef{{Name: "nonexistent-rule"}},
	}
	cat := &catalog.Catalog{Items: []catalog.ContentItem{}}

	opts := ApplyOptions{
		Mode:        "keep",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	_, err := Apply(manifest, cat, prov, opts)
	if err == nil {
		t.Fatal("expected resolve error")
	}
}

func TestReadJSONFileOrEmpty(t *testing.T) {
	t.Parallel()

	t.Run("valid JSON returns contents", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "settings.json")
		os.WriteFile(path, []byte(`{"hooks":[]}`), 0644)

		data, err := readJSONFileOrEmpty(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != `{"hooks":[]}` {
			t.Errorf("got %q, want %q", string(data), `{"hooks":[]}`)
		}
	})

	t.Run("missing file returns empty object", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "nonexistent.json")

		data, err := readJSONFileOrEmpty(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != "{}" {
			t.Errorf("got %q, want %q", string(data), "{}")
		}
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "settings.json")
		os.WriteFile(path, []byte(`{"hooks": [`), 0644)

		_, err := readJSONFileOrEmpty(path)
		if err == nil {
			t.Fatal("expected error for malformed JSON")
		}
		if !strings.Contains(err.Error(), "invalid JSON") {
			t.Errorf("error should mention invalid JSON, got: %v", err)
		}
	})

	t.Run("empty file returns error", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "settings.json")
		os.WriteFile(path, []byte(""), 0644)

		_, err := readJSONFileOrEmpty(path)
		if err == nil {
			t.Fatal("expected error for empty file")
		}
		if !strings.Contains(err.Error(), "invalid JSON") {
			t.Errorf("error should mention invalid JSON, got: %v", err)
		}
	})

	t.Run("truncated JSON returns error", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "settings.json")
		os.WriteFile(path, []byte(`{"hooks":[{"type":"command","command":"echo`), 0644)

		_, err := readJSONFileOrEmpty(path)
		if err == nil {
			t.Fatal("expected error for truncated JSON")
		}
		if !strings.Contains(err.Error(), "invalid JSON") {
			t.Errorf("error should mention invalid JSON, got: %v", err)
		}
	})
}

// setupUnsupportedHookEnv builds a windsurf-targeted env with one rule that
// works and one hook whose event (before_tool_execute) windsurf has no
// settings key for.
func setupUnsupportedHookEnv(t *testing.T) (homeDir string, projectRoot string, manifest *Manifest, cat *catalog.Catalog, prov provider.Provider) {
	t.Helper()
	homeDir = t.TempDir()
	projectRoot = t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".codeium", "rules"), 0755)

	ruleDir := filepath.Join(projectRoot, "content", "rules", "windsurf", "my-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# My Rule"), 0644)

	hookDir := filepath.Join(projectRoot, "content", "hooks", "windsurf", "dead-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{"spec":"hooks/0.1","hooks":[{"event":"before_tool_execute","handler":{"type":"command","command":"echo hi"}}]}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	manifest = &Manifest{
		Kind:     "loadout",
		Version:  1,
		Provider: "windsurf",
		Name:     "test-loadout",
		Rules:    []ItemRef{{Name: "my-rule"}},
		Hooks:    []ItemRef{{Name: "dead-hook"}},
	}

	cat = &catalog.Catalog{
		RepoRoot: projectRoot,
		Items: []catalog.ContentItem{
			{Name: "my-rule", Type: catalog.Rules, Provider: "windsurf", Path: ruleDir},
			{Name: "dead-hook", Type: catalog.Hooks, Provider: "windsurf", Path: hookDir},
		},
	}

	prov = provider.Provider{
		Name:      "Windsurf",
		Slug:      "windsurf",
		ConfigDir: ".codeium",
		InstallDir: func(home string, ct catalog.ContentType) string {
			switch ct {
			case catalog.Rules:
				return filepath.Join(home, ".codeium", "rules")
			case catalog.Hooks:
				return "__json_merge__"
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Rules || ct == catalog.Hooks
		},
	}

	return
}

// TestApply_UnsupportedHook_FailsWithoutSkipFlag: applying a loadout that
// contains a hook the target provider cannot read fails outright (nothing
// applied) unless SkipUnsupported is set — no silent partial coverage, no
// dead config (syllago-xqlc1).
func TestApply_UnsupportedHook_FailsWithoutSkipFlag(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupUnsupportedHookEnv(t)

	opts := ApplyOptions{
		Mode:        "keep",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	_, err := Apply(manifest, cat, prov, opts)
	if err == nil {
		t.Fatal("expected error applying loadout with unsupported hook event")
	}
	if !strings.Contains(err.Error(), "dead-hook") || !strings.Contains(err.Error(), "before_tool_execute") {
		t.Errorf("error should name the hook and event, got: %v", err)
	}

	// Nothing should have been applied.
	if _, statErr := os.Stat(filepath.Join(homeDir, ".codeium", "rules", "my-rule")); statErr == nil {
		t.Error("rule symlink should not exist after rejected apply")
	}
	if _, statErr := os.Stat(filepath.Join(homeDir, ".codeium", "settings.json")); statErr == nil {
		t.Error("settings.json should not exist after rejected apply")
	}
}

// TestApply_UnsupportedHook_SkippedWithFlag: with SkipUnsupported set, the
// incompatible hook is skipped (and reported) while the rest of the loadout
// applies normally.
func TestApply_UnsupportedHook_SkippedWithFlag(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupUnsupportedHookEnv(t)

	opts := ApplyOptions{
		Mode:            "keep",
		ProjectRoot:     projectRoot,
		HomeDir:         homeDir,
		RepoRoot:        projectRoot,
		SkipUnsupported: true,
	}

	result, err := Apply(manifest, cat, prov, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The rule applied; the hook did not.
	if _, statErr := os.Lstat(filepath.Join(homeDir, ".codeium", "rules", "my-rule")); statErr != nil {
		t.Errorf("rule symlink should exist: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(homeDir, ".codeium", "settings.json")); statErr == nil {
		data, _ := os.ReadFile(filepath.Join(homeDir, ".codeium", "settings.json"))
		if gjson.GetBytes(data, "hooks").Exists() {
			t.Errorf("no hooks should have been merged, got: %s", data)
		}
	}

	var skipped *PlannedAction
	for i := range result.Actions {
		if result.Actions[i].Action == "skip-unsupported" {
			skipped = &result.Actions[i]
		}
	}
	if skipped == nil {
		t.Fatal("expected a skip-unsupported action in the result")
	}
	if skipped.Name != "dead-hook" {
		t.Errorf("skip-unsupported action should be dead-hook, got %s", skipped.Name)
	}
}

// TestApplyHook_CrushFlattensAndRoutes: a loadout hook applied to crush must
// land as a FLAT entry ({name, matcher, command}) in crush.json — not as a
// CC-shape matcher group in a settings.json crush never reads (syllago-xqlc1,
// mirrors installer hookSettingsPathImpl + FlattenForCrush).
func TestApplyHook_CrushFlattensAndRoutes(t *testing.T) {
	t.Parallel()
	homeDir := t.TempDir()
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	hookDir := filepath.Join(projectRoot, "content", "hooks", "crush", "guard-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{"spec":"hooks/0.1","hooks":[{"name":"guard","event":"before_tool_execute","matcher":"shell","handler":{"type":"command","command":"echo guard"}}]}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	manifest := &Manifest{
		Kind:     "loadout",
		Version:  1,
		Provider: "crush",
		Name:     "crush-loadout",
		Hooks:    []ItemRef{{Name: "guard-hook"}},
	}

	cat := &catalog.Catalog{
		RepoRoot: projectRoot,
		Items: []catalog.ContentItem{
			{Name: "guard-hook", Type: catalog.Hooks, Provider: "crush", Path: hookDir},
		},
	}

	prov := provider.Provider{
		Name:      "Crush",
		Slug:      "crush",
		ConfigDir: ".config/crush",
		InstallDir: func(home string, ct catalog.ContentType) string {
			if ct == catalog.Hooks {
				return "__json_merge__"
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool {
			return ct == catalog.Hooks
		},
	}

	opts := ApplyOptions{
		Mode:        "keep",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	if _, err := Apply(manifest, cat, prov, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The hook must land in crush.json, not settings.json.
	crushPath := filepath.Join(homeDir, ".config", "crush", "crush.json")
	data, readErr := os.ReadFile(crushPath)
	if readErr != nil {
		t.Fatalf("crush.json should exist: %v", readErr)
	}
	if _, statErr := os.Stat(filepath.Join(homeDir, ".config", "crush", "settings.json")); statErr == nil {
		t.Error("settings.json should not be written for crush")
	}

	entry := gjson.GetBytes(data, "hooks.PreToolUse.0")
	if !entry.Exists() {
		t.Fatalf("expected hooks.PreToolUse.0 in crush.json, got: %s", data)
	}
	if got := entry.Get("command").String(); got != "echo guard" {
		t.Errorf("command: got %q, want 'echo guard'", got)
	}
	if got := entry.Get("matcher").String(); got != "bash" {
		t.Errorf("matcher: got %q, want 'bash' (canonical shell -> crush native)", got)
	}
	if got := entry.Get("name").String(); got != "guard" {
		t.Errorf("name: got %q, want 'guard'", got)
	}
	// Flat entry — no nested CC-shape hooks array.
	if entry.Get("hooks").Exists() {
		t.Errorf("crush entry must be flat, got nested hooks array: %s", entry.Raw)
	}

	// Tracking must record the command from the flat entry shape.
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("loading installed.json: %v", err)
	}
	if len(inst.Hooks) != 1 {
		t.Fatalf("expected 1 tracked hook, got %d", len(inst.Hooks))
	}
	if inst.Hooks[0].Command != "echo guard" {
		t.Errorf("tracked command: got %q, want 'echo guard'", inst.Hooks[0].Command)
	}
}

// TestApply_TryMode_CrushNoSessionEndCorruption: crush has no session_end
// event, so try-mode auto-revert injection must be skipped rather than writing
// a dead CC-shape hooks.SessionEnd group into the real crush.json. A warning
// tells the user to revert manually (syllago-xqlc1, codex review finding).
func TestApply_TryMode_CrushNoSessionEndCorruption(t *testing.T) {
	t.Parallel()
	homeDir := t.TempDir()
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	ruleDir := filepath.Join(projectRoot, "content", "rules", "crush", "my-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "AGENTS.md"), []byte("# Rule"), 0644)

	manifest := &Manifest{
		Kind:     "loadout",
		Version:  1,
		Provider: "crush",
		Name:     "crush-try",
		Rules:    []ItemRef{{Name: "my-rule"}},
	}
	cat := &catalog.Catalog{
		RepoRoot: projectRoot,
		Items: []catalog.ContentItem{
			{Name: "my-rule", Type: catalog.Rules, Provider: "crush", Path: ruleDir},
		},
	}
	prov := provider.Provider{
		Name:      "Crush",
		Slug:      "crush",
		ConfigDir: ".config/crush",
		InstallDir: func(home string, ct catalog.ContentType) string {
			if ct == catalog.Rules {
				return filepath.Join(home, ".config", "crush", "rules")
			}
			return ""
		},
		SupportsType: func(ct catalog.ContentType) bool { return ct == catalog.Rules },
	}

	opts := ApplyOptions{
		Mode:        "try",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	result, err := Apply(manifest, cat, prov, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AutoRevertArmed {
		t.Error("crush has no session_end event, so AutoRevertArmed should be false")
	}

	// crush.json must not have been created/corrupted by SessionEnd injection.
	crushPath := filepath.Join(homeDir, ".config", "crush", "crush.json")
	if data, statErr := os.ReadFile(crushPath); statErr == nil {
		if gjson.GetBytes(data, "hooks").Exists() {
			t.Errorf("crush.json should have no injected hooks, got: %s", data)
		}
	}

	// The user must be warned that auto-revert is unavailable.
	var warned bool
	for _, w := range result.Warnings {
		if strings.Contains(w, "auto-revert") || strings.Contains(w, "session-end") {
			warned = true
		}
	}
	if !warned {
		t.Errorf("expected a no-auto-revert warning, got warnings: %v", result.Warnings)
	}
}
