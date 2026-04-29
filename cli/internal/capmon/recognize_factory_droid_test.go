package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realFactoryDroidLandmarks is a snapshot of the headings extracted from
// Factory Droid's skills doc (.capmon-cache/factory-droid/skills.0/extracted.json)
// as of 2026-04-17, after the source manifest switched to
// fetch_method=chromedp + format=html for docs.factory.ai pages (Mintlify SPA).
//
// Mintlify landmarks have a leading zero-width space prefix (e.g.
// "\u200bSkill file format") which substring matchers handle transparently.
// Update this fixture when the upstream doc evolves.
var realFactoryDroidLandmarks = []string{
	"Skills",
	"\u200bWhat is a skill?",
	"\u200bSkill file format",
	"\u200bWhere skills live",
	"\u200bFrontmatter reference",
	"\u200bControl who invokes a skill",
	"\u200bInvocation summary",
	"\u200bQuickstart",
	"\u200bHow skills differ from other configuration",
	"\u200bWhy skills matter in enterprise codebases",
	"\u200bBest practices",
	"\u200bCookbook",
}

// realFactoryDroidHooksLandmarks is a snapshot of the headings extracted from
// Factory Droid's hooks reference doc
// (.capmon-cache/factory-droid/hooks.1/extracted.json) as of 2026-04-17.
//
// Per the curated format YAML (docs/provider-formats/factory-droid.yaml),
// only decision_control is supported among the 9 canonical hooks keys —
// Factory Droid hooks signal via exit codes. The other 8 keys are curated as
// unsupported and must NOT be emitted.
var realFactoryDroidHooksLandmarks = []string{
	"Hooks reference",
	"\u200bConfiguration",
	"\u200bHook Events",
	"\u200bPreToolUse",
	"\u200bPostToolUse",
	"\u200bUserPromptSubmit",
	"\u200bStop",
	"\u200bSessionStart",
	"\u200bHook Output",
	"\u200bSimple: Exit Code",
	"\u200bExit Code 2 Behavior",
	"\u200bAdvanced: JSON Output",
	"\u200bPreToolUse Decision Control",
	"\u200bPostToolUse Decision Control",
	"\u200bUserPromptSubmit Decision Control",
	"\u200bStop/SubagentStop Decision Control",
	"\u200bSessionStart Decision Control",
	"\u200bSessionEnd Decision Control",
}

// TestRecognizeFactoryDroid_RealLandmarks verifies skills recognition fires
// against the actual cache snapshot. Static facts (project_scope, global_scope,
// canonical_filename) merge in at "confirmed" after the landmark match.
func TestRecognizeFactoryDroid_RealLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: realFactoryDroidLandmarks,
	})
	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["skills.supported"] != "true" {
		t.Error("skills.supported missing")
	}
	for _, c := range []string{"frontmatter", "creation_workflow", "directory_structure", "invocation"} {
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

// TestRecognizeFactoryDroid_AnchorsMissing verifies the required-anchor guard
// suppresses skills emission when one of the unique skills anchors is absent.
func TestRecognizeFactoryDroid_AnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realFactoryDroidLandmarks))
	for _, lm := range realFactoryDroidLandmarks {
		if lm == "\u200bSkill file format" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if _, has := result.Capabilities["skills.supported"]; has {
		t.Error("skills.supported should NOT be present when 'Skill file format' anchor is missing")
	}
}

// TestRecognizeFactoryDroid_NoLandmarks verifies clean suppression with no
// landmark input.
func TestRecognizeFactoryDroid_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{Provider: "factory-droid", Format: "markdown"})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
	if len(result.Capabilities) != 0 {
		t.Errorf("expected zero capabilities, got %d", len(result.Capabilities))
	}
}

