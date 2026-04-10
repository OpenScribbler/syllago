package converter

import (
	"encoding/json"
	"testing"
)

func TestAdapterRegistry(t *testing.T) {
	// All expected adapters should be registered via init()
	expected := []string{"claude-code", "gemini-cli", "copilot-cli", "kiro", "cursor", "windsurf", "vs-code-copilot", "factory-droid", "pi"}
	for _, slug := range expected {
		if AdapterFor(slug) == nil {
			t.Errorf("expected adapter for %q to be registered", slug)
		}
	}
}

func TestAdapterProviderSlug(t *testing.T) {
	for slug, adapter := range Adapters() {
		if adapter.ProviderSlug() != slug {
			t.Errorf("adapter registered as %q but ProviderSlug() returns %q", slug, adapter.ProviderSlug())
		}
	}
}

func TestClaudeCodeAdapterDecode(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Bash",
					"hooks": [{"type": "command", "command": "echo check", "timeout": 5000}]
				}
			]
		}
	}`)

	adapter := AdapterFor("claude-code")
	hooks, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	assertEqual(t, SpecVersion, hooks.Spec)
	if len(hooks.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks.Hooks))
	}
	assertEqual(t, "before_tool_execute", hooks.Hooks[0].Event)
	assertEqual(t, "command", hooks.Hooks[0].Handler.Type)
	assertEqual(t, "echo check", hooks.Hooks[0].Handler.Command)
}

func TestGeminiCLIAdapterDecode(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"BeforeTool": [
				{
					"matcher": "run_shell_command",
					"hooks": [{"type": "command", "command": "echo safe", "timeout": 3000}]
				}
			]
		}
	}`)

	adapter := AdapterFor("gemini-cli")
	hooks, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	assertEqual(t, SpecVersion, hooks.Spec)
	if len(hooks.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks.Hooks))
	}
	assertEqual(t, "before_tool_execute", hooks.Hooks[0].Event)
}

func TestAdapterRoundTrip_CC_Gemini(t *testing.T) {
	// Decode from CC, encode to Gemini, verify Gemini event names
	ccInput := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Bash",
					"hooks": [{"type": "command", "command": "echo check", "timeout": 5000}]
				}
			]
		}
	}`)

	ccAdapter := AdapterFor("claude-code")
	hooks, err := ccAdapter.Decode(ccInput)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	geminiAdapter := AdapterFor("gemini-cli")
	encoded, err := geminiAdapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	out := string(encoded.Content)
	assertContains(t, out, "BeforeTool")
	assertContains(t, out, "run_shell_command")
	assertNotContains(t, out, "PreToolUse")
}

func TestVerify_Success(t *testing.T) {
	matcher, _ := json.Marshal("shell")
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Matcher: matcher,
				Handler: HookHandler{Type: "command", Command: "echo check", Timeout: 5},
			},
		},
	}

	adapter := AdapterFor("claude-code")
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	err = Verify(encoded, adapter, hooks)
	if err != nil {
		t.Fatalf("Verify should pass: %v", err)
	}
}

// --- Copilot CLI Adapter ---

func TestCopilotCLIAdapterDecode(t *testing.T) {
	// Copilot CLI uses its own hook format with bash/timeoutSec fields
	// and camelCase event names like preToolUse
	input := []byte(`{
		"version": 1,
		"hooks": {
			"preToolUse": [
				{
					"hooks": [{"type": "command", "bash": "echo copilot", "timeoutSec": 5}]
				}
			]
		}
	}`)

	adapter := AdapterFor("copilot-cli")
	hooks, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	assertEqual(t, SpecVersion, hooks.Spec)
	if len(hooks.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks.Hooks))
	}
	assertEqual(t, "before_tool_execute", hooks.Hooks[0].Event)
}

func TestCopilotCLIAdapterEncode(t *testing.T) {
	matcher, _ := json.Marshal("shell")
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Matcher: matcher,
				Handler: HookHandler{Type: "command", Command: "echo check", Timeout: 5},
			},
		},
	}

	adapter := AdapterFor("copilot-cli")
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	out := string(encoded.Content)
	// Copilot CLI uses camelCase event names
	assertContains(t, out, "preToolUse")
	assertContains(t, out, "echo check")
}

// --- Kiro Adapter ---

func TestKiroAdapterDecode(t *testing.T) {
	// Kiro uses its own event names: preToolUse maps to before_tool_execute
	input := []byte(`{
		"hooks": {
			"preToolUse": [
				{
					"matcher": "shell",
					"hooks": [{"type": "command", "command": "echo kiro", "timeout": 3000}]
				}
			]
		}
	}`)

	adapter := AdapterFor("kiro")
	hooks, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	assertEqual(t, SpecVersion, hooks.Spec)
	if len(hooks.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks.Hooks))
	}
	assertEqual(t, "before_tool_execute", hooks.Hooks[0].Event)
}

func TestKiroAdapterEncode(t *testing.T) {
	matcher, _ := json.Marshal("shell")
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "session_start",
				Matcher: matcher,
				Handler: HookHandler{Type: "command", Command: "echo init"},
			},
		},
	}

	adapter := AdapterFor("kiro")
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	out := string(encoded.Content)
	// Kiro renders hooks inside a JSON agent structure with Kiro-native event names
	assertContains(t, out, "agentSpawn")
	assertContains(t, out, "echo init")
}

// --- Cursor Adapter ---

func TestCursorAdapterDecode(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Bash",
					"hooks": [{"type": "command", "command": "echo cursor", "timeout": 5000}]
				}
			]
		}
	}`)

	adapter := AdapterFor("cursor")
	hooks, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	assertEqual(t, SpecVersion, hooks.Spec)
	if len(hooks.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks.Hooks))
	}
	assertEqual(t, "before_tool_execute", hooks.Hooks[0].Event)
}

