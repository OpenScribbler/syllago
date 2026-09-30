package converter

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDevinAdapterDecode_Simple(t *testing.T) {
	input := []byte(`{"hooks": {"pre_run_command": [{"command": "echo pre-run"}], "post_run_command": [{"command": "echo post-run"}]}}`)

	adapter := AdapterFor("devin")
	hooks, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	assertEqual(t, SpecVersion, hooks.Spec)
	if len(hooks.Hooks) != 2 {
		t.Fatalf("expected 2 hooks, got %d", len(hooks.Hooks))
	}

	// Find pre and post hooks by event
	var pre, post *CanonicalHook
	for i := range hooks.Hooks {
		h := &hooks.Hooks[i]
		switch h.Event {
		case "before_tool_execute":
			pre = h
		case "after_tool_execute":
			post = h
		}
	}

	if pre == nil {
		t.Fatal("expected before_tool_execute hook")
	}
	assertEqual(t, "echo pre-run", pre.Handler.Command)
	if !pre.Blocking {
		t.Error("pre-hook should be blocking")
	}
	// Matcher should be "shell" (derived from pre_run_command)
	var matcher string
	if err := json.Unmarshal(pre.Matcher, &matcher); err != nil {
		t.Fatalf("unmarshal pre matcher: %v", err)
	}
	assertEqual(t, "shell", matcher)

	if post == nil {
		t.Fatal("expected after_tool_execute hook")
	}
	assertEqual(t, "echo post-run", post.Handler.Command)
	if post.Blocking {
		t.Error("post-hook should not be blocking")
	}
}

func TestDevinAdapterEncode_ShellMatcher(t *testing.T) {
	matcherJSON, _ := json.Marshal("shell")
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:    "before_tool_execute",
				Matcher:  matcherJSON,
				Blocking: true,
				Handler:  HookHandler{Type: "command", Command: "echo check"},
			},
		},
	}

	adapter := AdapterFor("devin")
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	out := string(encoded.Content)
	// Should only have pre_run_command, not pre_read_code etc.
	assertContains(t, out, "pre_run_command")
	assertNotContains(t, out, "pre_read_code")
	assertNotContains(t, out, "pre_write_code")
	assertNotContains(t, out, "pre_mcp_tool_use")
	assertContains(t, out, "echo check")
}

func TestDevinAdapterEncode_WildcardExpands(t *testing.T) {
	// nil matcher → all 4 pre-events
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:    "before_tool_execute",
				Blocking: true,
				Handler:  HookHandler{Type: "command", Command: "echo guard"},
			},
		},
	}

	adapter := AdapterFor("devin")
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	out := string(encoded.Content)
	assertContains(t, out, "pre_run_command")
	assertContains(t, out, "pre_read_code")
	assertContains(t, out, "pre_write_code")
	assertContains(t, out, "pre_mcp_tool_use")
}

func TestDevinAdapterDecode_WildcardMerges(t *testing.T) {
	// All 4 pre-events with identical command → merged into 1 hook with nil matcher
	input := []byte(`{
		"hooks": {
			"pre_run_command":  [{"command": "echo guard"}],
			"pre_read_code":    [{"command": "echo guard"}],
			"pre_write_code":   [{"command": "echo guard"}],
			"pre_mcp_tool_use": [{"command": "echo guard"}]
		}
	}`)

	adapter := AdapterFor("devin")
	hooks, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if len(hooks.Hooks) != 1 {
		t.Fatalf("expected 1 merged hook, got %d", len(hooks.Hooks))
	}

	h := hooks.Hooks[0]
	assertEqual(t, "before_tool_execute", h.Event)
	assertEqual(t, "echo guard", h.Handler.Command)
	if h.Matcher != nil {
		t.Error("expected nil matcher for wildcard-merged hook")
	}
	if !h.Blocking {
		t.Error("merged pre-hook should be blocking")
	}
	// Should have provider_data marking wildcard origin
	if h.ProviderData == nil || h.ProviderData["devin"] == nil {
		t.Fatal("expected provider_data[\"devin\"] with expanded_from")
	}
}

func TestDevinAdapterDecode_PartialWildcardNotMerged(t *testing.T) {
	// Only 2 of 4 pre-events → NOT merged, stays as 2 separate hooks
	input := []byte(`{
		"hooks": {
			"pre_run_command": [{"command": "echo guard"}],
			"pre_read_code":   [{"command": "echo guard"}]
		}
	}`)

	adapter := AdapterFor("devin")
	hooks, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if len(hooks.Hooks) != 2 {
		t.Fatalf("expected 2 separate hooks (no merge), got %d", len(hooks.Hooks))
	}

	// Each should have its own matcher
	for _, h := range hooks.Hooks {
		if h.Matcher == nil {
			t.Error("expected non-nil matcher for non-merged hook")
		}
	}
}

