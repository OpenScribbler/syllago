package converter

import (
	"encoding/json"
	"strings"
	"testing"
)

// --- Task 1.1: Event translation tests ---

func TestTranslateEventToProvider(t *testing.T) {
	tests := []struct {
		name    string
		event   string
		slug    string
		want    string
		wantErr bool
	}{
		{"CC before_tool_execute", "before_tool_execute", "claude-code", "PreToolUse", false},
		{"Gemini before_tool_execute", "before_tool_execute", "gemini-cli", "BeforeTool", false},
		{"Copilot before_tool_execute", "before_tool_execute", "copilot-cli", "preToolUse", false},
		{"Kiro session_start", "session_start", "kiro", "agentSpawn", false},
		{"Cursor before_tool_execute", "before_tool_execute", "cursor", "PreToolUse", false},
		// Unsupported events return error (encode path is strict)
		{"Gemini subagent_start unsupported", "subagent_start", "gemini-cli", "", true},
		{"unknown event", "nonexistent_event", "claude-code", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TranslateEventToProvider(tt.event, tt.slug)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertEqual(t, tt.want, got)
		})
	}
}

func TestTranslateEventFromProvider(t *testing.T) {
	tests := []struct {
		name         string
		event        string
		slug         string
		wantCanon    string
		wantWarnings int
	}{
		{"CC PreToolUse", "PreToolUse", "claude-code", "before_tool_execute", 0},
		{"Gemini BeforeTool", "BeforeTool", "gemini-cli", "before_tool_execute", 0},
		{"Copilot preToolUse", "preToolUse", "copilot-cli", "before_tool_execute", 0},
		{"Kiro agentSpawn", "agentSpawn", "kiro", "session_start", 0},
		// Unknown event passes through for forward compat, emits warning
		{"unknown CC event", "NewFutureEvent", "claude-code", "NewFutureEvent", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, warnings := TranslateEventFromProvider(tt.event, tt.slug)
			assertEqual(t, tt.wantCanon, got)
			if len(warnings) != tt.wantWarnings {
				t.Errorf("warnings: got %d, want %d: %v", len(warnings), tt.wantWarnings, warnings)
			}
		})
	}
}

// --- Task 1.2: Timeout translation tests ---

