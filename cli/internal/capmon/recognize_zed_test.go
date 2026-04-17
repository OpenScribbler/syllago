package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realZedRulesLandmarks is a snapshot of the rules-relevant headings + filename
// landmarks extracted from zed's HTML rules doc
// (.capmon-cache/zed/rules.1/extracted.json) as of 2026-04-16. The HTML
// extractor surfaces both H2/H3 headings and the inline <code> filename
// listings under the ".rules files" section.
//
// rules.0 (zed's own .rules instance file with Rust coding guidelines) is
// intentionally not included — instance content is not capability vocabulary.
var realZedRulesLandmarks = []string{
	"Rules",
	".rules files",
	".rules",
	".cursorrules",
	".windsurfrules",
	".clinerules",
	".github/copilot-instructions.md",
	"AGENT.md",
	"AGENTS.md",
	"CLAUDE.md",
	"GEMINI.md",
	"Rules Library",
	"Opening the Rules Library",
	"Managing Rules",
	"Creating Rules",
	"Using Rules",
	"Default Rules",
	"Slash Commands in Rules",
	"Migrating from Prompt Library",
}

func TestRecognizeZed_RealRulesLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("zed", capmon.RecognitionContext{
		Provider:  "zed",
		Format:    "html",
		Landmarks: realZedRulesLandmarks,
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
		"activation_mode.slash_command",
		"cross_provider_recognition.agents_md",
		"cross_provider_recognition.claude_md",
		"cross_provider_recognition.gemini_md",
		"cross_provider_recognition.cursorrules",
		"cross_provider_recognition.windsurfrules",
		"cross_provider_recognition.clinerules",
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
		"rules.capabilities.auto_memory.supported",
		"rules.capabilities.hierarchical_loading.supported",
		"rules.capabilities.activation_mode.manual.supported",
		"rules.capabilities.activation_mode.model_decision.supported",
		"rules.capabilities.activation_mode.frontmatter_globs.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present for zed", absent)
		}
	}
}

// TestRecognizeZed_RulesAnchorsMissing proves the anchor-missing guardrail.
// Stripping "Migrating from Prompt Library" — one of the required anchors —
// suppresses recognition.
func TestRecognizeZed_RulesAnchorsMissing(t *testing.T) {
	mutated := []string{}
	for _, lm := range realZedRulesLandmarks {
		if lm == "Migrating from Prompt Library" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("zed", capmon.RecognitionContext{
		Provider:  "zed",
		Format:    "html",
		Landmarks: mutated,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities, got %d: %v", len(result.Capabilities), result.Capabilities)
	}
}

// TestRecognizeZed_NoLandmarks proves zero-input produces zero output (no
// false positives from empty extraction).
func TestRecognizeZed_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("zed", capmon.RecognitionContext{Provider: "zed", Format: "html"})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities, got %d", len(result.Capabilities))
	}
}

// realZedMcpLandmarks is a snapshot of headings extracted from zed's MCP
// HTML doc (.capmon-cache/zed/mcp.1/extracted.json — zed.dev/docs/ai/mcp)
// as of 2026-04-16.
//
// Zed's MCP doc maps only 2 of 8 canonical MCP keys at heading level:
// tool_filtering ("Tool Permissions") and marketplace ("As Extensions").
// The other 6 keys lack heading evidence.
var realZedMcpLandmarks = []string{
	"Model Context Protocol",
	"Supported Features",
	"Installing MCP Servers",
	"As Extensions",
	"As Custom Servers",
	"Using MCP Servers",
	"Configuration Check",
	"Agent Panel Usage",
	"Tool Permissions",
	"External Agents",
	"Error Handling",
}

// TestRecognizeZed_RealMcpLandmarks proves MCP recognition emits 2 canonical
// MCP keys at "inferred" confidence: tool_filtering and marketplace. The
// other 6 keys (transport_types, oauth_support, env_var_expansion,
// auto_approve, resource_referencing, enterprise_management) must NOT be
// emitted — none have heading-level evidence.
//
// transport_types is intentionally absent even though "As Custom Servers" /
// "As Extensions" sub-headings exist — those describe install methods, not
// transport types. The Rust struct ContextServerTransport (mcp.0) hints at
// transport abstraction but landmark matching can't anchor on a struct name
// from a code source.
//
// Test merges rules + MCP fixtures to mirror real-world cache merging.
func TestRecognizeZed_RealMcpLandmarks(t *testing.T) {
	merged := append([]string{}, realZedRulesLandmarks...)
	merged = append(merged, realZedMcpLandmarks...)
	result := capmon.RecognizeWithContext("zed", capmon.RecognitionContext{
		Provider:  "zed",
		Format:    "html",
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
		"tool_filtering",
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
		"mcp.capabilities.transport_types.supported",
		"mcp.capabilities.oauth_support.supported",
		"mcp.capabilities.env_var_expansion.supported",
		"mcp.capabilities.auto_approve.supported",
		"mcp.capabilities.resource_referencing.supported",
		"mcp.capabilities.enterprise_management.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present (no heading evidence)", absent)
		}
	}
}

