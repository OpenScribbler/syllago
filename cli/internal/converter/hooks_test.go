package converter

import (
	"encoding/json"
	"testing"

	"github.com/holdenhewett/nesco/cli/internal/provider"
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
	assertContains(t, matchers[0].Hooks[0].Command, "nesco-llm-hook")

	// ExtraFiles should contain the wrapper script
	if result.ExtraFiles == nil {
		t.Fatal("expected ExtraFiles to contain generated script")
	}
	if len(result.ExtraFiles) != 1 {
		t.Fatalf("expected 1 extra file, got %d", len(result.ExtraFiles))
	}

	// Verify script content
	for name, content := range result.ExtraFiles {
		assertContains(t, name, "nesco-llm-hook")
		assertContains(t, name, ".sh")
		script := string(content)
		assertContains(t, script, "#!/bin/bash")
		assertContains(t, script, "nesco-generated")
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
	assertContains(t, entries[0].Bash, "nesco-llm-hook")
	assertContains(t, entries[0].Comment, "nesco-generated")

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
