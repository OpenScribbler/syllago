package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realGeminiCliRulesLandmarks is a snapshot of headings extracted from
// .capmon-cache/gemini-cli/rules.0/extracted.json (gemini-md.md) as of
// 2026-04-16. Update when the doc evolves.
var realGeminiCliRulesLandmarks = []string{
	"Provide context with GEMINI.md files",
	"Understand the context hierarchy",
	"Example GEMINI.md file",
	"Manage context with the /memory command",
	"Modularize context with imports",
	"Customize the context file name",
	"Next steps",
}

// realGeminiCliHooksLandmarks is a snapshot of the merged headings from
// .capmon-cache/gemini-cli/hooks.{2,3}/extracted.json (docs/hooks/index.md
// and docs/hooks/reference.md) as of 2026-04-16. Update when the docs evolve.
var realGeminiCliHooksLandmarks = []string{
	// hooks.2 (index.md)
	"Gemini CLI hooks",
	"What are hooks?",
	"Getting started",
	"Core concepts",
	"Hook events",
	"Global mechanics",
	"Strict JSON requirements (The \"Golden Rule\")",
	"Exit codes",
	"Matchers",
	"Configuration",
	"Configuration schema",
	"Hook configuration fields",
	"Environment variables",
	"Security and risks",
	"Managing hooks",
	// hooks.3 (reference.md)
	"Hooks reference",
	"Global hook mechanics",
	"Hook definition",
	"Hook configuration",
	"Base input schema",
	"Common output fields",
	"Tool hooks",
	"Matchers and tool names",
	"BeforeTool",
	"AfterTool",
	"Agent hooks",
	"BeforeAgent",
	"AfterAgent",
	"Model hooks",
	"BeforeModel",
	"BeforeToolSelection",
	"AfterModel",
	"Lifecycle & system hooks",
	"SessionStart",
	"SessionEnd",
	"Notification",
	"PreCompress",
	"Stable Model API",
}

func TestRecognizeGeminiCli_RealRulesLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider:  "gemini-cli",
		Format:    "markdown",
		Landmarks: realGeminiCliRulesLandmarks,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["rules.supported"] != "true" {
		t.Error("rules.supported missing")
	}
	rulesInferred := []string{
		"activation_mode.always_on",
		"file_imports",
		"auto_memory",
		"hierarchical_loading",
	}
	for _, c := range rulesInferred {
		key := "rules.capabilities." + c + ".supported"
		if caps[key] != "true" {
			t.Errorf("%s missing", key)
		}
		if got := caps["rules.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("rules.%s.confidence = %q, want inferred", c, got)
		}
	}
	// Per seeder spec, gemini-cli does NOT document AGENTS.md or other
	// foreign-format recognition.
	if _, has := caps["rules.capabilities.cross_provider_recognition.agents_md.supported"]; has {
		t.Error("cross_provider_recognition.agents_md should NOT be present for gemini-cli")
	}
}

func TestRecognizeGeminiCli_AnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realGeminiCliRulesLandmarks))
	for _, lm := range realGeminiCliRulesLandmarks {
		if lm == "Understand the context hierarchy" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider:  "gemini-cli",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
}

func TestRecognizeGeminiCli_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider: "gemini-cli",
		Format:   "markdown",
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
}

// TestRecognizeGeminiCli_RealHooksLandmarks proves hooks recognition on the
// merged rules+hooks landmarks. Per the curated format YAML, only 3 of the 9
// canonical hooks keys are supported in gemini-cli: matcher_patterns,
// decision_control, json_io_protocol. handler_types is intentionally absent
// because gemini-cli only supports shell handlers.
func TestRecognizeGeminiCli_RealHooksLandmarks(t *testing.T) {
	merged := append([]string{}, realGeminiCliRulesLandmarks...)
	merged = append(merged, realGeminiCliHooksLandmarks...)
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider:  "gemini-cli",
		Format:    "markdown",
		Landmarks: merged,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["hooks.supported"] != "true" {
		t.Error("hooks.supported missing")
	}
	hooksInferred := []string{
		"matcher_patterns",
		"decision_control",
		"json_io_protocol",
	}
	for _, c := range hooksInferred {
		key := "hooks.capabilities." + c + ".supported"
		if caps[key] != "true" {
			t.Errorf("%s missing", key)
		}
		if got := caps["hooks.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("hooks.%s.confidence = %q, want inferred", c, got)
		}
	}
	for _, absent := range []string{
		"hooks.capabilities.handler_types.supported",
		"hooks.capabilities.async_execution.supported",
		"hooks.capabilities.hook_scopes.supported",
		"hooks.capabilities.context_injection.supported",
		"hooks.capabilities.permission_control.supported",
		"hooks.capabilities.input_modification.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present for gemini-cli (no heading evidence or curated as unsupported)", absent)
		}
	}
}