// TestRecognizeFactoryDroid_RealHooksLandmarks proves hooks recognition fires
// against the merged skills + hooks cache snapshot, emits the curated-supported
// decision_control capability at "inferred" confidence (the only canonical
// hooks key Factory Droid supports), and does NOT emit the 8 curated-as-
// unsupported keys.
func TestRecognizeFactoryDroid_RealHooksLandmarks(t *testing.T) {
	merged := append([]string{}, realFactoryDroidLandmarks...)
	merged = append(merged, realFactoryDroidHooksLandmarks...)
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
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
	if caps["hooks.capabilities.decision_control.supported"] != "true" {
		t.Error("hooks.capabilities.decision_control.supported missing")
	}
	if got := caps["hooks.capabilities.decision_control.confidence"]; got != "inferred" {
		t.Errorf("hooks.capabilities.decision_control.confidence = %q, want inferred", got)
	}
	for _, absent := range []string{
		"hooks.capabilities.handler_types.supported",
		"hooks.capabilities.matcher_patterns.supported",
		"hooks.capabilities.input_modification.supported",
		"hooks.capabilities.async_execution.supported",
		"hooks.capabilities.hook_scopes.supported",
		"hooks.capabilities.json_io_protocol.supported",
		"hooks.capabilities.context_injection.supported",
		"hooks.capabilities.permission_control.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present for factory-droid (curated as unsupported)", absent)
		}
	}
}

// realFactoryDroidAgentsLandmarks is a snapshot of the headings extracted from
// Factory Droid's "Custom Droids (Subagents)" doc
// (.capmon-cache/factory-droid/agents.0/extracted.json) as of 2026-04-17.
// Mintlify zero-width space prefix on H2/H3/H4 entries handled transparently
// by substring matchers.
var realFactoryDroidAgentsLandmarks = []string{
	"Custom Droids (Subagents)",
	"\u200b1 · What are custom droids?",
	"\u200b2 · Why use them?",
	"\u200b3 · Quick start",
	"\u200b4 · Configuration",
	"\u200bTool categories → concrete tools",
	"\u200b5 · Managing droids in the UI",
	"\u200b5.5 · Importing Claude Code subagents",
	"\u200bHow to import",
	"\u200bWhat happens during import",
	"\u200bExample import flow",
	"\u200bHandling tool validation errors",
	"\u200b6 · Using custom droids effectively",
	"\u200b7 · Examples",
	"\u200bCode reviewer (project scope)",
	"\u200bSecurity sweeper (personal scope)",
	"\u200bTask coordinator (with live progress)",
}

// TestRecognizeFactoryDroid_RealAgentsLandmarks proves agents recognition
// emits 2 canonical agents keys at "inferred" confidence: definition_format
// and tool_restrictions. The other 5 (invocation_patterns, agent_scopes,
// model_selection, per_agent_mcp, subagent_spawning) must NOT be emitted —
// no heading-level evidence in the agents doc, even though the curator marks
// most of them supported from broader source knowledge.
//
// Test merges skills + hooks + agents fixtures to mirror real-world cache
// merging — the agents recognizer must distinguish its capabilities from
// the others via the required-anchor uniqueness gate.
func TestRecognizeFactoryDroid_RealAgentsLandmarks(t *testing.T) {
	merged := append([]string{}, realFactoryDroidLandmarks...)
	merged = append(merged, realFactoryDroidHooksLandmarks...)
	merged = append(merged, realFactoryDroidAgentsLandmarks...)
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: merged,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["agents.supported"] != "true" {
		t.Error("agents.supported missing")
	}
	for _, c := range []string{"definition_format", "tool_restrictions"} {
		key := "agents.capabilities." + c + ".supported"
		if caps[key] != "true" {
			t.Errorf("%s missing", key)
		}
		if got := caps["agents.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("agents.%s.confidence = %q, want inferred", c, got)
		}
	}
	for _, absent := range []string{
		"agents.capabilities.invocation_patterns.supported",
		"agents.capabilities.agent_scopes.supported",
		"agents.capabilities.model_selection.supported",
		"agents.capabilities.per_agent_mcp.supported",
		"agents.capabilities.subagent_spawning.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present (no heading evidence; curator-only)", absent)
		}
	}
}