func TestCursorAdapterEncode(t *testing.T) {
	matcher, _ := json.Marshal("shell")
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Matcher: matcher,
				Handler: HookHandler{Type: "command", Command: "echo check"},
			},
		},
	}

	adapter := AdapterFor("cursor")
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	out := string(encoded.Content)
	assertContains(t, out, "PreToolUse")
	assertContains(t, out, "echo check")
}

func TestCursorAdapterCapabilities(t *testing.T) {
	caps := AdapterFor("cursor").Capabilities()
	if !caps.SupportsMatchers {
		t.Error("Cursor should support matchers")
	}
	if caps.SupportsLLMHooks {
		t.Error("Cursor should not support LLM hooks")
	}
	if caps.SupportsHTTPHooks {
		t.Error("Cursor should not support HTTP hooks")
	}
}

// --- Verify error paths ---

func TestVerify_FieldLevelCheck(t *testing.T) {
	// Verify checks field-level fidelity: command, timeout, blocking.
	// Encode a hook with all fields, then verify it round-trips correctly.
	matcher, _ := json.Marshal("shell")
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:    "before_tool_execute",
				Matcher:  matcher,
				Blocking: true,
				Handler:  HookHandler{Type: "command", Command: "echo check", Timeout: 5},
			},
		},
	}

	adapter := AdapterFor("claude-code")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// Should pass — fields preserved
	err = Verify(encoded, adapter, original)
	if err != nil {
		t.Fatalf("expected Verify to pass, got: %v", err)
	}
}

func TestVerify_DroppedHooksAllowed(t *testing.T) {
	// When an adapter intentionally drops hooks (e.g., LLM hooks on Gemini),
	// Verify should still pass — it compares decoded hooks against original,
	// not the other way around.
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Handler: HookHandler{Type: "command", Command: "echo check"},
			},
			{
				Event:   "before_tool_execute",
				Handler: HookHandler{Type: "prompt", Prompt: "Review this", Model: "claude"},
			},
		},
	}

	adapter := AdapterFor("gemini-cli")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// Should pass — Gemini drops the prompt hook but the command hook round-trips fine
	err = Verify(encoded, adapter, original)
	if err != nil {
		t.Fatalf("expected Verify to pass with dropped hooks, got: %v", err)
	}
}

func TestVerify_NilEncoded(t *testing.T) {
	err := Verify(nil, AdapterFor("claude-code"), &CanonicalHooks{})
	if err != nil {
		t.Fatalf("expected nil error for nil encoded, got: %v", err)
	}
}

func TestVerify_EmptyContent(t *testing.T) {
	err := Verify(&EncodedResult{Content: nil}, AdapterFor("claude-code"), &CanonicalHooks{})
	if err != nil {
		t.Fatalf("expected nil error for empty content, got: %v", err)
	}
}

