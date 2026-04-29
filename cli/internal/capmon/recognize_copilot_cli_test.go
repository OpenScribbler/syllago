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

// realCopilotCliHooksLandmarks is a snapshot of the headings from Copilot
// CLI's hooks-configuration doc (.capmon-cache/copilot-cli/hooks.0/extracted.json)
// as of 2026-04-16. Update when the doc evolves.
var realCopilotCliHooksLandmarks = []string{
	"Hook types",
	"Session start hook",
	"Session end hook",
	"User prompt submitted hook",
	"Pre-tool use hook",
	"Post-tool use hook",
	"Error occurred hook",
	"Script best practices",
	"Reading input",
	"Outputting JSON",
	"Error handling",
	"Handling timeouts",
	"Advanced patterns",
	"Multiple hooks of the same type",
	"Conditional logic in scripts",
	"Structured logging",
	"Integration with external systems",
	"Example use cases",
	"Compliance audit trail",
	"Cost tracking",
	"Code quality enforcement",
	"Notification system",
	"Further reading",
}

// realCopilotCliAgentsLandmarks is a snapshot of the headings extracted from
// Copilot CLI's custom-agents-configuration doc
// (.capmon-cache/copilot-cli/agents.0/extracted.json) as of 2026-04-17.
// agents.1 (create-custom-agents-for-cli.md) carries only generic landmarks
// ("Introduction", "Further reading") plus Liquid-template names — agents.0
// has all the heading-level capability evidence.
var realCopilotCliAgentsLandmarks = []string{
	"YAML frontmatter properties",
	"Tools",
	"Tool aliases",
	"Tool names for \"out-of-the-box\" MCP servers",
	"MCP server configuration details",
	"MCP server type",
	"MCP server environment variables and secrets",
	"Processing of agents",
	"Versioning",
	"Tools processing",
	"MCP server configurations",
	"Further reading",
}

