package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realCopilotCliSkillsLandmarks merges the headings extracted from both Copilot
// CLI skills sources (.capmon-cache/copilot-cli/skills.{0,1}/extracted.json) as
// of 2026-04-16. The doc surface is thin — Copilot CLI documents skills
// existence and usage but not granular format-level details at heading level.
var realCopilotCliSkillsLandmarks = []string{
	"About agent skills",
	"Next steps",
	"Using agent skills",
	"Skills commands in the CLI",
}

// realCopilotCliNonSkillsLandmarks is a sample drawn from Copilot CLI's other
// content-type docs. Required anchors must NOT match any of these.
var realCopilotCliNonSkillsLandmarks = []string{
	"Documentation Index",
	"Hook types", "Session start hook", "Pre-tool use hook",
	"Adding an MCP server", "Managing MCP servers",
	"Plugin structure", "Creating a plugin",
	"YAML frontmatter properties", "MCP server configuration details",
}

// realCopilotCliRulesLandmarks is a snapshot of the headings from Copilot
// CLI's custom-instructions rules.0/extracted.json (add-custom-instructions.md)
// as of 2026-04-16. Update when the doc evolves.
var realCopilotCliRulesLandmarks = []string{
	"Types of custom instructions",
	"Repository-wide custom instructions",
	"Path-specific custom instructions",
	"Agent instructions",
	"Local instructions",
	"Creating repository-wide custom instructions",
	"Creating path-specific custom instructions",
	"Further reading",
}

func TestRecognizeCopilotCli_RealLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("copilot-cli", capmon.RecognitionContext{
		Provider:  "copilot-cli",
		Format:    "markdown",
		Landmarks: realCopilotCliSkillsLandmarks,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q", result.Status, capmon.StatusRecognized)
	}
	caps := result.Capabilities
	if caps["skills.supported"] != "true" {
		t.Error("skills.supported missing")
	}
	// Single inferred capability from the CLI-management anchor
	if caps["skills.capabilities.cli_management.supported"] != "true" {
		t.Error("cli_management.supported missing")
	}
	if caps["skills.capabilities.cli_management.confidence"] != "inferred" {
		t.Errorf("cli_management.confidence = %q, want inferred",
			caps["skills.capabilities.cli_management.confidence"])
	}
	for _, c := range []string{"project_scope", "global_scope", "canonical_filename"} {
		if caps["skills.capabilities."+c+".confidence"] != "confirmed" {
			t.Errorf("%s.confidence = %q, want confirmed", c, caps["skills.capabilities."+c+".confidence"])
		}
	}
}

// TestRecognizeCopilotCli_NonSkillsLandmarks proves the multi-source false-
// positive guardrail: Copilot CLI's hooks/mcp/rules/commands/agents landmarks
// alone must NOT trigger skills recognition.
func TestRecognizeCopilotCli_NonSkillsLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("copilot-cli", capmon.RecognitionContext{
		Provider:  "copilot-cli",
		Format:    "markdown",
		Landmarks: realCopilotCliNonSkillsLandmarks,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities from non-skills landmarks, got %d: %v",
			len(result.Capabilities), result.Capabilities)
	}
}

// TestRecognizeCopilotCli_SupportWithoutSpecificCapability verifies the bare
// anchor-only pattern: when only the required anchors are present and no
// capability-specific matcher fires, skills.supported is still emitted.
func TestRecognizeCopilotCli_SupportWithoutSpecificCapability(t *testing.T) {
	result := capmon.RecognizeWithContext("copilot-cli", capmon.RecognitionContext{
		Provider:  "copilot-cli",
		Format:    "markdown",
		Landmarks: []string{"About agent skills", "Using agent skills"},
	})
	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q", result.Status, capmon.StatusRecognized)
	}
	if result.Capabilities["skills.supported"] != "true" {
		t.Error("skills.supported should be true even without specific-capability anchor")
	}
	if _, has := result.Capabilities["skills.capabilities.cli_management.supported"]; has {
		t.Error("cli_management should NOT be present when its anchor is missing")
	}
}

func TestRecognizeCopilotCli_AnchorsMissing(t *testing.T) {
	// Strip "Using agent skills" — one of the required anchors.
	result := capmon.RecognizeWithContext("copilot-cli", capmon.RecognitionContext{
		Provider:  "copilot-cli",
		Format:    "markdown",
		Landmarks: []string{"About agent skills", "Next steps"},
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities, got %d", len(result.Capabilities))
	}
}

func TestRecognizeCopilotCli_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("copilot-cli", capmon.RecognitionContext{Provider: "copilot-cli", Format: "markdown"})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities, got %d", len(result.Capabilities))
	}
}

// TestRecognizeCopilotCli_RealRulesLandmarks proves rules recognition on the
// merged skills+rules landmarks. Copilot CLI has the most comprehensive
// cross-provider compatibility surface in the cache (AGENTS.md + CLAUDE.md +
// GEMINI.md), all gated on the "Agent instructions" landmark.
func TestRecognizeCopilotCli_RealRulesLandmarks(t *testing.T) {
	merged := append([]string{}, realCopilotCliSkillsLandmarks...)
	merged = append(merged, realCopilotCliRulesLandmarks...)
	result := capmon.RecognizeWithContext("copilot-cli", capmon.RecognitionContext{
		Provider:  "copilot-cli",
		Format:    "markdown",
		Landmarks: merged,
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
		"cross_provider_recognition.agents_md",
		"cross_provider_recognition.claude_md",
		"cross_provider_recognition.gemini_md",
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
	// auto_memory must NOT be emitted — copilot-cli docs do not document an
	// agent-managed memory feature.
	if _, has := caps["rules.capabilities.auto_memory.supported"]; has {
		t.Error("rules.capabilities.auto_memory should NOT be present for copilot-cli")
	}
	// file_imports must NOT be emitted — no @-import syntax documented.
	if _, has := caps["rules.capabilities.file_imports.supported"]; has {
		t.Error("rules.capabilities.file_imports should NOT be present for copilot-cli")
	}
}