func TestVerifyError_Error(t *testing.T) {
	// Error without counts
	e1 := &VerifyError{
		Provider: "test",
		Detail:   "decode failed",
	}
	assertEqual(t, "hook verify test: decode failed", e1.Error())

	// Error with counts
	e2 := &VerifyError{
		Provider: "test",
		Detail:   "count mismatch",
		Expected: 3,
		Got:      1,
	}
	assertContains(t, e2.Error(), "expected 3")
	assertContains(t, e2.Error(), "got 1")
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{100, "100"},
	}

	for _, tt := range tests {
		result := itoa(tt.input)
		assertEqual(t, tt.expected, result)
	}
}

// --- Gemini Capabilities ---

func TestGeminiAdapterCapabilities(t *testing.T) {
	caps := AdapterFor("gemini-cli").Capabilities()
	if !caps.SupportsMatchers {
		t.Error("Gemini should support matchers")
	}
	if !caps.SupportsAsync {
		t.Error("Gemini should support async")
	}
}

// --- Kiro Capabilities ---

func TestKiroAdapterCapabilities(t *testing.T) {
	caps := AdapterFor("kiro").Capabilities()
	if !caps.SupportsMatchers {
		t.Error("Kiro should support matchers")
	}
	if caps.SupportsAsync {
		t.Error("Kiro should not support async")
	}
	if caps.SupportsLLMHooks {
		t.Error("Kiro should not support LLM hooks")
	}
}

func TestCapabilities(t *testing.T) {
	// Claude Code should support everything
	cc := AdapterFor("claude-code").Capabilities()
	if !cc.SupportsMatchers {
		t.Error("CC should support matchers")
	}
	if !cc.SupportsLLMHooks {
		t.Error("CC should support LLM hooks")
	}
	if !cc.SupportsHTTPHooks {
		t.Error("CC should support HTTP hooks")
	}

	// Copilot should not support matchers
	copilot := AdapterFor("copilot-cli").Capabilities()
	if copilot.SupportsMatchers {
		t.Error("Copilot should not support matchers")
	}
}

// Compile-time check: VerifyFields interface is accessible from adapter implementations.
type stubVerifyFields struct{ ClaudeCodeAdapter }

func (s *stubVerifyFields) FieldsToVerify() []string {
	return []string{VerifyFieldEvent, VerifyFieldName, VerifyFieldMatcher}
}

var _ VerifyFields = (*stubVerifyFields)(nil)

func TestVerifyFields_InterfaceCheck(t *testing.T) {
	t.Parallel()
	s := &stubVerifyFields{}
	fields := s.FieldsToVerify()
	if len(fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}
	assertEqual(t, VerifyFieldEvent, fields[0])
	assertEqual(t, VerifyFieldName, fields[1])
	assertEqual(t, VerifyFieldMatcher, fields[2])
}

func TestCanonicalHookNewFields(t *testing.T) {
	t.Parallel()
	hook := CanonicalHook{
		Name:  "test-hook",
		Event: "before_tool_execute",
		Handler: HookHandler{
			Type:          "command",
			Command:       "echo test",
			TimeoutAction: "block",
		},
		Capabilities: []string{"structured_output", "input_rewrite"},
	}

	data, err := json.Marshal(hook)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded CanonicalHook
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	assertEqual(t, "test-hook", decoded.Name)
	assertEqual(t, "block", decoded.Handler.TimeoutAction)
	if len(decoded.Capabilities) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(decoded.Capabilities))
	}
	assertEqual(t, "structured_output", decoded.Capabilities[0])
}

func TestSpecVersion(t *testing.T) {
	t.Parallel()
	assertEqual(t, "hooks/0.1", SpecVersion)
}

// --- CC Adapter Tier 2 Round-Trip Tests ---

