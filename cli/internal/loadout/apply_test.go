package loadout

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
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
  "event": "PostToolUse",
  "matcher": ".*",
  "hooks": [{"type": "command", "command": "echo test"}]
}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	manifest = &Manifest{
		Kind:     "loadout",
		Version:  1,
		Provider: "claude-code",
		Name:     "test-loadout",
		Rules:    []string{"my-rule"},
		Hooks:    []string{"my-hook"},
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

	_, err := Apply(manifest, cat, prov, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
		Rules:    []string{"nonexistent-rule"},
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
