package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realFactoryDroidLandmarks is a snapshot of the headings extracted from
// Factory Droid's skills doc (.capmon-cache/factory-droid/skills.0/extracted.json)
// as of 2026-04-17, after the source manifest switched to
// fetch_method=chromedp + format=html for docs.factory.ai pages (Mintlify SPA).
//
// Mintlify landmarks have a leading zero-width space prefix (e.g.
// "​Skill file format") which substring matchers handle transparently.
// Update this fixture when the upstream doc evolves.
var realFactoryDroidLandmarks = []string{
	"Skills",
	"\u200bWhat is a skill?",
	"\u200bSkill file format",
	"\u200bWhere skills live",
	"\u200bFrontmatter reference",
	"\u200bControl who invokes a skill",
	"\u200bInvocation summary",
	"\u200bQuickstart",
	"\u200bHow skills differ from other configuration",
	"\u200bWhy skills matter in enterprise codebases",
	"\u200bBest practices",
	"\u200bCookbook",
}

// realFactoryDroidHooksLandmarks is a snapshot of the headings extracted from
// Factory Droid's hooks reference doc
// (.capmon-cache/factory-droid/hooks.1/extracted.json) as of 2026-04-17.
//
// Per the curated format YAML (docs/provider-formats/factory-droid.yaml),
// only decision_control is supported among the 9 canonical hooks keys —
// Factory Droid hooks signal via exit codes. The other 8 keys are curated as
// unsupported and must NOT be emitted.
var realFactoryDroidHooksLandmarks = []string{
	"Hooks reference",
	"\u200bConfiguration",
	"\u200bHook Events",
	"\u200bPreToolUse",
	"\u200bPostToolUse",
	"\u200bUserPromptSubmit",
	"\u200bStop",
	"\u200bSessionStart",
	"\u200bHook Output",
	"\u200bSimple: Exit Code",
	"\u200bExit Code 2 Behavior",
	"\u200bAdvanced: JSON Output",
	"\u200bPreToolUse Decision Control",
	"\u200bPostToolUse Decision Control",
	"\u200bUserPromptSubmit Decision Control",
	"\u200bStop/SubagentStop Decision Control",
	"\u200bSessionStart Decision Control",
	"\u200bSessionEnd Decision Control",
}

// TestRecognizeFactoryDroid_RealLandmarks verifies skills recognition fires
// against the actual cache snapshot. Static facts (project_scope, global_scope,
// canonical_filename) merge in at "confirmed" after the landmark match.
func TestRecognizeFactoryDroid_RealLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: realFactoryDroidLandmarks,
	})
	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["skills.supported"] != "true" {
		t.Error("skills.supported missing")
	}
	for _, c := range []string{"frontmatter", "creation_workflow", "directory_structure", "invocation"} {
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

// TestRecognizeFactoryDroid_AnchorsMissing verifies the required-anchor guard
// suppresses skills emission when one of the unique skills anchors is absent.
func TestRecognizeFactoryDroid_AnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realFactoryDroidLandmarks))
	for _, lm := range realFactoryDroidLandmarks {
		if lm == "\u200bSkill file format" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if _, has := result.Capabilities["skills.supported"]; has {
		t.Error("skills.supported should NOT be present when 'Skill file format' anchor is missing")
	}
}

// TestRecognizeFactoryDroid_NoLandmarks verifies clean suppression with no
// landmark input.
func TestRecognizeFactoryDroid_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{Provider: "factory-droid", Format: "markdown"})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities, got %d", len(result.Capabilities))
	}
}

// TestRecognizeFactoryDroid_RealHooksLandmarks proves hooks recognition fires
// against the merged skills + hooks cache snapshot, emits the curated-supported
// decision_control capability at "inferred" confidence (the only canonical
// hooks key Factory Droid supports), and does NOT emit the 8 curated-as-
// unsupported keys.
func TestRecognizeFactoryDroid_RealHooksLandmarks(t *testing.T) {
	merged := append([]string{}, realFactoryDroidLandmarks...)
	merged = append(merged, realFactoryDroidHooksLandmarks...)
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
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
	if caps["hooks.capabilities.decision_control.supported"] != "true" {
		t.Error("hooks.capabilities.decision_control.supported missing")
	}
	if got := caps["hooks.capabilities.decision_control.confidence"]; got != "inferred" {
		t.Errorf("hooks.capabilities.decision_control.confidence = %q, want inferred", got)
	}
	for _, absent := range []string{
		"hooks.capabilities.handler_types.supported",
		"hooks.capabilities.matcher_patterns.supported",
		"hooks.capabilities.input_modification.supported",
		"hooks.capabilities.async_execution.supported",
		"hooks.capabilities.hook_scopes.supported",
		"hooks.capabilities.json_io_protocol.supported",
		"hooks.capabilities.context_injection.supported",
		"hooks.capabilities.permission_control.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present for factory-droid (curated as unsupported)", absent)
		}
	}
}

