package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/add"
	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/converter"
)

// TestBuildHookPreviewFiles_NoScripts verifies that a hook without any
// external script references surfaces only hook.json in the drill-in tree.
func TestBuildHookPreviewFiles_NoScripts(t *testing.T) {
	item := addDiscoveryItem{
		name:     "inline-hook",
		itemType: catalog.Hooks,
		hookData: &converter.HookData{
			Event: "before_tool_execute",
			Hooks: []converter.HookEntry{
				{Type: "command", Command: "echo hello"},
			},
		},
	}
	files := buildHookPreviewFiles(item)
	if len(files) != 1 || files[0] != "hook.json" {
		t.Fatalf("expected [hook.json], got %v", files)
	}
}

// TestBuildHookPreviewFiles_IncludesReferencedScripts verifies that scripts
// referenced by a hook's command and existing on disk appear alongside
// hook.json in the drill-in tree.
func TestBuildHookPreviewFiles_IncludesReferencedScripts(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "lint.sh"), []byte("#!/bin/bash\necho lint\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	item := addDiscoveryItem{
		name:     "script-hook",
		itemType: catalog.Hooks,
		hookData: &converter.HookData{
			Event: "before_tool_execute",
			Hooks: []converter.HookEntry{
				{Type: "command", Command: "bash ./lint.sh"},
			},
		},
		hookSourceDir: srcDir,
	}

	files := buildHookPreviewFiles(item)
	if len(files) != 2 {
		t.Fatalf("expected 2 files (hook.json + lint.sh), got %v", files)
	}
	if files[0] != "hook.json" {
		t.Errorf("expected hook.json first, got %q", files[0])
	}
	if files[1] != "lint.sh" {
		t.Errorf("expected lint.sh second, got %q", files[1])
	}
}

// TestBuildHookPreviewFiles_SkipsMissingScripts verifies that script refs
// that don't resolve to an existing file are silently dropped (rather than
// surfacing a tree entry that produces read errors in the preview pane).
func TestBuildHookPreviewFiles_SkipsMissingScripts(t *testing.T) {
	srcDir := t.TempDir()
	item := addDiscoveryItem{
		name:     "ghost-hook",
		itemType: catalog.Hooks,
		hookData: &converter.HookData{
			Event: "before_tool_execute",
			Hooks: []converter.HookEntry{
				{Type: "command", Command: "bash ./does-not-exist.sh"},
			},
		},
		hookSourceDir: srcDir,
	}
	files := buildHookPreviewFiles(item)
	if len(files) != 1 {
		t.Fatalf("expected only hook.json (missing script dropped), got %v", files)
	}
}

// TestBuildHookPreviewFiles_DeduplicatesScripts verifies that when multiple
// hook entries reference the same script, it appears only once in the tree.
func TestBuildHookPreviewFiles_DeduplicatesScripts(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "shared.sh"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	item := addDiscoveryItem{
		name:     "dup-hook",
		itemType: catalog.Hooks,
		hookData: &converter.HookData{
			Event: "before_tool_execute",
			Hooks: []converter.HookEntry{
				{Type: "command", Command: "bash ./shared.sh --a"},
				{Type: "command", Command: "bash ./shared.sh --b"},
			},
		},
		hookSourceDir: srcDir,
	}
	files := buildHookPreviewFiles(item)
	if len(files) != 2 {
		t.Fatalf("expected hook.json + single shared.sh, got %v", files)
	}
}

