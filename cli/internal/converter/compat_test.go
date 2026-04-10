package converter

import "testing"

func TestHookCapabilities_AllProvidersPresent(t *testing.T) {
	t.Parallel()
	for _, slug := range HookProviders() {
		if _, ok := HookCapabilities[slug]; !ok {
			t.Errorf("HookCapabilities missing provider %q", slug)
		}
	}
}

func TestHookCapabilities_AllFeaturesPresent(t *testing.T) {
	t.Parallel()
	allFeatures := []HookFeature{FeatureMatcher, FeatureAsync, FeatureStatusMessage, FeatureLLMHook, FeatureTimeout}
	for slug, cap := range HookCapabilities {
		for _, f := range allFeatures {
			if _, ok := cap.Features[f]; !ok {
				t.Errorf("provider %q missing feature %d", slug, f)
			}
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
	// Hook with no matcher, no async, no statusMessage — should be Full everywhere
	hook := HookData{
		Event: "session_start",
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

func TestHookOutputCapabilities_VSCodeCopilot(t *testing.T) {
	t.Parallel()
	caps, ok := HookOutputCapabilities["vs-code-copilot"]
	if !ok {
		t.Fatal("expected vs-code-copilot in HookOutputCapabilities")
	}
	for _, field := range AllOutputFields {
		if !caps[field] {
			t.Errorf("expected vs-code-copilot to support output field %q", field)
		}
	}
}

func TestHookCapabilities_VSCodeCopilot(t *testing.T) {
	t.Parallel()
	cap, ok := HookCapabilities["vs-code-copilot"]
	if !ok {
		t.Fatal("expected vs-code-copilot in HookCapabilities")
	}
	// VS Code Copilot should support all features like Claude Code
	for _, feat := range []HookFeature{FeatureMatcher, FeatureAsync, FeatureStatusMessage, FeatureLLMHook, FeatureTimeout} {
		fs := cap.Features[feat]
		if !fs.Supported {
			t.Errorf("expected vs-code-copilot to support feature %d", feat)
		}
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
