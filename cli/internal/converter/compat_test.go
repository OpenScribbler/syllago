package converter

import (
	"slices"
	"testing"
)

func TestHookProviders_ListsEveryAdapter(t *testing.T) {
	t.Parallel()
	got := HookProviders()
	if len(got) != len(adapterRegistry) {
		t.Fatalf("HookProviders() = %v, want all %d adapters", got, len(adapterRegistry))
	}
	for _, slug := range []string{"cursor", "devin", "factory-droid", "pi"} {
		if !slices.Contains(got, slug) {
			t.Errorf("HookProviders() is missing %s", slug)
		}
	}
}

func TestCompatLevel_Symbol(t *testing.T) {
	t.Parallel()
	cases := []struct {
		level CompatLevel
		sym   string
	}{
		{CompatFull, "✓"},
		{CompatDegraded, "~"},
		{CompatBroken, "!"},
		{CompatNone, "✗"},
	}
	for _, tc := range cases {
		if got := tc.level.Symbol(); got != tc.sym {
			t.Errorf("level %d: got %q want %q", tc.level, got, tc.sym)
		}
	}
}

func TestCompatLevel_Label(t *testing.T) {
	t.Parallel()
	cases := []struct {
		level CompatLevel
		label string
	}{
		{CompatFull, "Full"},
		{CompatDegraded, "Degraded"},
		{CompatBroken, "Broken"},
		{CompatNone, "None"},
	}
	for _, tc := range cases {
		if got := tc.level.Label(); got != tc.label {
			t.Errorf("level %d: got %q want %q", tc.level, got, tc.label)
		}
	}
}

func TestAnalyzeHookCompat_FullCompat(t *testing.T) {
	t.Parallel()
	hook := HookData{
		Event:   "before_tool_execute",
		Matcher: "shell",
		Hooks:   []HookEntry{{Type: "command", Command: "go vet ./..."}},
	}

	r := AnalyzeHookCompat(hook, "claude-code")
	if r.Level != CompatFull {
		t.Errorf("claude-code: expected Full, got %v", r.Level)
	}

	r2 := AnalyzeHookCompat(hook, "gemini-cli")
	if r2.Level != CompatFull {
		t.Errorf("gemini-cli: expected Full, got %v", r2.Level)
	}
}

func TestAnalyzeHookCompat_BrokenMatcher_Copilot(t *testing.T) {
	t.Parallel()
	hook := HookData{
		Event:   "before_tool_execute",
		Matcher: "shell",
		Hooks:   []HookEntry{{Type: "command", Command: "go vet ./..."}},
	}
	r := AnalyzeHookCompat(hook, "copilot-cli")
	if r.Level != CompatBroken {
		t.Errorf("expected Broken, got %v", r.Level)
	}
	// Verify FeatureMatcher is in the results
	foundMatcher := false
	for _, fr := range r.Features {
		if fr.Feature == FeatureMatcher && !fr.Supported {
			foundMatcher = true
		}
	}
	if !foundMatcher {
		t.Error("expected FeatureMatcher in broken features")
	}
}

func TestAnalyzeHookCompat_NoneEvent(t *testing.T) {
	t.Parallel()
	hook := HookData{
		Event: "subagent_start",
		Hooks: []HookEntry{{Type: "command", Command: "echo hi"}},
	}
	for _, target := range []string{"gemini-cli", "copilot-cli", "kiro"} {
		t.Run(target, func(t *testing.T) {
			r := AnalyzeHookCompat(hook, target)
			if r.Level != CompatNone {
				t.Errorf("expected None, got %v", r.Level)
			}
		})
	}
}

func TestAnalyzeHookCompat_LLMHook_NoneForNonClaude(t *testing.T) {
	t.Parallel()
	hook := HookData{
		Event: "before_tool_execute",
		Hooks: []HookEntry{{Type: "prompt", Command: "Is this safe?"}},
	}
	for _, target := range []string{"gemini-cli", "copilot-cli", "kiro"} {
		t.Run(target, func(t *testing.T) {
			r := AnalyzeHookCompat(hook, target)
			if r.Level != CompatNone {
				t.Errorf("expected None for LLM hook, got %v", r.Level)
			}
		})
	}
}

func TestAnalyzeHookCompat_StatusMessage_Kiro_Degraded(t *testing.T) {
	t.Parallel()
	hook := HookData{
		Event: "before_tool_execute",
		Hooks: []HookEntry{{Type: "command", Command: "echo hi", StatusMessage: "Working..."}},
	}
	r := AnalyzeHookCompat(hook, "kiro")
	if r.Level != CompatDegraded {
		t.Errorf("expected Degraded, got %v", r.Level)
	}
}

