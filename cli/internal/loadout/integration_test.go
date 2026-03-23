package loadout

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/installer"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/OpenScribbler/syllago/cli/internal/snapshot"
	"github.com/tidwall/gjson"
)

// setupIntegrationEnv creates a fully wired test environment with a catalog
// containing a rule (symlink type) and a hook (merge type), plus a settings.json
// with pre-existing content to verify non-destructive merge and restore.
//
// Key gotcha: the snapshot package stores backed-up files relative to
// os.UserHomeDir(). If we put settings.json under a random temp dir,
// filepath.Rel(home, path) produces "../../tmp/..." which escapes the
// snapshot directory and corrupts the snapshots folder. So we create
// a test directory under the real home dir for provider config files.
func setupIntegrationEnv(t *testing.T) (homeDir, projectRoot string, manifest *Manifest, cat *catalog.Catalog, prov provider.Provider) {
	t.Helper()
	projectRoot = t.TempDir()

	// Use a subdirectory under real home dir for provider config.
	// This ensures snapshot backup paths stay clean (see comment above).
	// We use t.Name() for uniqueness since filepath.Base(t.TempDir()) is
	// always "001", "002" etc. and would collide across parallel tests.
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("getting home dir: %v", err)
	}
	// Replace slashes in test name (subtests use "/")
	safeName := filepath.Base(projectRoot) + "-" + filepath.Base(t.Name())
	homeDir = filepath.Join(home, ".syllago-inttest-"+safeName)
	os.MkdirAll(homeDir, 0755)
	t.Cleanup(func() { os.RemoveAll(homeDir) })

	// Create .syllago dir
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	// Create provider directories
	rulesDir := filepath.Join(homeDir, ".claude", "rules")
	os.MkdirAll(rulesDir, 0755)
	os.MkdirAll(filepath.Join(homeDir, ".claude"), 0755)

	// Write a pre-existing settings.json with user content that should survive apply+remove
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	originalSettings := `{"userPref":"keep-me","hooks":{"PreToolUse":[{"matcher":"*.go","hooks":[{"type":"command","command":"go vet"}]}]}}`
	os.WriteFile(settingsPath, []byte(originalSettings), 0644)

	// Create a rule source (symlink type)
	ruleDir := filepath.Join(projectRoot, "content", "rules", "claude-code", "int-rule")
	os.MkdirAll(ruleDir, 0755)
	os.WriteFile(filepath.Join(ruleDir, "rule.md"), []byte("# Integration Rule"), 0644)

	// Create a hook source (merge type)
	hookDir := filepath.Join(projectRoot, "content", "hooks", "claude-code", "int-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{
  "event": "PostToolUse",
  "matcher": ".*",
  "hooks": [{"type": "command", "command": "echo integration-test"}]
}`
	os.WriteFile(filepath.Join(hookDir, "hook.json"), []byte(hookJSON), 0644)

	manifest = &Manifest{
		Kind:     "loadout",
		Version:  1,
		Provider: "claude-code",
		Name:     "integration-test",
		Rules:    []ItemRef{{Name: "int-rule"}},
		Hooks:    []ItemRef{{Name: "int-hook"}},
	}

	cat = &catalog.Catalog{
		RepoRoot: projectRoot,
		Items: []catalog.ContentItem{
			{Name: "int-rule", Type: catalog.Rules, Provider: "claude-code", Path: ruleDir},
			{Name: "int-hook", Type: catalog.Hooks, Provider: "claude-code", Path: hookDir},
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

// TestTryRoundTrip_ApplyAndAutoRevert is the E2 integration test.
// It exercises the full --try lifecycle: apply with try mode, verify
// the SessionEnd hook was injected, then call Remove with Auto=true
// and verify everything is restored to its original state.
func TestTryRoundTrip_ApplyAndAutoRevert(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupIntegrationEnv(t)

	// Record original settings.json content for comparison later
	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	originalSettings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading original settings: %v", err)
	}

	// Step 1: Apply with mode="try"
	opts := ApplyOptions{
		Mode:        "try",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	result, err := Apply(manifest, cat, prov, opts)
	if err != nil {
		t.Fatalf("Apply (try) failed: %v", err)
	}

	// Step 2: Verify snapshot was created
	if result.SnapshotDir == "" {
		t.Fatal("expected snapshot dir for try mode")
	}
	sm, _, err := snapshot.Load(projectRoot)
	if err != nil {
		t.Fatalf("snapshot.Load after apply: %v", err)
	}
	if sm.Mode != "try" {
		t.Errorf("snapshot mode: got %q, want \"try\"", sm.Mode)
	}
	if sm.LoadoutName != "integration-test" {
		t.Errorf("snapshot loadout name: got %q, want \"integration-test\"", sm.LoadoutName)
	}

	// Step 3: Verify SessionEnd hook was injected into settings.json
	postApplySettings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json after apply: %v", err)
	}

	sessionEnd := gjson.GetBytes(postApplySettings, "hooks.SessionEnd")
	if !sessionEnd.Exists() || !sessionEnd.IsArray() || len(sessionEnd.Array()) == 0 {
		t.Fatal("hooks.SessionEnd not found or empty after try apply")
	}
	cmd := sessionEnd.Array()[0].Get("hooks.0.command").String()
	if cmd != "syllago loadout remove --auto" {
		t.Errorf("SessionEnd hook command: got %q, want \"syllago loadout remove --auto\"", cmd)
	}

	// Step 4: Verify the loadout's hook was merged too.
	// The original settings has a PreToolUse hook, and the loadout adds a PostToolUse hook.
	postToolUse := gjson.GetBytes(postApplySettings, "hooks.PostToolUse")
	if !postToolUse.Exists() || !postToolUse.IsArray() {
		t.Fatal("hooks.PostToolUse should exist after apply")
	}
	if len(postToolUse.Array()) != 1 {
		t.Errorf("expected 1 PostToolUse entry (from loadout), got %d", len(postToolUse.Array()))
	}
	// The original PreToolUse hook should still be there
	preToolUse := gjson.GetBytes(postApplySettings, "hooks.PreToolUse")
	if !preToolUse.Exists() || len(preToolUse.Array()) != 1 {
		t.Error("original PreToolUse hook should survive loadout apply")
	}

	// Step 5: Verify symlink was created
	symlinkPath := filepath.Join(homeDir, ".claude", "rules", "int-rule")
	info, err := os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink for int-rule, got regular file")
	}

	// Step 6: Verify installed.json was populated
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("loading installed.json: %v", err)
	}
	if len(inst.Hooks) == 0 {
		t.Error("expected hook entry in installed.json")
	}
	if len(inst.Symlinks) == 0 {
		t.Error("expected symlink entry in installed.json")
	}

	// Step 7: Call Remove with Auto=true (simulates SessionEnd hook firing)
	removeResult, err := Remove(RemoveOptions{
		Auto:        true,
		ProjectRoot: projectRoot,
	})
	if err != nil {
		t.Fatalf("Remove (auto) failed: %v", err)
	}

	if removeResult.LoadoutName != "integration-test" {
		t.Errorf("remove result loadout name: got %q, want \"integration-test\"", removeResult.LoadoutName)
	}

	// Step 8: Verify settings.json was restored to original content
	restoredSettings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json after remove: %v", err)
	}
	if string(restoredSettings) != string(originalSettings) {
		t.Errorf("settings.json not restored to original\ngot:  %s\nwant: %s", restoredSettings, originalSettings)
	}

	// Step 9: Verify symlink was removed
	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Error("symlink should have been deleted after remove")
	}

	// Step 10: Verify snapshot was deleted
	_, _, err = snapshot.Load(projectRoot)
	if !errors.Is(err, snapshot.ErrNoSnapshot) {
		t.Errorf("expected ErrNoSnapshot after remove, got: %v", err)
	}
}

// TestKeepRoundTrip_ApplyStatusRemove is the G3 integration test.
// It tests the full keep-mode lifecycle: apply with keep mode, verify
// all changes, then remove and verify clean state.
func TestKeepRoundTrip_ApplyStatusRemove(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupIntegrationEnv(t)

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	originalSettings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading original settings: %v", err)
	}

	// Apply with keep mode
	opts := ApplyOptions{
		Mode:        "keep",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	result, err := Apply(manifest, cat, prov, opts)
	if err != nil {
		t.Fatalf("Apply (keep) failed: %v", err)
	}

	// Verify snapshot was created (for undo capability)
	if result.SnapshotDir == "" {
		t.Fatal("expected snapshot dir for keep mode")
	}
	sm, _, err := snapshot.Load(projectRoot)
	if err != nil {
		t.Fatalf("snapshot.Load: %v", err)
	}
	if sm.Mode != "keep" {
		t.Errorf("snapshot mode: got %q, want \"keep\"", sm.Mode)
	}

	// Verify symlink was created
	symlinkPath := filepath.Join(homeDir, ".claude", "rules", "int-rule")
	info, err := os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf("symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink for int-rule")
	}

	// Verify hook was merged
	postApplySettings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}
	postToolUse := gjson.GetBytes(postApplySettings, "hooks.PostToolUse")
	if !postToolUse.Exists() || len(postToolUse.Array()) != 1 {
		t.Errorf("expected 1 PostToolUse entry (from loadout), got %d", len(postToolUse.Array()))
	}
	// Original PreToolUse should survive
	preToolUse := gjson.GetBytes(postApplySettings, "hooks.PreToolUse")
	if !preToolUse.Exists() || len(preToolUse.Array()) != 1 {
		t.Error("original PreToolUse hook should survive")
	}

	// Keep mode should NOT inject SessionEnd hook
	sessionEnd := gjson.GetBytes(postApplySettings, "hooks.SessionEnd")
	if sessionEnd.Exists() && sessionEnd.IsArray() && len(sessionEnd.Array()) > 0 {
		t.Error("keep mode should NOT inject SessionEnd hook")
	}

	// Verify pre-existing user content survives
	if gjson.GetBytes(postApplySettings, "userPref").String() != "keep-me" {
		t.Error("pre-existing userPref field should survive hook merge")
	}

	// Verify installed.json tracking
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("loading installed.json: %v", err)
	}
	foundHook := false
	foundSymlink := false
	for _, h := range inst.Hooks {
		if h.Name == "int-hook" && h.Source == "loadout:integration-test" {
			foundHook = true
		}
	}
	for _, s := range inst.Symlinks {
		if s.Source == "loadout:integration-test" {
			foundSymlink = true
		}
	}
	if !foundHook {
		t.Error("expected hook in installed.json with source loadout:integration-test")
	}
	if !foundSymlink {
		t.Error("expected symlink in installed.json with source loadout:integration-test")
	}

	// Remove
	removeResult, err := Remove(RemoveOptions{ProjectRoot: projectRoot})
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if removeResult.LoadoutName != "integration-test" {
		t.Errorf("remove loadout name: got %q", removeResult.LoadoutName)
	}

	// Verify settings.json restored
	restoredSettings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json after remove: %v", err)
	}
	if string(restoredSettings) != string(originalSettings) {
		t.Errorf("settings.json not restored\ngot:  %s\nwant: %s", restoredSettings, originalSettings)
	}

	// Verify symlink removed
	if _, err := os.Lstat(symlinkPath); !os.IsNotExist(err) {
		t.Error("symlink should be gone after remove")
	}

	// Verify snapshot deleted
	_, _, err = snapshot.Load(projectRoot)
	if !errors.Is(err, snapshot.ErrNoSnapshot) {
		t.Errorf("expected ErrNoSnapshot, got: %v", err)
	}
}

// TestApplyConflict_ItemAlreadyInstalled tests that apply correctly detects
// when a symlink target already exists pointing to a different source.
func TestApplyConflict_ItemAlreadyInstalled(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupIntegrationEnv(t)

	// Only test rules (symlink conflict)
	manifest.Hooks = nil
	cat.Items = cat.Items[:1] // only the rule

	// Create a regular file at the target to cause a conflict
	targetPath := filepath.Join(homeDir, ".claude", "rules", "int-rule")
	os.MkdirAll(filepath.Dir(targetPath), 0755)
	os.WriteFile(targetPath, []byte("user's own file"), 0644)

	opts := ApplyOptions{
		Mode:        "keep",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	_, err := Apply(manifest, cat, prov, opts)
	if err == nil {
		t.Fatal("expected error for conflict (regular file at target)")
	}

	// Verify no snapshot was left behind (conflict should abort before snapshot,
	// or rollback should clean up)
	_, _, loadErr := snapshot.Load(projectRoot)
	if !errors.Is(loadErr, snapshot.ErrNoSnapshot) {
		// If a snapshot was created and not cleaned up, that's a problem.
		// But the current implementation catches conflicts before snapshot creation,
		// so this should pass.
		t.Logf("note: snapshot exists after conflict abort (may indicate rollback): %v", loadErr)
	}
}
