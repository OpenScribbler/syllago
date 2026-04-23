package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
)

// TestWriteHookToLibrary_WritesManifestHookJSON is a regression test for the bug
// where TUI-added hooks wrote the whole settings.json file as hook.json.
// Verifies the resulting hook.json is the canonical hooks/0.1 Manifest — a
// single-handler {spec, hooks[]} document — matching what installHook's
// parseHookFile expects.
func TestWriteHookToLibrary_WritesManifestHookJSON(t *testing.T) {
	contentRoot := t.TempDir()

	hook := converter.HookData{
		Event:   "before_tool_execute",
		Matcher: "Edit",
		Hooks: []converter.HookEntry{
			{Type: "command", Command: "echo lint"},
		},
	}

	item := addDiscoveryItem{
		name:          "my-hook",
		itemType:      catalog.Hooks,
		scope:         "global",
		hookData:      &hook,
		hookSourceDir: "",
	}

	result := writeHookToLibrary(item, contentRoot, "", "", "claude-code")
	if result.status != "added" {
		t.Fatalf("expected status=added, got %q err=%v", result.status, result.err)
	}

	hookPath := filepath.Join(contentRoot, string(catalog.Hooks), "claude-code", "my-hook", "hook.json")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("reading hook.json: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("hook.json must be valid JSON: %v", err)
	}

	// Manifest shape: top-level "spec" + "hooks" array.
	if parsed["spec"] != converter.SpecVersion {
		t.Errorf("hook.json missing/wrong spec: got %v want %q", parsed["spec"], converter.SpecVersion)
	}
	hooksArr, ok := parsed["hooks"].([]any)
	if !ok {
		t.Fatalf("hook.json hooks field must be array, got %T", parsed["hooks"])
	}
	if len(hooksArr) != 1 {
		t.Fatalf("manifest must contain exactly one hook entry, got %d", len(hooksArr))
	}
	first, ok := hooksArr[0].(map[string]any)
	if !ok {
		t.Fatalf("hooks[0] must be object, got %T", hooksArr[0])
	}
	if first["event"] != "before_tool_execute" {
		t.Errorf("hooks[0].event: got %v want before_tool_execute", first["event"])
	}
	if first["matcher"] != "Edit" {
		t.Errorf("hooks[0].matcher: got %v want Edit", first["matcher"])
	}
	handler, ok := first["handler"].(map[string]any)
	if !ok {
		t.Fatalf("hooks[0].handler must be object, got %T", first["handler"])
	}
	if handler["type"] != "command" || handler["command"] != "echo lint" {
		t.Errorf("handler mismatch: got %v", handler)
	}

	// Metadata should exist.
	metaPath := filepath.Join(contentRoot, string(catalog.Hooks), "claude-code", "my-hook", ".syllago.yaml")
	if _, err := os.Stat(metaPath); err != nil {
		t.Errorf(".syllago.yaml should exist: %v", err)
	}
}

// TestWriteHookToLibrary_SkipsExisting verifies the no-overwrite path.
func TestWriteHookToLibrary_SkipsExisting(t *testing.T) {
	contentRoot := t.TempDir()

	existing := filepath.Join(contentRoot, string(catalog.Hooks), "claude-code", "my-hook")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}

	hook := converter.HookData{
		Event: "before_tool_execute",
		Hooks: []converter.HookEntry{{Type: "command", Command: "echo hi"}},
	}
	item := addDiscoveryItem{
		name:      "my-hook",
		itemType:  catalog.Hooks,
		hookData:  &hook,
		overwrite: false,
	}

	result := writeHookToLibrary(item, contentRoot, "", "", "claude-code")
	if result.status != "skipped" {
		t.Errorf("expected skipped for existing dir without overwrite, got %q", result.status)
	}
}