// TestRecognizeCopilotCli_RealAgentsLandmarks proves agents recognition emits
// 3 canonical agents keys at "inferred" confidence: definition_format,
// tool_restrictions, per_agent_mcp. The other 4 keys (invocation_patterns,
// agent_scopes, model_selection, subagent_spawning) must NOT be emitted —
// no heading-level evidence in the agents configuration doc.
//
// Test merges skills + rules + hooks + agents fixtures to mirror real-world
// cache merging — the agents recognizer must distinguish its capabilities
// from the others via the required-anchor uniqueness gate.
func TestRecognizeCopilotCli_RealAgentsLandmarks(t *testing.T) {
	merged := append([]string{}, realCopilotCliSkillsLandmarks...)
	merged = append(merged, realCopilotCliRulesLandmarks...)
	merged = append(merged, realCopilotCliHooksLandmarks...)
	merged = append(merged, realCopilotCliAgentsLandmarks...)
	result := capmon.RecognizeWithContext("copilot-cli", capmon.RecognitionContext{
		Provider:  "copilot-cli",
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
	agentsInferred := []string{
		"definition_format",
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

// TestRecognizeCopilotCli_AgentsAnchorsMissing proves the required-anchor
// guard suppresses agents emission when "Tool aliases" is absent — preventing
// agents patterns from firing on contexts that contain only the parent
// "YAML frontmatter properties" landmark (which could appear in unrelated
// frontmatter docs).
func TestRecognizeCopilotCli_AgentsAnchorsMissing(t *testing.T) {
	mutated := []string{}
	for _, lm := range realCopilotCliAgentsLandmarks {
		if lm == "Tool aliases" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("copilot-cli", capmon.RecognitionContext{
		Provider:  "copilot-cli",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["agents.supported"]; has {
		t.Error("agents.supported should NOT be present when 'Tool aliases' anchor is missing")
	}
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

// TestRecognizeCopilotCli_RealHooksLandmarks proves hooks recognition on the
// merged skills+rules+hooks landmarks. Copilot CLI documents 2 of the 9
// canonical hooks keys at the heading level (handler_types, json_io_protocol);
// the other 7 are not surfaced as headings and must NOT be emitted.
func TestRecognizeCopilotCli_RealHooksLandmarks(t *testing.T) {
	merged := append([]string{}, realCopilotCliSkillsLandmarks...)
	merged = append(merged, realCopilotCliRulesLandmarks...)
	merged = append(merged, realCopilotCliHooksLandmarks...)
	result := capmon.RecognizeWithContext("copilot-cli", capmon.RecognitionContext{
		Provider:  "copilot-cli",
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
	for _, absent := range []string{
		"hooks.capabilities.matcher_patterns.supported",
		"hooks.capabilities.decision_control.supported",
		"hooks.capabilities.async_execution.supported",
		"hooks.capabilities.hook_scopes.supported",
		"hooks.capabilities.context_injection.supported",
		"hooks.capabilities.permission_control.supported",
		"hooks.capabilities.input_modification.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present for copilot-cli (no heading evidence)", absent)
		}
	}
}

// realCopilotCliMcpLandmarks is a snapshot of the headings from Copilot CLI's
// MCP doc (.capmon-cache/copilot-cli/mcp.0/extracted.json — add-mcp-servers.md
// procedural walkthrough). Update this fixture when the upstream doc evolves.
var realCopilotCliMcpLandmarks = []string{
	"Adding an MCP server",
	"Using the /mcp add command",
	"Editing the configuration file",
	"Managing MCP servers",
	"Using MCP servers",
	"Further reading",
}

// TestRecognizeCopilotCli_RealMcpLandmarks proves MCP recognition emits only
// the top-level mcp.supported=true signal (empty-Capability pattern) without
// any per-key emission. Per docs/provider-formats/copilot-cli.yaml, all 8
// canonical MCP keys are individually curated (1 supported confirmed via CLI
// flags + 7 unsupported); the recognizer adds no per-key signal that the
// curator has not already captured. The empty-Capability pattern confirms
// "this provider documents an MCP surface" without contradicting curator
// judgments on individual capabilities.
func TestRecognizeCopilotCli_RealMcpLandmarks(t *testing.T) {
	merged := append([]string{}, realCopilotCliSkillsLandmarks...)
	merged = append(merged, realCopilotCliRulesLandmarks...)
	merged = append(merged, realCopilotCliHooksLandmarks...)
	merged = append(merged, realCopilotCliAgentsLandmarks...)
	merged = append(merged, realCopilotCliMcpLandmarks...)
	result := capmon.RecognizeWithContext("copilot-cli", capmon.RecognitionContext{
		Provider:  "copilot-cli",
		Format:    "markdown",
		Landmarks: merged,
	})

	if result.Status != capmon.StatusRecognized {
		t.Fatalf("status = %q, want %q (missing=%v)", result.Status, capmon.StatusRecognized, result.MissingAnchors)
	}
	caps := result.Capabilities
	if caps["mcp.supported"] != "true" {
		t.Errorf("mcp.supported = %q, want %q", caps["mcp.supported"], "true")
	}
	for k := range caps {
		if len(k) > len("mcp.capabilities.") && k[:len("mcp.capabilities.")] == "mcp.capabilities." {
			t.Errorf("mcp.capabilities.* should NOT be emitted (curator owns per-key signals): %s = %q", k, caps[k])
		}
	}
}

// TestRecognizeCopilotCli_McpAnchorsMissing proves the required-anchor guard
// suppresses MCP emission when "Adding an MCP server" is absent — preventing
// the pattern from firing on agents.0 (which has "MCP server configurations"
// in its landmarks for per-agent MCP scoping).
func TestRecognizeCopilotCli_McpAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realCopilotCliMcpLandmarks))
	for _, lm := range realCopilotCliMcpLandmarks {
		if lm == "Adding an MCP server" {
			continue
		}
		mutated = append(mutated, lm)
	}
	merged := append([]string{}, realCopilotCliSkillsLandmarks...)
	merged = append(merged, realCopilotCliAgentsLandmarks...)
	merged = append(merged, mutated...)
	result := capmon.RecognizeWithContext("copilot-cli", capmon.RecognitionContext{
		Provider:  "copilot-cli",
		Format:    "markdown",
		Landmarks: merged,
	})
	if _, has := result.Capabilities["mcp.supported"]; has {
		t.Error("mcp.supported should NOT be present when 'Adding an MCP server' anchor is missing (agents.0 'MCP server configurations' landmark must not trigger MCP recognition)")
	}
}

// realCopilotCliCommandsLandmarks is a snapshot of the headings extracted from
// Copilot CLI's CLI command reference (.capmon-cache/copilot-cli/commands.0/
// extracted.json) as of 2026-04-28. The source URL switched from three plugin
// docs (which described plugin packaging, not slash commands) to
// raw.githubusercontent.com/.../cli-command-reference.md — the unified
// reference for the interactive interface (~50 built-in slash commands,
// command-line flags, environment variables, hooks/MCP/skills/agents
// subsections).
//
// The fixture intentionally omits the page's hooks/MCP/skills/agents
// subsections — those are documented separately in their own caches and
// would invite cross-content-type false positives if blanket-included here.
// The recognizer's required-anchor uniqueness gate ("Slash commands in the
// interactive interface" + "Command-line commands") protects against drift
// in either direction.
var realCopilotCliCommandsLandmarks = []string{
	"Command-line commands",
	"copilot login options",
	"Using copilot completion",
	"Usage examples",
	"Global shortcuts in the interactive interface",
	"Timeline shortcuts in the interactive interface",
	"Navigation shortcuts in the interactive interface",
	"Slash commands in the interactive interface",
	"Command-line options",
	"Tool availability values",
	"Shell tools",
	"File operation tools",
	"Agent and task delegation tools",
	"Other tools",
	"Tool permission patterns",
	"Environment variables",
	"Configuration file settings",
}

// TestRecognizeCopilotCli_RealCommandsLandmarks proves commands recognition
// fires from the unified CLI command reference and emits builtin_commands at
// "inferred" confidence (heading-evidence tier — the curator owns "confirmed"
// at the format YAML layer). argument_substitution must NOT be emitted: the
// reference documents how built-in slash commands accept literal positional
// arguments (e.g. /add-dir PATH, /init suppress) but does not document a
// user-authored custom-command mechanism with template-substitution syntax
// (no $ARGUMENTS, $1/$2, or {{args}} interpolation).
//
// Test merges all six other content-type fixtures to mirror real-world cache
// merging — the commands recognizer must distinguish its capabilities from
// skills, rules, hooks, agents, and mcp via the required-anchor uniqueness
// gate.
func TestRecognizeCopilotCli_RealCommandsLandmarks(t *testing.T) {
	merged := append([]string{}, realCopilotCliSkillsLandmarks...)
	merged = append(merged, realCopilotCliNonSkillsLandmarks...)
	merged = append(merged, realCopilotCliRulesLandmarks...)
	merged = append(merged, realCopilotCliHooksLandmarks...)
	merged = append(merged, realCopilotCliAgentsLandmarks...)
	merged = append(merged, realCopilotCliMcpLandmarks...)
	merged = append(merged, realCopilotCliCommandsLandmarks...)
	result := capmon.RecognizeWithContext("copilot-cli", capmon.RecognitionContext{
		Provider:  "copilot-cli",
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
	if caps["commands.capabilities.builtin_commands.supported"] != "true" {
		t.Error("commands.capabilities.builtin_commands.supported missing")
	}
	if got := caps["commands.capabilities.builtin_commands.confidence"]; got != "inferred" {
		t.Errorf("commands.builtin_commands.confidence = %q, want inferred", got)
	}
	if _, has := caps["commands.capabilities.argument_substitution.supported"]; has {
		t.Error("commands.capabilities.argument_substitution.supported should NOT be present (Copilot CLI does not document a user-authored custom-command authoring mechanism with template-substitution syntax)")
	}
}

// TestRecognizeCopilotCli_CommandsAnchorsMissing proves the required-anchor
// guard suppresses commands emission when "Slash commands in the interactive
// interface" is absent — preventing patterns from firing on contexts that
// only mention "Command-line commands" without the slash-command taxonomy
// heading.
func TestRecognizeCopilotCli_CommandsAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realCopilotCliCommandsLandmarks))
	for _, lm := range realCopilotCliCommandsLandmarks {
		if lm == "Slash commands in the interactive interface" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("copilot-cli", capmon.RecognitionContext{
		Provider:  "copilot-cli",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["commands.supported"]; has {
		t.Error("commands.supported should NOT be present when 'Slash commands in the interactive interface' anchor is missing")
	}
}
