package loadout

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
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

// setupUnsupportedHookEnv builds a devin-targeted env with one rule that
// works and one hook whose event (worktree_create) devin has no
// settings key for.
func setupUnsupportedHookEnv(t *testing.T) (homeDir string, projectRoot string, manifest *Manifest, cat *catalog.Catalog, prov provider.Provider) {
	t.Helper()
	homeDir = t.TempDir()
	projectRoot = t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)
	os.MkdirAll(filepath.Join(homeDir, ".codeium", "rules"), 0755)

	ruleDir := filepath.Join(projectRoot, "content", "rules", "devin", "my-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# My Rule"), 0644)

	hookDir := filepath.Join(projectRoot, "content", "hooks", "devin", "dead-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{"spec":"hooks/0.1","hooks":[{"event":"worktree_create","handler":{"type":"command","command":"echo hi"}}]}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	manifest = &Manifest{
		Kind:     "loadout",
		Version:  1,
		Provider: "devin",
		Name:     "test-loadout",
		Rules:    []ItemRef{{Name: "my-rule"}},
		Hooks:    []ItemRef{{Name: "dead-hook"}},
	}

	cat = &catalog.Catalog{
		RepoRoot: projectRoot,
		Items: []catalog.ContentItem{
			{Name: "my-rule", Type: catalog.Rules, Provider: "devin", Path: ruleDir},
			{Name: "dead-hook", Type: catalog.Hooks, Provider: "devin", Path: hookDir},
		},
	}

	prov = provider.Provider{
		Name:      "Devin Desktop",
		Slug:      "devin",
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
	if !strings.Contains(err.Error(), "dead-hook") || !strings.Contains(err.Error(), "worktree_create") {
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
	if inst.Hooks[0].Provider != "crush" {
		t.Errorf("tracked provider: got %q, want 'crush'", inst.Hooks[0].Provider)
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

// TestCollectBackupFiles_TryModeSkipsSettingsWithoutSessionEnd is a regression
// test for the codex-review finding on PR #512: try mode used to back up the
// provider settings file unconditionally "for SessionEnd injection", but since
// injectSessionEndHook now skips providers with no session_end event, backing
// up their settings file means loadout remove would restore (clobber) a file
// this apply never wrote to. For crush that file is the real crush.json.
func TestCollectBackupFiles_TryModeSkipsSettingsWithoutSessionEnd(t *testing.T) {
	t.Parallel()
	home := t.TempDir()

	crush := provider.Provider{Name: "Crush", Slug: "crush", ConfigDir: ".config/crush"}
	cc := provider.Provider{Name: "Claude Code", Slug: "claude-code", ConfigDir: ".claude"}

	// Rules-only loadout — no merge-hook actions, so nothing writes to the
	// provider settings file except (potentially) session-end injection.
	actions := []PlannedAction{{Type: catalog.Rules, Name: "r", Action: "create-symlink"}}
	opts := ApplyOptions{Mode: "try", HomeDir: home, ProjectRoot: t.TempDir()}

	for _, f := range collectBackupFiles(actions, crush, opts) {
		if strings.HasSuffix(f, "crush.json") {
			t.Errorf("crush has no session_end event; try-mode rules-only apply must not back up crush.json (remove would clobber user edits), got %v", f)
		}
	}

	found := false
	for _, f := range collectBackupFiles(actions, cc, opts) {
		if strings.HasSuffix(f, "settings.json") {
			found = true
		}
	}
	if !found {
		t.Error("claude-code supports session_end; try-mode must back up settings.json so auto-revert injection can be reverted")
	}
}

// TestApply_RollbackReadsItsOwnSnapshot: a failed apply restores from the
// snapshot it took, so an unreadable directory another run left among the
// snapshots does not stop the rollback.
func TestApply_RollbackReadsItsOwnSnapshot(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a directory the test cannot read")
	}
	homeDir, projectRoot, manifest, cat, prov := setupTestEnv(t)
	syllagoDir := filepath.Join(projectRoot, ".syllago")
	leftover := filepath.Join(syllagoDir, "snapshots", "leftover")
	os.MkdirAll(leftover, 0755)
	os.Chmod(leftover, 0)
	// installed.json cannot be saved, which fails the apply after its writes.
	os.Chmod(syllagoDir, 0555)
	t.Cleanup(func() {
		os.Chmod(syllagoDir, 0755)
		os.Chmod(leftover, 0755)
	})

	_, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot})
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("Apply: got %v, want a rolled-back error", err)
	}
	if _, err := os.Lstat(filepath.Join(homeDir, ".claude", "settings.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("settings.json the apply created is still there (lstat err %v)", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(homeDir, ".claude", "rules")); len(entries) != 0 {
		t.Errorf("rule symlink still there: %v", entries)
	}
}

