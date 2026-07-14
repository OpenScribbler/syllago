package converter

import (
	"encoding/json"
	"testing"

	"github.com/tidwall/gjson"
)

func TestCrushAdapterEncode_Basic(t *testing.T) {
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Name:     "guard-shell",
				Event:    "before_tool_execute",
				Matcher:  json.RawMessage(`"shell"`),
				Blocking: true,
				Handler: HookHandler{
					Type:    "command",
					Command: "echo check",
					Timeout: 5,
				},
			},
		},
	}

	adapter := AdapterFor("crush")
	if adapter == nil {
		t.Fatal("crush adapter not registered")
	}
	encoded, err := adapter.Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	if encoded.Filename != "crush.json" {
		t.Errorf("filename: got %q, want crush.json", encoded.Filename)
	}

	entry := gjson.GetBytes(encoded.Content, "hooks.PreToolUse.0")
	if !entry.Exists() {
		t.Fatalf("expected hooks.PreToolUse.0 in output, got: %s", encoded.Content)
	}
	if got := entry.Get("command").String(); got != "echo check" {
		t.Errorf("command: got %q, want %q", got, "echo check")
	}
	// Canonical "shell" must translate to crush's native tool name "bash".
	if got := entry.Get("matcher").String(); got != "bash" {
		t.Errorf("matcher: got %q, want %q", got, "bash")
	}
	// Crush timeouts are seconds — canonical 5s stays 5, not 5000.
	if got := entry.Get("timeout").Int(); got != 5 {
		t.Errorf("timeout: got %d, want 5", got)
	}
	if got := entry.Get("name").String(); got != "guard-shell" {
		t.Errorf("name: got %q, want %q", got, "guard-shell")
	}
}

func TestCrushAdapterEncode_UnsupportedEventSkipped(t *testing.T) {
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "session_start",
				Handler: HookHandler{Type: "command", Command: "echo hi"},
			},
		},
	}

	encoded, err := AdapterFor("crush").Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if gjson.GetBytes(encoded.Content, "hooks.session_start").Exists() ||
		gjson.GetBytes(encoded.Content, "hooks.SessionStart").Exists() {
		t.Error("unsupported event should not appear in output")
	}
	if len(encoded.Warnings) == 0 {
		t.Error("expected a warning for the skipped event")
	}
}

func TestCrushAdapterEncode_DegradationBlock(t *testing.T) {
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:       "before_tool_execute",
				Degradation: map[string]string{"llm_evaluated": "block"},
				Handler:     HookHandler{Type: "prompt", Prompt: "Is this safe?"},
			},
		},
	}

	encoded, err := AdapterFor("crush").Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if gjson.GetBytes(encoded.Content, "hooks.PreToolUse").Exists() {
		t.Error("blocked hook should not appear in output")
	}
	var sawError bool
	for _, w := range encoded.Warnings {
		if w.Severity == "error" {
			sawError = true
		}
	}
	if !sawError {
		t.Errorf("expected error-severity warning from block degradation, got: %+v", encoded.Warnings)
	}
}

func TestCrushAdapterEncode_LLMExcludedByDefault(t *testing.T) {
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:   "before_tool_execute",
				Handler: HookHandler{Type: "prompt", Prompt: "Is this safe?"},
			},
		},
	}

	encoded, err := AdapterFor("crush").Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if gjson.GetBytes(encoded.Content, "hooks.PreToolUse").Exists() {
		t.Error("LLM hook should be excluded for crush")
	}
	if len(encoded.Warnings) == 0 {
		t.Error("expected a warning for the excluded LLM hook")
	}
}

