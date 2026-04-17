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
