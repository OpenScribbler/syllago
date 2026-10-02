package converter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestDevinAdapterEncode_Basic(t *testing.T) {
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:    "before_tool_execute",
				Matcher:  json.RawMessage(`"shell"`),
				Blocking: true,
				Handler:  HookHandler{Type: "command", Command: "echo check", Timeout: 5},
			},
		},
	}

	encoded, err := AdapterFor("devin").Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	assertEqual(t, "config.json", encoded.Filename)

	group := gjson.GetBytes(encoded.Content, "hooks.PreToolUse.0")
	if !group.Exists() {
		t.Fatalf("expected hooks.PreToolUse.0, got: %s", encoded.Content)
	}
	// Canonical "shell" must translate to Devin's native tool name "exec".
	assertEqual(t, "exec", group.Get("matcher").String())
	entry := group.Get("hooks.0")
	assertEqual(t, "command", entry.Get("type").String())
	assertEqual(t, "echo check", entry.Get("command").String())
	// Devin timeouts are seconds — canonical 5s stays 5, not 5000.
	if got := entry.Get("timeout").Int(); got != 5 {
		t.Errorf("timeout: got %d, want 5", got)
	}
	if len(encoded.Warnings) != 0 {
		t.Errorf("expected no warnings, got %v", encoded.Warnings)
	}
}

func TestDevinAdapterEncode_EntryCarriesOnlyDocumentedFields(t *testing.T) {
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:    "session_start",
				Blocking: false,
				Handler: HookHandler{
					Type:          "command",
					Command:       "echo hi",
					CWD:           "/tmp",
					Env:           map[string]string{"A": "1"},
					Async:         true,
					StatusMessage: "starting",
				},
			},
		},
	}

	encoded, err := AdapterFor("devin").Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	entry := gjson.GetBytes(encoded.Content, "hooks.SessionStart.0.hooks.0")
	entry.ForEach(func(key, _ gjson.Result) bool {
		switch key.String() {
		case "type", "command", "timeout":
		default:
			t.Errorf("unexpected field %q in devin hook entry: %s", key.String(), entry.Raw)
		}
		return true
	})
	if !hasWarningContaining(encoded.Warnings, "cwd, env, platform, async, and statusMessage dropped") {
		t.Errorf("expected dropped-fields warning, got %v", encoded.Warnings)
	}
}

func TestDevinAdapterEncode_MCPMatcher(t *testing.T) {
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{
				Event:    "before_tool_execute",
				Matcher:  json.RawMessage(`{"mcp":{"server":"github","tool":"create_issue"}}`),
				Blocking: true,
				Handler:  HookHandler{Type: "command", Command: "echo mcp"},
			},
		},
	}

	encoded, err := AdapterFor("devin").Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	assertEqual(t, "mcp__github__create_issue", gjson.GetBytes(encoded.Content, "hooks.PreToolUse.0.matcher").String())
}

func TestDevinAdapterEncode_GroupsByEventAndMatcher(t *testing.T) {
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{Event: "before_tool_execute", Matcher: json.RawMessage(`"shell"`), Blocking: true, Handler: HookHandler{Type: "command", Command: "a"}},
			{Event: "before_tool_execute", Matcher: json.RawMessage(`"shell"`), Blocking: true, Handler: HookHandler{Type: "command", Command: "b"}},
			{Event: "before_tool_execute", Matcher: json.RawMessage(`"file_edit"`), Blocking: true, Handler: HookHandler{Type: "command", Command: "c"}},
		},
	}

	encoded, err := AdapterFor("devin").Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	groups := gjson.GetBytes(encoded.Content, "hooks.PreToolUse").Array()
	if len(groups) != 2 {
		t.Fatalf("expected 2 matcher groups, got %d: %s", len(groups), encoded.Content)
	}
	assertEqual(t, "exec", groups[0].Get("matcher").String())
	if n := len(groups[0].Get("hooks").Array()); n != 2 {
		t.Errorf("exec group: got %d entries, want 2", n)
	}
	assertEqual(t, "edit", groups[1].Get("matcher").String())
}

func TestDevinAdapterEncode_UnsupportedEventSkipped(t *testing.T) {
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{Event: "worktree_create", Handler: HookHandler{Type: "command", Command: "echo wt"}},
		},
	}

	encoded, err := AdapterFor("devin").Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if strings.Contains(string(encoded.Content), "echo wt") {
		t.Errorf("unsupported event should be skipped, got: %s", encoded.Content)
	}
	if !hasWarningContaining(encoded.Warnings, `"worktree_create" not supported`) {
		t.Errorf("expected unsupported-event warning, got %v", encoded.Warnings)
	}
}

func TestDevinAdapterEncode_NonBlockingPreToolWarns(t *testing.T) {
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{Event: "before_tool_execute", Blocking: false, Handler: HookHandler{Type: "command", Command: "echo log"}},
		},
	}

	encoded, err := AdapterFor("devin").Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !hasWarningContaining(encoded.Warnings, "non-blocking intent is not preserved") {
		t.Errorf("expected non-blocking warning, got %v", encoded.Warnings)
	}
}

