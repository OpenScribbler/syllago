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

// realCursorSkillsLandmarks is a snapshot of the headings from cursor's Skills
// doc (.capmon-cache/cursor/skills.0/extracted.json — cursor.com/docs/skills,
// HTML) as of 2026-04-22. Cursor implements the Agent Skills open standard;
// the doc covers the SKILL.md frontmatter shape, scope directories, and the
// disable_model_invocation toggle as headings.
var realCursorSkillsLandmarks = []string{
	// Top nav (shared across cursor docs)
	"Command Palette",
	"Get Started",
	"Agent",
	"Customizing",
	"Cloud Agents",
	"Integrations",
	"CLI",
	"Teams & Enterprise",
	// Skills page
	"Agent Skills",
	"What are skills?",
	"How skills work",
	"Skill directories",
	"SKILL.md file format",
	"Frontmatter fields",
	"Disabling automatic invocation",
	"Including scripts in skills",
	"Optional directories",
	"Viewing skills",
	"Installing skills from GitHub",
	"Migrating rules and commands to skills",
	"Learn more",
}

// TestRecognizeCursor_RealSkillsLandmarks proves Skills recognition emits the
// expected canonical skills keys at the appropriate confidence levels:
//   - canonical_filename "inferred" — anchored on "SKILL.md file format"
//   - disable_model_invocation "inferred" — anchored on "Disabling automatic
//     invocation"
//   - skills.supported = "true" — implied by any successful pattern
//
// Test merges rules + skills fixtures so the recognizer must distinguish
// skills capabilities from rules ones via the required-anchor uniqueness
// gate.
func TestRecognizeCursor_RealSkillsLandmarks(t *testing.T) {
	merged := append([]string{}, realCursorRulesLandmarks...)
	merged = append(merged, realCursorSkillsLandmarks...)
	result := capmon.RecognizeWithContext("cursor", capmon.RecognitionContext{
		Provider:  "cursor",
		Format:    "html",
		Landmarks: merged,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["skills.supported"] != "true" {
		t.Error("skills.supported missing")
	}
	skillsInferred := []string{
		"canonical_filename",
		"disable_model_invocation",
	}
	for _, c := range skillsInferred {
		key := "skills.capabilities." + c + ".supported"
		if caps[key] != "true" {
			t.Errorf("%s missing", key)
		}
		if got := caps["skills.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("skills.%s.confidence = %q, want inferred", c, got)
		}
	}
}

// TestRecognizeCursor_SkillsAnchorsMissing proves the required-anchor guard
// suppresses skills emission when "SKILL.md file format" is absent — so
// rules/mcp landmarks alone cannot trigger skills emission.
func TestRecognizeCursor_SkillsAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realCursorSkillsLandmarks))
	for _, lm := range realCursorSkillsLandmarks {
		if lm == "SKILL.md file format" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("cursor", capmon.RecognitionContext{
		Provider:  "cursor",
		Format:    "html",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["skills.supported"]; has {
		t.Error("skills.supported should NOT be present when 'SKILL.md file format' anchor is missing")
	}
}

// realCursorHooksLandmarks is a snapshot of the headings from cursor's Hooks
// doc (.capmon-cache/cursor/hooks.0/extracted.json — cursor.com/docs/hooks,
// HTML) as of 2026-04-22. Cursor documents an extensive lifecycle event set,
// matcher configuration, and JSON I/O protocol as headings.
var realCursorHooksLandmarks = []string{
	// Top nav
	"Command Palette",
	"Get Started",
	"Agent",
	"Customizing",
	"Cloud Agents",
	"Integrations",
	"CLI",
	"Teams & Enterprise",
	// Hooks page
	"Hooks",
	"Agent and Tab Support",
	"Quickstart",
	"Hook Types",
	"Command-Based Hooks",
	"Prompt-Based Hooks",
	"Examples",
	"TypeScript stop automation hook",
	"Python manifest guard hook",
	"Partner Integrations",
	"MCP governance and visibility",
	"Code security and best practices",
	"Dependency security",
	"Agent security and safety",
	"Secrets management",
	"Configuration",
	"Configuration file",
	"Global Configuration Options",
	"Per-Script Configuration Options",
	"Matcher Configuration",
	"Team Distribution",
	"Project Hooks (Version Control)",
	"MDM Distribution",
	"Cloud Distribution (Enterprise Only)",
	"Reference",
	"Common schema",
	"Input (all hooks)",
	"Hook events",
	"preToolUse",
	"postToolUse",
	"postToolUseFailure",
	"subagentStart",
	"subagentStop",
	"beforeShellExecution / beforeMCPExecution",
	"afterShellExecution",
	"afterMCPExecution",
	"afterFileEdit",
	"beforeReadFile",
	"beforeTabFileRead",
	"afterTabFileEdit",
	"beforeSubmitPrompt",
	"afterAgentResponse",
	"afterAgentThought",
	"stop",
	"sessionStart",
	"sessionEnd",
	"preCompact",
	"Environment Variables",
	"Troubleshooting",
}

// TestRecognizeCursor_RealHooksLandmarks proves Hooks recognition emits the
// expected canonical hooks keys at "inferred" confidence:
//   - matcher_patterns — anchored on "Matcher Configuration" heading
//   - hook_scopes — anchored on "Project Hooks (Version Control)" heading
//   - json_io_protocol — anchored on "Input (all hooks)" heading
//
// Test merges rules + hooks fixtures so the recognizer must distinguish hooks
// capabilities from rules ones via the required-anchor uniqueness gate.
func TestRecognizeCursor_RealHooksLandmarks(t *testing.T) {
	merged := append([]string{}, realCursorRulesLandmarks...)
	merged = append(merged, realCursorHooksLandmarks...)
	result := capmon.RecognizeWithContext("cursor", capmon.RecognitionContext{
		Provider:  "cursor",
		Format:    "html",
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
		"matcher_patterns",
		"hook_scopes",
		"json_io_protocol",
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
	// handler_types must NOT be emitted — the format-YAML curator marks it
	// supported: false confirmed (cursor hooks execute shell commands only).
	// The cache does include "Command-Based Hooks" and "Prompt-Based Hooks"
	// landmarks suggesting the curator's claim may be outdated, but the
	// recognizer must not contradict the curator's confirmed judgment.
	if _, has := caps["hooks.capabilities.handler_types.supported"]; has {
		t.Error("hooks.capabilities.handler_types should NOT be emitted (curator marks supported: false confirmed)")
	}
}

// TestRecognizeCursor_HooksAnchorsMissing proves the required-anchor guard
// suppresses hooks emission when "Hook Types" is absent — preventing hooks
// patterns from firing on a context that contains only event-name landmarks
// scraped from a non-hooks doc.
func TestRecognizeCursor_HooksAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realCursorHooksLandmarks))
	for _, lm := range realCursorHooksLandmarks {
		if lm == "Hook Types" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("cursor", capmon.RecognitionContext{
		Provider:  "cursor",
		Format:    "html",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["hooks.supported"]; has {
		t.Error("hooks.supported should NOT be present when 'Hook Types' anchor is missing")
	}
}

// realCursorAgentsLandmarks is a snapshot of headings extracted from
// .capmon-cache/cursor/agents.0/extracted.json (cursor.com/docs/subagents,
// HTML) as of 2026-04-28. Cursor's subagents doc maps 6 of 7 canonical agents
// keys at heading-level evidence; per_agent_mcp is correctly absent because
// subagents inherit MCP from the parent rather than scoping per-agent.
//
// Source URL was migrated from cursor.com/docs/agent/overview (404'd / built-in
// Agent feature page) to cursor.com/docs/subagents (file-based custom subagents
// surface) on 2026-04-28 per .claude/rules/capmon-drift-detection.md workflow.
var realCursorAgentsLandmarks = []string{
	// Top nav
	"Command Palette",
	"Get Started",
	"Agent",
	"Customizing",
	"Cloud Agents",
	"Integrations",
	"SDK",
	"CLI",
	"Teams & Enterprise",
	// Subagents page
	"Subagents",
	"How subagents work",
	"Foreground vs background",
	"Built-in subagents",
	"Why these subagents exist",
	"When to use subagents",
	"Quick start",
	"Custom subagents",
	"File locations",
	"File format",
	"Configuration fields",
	"Model configuration",
	"When the configured model won't be used",
	"Using subagents",
	"Automatic delegation",
	"Explicit invocation",
	"Parallel execution",
	"Resuming subagents",
	"Common patterns",
	"Verification agent",
	"Orchestrator pattern",
	"Example subagents",
	"Debugger",
	"Test runner",
	"Best practices",
	"Anti-patterns to avoid",
	"Managing subagents",
	"Creating subagents",
	"Viewing subagents",
	"Performance and cost",
	"Token and cost considerations",
	"FAQ",
	"What are the built-in subagents?",
	"Can subagents launch other subagents?",
	"How do I see what a subagent is doing?",
	"What happens if a subagent fails?",
	"Can I use MCP tools in subagents?",
	"How do I debug a misbehaving subagent?",
	"Why is my subagent using a different model?",
}

// TestRecognizeCursor_RealAgentsLandmarks proves Agents recognition emits the
// expected canonical agents keys at "inferred" confidence:
//   - definition_format         — anchored on "File format" heading
//   - tool_restrictions         — anchored on "Configuration fields"
//   - invocation_patterns.automatic_delegation — anchored on "Automatic delegation"
//   - invocation_patterns.explicit             — anchored on "Explicit invocation"
//   - agent_scopes.project, agent_scopes.user  — anchored on "File locations"
//   - model_selection           — anchored on "Model configuration"
//   - subagent_spawning         — anchored on FAQ "Can subagents launch other subagents?"
//
// Test merges rules + agents fixtures so the recognizer must distinguish
// agents capabilities from rules ones via the required-anchor uniqueness gate.
func TestRecognizeCursor_RealAgentsLandmarks(t *testing.T) {
	merged := append([]string{}, realCursorRulesLandmarks...)
	merged = append(merged, realCursorAgentsLandmarks...)
	result := capmon.RecognizeWithContext("cursor", capmon.RecognitionContext{
		Provider:  "cursor",
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
	agentsKeys := []string{
		"definition_format",
		"tool_restrictions",
		"invocation_patterns.automatic_delegation",
		"invocation_patterns.explicit",
		"agent_scopes.project",
		"agent_scopes.user",
		"model_selection",
		"subagent_spawning",
	}
	for _, c := range agentsKeys {
		key := "agents.capabilities." + c + ".supported"
		if caps[key] != "true" {
			t.Errorf("%s missing", key)
		}
		if got := caps["agents.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("agents.%s.confidence = %q, want inferred", c, got)
		}
	}
	// per_agent_mcp must NOT be emitted — subagents inherit MCP from parent
	// rather than scoping per-agent. Format YAML correctly marks supported:
	// false (inferred). The subagents page has an FAQ "Can I use MCP tools in
	// subagents?" but the answer ("Subagents inherit all tools from the
	// parent") is body prose, not an anchor for per-agent scoping.
	if _, has := caps["agents.capabilities.per_agent_mcp.supported"]; has {
		t.Error("agents.capabilities.per_agent_mcp should NOT be emitted (subagents inherit MCP from parent, not per-agent)")
	}
}

// TestRecognizeCursor_AgentsAnchorsMissing proves the required-anchor guard
// suppresses agents emission when "Custom subagents" is absent — preventing
// the agents patterns from firing on a context that contains only generic
// "File format" or "Configuration fields" landmarks scraped from a non-agents
// doc.
func TestRecognizeCursor_AgentsAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realCursorAgentsLandmarks))
	for _, lm := range realCursorAgentsLandmarks {
		if lm == "Custom subagents" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("cursor", capmon.RecognitionContext{
		Provider:  "cursor",
		Format:    "html",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["agents.supported"]; has {
		t.Error("agents.supported should NOT be present when 'Custom subagents' anchor is missing")
	}
}