// TestRecognizeZed_McpAnchorsMissing proves the required-anchor guard
// suppresses MCP emission when "Installing MCP Servers" is absent.
func TestRecognizeZed_McpAnchorsMissing(t *testing.T) {
	mutated := []string{}
	for _, lm := range realZedMcpLandmarks {
		if lm == "Installing MCP Servers" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("zed", capmon.RecognitionContext{
		Provider:  "zed",
		Format:    "html",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["mcp.supported"]; has {
		t.Error("mcp.supported should NOT be present when 'Installing MCP Servers' anchor is missing")
	}
}

// realZedAgentsLandmarks is a snapshot of headings extracted from zed's
// "Agent Settings" HTML doc (.capmon-cache/zed/agents.2/extracted.json —
// zed.dev/docs/ai/agent-settings) as of 2026-04-16.
//
// Maps 2 of 7 canonical agents keys at heading level: tool_restrictions
// ("Per-tool Permission Rules" / "Default Tool Permissions") and per_agent_mcp
// ("MCP Tool Permissions"). The other 5 canonical keys lack heading evidence
// in this doc — see recognize_zed.go zedAgentsLandmarkOptions doc-comment.
var realZedAgentsLandmarks = []string{
	"Agent Settings",
	"Model Settings",
	"Default Model",
	"Feature-specific Models",
	"Alternative Models for Inline Assists",
	"Model Temperature",
	"Agent Panel Settings",
	"Font Size",
	"Default Tool Permissions",
	"Per-tool Permission Rules",
	"Pattern Precedence",
	"Case Sensitivity",
	"copy_path and move_path Patterns",
	"MCP Tool Permissions",
	"Edit Display Mode",
	"Sound Notification",
	"Message Editor Size",
	"Modifier to Send",
	"Edit Card",
	"Terminal Card",
	"Feedback Controls",
}

// TestRecognizeZed_RealAgentsLandmarks proves agents recognition emits 2
// canonical agents keys at "inferred" confidence: tool_restrictions and
// per_agent_mcp. Five other canonical keys (definition_format,
// invocation_patterns, agent_scopes, model_selection, subagent_spawning) must
// NOT be emitted — none have heading-level evidence in the agent-settings
// doc.
//
// Test merges rules + MCP + agents fixtures to mirror real-world cache
// merging across content types. The required-anchor uniqueness gate
// ("Agent Settings" + "Per-tool Permission Rules") prevents agents emission
// from firing on rules or MCP landmarks alone.
func TestRecognizeZed_RealAgentsLandmarks(t *testing.T) {
	merged := append([]string{}, realZedRulesLandmarks...)
	merged = append(merged, realZedMcpLandmarks...)
	merged = append(merged, realZedAgentsLandmarks...)
	result := capmon.RecognizeWithContext("zed", capmon.RecognitionContext{
		Provider:  "zed",
		Format:    "html",
		Landmarks: merged,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["agents.supported"] != "true" {
		t.Error("agents.supported missing")
	}
	agentsInferred := []string{
		"tool_restrictions",
		"per_agent_mcp",
	}
	for _, c := range agentsInferred {
		key := "agents.capabilities." + c + ".supported"
		if caps[key] != "true" {
			t.Errorf("%s missing", key)
		}
		if got := caps["agents.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("agents.%s.confidence = %q, want inferred", c, got)
		}
	}
	for _, absent := range []string{
		"agents.capabilities.definition_format.supported",
		"agents.capabilities.invocation_patterns.supported",
		"agents.capabilities.agent_scopes.supported",
		"agents.capabilities.model_selection.supported",
		"agents.capabilities.subagent_spawning.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present (no heading evidence)", absent)
		}
	}
}

// TestRecognizeZed_AgentsAnchorsMissing proves the required-anchor guard
// suppresses agents emission when "Per-tool Permission Rules" is absent.
// This is critical because the rules and mcp docs both contain landmarks
// that could otherwise trigger false-positive agents recognition.
func TestRecognizeZed_AgentsAnchorsMissing(t *testing.T) {
	mutated := []string{}
	for _, lm := range realZedAgentsLandmarks {
		if lm == "Per-tool Permission Rules" {
			continue
		}
		mutated = append(mutated, lm)
	}
	// Merge with rules + mcp to simulate real cache state minus the agents
	// uniqueness anchor.
	merged := append([]string{}, realZedRulesLandmarks...)
	merged = append(merged, realZedMcpLandmarks...)
	merged = append(merged, mutated...)
	result := capmon.RecognizeWithContext("zed", capmon.RecognitionContext{
		Provider:  "zed",
		Format:    "html",
		Landmarks: merged,
	})
	if _, has := result.Capabilities["agents.supported"]; has {
		t.Error("agents.supported should NOT be present when 'Per-tool Permission Rules' anchor is missing")
	}
	for _, absent := range []string{
		"agents.capabilities.tool_restrictions.supported",
		"agents.capabilities.per_agent_mcp.supported",
	} {
		if _, has := result.Capabilities[absent]; has {
			t.Errorf("%s should NOT be present without required anchor", absent)
		}
	}
}

// TestRecognizeZed_InstanceLandmarksNoMatch proves zed's own .rules instance
// file (Rust coding guidelines) does NOT trigger recognition. This is the
// instance-vs-spec guardrail: rules.0 is example content, not vocabulary.
func TestRecognizeZed_InstanceLandmarksNoMatch(t *testing.T) {
	instanceLandmarks := []string{
		"Rust coding guidelines",
		"Timers in tests",
		"GPUI",
		"Concurrency",
		"Rules Hygiene",
		"After any agentic session",
		"High bar for new rules",
		"What NOT to put in .rules",
	}
	result := capmon.RecognizeWithContext("zed", capmon.RecognitionContext{
		Provider:  "zed",
		Format:    "html",
		Landmarks: instanceLandmarks,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities from instance landmarks, got %d: %v",
			len(result.Capabilities), result.Capabilities)
	}
}
