package converter

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/tidwall/gjson"
)

// convertHooksT runs ConvertHooks and fails the test on error.
func convertHooksT(t *testing.T, raw []byte, from, to string) *Result {
	t.Helper()
	res, err := ConvertHooks(raw, from, to)
	if err != nil {
		t.Fatalf("ConvertHooks(%s -> %s): %v", from, to, err)
	}
	return res
}

// decodeHooksT runs DecodeHooks, the canonicalize step of ConvertHooks, and
// fails the test on error.
func decodeHooksT(t *testing.T, raw []byte, from string) *CanonicalHooks {
	t.Helper()
	hooks, err := DecodeHooks(raw, from)
	if err != nil {
		t.Fatalf("DecodeHooks(%s): %v", from, err)
	}
	if len(hooks.Hooks) == 0 {
		t.Fatalf("DecodeHooks(%s): no hooks", from)
	}
	return hooks
}

// matcherOf returns a canonical hook's matcher as a bare string.
func matcherOf(t *testing.T, h CanonicalHook) string {
	t.Helper()
	var s string
	if err := json.Unmarshal(h.Matcher, &s); err != nil {
		t.Fatalf("matcher %s is not a bare string: %v", h.Matcher, err)
	}
	return s
}

// manifestJSON wraps canonical hook objects in a hooks/0.1 manifest.
func manifestJSON(hooks ...string) []byte {
	return []byte(`{"spec":"hooks/0.1","hooks":[` + strings.Join(hooks, ",") + `]}`)
}

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

	result := convertHooksT(t, input, "claude-code", "gemini-cli")

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

	result := convertHooksT(t, input, "gemini-cli", "claude-code")

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

	result := convertHooksT(t, input, "claude-code", "gemini-cli")

	// Find the event-specific warning (not the structured output warning)
	found := false
	for _, w := range result.Warnings {
		if containsStr(w, "subagent_start") && containsStr(w, "not supported") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected warning mentioning SubagentStart and 'not supported', got: %v", result.Warnings)
	}
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

	result := convertHooksT(t, input, "claude-code", "gemini-cli")

	// Find the LLM-specific warning (not the structured output warning)
	foundPromptWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, "prompt") && !containsStr(w, "structured hook output") {
			foundPromptWarning = true
			break
		}
	}
	if !foundPromptWarning {
		t.Fatal("expected warning for LLM-evaluated hook mentioning 'prompt'")
	}

	// The only hook was dropped, so there is no content.
	if result.Content != nil {
		t.Fatalf("expected LLM hook to be dropped, got: %s", result.Content)
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

	result := convertHooksT(t, input, "copilot-cli", "claude-code")

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

	result := convertHooksT(t, input, "claude-code", "copilot-cli")

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

	result, err := ConvertHooksWrappingLLM(input, "claude-code", "gemini-cli")
	if err != nil {
		t.Fatalf("ConvertHooksWrappingLLM: %v", err)
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

	result, err := ConvertHooksWrappingLLM(input, "claude-code", "copilot-cli")
	if err != nil {
		t.Fatalf("ConvertHooksWrappingLLM: %v", err)
	}

	// Copilot format: should have bash field with script reference
	var cfg copilotNativeConfig
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

	// ConvertHooks (no LLM wrapping) is skip mode
	result := convertHooksT(t, input, "claude-code", "gemini-cli")

	// Should be dropped (skip mode)
	if result.Content != nil {
		t.Fatalf("expected LLM hook to be dropped in skip mode, got: %s", result.Content)
	}

	// The drop is explained; sync install adds the --llm-hooks hint
	foundLLMWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, `hook type "prompt" is not supported`) {
			foundLLMWarning = true
			break
		}
	}
	if !foundLLMWarning {
		t.Fatalf("expected a warning that the prompt hook was dropped, got: %v", result.Warnings)
	}
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

	result := convertHooksT(t, input, "claude-code", "kiro")

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