func TestTranslateTimeoutToProvider(t *testing.T) {
	tests := []struct {
		name    string
		seconds int
		slug    string
		want    int
	}{
		{"CC 5s -> 5000ms", 5, "claude-code", 5000},
		{"Gemini 10s -> 10000ms", 10, "gemini-cli", 10000},
		{"Cursor 3s -> 3000ms", 3, "cursor", 3000},
		{"Kiro 7s -> 7000ms", 7, "kiro", 7000},
		// Copilot uses seconds — no conversion
		{"Copilot 5s -> 5s", 5, "copilot-cli", 5},
		// Zero timeout passes through
		{"zero CC", 0, "claude-code", 0},
		{"zero Copilot", 0, "copilot-cli", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateTimeoutToProvider(tt.seconds, tt.slug)
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestTranslateTimeoutFromProvider(t *testing.T) {
	tests := []struct {
		name  string
		value int
		slug  string
		want  int
	}{
		{"CC 5000ms -> 5s", 5000, "claude-code", 5},
		{"Gemini 3000ms -> 3s", 3000, "gemini-cli", 3},
		{"Cursor 10000ms -> 10s", 10000, "cursor", 10},
		{"Kiro 7000ms -> 7s", 7000, "kiro", 7},
		// Copilot uses seconds — no conversion
		{"Copilot 5s -> 5s", 5, "copilot-cli", 5},
		// Zero passes through
		{"zero CC", 0, "claude-code", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateTimeoutFromProvider(tt.value, tt.slug)
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

// --- Task 1.3: Matcher translation tests ---

func TestTranslateMatcherToProvider_BareString(t *testing.T) {
	matcher, _ := json.Marshal("shell")
	result, warnings := TranslateMatcherToProvider(matcher, "claude-code")
	var got string
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("expected string result: %v", err)
	}
	assertEqual(t, "Bash", got)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestTranslateMatcherToProvider_BareString_Gemini(t *testing.T) {
	matcher, _ := json.Marshal("file_read")
	result, warnings := TranslateMatcherToProvider(matcher, "gemini-cli")
	var got string
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("expected string result: %v", err)
	}
	assertEqual(t, "read_file", got)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestTranslateMatcherToProvider_MCPObject(t *testing.T) {
	matcher := json.RawMessage(`{"mcp":{"server":"github","tool":"create_issue"}}`)
	result, warnings := TranslateMatcherToProvider(matcher, "claude-code")
	// Should produce bare string "mcp__github__create_issue"
	var got string
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("expected string result: %v", err)
	}
	assertEqual(t, "mcp__github__create_issue", got)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestTranslateMatcherToProvider_MCPObject_Copilot(t *testing.T) {
	matcher := json.RawMessage(`{"mcp":{"server":"github","tool":"create_issue"}}`)
	result, warnings := TranslateMatcherToProvider(matcher, "copilot-cli")
	var got string
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("expected string result: %v", err)
	}
	assertEqual(t, "github/create_issue", got)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestTranslateMatcherToProvider_MCPObject_Gemini(t *testing.T) {
	matcher := json.RawMessage(`{"mcp":{"server":"github","tool":"create_issue"}}`)
	result, warnings := TranslateMatcherToProvider(matcher, "gemini-cli")
	var got string
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("expected string result: %v", err)
	}
	assertEqual(t, "mcp_github_create_issue", got)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestTranslateMatcherToProvider_PatternObject(t *testing.T) {
	matcher := json.RawMessage(`{"pattern":"file_(read|write)"}`)
	result, warnings := TranslateMatcherToProvider(matcher, "claude-code")
	// Pattern objects: flattened to bare regex string with a warning
	var got string
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("expected string result: %v", err)
	}
	assertEqual(t, "file_(read|write)", got)
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning for pattern passthrough, got %d: %v", len(warnings), warnings)
	}
	// Verify the warning message describes the lossy behavior
	if len(warnings) == 1 && warnings[0].Description == "" {
		t.Error("warning should have a description explaining the lossy pattern flattening")
	}
}

func TestTranslateMatcherToProvider_Array(t *testing.T) {
	matcher := json.RawMessage(`["shell","file_read"]`)
	result, warnings := TranslateMatcherToProvider(matcher, "claude-code")
	var got []string
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("expected array result: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(got))
	}
	assertEqual(t, "Bash", got[0])
	assertEqual(t, "Read", got[1])
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestTranslateMatcherToProvider_NilMatcher(t *testing.T) {
	result, warnings := TranslateMatcherToProvider(nil, "claude-code")
	if result != nil {
		t.Errorf("expected nil result, got %s", string(result))
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestTranslateMatcherFromProvider_BareString(t *testing.T) {
	matcher, _ := json.Marshal("Bash")
	result, warnings := TranslateMatcherFromProvider(matcher, "claude-code")
	var got string
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("expected string result: %v", err)
	}
	assertEqual(t, "shell", got)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestTranslateMatcherFromProvider_MCPString(t *testing.T) {
	// CC MCP format: "mcp__github__create_issue" -> canonical MCP object
	matcher, _ := json.Marshal("mcp__github__create_issue")
	result, warnings := TranslateMatcherFromProvider(matcher, "claude-code")
	// Should produce canonical MCP object
	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("expected object result: %v", err)
	}
	if obj["mcp"] == nil {
		t.Errorf("expected canonical MCP object with 'mcp' key, got %s", string(result))
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestTranslateMatcherFromProvider_MCPString_Copilot(t *testing.T) {
	// Copilot MCP format: "github/create_issue" -> canonical MCP object
	matcher, _ := json.Marshal("github/create_issue")
	result, warnings := TranslateMatcherFromProvider(matcher, "copilot-cli")
	var obj map[string]any
	if err := json.Unmarshal(result, &obj); err != nil {
		t.Fatalf("expected object result: %v", err)
	}
	if obj["mcp"] == nil {
		t.Errorf("expected canonical MCP object with 'mcp' key, got %s", string(result))
	}
	mcpMap := obj["mcp"].(map[string]any)
	assertEqual(t, "github", mcpMap["server"].(string))
	assertEqual(t, "create_issue", mcpMap["tool"].(string))
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestTranslateMatcherFromProvider_Array(t *testing.T) {
	matcher := json.RawMessage(`["Bash","Read"]`)
	result, warnings := TranslateMatcherFromProvider(matcher, "claude-code")
	var got []json.RawMessage
	if err := json.Unmarshal(result, &got); err != nil {
		t.Fatalf("expected array result: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(got))
	}
	var s0, s1 string
	json.Unmarshal(got[0], &s0)
	json.Unmarshal(got[1], &s1)
	assertEqual(t, "shell", s0)
	assertEqual(t, "file_read", s1)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestTranslateMatcherFromProvider_NilMatcher(t *testing.T) {
	result, warnings := TranslateMatcherFromProvider(nil, "claude-code")
	if result != nil {
		t.Errorf("expected nil result, got %s", string(result))
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

// --- Task 1.4: Handler type and LLM wrapper tests ---

func TestTranslateHandlerType_CommandAlwaysKept(t *testing.T) {
	h := HookHandler{Type: "command", Command: "echo check"}
	result, warnings, keep := TranslateHandlerType(h, "gemini-cli", nil)
	if !keep {
		t.Error("command handler should always be kept")
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	assertEqual(t, "command", result.Type)
}

func TestTranslateHandlerType_EmptyTypeDefaultsToCommand(t *testing.T) {
	h := HookHandler{Command: "echo check"}
	_, warnings, keep := TranslateHandlerType(h, "gemini-cli", nil)
	if !keep {
		t.Error("empty type (defaults to command) should always be kept")
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestTranslateHandlerType_PromptDroppedForNonCC(t *testing.T) {
	h := HookHandler{Type: "prompt", Prompt: "Is this safe?"}
	_, warnings, keep := TranslateHandlerType(h, "gemini-cli", nil)
	if keep {
		t.Error("prompt handler should not be kept for gemini-cli")
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning for dropped prompt hook, got %d", len(warnings))
	}
}

func TestTranslateHandlerType_PromptKeptForCC(t *testing.T) {
	h := HookHandler{Type: "prompt", Prompt: "Is this safe?"}
	result, warnings, keep := TranslateHandlerType(h, "claude-code", nil)
	if !keep {
		t.Error("prompt handler should be kept for claude-code")
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	assertEqual(t, "prompt", result.Type)
}

func TestTranslateHandlerType_AgentDroppedForNonCC(t *testing.T) {
	h := HookHandler{Type: "agent", Agent: json.RawMessage(`{"task":"review"}`)}
	_, warnings, keep := TranslateHandlerType(h, "kiro", nil)
	if keep {
		t.Error("agent handler should not be kept for kiro")
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning for dropped agent hook, got %d", len(warnings))
	}
}

func TestTranslateHandlerType_HTTPDroppedForNonCC(t *testing.T) {
	h := HookHandler{Type: "http", URL: "https://example.com"}
	_, warnings, keep := TranslateHandlerType(h, "kiro", nil)
	if keep {
		t.Error("http handler should not be kept for kiro")
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning for dropped http hook, got %d", len(warnings))
	}
}

func TestTranslateHandlerType_HTTPKeptForCC(t *testing.T) {
	h := HookHandler{Type: "http", URL: "https://example.com"}
	result, warnings, keep := TranslateHandlerType(h, "claude-code", nil)
	if !keep {
		t.Error("http handler should be kept for claude-code")
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	assertEqual(t, "http", result.Type)
}

func TestTranslateHandlerType_UnknownProvider(t *testing.T) {
	// Unknown provider with command type should still keep
	h := HookHandler{Type: "command", Command: "echo check"}
	_, _, keep := TranslateHandlerType(h, "unknown-provider", nil)
	if !keep {
		t.Error("command handler should be kept even for unknown provider")
	}

	// Unknown provider with prompt type should drop
	h2 := HookHandler{Type: "prompt", Prompt: "check this"}
	_, warnings, keep2 := TranslateHandlerType(h2, "unknown-provider", nil)
	if keep2 {
		t.Error("prompt handler should be dropped for unknown provider")
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(warnings))
	}
}

// --- Degradation strategy tests ---

func TestTranslateHandlerType_DegradationBlock(t *testing.T) {
	// Author says "block" — conversion should fail with error severity
	h := HookHandler{Type: "prompt", Prompt: "Security review"}
	degradation := map[string]string{"llm_evaluated": "block"}
	_, warnings, keep := TranslateHandlerType(h, "gemini-cli", degradation)
	if keep {
		t.Error("should not keep hook when degradation=block")
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if warnings[0].Severity != "error" {
		t.Errorf("expected severity=error for block strategy, got %q", warnings[0].Severity)
	}
	assertContains(t, warnings[0].Description, "blocked by degradation policy")
}

func TestTranslateHandlerType_DegradationExclude(t *testing.T) {
	// Author says "exclude" — drop with warning
	h := HookHandler{Type: "prompt", Prompt: "Check this"}
	degradation := map[string]string{"llm_evaluated": "exclude"}
	_, warnings, keep := TranslateHandlerType(h, "gemini-cli", degradation)
	if keep {
		t.Error("should not keep hook when degradation=exclude")
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if warnings[0].Severity != "warning" {
		t.Errorf("expected severity=warning for exclude strategy, got %q", warnings[0].Severity)
	}
	assertContains(t, warnings[0].Description, "exclude")
}

func TestTranslateHandlerType_DegradationWarn(t *testing.T) {
	// Author says "warn" — currently behaves like exclude with suggestion
	h := HookHandler{Type: "http", URL: "https://example.com/hook"}
	degradation := map[string]string{"http_handler": "warn"}
	_, warnings, keep := TranslateHandlerType(h, "gemini-cli", degradation)
	if keep {
		t.Error("should not keep hook (full warn with wrapper generation deferred)")
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if warnings[0].Severity != "warning" {
		t.Errorf("expected severity=warning, got %q", warnings[0].Severity)
	}
	if warnings[0].Suggestion == "" {
		t.Error("expected suggestion for warn strategy")
	}
}

func TestTranslateHandlerType_DefaultDegradation_LLMExclude(t *testing.T) {
	// No author degradation specified — llm_evaluated defaults to "exclude"
	h := HookHandler{Type: "prompt", Prompt: "Check this"}
	_, warnings, keep := TranslateHandlerType(h, "gemini-cli", nil)
	if keep {
		t.Error("should not keep prompt hook on gemini (default: exclude)")
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	// Default is exclude, not block — so severity should be warning, not error
	if warnings[0].Severity != "warning" {
		t.Errorf("expected severity=warning for default exclude, got %q", warnings[0].Severity)
	}
}

func TestTranslateHandlerType_DegradationBlock_HTTP(t *testing.T) {
	// Author blocks HTTP degradation
	h := HookHandler{Type: "http", URL: "https://critical-webhook.example.com"}
	degradation := map[string]string{"http_handler": "block"}
	_, warnings, keep := TranslateHandlerType(h, "cursor", degradation)
	if keep {
		t.Error("should not keep hook when degradation=block")
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if warnings[0].Severity != "error" {
		t.Errorf("expected severity=error for block, got %q", warnings[0].Severity)
	}
}

func TestGenerateLLMWrapperScript(t *testing.T) {
	h := CanonicalHook{
		Handler: HookHandler{Type: "prompt", Prompt: "Is this command safe?"},
	}
	name, content := GenerateLLMWrapperScript(h, "gemini-cli", "before_tool_execute", 0)
	if name == "" {
		t.Error("expected non-empty script name")
	}
	if len(content) == 0 {
		t.Error("expected non-empty script content")
	}
	script := string(content)
	if !strings.Contains(script, "#!/bin/bash") {
		t.Error("script should start with bash shebang")
	}
	if !strings.Contains(script, "Is this command safe?") {
		t.Error("script should include the prompt text")
	}
	if !strings.Contains(script, "gemini") {
		t.Error("script should reference the gemini CLI")
	}
}

func TestGenerateLLMWrapperScript_CC(t *testing.T) {
	h := CanonicalHook{
		Handler: HookHandler{Type: "agent", Command: "review code"},
	}
	name, content := GenerateLLMWrapperScript(h, "claude-code", "after_tool_execute", 1)
	if name == "" {
		t.Error("expected non-empty script name")
	}
	script := string(content)
	if !strings.Contains(script, "claude") {
		t.Error("script should reference the claude CLI")
	}
	_ = script // avoid unused
}

// --- Task 1.5: Structured output loss tests ---

func TestCheckStructuredOutputLoss(t *testing.T) {
	// Claude -> Gemini: fields lost (decision+system_message kept, 4 others lost)
	warnings := CheckStructuredOutputLoss("claude-code", "gemini-cli")
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	if warnings[0].Capability == "" {
		t.Error("warning should have capability set")
	}
	if warnings[0].Severity != "warning" {
		t.Errorf("expected severity 'warning', got %q", warnings[0].Severity)
	}

	// Claude -> Claude: no loss
	noWarnings := CheckStructuredOutputLoss("claude-code", "claude-code")
	if len(noWarnings) != 0 {
		t.Errorf("expected no warnings for same provider, got: %v", noWarnings)
	}

	// Empty source: no warnings
	emptyWarnings := CheckStructuredOutputLoss("", "gemini-cli")
	if len(emptyWarnings) != 0 {
		t.Errorf("expected no warnings for empty source, got: %v", emptyWarnings)
	}
}

// --- Additional edge cases ---

func TestTranslateMCPToProvider(t *testing.T) {
	tests := []struct {
		name   string
		server string
		tool   string
		slug   string
		want   string
	}{
		{"CC", "github", "create_issue", "claude-code", "mcp__github__create_issue"},
		{"Gemini", "github", "create_issue", "gemini-cli", "mcp_github_create_issue"},
		{"Copilot", "github", "create_issue", "copilot-cli", "github/create_issue"},
		{"Cursor", "github", "create_issue", "cursor", "github__create_issue"},
		{"Kiro", "github", "create_issue", "kiro", "mcp__github__create_issue"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateMCPToProvider(tt.server, tt.tool, tt.slug)
			assertEqual(t, tt.want, got)
		})
	}
}

func TestTranslateMCPFromProvider(t *testing.T) {
	tests := []struct {
		name     string
		mcpName  string
		slug     string
		wantSrv  string
		wantTool string
		wantOK   bool
	}{
		{"CC", "mcp__github__create_issue", "claude-code", "github", "create_issue", true},
		{"Gemini", "mcp_github_create_issue", "gemini-cli", "github", "create_issue", true},
		{"Copilot", "github/create_issue", "copilot-cli", "github", "create_issue", true},
		{"Cursor", "github__create_issue", "cursor", "github", "create_issue", true},
		{"not MCP", "Bash", "claude-code", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, tool, ok := TranslateMCPFromProvider(tt.mcpName, tt.slug)
			if ok != tt.wantOK {
				t.Errorf("ok: got %v, want %v", ok, tt.wantOK)
			}
			if ok {
				assertEqual(t, tt.wantSrv, srv)
				assertEqual(t, tt.wantTool, tool)
			}
		})
	}
}
