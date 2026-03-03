package installer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
	"github.com/tidwall/gjson"
)

func TestInstallHook_RecordsInInstalledJSON(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	// Create hook file
	hookDir := filepath.Join(projectRoot, "hooks", "test-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{
  "event": "PreToolUse",
  "matcher": ".*",
  "hooks": [{"type": "command", "command": "echo lint"}]
}`
	hookFile := filepath.Join(hookDir, "hook.json")
	os.WriteFile(hookFile, []byte(hookJSON), 0644)

	item := catalog.ContentItem{
		Name: "test-hook",
		Type: catalog.Hooks,
		Path: hookFile, // installHook expects item.Path = hook file path
	}

	// The installHook function uses os.UserHomeDir for settings path,
	// which we can't easily override. So we test the installed.json
	// interaction through the public API: LoadInstalled + SaveInstalled.

	// Simulate what installHook does for tracking:
	inst, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}

	// Before install: should not find the hook
	if inst.FindHook("test-hook", "PreToolUse") >= 0 {
		t.Error("hook should not exist before install")
	}

	// Record as installHook would
	inst.Hooks = append(inst.Hooks, InstalledHook{
		Name:    item.Name,
		Event:   "PreToolUse",
		Command: "echo lint",
		Source:  "export",
	})
	if err := SaveInstalled(projectRoot, inst); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}

	// Reload and verify
	inst2, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled (reload): %v", err)
	}
	idx := inst2.FindHook("test-hook", "PreToolUse")
	if idx < 0 {
		t.Fatal("hook not found after install")
	}
	if inst2.Hooks[idx].Command != "echo lint" {
		t.Errorf("command: got %q, want %q", inst2.Hooks[idx].Command, "echo lint")
	}
	if inst2.Hooks[idx].Source != "export" {
		t.Errorf("source: got %q, want %q", inst2.Hooks[idx].Source, "export")
	}
}

func TestUninstallHook_RemovesFromInstalledJSON(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	// Pre-populate installed.json with a hook
	inst := &Installed{
		Hooks: []InstalledHook{
			{Name: "remove-me", Event: "PostToolUse", Command: "echo bye", Source: "export"},
			{Name: "keep-me", Event: "PreToolUse", Command: "echo stay", Source: "export"},
		},
	}
	if err := SaveInstalled(projectRoot, inst); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}

	// Simulate uninstallHook's installed.json part
	inst, _ = LoadInstalled(projectRoot)
	idx := inst.FindHook("remove-me", "PostToolUse")
	if idx < 0 {
		t.Fatal("expected to find hook")
	}
	inst.RemoveHook(idx)
	SaveInstalled(projectRoot, inst)

	// Verify
	inst, _ = LoadInstalled(projectRoot)
	if inst.FindHook("remove-me", "PostToolUse") >= 0 {
		t.Error("hook should have been removed")
	}
	if inst.FindHook("keep-me", "PreToolUse") < 0 {
		t.Error("other hook should still exist")
	}
}

func TestCheckHookStatus_UsesInstalledJSON(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	// Create hook file for parsing
	hookDir := filepath.Join(projectRoot, "hooks", "status-hook")
	os.MkdirAll(hookDir, 0755)
	hookJSON := `{
  "event": "PostToolUse",
  "matcher": ".*",
  "hooks": [{"type": "command", "command": "echo status"}]
}`
	hookFile := filepath.Join(hookDir, "hook.json")
	os.WriteFile(hookFile, []byte(hookJSON), 0644)

	item := catalog.ContentItem{
		Name: "status-hook",
		Type: catalog.Hooks,
		Path: hookFile,
	}

	// Create a settings.json at the provider's config path
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("getting home dir: %v", err)
	}
	configDir := filepath.Join(home, ".syllago-test-hookstatus-"+filepath.Base(projectRoot))
	os.MkdirAll(configDir, 0755)
	t.Cleanup(func() { os.RemoveAll(configDir) })
	os.WriteFile(filepath.Join(configDir, "settings.json"), []byte("{}"), 0644)

	prov := provider.Provider{
		Name:      "test-provider",
		Slug:      "test",
		ConfigDir: filepath.Base(configDir),
	}

	// Without installed.json entry: should be NotInstalled
	status := checkHookStatus(item, prov, projectRoot)
	if status != StatusNotInstalled {
		t.Errorf("expected NotInstalled without installed.json entry, got %v", status)
	}

	// Add to installed.json
	inst := &Installed{
		Hooks: []InstalledHook{
			{Name: "status-hook", Event: "PostToolUse", Command: "echo status", Source: "export"},
		},
	}
	SaveInstalled(projectRoot, inst)

	// With installed.json entry: should be Installed
	status = checkHookStatus(item, prov, projectRoot)
	if status != StatusInstalled {
		t.Errorf("expected Installed with installed.json entry, got %v", status)
	}
}

func TestParseHookFile_Valid(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	hookJSON := `{
  "event": "PostToolUse",
  "matcher": "*.go",
  "hooks": [{"type": "command", "command": "go test"}]
}`
	hookFile := filepath.Join(tmpDir, "hook.json")
	os.WriteFile(hookFile, []byte(hookJSON), 0644)

	event, matcherGroup, err := parseHookFile(hookFile)
	if err != nil {
		t.Fatalf("parseHookFile: %v", err)
	}
	if event != "PostToolUse" {
		t.Errorf("event: got %q, want PostToolUse", event)
	}

	// The event field should be stripped from matcherGroup
	if gjson.GetBytes(matcherGroup, "event").Exists() {
		t.Error("event field should be stripped from matcher group")
	}
	// Other fields should remain
	if gjson.GetBytes(matcherGroup, "matcher").String() != "*.go" {
		t.Error("matcher field should be preserved")
	}
	if !gjson.GetBytes(matcherGroup, "hooks").IsArray() {
		t.Error("hooks array should be preserved")
	}
}

func TestParseHookFile_DirectoryFormat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	hookJSON := `{"event":"PreToolUse","matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}`
	os.WriteFile(filepath.Join(dir, "hook.json"), []byte(hookJSON), 0644)

	event, matcherGroup, err := parseHookFile(dir)
	if err != nil {
		t.Fatalf("parseHookFile with directory: %v", err)
	}
	if event != "PreToolUse" {
		t.Errorf("event: got %q, want PreToolUse", event)
	}
	if gjson.GetBytes(matcherGroup, "event").Exists() {
		t.Error("event field should be stripped from matcher group")
	}
	if gjson.GetBytes(matcherGroup, "matcher").String() != "Bash" {
		t.Error("matcher field should be preserved")
	}
}

func TestParseHookFile_MissingEvent(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	hookJSON := `{"matcher": ".*", "hooks": [{"type": "command", "command": "echo"}]}`
	hookFile := filepath.Join(tmpDir, "hook.json")
	os.WriteFile(hookFile, []byte(hookJSON), 0644)

	_, _, err := parseHookFile(hookFile)
	if err == nil {
		t.Fatal("expected error for missing event field")
	}
}

func TestComputeGroupHash_Deterministic(t *testing.T) {
	t.Parallel()
	data := []byte(`{"matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}`)
	h1 := computeGroupHash(data)
	h2 := computeGroupHash(data)
	if h1 != h2 {
		t.Error("hash should be deterministic")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64 char hex, got %d", len(h1))
	}
}

func TestComputeGroupHash_DifferentContent(t *testing.T) {
	t.Parallel()
	d1 := []byte(`{"hooks":[{"type":"command","command":"echo a"}]}`)
	d2 := []byte(`{"hooks":[{"type":"command","command":"echo b"}]}`)
	if computeGroupHash(d1) == computeGroupHash(d2) {
		t.Error("different content should have different hash")
	}
}

func TestInstallHook_HashComputation(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	hookJSON := `{"event":"PreToolUse","matcher":"Bash","hooks":[{"type":"command","command":"echo hash-test"}]}`

	// Simulate what installHook does: parse, compute hash, record
	hookFile := filepath.Join(projectRoot, "hook.json")
	os.WriteFile(hookFile, []byte(hookJSON), 0644)

	_, matcherGroup, err := parseHookFile(hookFile)
	if err != nil {
		t.Fatalf("parseHookFile: %v", err)
	}

	groupHash := computeGroupHash(matcherGroup)
	if len(groupHash) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars", len(groupHash))
	}

	inst := &Installed{}
	inst.Hooks = append(inst.Hooks, InstalledHook{
		Name:      "hash-test-hook",
		Event:     "PreToolUse",
		GroupHash: groupHash,
		Command:   "echo hash-test",
		Source:    "export",
		Scope:     "global",
	})
	if err := SaveInstalled(projectRoot, inst); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}

	// Reload and verify GroupHash is persisted
	inst2, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	idx := inst2.FindHook("hash-test-hook", "PreToolUse")
	if idx < 0 {
		t.Fatal("hook not found after save")
	}
	if inst2.Hooks[idx].GroupHash != groupHash {
		t.Errorf("GroupHash: got %q, want %q", inst2.Hooks[idx].GroupHash, groupHash)
	}
	if inst2.Hooks[idx].Scope != "global" {
		t.Errorf("Scope: got %q, want %q", inst2.Hooks[idx].Scope, "global")
	}
}