func TestCrushAdapterEncode_NonBlockingWarns(t *testing.T) {
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:    "before_tool_execute",
				Blocking: false,
				Handler:  HookHandler{Type: "command", Command: "echo log"},
			},
		},
	}

	encoded, err := AdapterFor("crush").Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// The hook is still encoded (command unchanged) but crush PreToolUse hooks
	// always carry veto power, so the non-blocking intent is flagged.
	if got := gjson.GetBytes(encoded.Content, "hooks.PreToolUse.0.command").String(); got != "echo log" {
		t.Errorf("command: got %q, want %q", got, "echo log")
	}
	if len(encoded.Warnings) == 0 {
		t.Error("expected an info warning that non-blocking intent is not preserved")
	}
}

func TestCrushAdapterDecode_Basic(t *testing.T) {
	input := []byte(`{
		"hooks": {
			"PreToolUse": [
				{"name": "guard-shell", "matcher": "bash", "command": "echo check", "timeout": 5}
			]
		}
	}`)

	hooks, err := AdapterFor("crush").Decode(input)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(hooks.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(hooks.Hooks))
	}
	h := hooks.Hooks[0]
	assertEqual(t, "before_tool_execute", h.Event)
	assertEqual(t, "guard-shell", h.Name)
	assertEqual(t, "echo check", h.Handler.Command)
	assertEqual(t, "command", h.Handler.Type)
	// Crush native "bash" reverse-translates to canonical "shell".
	var matcher string
	if err := json.Unmarshal(h.Matcher, &matcher); err != nil {
		t.Fatalf("matcher unmarshal: %v", err)
	}
	assertEqual(t, "shell", matcher)
	// Seconds stay seconds.
	if h.Handler.Timeout != 5 {
		t.Errorf("timeout: got %d, want 5", h.Handler.Timeout)
	}
	// Crush PreToolUse hooks run before permission checks with veto power.
	if !h.Blocking {
		t.Error("crush hooks should decode as blocking")
	}
}

func TestCrushAdapter_VerifyRoundTrip(t *testing.T) {
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Name:     "rt",
				Event:    "before_tool_execute",
				Matcher:  json.RawMessage(`"shell"`),
				Blocking: true,
				Handler: HookHandler{
					Type:    "command",
					Command: "echo rt",
					Timeout: 10,
				},
			},
		},
	}

	adapter := AdapterFor("crush")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if err := Verify(encoded, adapter, original); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

func TestCrushAdapter_Capabilities(t *testing.T) {
	caps := AdapterFor("crush").Capabilities()
	if len(caps.Events) != 1 || caps.Events[0] != "before_tool_execute" {
		t.Errorf("events: got %v, want [before_tool_execute]", caps.Events)
	}
	if !caps.SupportsMatchers {
		t.Error("crush supports matchers")
	}
	if !caps.SupportsBlocking {
		t.Error("crush supports blocking (exit code 2 / deny decision)")
	}
	if !caps.SupportsStructuredOutput {
		t.Error("crush supports structured output (JSON decision/context)")
	}
	if caps.TimeoutUnit != "seconds" {
		t.Errorf("timeout unit: got %q, want seconds", caps.TimeoutUnit)
	}
	if caps.SupportsAsync {
		t.Error("crush hooks are synchronous")
	}
	if caps.SupportsLLMHooks || caps.SupportsHTTPHooks {
		t.Error("crush supports command hooks only")
	}
}

func TestTranslateHookEvent_Crush(t *testing.T) {
	got, ok := TranslateHookEvent("before_tool_execute", "crush")
	if !ok || got != "PreToolUse" {
		t.Errorf("before_tool_execute: got (%q, %v), want (PreToolUse, true)", got, ok)
	}
	if _, ok := TranslateHookEvent("session_start", "crush"); ok {
		t.Error("session_start should not be supported by crush")
	}
}

func TestTranslateMatcher_Crush(t *testing.T) {
	if got := TranslateMatcher("file_edit|shell", "crush"); got != "edit|bash" {
		t.Errorf("got %q, want edit|bash", got)
	}
	if got := ReverseTranslateTool("view", "crush"); got != "file_read" {
		t.Errorf("got %q, want file_read", got)
	}
}