// A claim makes the path the placement fills and fails on anything already
// there, leaving it as it was.
func TestClaim(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		method  installer.InstallMethod
		srcDir  bool
		wantDir bool
	}{
		{"symlink", installer.MethodSymlink, true, false},
		{"copy of a directory", installer.MethodCopy, true, true},
		{"copy of a file", installer.MethodCopy, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			src := filepath.Join(dir, "src")
			if tc.srcDir {
				if err := os.Mkdir(src, 0o755); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}

			dst := filepath.Join(dir, "new", "dst")
			if err := claim(src, dst, tc.method); err != nil {
				t.Fatalf("claim on an empty path: %v", err)
			}
			info, err := os.Lstat(dst)
			if err != nil {
				t.Fatalf("claimed path: %v", err)
			}
			if isLink := info.Mode()&os.ModeSymlink != 0; isLink != (tc.method == installer.MethodSymlink) || info.IsDir() != tc.wantDir {
				t.Errorf("claimed path has mode %v", info.Mode())
			}

			for _, theirs := range []bool{false, true} {
				taken := filepath.Join(dir, fmt.Sprintf("taken-%v", theirs))
				if theirs {
					if err := os.MkdirAll(filepath.Join(taken, "mine"), 0o755); err != nil {
						t.Fatal(err)
					}
				} else if err := os.WriteFile(taken, []byte("mine"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := claim(src, taken, tc.method); !errors.Is(err, fs.ErrExist) {
					t.Errorf("claim on a taken path: got %v, want fs.ErrExist", err)
				}
			}
			if got, err := os.ReadFile(filepath.Join(dir, "taken-false")); err != nil || string(got) != "mine" {
				t.Errorf("taken file after a refused claim: %q, %v", got, err)
			}
			if _, err := os.Stat(filepath.Join(dir, "taken-true", "mine")); err != nil {
				t.Errorf("taken directory after a refused claim: %v", err)
			}
		})
	}
}

