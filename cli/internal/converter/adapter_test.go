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