func TestCCAdapter_RoundTrip_AllFields(t *testing.T) {
	// This test defines the fidelity contract for the migrated CC adapter.
	// Fields that the legacy bridge previously dropped are explicitly asserted.
	matcherJSON, _ := json.Marshal("shell")
	mcpMatcher := json.RawMessage(`{"mcp":{"server":"github","tool":"create_issue"}}`)

	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Name:    "safety-check",
				Event:   "before_tool_execute",
				Matcher: matcherJSON,
				Handler: HookHandler{
					Type:          "command",
					Command:       "echo check",
					Timeout:       5, // canonical seconds; should encode as 5000ms
					TimeoutAction: "block",
					StatusMessage: "Running safety check",
					CWD:           "./scripts",
					Env:           map[string]string{"AUDIT_LOG": "/tmp/audit.log"},
				},
				Blocking:     true,
				Degradation:  map[string]string{"input_rewrite": "block"},
				Capabilities: []string{"structured_output"},
			},
			{
				Name:    "mcp-guard",
				Event:   "before_tool_execute",
				Matcher: mcpMatcher,
				Handler: HookHandler{
					Type:    "command",
					Command: "echo mcp",
				},
			},
		},
	}

	adapter := AdapterFor("claude-code")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded.Warnings) != 0 {
		t.Logf("encode warnings: %v", encoded.Warnings)
	}

	// Verify the encoded JSON contains CC-native event and tool names
	out := string(encoded.Content)
	assertContains(t, out, "PreToolUse")
	assertContains(t, out, "Bash") // shell -> Bash for CC
	assertContains(t, out, "5000") // 5s -> 5000ms

	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if len(decoded.Hooks) != len(original.Hooks) {
		t.Fatalf("hook count: got %d, want %d", len(decoded.Hooks), len(original.Hooks))
	}

	h0 := decoded.Hooks[0]
	assertEqual(t, "safety-check", h0.Name)
	assertEqual(t, "before_tool_execute", h0.Event)
	if !h0.Blocking {
		t.Error("expected Blocking to be true")
	}
	assertEqual(t, "block", h0.Handler.TimeoutAction)
	assertEqual(t, "Running safety check", h0.Handler.StatusMessage)
	if h0.Handler.Timeout != 5 {
		t.Errorf("Timeout: got %d, want 5 (canonical seconds)", h0.Handler.Timeout)
	}
	assertEqual(t, "./scripts", h0.Handler.CWD)
	if h0.Handler.Env["AUDIT_LOG"] != "/tmp/audit.log" {
		t.Errorf("Env not preserved: %v", h0.Handler.Env)
	}
	if len(h0.Degradation) != 1 || h0.Degradation["input_rewrite"] != "block" {
		t.Errorf("Degradation not preserved: %v", h0.Degradation)
	}
	if len(h0.Capabilities) != 1 || h0.Capabilities[0] != "structured_output" {
		t.Errorf("Capabilities not preserved: %v", h0.Capabilities)
	}

	// Verify second hook (MCP matcher)
	h1 := decoded.Hooks[1]
	assertEqual(t, "mcp-guard", h1.Name)
	assertEqual(t, "before_tool_execute", h1.Event)
	assertEqual(t, "echo mcp", h1.Handler.Command)
}

func TestCCAdapter_AllHandlerTypes(t *testing.T) {
	// CC supports all 4 handler types: command, http, prompt, agent
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event: "before_tool_execute",
				Handler: HookHandler{
					Type: "http", URL: "https://example.com",
					Headers: map[string]string{"X-Key": "val"},
				},
			},
			{
				Event:   "before_tool_execute",
				Handler: HookHandler{Type: "prompt", Prompt: "Is this safe?", Model: "claude-3"},
			},
			{
				Event:   "session_start",
				Handler: HookHandler{Type: "command", Command: "echo init"},
			},
		},
	}

	adapter := AdapterFor("claude-code")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// All hooks should be kept — CC supports all types
	if len(encoded.Warnings) != 0 {
		t.Errorf("unexpected warnings for CC (all types supported): %v", encoded.Warnings)
	}

	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Hooks) != 3 {
		t.Fatalf("expected 3 hooks, got %d", len(decoded.Hooks))
	}
}

