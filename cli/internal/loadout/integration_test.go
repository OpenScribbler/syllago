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

// TestApply_MultiItemRollback_HookFailureRevertsSymlinks exercises the full
// rollback path: a multi-item loadout where one item fails mid-apply must leave
// the filesystem in exactly the pre-apply state — no orphaned symlinks, no
// partial JSON merges, no leftover snapshot.
//
// This is the regression test for the audit finding that integration_test.go:398
// contained a hollow `t.Logf` about rollback ("may indicate rollback") with no
// actual assertion.
//
// Scenario: 2 rules (symlinks) + 1 hook with an empty "event" field. The rules
// produce "create-symlink" actions; the hook produces "merge-hook". Hook merge
// fails in applyHook (event == "" check). Because map iteration over
// RefsByType is non-deterministic, the rules may run before OR after the hook —
// but the rollback must unconditionally remove all planned symlinks via the
// symlinkRecords loop, so the post-state is the same regardless.
func TestApply_MultiItemRollback_HookFailureRevertsSymlinks(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, manifest, cat, prov := setupIntegrationEnv(t)

	// Add a second rule so we have two symlinks that must all be rolled back
	ruleBDir := filepath.Join(projectRoot, "content", "rules", "claude-code", "int-rule-b")
	os.MkdirAll(ruleBDir, 0755)
	os.WriteFile(filepath.Join(ruleBDir, "rule.md"), []byte("# Rule B"), 0644)
	cat.Items = append(cat.Items, catalog.ContentItem{
		Name: "int-rule-b", Type: catalog.Rules, Provider: "claude-code", Path: ruleBDir,
	})
	manifest.Rules = append(manifest.Rules, ItemRef{Name: "int-rule-b"})

	// Replace the good hook with a broken one (missing event field triggers
	// applyHook's "hook file missing 'event' field" error).
	brokenHookDir := filepath.Join(projectRoot, "content", "hooks", "claude-code", "int-hook")
	os.WriteFile(filepath.Join(brokenHookDir, "hook.json"), []byte(`{"matcher":".*","hooks":[{"type":"command","command":"echo oops"}]}`), 0644)

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	originalSettings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading original settings: %v", err)
	}

	opts := ApplyOptions{
		Mode:        "keep",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	_, err = Apply(manifest, cat, prov, opts)
	if err == nil {
		t.Fatal("expected Apply to fail on broken hook, got nil")
	}
	// The rollback message prefix is part of the contract — callers match on it.
	if !containsAll(err.Error(), "rolled back", "event") {
		t.Errorf("error should mention rollback and the underlying cause; got: %v", err)
	}

	// Both rule symlinks must be absent. If either is present, rollback failed
	// to clean up a partially-applied item.
	for _, name := range []string{"int-rule", "int-rule-b"} {
		symlinkPath := filepath.Join(homeDir, ".claude", "rules", name)
		if _, statErr := os.Lstat(symlinkPath); !os.IsNotExist(statErr) {
			t.Errorf("symlink %s should not exist after rollback; got err=%v", name, statErr)
		}
	}

	// settings.json must be byte-exact identical to pre-apply. If the broken
	// hook ran before any successful merge this is trivially true, but if
	// rollback ran after any non-hook item succeeded this proves snapshot
	// restore + symlink cleanup worked.
	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json after rollback: %v", err)
	}
	if string(got) != string(originalSettings) {
		t.Errorf("settings.json mutated after rollback\ngot:  %s\nwant: %s", got, originalSettings)
	}

	// installed.json must not exist (applyActions never reached SaveInstalled)
	// or must be empty.
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("loading installed.json: %v", err)
	}
	if len(inst.Symlinks) != 0 || len(inst.Hooks) != 0 || len(inst.MCP) != 0 {
		t.Errorf("installed.json should be empty after rollback; got symlinks=%d hooks=%d mcp=%d",
			len(inst.Symlinks), len(inst.Hooks), len(inst.MCP))
	}

	// The snapshot created pre-apply must have been deleted by the rollback.
	if _, _, loadErr := snapshot.Load(projectRoot); !errors.Is(loadErr, snapshot.ErrNoSnapshot) {
		t.Errorf("snapshot should be gone after rollback; got: %v", loadErr)
	}
}

