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
// docs (mcp, commands). The required anchors must NOT match any of these
// — proves the false-positive guardrail works under multi-source merge.
// Rules and hooks anchors are excluded here because they are now legitimately
// recognized by their content types; see realClineRulesLandmarks /
// realClineHooksLandmarks for those cases. We do include partial hooks
// landmarks (Hook Types, Hook Lifecycle) to verify the hooks required-anchor
// guard suppresses cleanly when "Hook Locations" is absent.
var realClineNonSkillsLandmarks = []string{
	"Documentation Index",
	"Hooks", "What You Can Build", "Hook Types", "Hook Lifecycle",
	"Adding & Configuring Servers", "Finding MCP Servers", "Managing Servers",
	"Using Commands", "Slash Commands",
}

// realClineHooksLandmarks is a snapshot of the hooks-doc headings from
// .capmon-cache/cline/hooks.0/extracted.json (cline customization/hooks.md)
// as of 2026-04-16.
var realClineHooksLandmarks = []string{
	"Documentation Index",
	"Hooks",
	"What You Can Build",
	"Hook Types",
	"Hook Lifecycle",
	"Hook Locations",
	"Creating a Hook",
	"How Hooks Work",
	"Input Structure",
	"Output Structure",
	"Context Modification",
	"Hook Reference",
	"Task Lifecycle Hooks",
	"Tool Hooks",
}

