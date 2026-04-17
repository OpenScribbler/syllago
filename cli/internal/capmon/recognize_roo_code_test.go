package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realRooCodeAgentsLandmarks is a snapshot of landmarks extracted from
// roo-code's three agents caches as of 2026-04-16:
//   - agents.0 (TypeScript: packages/types/src/mode.ts) — type names
//   - agents.1 (TypeScript: src/core/config/CustomModesManager.ts) —
//     export/import type names
//   - agents.2 (YAML instance: .roomodes) — root key
//
// Combined fixture mimics the merged-cache state the recognizer sees in
// production. ModeConfig + customModes are the required-anchor pair for
// agents recognition.
var realRooCodeAgentsLandmarks = []string{
	// agents.0 (TS types)
	"GroupOptions",
	"GroupEntry",
	"ModeConfig",
	"CustomModesSettings",
	"PromptComponent",
	"CustomModePrompts",
	"CustomSupportPrompts",
	// agents.1 (TS export/import types)
	"RuleFile",
	"ExportedModeConfig",
	"ImportData",
	"ExportResult",
	"ImportResult",
	// agents.2 (YAML root key)
	"customModes",
}

// TestRecognizeRooCode_RealAgentsLandmarks proves agents recognition emits
// 3 canonical agents keys at "inferred" confidence: definition_format,
// tool_restrictions, and the nested agent_scopes (.project + .user). Four
// other canonical keys (invocation_patterns, per_agent_mcp, model_selection,
// subagent_spawning) must NOT be emitted — none have type or YAML evidence.
func TestRecognizeRooCode_RealAgentsLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("roo-code", capmon.RecognitionContext{
		Provider:  "roo-code",
		Format:    "typescript",
		Landmarks: realRooCodeAgentsLandmarks,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["agents.supported"] != "true" {
		t.Error("agents.supported missing")
	}
	agentsInferred := []string{
		"definition_format",
		"tool_restrictions",
		"agent_scopes.project",
		"agent_scopes.user",
	}
	for _, c := range agentsInferred {
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
		"agents.capabilities.per_agent_mcp.supported",
		"agents.capabilities.model_selection.supported",
		"agents.capabilities.subagent_spawning.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present (no type or YAML evidence)", absent)
		}
	}
}

// TestRecognizeRooCode_AgentsAnchorsMissing proves the required-anchor guard
// suppresses agents emission when "ModeConfig" substring evidence is absent.
// ModeConfig is the TypeScript type-name anchor that distinguishes agents
// evidence from the YAML-only customModes anchor. Substring matchers also
// fire on ExportedModeConfig — both landmarks must be stripped to fully
// remove the ModeConfig substring evidence.
func TestRecognizeRooCode_AgentsAnchorsMissing(t *testing.T) {
	mutated := []string{}
	for _, lm := range realRooCodeAgentsLandmarks {
		if lm == "ModeConfig" || lm == "ExportedModeConfig" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("roo-code", capmon.RecognitionContext{
		Provider:  "roo-code",
		Format:    "typescript",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["agents.supported"]; has {
		t.Error("agents.supported should NOT be present when 'ModeConfig' anchor is missing")
	}
	for _, absent := range []string{
		"agents.capabilities.definition_format.supported",
		"agents.capabilities.tool_restrictions.supported",
		"agents.capabilities.agent_scopes.project.supported",
		"agents.capabilities.agent_scopes.user.supported",
	} {
		if _, has := result.Capabilities[absent]; has {
			t.Errorf("%s should NOT be present without required anchor", absent)
		}
	}
}