// A placement that will not delete is reported, so rollback keeps the
// snapshot instead of claiming success. Only the placements it deleted come
// back, so only those leave the snapshot, and the error names the rest.
func TestUnplace_ReportsFailures(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a directory the test cannot write to")
	}
	parent := t.TempDir()
	placedCopy := filepath.Join(parent, "placed")
	if err := os.MkdirAll(filepath.Join(placedCopy, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(placedCopy, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(placedCopy, 0o755) })

	gone := filepath.Join(parent, "gone")
	if err := os.Symlink(placedCopy, gone); err != nil {
		t.Fatal(err)
	}

	placed := []snapshot.SymlinkRecord{{Path: gone}, {Path: placedCopy, Copied: true}}
	snapshotDir, err := snapshot.Create(t.TempDir(), "l", "keep", nil, placed, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	removed, err := unplace(placed, snapshotDir)
	if err == nil {
		t.Fatal("expected an error deleting a copy whose contents cannot be removed")
	}
	if !slices.Equal(removed, []string{gone}) {
		t.Errorf("removed = %q, want only %q", removed, gone)
	}
	if sm, err := snapshot.ReadManifest(snapshotDir); err != nil || len(sm.Symlinks) != 1 || sm.Symlinks[0].Path != placedCopy {
		t.Errorf("recorded symlinks = %v (%v), want only %s", sm, err, placedCopy)
	}
	if got, want := leftBehind(placed, removed, nil), "; rollback could not delete "+placedCopy+", which the snapshot does not record, so delete those by hand"; got != want {
		t.Errorf("leftBehind unrecorded = %q, want %q", got, want)
	}
	if got, want := leftBehind(placed, removed, placed), "; rollback could not delete "+placedCopy; got != want {
		t.Errorf("leftBehind recorded = %q, want %q", got, want)
	}
}

// Each placement is recorded in the snapshot once it is claimed. Something
// that appears at a planned path after the preview is not the apply's: it
// is refused, left alone, and never recorded as placed.
func TestApplyActions_RecordsOnlyWhatItClaimed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	first := filepath.Join(dir, "first")
	taken := filepath.Join(dir, "taken")
	for _, d := range []string{src, filepath.Join(taken, "mine")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	snapshotDir, err := snapshot.Create(dir, "l", "keep", nil, nil, nil)
	if err != nil {
		t.Fatalf("creating snapshot: %v", err)
	}
	actions := []PlannedAction{
		{Type: catalog.Rules, Name: "a", Action: "create-symlink", Detail: first},
		{Type: catalog.Rules, Name: "b", Action: "create-symlink", Detail: taken},
	}
	refs := []ResolvedRef{
		{Type: catalog.Rules, Name: "a", Item: catalog.ContentItem{Path: src}},
		{Type: catalog.Rules, Name: "b", Item: catalog.ContentItem{Path: src}},
	}

	_, placed, err := applyActions(actions, refs, provider.Provider{}, ApplyOptions{Method: installer.MethodCopy, ProjectRoot: dir}, "l", snapshotDir)
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("applyActions: got %v, want fs.ErrExist", err)
	}
	if len(placed) != 1 || placed[0].Path != first {
		t.Errorf("placed: got %+v, want only %s", placed, first)
	}
	m, err := snapshot.ReadManifest(snapshotDir)
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}
	if len(m.Symlinks) != 1 || m.Symlinks[0].Path != first || !m.Symlinks[0].Copied {
		t.Errorf("recorded: got %+v, want only the copy at %s", m.Symlinks, first)
	}
	if _, err := os.Stat(filepath.Join(taken, "mine")); err != nil {
		t.Errorf("what appeared at %s should be untouched: %v", taken, err)
	}
}

// Remove deletes what an apply placed, so an apply refuses a destination
// remove would refuse, and two items that would claim one path, before
// it changes anything.
func TestApply_RefusesDestinationsRemoveCannotTakeBack(t *testing.T) {
	t.Run("relative base dir", func(t *testing.T) {
		homeDir, projectRoot, manifest, cat, prov := setupIntegrationEnv(t)
		// The relative path resolves under a temp dir if the check fails.
		t.Chdir(t.TempDir())
		_, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot, Resolver: config.NewResolver(nil, "rel-base")})
		if err == nil || !strings.Contains(err.Error(), "not a clean absolute path") {
			t.Fatalf("Apply: got %v, want the relative destination refused", err)
		}
		if _, _, err := snapshot.Load(projectRoot); !errors.Is(err, snapshot.ErrNoSnapshot) {
			t.Errorf("snapshot after the refusal: %v, want none", err)
		}
	})
	t.Run("two items, one path", func(t *testing.T) {
		t.Parallel()
		homeDir, projectRoot, manifest, cat, prov := setupIntegrationEnv(t)
		other := filepath.Join(projectRoot, "content", "other", "int-rule")
		if err := os.MkdirAll(other, 0o755); err != nil {
			t.Fatal(err)
		}
		manifest.Rules = append(manifest.Rules, ItemRef{Name: "int-rule-2"})
		cat.Items = append(cat.Items, catalog.ContentItem{Name: "int-rule-2", Type: catalog.Rules, Provider: "claude-code", Path: other})
		_, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot})
		if err == nil || !strings.Contains(err.Error(), "both install to") {
			t.Fatalf("Apply: got %v, want the shared destination refused", err)
		}
		if _, _, err := snapshot.Load(projectRoot); !errors.Is(err, snapshot.ErrNoSnapshot) {
			t.Errorf("snapshot after the refusal: %v, want none", err)
		}
		if _, err := os.Lstat(filepath.Join(homeDir, ".claude", "rules", "int-rule")); !os.IsNotExist(err) {
			t.Errorf("rule placed despite the refusal: %v", err)
		}
	})
}