// realFactoryDroidAgentsLandmarks is a snapshot of the headings extracted from
// Factory Droid's "Custom Droids (Subagents)" doc
// (.capmon-cache/factory-droid/agents.0/extracted.json) as of 2026-04-17.
// Mintlify zero-width space prefix on H2/H3/H4 entries handled transparently
// by substring matchers.
var realFactoryDroidAgentsLandmarks = []string{
	"Custom Droids (Subagents)",
	"\u200b1 · What are custom droids?",
	"\u200b2 · Why use them?",
	"\u200b3 · Quick start",
	"\u200b4 · Configuration",
	"\u200bTool categories → concrete tools",
	"\u200b5 · Managing droids in the UI",
	"\u200b5.5 · Importing Claude Code subagents",
	"\u200bHow to import",
	"\u200bWhat happens during import",
	"\u200bExample import flow",
	"\u200bHandling tool validation errors",
	"\u200b6 · Using custom droids effectively",
	"\u200b7 · Examples",
	"\u200bCode reviewer (project scope)",
	"\u200bSecurity sweeper (personal scope)",
	"\u200bTask coordinator (with live progress)",
}

// TestRecognizeFactoryDroid_RealAgentsLandmarks proves agents recognition
// emits 2 canonical agents keys at "inferred" confidence: definition_format
// and tool_restrictions. The other 5 (invocation_patterns, agent_scopes,
// model_selection, per_agent_mcp, subagent_spawning) must NOT be emitted —
// no heading-level evidence in the agents doc, even though the curator marks
// most of them supported from broader source knowledge.
//
// Test merges skills + hooks + agents fixtures to mirror real-world cache
// merging — the agents recognizer must distinguish its capabilities from
// the others via the required-anchor uniqueness gate.
func TestRecognizeFactoryDroid_RealAgentsLandmarks(t *testing.T) {
	merged := append([]string{}, realFactoryDroidLandmarks...)
	merged = append(merged, realFactoryDroidHooksLandmarks...)
	merged = append(merged, realFactoryDroidAgentsLandmarks...)
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: merged,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["agents.supported"] != "true" {
		t.Error("agents.supported missing")
	}
	for _, c := range []string{"definition_format", "tool_restrictions"} {
		key := "agents.capabilities." + c + ".supported"
		if caps[key] != "true" {
			t.Errorf("%s missing", key)
		}
		if got := caps["agents.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("agents.%s.confidence = %q, want inferred", c, got)
		}
	}
	for _, absent := range []string{
		"agents.capabilities.invocation_patterns.supported",
		"agents.capabilities.agent_scopes.supported",
		"agents.capabilities.model_selection.supported",
		"agents.capabilities.per_agent_mcp.supported",
		"agents.capabilities.subagent_spawning.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present (no heading evidence; curator-only)", absent)
		}
	}
}

// TestRecognizeFactoryDroid_AgentsAnchorsMissing proves the required-anchor
// guard suppresses agents emission when "Tool categories" is absent —
// preventing agents patterns from firing on contexts that mention only the
// parent "Custom Droids" landmark.
func TestRecognizeFactoryDroid_AgentsAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realFactoryDroidAgentsLandmarks))
	for _, lm := range realFactoryDroidAgentsLandmarks {
		if lm == "\u200bTool categories → concrete tools" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["agents.supported"]; has {
		t.Error("agents.supported should NOT be present when 'Tool categories' anchor is missing")
	}
}

// TestRecognizeFactoryDroid_HooksAnchorsMissing proves the required-anchor
// guard suppresses hooks emission when "Hooks reference" is absent — without
// the guard, the substring "Decision Control" pattern would fire on any
// content type cached from a doc that mentions decision control.
func TestRecognizeFactoryDroid_HooksAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realFactoryDroidHooksLandmarks))
	for _, lm := range realFactoryDroidHooksLandmarks {
		if lm == "Hooks reference" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["hooks.supported"]; has {
		t.Error("hooks.supported should NOT be present when 'Hooks reference' anchor is missing")
	}
}