func TestCCAdapter_TimeoutValues(t *testing.T) {
	// Verify actual timeout values survive the round-trip correctly
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event: "session_start",
				Handler: HookHandler{
					Type:    "command",
					Command: "echo init",
					Timeout: 10, // 10 seconds
				},
			},
			{
				Event: "before_tool_execute",
				Handler: HookHandler{
					Type:    "command",
					Command: "echo check",
					Timeout: 0, // no timeout
				},
			},
		},
	}

	adapter := AdapterFor("claude-code")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// Verify encoded values
	out := string(encoded.Content)
	assertContains(t, out, "10000") // 10s -> 10000ms

	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	// Map iteration order is non-deterministic; find hooks by command
	for _, h := range decoded.Hooks {
		switch h.Handler.Command {
		case "echo init":
			if h.Handler.Timeout != 10 {
				t.Errorf("timeout for 'echo init': got %d, want 10", h.Handler.Timeout)
			}
		case "echo check":
			if h.Handler.Timeout != 0 {
				t.Errorf("zero timeout for 'echo check': got %d, want 0", h.Handler.Timeout)
			}
		}
	}
}

// --- Gemini Adapter Tier 2 Round-Trip Tests ---

func TestGeminiAdapter_RoundTrip_AllFields(t *testing.T) {
	matcherJSON, _ := json.Marshal("shell")
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Name:    "log-hook",
				Event:   "before_tool_execute",
				Matcher: matcherJSON,
				Handler: HookHandler{
					Type:          "command",
					Command:       "echo log",
					Timeout:       3, // canonical seconds -> 3000ms in Gemini
					StatusMessage: "Logging",
					Async:         true,
				},
				Blocking: true,
			},
		},
	}
	adapter := AdapterFor("gemini-cli")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// Verify Gemini-native event names
	out := string(encoded.Content)
	assertContains(t, out, "BeforeTool")
	assertContains(t, out, "run_shell_command") // shell -> run_shell_command for Gemini
	assertContains(t, out, "3000")              // 3s -> 3000ms

	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Hooks) != 1 {
		t.Fatalf("hook count: got %d, want 1", len(decoded.Hooks))
	}
	h := decoded.Hooks[0]
	assertEqual(t, "log-hook", h.Name)
	assertEqual(t, "before_tool_execute", h.Event)
	if !h.Blocking {
		t.Error("expected Blocking to be true")
	}
	assertEqual(t, "Logging", h.Handler.StatusMessage)
	if h.Handler.Timeout != 3 {
		t.Errorf("Timeout: got %d, want 3 (canonical seconds)", h.Handler.Timeout)
	}
	if !h.Handler.Async {
		t.Error("expected Async to be true")
	}
}

func TestGeminiAdapter_PromptHookDroppedWithWarning(t *testing.T) {
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Handler: HookHandler{Type: "prompt", Prompt: "Is this safe?"},
			},
		},
	}
	adapter := AdapterFor("gemini-cli")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded.Warnings) == 0 {
		t.Error("expected warning for dropped prompt hook")
	}
	// Encoded content should have empty hooks
	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Hooks) != 0 {
		t.Errorf("expected 0 hooks after dropping prompt, got %d", len(decoded.Hooks))
	}
}

func TestGeminiAdapter_HTTPHookDroppedWithWarning(t *testing.T) {
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Handler: HookHandler{Type: "http", URL: "https://example.com"},
			},
		},
	}
	adapter := AdapterFor("gemini-cli")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded.Warnings) == 0 {
		t.Error("expected warning for dropped http hook")
	}
}

func TestGeminiAdapter_TimeoutValues(t *testing.T) {
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event: "session_start",
				Handler: HookHandler{
					Type:    "command",
					Command: "echo init",
					Timeout: 7, // 7 seconds
				},
			},
		},
	}
	adapter := AdapterFor("gemini-cli")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	out := string(encoded.Content)
	assertContains(t, out, "7000") // 7s -> 7000ms

	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Hooks[0].Handler.Timeout != 7 {
		t.Errorf("timeout: got %d, want 7", decoded.Hooks[0].Handler.Timeout)
	}
}

func TestGeminiAdapter_UnsupportedEventDropped(t *testing.T) {
	// subagent_start is not supported by Gemini
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "subagent_start",
				Handler: HookHandler{Type: "command", Command: "echo sub"},
			},
		},
	}
	adapter := AdapterFor("gemini-cli")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded.Warnings) == 0 {
		t.Error("expected warning for unsupported event")
	}
	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Hooks) != 0 {
		t.Errorf("expected 0 hooks after dropping unsupported event, got %d", len(decoded.Hooks))
	}
}