// A copy that fails partway leaves a partial directory, which rollback
// deletes with everything the copy wrote into it.
func TestApply_CopyModeRollbackDeletesPartialCopy(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a file the test cannot read")
	}
	homeDir, projectRoot, manifest, cat, prov := setupIntegrationEnv(t)
	manifest.Hooks = nil
	// rule.md copies first, then the unreadable file fails the copy.
	unreadable := filepath.Join(projectRoot, "content", "rules", "claude-code", "int-rule", "z.md")
	if err := os.WriteFile(unreadable, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}

	_, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", Method: installer.MethodCopy, ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot})
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("Apply: got %v, want the copy to fail and roll back", err)
	}
	copyPath := filepath.Join(homeDir, ".claude", "rules", "int-rule")
	if _, err := os.Lstat(copyPath); !os.IsNotExist(err) {
		t.Errorf("partial copy %s should be gone after rollback; got err=%v", copyPath, err)
	}
}

// A copy rollback cannot delete fails the rollback, so the snapshot stays,
// and it lists only what the apply placed: a planned path the apply never
// reached is not remove's to delete when something appears there.
func TestApply_FailedCopyRollbackKeepsOnlyWhatItPlaced(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a directory the test cannot write to")
	}
	t.Cleanup(func() { restoreSnapshot = snapshot.Restore })
	homeDir, projectRoot, manifest, cat, prov := setupIntegrationEnv(t)
	manifest.Hooks = nil
	manifest.Rules = append(manifest.Rules, ItemRef{Name: "int-rule-gone"})
	cat.Items = append(cat.Items, catalog.ContentItem{
		Name: "int-rule-gone", Type: catalog.Rules, Provider: "claude-code",
		Path: filepath.Join(projectRoot, "content", "rules", "claude-code", "int-rule-gone"),
	})
	rulesDir := filepath.Join(homeDir, ".claude", "rules")
	copyPath := filepath.Join(rulesDir, "int-rule")
	unreached := filepath.Join(rulesDir, "int-rule-gone")
	restoreSnapshot = func(dir string, sm *snapshot.SnapshotManifest) error {
		if err := os.MkdirAll(filepath.Join(unreached, "mine"), 0o755); err != nil {
			return err
		}
		if err := os.Chmod(copyPath, 0o555); err != nil {
			return err
		}
		return snapshot.Restore(dir, sm)
	}
	t.Cleanup(func() { os.Chmod(copyPath, 0o755) })

	_, err := Apply(manifest, cat, prov, ApplyOptions{Mode: "keep", Method: installer.MethodCopy, ProjectRoot: projectRoot, HomeDir: homeDir, RepoRoot: projectRoot})
	if err == nil || !strings.Contains(err.Error(), "rolling back failed") {
		t.Fatalf("Apply: got %v, want a failed rollback", err)
	}
	sm, _, err := snapshot.Load(projectRoot)
	if err != nil {
		t.Fatalf("snapshot.Load after the failed rollback: %v", err)
	}
	if len(sm.Symlinks) != 1 || sm.Symlinks[0].Path != copyPath {
		t.Errorf("Symlinks: got %+v, want only %s", sm.Symlinks, copyPath)
	}
	if _, err := os.Stat(filepath.Join(unreached, "mine")); err != nil {
		t.Errorf("unreached path should be untouched: %v", err)
	}
}

// A deletion rollback cannot record stops the rest, so a rollback killed
// afterward leaves at most that one deleted path recorded.
func TestUnplace_StopsWhenItCannotRecordADeletion(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a directory the test cannot write to")
	}
	parent := t.TempDir()
	first := filepath.Join(parent, "first")
	second := filepath.Join(parent, "second")
	for _, p := range []string{first, second} {
		if err := os.Symlink(filepath.Join(parent, "src"), p); err != nil {
			t.Fatal(err)
		}
	}
	placed := []snapshot.SymlinkRecord{{Path: first}, {Path: second}}
	snapshotDir, err := snapshot.Create(parent, "l", "keep", nil, placed, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := os.Chmod(snapshotDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(snapshotDir, 0o755) })

	removed, err := unplace(placed, snapshotDir)
	if err == nil {
		t.Fatal("expected an error recording the deletion")
	}
	if !slices.Equal(removed, []string{first}) {
		t.Errorf("removed = %q, want only %q", removed, first)
	}
	if _, err := os.Lstat(second); err != nil {
		t.Errorf("unplace went on to delete %s: %v", second, err)
	}
}
