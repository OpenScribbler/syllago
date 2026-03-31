package converter

import (
	"encoding/json"
	"testing"
)

func TestVSCodeCopilotAdapterDecode(t *testing.T) {
	input := []byte(`{"hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo check", "timeout": 5000}]}]}}`)

	adapter := AdapterFor("vs-code-copilot")
	hooks, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	assertEqual(t, SpecVersion, hooks.Spec)
	if len(hooks.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks.Hooks))
	}
	h := hooks.Hooks[0]
	assertEqual(t, "before_tool_execute", h.Event)
	assertEqual(t, "echo check", h.Handler.Command)
	if h.Handler.Timeout != 5 {
		t.Errorf("Timeout: got %d, want 5 (5000ms -> 5s)", h.Handler.Timeout)
	}
}

func TestVSCodeCopilotAdapterEncode(t *testing.T) {
	matcherJSON, _ := json.Marshal("shell")
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:    "before_tool_execute",
				Matcher:  matcherJSON,
				Blocking: true,
				Handler:  HookHandler{Type: "command", Command: "echo check", Timeout: 5},
			},
		},
	}

	adapter := AdapterFor("vs-code-copilot")
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	out := string(encoded.Content)
	assertContains(t, out, "PreToolUse")
	assertContains(t, out, "Bash") // shell -> Bash for vs-code-copilot
	assertContains(t, out, "5000") // 5s -> 5000ms
	assertContains(t, out, "echo check")
}

func TestVSCodeCopilotAdapter_PlatformCommands_RoundTrip(t *testing.T) {
	input := []byte(`{"hooks": {"PreToolUse": [{"hooks": [{"type": "command", "command": "echo default", "platform": {"darwin": "echo mac", "linux": "echo linux"}}]}]}}`)

	adapter := AdapterFor("vs-code-copilot")
	decoded, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if len(decoded.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(decoded.Hooks))
	}
	h := decoded.Hooks[0]
	if h.Handler.Platform == nil || h.Handler.Platform["darwin"] != "echo mac" {
		t.Errorf("platform not preserved in decode: %v", h.Handler.Platform)
	}

	// Re-encode
	encoded, err := adapter.Encode(decoded)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	out := string(encoded.Content)
	assertContains(t, out, "platform")
	assertContains(t, out, "echo mac")
	assertContains(t, out, "echo linux")
}

func TestVSCodeCopilotAdapter_EnvMap_RoundTrip(t *testing.T) {
	input := []byte(`{"hooks": {"SessionStart": [{"hooks": [{"type": "command", "command": "echo init", "env": {"AUDIT_LOG": "/tmp/audit.log"}}]}]}}`)

	adapter := AdapterFor("vs-code-copilot")
	decoded, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if len(decoded.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(decoded.Hooks))
	}
	h := decoded.Hooks[0]
	if h.Handler.Env == nil || h.Handler.Env["AUDIT_LOG"] != "/tmp/audit.log" {
		t.Errorf("env not preserved in decode: %v", h.Handler.Env)
	}

	// Re-encode
	encoded, err := adapter.Encode(decoded)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	out := string(encoded.Content)
	assertContains(t, out, "env")
	assertContains(t, out, "AUDIT_LOG")
}

func TestVSCodeCopilotAdapter_UnsupportedEvent_Dropped(t *testing.T) {
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "worktree_create",
				Handler: HookHandler{Type: "command", Command: "echo wt"},
			},
		},
	}

	adapter := AdapterFor("vs-code-copilot")
	encoded, err := adapter.Encode(hooks)
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

func TestVSCodeCopilotAdapter_LLMHookDropped(t *testing.T) {
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Handler: HookHandler{Type: "prompt", Prompt: "Is this safe?"},
			},
		},
	}

	adapter := AdapterFor("vs-code-copilot")
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded.Warnings) == 0 {
		t.Error("expected warning for dropped prompt hook")
	}
}

func TestVSCodeCopilotAdapterCapabilities(t *testing.T) {
	caps := AdapterFor("vs-code-copilot").Capabilities()
	if !caps.SupportsPlatform {
		t.Error("VS Code Copilot should support platform commands")
	}
	if !caps.SupportsEnv {
		t.Error("VS Code Copilot should support env")
	}
	if caps.SupportsLLMHooks {
		t.Error("VS Code Copilot should not support LLM hooks")
	}
	if caps.SupportsHTTPHooks {
		t.Error("VS Code Copilot should not support HTTP hooks")
	}
	if !caps.SupportsMatchers {
		t.Error("VS Code Copilot should support matchers")
	}
}

func TestVSCodeCopilotAdapterRoundTrip(t *testing.T) {
	matcherJSON, _ := json.Marshal("shell")
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:    "before_tool_execute",
				Matcher:  matcherJSON,
				Blocking: true,
				Handler:  HookHandler{Type: "command", Command: "echo check", Timeout: 5},
			},
		},
	}

	adapter := AdapterFor("vs-code-copilot")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	err = Verify(encoded, adapter, original)
	if err != nil {
		t.Fatalf("Verify should pass: %v", err)
	}
}