// TestRecognizeFactoryDroid_AgentsAnchorsMissing proves the required-anchor
// guard suppresses agents emission when "Tool categories" is absent —
// preventing agents patterns from firing on contexts that mention only the
// parent "Custom Droids" landmark.
func TestRecognizeFactoryDroid_AgentsAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realFactoryDroidAgentsLandmarks))
	for _, lm := range realFactoryDroidAgentsLandmarks {
		if lm == "\u200bTool categories → concrete tools" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["agents.supported"]; has {
		t.Error("agents.supported should NOT be present when 'Tool categories' anchor is missing")
	}
}

// TestRecognizeFactoryDroid_HooksAnchorsMissing proves the required-anchor
// guard suppresses hooks emission when "Hooks reference" is absent — without
// the guard, the substring "Decision Control" pattern would fire on any
// content type cached from a doc that mentions decision control.
func TestRecognizeFactoryDroid_HooksAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realFactoryDroidHooksLandmarks))
	for _, lm := range realFactoryDroidHooksLandmarks {
		if lm == "Hooks reference" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["hooks.supported"]; has {
		t.Error("hooks.supported should NOT be present when 'Hooks reference' anchor is missing")
	}
}

// realFactoryDroidCommandsLandmarks is a snapshot of the headings extracted
// from Factory Droid's "Custom Slash Commands" doc
// (.capmon-cache/factory-droid/commands.0/extracted.json) as of 2026-04-17.
// Mintlify zero-width space prefix on H2/H3 entries is handled transparently
// by substring matchers.
var realFactoryDroidCommandsLandmarks = []string{
	"Custom Slash Commands",
	"\u200b1 · Discovery & naming",
	"\u200b2 · Markdown commands",
	"\u200b3 · Executable commands",
	"\u200b4 · Managing commands",
	"\u200b5 · Usage patterns",
	"\u200b6 · Examples",
	"\u200bCode review rubric (Markdown)",
	"\u200bDaily standup helper (Markdown)",
	"\u200bRegression smoke test (Executable)",
}

// TestRecognizeFactoryDroid_RealCommandsLandmarks proves commands recognition
// fires on the merged skills+hooks+agents+commands fixture, emits
// argument_substitution at "inferred" confidence, and does NOT emit
// builtin_commands (intentionally unmapped — factory-droid has no built-in
// slash commands per the curator).
func TestRecognizeFactoryDroid_RealCommandsLandmarks(t *testing.T) {
	merged := append([]string{}, realFactoryDroidLandmarks...)
	merged = append(merged, realFactoryDroidHooksLandmarks...)
	merged = append(merged, realFactoryDroidAgentsLandmarks...)
	merged = append(merged, realFactoryDroidCommandsLandmarks...)
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: merged,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["commands.supported"] != "true" {
		t.Error("commands.supported missing")
	}
	if caps["commands.capabilities.argument_substitution.supported"] != "true" {
		t.Error("commands.capabilities.argument_substitution.supported missing")
	}
	if got := caps["commands.capabilities.argument_substitution.confidence"]; got != "inferred" {
		t.Errorf("commands.argument_substitution.confidence = %q, want inferred", got)
	}
	if _, has := caps["commands.capabilities.builtin_commands.supported"]; has {
		t.Error("commands.capabilities.builtin_commands.supported should NOT be present (factory-droid has no built-in slash commands)")
	}
}

// TestRecognizeFactoryDroid_CommandsAnchorsMissing proves the required-anchor
// guard suppresses commands emission when "Markdown commands" is absent —
// preventing patterns from firing on contexts that only mention "Custom
// Slash Commands" without the taxonomy heading.
func TestRecognizeFactoryDroid_CommandsAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realFactoryDroidCommandsLandmarks))
	for _, lm := range realFactoryDroidCommandsLandmarks {
		if lm == "\u200b2 · Markdown commands" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["commands.supported"]; has {
		t.Error("commands.supported should NOT be present when 'Markdown commands' anchor is missing")
	}
}