// A provider with no hook support has no hook encoder, so converting to it
// reports ErrNoHookEncoder instead of producing output.
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

	for _, slug := range []string{"zed", "roo-code"} {
		t.Run(slug, func(t *testing.T) {
			result, err := ConvertHooks(input, "claude-code", slug)
			if !errors.Is(err, ErrNoHookEncoder) {
				t.Fatalf("ConvertHooks to %s: err = %v, want ErrNoHookEncoder", slug, err)
			}
			if result != nil {
				t.Errorf("expected no result for hookless provider %s, got %+v", slug, result)
			}
			assertContains(t, err.Error(), slug)
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
	hooks := decodeHooksT(t, []byte(input), "gemini-cli")
	if hooks.Hooks[0].Event != "before_tool_execute" {
		t.Errorf("event not translated: got %q", hooks.Hooks[0].Event)
	}
	if got := matcherOf(t, hooks.Hooks[0]); got != "shell" {
		t.Errorf("matcher not translated: got %q", got)
	}
}

func TestCanonicalizeFlatHook_ClaudeCode(t *testing.T) {
	t.Parallel()
	input := `{"event":"PreToolUse","matcher":"Bash","hooks":[{"type":"command","command":"echo check"}]}`
	hooks := decodeHooksT(t, []byte(input), "claude-code")
	if hooks.Hooks[0].Event != "before_tool_execute" {
		t.Errorf("event should be translated to neutral: got %q", hooks.Hooks[0].Event)
	}
	if got := matcherOf(t, hooks.Hooks[0]); got != "shell" {
		t.Errorf("matcher should be translated to neutral: got %q", got)
	}
}

func TestRenderFlat_Copilot(t *testing.T) {
	t.Parallel()
	input := manifestJSON(`{"event":"before_tool_execute","matcher":"shell","handler":{"type":"command","command":"echo check","timeout":3,"status_message":"Checking..."}}`)
	result := convertHooksT(t, input, "", "copilot-cli")
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

func TestRenderFlat_Crush(t *testing.T) {
	t.Parallel()
	input := manifestJSON(`{"event":"before_tool_execute","matcher":"shell","handler":{"type":"command","command":"echo check","timeout":3}}`)
	result := convertHooksT(t, input, "", "crush")
	entry := gjson.GetBytes(result.Content, "hooks.PreToolUse.0")
	if !entry.Exists() {
		t.Fatalf("expected hooks.PreToolUse.0, got: %s", result.Content)
	}
	// Crush entries are flat — command lives directly on the entry, not in a
	// nested hooks array.
	if got := entry.Get("command").String(); got != "echo check" {
		t.Errorf("command: got %q, want %q", got, "echo check")
	}
	if entry.Get("hooks").Exists() {
		t.Error("crush entries must not contain a nested hooks array")
	}
	if got := entry.Get("matcher").String(); got != "bash" {
		t.Errorf("matcher: got %q, want bash", got)
	}
	// Seconds stay seconds — 3 must not become 3000.
	if got := entry.Get("timeout").Int(); got != 3 {
		t.Errorf("timeout: got %d, want 3", got)
	}
}

func TestRenderCrush_UnsupportedEventDropped(t *testing.T) {
	t.Parallel()
	input := manifestJSON(`{"event":"session_start","handler":{"type":"command","command":"echo hi"}}`)
	result := convertHooksT(t, input, "", "crush")
	if result.Content != nil {
		t.Errorf("unsupported event should be dropped, got: %s", result.Content)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected warning for unsupported event")
	}
}

func TestCanonicalize_Crush(t *testing.T) {
	t.Parallel()
	input := `{"hooks":{"PreToolUse":[{"matcher":"bash","command":"echo safe","timeout":5}]}}`
	hooks := decodeHooksT(t, []byte(input), "crush")
	if len(hooks.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got: %+v", hooks.Hooks)
	}
	h := hooks.Hooks[0]
	if h.Event != "before_tool_execute" {
		t.Errorf("event: got %q, want before_tool_execute", h.Event)
	}
	if got := matcherOf(t, h); got != "shell" {
		t.Errorf("matcher: got %q, want shell", got)
	}
	if h.Handler.Command != "echo safe" {
		t.Fatalf("command: got %q", h.Handler.Command)
	}
	// Crush timeouts are seconds — canonical unit, no /1000.
	if h.Handler.Timeout != 5 {
		t.Errorf("timeout: got %d, want 5", h.Handler.Timeout)
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

	result := convertHooksT(t, input, "claude-code", "copilot-cli")

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
	// Hooks with a matcher should preserve it when rendering to Copilot
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

	result := convertHooksT(t, input, "claude-code", "copilot-cli")

	var cfg copilotNativeConfig
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
		switch g.Matcher {
		case "bash":
			foundMatcher = true
			if len(g.Hooks) != 1 || g.Hooks[0].Bash != "echo safe" {
				t.Errorf("matched group unexpected content: %+v", g)
			}
		case "":
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

	result := convertHooksT(t, input, "claude-code", "copilot-cli")

	var cfg copilotNativeConfig
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

	// Canonical should have matcher translated to neutral (shell)
	canonical := decodeHooksT(t, input, "copilot-cli")
	if canonical.Hooks[0].Event != "before_tool_execute" {
		t.Errorf("expected canonical event before_tool_execute, got %q", canonical.Hooks[0].Event)
	}
	if got := matcherOf(t, canonical.Hooks[0]); got != "shell" {
		t.Errorf("expected canonical matcher 'shell', got %q", got)
	}

	// Render back to Copilot
	result := convertHooksT(t, input, "copilot-cli", "copilot-cli")

	var outCfg copilotNativeConfig
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

	hooks := decodeHooksT(t, input, "claude-code")
	if hooks.Hooks[0].Event != "before_tool_execute" {
		t.Fatalf("expected before_tool_execute, got %q", hooks.Hooks[0].Event)
	}

	h := hooks.Hooks[0].Handler
	assertEqual(t, "http", h.Type)
	assertEqual(t, "https://example.com/hook", h.URL)
	if h.Timeout != 10 { // 10000ms -> 10s canonical
		t.Errorf("timeout: got %d, want 10", h.Timeout)
	}
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

	hooks := decodeHooksT(t, input, "claude-code")
	if hooks.Hooks[0].Event != "before_tool_execute" {
		t.Fatalf("expected before_tool_execute, got %q", hooks.Hooks[0].Event)
	}

	h := hooks.Hooks[0].Handler
	assertEqual(t, "prompt", h.Type)
	assertEqual(t, "Is this command safe to run?", h.Prompt)
	assertEqual(t, "claude-sonnet-4-20250514", h.Model)
	if h.Timeout != 15 { // 15000ms -> 15s
		t.Errorf("timeout: got %d, want 15", h.Timeout)
	}
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

	hooks := decodeHooksT(t, input, "claude-code")
	if hooks.Hooks[0].Event != "before_tool_execute" {
		t.Fatalf("expected before_tool_execute, got %q", hooks.Hooks[0].Event)
	}

	h := hooks.Hooks[0].Handler
	assertEqual(t, "agent", h.Type)
	// Agent is json.RawMessage — verify it round-trips
	assertEqual(t, `"security-reviewer"`, string(h.Agent))
}

func TestHookRenderClaudeCodeIncludesTypeSpecificFields(t *testing.T) {
	t.Parallel()
	// Canonical manifest with all 4 types (neutral event/tool names)
	input := manifestJSON(
		`{"event":"before_tool_execute","matcher":"shell","handler":{"type":"command","command":"echo check","timeout":5}}`,
		`{"event":"before_tool_execute","matcher":"shell","handler":{"type":"http","url":"https://example.com/hook","headers":{"Authorization":"Bearer token"},"allowed_env_vars":["TOKEN"],"timeout":10}}`,
		`{"event":"before_tool_execute","matcher":"shell","handler":{"type":"prompt","prompt":"Is this safe?","model":"claude-sonnet-4-20250514","timeout":15}}`,
		`{"event":"before_tool_execute","matcher":"shell","handler":{"type":"agent","agent":"security-reviewer","timeout":30}}`,
	)

	result := convertHooksT(t, input, "", "claude-code")

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
	input := manifestJSON(`{"event":"before_tool_execute","handler":{"type":"http","url":"https://example.com/hook","timeout":10}}`)

	for _, slug := range []string{"gemini-cli", "copilot-cli", "kiro"} {
		t.Run(slug, func(t *testing.T) {
			result := convertHooksT(t, input, "", slug)

			// The only hook is http, which the target cannot hold.
			if result.Content != nil {
				t.Errorf("expected http hook dropped for %s, got: %s", slug, result.Content)
			}

			foundHTTPWarning := false
			for _, w := range result.Warnings {
				if containsStr(w, `"http"`) && containsStr(w, "not supported by "+slug) {
					foundHTTPWarning = true
					break
				}
			}
			if !foundHTTPWarning {
				t.Errorf("expected warning that http is not supported by %s, got: %v", slug, result.Warnings)
			}
		})
	}
}

// --- Gemini-only hook event tests ---

func TestGeminiOnlyEventsRoundtrip(t *testing.T) {
	// Gemini-only events (BeforeModel, AfterModel, BeforeToolSelection) should
	// survive import from Gemini CLI and export back to Gemini CLI.
	input := []byte(`{
		"hooks": {
			"BeforeModel": [
				{
					"hooks": [
						{"type": "command", "command": "echo before-model", "timeout": 5000}
					]
				}
			],
			"AfterModel": [
				{
					"hooks": [
						{"type": "command", "command": "echo after-model", "timeout": 3000}
					]
				}
			],
			"BeforeToolSelection": [
				{
					"hooks": [
						{"type": "command", "command": "echo before-tool-selection"}
					]
				}
			]
		}
	}`)

	// Canonical should preserve all 3 events
	canonical := decodeHooksT(t, input, "gemini-cli")
	events := map[string]bool{}
	for _, h := range canonical.Hooks {
		events[h.Event] = true
	}
	for _, event := range []string{"before_model", "after_model", "before_tool_selection"} {
		if !events[event] {
			t.Errorf("expected canonical to have event %q", event)
		}
	}

	// Render back to Gemini CLI — all 3 events should survive
	result := convertHooksT(t, input, "gemini-cli", "gemini-cli")

	out := string(result.Content)
	assertContains(t, out, "BeforeModel")
	assertContains(t, out, "AfterModel")
	assertContains(t, out, "BeforeToolSelection")
	assertContains(t, out, "echo before-model")
	assertContains(t, out, "echo after-model")
	assertContains(t, out, "echo before-tool-selection")

	// No warnings — these events are supported by Gemini
	for _, w := range result.Warnings {
		if containsStr(w, "not supported") {
			t.Errorf("unexpected warning: %s", w)
		}
	}
}

func TestGeminiOnlyEventsDroppedForOtherProviders(t *testing.T) {
	// Gemini-only events should be dropped with warnings when targeting non-Gemini providers.
	input := []byte(`{
		"hooks": {
			"BeforeModel": [
				{
					"hooks": [
						{"type": "command", "command": "echo model"}
					]
				}
			],
			"AfterModel": [
				{
					"hooks": [
						{"type": "command", "command": "echo after"}
					]
				}
			],
			"BeforeToolSelection": [
				{
					"hooks": [
						{"type": "command", "command": "echo select"}
					]
				}
			]
		}
	}`)

	for _, slug := range []string{"copilot-cli", "kiro"} {
		t.Run(slug, func(t *testing.T) {
			result := convertHooksT(t, input, "gemini-cli", slug)

			// All 3 events should generate warnings
			warnEvents := map[string]bool{}
			for _, w := range result.Warnings {
				for _, event := range []string{"before_model", "after_model", "before_tool_selection"} {
					if containsStr(w, event) && containsStr(w, "not supported") {
						warnEvents[event] = true
					}
				}
			}
			for _, event := range []string{"before_model", "after_model", "before_tool_selection"} {
				if !warnEvents[event] {
					t.Errorf("expected warning for %q on %s, got warnings: %v", event, slug, result.Warnings)
				}
			}
		})
	}
}

func TestGeminiOnlyEventsFlatFormat(t *testing.T) {
	t.Parallel()
	// Flat-format Gemini-only events should canonicalize correctly.
	input := `{"event":"BeforeModel","hooks":[{"type":"command","command":"echo model-check"}]}`
	hooks := decodeHooksT(t, []byte(input), "gemini-cli")
	// BeforeModel in Gemini maps to canonical "before_model"
	if hooks.Hooks[0].Event != "before_model" {
		t.Errorf("event not translated correctly: got %q, want %q", hooks.Hooks[0].Event, "before_model")
	}
}

// --- Structured output capability warnings ---

func TestStructuredOutputWarnings_ClaudeToGemini(t *testing.T) {
	t.Parallel()
	// Claude Code hooks with structured output -> Gemini should warn about all lost fields
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

	result := convertHooksT(t, input, "claude-code", "gemini-cli")

	// Should have a warning about structured output fields
	foundOutputWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, "structured hook output") && containsStr(w, "claude-code") && containsStr(w, "gemini-cli") {
			foundOutputWarning = true
			// Should mention lost fields (4 of 6 — Gemini now supports decision + system_message)
			assertContains(t, w, "updated_input")
			assertContains(t, w, "suppress_output")
			assertContains(t, w, "context")
			assertContains(t, w, "continue")
			// Gemini now supports these — should NOT be in warning
			assertNotContains(t, w, "decision")
			assertNotContains(t, w, "system_message")
			break
		}
	}
	if !foundOutputWarning {
		t.Fatalf("expected structured output warning, got: %v", result.Warnings)
	}
}

func TestStructuredOutputWarnings_ClaudeToCopilot(t *testing.T) {
	t.Parallel()
	// Copilot supports decision, so only 5 fields should be lost
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"hooks": [
						{"type": "command", "command": "echo check", "timeout": 5000}
					]
				}
			]
		}
	}`)

	result := convertHooksT(t, input, "claude-code", "copilot-cli")

	foundOutputWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, "structured hook output") {
			foundOutputWarning = true
			// Should mention lost fields but NOT decision (Copilot supports it)
			assertContains(t, w, "updated_input")
			assertContains(t, w, "suppress_output")
			assertNotContains(t, w, "decision")
			break
		}
	}
	if !foundOutputWarning {
		t.Fatalf("expected structured output warning, got: %v", result.Warnings)
	}
}

func TestStructuredOutputWarnings_GeminiToClaude(t *testing.T) {
	t.Parallel()
	// Gemini -> Claude: Gemini has no output capabilities, so nothing is lost
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

	result := convertHooksT(t, input, "gemini-cli", "claude-code")

	// Should NOT have structured output warnings
	for _, w := range result.Warnings {
		if containsStr(w, "structured hook output") {
			t.Fatalf("unexpected structured output warning for gemini->claude: %s", w)
		}
	}
}

func TestStructuredOutputWarnings_SameProvider(t *testing.T) {
	t.Parallel()
	// Claude -> Claude: no capability loss
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"hooks": [
						{"type": "command", "command": "echo check"}
					]
				}
			]
		}
	}`)

	result := convertHooksT(t, input, "claude-code", "claude-code")

	for _, w := range result.Warnings {
		if containsStr(w, "structured hook output") {
			t.Fatalf("unexpected structured output warning for same provider: %s", w)
		}
	}
}

