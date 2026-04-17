package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realPiHooksLandmarks is a snapshot of the H2/H3 headings extracted from
// pi's extensions doc (.capmon-cache/pi/hooks.1/extracted.json) as of
// 2026-04-16. Pi brands its hook system as "Extensions". Update when the
// doc evolves.
var realPiHooksLandmarks = []string{
	"Extensions",
	"Table of Contents",
	"Quick Start",
	"Extension Locations",
	"Available Imports",
	"Writing an Extension",
	"Extension Styles",
	"Events",
	"Lifecycle Overview",
	"Resource Events",
	"resources_discover",
	"Session Events",
	"session_start",
	"session_before_switch",
	"session_before_fork",
	"session_before_compact / session_compact",
	"session_before_tree / session_tree",
	"session_shutdown",
	"Agent Events",
	"before_agent_start",
	"agent_start / agent_end",
	"turn_start / turn_end",
	"message_start / message_update / message_end",
	"tool_execution_start / tool_execution_update / tool_execution_end",
	"context",
	"before_provider_request",
	"after_provider_response",
	"Model Events",
	"model_select",
	"Tool Events",
	"tool_call",
	"tool_result",
	"User Bash Events",
	"user_bash",
	"Input Events",
	"input",
	"ExtensionContext",
	"ExtensionAPI Methods",
	"pi.on(event, handler)",
	"pi.registerTool(definition)",
	"Custom Tools",
	"Tool Definition",
	"Best Practices",
	"Examples Reference",
}

// TestRecognizePi_RealHooksLandmarks proves hooks recognition emits the 2
// canonical hooks keys curated as supported in the format YAML:
// handler_types (TypeScript-native handlers via 'Writing an Extension') and
// input_modification (tool_call event interception). The other 7 canonical
// keys are curated as unsupported and must NOT be emitted.
func TestRecognizePi_RealHooksLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("pi", capmon.RecognitionContext{
		Provider:  "pi",
		Format:    "markdown",
		Landmarks: realPiHooksLandmarks,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["hooks.supported"] != "true" {
		t.Error("hooks.supported missing")
	}
	hooksInferred := []string{
		"handler_types",
		"input_modification",
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
		"hooks.capabilities.matcher_patterns.supported",
		"hooks.capabilities.decision_control.supported",
		"hooks.capabilities.async_execution.supported",
		"hooks.capabilities.hook_scopes.supported",
		"hooks.capabilities.json_io_protocol.supported",
		"hooks.capabilities.context_injection.supported",
		"hooks.capabilities.permission_control.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present for pi (curated as unsupported)", absent)
		}
	}
}

// TestRecognizePi_HooksAnchorsMissing proves the required-anchor guard
// suppresses when one of the unique anchors is absent.
func TestRecognizePi_HooksAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realPiHooksLandmarks))
	for _, lm := range realPiHooksLandmarks {
		if lm == "ExtensionAPI Methods" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("pi", capmon.RecognitionContext{
		Provider:  "pi",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if _, has := result.Capabilities["hooks.supported"]; has {
		t.Error("hooks.supported should NOT be present when required anchor is missing")
	}
}

func TestRecognizePi_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("pi", capmon.RecognitionContext{
		Provider: "pi",
		Format:   "markdown",
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
}
