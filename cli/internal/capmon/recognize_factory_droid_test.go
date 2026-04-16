package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// syntheticFactoryDroidLandmarks represents what we EXPECT Factory Droid's
// skills doc to surface once the upstream extractor handles its JS-rendered
// Mintlify pages. The current production landmarks are null (see spike note in
// recognize_factory_droid.go), so this test verifies the recognizer is correct
// for the future-state input.
var syntheticFactoryDroidLandmarks = []string{
	"Skills",
	"Frontmatter",
	"Creating a Skill",
	"Skill Locations",
	"Invoking Skills",
}

func TestRecognizeFactoryDroid_SyntheticLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: syntheticFactoryDroidLandmarks,
	})
	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q", result.Status, capmon.StatusRecognized)
	}
	caps := result.Capabilities
	if caps["skills.supported"] != "true" {
		t.Error("skills.supported missing")
	}
	for _, c := range []string{"frontmatter", "creation_workflow", "directory_structure", "invocation"} {
		if caps["skills.capabilities."+c+".supported"] != "true" {
			t.Errorf("%s.supported missing", c)
		}
		if caps["skills.capabilities."+c+".confidence"] != "inferred" {
			t.Errorf("%s.confidence = %q, want inferred", c, caps["skills.capabilities."+c+".confidence"])
		}
	}
	for _, c := range []string{"project_scope", "global_scope", "canonical_filename"} {
		if caps["skills.capabilities."+c+".confidence"] != "confirmed" {
			t.Errorf("%s.confidence = %q, want confirmed", c, caps["skills.capabilities."+c+".confidence"])
		}
	}
}

// TestRecognizeFactoryDroid_PageTitleOnly verifies the bare-anchor pattern:
// when only the page title "Skills" is present (the realistic state once HTML
// extraction sees just the H1), skills.supported is still emitted.
func TestRecognizeFactoryDroid_PageTitleOnly(t *testing.T) {
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: []string{"Skills"},
	})
	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q", result.Status, capmon.StatusRecognized)
	}
	if result.Capabilities["skills.supported"] != "true" {
		t.Error("skills.supported should fire from page title alone")
	}
	for _, c := range []string{"frontmatter", "creation_workflow", "directory_structure", "invocation"} {
		if _, has := result.Capabilities["skills.capabilities."+c+".supported"]; has {
			t.Errorf("%s should NOT be present when its anchor is missing", c)
		}
	}
}

// TestRecognizeFactoryDroid_LiveCacheCurrentState documents the current
// production state: factory-droid extraction returns null landmarks, so the
// recognizer suppresses cleanly. When upstream extraction is fixed, this test
// will start failing and should be updated to assert the new live behavior.
func TestRecognizeFactoryDroid_LiveCacheCurrentState(t *testing.T) {
	// Simulates the merged context from .capmon-cache/factory-droid as of
	// 2026-04-16: skills.0/agents.0/commands.0/hooks.X/rules.0 all return null
	// landmarks; only mcp.0 contributes.
	mergedFromLiveCache := []string{
		"Factory Documentation", "Docs", "OpenAPI Specs", "Optional",
	}
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: mergedFromLiveCache,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q (extraction issue still present)", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities pre-extraction-fix, got %d: %v",
			len(result.Capabilities), result.Capabilities)
	}
}

func TestRecognizeFactoryDroid_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{Provider: "factory-droid", Format: "markdown"})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
}