func TestDevinAdapterDecode(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("testdata", "devin", "simple.json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	hooks, err := AdapterFor("devin").Decode(content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	assertEqual(t, SpecVersion, hooks.Spec)
	if len(hooks.Hooks) != 3 {
		t.Fatalf("expected 3 hooks, got %d", len(hooks.Hooks))
	}

	byCommand := map[string]CanonicalHook{}
	for _, h := range hooks.Hooks {
		byCommand[h.Handler.Command] = h
	}

	pre := byCommand["./guard.sh"]
	assertEqual(t, "before_tool_execute", pre.Event)
	assertEqual(t, `"shell"`, string(pre.Matcher))
	if pre.Handler.Timeout != 10 {
		t.Errorf("timeout: got %d, want 10 (seconds, unscaled)", pre.Handler.Timeout)
	}
	if !pre.Blocking {
		t.Error("PreToolUse hook should decode as blocking")
	}

	post := byCommand["./audit.sh"]
	assertEqual(t, "after_tool_execute", post.Event)
	assertEqual(t, `{"mcp":{"server":"github","tool":"create_issue"}}`, string(post.Matcher))
	if post.Blocking {
		t.Error("PostToolUse hook should not decode as blocking")
	}

	compact := byCommand["./compacted.sh"]
	assertEqual(t, "after_compact", compact.Event)
	if compact.Matcher != nil {
		t.Errorf("expected no matcher, got %s", compact.Matcher)
	}
	assertEqual(t, "command", compact.Handler.Type)
}

func TestDevinAdapterRoundTrip(t *testing.T) {
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{Event: "before_tool_execute", Matcher: json.RawMessage(`"file_write"`), Blocking: true, Handler: HookHandler{Type: "command", Command: "echo w", Timeout: 30}},
			{Event: "before_prompt", Handler: HookHandler{Type: "command", Command: "echo p"}},
			{Event: "permission_request", Handler: HookHandler{Type: "command", Command: "echo perm"}},
		},
	}

	adapter := AdapterFor("devin")
	encoded, err := adapter.Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if err := Verify(encoded, adapter, original); err != nil {
		t.Errorf("Verify: %v", err)
	}

	decoded, err := adapter.Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(decoded.Hooks) != len(original.Hooks) {
		t.Fatalf("round-trip: got %d hooks, want %d", len(decoded.Hooks), len(original.Hooks))
	}
	for _, h := range decoded.Hooks {
		if h.Event == "before_tool_execute" && h.Handler.Timeout != 30 {
			t.Errorf("round-trip timeout: got %d, want 30", h.Handler.Timeout)
		}
	}
}

func TestDevinAdapterCapabilities(t *testing.T) {
	caps := AdapterFor("devin").Capabilities()
	if caps.TimeoutUnit != "seconds" {
		t.Errorf("timeout unit: got %q, want seconds", caps.TimeoutUnit)
	}
	if !caps.SupportsMatchers || !caps.SupportsBlocking || !caps.SupportsStructuredOutput {
		t.Errorf("devin supports matchers, blocking, and structured output: got %+v", caps)
	}
	for _, ev := range []string{"before_tool_execute", "after_compact", "permission_request"} {
		found := false
		for _, e := range caps.Events {
			if e == ev {
				found = true
			}
		}
		if !found {
			t.Errorf("expected %q in devin events %v", ev, caps.Events)
		}
	}
}

func hasWarningContaining(warnings []ConversionWarning, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w.Description, substr) {
			return true
		}
	}
	return false
}

func TestDevinAdapterEncode_ArrayMatcherBecomesAlternation(t *testing.T) {
	// A canonical array must not collapse to "", which Devin treats as match-all.
	hooks := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{Event: "before_tool_execute", Matcher: json.RawMessage(`["shell","file_write"]`), Blocking: true, Handler: HookHandler{Type: "command", Command: "echo guard"}},
		},
	}

	encoded, err := AdapterFor("devin").Encode(hooks)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	assertEqual(t, "exec|write", gjson.GetBytes(encoded.Content, "hooks.PreToolUse.0.matcher").String())
}

func TestDevinAdapterDecode_BareHooksV1File(t *testing.T) {
	content := []byte(`{"PreToolUse": [{"matcher": "exec", "hooks": [{"type": "command", "command": "./guard.sh"}]}]}`)
	hooks, err := AdapterFor("devin").Decode(content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(hooks.Hooks) != 1 {
		t.Fatalf("expected 1 hook from a bare hooks.v1.json, got %d", len(hooks.Hooks))
	}
	assertEqual(t, "before_tool_execute", hooks.Hooks[0].Event)
}

func TestDevinAdapterDecode_SettingsWithoutHooks(t *testing.T) {
	// A config.json with other settings and no hooks key holds no hooks.
	hooks, err := AdapterFor("devin").Decode([]byte(`{"model": "swe-1.5", "permissions": {"allow": ["exec"]}}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(hooks.Hooks) != 0 {
		t.Errorf("expected no hooks, got %d", len(hooks.Hooks))
	}
}

func TestDevinAdapterRoundTrip_MixedMCPAlternation(t *testing.T) {
	// "mcp__github__create_issue|exec" must not decode as one MCP tool named
	// "create_issue|exec", or the exec part never translates to other providers.
	original := &CanonicalHooks{
		Spec: SpecVersion,
		Hooks: []CanonicalHook{
			{Event: "before_tool_execute", Matcher: json.RawMessage(`[{"mcp":{"server":"github","tool":"create_issue"}},"shell"]`), Blocking: true, Handler: HookHandler{Type: "command", Command: "echo guard"}},
		},
	}
	encoded, err := AdapterFor("devin").Encode(original)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	assertEqual(t, "mcp__github__create_issue|exec", gjson.GetBytes(encoded.Content, "hooks.PreToolUse.0.matcher").String())

	decoded, err := AdapterFor("devin").Decode(encoded.Content)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	cc, err := AdapterFor("claude-code").Encode(decoded)
	if err != nil {
		t.Fatalf("Encode claude-code: %v", err)
	}
	assertEqual(t, "mcp__github__create_issue|Bash", gjson.GetBytes(cc.Content, "hooks.PreToolUse.0.matcher").String())
}