// realFactoryDroidRulesLandmarks is a snapshot of the headings extracted from
// Factory Droid's rules doc (.capmon-cache/factory-droid/rules.0/extracted.json)
// as of 2026-04-17. The source URL (docs.factory.ai/cli/configuration) is the
// AGENTS.md cross-provider rules format documentation page (Mintlify SPA
// fetched via chromedp).
//
// Mintlify landmarks have a leading zero-width space prefix (e.g.
// "\u200b3 · File locations & discovery hierarchy"); substring matchers handle
// this transparently. Update this fixture when the upstream doc evolves.
var realFactoryDroidRulesLandmarks = []string{
	"AGENTS.md",
	"\u200b1 · What is AGENTS.md?",
	"\u200bWhy AGENTS.md?",
	"\u200bWhat it contains:",
	"\u200b2 · One AGENTS.md works across many agents",
	"\u200b3 · File locations & discovery hierarchy",
	"\u200b4 · File structure & syntax",
	"\u200b5 · Common sections",
	"\u200b6 · Templates & examples",
	"\u200bFactory-style comprehensive example",
	"\u200bNode + React monorepo",
	"\u200bPython microservice",
	"\u200b7 · Best practices",
	"\u200b8 · How agents use AGENTS.md",
	"\u200b9 · When things go wrong",
	"\u200bWarning signs of agent drift:",
	"\u200bRecovery playbook:",
	"\u200b10 · Getting started",
	"Specification Mode",
	"Auto-Run",
	"\u200bSummary",
}

// TestRecognizeFactoryDroid_RealRulesLandmarks proves rules recognition fires
// from the AGENTS.md doc and emits the two supported canonical-key signals
// (cross_provider_recognition.agents_md, hierarchical_loading) at inferred
// confidence. The other three canonical rules keys (activation_mode,
// file_imports, auto_memory) are curated as unsupported and must NOT be
// emitted.
func TestRecognizeFactoryDroid_RealRulesLandmarks(t *testing.T) {
	merged := append([]string{}, realFactoryDroidLandmarks...)
	merged = append(merged, realFactoryDroidHooksLandmarks...)
	merged = append(merged, realFactoryDroidAgentsLandmarks...)
	merged = append(merged, realFactoryDroidCommandsLandmarks...)
	merged = append(merged, realFactoryDroidRulesLandmarks...)
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: merged,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["rules.supported"] != "true" {
		t.Errorf("rules.supported = %q, want %q", caps["rules.supported"], "true")
	}
	for _, key := range []string{
		"rules.capabilities.cross_provider_recognition.agents_md.supported",
		"rules.capabilities.cross_provider_recognition.agents_md.mechanism",
		"rules.capabilities.cross_provider_recognition.agents_md.confidence",
		"rules.capabilities.hierarchical_loading.supported",
		"rules.capabilities.hierarchical_loading.mechanism",
		"rules.capabilities.hierarchical_loading.confidence",
	} {
		if _, ok := caps[key]; !ok {
			t.Errorf("%s missing", key)
		}
	}
	for _, c := range []string{"cross_provider_recognition.agents_md", "hierarchical_loading"} {
		if got := caps["rules.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("rules.capabilities.%s.confidence = %q, want inferred", c, got)
		}
	}
	for _, absent := range []string{
		"rules.capabilities.activation_mode.supported",
		"rules.capabilities.file_imports.supported",
		"rules.capabilities.auto_memory.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present (curated as unsupported)", absent)
		}
	}
}

// TestRecognizeFactoryDroid_RulesAnchorsMissing proves the required-anchor
// guard suppresses rules emission when "1 · What is AGENTS.md?" is absent —
// preventing patterns from firing on contexts that mention AGENTS.md only
// in passing (e.g. agents docs that mention AGENTS.md interop).
func TestRecognizeFactoryDroid_RulesAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realFactoryDroidRulesLandmarks))
	for _, lm := range realFactoryDroidRulesLandmarks {
		if lm == "\u200b1 · What is AGENTS.md?" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["rules.supported"]; has {
		t.Error("rules.supported should NOT be present when '1 · What is AGENTS.md?' anchor is missing")
	}
}

