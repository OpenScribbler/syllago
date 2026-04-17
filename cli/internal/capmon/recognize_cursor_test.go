package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realCursorRulesLandmarks is a snapshot of headings extracted from
// .capmon-cache/cursor/rules.0/extracted.json (cursor.com/docs/rules) as of
// 2026-04-16. Update when the doc evolves.
var realCursorRulesLandmarks = []string{
	// Top-level nav
	"Command Palette",
	"Get Started",
	"Agent",
	"Customizing",
	"Cloud Agents",
	"Integrations",
	"CLI",
	"Teams & Enterprise",
	// Rules page
	"Rules",
	"How rules work",
	"Project rules",
	"Rule file structure",
	"Rule anatomy",
	"Creating a rule",
	"Best practices",
	"What to avoid in rules",
	"Rule file format",
	"Examples",
	"Standards for frontend components and API validation",
	"Templates for Express services and React components",
	"Automating development workflows and documentation generation",
	"Adding a new setting in Cursor",
	"Team Rules",
	"Managing Team Rules",
	"Activation and enforcement",
	"Format and how Team Rules are applied",
	"Importing Rules",
	"Remote rules (via GitHub)",
	"AGENTS.md",
	"Improvements",
	"Nested AGENTS.md support",
	"User Rules",
	"FAQ",
	"Why isn't my rule being applied?",
	"Can rules reference other rules or files?",
	"Can I create a rule from chat?",
	"Do rules impact Cursor Tab or other AI features?",
	"Do User Rules apply to Inline Edit (Cmd/Ctrl+K)?",
}

func TestRecognizeCursor_RealRulesLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("cursor", capmon.RecognitionContext{
		Provider:  "cursor",
		Format:    "html",
		Landmarks: realCursorRulesLandmarks,
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
		"activation_mode.manual",
		"activation_mode.model_decision",
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
	// auto_memory must NOT be emitted — cursor docs do not document an
	// agent-managed memory feature.
	if _, has := caps["rules.capabilities.auto_memory.supported"]; has {
		t.Error("rules.capabilities.auto_memory should NOT be present for cursor")
	}
	// skills.* must be empty — cursor does not implement Agent Skills.
	for k := range caps {
		if len(k) >= 7 && k[:7] == "skills." {
			t.Errorf("unexpected skills.* capability for cursor: %q", k)
		}
	}
}

func TestRecognizeCursor_AnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realCursorRulesLandmarks))
	for _, lm := range realCursorRulesLandmarks {
		if lm == "Rule anatomy" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("cursor", capmon.RecognitionContext{
		Provider:  "cursor",
		Format:    "html",
		Landmarks: mutated,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
}

func TestRecognizeCursor_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("cursor", capmon.RecognitionContext{
		Provider: "cursor",
		Format:   "html",
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
}
