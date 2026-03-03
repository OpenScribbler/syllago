package converter

import (
	"encoding/json"
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
	input := []byte(`{
		"hooks": {
			"preToolUse": [
				{
					"bash": "echo check",
					"timeoutSec": 5,
					"comment": "Safety check"
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

	// Matcher should generate a warning
	hasMatcherWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, "matcher") {
			hasMatcherWarning = true
			break
		}
	}
	if !hasMatcherWarning {
		t.Fatal("expected warning about dropped matcher")
	}
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
	entries := cfg.Hooks["preToolUse"]
	if len(entries) == 0 {
		t.Fatal("expected LLM hook to be converted for copilot")
	}
	assertContains(t, entries[0].Bash, "syllago-llm-hook")
	assertContains(t, entries[0].Comment, "syllago-generated")

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
		{"cline", provider.Cline},
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
		Hooks:   []HookEntry{{Type: "command", Command: "echo check", Timeout: 3000, StatusMessage: "Checking..."}},
	}
	conv := &HooksConverter{}
	result, err := conv.RenderFlat(hook, provider.CopilotCLI)
	if err != nil {
		t.Fatalf("RenderFlat: %v", err)
	}
	out := string(result.Content)
	assertContains(t, out, "preToolUse")
	assertContains(t, out, "echo check")
	// Matcher dropped with warning
	hasMatcherWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, "matcher") {
			hasMatcherWarning = true
		}
	}
	if !hasMatcherWarning {
		t.Error("expected matcher dropped warning for copilot")
	}
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
