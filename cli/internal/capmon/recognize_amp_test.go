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