func TestStructuredOutputWarnings_ClaudeToKiro(t *testing.T) {
	t.Parallel()
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

	result := convertHooksT(t, input, "claude-code", "kiro")

	foundOutputWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, "structured hook output") && containsStr(w, "kiro") {
			foundOutputWarning = true
			// All 6 fields should be listed as lost
			assertContains(t, w, "updated_input")
			assertContains(t, w, "decision")
			break
		}
	}
	if !foundOutputWarning {
		t.Fatalf("expected structured output warning for claude->kiro, got: %v", result.Warnings)
	}
}

func TestOutputFieldsLostWarnings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		source string
		target string
		want   int // expected number of lost fields
	}{
		{"claude->gemini: 4 lost (decision+system_message kept)", "claude-code", "gemini-cli", 4},
		{"claude->copilot: 5 lost (decision kept)", "claude-code", "copilot-cli", 5},
		{"claude->cursor: 5 lost (decision kept)", "claude-code", "cursor", 5},
		{"claude->crush: 3 lost (updated_input+decision+context kept)", "claude-code", "crush", 3},
		{"crush->claude: none lost", "crush", "claude-code", 0},
		{"claude->claude: none lost", "claude-code", "claude-code", 0},
		{"gemini->claude: none lost", "gemini-cli", "claude-code", 0},
		{"copilot->gemini: none lost", "copilot-cli", "gemini-cli", 0},
		{"copilot->claude: none lost", "copilot-cli", "claude-code", 0},
		{"gemini->gemini: none lost", "gemini-cli", "gemini-cli", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lost := OutputFieldsLostWarnings(tt.source, tt.target)
			if len(lost) != tt.want {
				t.Errorf("OutputFieldsLostWarnings(%s, %s) = %v (len %d), want len %d",
					tt.source, tt.target, lost, len(lost), tt.want)
			}
		})
	}

	// Exact-field check for crush: a count alone would pass with the wrong
	// three fields kept. updated_input must be among the kept fields
	// (crush shallow-merges it; see charmbracelet/crush docs/hooks/README.md).
	t.Run("claude->crush exact lost fields", func(t *testing.T) {
		lost := OutputFieldsLostWarnings("claude-code", "crush")
		want := []string{"suppress_output", "system_message", "continue"}
		if len(lost) != len(want) {
			t.Fatalf("lost = %v, want %v", lost, want)
		}
		for i, f := range want {
			if lost[i] != f {
				t.Errorf("lost[%d] = %q, want %q (full: %v)", i, lost[i], f, lost)
			}
		}
	})
}