// TestApply_PartialHookMergeRollback_RestoresSettingsJson is the deterministic
// "partial merge is reverted" test. Unlike the previous test, this one
// guarantees the first hook WAS applied before the second one fails — proving
// rollback restores settings.json from snapshot rather than just leaving
// whatever the first hook merged in place.
//
// Two hooks of the same type iterate in slice order (manifest.Hooks preserves
// order), so we can reliably arrange "good hook succeeds, next hook fails."
func TestApply_PartialHookMergeRollback_RestoresSettingsJson(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, _, cat, prov := setupIntegrationEnv(t)

	// Keep int-hook (the pre-built good hook). Add a second hook whose
	// directory has NO json files — findHookFile returns "" and applyHook
	// fails with "no hook JSON file found in ...".
	brokenHookDir := filepath.Join(projectRoot, "content", "hooks", "claude-code", "int-hook-broken")
	os.MkdirAll(brokenHookDir, 0755)
	// Intentionally write a non-json file so the dir exists but findHookFile
	// cannot locate a hook source.
	os.WriteFile(filepath.Join(brokenHookDir, "README.md"), []byte("no hook here"), 0644)

	cat.Items = append(cat.Items, catalog.ContentItem{
		Name: "int-hook-broken", Type: catalog.Hooks, Provider: "claude-code", Path: brokenHookDir,
	})

	// Hooks-only manifest so slice order is deterministic. No rules = no
	// cross-type map randomness; good hook MUST run before broken hook.
	manifest := &Manifest{
		Kind:     "loadout",
		Version:  1,
		Provider: "claude-code",
		Name:     "integration-test",
		Hooks: []ItemRef{
			{Name: "int-hook"},        // merges PostToolUse into settings.json
			{Name: "int-hook-broken"}, // fails, triggers rollback
		},
	}

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	originalSettings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading original settings: %v", err)
	}

	opts := ApplyOptions{
		Mode:        "keep",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	_, err = Apply(manifest, cat, prov, opts)
	if err == nil {
		t.Fatal("expected Apply to fail on hook with no JSON file, got nil")
	}
	if !containsAll(err.Error(), "rolled back") {
		t.Errorf("error should mention rollback; got: %v", err)
	}

	// Core assertion: settings.json must be byte-exact pre-apply content.
	// The good hook's PostToolUse entry was merged before the second hook
	// failed — if rollback didn't call snapshot.Restore, that merged entry
	// would still be visible here.
	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json after rollback: %v", err)
	}
	if string(got) != string(originalSettings) {
		t.Errorf("settings.json not restored to pre-apply state\ngot:  %s\nwant: %s", got, originalSettings)
	}

	// Belt-and-suspenders: confirm the loadout's PostToolUse hook is NOT in
	// the post-rollback settings. If this fails but the byte-compare above
	// passed, the fixture is broken — investigate, don't just update.
	postApply := gjson.GetBytes(got, "hooks.PostToolUse")
	if postApply.Exists() {
		t.Errorf("hooks.PostToolUse should not exist after rollback; got: %s", postApply.Raw)
	}

	// installed.json must show no hook entries (SaveInstalled never ran).
	inst, err := installer.LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("loading installed.json: %v", err)
	}
	if len(inst.Hooks) != 0 {
		t.Errorf("installed.json should have no hooks after rollback; got %d", len(inst.Hooks))
	}

	if _, _, loadErr := snapshot.Load(projectRoot); !errors.Is(loadErr, snapshot.ErrNoSnapshot) {
		t.Errorf("snapshot should be gone after rollback; got: %v", loadErr)
	}
}