// realClineRulesLandmarks is a snapshot of the rules-doc headings from
// .capmon-cache/cline/rules.0/extracted.json (cline-rules.md) as of 2026-04-16.
var realClineRulesLandmarks = []string{
	"Documentation Index",
	"Rules",
	"Supported Rule Types",
	"Where Rules Live",
	"Global Rules Directory",
	"Creating Rules",
	"Toggling Rules",
	"Writing Effective Rules",
	"Conditional Rules",
	"How It Works",
	"Writing Conditional Rules",
	"The paths Conditional",
	"Behavior Details",
	"Practical Examples",
	"Frontend vs Backend Rules",
	"Test File Rules",
	"Documentation Rules",
	"Combining with Rule Toggles",
	"Tips for Effective Conditional Rules",
	"Troubleshooting Conditional Rules",
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

// TestRecognizeCline_RealRulesLandmarks proves rules recognition on the merged
// skills+rules landmarks. Per the seeder spec, cline supports a smaller
// activation_mode vocabulary (only always_on + frontmatter_globs) than
// cursor/kiro/windsurf. file_imports, cross_provider_recognition, and
// auto_memory are intentionally absent.
func TestRecognizeCline_RealRulesLandmarks(t *testing.T) {
	merged := append([]string{}, realClineLandmarks...)
	merged = append(merged, realClineRulesLandmarks...)
	result := capmon.RecognizeWithContext("cline", capmon.RecognitionContext{
		Provider:  "cline",
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
	for _, absent := range []string{
		"rules.capabilities.file_imports.supported",
		"rules.capabilities.cross_provider_recognition.agents_md.supported",
		"rules.capabilities.auto_memory.supported",
		"rules.capabilities.activation_mode.manual.supported",
		"rules.capabilities.activation_mode.model_decision.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present for cline", absent)
		}
	}
}

// realClineMcpLandmarks is a snapshot of the MCP-doc headings combined from
// .capmon-cache/cline/mcp.0/extracted.json (configuring-mcp-servers.md) and
// .capmon-cache/cline/mcp.1/extracted.json (transport-mechanisms.md) as of
// 2026-04-16. mcp.0 supplies the required anchor "Adding & Configuring
// Servers" and the marketplace anchor "Finding MCP Servers"; mcp.1 supplies
// the second required anchor "MCP Transport Mechanisms" and the
// transport_types evidence.
var realClineMcpLandmarks = []string{
	// mcp.0 — configuring-mcp-servers.md
	"Documentation Index",
	"Adding & Configuring Servers",
	"Finding MCP Servers",
	"Adding Servers with Cline",
	"Managing Servers",
	"Enable/Disable",
	"Restart",
	"Delete",
	"Network Timeout",
	"Editing Configuration Files",
	"STDIO Transport (Local Servers)",
	"SSE Transport (Remote Servers)",
	"Global MCP Mode",
	"Using MCP Tools",
	"Troubleshooting",
	"Related",
	// mcp.1 — transport-mechanisms.md
	"MCP Transport Mechanisms",
	"STDIO Transport",
	"How STDIO Transport Works",
	"STDIO Characteristics",
	"When to Use STDIO",
	"STDIO Implementation Example",
	"SSE Transport",
	"How SSE Transport Works",
	"SSE Characteristics",
	"When to Use SSE",
	"SSE Implementation Example",
	"Local vs. Hosted: Deployment Aspects",
	"STDIO: Local Deployment Model",
	"SSE: Hosted Deployment Model",
	"Hybrid Approaches",
	"Choosing Between STDIO and SSE",
	"Configuring Transports in Cline",
}

// TestRecognizeCline_RealMcpLandmarks proves MCP recognition emits the 2
// canonical MCP keys backed by heading-level evidence: transport_types and
// marketplace. The other 6 canonical keys are either curated as unsupported
// or backed only by body-text evidence (alwaysAllow JSON field for
// tool_filtering / auto_approve) and must NOT be emitted by the recognizer.
// Test merges all four content type fixtures to verify cross-content-type
// robustness.
func TestRecognizeCline_RealMcpLandmarks(t *testing.T) {
	merged := append([]string{}, realClineLandmarks...)
	merged = append(merged, realClineRulesLandmarks...)
	merged = append(merged, realClineHooksLandmarks...)
	merged = append(merged, realClineMcpLandmarks...)
	result := capmon.RecognizeWithContext("cline", capmon.RecognitionContext{
		Provider:  "cline",
		Format:    "markdown",
		Landmarks: merged,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["mcp.supported"] != "true" {
		t.Error("mcp.supported missing")
	}
	mcpInferred := []string{
		"transport_types",
		"marketplace",
	}
	for _, c := range mcpInferred {
		key := "mcp.capabilities." + c + ".supported"
		if caps[key] != "true" {
			t.Errorf("%s missing", key)
		}
		if got := caps["mcp.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("mcp.%s.confidence = %q, want inferred", c, got)
		}
	}
	for _, absent := range []string{
		"mcp.capabilities.oauth_support.supported",
		"mcp.capabilities.env_var_expansion.supported",
		"mcp.capabilities.tool_filtering.supported",
		"mcp.capabilities.auto_approve.supported",
		"mcp.capabilities.resource_referencing.supported",
		"mcp.capabilities.enterprise_management.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present for cline (no heading evidence or curated unsupported)", absent)
		}
	}
}

// TestRecognizeCline_McpAnchorsMissing proves the required-anchor guard
// suppresses MCP emission when "MCP Transport Mechanisms" is absent.
func TestRecognizeCline_McpAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realClineMcpLandmarks))
	for _, lm := range realClineMcpLandmarks {
		if lm == "MCP Transport Mechanisms" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("cline", capmon.RecognitionContext{
		Provider:  "cline",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["mcp.supported"]; has {
		t.Error("mcp.supported should NOT be present when 'MCP Transport Mechanisms' anchor is missing")
	}
}

// TestRecognizeCline_RealHooksLandmarks proves hooks recognition on the merged
// skills+rules+hooks landmarks. Cline documents 4 of the 9 canonical hooks
// keys at the heading level (handler_types, hook_scopes, json_io_protocol,
// context_injection); the rest live in body text or are not documented.
func TestRecognizeCline_RealHooksLandmarks(t *testing.T) {
	merged := append([]string{}, realClineLandmarks...)
	merged = append(merged, realClineRulesLandmarks...)
	merged = append(merged, realClineHooksLandmarks...)
	result := capmon.RecognizeWithContext("cline", capmon.RecognitionContext{
		Provider:  "cline",
		Format:    "markdown",
		Landmarks: merged,
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
		"hook_scopes",
		"json_io_protocol",
		"context_injection",
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
		"hooks.capabilities.permission_control.supported",
		"hooks.capabilities.input_modification.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present for cline (no heading evidence)", absent)
		}
	}
}