func TestStructuredOutputWarnings_FlatFormat(t *testing.T) {
	t.Parallel()
	// A legacy flat hook.json carries its source provider through conversion
	input := []byte(`{
		"event": "PreToolUse",
		"matcher": "Bash",
		"hooks": [
			{"type": "command", "command": "echo check", "timeout": 5000}
		]
	}`)

	result := convertHooksT(t, input, "claude-code", "gemini-cli")

	foundOutputWarning := false
	for _, w := range result.Warnings {
		if containsStr(w, "structured hook output") {
			foundOutputWarning = true
			break
		}
	}
	if !foundOutputWarning {
		t.Fatalf("expected structured output warning for a flat hook, got: %v", result.Warnings)
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

	hooks := decodeHooksT(t, input, "claude-code")
	h := hooks.Hooks[0].Handler

	assertEqual(t, "http", h.Type)
	assertEqual(t, "https://example.com/check", h.URL)
	if h.Timeout != 5 { // ms -> s
		t.Errorf("timeout: got %d, want 5", h.Timeout)
	}
	assertEqual(t, "value", h.Headers["X-Custom"])
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

// TestClaudeHooksDevinRoundTrip covers the convert path: Devin uses Claude
// Code's event names but its own tool names and second-based timeouts.
func TestClaudeHooksDevinRoundTrip(t *testing.T) {
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

	result := convertHooksT(t, input, "claude-code", "devin")
	group := gjson.GetBytes(result.Content, "hooks.PreToolUse.0")
	if got := group.Get("matcher").String(); got != "exec" {
		t.Errorf("matcher: got %q, want exec; output: %s", got, result.Content)
	}
	if got := group.Get("hooks.0.timeout").Int(); got != 5 {
		t.Errorf("timeout: got %d, want 5 (seconds); output: %s", got, result.Content)
	}

	cc := convertHooksT(t, result.Content, "devin", "claude-code")
	ccGroup := gjson.GetBytes(cc.Content, "hooks.PreToolUse.0")
	if got := ccGroup.Get("matcher").String(); got != "Bash" {
		t.Errorf("round-trip matcher: got %q, want Bash", got)
	}
	if got := ccGroup.Get("hooks.0.timeout").Int(); got != 5000 {
		t.Errorf("round-trip timeout: got %d, want 5000 (ms)", got)
	}
}

func TestHooksConverterDevin_BareHooksV1File(t *testing.T) {
	// .devin/hooks.v1.json is the event map itself, with no "hooks" wrapper.
	input := []byte(`{"PreToolUse": [{"matcher": "exec", "hooks": [{"type": "command", "command": "./guard.sh", "timeout": 10}]}]}`)

	hooks := decodeHooksT(t, input, "devin")
	h := hooks.Hooks[0]
	if h.Event != "before_tool_execute" {
		t.Errorf("event: got %q, want before_tool_execute", h.Event)
	}
	if got := matcherOf(t, h); got != "shell" {
		t.Errorf("matcher: got %q, want shell", got)
	}
	if h.Handler.Command != "./guard.sh" {
		t.Errorf("command: got %q", h.Handler.Command)
	}
}

func TestHooksConverterDevin_RenderDropsUnsupportedFields(t *testing.T) {
	input := []byte(`{"hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [
		{"type": "command", "command": "echo hi", "timeout": 5000, "async": true, "statusMessage": "checking"}
	]}]}}`)

	result := convertHooksT(t, input, "claude-code", "devin")
	entry := gjson.GetBytes(result.Content, "hooks.PreToolUse.0.hooks.0")
	entry.ForEach(func(key, _ gjson.Result) bool {
		switch key.String() {
		case "type", "command", "timeout":
		default:
			t.Errorf("unexpected field %q in devin entry: %s", key.String(), entry.Raw)
		}
		return true
	})
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "async") && strings.Contains(w, "statusMessage dropped") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected dropped-fields warning, got %v", result.Warnings)
	}
}