// TestApply_MidBundleFailure_LaterItemsNeverApplied proves that when the Nth
// item in a deterministic sequence fails, items N+1..end never mutate state.
// This is the third rollback property: early items are reverted, later items
// never applied.
//
// Sequence: 3 hooks where hooks[0] and hooks[2] merge a unique matcher string,
// and hooks[1] fails (missing event field). After Apply fails, neither the
// hooks[0] marker nor the hooks[2] marker can be present in settings.json —
// the former because rollback restored, the latter because it never ran.
func TestApply_MidBundleFailure_LaterItemsNeverApplied(t *testing.T) {
	t.Parallel()
	homeDir, projectRoot, _, cat, prov := setupIntegrationEnv(t)

	writeHook := func(name, contents string) {
		dir := filepath.Join(projectRoot, "content", "hooks", "claude-code", name)
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "hook.json"), []byte(contents), 0644)
		cat.Items = append(cat.Items, catalog.ContentItem{
			Name: name, Type: catalog.Hooks, Provider: "claude-code", Path: dir,
		})
	}

	// hooks[0]: good hook with a marker matcher we can grep for
	writeHook("int-hook-first", `{
  "event": "PostToolUse",
  "matcher": "FIRST_MARKER",
  "hooks": [{"type": "command", "command": "echo first"}]
}`)
	// hooks[1]: broken hook, missing event field — triggers mid-bundle failure
	writeHook("int-hook-broken", `{
  "matcher": "SHOULD_NEVER_APPEAR",
  "hooks": [{"type": "command", "command": "echo broken"}]
}`)
	// hooks[2]: good hook that MUST never be applied because hooks[1] aborts
	// the loop first. Distinct marker so we can assert its absence.
	writeHook("int-hook-third", `{
  "event": "PreToolUse",
  "matcher": "THIRD_MARKER",
  "hooks": [{"type": "command", "command": "echo third"}]
}`)

	manifest := &Manifest{
		Kind:     "loadout",
		Version:  1,
		Provider: "claude-code",
		Name:     "integration-test",
		Hooks: []ItemRef{
			{Name: "int-hook-first"},
			{Name: "int-hook-broken"},
			{Name: "int-hook-third"},
		},
	}

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")
	originalSettings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading original settings: %v", err)
	}

	opts := ApplyOptions{
		Mode:        "keep",
		ProjectRoot: projectRoot,
		HomeDir:     homeDir,
		RepoRoot:    projectRoot,
	}

	_, err = Apply(manifest, cat, prov, opts)
	if err == nil {
		t.Fatal("expected Apply to fail on mid-bundle broken hook, got nil")
	}
	// Guard against a false-pass where Resolve/Validate fails before the
	// apply loop runs — those failures don't exercise rollback at all.
	if !containsAll(err.Error(), "rolled back") {
		t.Fatalf("expected rollback path to run; got: %v", err)
	}

	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("reading settings.json: %v", err)
	}

	// Pre-apply state must be restored.
	if string(got) != string(originalSettings) {
		t.Errorf("settings.json not restored after mid-bundle failure\ngot:  %s\nwant: %s", got, originalSettings)
	}

	// Explicit marker checks — both markers must be absent. FIRST_MARKER would
	// indicate rollback failed to restore; THIRD_MARKER would indicate the
	// loop kept going past the failure.
	gotStr := string(got)
	if containsString(gotStr, "FIRST_MARKER") {
		t.Error("FIRST_MARKER present in settings.json — rollback did not restore pre-apply content")
	}
	if containsString(gotStr, "THIRD_MARKER") {
		t.Error("THIRD_MARKER present in settings.json — items after the failure kept being applied")
	}

	if _, _, loadErr := snapshot.Load(projectRoot); !errors.Is(loadErr, snapshot.ErrNoSnapshot) {
		t.Errorf("snapshot should be gone after rollback; got: %v", loadErr)
	}
}

// containsAll reports whether s contains every substring in subs.
// Local helper kept small to avoid importing strings for a single use.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !containsString(s, sub) {
			return false
		}
	}
	return true
}

func containsString(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
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

	// Verify no snapshot was left behind. The current implementation catches
	// conflicts before snapshot creation, so Load must return ErrNoSnapshot.
	// If a snapshot is present, rollback either failed to clean it up or the
	// conflict-detection path moved after snapshot creation — both are bugs.
	_, _, loadErr := snapshot.Load(projectRoot)
	if !errors.Is(loadErr, snapshot.ErrNoSnapshot) {
		t.Errorf("snapshot should not exist after conflict abort (either conflict-detect order regressed, or rollback failed to clean up); got: %v", loadErr)
	}
}