// TestReadHookPreviewContent_HookJSON verifies that requesting "hook.json"
// returns pretty-printed canonical Manifest JSON (hooks/0.1) — NOT a JSON
// fragment with "// Event:" comments and NOT the raw settings.json.
func TestReadHookPreviewContent_HookJSON(t *testing.T) {
	item := addDiscoveryItem{
		name:     "canonical-hook",
		itemType: catalog.Hooks,
		hookData: &converter.HookData{
			Event:   "before_tool_execute",
			Matcher: "Edit",
			Hooks: []converter.HookEntry{
				{Type: "command", Command: "echo go"},
			},
		},
	}

	content, err := readHookPreviewContent(item, "hook.json")
	if err != nil {
		t.Fatalf("readHookPreviewContent: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("preview is not valid JSON: %v\ncontent:\n%s", err, content)
	}
	if parsed["spec"] != converter.SpecVersion {
		t.Errorf("expected spec=%q, got %v", converter.SpecVersion, parsed["spec"])
	}
	hooksArr, ok := parsed["hooks"].([]any)
	if !ok {
		t.Fatalf("expected Manifest.hooks array, got %T — preview must be hooks/0.1 Manifest, not settings.json shape", parsed["hooks"])
	}
	if len(hooksArr) != 1 {
		t.Fatalf("expected exactly one hook entry, got %d", len(hooksArr))
	}
	first, _ := hooksArr[0].(map[string]any)
	if first["event"] != "before_tool_execute" {
		t.Errorf("expected hooks[0].event=before_tool_execute, got %v", first["event"])
	}
	if first["matcher"] != "Edit" {
		t.Errorf("expected hooks[0].matcher=Edit, got %v", first["matcher"])
	}
}

// TestReadHookPreviewContent_Script verifies that requesting a script
// filename returns the contents of that file from item.hookSourceDir.
func TestReadHookPreviewContent_Script(t *testing.T) {
	srcDir := t.TempDir()
	body := "#!/bin/bash\necho lint-script-body\n"
	if err := os.WriteFile(filepath.Join(srcDir, "lint.sh"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	item := addDiscoveryItem{
		name:     "linter",
		itemType: catalog.Hooks,
		hookData: &converter.HookData{
			Event: "before_tool_execute",
			Hooks: []converter.HookEntry{
				{Type: "command", Command: "bash ./lint.sh"},
			},
		},
		hookSourceDir: srcDir,
	}
	content, err := readHookPreviewContent(item, "lint.sh")
	if err != nil {
		t.Fatalf("readHookPreviewContent(lint.sh): %v", err)
	}
	if content != body {
		t.Errorf("unexpected body:\ngot  %q\nwant %q", content, body)
	}
}

// TestBuildHookPreviewFiles_ExpandsEnvVars verifies that hooks using env-var
// prefixed script paths ($PAI_DIR, ${PAI_DIR}) are expanded via os.ExpandEnv
// so the referenced script still shows up in the drill-in tree.
func TestBuildHookPreviewFiles_ExpandsEnvVars(t *testing.T) {
	scriptDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(scriptDir, "lint.sh"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PAI_DIR_TEST", scriptDir)

	item := addDiscoveryItem{
		name:     "env-hook",
		itemType: catalog.Hooks,
		hookData: &converter.HookData{
			Event: "before_tool_execute",
			Hooks: []converter.HookEntry{
				{Type: "command", Command: "bash $PAI_DIR_TEST/lint.sh"},
			},
		},
		// hookSourceDir deliberately empty — the env-var yields an absolute path.
	}
	files := buildHookPreviewFiles(item)
	if len(files) != 2 || files[1] != "lint.sh" {
		t.Fatalf("expected [hook.json, lint.sh], got %v", files)
	}
}

// TestReadHookPreviewContent_ExpandsEnvVars verifies the preview reads the
// file at the env-var-expanded path, not at a literal $VAR/... path.
func TestReadHookPreviewContent_ExpandsEnvVars(t *testing.T) {
	scriptDir := t.TempDir()
	body := "#!/bin/bash\necho env-expanded\n"
	if err := os.WriteFile(filepath.Join(scriptDir, "lint.sh"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PAI_DIR_TEST2", scriptDir)

	item := addDiscoveryItem{
		name:     "env-hook",
		itemType: catalog.Hooks,
		hookData: &converter.HookData{
			Event: "before_tool_execute",
			Hooks: []converter.HookEntry{
				{Type: "command", Command: "bash ${PAI_DIR_TEST2}/lint.sh"},
			},
		},
	}
	content, err := readHookPreviewContent(item, "lint.sh")
	if err != nil {
		t.Fatalf("readHookPreviewContent: %v", err)
	}
	if content != body {
		t.Errorf("got %q, want %q", content, body)
	}
}

// TestBuildHookPreviewFiles_UnsetEnvVarSkipped verifies that a $VAR that
// isn't set in the environment doesn't surface a bogus file in the tree.
func TestBuildHookPreviewFiles_UnsetEnvVarSkipped(t *testing.T) {
	// Make sure the var is unset.
	os.Unsetenv("SYLLAGO_TEST_UNSET_VAR")

	item := addDiscoveryItem{
		name:     "ghost-hook",
		itemType: catalog.Hooks,
		hookData: &converter.HookData{
			Event: "before_tool_execute",
			Hooks: []converter.HookEntry{
				{Type: "command", Command: "bash $SYLLAGO_TEST_UNSET_VAR/lint.sh"},
			},
		},
	}
	files := buildHookPreviewFiles(item)
	if len(files) != 1 {
		t.Fatalf("expected only hook.json, got %v", files)
	}
}

// TestReadHookPreviewContent_UnreferencedScriptRejected verifies we don't
// happily read arbitrary files from hookSourceDir — only files that the
// hook actually references through one of its commands.
func TestReadHookPreviewContent_UnreferencedScriptRejected(t *testing.T) {
	srcDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(srcDir, "secret.env"), []byte("TOKEN=1"), 0o600)
	item := addDiscoveryItem{
		name:     "linter",
		itemType: catalog.Hooks,
		hookData: &converter.HookData{
			Event: "before_tool_execute",
			Hooks: []converter.HookEntry{
				{Type: "command", Command: "bash ./lint.sh"},
			},
		},
		hookSourceDir: srcDir,
	}
	if _, err := readHookPreviewContent(item, "secret.env"); err == nil {
		t.Fatal("expected error for unreferenced file, got nil")
	}
}

// TestNativeItemsToDiscovery_HooksSplitPerEntry verifies that when local-path
// discovery surfaces a settings.json containing multiple events/matchers,
// nativeItemsToDiscovery produces ONE addDiscoveryItem per canonical hook —
// each with its own hookData — rather than a single item whose drill-in shows
// the whole settings.json. This is the regression test for the "add wizard
// shows multiple events under one name" bug.
func TestNativeItemsToDiscovery_HooksSplitPerEntry(t *testing.T) {
	baseDir := t.TempDir()
	settingsRel := filepath.Join(".claude", "settings.json")
	settingsAbs := filepath.Join(baseDir, settingsRel)
	if err := os.MkdirAll(filepath.Dir(settingsAbs), 0o755); err != nil {
		t.Fatal(err)
	}

	// Three hooks across three distinct (event, matcher) combinations.
	settings := `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Edit", "hooks": [{"type": "command", "command": "echo pre-edit"}]},
      {"matcher": "Write", "hooks": [{"type": "command", "command": "echo pre-write"}]}
    ],
    "Stop": [
      {"hooks": [{"type": "command", "command": "echo stop"}]}
    ]
  }
}`
	if err := os.WriteFile(settingsAbs, []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate what extractEmbeddedHooks produces: one NativeItem per
	// (event, index-in-array), all sharing the same settings.json path.
	native := []catalog.NativeItem{
		{Name: "pretooluse-0", Path: settingsRel, HookEvent: "PreToolUse", HookIndex: 0},
		{Name: "pretooluse-1", Path: settingsRel, HookEvent: "PreToolUse", HookIndex: 1},
		{Name: "stop-0", Path: settingsRel, HookEvent: "Stop", HookIndex: 0},
	}
	result := catalog.NativeScanResult{
		Providers: []catalog.NativeProviderContent{{
			ProviderSlug: "claude-code",
			ProviderName: "Claude Code",
			Items:        map[string][]catalog.NativeItem{"hooks": native},
		}},
	}
	typeSet := map[catalog.ContentType]bool{catalog.Hooks: true}

	items := nativeItemsToDiscovery(baseDir, result, typeSet, add.LibraryIndex{})

	if len(items) != 3 {
		t.Fatalf("expected 3 discovery items (one per canonical hook), got %d", len(items))
	}

	names := map[string]bool{}
	for i, it := range items {
		if it.itemType != catalog.Hooks {
			t.Errorf("item[%d]: expected type=hooks, got %q", i, it.itemType)
		}
		if it.hookData == nil {
			t.Errorf("item[%d] (%s): hookData must be populated so drill-in can render a flat hook.json", i, it.name)
		}
		if it.hookSourceDir == "" {
			t.Errorf("item[%d] (%s): hookSourceDir must be set so referenced scripts resolve", i, it.name)
		}
		if names[it.name] {
			t.Errorf("duplicate discovery name %q — each hook must get a unique derived name", it.name)
		}
		names[it.name] = true
	}
}
