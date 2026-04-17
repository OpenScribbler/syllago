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

// realCursorMcpLandmarks is a snapshot of the headings from cursor's MCP doc
// (.capmon-cache/cursor/mcp.0/extracted.json — cursor.com/docs/context/mcp,
// HTML) as of 2026-04-16. Only entries from the actual `landmarks` array are
// included — table cells like "Resources", "Tools", "Prompts" live in Fields
// not Landmarks and cannot be anchored on via substring matching.
//
// Cursor's MCP doc maps 6 of 8 canonical MCP keys via heading-level evidence.
// resource_referencing and enterprise_management are absent (table-cell-only
// and admin-console-only respectively).
var realCursorMcpLandmarks = []string{
	// Top nav (shared across cursor docs, present here too)
	"Command Palette",
	"Get Started",
	"Agent",
	"Customizing",
	"Cloud Agents",
	"Integrations",
	"CLI",
	"Teams & Enterprise",
	// MCP discovery + protocol
	"Model Context Protocol (MCP)",
	"What is MCP?",
	"Why use MCP?",
	"How it works",
	"Protocol and extension support",
	"MCP apps",
	// Installation
	"Installing MCP servers",
	"One-click installation",
	"Using mcp.json",
	"Configuration locations",
	// Transport types (stdio is the only one with a dedicated heading)
	"STDIO server configuration",
	// OAuth
	"Static OAuth for remote servers",
	"Static redirect URL",
	"Authentication",
	// Config interpolation
	"Combining with config interpolation",
	"Config interpolation",
	// Tool / approval surface
	"Using MCP in chat",
	"Tool approval",
	"Auto-run",
	"Tool response",
	"Images as context",
	// Other sections + FAQs
	"Using the Extension API",
	"Security considerations",
	"Real-world examples",
	"FAQ",
	"What's the point of MCP servers?",
	"How do I debug MCP server issues?",
	"Can I temporarily disable an MCP server?",
	"What happens if an MCP server crashes or times out?",
	"How do I update an MCP server?",
	"Can I use MCP servers with sensitive data?",
}

// TestRecognizeCursor_RealMcpLandmarks proves MCP recognition emits 6
// canonical MCP keys at "inferred" confidence: transport_types, oauth_support,
// env_var_expansion, tool_filtering, auto_approve, marketplace.
// resource_referencing and enterprise_management must NOT be emitted —
// resource_referencing has table-cell-only evidence (not in Landmarks),
// enterprise_management has no heading evidence at all.
//
// Test merges rules + MCP fixtures to mirror real-world cache merging — the
// recognizer must distinguish MCP capabilities from rules ones via the
// required-anchor uniqueness gate.
func TestRecognizeCursor_RealMcpLandmarks(t *testing.T) {
	merged := append([]string{}, realCursorRulesLandmarks...)
	merged = append(merged, realCursorMcpLandmarks...)
	result := capmon.RecognizeWithContext("cursor", capmon.RecognitionContext{
		Provider:  "cursor",
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
		"transport_types",
		"oauth_support",
		"env_var_expansion",
		"tool_filtering",
		"auto_approve",
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
		"mcp.capabilities.resource_referencing.supported",
		"mcp.capabilities.enterprise_management.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present (no heading evidence)", absent)
		}
	}
}

// TestRecognizeCursor_McpAnchorsMissing proves the required-anchor guard
// suppresses MCP emission when "What is MCP?" is absent — preventing MCP
// patterns from firing on a context that happens to include "Tool approval"
// or "OAuth" landmarks from a non-MCP doc.
func TestRecognizeCursor_McpAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realCursorMcpLandmarks))
	for _, lm := range realCursorMcpLandmarks {
		if lm == "What is MCP?" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("cursor", capmon.RecognitionContext{
		Provider:  "cursor",
		Format:    "html",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["mcp.supported"]; has {
		t.Error("mcp.supported should NOT be present when 'What is MCP?' anchor is missing")
	}
}