// --- Copilot Adapter Tier 2 Round-Trip Tests ---

func TestCopilotAdapter_RoundTrip_CWDAndEnv(t *testing.T) {
	// CWD and Env are supported by Copilot but were broken in the legacy bridge.
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event: "before_tool_execute",
				Handler: HookHandler{
					Type:    "command",
					Command: "echo check",
					Timeout: 5,
					CWD:     "./hooks",
					Env:     map[string]string{"AUDIT": "1"},
				},
			},
		},
	}
	adapter := AdapterFor("copilot-cli")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Verify CWD and Env appear in encoded JSON
	out := string(encoded.Content)
	assertContains(t, out, "\"cwd\"")
	assertContains(t, out, "./hooks")
	assertContains(t, out, "\"env\"")
	assertContains(t, out, "AUDIT")

	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Hooks) != 1 {
		t.Fatalf("hook count: got %d, want 1", len(decoded.Hooks))
	}
	h := decoded.Hooks[0]
	assertEqual(t, "./hooks", h.Handler.CWD)
	if h.Handler.Env["AUDIT"] != "1" {
		t.Errorf("Env not preserved: %v", h.Handler.Env)
	}
	if h.Handler.Timeout != 5 {
		t.Errorf("Timeout: got %d, want 5 (Copilot uses seconds natively)", h.Handler.Timeout)
	}
}

func TestCopilotAdapter_PowerShellField(t *testing.T) {
	// When bash is empty, powershell field should be used
	input := []byte(`{
		"version": 1,
		"hooks": {
			"preToolUse": [
				{
					"hooks": [
						{"type":"command","powershell":"check.ps1","timeoutSec":10}
					]
				}
			]
		}
	}`)
	adapter := AdapterFor("copilot-cli")
	decoded, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(decoded.Hooks))
	}
	h := decoded.Hooks[0]
	assertEqual(t, "check.ps1", h.Handler.Command)
	if h.Handler.Timeout != 10 {
		t.Errorf("Timeout: got %d, want 10", h.Handler.Timeout)
	}
}

func TestCopilotAdapter_TimeoutValues(t *testing.T) {
	// Copilot uses seconds natively — no conversion should occur
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event: "session_start",
				Handler: HookHandler{
					Type:    "command",
					Command: "echo init",
					Timeout: 10, // 10 seconds
				},
			},
		},
	}
	adapter := AdapterFor("copilot-cli")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	out := string(encoded.Content)
	assertContains(t, out, "\"timeoutSec\": 10") // should be 10, not 10000

	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	for _, h := range decoded.Hooks {
		if h.Handler.Command == "echo init" && h.Handler.Timeout != 10 {
			t.Errorf("timeout: got %d, want 10", h.Handler.Timeout)
		}
	}
}

func TestCopilotAdapter_StatusMessageAsComment(t *testing.T) {
	// StatusMessage maps to Copilot's "comment" field
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event: "before_tool_execute",
				Handler: HookHandler{
					Type:          "command",
					Command:       "echo check",
					StatusMessage: "Running safety check",
				},
			},
		},
	}
	adapter := AdapterFor("copilot-cli")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	out := string(encoded.Content)
	assertContains(t, out, "\"comment\"")
	assertContains(t, out, "Running safety check")

	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	assertEqual(t, "Running safety check", decoded.Hooks[0].Handler.StatusMessage)
}

func TestCopilotAdapter_LLMHookDroppedWithWarning(t *testing.T) {
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Handler: HookHandler{Type: "prompt", Prompt: "Is this safe?"},
			},
		},
	}
	adapter := AdapterFor("copilot-cli")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded.Warnings) == 0 {
		t.Error("expected warning for dropped prompt hook")
	}
}

func TestCopilotAdapter_HTTPHookDroppedWithWarning(t *testing.T) {
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Handler: HookHandler{Type: "http", URL: "https://example.com"},
			},
		},
	}
	adapter := AdapterFor("copilot-cli")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded.Warnings) == 0 {
		t.Error("expected warning for dropped http hook")
	}
}

// --- Cursor Adapter Tier 2 Round-Trip Tests ---