func TestDevinAdapterEncode_TimeoutDroppedWithWarning(t *testing.T) {
	matcherJSON, _ := json.Marshal("shell")
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:    "before_tool_execute",
				Matcher:  matcherJSON,
				Blocking: true,
				Handler:  HookHandler{Type: "command", Command: "echo check", Timeout: 10},
			},
		},
	}

	adapter := AdapterFor("devin")
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// Should have a timeout warning
	hasWarning := false
	for _, w := range encoded.Warnings {
		if strings.Contains(w.Description, "timeout") {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Error("expected timeout warning")
	}

	// "timeout" should not appear in the output JSON
	assertNotContains(t, string(encoded.Content), "timeout")
}

func TestDevinAdapterEncode_BlockingFalseWrapped(t *testing.T) {
	matcherJSON, _ := json.Marshal("shell")
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:    "before_tool_execute",
				Matcher:  matcherJSON,
				Blocking: false, // non-blocking
				Handler:  HookHandler{Type: "command", Command: "echo check"},
			},
		},
	}

	adapter := AdapterFor("devin")
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	out := string(encoded.Content)
	assertContains(t, out, "|| true")
}

func TestDevinAdapterEncode_DirectMappedEvents(t *testing.T) {
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "session_start",
				Handler: HookHandler{Type: "command", Command: "echo init"},
			},
			{
				Event:   "before_prompt",
				Handler: HookHandler{Type: "command", Command: "echo prompt"},
			},
		},
	}

	adapter := AdapterFor("devin")
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	out := string(encoded.Content)
	assertContains(t, out, "session_start")
	assertContains(t, out, "pre_user_prompt")
}

func TestDevinAdapterEncode_CWDPreserved(t *testing.T) {
	matcherJSON, _ := json.Marshal("shell")
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:    "before_tool_execute",
				Matcher:  matcherJSON,
				Blocking: true,
				Handler:  HookHandler{Type: "command", Command: "echo check", CWD: "./scripts"},
			},
		},
	}

	adapter := AdapterFor("devin")
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	out := string(encoded.Content)
	assertContains(t, out, "working_directory")
	assertContains(t, out, "./scripts")
}

func TestDevinAdapterRoundTrip(t *testing.T) {
	matcherJSON, _ := json.Marshal("shell")
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:    "before_tool_execute",
				Matcher:  matcherJSON,
				Blocking: true,
				Handler:  HookHandler{Type: "command", Command: "echo check"},
			},
		},
	}

	adapter := AdapterFor("devin")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	err = Verify(encoded, adapter, original)
	if err != nil {
		t.Fatalf("Verify should pass: %v", err)
	}
}

func TestDevinAdapterCapabilities(t *testing.T) {
	caps := AdapterFor("devin").Capabilities()
	if !caps.SupportsMatchers {
		t.Error("Devin should support matchers (via split-events)")
	}
	if !caps.SupportsBlocking {
		t.Error("Devin should support blocking")
	}
	if !caps.SupportsCWD {
		t.Error("Devin should support CWD")
	}
	if caps.SupportsLLMHooks {
		t.Error("Devin should not support LLM hooks")
	}
	if caps.SupportsHTTPHooks {
		t.Error("Devin should not support HTTP hooks")
	}
	if caps.TimeoutUnit != "" {
		t.Errorf("Devin TimeoutUnit should be empty, got %q", caps.TimeoutUnit)
	}
}

func TestDevinAdapterDecode_ShowOutputPreserved(t *testing.T) {
	input := []byte(`{"hooks": {"pre_run_command": [{"command": "echo check", "show_output": true}]}}`)

	adapter := AdapterFor("devin")
	hooks, err := adapter.Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if len(hooks.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks.Hooks))
	}
	h := hooks.Hooks[0]

	// show_output should be in provider_data
	if h.ProviderData == nil || h.ProviderData["devin"] == nil {
		t.Fatal("expected provider_data[\"devin\"] with show_output")
	}
	wsData, ok := h.ProviderData["devin"].(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", h.ProviderData["devin"])
	}
	if wsData["show_output"] != true {
		t.Errorf("expected show_output true, got %v", wsData["show_output"])
	}

	// Re-encode should include show_output
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	assertContains(t, string(encoded.Content), "show_output")
}