// realFactoryDroidMcpLandmarks is a snapshot of the headings extracted from
// Factory Droid's MCP doc (.capmon-cache/factory-droid/mcp.0/extracted.json)
// as of 2026-04-28. The source URL switched from docs.factory.ai/llms.txt
// (a 4-landmark navigation index) to docs.factory.ai/cli/configuration/mcp
// (a 19-landmark Mintlify SPA fetched via chromedp).
//
// Mintlify landmarks have a leading zero-width space prefix on H2/H3 entries
// (e.g. "\u200bAdding HTTP Servers"); substring matchers handle this transparently.
// Update this fixture when the upstream doc evolves.
var realFactoryDroidMcpLandmarks = []string{
	"Model Context Protocol (MCP)",
	"\u200bQuick Start: Add from Registry",
	"\u200bInteractive Manager (/mcp)",
	"\u200bAdding Servers via CLI",
	"\u200bAdding HTTP Servers",
	"\u200bPopular HTTP MCP Servers",
	"\u200bDevelopment & Testing",
	"\u200bProject Management & Documentation",
	"\u200bPayments & Commerce",
	"\u200bDesign & Media",
	"\u200bInfrastructure & DevOps",
	"\u200bAdding Stdio Servers",
	"\u200bPopular Stdio MCP Servers",
	"\u200bRemoving Servers",
	"\u200bManaging Servers",
	"\u200bConfiguration",
	"\u200bHow Layering Works",
	"\u200bOAuth Tokens",
	"\u200bConfiguration Schema",
}

// TestRecognizeFactoryDroid_RealMcpLandmarks proves MCP recognition fires from
// the real docs page and emits 4 of 8 canonical MCP keys at "inferred"
// confidence: transport_types, oauth_support, tool_filtering, marketplace.
// The other 4 (env_var_expansion, auto_approve, resource_referencing,
// enterprise_management) are curated as unsupported on the live page and must
// NOT be emitted.
//
// Test merges all five other content-type fixtures to mirror real-world cache
// merging — the MCP recognizer must distinguish its capabilities from rules,
// hooks, agents, commands, and skills via the required-anchor uniqueness gate.
func TestRecognizeFactoryDroid_RealMcpLandmarks(t *testing.T) {
	merged := append([]string{}, realFactoryDroidLandmarks...)
	merged = append(merged, realFactoryDroidHooksLandmarks...)
	merged = append(merged, realFactoryDroidAgentsLandmarks...)
	merged = append(merged, realFactoryDroidCommandsLandmarks...)
	merged = append(merged, realFactoryDroidRulesLandmarks...)
	merged = append(merged, realFactoryDroidMcpLandmarks...)
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
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
	for _, c := range []string{"transport_types", "oauth_support", "tool_filtering", "marketplace"} {
		key := "mcp.capabilities." + c + ".supported"
		if caps[key] != "true" {
			t.Errorf("%s missing", key)
		}
		if got := caps["mcp.capabilities."+c+".confidence"]; got != "inferred" {
			t.Errorf("mcp.%s.confidence = %q, want inferred", c, got)
		}
	}
	for _, absent := range []string{
		"mcp.capabilities.env_var_expansion.supported",
		"mcp.capabilities.auto_approve.supported",
		"mcp.capabilities.resource_referencing.supported",
		"mcp.capabilities.enterprise_management.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present (curated as unsupported on the live page)", absent)
		}
	}
}

// TestRecognizeFactoryDroid_McpAnchorsMissing proves the required-anchor guard
// suppresses MCP emission when "Configuration Schema" is absent — preventing
// patterns from firing on contexts that mention "OAuth Tokens" or
// "Adding HTTP Servers" in passing (e.g. a hooks doc snippet) without the MCP
// page's structural anchor.
func TestRecognizeFactoryDroid_McpAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realFactoryDroidMcpLandmarks))
	for _, lm := range realFactoryDroidMcpLandmarks {
		if lm == "\u200bConfiguration Schema" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("factory-droid", capmon.RecognitionContext{
		Provider:  "factory-droid",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["mcp.supported"]; has {
		t.Error("mcp.supported should NOT be present when 'Configuration Schema' anchor is missing")
	}
}