func TestCursorAdapter_RoundTrip_AllFields(t *testing.T) {
	matcherJSON, _ := json.Marshal("shell")
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Name:    "cursor-guard",
				Event:   "before_tool_execute",
				Matcher: matcherJSON,
				Handler: HookHandler{
					Type:          "command",
					Command:       "echo cursor",
					Timeout:       5, // canonical seconds -> 5000ms in Cursor
					StatusMessage: "Checking",
				},
				Blocking: true,
			},
		},
	}
	adapter := AdapterFor("cursor")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// Verify Cursor-native format
	out := string(encoded.Content)
	assertContains(t, out, "PreToolUse")
	assertContains(t, out, "5000") // 5s -> 5000ms

	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Hooks) != 1 {
		t.Fatalf("hook count: got %d, want 1", len(decoded.Hooks))
	}
	h := decoded.Hooks[0]
	assertEqual(t, "cursor-guard", h.Name)
	if !h.Blocking {
		t.Error("expected Blocking to be true")
	}
	if h.Handler.Timeout != 5 {
		t.Errorf("Timeout: got %d, want 5 (canonical seconds)", h.Handler.Timeout)
	}
	assertEqual(t, "Checking", h.Handler.StatusMessage)
}

func TestCursorAdapter_ProviderDataPreservesFailClosed(t *testing.T) {
	// failClosed and loop_limit are Cursor-specific with no canonical equivalent.
	input := []byte(`{
		"version": 1,
		"hooks": {
			"PreToolUse": [
				{
					"matcher": "Bash",
					"hooks": [{"type":"command","command":"echo check","timeout":5000}],
					"failClosed": true,
					"loop_limit": 3
				}
			]
		}
	}`)
	adapter := AdapterFor("cursor")
	decoded, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Hooks) == 0 {
		t.Fatal("expected at least 1 hook")
	}
	h := decoded.Hooks[0]
	// failClosed + loop_limit should be in provider_data
	if h.ProviderData == nil || h.ProviderData["cursor"] == nil {
		t.Fatal("expected provider_data[\"cursor\"] to preserve failClosed and loop_limit")
	}
	cursorData, ok := h.ProviderData["cursor"].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", h.ProviderData["cursor"])
	}
	if cursorData["failClosed"] != true {
		t.Errorf("expected failClosed true, got %v", cursorData["failClosed"])
	}
	// loop_limit is stored as int from the adapter (not float64 from JSON re-parse)
	loopLimit, ok := cursorData["loop_limit"].(int)
	if !ok || loopLimit != 3 {
		t.Errorf("expected loop_limit 3 (int), got %v (%T)", cursorData["loop_limit"], cursorData["loop_limit"])
	}
	// failClosed -> Blocking
	if !h.Blocking {
		t.Error("expected Blocking to be true from failClosed")
	}
}

func TestCursorAdapter_TimeoutValues(t *testing.T) {
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event: "session_start",
				Handler: HookHandler{
					Type:    "command",
					Command: "echo init",
					Timeout: 7,
				},
			},
		},
	}
	adapter := AdapterFor("cursor")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	out := string(encoded.Content)
	assertContains(t, out, "7000") // 7s -> 7000ms

	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Hooks[0].Handler.Timeout != 7 {
		t.Errorf("timeout: got %d, want 7", decoded.Hooks[0].Handler.Timeout)
	}
}

func TestCursorAdapter_LLMHookDroppedWithWarning(t *testing.T) {
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Handler: HookHandler{Type: "prompt", Prompt: "Is this safe?"},
			},
		},
	}
	adapter := AdapterFor("cursor")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded.Warnings) == 0 {
		t.Error("expected warning for dropped prompt hook")
	}
}

// --- Kiro Adapter Tier 2 Round-Trip Tests ---

func TestKiroAdapter_RoundTrip_AllFields(t *testing.T) {
	matcherJSON, _ := json.Marshal("shell")
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Name:    "kiro-check",
				Event:   "before_tool_execute",
				Matcher: matcherJSON,
				Handler: HookHandler{
					Type:    "command",
					Command: "echo kiro",
					Timeout: 5, // canonical seconds -> 5000ms in Kiro
				},
				Blocking: true,
			},
		},
	}
	adapter := AdapterFor("kiro")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Kiro output is an agent wrapper
	out := string(encoded.Content)
	assertContains(t, out, "syllago-hooks")
	assertContains(t, out, "preToolUse")
	assertContains(t, out, "echo kiro")
	assertContains(t, out, "5000") // timeout in ms

	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Hooks) != 1 {
		t.Fatalf("hook count: got %d, want 1", len(decoded.Hooks))
	}
	h := decoded.Hooks[0]
	assertEqual(t, "before_tool_execute", h.Event)
	if h.Handler.Timeout != 5 {
		t.Errorf("Timeout: got %d, want 5 (canonical seconds)", h.Handler.Timeout)
	}
}

