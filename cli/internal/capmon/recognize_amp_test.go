package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realAmpLandmarks is a snapshot of the headings extracted from amp's skills doc
// (.capmon-cache/amp/skills.0/extracted.json) as of 2026-04-16. Update when the
// upstream doc evolves.
var realAmpLandmarks = []string{
	"Agent Skills",
	"Creating Skills",
	"Installing Skills",
	"Skill Format",
	"MCP Servers in Skills",
	"Executable Tools in Skills",
}

// realAmpRulesLandmarks is a snapshot of the rules-relevant headings from
// amp's HTML Owner's Manual (.capmon-cache/amp/rules.2/extracted.json) as of
// 2026-04-16. Includes the trailing "##" anchor-link suffixes that amp's HTML
// extractor emits — the recognizer's substring matchers handle them
// transparently. rules.0 and rules.1 are example AGENTS.md instances, not the
// spec, so they are intentionally not included here.
var realAmpRulesLandmarks = []string{
	"AGENTS.md##",
	"Writing AGENTS.md Files##",
	"Granular Guidance##",
	"Migrating to AGENTS.md",
	"Handoff##",
	"Tools##",
}

// realAmpHooksLandmarks is a snapshot of the headings from amp's permissions
// reference doc (.capmon-cache/amp/hooks.1/extracted.json — permissions-
// reference.md) as of 2026-04-16. The hooks.0 doc (hooks.md) emits only a
// single landmark "Hooks" — too thin to anchor on — so amp's hooks recognition
// uses the permissions doc instead. Update when the upstream doc evolves.
var realAmpHooksLandmarks = []string{
	"Permissions Reference",
	"How Permissions Work",
	"Configuration",
	"Match Conditions",
	"Regular Expression Patterns",
	"Value Type Matching",
	"Examples",
	"Basic Permission Rules",
	"Delegation",
	"Text Format",
	"Listing Rules",
	"Testing Rules",
	"Editing Rules",
	"Add Rules",
	"Matching multiple tools with a single rule",
	"Context Restrictions",
}

func TestRecognizeAmp_RealLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("amp", capmon.RecognitionContext{
		Provider:  "amp",
		Format:    "markdown",
		Landmarks: realAmpLandmarks,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q", result.Status, capmon.StatusRecognized)
	}
	caps := result.Capabilities
	if caps["skills.supported"] != "true" {
		t.Error("skills.supported missing")
	}
	for _, c := range []string{"creation_workflow", "installation_workflow", "mcp_integration", "executable_tools"} {
		if caps["skills.capabilities."+c+".supported"] != "true" {
			t.Errorf("%s.supported missing", c)
		}
		if got := caps["skills.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("%s.confidence = %q, want inferred", c, got)
		}
	}
	for _, c := range []string{"project_scope", "global_scope", "canonical_filename"} {
		if caps["skills.capabilities."+c+".confidence"] != "confirmed" {
			t.Errorf("%s.confidence = %q, want confirmed", c, caps["skills.capabilities."+c+".confidence"])
		}
	}
}

func TestRecognizeAmp_AnchorsMissing(t *testing.T) {
	// Strip "Skill Format" — passing-mention guardrail.
	mutated := []string{}
	for _, lm := range realAmpLandmarks {
		if lm == "Skill Format" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("amp", capmon.RecognitionContext{
		Provider:  "amp",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities, got %d", len(result.Capabilities))
	}
}

func TestRecognizeAmp_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("amp", capmon.RecognitionContext{Provider: "amp", Format: "markdown"})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities, got %d", len(result.Capabilities))
	}
}

// TestRecognizeAmp_RealRulesLandmarks proves rules recognition on the merged
// skills+rules landmarks. amp's HTML extractor emits anchor-link suffixes
// ("AGENTS.md##") which substring matchers handle transparently.
func TestRecognizeAmp_RealRulesLandmarks(t *testing.T) {
	merged := append([]string{}, realAmpLandmarks...)
	merged = append(merged, realAmpRulesLandmarks...)
	result := capmon.RecognizeWithContext("amp", capmon.RecognitionContext{
		Provider:  "amp",
		Format:    "markdown",
		Landmarks: merged,
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
		"activation_mode.frontmatter_globs",
		"file_imports",
		"cross_provider_recognition.agents_md",
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
	// auto_memory must NOT be emitted — amp docs do not document an
	// agent-managed memory feature (Handoff is for inter-thread context, not
	// auto-memory).
	if _, has := caps["rules.capabilities.auto_memory.supported"]; has {
		t.Error("rules.capabilities.auto_memory should NOT be present for amp")
	}
}

// TestRecognizeAmp_RealHooksLandmarks proves hooks recognition emits the 2
// canonical hooks keys curated as supported in the format YAML: matcher_patterns
// (per-tool input matching via "Match Conditions" + "Regular Expression
// Patterns") and permission_control (hooks integrate with the permission system
// via "How Permissions Work" + "Add Rules"). The other 7 canonical keys are
// curated as unsupported and must NOT be emitted. Test merges all three content
// type fixtures to verify cross-content-type robustness.
func TestRecognizeAmp_RealHooksLandmarks(t *testing.T) {
	merged := append([]string{}, realAmpLandmarks...)
	merged = append(merged, realAmpRulesLandmarks...)
	merged = append(merged, realAmpHooksLandmarks...)
	result := capmon.RecognizeWithContext("amp", capmon.RecognitionContext{
		Provider:  "amp",
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
		"permission_control",
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
		"hooks.capabilities.decision_control.supported",
		"hooks.capabilities.async_execution.supported",
		"hooks.capabilities.hook_scopes.supported",
		"hooks.capabilities.json_io_protocol.supported",
		"hooks.capabilities.context_injection.supported",
		"hooks.capabilities.input_modification.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present for amp (curated as unsupported)", absent)
		}
	}
}

// TestRecognizeAmp_HooksAnchorsMissing proves the required-anchor guard
// suppresses hooks emission when "Permissions Reference" is absent — the
// "How Permissions Work" matcher would otherwise fire on rules-only contexts.
func TestRecognizeAmp_HooksAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realAmpHooksLandmarks))
	for _, lm := range realAmpHooksLandmarks {
		if lm == "Permissions Reference" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("amp", capmon.RecognitionContext{
		Provider:  "amp",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["hooks.supported"]; has {
		t.Error("hooks.supported should NOT be present when 'Permissions Reference' anchor is missing")
	}
}
