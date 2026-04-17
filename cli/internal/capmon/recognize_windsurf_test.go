package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realWindsurfRulesLandmarks is a snapshot of headings extracted from
// .capmon-cache/windsurf/rules.0/extracted.json (Memories & Rules doc) and
// .capmon-cache/windsurf/rules.1/extracted.json (AGENTS.md doc) as of
// 2026-04-16. Update when the docs evolve.
var realWindsurfRulesLandmarks = []string{
	// rules.0 — Memories & Rules
	"Documentation Index",
	"Memories & Rules",
	"Memories, Rules, Workflows, or Skills?",
	"How to Manage Memories",
	"Memories",
	"Rules",
	"Rules Discovery",
	"Rules Storage Locations",
	"Activation Modes",
	"Best Practices",
	"System-Level Rules (Enterprise)",
	"How System Rules Work",
	// rules.1 — AGENTS.md
	"AGENTS.md",
	"How It Works",
	"Creating an AGENTS.md File",
	"Discovery and Scoping",
	"Automatic Scoping",
	"Comparison with Rules",
}

// TestRecognizeWindsurf_RealRulesLandmarks proves the canary path: feeding the
// recognizer the real merged rules landmarks (from rules.0 + rules.1)
// produces all expected rules capability dot-paths at confidence "inferred".
func TestRecognizeWindsurf_RealRulesLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("windsurf", capmon.RecognitionContext{
		Provider:  "windsurf",
		Format:    "markdown",
		Landmarks: realWindsurfRulesLandmarks,
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
		"activation_mode.manual",
		"activation_mode.model_decision",
		"activation_mode.glob",
		"cross_provider_recognition.agents_md",
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
	// file_imports must NOT be emitted — windsurf docs do not document
	// cross-file import syntax (per seeder spec; XML grouping is in-file).
	if _, has := caps["rules.capabilities.file_imports.supported"]; has {
		t.Error("rules.capabilities.file_imports should NOT be present for windsurf")
	}
}

// TestRecognizeWindsurf_AnchorsMissing proves the negative path: stripping a
// required anchor suppresses all rules patterns and surfaces the missing
// anchor name.
func TestRecognizeWindsurf_AnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realWindsurfRulesLandmarks))
	for _, lm := range realWindsurfRulesLandmarks {
		if lm == "Rules Discovery" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("windsurf", capmon.RecognitionContext{
		Provider:  "windsurf",
		Format:    "markdown",
		Landmarks: mutated,
	})

	if result.Status != capmon.StatusAnchorsMissing {
		t.Fatalf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if _, has := result.Capabilities["rules.supported"]; has {
		t.Error("rules.supported should be absent when required anchor missing")
	}
	found := false
	for _, m := range result.MissingAnchors {
		if m == "Rules Discovery" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("MissingAnchors %v does not include 'Rules Discovery'", result.MissingAnchors)
	}
}

// TestRecognizeWindsurf_NoLandmarks proves an empty landmark list produces
// "anchors_missing" status with no capabilities.
func TestRecognizeWindsurf_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("windsurf", capmon.RecognitionContext{
		Provider: "windsurf",
		Format:   "markdown",
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities, got %d", len(result.Capabilities))
	}
}

// realWindsurfHooksLandmarks is a snapshot of the H2/H3 headings from
// .capmon-cache/windsurf/hooks.0/extracted.json (cascade/hooks.md) as of
// 2026-04-16. Update when the doc evolves.
var realWindsurfHooksLandmarks = []string{
	"Documentation Index",
	"Cascade Hooks",
	"What You Can Build",
	"How Hooks Work",
	"Configuration",
	"System-Level",
	"User-Level",
	"Workspace-Level",
	"Basic Structure",
	"Configuration Options",
	"Cross-Platform Behavior",
	"Hook Events",
	"Common Input Structure",
	"pre_read_code",
	"post_read_code",
	"pre_write_code",
	"post_write_code",
	"pre_run_command",
	"post_run_command",
	"pre_mcp_tool_use",
	"post_mcp_tool_use",
	"pre_user_prompt",
	"post_cascade_response",
	"post_cascade_response_with_transcript",
	"post_setup_worktree",
	"Exit Codes",
	"Best Practices",
	"Security",
	"Enterprise Distribution",
}

// TestRecognizeWindsurf_RealHooksLandmarks proves hooks recognition emits the
// 2 canonical hooks keys curated as supported in the format YAML: hook_scopes
// (System/User/Workspace three-tier scopes) and json_io_protocol (JSON event
// context via stdin). The other 7 canonical keys are curated as unsupported.
func TestRecognizeWindsurf_RealHooksLandmarks(t *testing.T) {
	merged := append([]string{}, realWindsurfRulesLandmarks...)
	merged = append(merged, realWindsurfHooksLandmarks...)
	result := capmon.RecognizeWithContext("windsurf", capmon.RecognitionContext{
		Provider:  "windsurf",
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
		"hook_scopes",
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
		"hooks.capabilities.matcher_patterns.supported",
		"hooks.capabilities.decision_control.supported",
		"hooks.capabilities.async_execution.supported",
		"hooks.capabilities.context_injection.supported",
		"hooks.capabilities.permission_control.supported",
		"hooks.capabilities.input_modification.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present for windsurf (curated as unsupported)", absent)
		}
	}
}