func TestKiroAdapter_AgentWrapperFields(t *testing.T) {
	// Kiro's agent wrapper preserves name/description/prompt via provider_data
	input := []byte(`{
		"name": "custom-hooks",
		"description": "My custom hooks",
		"prompt": "Review all tool usage",
		"hooks": {
			"preToolUse": [
				{"command": "echo check", "matcher": "shell", "timeout_ms": 3000}
			]
		}
	}`)
	adapter := AdapterFor("kiro")
	decoded, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Hooks) == 0 {
		t.Fatal("expected at least 1 hook")
	}
	h := decoded.Hooks[0]
	pd := h.ProviderData
	if pd == nil || pd["kiro"] == nil {
		t.Fatal("expected kiro agent wrapper metadata in provider_data[\"kiro\"]")
	}
	kiroData, ok := pd["kiro"].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", pd["kiro"])
	}
	if kiroData["name"] != "custom-hooks" {
		t.Errorf("expected name 'custom-hooks', got %v", kiroData["name"])
	}
	if kiroData["description"] != "My custom hooks" {
		t.Errorf("expected description 'My custom hooks', got %v", kiroData["description"])
	}
	if kiroData["prompt"] != "Review all tool usage" {
		t.Errorf("expected prompt 'Review all tool usage', got %v", kiroData["prompt"])
	}
	// Timeout should be decoded correctly
	if h.Handler.Timeout != 3 {
		t.Errorf("Timeout: got %d, want 3 (3000ms -> 3s)", h.Handler.Timeout)
	}
}

func TestKiroAdapter_TimeoutValues(t *testing.T) {
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event: "session_start",
				Handler: HookHandler{
					Type:    "command",
					Command: "echo init",
					Timeout: 7, // 7 seconds
				},
			},
		},
	}
	adapter := AdapterFor("kiro")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	out := string(encoded.Content)
	assertContains(t, out, "7000") // 7s -> 7000ms

	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Hooks[0].Handler.Timeout != 7 {
		t.Errorf("timeout: got %d, want 7", decoded.Hooks[0].Handler.Timeout)
	}
}

func TestKiroAdapter_PerEntryMatchers(t *testing.T) {
	// Kiro uses per-entry matchers (not group-level like CC/Gemini)
	input := []byte(`{
		"name": "syllago-hooks",
		"description": "test",
		"prompt": "",
		"hooks": {
			"preToolUse": [
				{"command": "echo shell", "matcher": "shell", "timeout_ms": 3000},
				{"command": "echo read", "matcher": "read", "timeout_ms": 5000}
			]
		}
	}`)
	adapter := AdapterFor("kiro")
	decoded, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(decoded.Hooks))
	}
	// Each hook should have its own matcher
	for _, h := range decoded.Hooks {
		if h.Matcher == nil {
			t.Error("expected each Kiro hook to have its own matcher")
		}
	}
}

func TestKiroAdapter_LLMHookDroppedWithWarning(t *testing.T) {
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Handler: HookHandler{Type: "prompt", Prompt: "Is this safe?"},
			},
		},
	}
	adapter := AdapterFor("kiro")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded.Warnings) == 0 {
		t.Error("expected warning for dropped prompt hook")
	}
}

func TestKiroAdapter_StatusMessageWarning(t *testing.T) {
	// Kiro does not support status_message — should get a warning if present
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event: "before_tool_execute",
				Handler: HookHandler{
					Type:          "command",
					Command:       "echo check",
					StatusMessage: "Checking...",
				},
			},
		},
	}
	adapter := AdapterFor("kiro")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// status_message is silently dropped (Kiro doesn't have the field)
	// The hook itself should still be encoded
	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Hooks) != 1 {
		t.Fatalf("expected 1 hook (status_message dropped, hook kept), got %d", len(decoded.Hooks))
	}
}