func TestAnalyzeHookCompat_Async_Kiro_Broken(t *testing.T) {
	t.Parallel()
	hook := HookData{
		Event: "after_tool_execute",
		Hooks: []HookEntry{{Type: "command", Command: "echo done", Async: true}},
	}
	r := AnalyzeHookCompat(hook, "kiro")
	if r.Level != CompatBroken {
		t.Errorf("expected Broken, got %v", r.Level)
	}
}

func TestAnalyzeHookCompat_Async_Copilot_Broken(t *testing.T) {
	t.Parallel()
	hook := HookData{
		Event: "before_tool_execute",
		Hooks: []HookEntry{{Type: "command", Command: "echo check", Async: true}},
	}
	r := AnalyzeHookCompat(hook, "copilot-cli")
	if r.Level != CompatBroken {
		t.Errorf("expected Broken, got %v", r.Level)
	}
}

func TestAnalyzeHookCompat_NoMatcher_FullEverywhere(t *testing.T) {
	t.Parallel()
	// Hook with no matcher, no async, no statusMessage — should be Full
	// everywhere. Uses before_tool_execute: the only event every hook
	// provider supports (crush supports nothing else).
	hook := HookData{
		Event: "before_tool_execute",
		Hooks: []HookEntry{{Type: "command", Command: "echo start"}},
	}
	for _, target := range HookProviders() {
		t.Run(target, func(t *testing.T) {
			r := AnalyzeHookCompat(hook, target)
			if r.Level != CompatFull {
				t.Errorf("expected Full for %s, got %v", target, r.Level)
			}
		})
	}
}

func TestAnalyzeHookCompat_UnknownProvider(t *testing.T) {
	t.Parallel()
	hook := HookData{
		Event: "before_tool_execute",
		Hooks: []HookEntry{{Type: "command", Command: "echo"}},
	}
	r := AnalyzeHookCompat(hook, "nonexistent-provider")
	if r.Level != CompatNone {
		t.Errorf("expected None for unknown provider, got %v", r.Level)
	}
}

func TestOutputFields_VSCodeCopilot(t *testing.T) {
	t.Parallel()
	got := AdapterFor("vs-code-copilot").Capabilities().OutputFields
	for _, field := range AllOutputFields {
		if !slices.Contains(got, field) {
			t.Errorf("expected vs-code-copilot to support output field %q", field)
		}
	}
}

// The VS Code adapter writes only command hooks, so compat must not promise
// a prompt hook will work there.
func TestAnalyzeHookCompat_LLMHook_NoneForVSCodeCopilot(t *testing.T) {
	t.Parallel()
	hook := HookData{
		Event: "before_tool_execute",
		Hooks: []HookEntry{{Type: "prompt", Command: "Is this safe?"}},
	}
	if r := AnalyzeHookCompat(hook, "vs-code-copilot"); r.Level != CompatNone {
		t.Errorf("expected None, got %v (%s)", r.Level, r.Notes)
	}
}

func TestAnalyzeHookCompat_VSCodeCopilotFull(t *testing.T) {
	t.Parallel()
	hook := HookData{
		Event:   "before_tool_execute",
		Matcher: "shell",
		Hooks:   []HookEntry{{Type: "command", Command: "echo check", StatusMessage: "Checking...", Timeout: 5}},
	}
	r := AnalyzeHookCompat(hook, "vs-code-copilot")
	if r.Level != CompatFull {
		t.Errorf("expected Full compat for vs-code-copilot, got %v", r.Level)
	}
}

// Pi compares tool names exactly: a wildcard and an alternation encode, and
// any other regular expression leaves the hook matching every tool.
func TestAnalyzeHookCompat_PiMatcherShapes(t *testing.T) {
	cases := []struct {
		matcher string
		want    CompatLevel
	}{
		{"shell", CompatFull},
		{"*", CompatFull},
		{"shell|file_read", CompatFull},
		{"file_.*", CompatBroken},
	}
	for _, tc := range cases {
		t.Run(tc.matcher, func(t *testing.T) {
			hook := HookData{
				Event:   "before_tool_execute",
				Matcher: tc.matcher,
				Hooks:   []HookEntry{{Type: "command", Command: "./check.sh"}},
			}
			if r := AnalyzeHookCompat(hook, "pi"); r.Level != tc.want {
				t.Errorf("level = %v, want %v (%+v)", r.Level, tc.want, r.Features)
			}
		})
	}
}
