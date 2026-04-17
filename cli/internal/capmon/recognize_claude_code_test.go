package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realClaudeCodeLandmarks is a snapshot of the H2/H3 headings extracted from
// docs.claude.com/en/docs/claude-code/skills as of 2026-04-16. Source of truth:
// .capmon-cache/claude-code/skills.0/extracted.json. Update when the doc evolves.
var realClaudeCodeLandmarks = []string{
	"Documentation Index",
	"Extend Claude with skills",
	"Bundled skills",
	"Getting started",
	"Create your first skill",
	"Where skills live",
	"Live change detection",
	"Automatic discovery from nested directories",
	"Skills from additional directories",
	"Configure skills",
	"Types of skill content",
	"Frontmatter reference",
	"Available string substitutions",
	"Add supporting files",
	"Control who invokes a skill",
	"Skill content lifecycle",
	"Pre-approve tools for a skill",
	"Pass arguments to skills",
	"Advanced patterns",
	"Inject dynamic context",
	"Run skills in a subagent",
	"Example: Research skill using Explore agent",
	"Restrict Claude's skill access",
	"Share skills",
	"Generate visual output",
	"Troubleshooting",
	"Skill not triggering",
	"Skill triggers too often",
	"Skill descriptions are cut short",
	"Related resources",
}

// realClaudeCodeRulesLandmarks is a snapshot of the H2/H3 headings extracted
// from docs.claude.com/en/docs/claude-code/memory as of 2026-04-16. Source of
// truth: .capmon-cache/claude-code/rules.0/extracted.json. Update when the
// doc evolves.
var realClaudeCodeRulesLandmarks = []string{
	"Documentation Index",
	"How Claude remembers your project",
	"CLAUDE.md vs auto memory",
	"CLAUDE.md files",
	"When to add to CLAUDE.md",
	"Choose where to put CLAUDE.md files",
	"Set up a project CLAUDE.md",
	"Write effective instructions",
	"Import additional files",
	"AGENTS.md",
	"How CLAUDE.md files load",
	"Load from additional directories",
	"Organize rules with .claude/rules/",
	"Set up rules",
	"Path-specific rules",
	"Share rules across projects with symlinks",
	"User-level rules",
	"Manage CLAUDE.md for large teams",
	"Deploy organization-wide CLAUDE.md",
	"Exclude specific CLAUDE.md files",
	"Auto memory",
	"Enable or disable auto memory",
	"Storage location",
	"How it works",
	"Audit and edit your memory",
	"View and edit with /memory",
	"Troubleshoot memory issues",
}

// TestRecognizeClaudeCode_RealLandmarks proves the canary path: feeding the
// recognizer the merged real landmarks from the live skills + rules docs
// produces a non-empty result with both content types' expected capability
// sets at confidence "inferred" (and the static facts at "confirmed").
func TestRecognizeClaudeCode_RealLandmarks(t *testing.T) {
	merged := append([]string{}, realClaudeCodeLandmarks...)
	merged = append(merged, realClaudeCodeRulesLandmarks...)
	result := capmon.RecognizeWithContext("claude-code", capmon.RecognitionContext{
		Provider:  "claude-code",
		Format:    "markdown",
		Landmarks: merged,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q", result.Status, capmon.StatusRecognized)
	}
	if len(result.MissingAnchors) != 0 {
		t.Errorf("expected no missing anchors, got %v", result.MissingAnchors)
	}

	caps := result.Capabilities
	if caps["skills.supported"] != "true" {
		t.Error("skills.supported missing")
	}

	// Inferred capabilities (from landmark patterns)
	inferred := []string{
		"frontmatter",
		"live_reload",
		"nested_directories",
		"additional_directories",
		"arguments",
		"tool_preapproval",
		"subagent_invocation",
		"dynamic_context",
		"invoker_control",
	}
	for _, c := range inferred {
		key := "skills.capabilities." + c + ".supported"
		if caps[key] != "true" {
			t.Errorf("%s missing", key)
		}
		if got := caps["skills.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("%s.confidence = %q, want inferred", c, got)
		}
	}

	// Confirmed capabilities (static facts merged after landmark match)
	confirmed := []string{"project_scope", "global_scope", "canonical_filename"}
	for _, c := range confirmed {
		if caps["skills.capabilities."+c+".supported"] != "true" {
			t.Errorf("%s.supported missing", c)
		}
		if got := caps["skills.capabilities."+c+".confidence"]; got != "confirmed" {
			t.Errorf("%s.confidence = %q, want confirmed", c, got)
		}
	}

	// Rules content type must also recognize on the merged landmarks
	if caps["rules.supported"] != "true" {
		t.Error("rules.supported missing")
	}
	rulesCaps := []string{
		"activation_mode.always_on",
		"activation_mode.glob",
		"file_imports",
		"cross_provider_recognition.agents_md",
		"auto_memory",
		"hierarchical_loading",
	}
	for _, c := range rulesCaps {
		key := "rules.capabilities." + c + ".supported"
		if caps[key] != "true" {
			t.Errorf("%s missing", key)
		}
		if got := caps["rules.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("rules.%s.confidence = %q, want inferred", c, got)
		}
	}
}

// TestRecognizeClaudeCode_AnchorsMissing proves the negative path: stripping
// one of the required anchors from the input causes the entire recognition to
// suppress (status=anchors_missing, no capabilities, anchor name surfaced).
// This is the false-positive guardrail — a docs index that lists "Skills" as
// a link must NOT trigger the recognizer.
func TestRecognizeClaudeCode_AnchorsMissing(t *testing.T) {
	// Strip "Where skills live" — the location anchor that proves we're on
	// the format-describing doc, not a passing reference.
	mutated := make([]string, 0, len(realClaudeCodeLandmarks))
	for _, lm := range realClaudeCodeLandmarks {
		if lm == "Where skills live" {
			continue
		}
		mutated = append(mutated, lm)
	}

	result := capmon.RecognizeWithContext("claude-code", capmon.RecognitionContext{
		Provider:  "claude-code",
		Format:    "markdown",
		Landmarks: mutated,
	})

	if result.Status != capmon.StatusAnchorsMissing {
		t.Fatalf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities when anchor missing, got %d: %v", len(result.Capabilities), result.Capabilities)
	}
	// MissingAnchors should mention the stripped anchor
	found := false
	for _, m := range result.MissingAnchors {
		if m == "Where skills live" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("MissingAnchors %v does not include 'Where skills live'", result.MissingAnchors)
	}
}

// TestRecognizeClaudeCode_NoLandmarks proves an entirely empty landmark list
// produces "anchors_missing" status (since required anchors fail) with no
// capabilities. This is the universal "fed nothing" case.
func TestRecognizeClaudeCode_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("claude-code", capmon.RecognitionContext{
		Provider:  "claude-code",
		Format:    "markdown",
		Landmarks: nil,
	})

	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities, got %d", len(result.Capabilities))
	}
}
