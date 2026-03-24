package converter

import (
	"encoding/json"
	"testing"
)

func TestAdapterRegistry(t *testing.T) {
	// All expected adapters should be registered via init()
	expected := []string{"claude-code", "gemini-cli", "copilot-cli", "kiro", "cursor"}
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

func TestFromLegacyHooksConfig(t *testing.T) {
	cfg := hooksConfig{
		Hooks: map[string][]hookMatcher{
			"before_tool_execute": {
				{Matcher: "shell", Hooks: []HookEntry{
					{Type: "command", Command: "echo check", Timeout: 5},
				}},
			},
			"session_start": {
				{Hooks: []HookEntry{
					{Type: "command", Command: "echo init"},
				}},
			},
		},
	}

	ch := FromLegacyHooksConfig(cfg)
	assertEqual(t, SpecVersion, ch.Spec)
	if len(ch.Hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(ch.Hooks))
	}
}

func TestToLegacyHooksConfig(t *testing.T) {
	matcher, _ := json.Marshal("shell")
	ch := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Matcher: matcher,
				Handler: HookHandler{Type: "command", Command: "echo check", Timeout: 5},
			},
		},
	}

	cfg := ch.ToLegacyHooksConfig()
	matchers, ok := cfg.Hooks["before_tool_execute"]
	if !ok {
		t.Fatal("expected before_tool_execute event in legacy config")
	}
	if len(matchers) != 1 {
		t.Fatalf("expected 1 matcher, got %d", len(matchers))
	}
	assertEqual(t, "shell", matchers[0].Matcher)
	assertEqual(t, "echo check", matchers[0].Hooks[0].Command)
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

func TestVerify_CountMismatch(t *testing.T) {
	matcher, _ := json.Marshal("shell")
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Matcher: matcher,
				Handler: HookHandler{Type: "command", Command: "echo check"},
			},
			{
				Event:   "session_start",
				Handler: HookHandler{Type: "command", Command: "echo init"},
			},
		},
	}

	// Encode with just 1 hook
	singleHook := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Matcher: matcher,
				Handler: HookHandler{Type: "command", Command: "echo check"},
			},
		},
	}

	adapter := AdapterFor("claude-code")
	encoded, err := adapter.Encode(singleHook)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	err = Verify(encoded, adapter, original)
	if err == nil {
		t.Fatal("expected Verify to fail on count mismatch")
	}

	verErr, ok := err.(*VerifyError)
	if !ok {
		t.Fatalf("expected *VerifyError, got %T", err)
	}
	assertContains(t, verErr.Error(), "hook count mismatch")
	assertContains(t, verErr.Error(), "expected 2")
	assertContains(t, verErr.Error(), "got 1")
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

func TestLegacyResultToEncoded(t *testing.T) {
	r := &Result{
		Content:  []byte("content"),
		Filename: "hooks.json",
		Warnings: []string{"warning 1", "warning 2"},
	}

	er := legacyResultToEncoded(r)
	assertEqual(t, "hooks.json", er.Filename)
	if len(er.Warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d", len(er.Warnings))
	}
	assertEqual(t, "warning", er.Warnings[0].Severity)
	assertEqual(t, "warning 1", er.Warnings[0].Description)
}

func TestLegacyResultToEncodedNoWarnings(t *testing.T) {
	r := &Result{
		Content:  []byte("content"),
		Filename: "hooks.json",
	}

	er := legacyResultToEncoded(r)
	if len(er.Warnings) != 0 {
		t.Fatalf("expected 0 warnings, got %d", len(er.Warnings))
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
