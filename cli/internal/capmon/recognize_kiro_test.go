package capmon_test

import (
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realKiroSkillsLandmarks is a snapshot of the headings extracted from kiro's
// powers doc (.capmon-cache/kiro/skills.0/extracted.json) as of 2026-04-16.
// Includes the AWS docs cookie-banner boilerplate that prefixes every kiro
// page — these landmarks are noise but the recognizer must ignore them.
var realKiroSkillsLandmarks = []string{
	"Select your cookie preferences",
	"Customize cookie preferences",
	"Essential", "Performance", "Functional", "Advertising",
	"Your privacy choices",
	"Unable to save cookie preferences",
	"Create powers",
	"What you need",
	"Creating POWER.md",
	"Frontmatter: When to activate",
	"Onboarding instructions",
	"Steering instructions",
	"Adding MCP servers",
	"Directory structure",
	"Testing locally",
	"Sharing your power",
	"Examples",
}

// realKiroNonSkillsLandmarks is a sample drawn from kiro's non-skills,
// non-rules content-type docs (agents, hooks, mcp). Required anchors for the
// SKILLS recognizer must NOT match any of these. Note: rules/steering anchors
// are deliberately excluded — those drive rules recognition.
var realKiroNonSkillsLandmarks = []string{
	"Select your cookie preferences", "Essential", "Performance",
	"Agent configuration reference", "Name field", "Description field",
	"Hooks", "What are agent hooks?", "How agent hooks work",
	"Configuration", "Configuration file structure", "Remote server", "Local server",
}

// realKiroRulesLandmarks is a snapshot of kiro's steering doc landmarks
// (.capmon-cache/kiro/rules.0/extracted.json). Includes the cookie-banner
// boilerplate for false-positive checking, plus the substantive Steering
// section + Inclusion modes + Agents.md fallback.
var realKiroRulesLandmarks = []string{
	"Select your cookie preferences",
	"Customize cookie preferences",
	"Essential", "Performance", "Functional", "Advertising",
	"Your privacy choices",
	"Unable to save cookie preferences",
	"Steering",
	"What is steering?",
	"Key benefits",
	"Steering file scope",
	"Workspace steering",
	"Global steering",
	"Team steering",
	"Foundational steering files",
	"Creating custom steering files",
	"Agents.md",
	"Inclusion modes",
	"Always included (default)",
	"Conditional inclusion",
	"Manual inclusion",
	"Auto inclusion",
	"File references",
	"Best practices",
	"Common steering file strategies",
	"Related documentation",
}

func TestRecognizeKiro_RealLandmarks(t *testing.T) {
	merged := append([]string{}, realKiroSkillsLandmarks...)
	merged = append(merged, realKiroRulesLandmarks...)
	result := capmon.RecognizeWithContext("kiro", capmon.RecognitionContext{
		Provider:  "kiro",
		Format:    "markdown",
		Landmarks: merged,
	})
	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q", result.Status, capmon.StatusRecognized)
	}
	caps := result.Capabilities
	if caps["skills.supported"] != "true" {
		t.Error("skills.supported missing")
	}
	inferred := []string{
		"frontmatter", "onboarding_instructions", "steering_instructions",
		"mcp_integration", "directory_structure", "testing", "sharing",
	}
	for _, c := range inferred {
		if caps["skills.capabilities."+c+".supported"] != "true" {
			t.Errorf("%s.supported missing", c)
		}
		if caps["skills.capabilities."+c+".confidence"] != "inferred" {
			t.Errorf("%s.confidence = %q, want inferred", c, caps["skills.capabilities."+c+".confidence"])
		}
	}
	// Kiro has project_scope and canonical_filename, but NOT global_scope
	// (powers install via UI panel, no fixed filesystem path).
	for _, c := range []string{"project_scope", "canonical_filename"} {
		if caps["skills.capabilities."+c+".confidence"] != "confirmed" {
			t.Errorf("%s.confidence = %q, want confirmed", c, caps["skills.capabilities."+c+".confidence"])
		}
	}
	if _, has := caps["skills.capabilities.global_scope.supported"]; has {
		t.Error("global_scope should NOT be present for kiro (no global filesystem path)")
	}

	// Rules content type
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
	}
	// auto_memory must NOT be emitted — kiro has no auto-memory feature
	if _, has := caps["rules.capabilities.auto_memory.supported"]; has {
		t.Error("rules.capabilities.auto_memory should NOT be present for kiro")
	}

	// Sanity: the cookie banner landmarks should not have produced any
	// capabilities — confirms no accidental substring match.
	for k := range caps {
		for _, bad := range []string{"cookie", "essential", "performance", "advertising"} {
			if strings.Contains(k, bad) {
				t.Errorf("capability %q appears derived from cookie banner noise", k)
			}
		}
	}
}

// TestRecognizeKiro_NonSkillsLandmarks proves the multi-source false-positive
// guardrail: kiro's agents/hooks/mcp/rules landmarks (with shared cookie
// banner) must NOT trigger skills recognition.
func TestRecognizeKiro_NonSkillsLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("kiro", capmon.RecognitionContext{
		Provider:  "kiro",
		Format:    "markdown",
		Landmarks: realKiroNonSkillsLandmarks,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities, got %d: %v",
			len(result.Capabilities), result.Capabilities)
	}
}

func TestRecognizeKiro_AnchorsMissing(t *testing.T) {
	// Strip "Creating POWER.md" — required anchor.
	mutated := []string{}
	for _, lm := range realKiroSkillsLandmarks {
		if lm == "Creating POWER.md" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("kiro", capmon.RecognitionContext{
		Provider:  "kiro",
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

func TestRecognizeKiro_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("kiro", capmon.RecognitionContext{Provider: "kiro", Format: "markdown"})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
}