func TestUninstallHook_HashMatching(t *testing.T) {
	t.Parallel()
	projectRoot := t.TempDir()
	os.MkdirAll(filepath.Join(projectRoot, ".syllago"), 0755)

	// The matcher group JSON as it would appear in settings.json
	entryJSON := `{"matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}`
	groupHash := computeGroupHash([]byte(entryJSON))

	// Create installed.json with the stored hash
	inst := &Installed{
		Hooks: []InstalledHook{
			{
				Name:      "hash-match-hook",
				Event:     "PreToolUse",
				GroupHash: groupHash,
				Command:   "echo hi",
				Source:    "export",
				Scope:     "global",
			},
		},
	}
	if err := SaveInstalled(projectRoot, inst); err != nil {
		t.Fatalf("SaveInstalled: %v", err)
	}

	// Reload and verify we can find the hook and its hash matches the entry
	inst2, err := LoadInstalled(projectRoot)
	if err != nil {
		t.Fatalf("LoadInstalled: %v", err)
	}
	idx := inst2.FindHook("hash-match-hook", "PreToolUse")
	if idx < 0 {
		t.Fatal("hook not found in installed.json")
	}

	storedHash := inst2.Hooks[idx].GroupHash
	recomputedHash := computeGroupHash([]byte(entryJSON))
	if storedHash != recomputedHash {
		t.Errorf("hash mismatch: stored %q, recomputed %q", storedHash, recomputedHash)
	}

	// Verify a different entry does NOT match
	differentEntry := `{"matcher":"Bash","hooks":[{"type":"command","command":"echo bye"}]}`
	differentHash := computeGroupHash([]byte(differentEntry))
	if storedHash == differentHash {
		t.Error("different entry should produce a different hash")
	}
}
