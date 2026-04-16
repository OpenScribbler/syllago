package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realClineLandmarks is a snapshot of the headings extracted from cline's skills
// doc (.capmon-cache/cline/skills.0/extracted.json) as of 2026-04-16.
var realClineLandmarks = []string{
	"Documentation Index",
	"Skills",
	"How Skills Work",
	"Skill Structure",
	"Creating a Skill",
	"Toggling Skills",
	"Writing Your SKILL.md",
	"Naming Conventions",
	"Writing Effective Descriptions",
	"Keeping Skills Focused",
	"Where Skills Live",
	"Bundling Supporting Files",
	"docs/",
	"templates/",
	"scripts/",
	"Referencing Bundled Files",
	"Example: Data Analysis Skill",
}

// realClineNonSkillsLandmarks is a sample drawn from cline's other content-type
// docs (rules, hooks, mcp, commands). The required anchors must NOT match any
// of these — proves the false-positive guardrail works under multi-source merge.
var realClineNonSkillsLandmarks = []string{
	"Documentation Index",
	"Rules", "Supported Rule Types", "Where Rules Live", "Global Rules Directory",
	"Hooks", "What You Can Build", "Hook Types", "Hook Lifecycle",
	"Adding & Configuring Servers", "Finding MCP Servers", "Managing Servers",
	"Using Commands", "Slash Commands",
}

func TestRecognizeCline_RealLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("cline", capmon.RecognitionContext{
		Provider:  "cline",
		Format:    "markdown",
		Landmarks: realClineLandmarks,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q", result.Status, capmon.StatusRecognized)
	}
	caps := result.Capabilities
	if caps["skills.supported"] != "true" {
		t.Error("skills.supported missing")
	}
	inferred := []string{
		"directory_structure",
		"creation_workflow",
		"toggling",
		"frontmatter",
		"naming_conventions",
		"description_guidance",
		"bundled_files",
		"file_references",
	}
	for _, c := range inferred {
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

// TestRecognizeCline_NonSkillsLandmarks proves the false-positive guardrail:
// when cline's other content-type doc landmarks are present (rules, hooks, mcp,
// commands) but the skills-specific anchors are NOT, the recognizer suppresses.
// This is the realistic multi-source case — every cline run merges all sources.
func TestRecognizeCline_NonSkillsLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("cline", capmon.RecognitionContext{
		Provider:  "cline",
		Format:    "markdown",
		Landmarks: realClineNonSkillsLandmarks,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities from non-skills landmarks, got %d: %v",
			len(result.Capabilities), result.Capabilities)
	}
}

func TestRecognizeCline_AnchorsMissing(t *testing.T) {
	mutated := []string{}
	for _, lm := range realClineLandmarks {
		if lm == "Where Skills Live" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("cline", capmon.RecognitionContext{
		Provider:  "cline",
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

func TestRecognizeCline_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("cline", capmon.RecognitionContext{Provider: "cline", Format: "markdown"})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities, got %d", len(result.Capabilities))
	}
}
