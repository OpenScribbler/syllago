package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

// --- looksLikeScript ---

func TestLooksLikeScript(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		cmd  string
		want bool
	}{
		{"./scripts/lint.sh", true}, // path separator
		{"sub\\path.exe", true},     // backslash
		{"plain.sh", true},          // .sh suffix
		{"helper.py", true},         // .py
		{"build.js", true},          // .js
		{"runner.ts", true},         // .ts
		{"task.rb", true},           // .rb
		{"shell.bash", true},        // .bash
		{"echo hello", false},       // bare command, no slash, no extension
		{"true", false},             // bare command
		{"npx foo", false},          // bare command tokens
		{"", false},                 // empty
	} {
		if got := looksLikeScript(tc.cmd); got != tc.want {
			t.Errorf("looksLikeScript(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

// --- scriptBaseName ---

func TestScriptBaseName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		path string
		want string
	}{
		{"./scripts/lint-check.sh", "lint-check"},
		{"foo/bar/baz.py", "baz"},
		{"plain.sh", "plain"},
		{"no-extension", "no-extension"},
		{"/abs/path/runner.bash", "runner"},
		{"./multi.dot.name.js", "multi.dot.name"}, // only last extension stripped
	} {
		if got := scriptBaseName(tc.path); got != tc.want {
			t.Errorf("scriptBaseName(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// --- hookScriptName ---

func TestHookScriptName_ManifestShape(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"hooks": [
			{"event": "PreToolUse", "handler": {"command": "./scripts/lint-check.sh"}}
		]
	}`)
	if got := hookScriptName(data); got != "lint-check" {
		t.Errorf("got %q, want lint-check", got)
	}
}

func TestHookScriptName_ManifestSkipsBareCommand(t *testing.T) {
	t.Parallel()
	// Bare commands (no script-like form) are skipped, falls through to other shapes.
	data := []byte(`{
		"hooks": [
			{"event": "PreToolUse", "handler": {"command": "true"}}
		]
	}`)
	if got := hookScriptName(data); got != "" {
		t.Errorf("got %q, want empty (bare command not script-like)", got)
	}
}

func TestHookScriptName_ProviderSettingsShape(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"hooks": {
			"PreToolUse": [
				{"matcher": "Bash", "command": "scripts/check.py"}
			]
		}
	}`)
	if got := hookScriptName(data); got != "check" {
		t.Errorf("got %q, want check", got)
	}
}

func TestHookScriptName_LegacyFlatShape(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"command": "./tools/format.sh"
	}`)
	if got := hookScriptName(data); got != "format" {
		t.Errorf("got %q, want format", got)
	}
}

func TestHookScriptName_NoMatch(t *testing.T) {
	t.Parallel()
	data := []byte(`{"unrelated": "field"}`)
	if got := hookScriptName(data); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// --- hookEventName ---

func TestHookEventName_ManifestShape(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"hooks": [
			{"event": "PreToolUse", "matcher": "Bash"}
		]
	}`)
	if got := hookEventName(data); got != "PreToolUse · Bash" {
		t.Errorf("got %q, want %q", got, "PreToolUse · Bash")
	}
}

func TestHookEventName_ManifestMultiple(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"hooks": [
			{"event": "PreToolUse", "matcher": "Bash"},
			{"event": "PostToolUse"}
		]
	}`)
	got := hookEventName(data)
	want := "PreToolUse · Bash, PostToolUse"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHookEventName_ProviderSettingsShape(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"hooks": {
			"SessionEnd": [
				{"matcher": "all", "command": "echo done"}
			]
		}
	}`)
	if got := hookEventName(data); got != "SessionEnd · all" {
		t.Errorf("got %q, want %q", got, "SessionEnd · all")
	}
}

func TestHookEventName_ProviderSettings_NoMatcher(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"hooks": {
			"SessionStart": [{"command": "echo hi"}]
		}
	}`)
	if got := hookEventName(data); got != "SessionStart" {
		t.Errorf("got %q, want SessionStart", got)
	}
}

func TestHookEventName_LegacyFlat(t *testing.T) {
	t.Parallel()
	data := []byte(`{"event": "PreToolUse", "matcher": "Bash"}`)
	if got := hookEventName(data); got != "PreToolUse · Bash" {
		t.Errorf("got %q, want %q", got, "PreToolUse · Bash")
	}
}

func TestHookEventName_LegacyFlat_NoMatcher(t *testing.T) {
	t.Parallel()
	data := []byte(`{"event": "PreToolUse"}`)
	if got := hookEventName(data); got != "PreToolUse" {
		t.Errorf("got %q, want PreToolUse", got)
	}
}

func TestHookEventName_Empty(t *testing.T) {
	t.Parallel()
	if got := hookEventName([]byte(`{}`)); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// --- deriveHookDisplayName (integrates the above) ---

func TestDeriveHookDisplayName_NoHookFile(t *testing.T) {
	t.Parallel()
	item := &ContentItem{
		Path:  "/nowhere",
		Files: []string{"README.md"}, // no hook.json among files
		Type:  Hooks,
	}
	if got := deriveHookDisplayName(item); got != "" {
		t.Errorf("got %q, want empty (no hook file)", got)
	}
}

func TestDeriveHookDisplayName_UnreadableFile(t *testing.T) {
	t.Parallel()
	// hook.json listed in Files but not actually on disk
	item := &ContentItem{
		Path:  "/does/not/exist",
		Files: []string{"hook.json"},
		Type:  Hooks,
	}
	if got := deriveHookDisplayName(item); got != "" {
		t.Errorf("got %q, want empty (unreadable file)", got)
	}
}

func TestDeriveHookDisplayName_PrefersScriptName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hookPath := filepath.Join(dir, "hook.json")
	if err := os.WriteFile(hookPath, []byte(`{
		"hooks": [
			{"event": "PreToolUse", "matcher": "Bash", "handler": {"command": "./scripts/lint.sh"}}
		]
	}`), 0644); err != nil {
		t.Fatal(err)
	}
	item := &ContentItem{
		Path:  dir,
		Files: []string{"hook.json"},
		Type:  Hooks,
	}
	// Script name "lint" wins over event "PreToolUse · Bash"
	if got := deriveHookDisplayName(item); got != "lint" {
		t.Errorf("got %q, want lint (script preferred over event)", got)
	}
}

func TestDeriveHookDisplayName_FallsBackToEvent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hookPath := filepath.Join(dir, "hook.json")
	// No script-like command — must fall back to event name.
	if err := os.WriteFile(hookPath, []byte(`{
		"hooks": [
			{"event": "SessionStart", "handler": {"command": "true"}}
		]
	}`), 0644); err != nil {
		t.Fatal(err)
	}
	item := &ContentItem{
		Path:  dir,
		Files: []string{"hook.json"},
		Type:  Hooks,
	}
	if got := deriveHookDisplayName(item); got != "SessionStart" {
		t.Errorf("got %q, want SessionStart", got)
	}
}
