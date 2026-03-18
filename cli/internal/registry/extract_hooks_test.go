package registry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestScanUserHooks_Basic(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	settings := `{
		"hooks": {
			"PostToolUse": [
				{"matcher": "Edit|Write", "hooks": [{"type": "command", "command": "echo lint"}]},
				{"hooks": [{"type": "command", "command": "echo log"}]}
			],
			"PreToolUse": [
				{"matcher": "Bash", "hooks": [{"type": "command", "command": "./scripts/check.sh"}]}
			]
		}
	}`
	os.WriteFile(settingsPath, []byte(settings), 0644)

	hooks, err := ScanUserHooks(settingsPath, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(hooks) != 3 {
		t.Fatalf("expected 3 hooks, got %d", len(hooks))
	}

	// Check events are correct
	events := map[string]int{}
	for _, h := range hooks {
		events[h.Event]++
	}
	if events["PostToolUse"] != 2 || events["PreToolUse"] != 1 {
		t.Errorf("events: %v", events)
	}
}

func TestScanUserHooks_NoHooks(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	os.WriteFile(settingsPath, []byte(`{"permissions": {}}`), 0644)

	hooks, err := ScanUserHooks(settingsPath, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(hooks) != 0 {
		t.Errorf("expected 0 hooks, got %d", len(hooks))
	}
}

func TestScanUserHooks_ScriptDetection(t *testing.T) {
	dir := t.TempDir()
	// Create an in-repo script
	scriptDir := filepath.Join(dir, "scripts")
	os.MkdirAll(scriptDir, 0755)
	os.WriteFile(filepath.Join(scriptDir, "check.sh"), []byte("#!/bin/bash"), 0755)

	settingsPath := filepath.Join(dir, "settings.json")
	settings := `{
		"hooks": {
			"PreToolUse": [
				{"hooks": [{"type": "command", "command": "./scripts/check.sh"}]}
			]
		}
	}`
	os.WriteFile(settingsPath, []byte(settings), 0644)

	hooks, err := ScanUserHooks(settingsPath, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks))
	}
	if !hooks[0].ScriptInRepo {
		t.Error("expected ScriptInRepo=true for ./scripts/check.sh")
	}
}

func TestExtractHooksToDir(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "hooks")

	hooks := []UserScopedHook{
		{
			Name:       "posttooluse-edit-write",
			Event:      "PostToolUse",
			Index:      0,
			Definition: json.RawMessage(`{"matcher": "Edit|Write", "hooks": [{"type": "command", "command": "echo lint"}]}`),
			Command:    "echo lint",
		},
	}

	if err := ExtractHooksToDir(hooks, targetDir); err != nil {
		t.Fatal(err)
	}

	// Verify hook.json was created
	hookPath := filepath.Join(targetDir, "posttooluse-edit-write", "hook.json")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("reading hook.json: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parsing hook.json: %v", err)
	}

	if parsed["event"] != "PostToolUse" {
		t.Errorf("event: got %v", parsed["event"])
	}
	if parsed["matcher"] != "Edit|Write" {
		t.Errorf("matcher: got %v", parsed["matcher"])
	}
}
