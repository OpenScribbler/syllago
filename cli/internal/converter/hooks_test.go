package converter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/provider"
)

func TestClaudeHooksToGemini(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Bash",
					"hooks": [
						{"type": "command", "command": "echo checking", "timeout": 5000}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "BeforeTool")
	assertContains(t, out, "run_shell_command")
	assertContains(t, out, "echo checking")
	assertNotContains(t, out, "PreToolUse")
	assertNotContains(t, out, "\"Bash\"")
}

func TestGeminiHooksToClaude(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"BeforeTool": [
				{
					"matcher": "run_shell_command",
					"hooks": [
						{"type": "command", "command": "echo safe", "timeout": 3000}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "PreToolUse")
	assertContains(t, out, "\"Bash\"")
	assertNotContains(t, out, "BeforeTool")
}

func TestUnsupportedEventDroppedWithWarning(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"SubagentStart": [
				{
					"hooks": [
						{"type": "command", "command": "echo subagent"}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for unsupported event")
	}
	assertContains(t, result.Warnings[0], "SubagentStart")
	assertContains(t, result.Warnings[0], "not supported")
}

func TestLLMHookDroppedWithWarning(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"hooks": [
						{"type": "prompt", "command": "Is this safe?"}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for LLM-evaluated hook")
	}
	assertContains(t, result.Warnings[0], "prompt")

	// Verify the hook was dropped (empty hooks)
	var cfg hooksConfig
	json.Unmarshal(result.Content, &cfg)
	if matchers, ok := cfg.Hooks["BeforeTool"]; ok && len(matchers) > 0 {
		t.Fatal("expected LLM hook to be dropped")
	}
}

func TestCopilotHooksToClaude(t *testing.T) {
	// Copilot hooks use matcher groups: {"version":1, "hooks":{"event":[{"matcher":"...","hooks":[...]}]}}
	input := []byte(`{
		"version": 1,
		"hooks": {
			"preToolUse": [
				{
					"matcher": "bash",
					"hooks": [
						{
							"type": "command",
							"bash": "echo check",
							"timeoutSec": 5,
							"comment": "Safety check"
						}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "copilot-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "PreToolUse")
	assertContains(t, out, "echo check")
	assertContains(t, out, "5000") // Converted back to ms
	assertContains(t, out, "Safety check")
	// Matcher should be translated from copilot "bash" to canonical "Bash"
	assertContains(t, out, "\"Bash\"")
}

func TestClaudeHooksToCopilot(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Bash",
					"hooks": [
						{"type": "command", "command": "echo verify", "timeout": 3000, "statusMessage": "Verifying..."}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	assertContains(t, out, "preToolUse")
	assertContains(t, out, "echo verify")
	assertContains(t, out, "\"timeoutSec\": 3")
	assertContains(t, out, "Verifying...")
	// Version field present
	assertContains(t, out, "\"version\": 1")
	// Matcher should be preserved (translated to Copilot tool name)
	assertContains(t, out, "\"matcher\": \"bash\"")
	// Type field should be present
	assertContains(t, out, "\"type\": \"command\"")
}

func TestLLMHookGenerateMode(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"hooks": [
						{"type": "prompt", "command": "Is this command safe? Respond with allow or deny."}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{LLMHooksMode: LLMHooksModeGenerate}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Hook should NOT be dropped — should be replaced with command type
	var cfg hooksConfig
	json.Unmarshal(result.Content, &cfg)
	matchers := cfg.Hooks["BeforeTool"]
	if len(matchers) == 0 {
		t.Fatal("expected LLM hook to be converted, not dropped")
	}
	if matchers[0].Hooks[0].Type != "command" {
		t.Fatalf("expected type 'command', got %q", matchers[0].Hooks[0].Type)
	}
	assertContains(t, matchers[0].Hooks[0].Command, "syllago-llm-hook")

	// ExtraFiles should contain the wrapper script
	if result.ExtraFiles == nil {
		t.Fatal("expected ExtraFiles to contain generated script")
	}
	if len(result.ExtraFiles) != 1 {
		t.Fatalf("expected 1 extra file, got %d", len(result.ExtraFiles))
	}

	// Verify script content
	for name, content := range result.ExtraFiles {
		assertContains(t, name, "syllago-llm-hook")
		assertContains(t, name, ".sh")
		script := string(content)
		assertContains(t, script, "#!/bin/bash")
		assertContains(t, script, "syllago-generated")
		assertContains(t, script, "gemini")
		assertContains(t, script, "Is this command safe")
	}

	// Should have a warning noting the conversion (not a drop)
	hasConvertedWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, "converted to wrapper script") {
			hasConvertedWarning = true
			break
		}
	}
	if !hasConvertedWarning {
		t.Fatalf("expected 'converted to wrapper script' warning, got: %v", result.Warnings)
	}
}

func TestLLMHookGenerateModeCopilot(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"hooks": [
						{"type": "prompt", "command": "Check safety"}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{LLMHooksMode: LLMHooksModeGenerate}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Copilot format: should have bash field with script reference
	var cfg copilotHooksConfig
	json.Unmarshal(result.Content, &cfg)
	groups := cfg.Hooks["preToolUse"]
	if len(groups) == 0 || len(groups[0].Hooks) == 0 {
		t.Fatal("expected LLM hook to be converted for copilot")
	}
	assertContains(t, groups[0].Hooks[0].Bash, "syllago-llm-hook")
	assertContains(t, groups[0].Hooks[0].Comment, "syllago-generated")

	if result.ExtraFiles == nil || len(result.ExtraFiles) != 1 {
		t.Fatal("expected 1 extra file for copilot LLM hook")
	}
}

func TestLLMHookDefaultSkipMode(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"hooks": [
						{"type": "prompt", "command": "Check safety"}
					]
				}
			]
		}
	}`)

	// Empty LLMHooksMode should default to skip
	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.GeminiCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Should be dropped (skip mode)
	var cfg hooksConfig
	json.Unmarshal(result.Content, &cfg)
	if matchers, ok := cfg.Hooks["BeforeTool"]; ok && len(matchers) > 0 {
		t.Fatal("expected LLM hook to be dropped in skip mode")
	}

	// Warning should mention --llm-hooks=generate
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning")
	}
	assertContains(t, result.Warnings[0], "--llm-hooks=generate")
}

// --- Kiro hooks ---

func TestClaudeHooksToKiro(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Bash",
					"hooks": [
						{"type": "command", "command": "echo checking", "timeout": 5000}
					]
				}
			],
			"SessionStart": [
				{
					"hooks": [
						{"type": "command", "command": "echo starting"}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.Kiro)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)
	// Output is the syllago-hooks.json agent file
	assertContains(t, out, `"name": "syllago-hooks"`)
	assertContains(t, out, `"preToolUse"`)
	assertContains(t, out, `"agentSpawn"`)
	assertContains(t, out, "echo checking")
	assertContains(t, out, "echo starting")
	// Matcher translated: Bash → shell
	assertContains(t, out, `"matcher": "shell"`)
	assertNotContains(t, out, "PreToolUse")
	assertEqual(t, "syllago-hooks.json", result.Filename)
}

func TestHooklessProviderWarning(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Bash",
					"hooks": [
						{"type": "command", "command": "echo check", "timeout": 5000}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	hooklessTargets := []struct {
		name string
		prov provider.Provider
	}{
		{"opencode", provider.OpenCode},
		{"zed", provider.Zed},
		{"roo-code", provider.RooCode},
	}

	for _, tt := range hooklessTargets {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.Render(canonical.Content, tt.prov)
			if err != nil {
				t.Fatalf("Render to %s: %v", tt.name, err)
			}

			// Content should be nil (no output)
			if result.Content != nil {
				t.Errorf("expected nil Content for hookless provider %s, got %d bytes", tt.name, len(result.Content))
			}

			// Filename should be empty
			if result.Filename != "" {
				t.Errorf("expected empty Filename for hookless provider %s, got %q", tt.name, result.Filename)
			}

			// Should have exactly one warning
			if len(result.Warnings) != 1 {
				t.Fatalf("expected 1 warning for %s, got %d: %v", tt.name, len(result.Warnings), result.Warnings)
			}
			assertContains(t, result.Warnings[0], "does not support hooks")
			assertContains(t, result.Warnings[0], tt.name)
		})
	}
}

// --- Flat format tests (Task 1.3) ---

func TestDetectHookFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"flat", `{"event":"PreToolUse","hooks":[]}`, "flat"},
		{"nested", `{"hooks":{"PreToolUse":[]}}`, "nested"},
		{"flat with matcher", `{"event":"PostToolUse","matcher":"Bash","hooks":[]}`, "flat"},
		{"invalid json", `not json`, "nested"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectHookFormat([]byte(tt.input))
			if got != tt.expect {
				t.Errorf("DetectHookFormat: got %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestParseFlat(t *testing.T) {
	t.Parallel()
	input := `{"event":"PreToolUse","matcher":"Bash","hooks":[{"type":"command","command":"go vet ./...","timeout":5000}]}`
	hd, err := ParseFlat([]byte(input))
	if err != nil {
		t.Fatalf("ParseFlat: %v", err)
	}
	if hd.Event != "PreToolUse" {
		t.Errorf("event: got %q", hd.Event)
	}
	if hd.Matcher != "Bash" {
		t.Errorf("matcher: got %q", hd.Matcher)
	}
	if len(hd.Hooks) != 1 {
		t.Fatalf("hooks count: got %d", len(hd.Hooks))
	}
	if hd.Hooks[0].Command != "go vet ./..." {
		t.Errorf("command: got %q", hd.Hooks[0].Command)
	}
}

func TestParseFlat_MissingEvent(t *testing.T) {
	t.Parallel()
	input := `{"matcher":"Bash","hooks":[{"type":"command","command":"echo"}]}`
	_, err := ParseFlat([]byte(input))
	if err == nil {
		t.Fatal("expected error for missing event")
	}
}

func TestParseNested(t *testing.T) {
	t.Parallel()
	input := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo"}]}],"PostToolUse":[{"hooks":[{"type":"command","command":"echo done"}]}]}}`
	items, err := ParseNested([]byte(input))
	if err != nil {
		t.Fatalf("ParseNested: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	// Verify we got both events (order is map-iteration dependent)
	events := map[string]bool{}
	for _, item := range items {
		events[item.Event] = true
	}
	if !events["PreToolUse"] || !events["PostToolUse"] {
		t.Errorf("expected PreToolUse and PostToolUse, got %v", events)
	}
}

func TestCanonicalizeFlatHook_GeminiCLI(t *testing.T) {
	t.Parallel()
	input := `{"event":"BeforeTool","matcher":"run_shell_command","hooks":[{"type":"command","command":"echo safe"}]}`
	conv := &HooksConverter{}
	result, err := conv.Canonicalize([]byte(input), "gemini-cli")
	if err != nil {
		t.Fatalf("Canonicalize flat: %v", err)
	}
	var hd HookData
	json.Unmarshal(result.Content, &hd)
	if hd.Event != "PreToolUse" {
		t.Errorf("event not translated: got %q", hd.Event)
	}
	if hd.Matcher != "Bash" {
		t.Errorf("matcher not translated: got %q", hd.Matcher)
	}
}

func TestCanonicalizeFlatHook_ClaudeCode(t *testing.T) {
	t.Parallel()
	input := `{"event":"PreToolUse","matcher":"Bash","hooks":[{"type":"command","command":"echo check"}]}`
	conv := &HooksConverter{}
	result, err := conv.Canonicalize([]byte(input), "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize flat: %v", err)
	}
	var hd HookData
	json.Unmarshal(result.Content, &hd)
	if hd.Event != "PreToolUse" {
		t.Errorf("event should pass through: got %q", hd.Event)
	}
	if hd.Matcher != "Bash" {
		t.Errorf("matcher should pass through: got %q", hd.Matcher)
	}
}

func TestRenderFlat_Copilot(t *testing.T) {
	t.Parallel()
	hook := HookData{
		Event:   "PreToolUse",
		Matcher: "Bash",
		Hooks:   []HookEntry{{Type: "command", Command: "echo check", Timeout: 3, StatusMessage: "Checking..."}},
	}
	conv := &HooksConverter{}
	result, err := conv.RenderFlat(hook, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("RenderFlat: %v", err)
	}
	out := string(result.Content)
	assertContains(t, out, "preToolUse")
	assertContains(t, out, "echo check")
	// Matcher should be preserved and translated
	assertContains(t, out, "\"matcher\": \"bash\"")
	// Version field
	assertContains(t, out, "\"version\": 1")
	// Type field
	assertContains(t, out, "\"type\": \"command\"")
}

func TestLoadHookData_DirectoryFormat(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hookJSON := `{"event":"PreToolUse","matcher":"Bash","hooks":[{"type":"command","command":"go vet ./..."}]}`
	os.WriteFile(filepath.Join(dir, "hook.json"), []byte(hookJSON), 0644)
	item := catalog.ContentItem{Type: catalog.Hooks, Path: dir}

	hd, err := LoadHookData(item)
	if err != nil {
		t.Fatalf("LoadHookData: %v", err)
	}
	if hd.Event != "PreToolUse" {
		t.Errorf("event: %q", hd.Event)
	}
	if hd.Matcher != "Bash" {
		t.Errorf("matcher: %q", hd.Matcher)
	}
}

func TestLoadHookData_NestedFallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	hookJSON := `{"hooks":{"PostToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"echo lint"}]}]}}`
	os.WriteFile(filepath.Join(dir, "hook.json"), []byte(hookJSON), 0644)
	item := catalog.ContentItem{Type: catalog.Hooks, Path: dir}

	hd, err := LoadHookData(item)
	if err != nil {
		t.Fatalf("LoadHookData nested: %v", err)
	}
	if hd.Event != "PostToolUse" {
		t.Errorf("event: %q", hd.Event)
	}
}

func TestCopilotHooksVersionField(t *testing.T) {
	t.Parallel()
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"hooks": [
						{"type": "command", "command": "echo hello", "timeout": 5}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Rendered Copilot hooks must have version: 1
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(result.Content, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	versionRaw, ok := raw["version"]
	if !ok {
		t.Fatal("expected top-level 'version' field in Copilot hooks output")
	}
	if string(versionRaw) != "1" {
		t.Fatalf("expected version 1, got %s", string(versionRaw))
	}
}

func TestCopilotHooksMatcherPreserved(t *testing.T) {
	t.Parallel()
	// Canonical hooks with a matcher should preserve it when rendering to Copilot
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Bash",
					"hooks": [
						{"type": "command", "command": "echo safe", "timeout": 3}
					]
				},
				{
					"hooks": [
						{"type": "command", "command": "echo general"}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var cfg copilotHooksConfig
	if err := json.Unmarshal(result.Content, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	groups := cfg.Hooks["preToolUse"]
	if len(groups) != 2 {
		t.Fatalf("expected 2 matcher groups, got %d", len(groups))
	}

	// Find the group with matcher
	foundMatcher := false
	foundNoMatcher := false
	for _, g := range groups {
		if g.Matcher == "bash" {
			foundMatcher = true
			if len(g.Hooks) != 1 || g.Hooks[0].Bash != "echo safe" {
				t.Errorf("matched group unexpected content: %+v", g)
			}
		} else if g.Matcher == "" {
			foundNoMatcher = true
			if len(g.Hooks) != 1 || g.Hooks[0].Bash != "echo general" {
				t.Errorf("unmatched group unexpected content: %+v", g)
			}
		}
	}
	if !foundMatcher {
		t.Error("expected group with matcher 'bash'")
	}
	if !foundNoMatcher {
		t.Error("expected group without matcher")
	}

	// No warnings about dropped matchers
	for _, w := range result.Warnings {
		if containsStr(w, "matcher") && containsStr(w, "dropped") {
			t.Errorf("unexpected matcher dropped warning: %s", w)
		}
	}
}

func TestCopilotHookEntryTypeField(t *testing.T) {
	t.Parallel()
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"hooks": [
						{"type": "command", "command": "echo check", "timeout": 5}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	result, err := conv.Render(canonical.Content, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var cfg copilotHooksConfig
	if err := json.Unmarshal(result.Content, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	groups := cfg.Hooks["preToolUse"]
	if len(groups) == 0 || len(groups[0].Hooks) == 0 {
		t.Fatal("expected hooks in output")
	}

	entry := groups[0].Hooks[0]
	if entry.Type != "command" {
		t.Errorf("expected type 'command', got %q", entry.Type)
	}
}

func TestCopilotHooksRoundtripWithMatcher(t *testing.T) {
	t.Parallel()
	// Full roundtrip: Copilot (with matcher) -> canonical -> Copilot
	input := []byte(`{
		"version": 1,
		"hooks": {
			"preToolUse": [
				{
					"matcher": "bash",
					"hooks": [
						{
							"type": "command",
							"bash": "echo safety",
							"timeoutSec": 10,
							"comment": "Safety check"
						}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	canonical, err := conv.Canonicalize(input, "copilot-cli")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	// Canonical should have matcher translated to canonical (Bash)
	var canonCfg hooksConfig
	json.Unmarshal(canonical.Content, &canonCfg)
	matchers := canonCfg.Hooks["PreToolUse"]
	if len(matchers) == 0 {
		t.Fatal("expected matchers in canonical")
	}
	if matchers[0].Matcher != "Bash" {
		t.Errorf("expected canonical matcher 'Bash', got %q", matchers[0].Matcher)
	}

	// Render back to Copilot
	result, err := conv.Render(canonical.Content, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var outCfg copilotHooksConfig
	json.Unmarshal(result.Content, &outCfg)

	groups := outCfg.Hooks["preToolUse"]
	if len(groups) == 0 {
		t.Fatal("expected groups in output")
	}
	if groups[0].Matcher != "bash" {
		t.Errorf("expected matcher 'bash' in output, got %q", groups[0].Matcher)
	}
	if outCfg.Version != 1 {
		t.Errorf("expected version 1, got %d", outCfg.Version)
	}
}

// --- Hook type support tests (http, prompt, agent) ---

func TestHookCanonicalizeHTTPPreservesFields(t *testing.T) {
	t.Parallel()
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Bash",
					"hooks": [
						{
							"type": "http",
							"url": "https://example.com/hook",
							"headers": {"Authorization": "Bearer $TOKEN", "Content-Type": "application/json"},
							"allowedEnvVars": ["TOKEN", "API_KEY"],
							"timeout": 10000
						}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	result, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	var cfg hooksConfig
	if err := json.Unmarshal(result.Content, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	matchers := cfg.Hooks["PreToolUse"]
	if len(matchers) == 0 || len(matchers[0].Hooks) == 0 {
		t.Fatal("expected hook entries")
	}

	h := matchers[0].Hooks[0]
	assertEqual(t, "http", h.Type)
	assertEqual(t, "https://example.com/hook", h.URL)
	assertEqual(t, "10", fmt.Sprintf("%d", h.Timeout)) // 10000ms -> 10s canonical
	if len(h.Headers) != 2 {
		t.Fatalf("expected 2 headers, got %d", len(h.Headers))
	}
	assertEqual(t, "Bearer $TOKEN", h.Headers["Authorization"])
	if len(h.AllowedEnvVars) != 2 {
		t.Fatalf("expected 2 allowedEnvVars, got %d", len(h.AllowedEnvVars))
	}
}

func TestHookCanonicalizePromptPreservesFields(t *testing.T) {
	t.Parallel()
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"hooks": [
						{
							"type": "prompt",
							"prompt": "Is this command safe to run?",
							"model": "claude-sonnet-4-20250514",
							"timeout": 15000
						}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	result, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	var cfg hooksConfig
	if err := json.Unmarshal(result.Content, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	matchers := cfg.Hooks["PreToolUse"]
	if len(matchers) == 0 || len(matchers[0].Hooks) == 0 {
		t.Fatal("expected hook entries")
	}

	h := matchers[0].Hooks[0]
	assertEqual(t, "prompt", h.Type)
	assertEqual(t, "Is this command safe to run?", h.Prompt)
	assertEqual(t, "claude-sonnet-4-20250514", h.Model)
	assertEqual(t, "15", fmt.Sprintf("%d", h.Timeout)) // 15000ms -> 15s
}

func TestHookCanonicalizeAgentPreservesFields(t *testing.T) {
	t.Parallel()
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"hooks": [
						{
							"type": "agent",
							"agent": "security-reviewer",
							"timeout": 30000
						}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	result, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}

	var cfg hooksConfig
	if err := json.Unmarshal(result.Content, &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	matchers := cfg.Hooks["PreToolUse"]
	if len(matchers) == 0 || len(matchers[0].Hooks) == 0 {
		t.Fatal("expected hook entries")
	}

	h := matchers[0].Hooks[0]
	assertEqual(t, "agent", h.Type)
	// Agent is json.RawMessage — verify it round-trips
	assertEqual(t, `"security-reviewer"`, string(h.Agent))
}

func TestHookRenderClaudeCodeIncludesTypeSpecificFields(t *testing.T) {
	t.Parallel()
	// Canonical input with all 4 types
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Bash",
					"hooks": [
						{"type": "command", "command": "echo check", "timeout": 5},
						{
							"type": "http",
							"url": "https://example.com/hook",
							"headers": {"Authorization": "Bearer token"},
							"allowedEnvVars": ["TOKEN"],
							"timeout": 10
						},
						{
							"type": "prompt",
							"prompt": "Is this safe?",
							"model": "claude-sonnet-4-20250514",
							"timeout": 15
						},
						{
							"type": "agent",
							"agent": "security-reviewer",
							"timeout": 30
						}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}
	result, err := conv.Render(input, provider.ClaudeCode)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	out := string(result.Content)

	// All 4 hooks should be present (no warnings about dropped types)
	for _, w := range result.Warnings {
		if containsStr(w, "dropped") {
			t.Errorf("unexpected drop warning: %s", w)
		}
	}

	// Command hook
	assertContains(t, out, `"type": "command"`)
	assertContains(t, out, "echo check")

	// HTTP hook fields
	assertContains(t, out, `"type": "http"`)
	assertContains(t, out, "https://example.com/hook")
	assertContains(t, out, "Bearer token")
	assertContains(t, out, "TOKEN")

	// Prompt hook fields
	assertContains(t, out, `"type": "prompt"`)
	assertContains(t, out, "Is this safe?")
	assertContains(t, out, "claude-sonnet-4-20250514")

	// Agent hook fields
	assertContains(t, out, `"type": "agent"`)
	assertContains(t, out, "security-reviewer")

	// Timeouts should be converted to ms (canonical seconds * 1000)
	assertContains(t, out, "5000")  // command
	assertContains(t, out, "10000") // http
	assertContains(t, out, "15000") // prompt
	assertContains(t, out, "30000") // agent
}

func TestHookRenderNonClaudeWarnsHTTPType(t *testing.T) {
	t.Parallel()
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"hooks": [
						{
							"type": "http",
							"url": "https://example.com/hook",
							"timeout": 10
						}
					]
				}
			]
		}
	}`)

	conv := &HooksConverter{}

	targets := []struct {
		name string
		prov provider.Provider
	}{
		{"gemini-cli", provider.GeminiCLI},
		{"copilot-cli", provider.CopilotCLI},
		{"kiro", provider.Kiro},
	}

	for _, tt := range targets {
		t.Run(tt.name, func(t *testing.T) {
			result, err := conv.Render(input, tt.prov)
			if err != nil {
				t.Fatalf("Render to %s: %v", tt.name, err)
			}

			// Should have warning about unsupported type
			if len(result.Warnings) == 0 {
				t.Fatalf("expected warning for http hook type on %s", tt.name)
			}

			foundHTTPWarning := false
			for _, w := range result.Warnings {
				if containsStr(w, "http") && containsStr(w, "Claude Code") {
					foundHTTPWarning = true
					break
				}
			}
			if !foundHTTPWarning {
				t.Errorf("expected warning mentioning 'http' and 'Claude Code', got: %v", result.Warnings)
			}
		})
	}
}

func TestHookCanonicalizeFlatHTTPHook(t *testing.T) {
	t.Parallel()
	input := []byte(`{
		"event": "PreToolUse",
		"matcher": "Bash",
		"hooks": [
			{
				"type": "http",
				"url": "https://example.com/check",
				"headers": {"X-Custom": "value"},
				"timeout": 5000
			}
		]
	}`)

	conv := &HooksConverter{}
	result, err := conv.Canonicalize(input, "claude-code")
	if err != nil {
		t.Fatalf("Canonicalize flat http: %v", err)
	}

	var hd HookData
	if err := json.Unmarshal(result.Content, &hd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	assertEqual(t, "http", hd.Hooks[0].Type)
	assertEqual(t, "https://example.com/check", hd.Hooks[0].URL)
	assertEqual(t, "5", fmt.Sprintf("%d", hd.Hooks[0].Timeout)) // ms -> s
	assertEqual(t, "value", hd.Hooks[0].Headers["X-Custom"])
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
