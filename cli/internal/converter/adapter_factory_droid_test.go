package converter

import (
	"encoding/json"
	"testing"
)

func TestFactoryDroidAdapterDecode(t *testing.T) {
	// Execute is Factory Droid's native name for shell
	input := []byte(`{"hooks": {"PreToolUse": [{"matcher": "Execute", "hooks": [{"type": "command", "command": "echo check", "timeout": 5000}]}]}}`)

	adapter := AdapterFor("factory-droid")
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

	// Execute → canonical shell
	var matcher string
	if err := json.Unmarshal(h.Matcher, &matcher); err != nil {
		t.Fatalf("unmarshal matcher: %v", err)
	}
	assertEqual(t, "shell", matcher)
}

func TestFactoryDroidAdapterEncode(t *testing.T) {
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

	adapter := AdapterFor("factory-droid")
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	out := string(encoded.Content)
	assertContains(t, out, "PreToolUse")
	assertContains(t, out, "Execute") // shell → Execute for Factory Droid
	assertContains(t, out, "5000")
	assertEqual(t, "settings.json", encoded.Filename)
}

func TestFactoryDroidAdapterEncode_WriteBecomesCreate(t *testing.T) {
	matcherJSON, _ := json.Marshal("file_write")
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Matcher: matcherJSON,
				Handler: HookHandler{Type: "command", Command: "echo write-check"},
			},
		},
	}

	adapter := AdapterFor("factory-droid")
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	out := string(encoded.Content)
	assertContains(t, out, "Create") // file_write → Create for Factory Droid
	assertNotContains(t, out, "Write")
}

func TestFactoryDroidAdapterEncode_CCExclusiveEventDropped(t *testing.T) {
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "worktree_create",
				Handler: HookHandler{Type: "command", Command: "echo wt"},
			},
		},
	}

	adapter := AdapterFor("factory-droid")
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(encoded.Warnings) == 0 {
		t.Error("expected warning for unsupported CC-exclusive event")
	}
}

func TestFactoryDroidAdapterRoundTrip(t *testing.T) {
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

	adapter := AdapterFor("factory-droid")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	err = Verify(encoded, adapter, original)
	if err != nil {
		t.Fatalf("Verify should pass: %v", err)
	}
}

func TestFactoryDroidAdapterCapabilities(t *testing.T) {
	caps := AdapterFor("factory-droid").Capabilities()
	if !caps.SupportsMatchers {
		t.Error("Factory Droid should support matchers")
	}
	if caps.SupportsLLMHooks {
		t.Error("Factory Droid should not support LLM hooks")
	}
	if caps.SupportsHTTPHooks {
		t.Error("Factory Droid should not support HTTP hooks")
	}
	if len(caps.Events) != 9 {
		t.Errorf("expected 9 events, got %d", len(caps.Events))
	}
}
