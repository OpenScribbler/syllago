package capmon_test

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// realGeminiCliRulesLandmarks is a snapshot of headings extracted from
// .capmon-cache/gemini-cli/rules.0/extracted.json (gemini-md.md) as of
// 2026-04-16. Update when the doc evolves.
var realGeminiCliRulesLandmarks = []string{
	"Provide context with GEMINI.md files",
	"Understand the context hierarchy",
	"Example GEMINI.md file",
	"Manage context with the /memory command",
	"Modularize context with imports",
	"Customize the context file name",
	"Next steps",
}

// realGeminiCliHooksLandmarks is a snapshot of the merged headings from
// .capmon-cache/gemini-cli/hooks.{2,3}/extracted.json (docs/hooks/index.md
// and docs/hooks/reference.md) as of 2026-04-16. Update when the docs evolve.
var realGeminiCliHooksLandmarks = []string{
	// hooks.2 (index.md)
	"Gemini CLI hooks",
	"What are hooks?",
	"Getting started",
	"Core concepts",
	"Hook events",
	"Global mechanics",
	"Strict JSON requirements (The \"Golden Rule\")",
	"Exit codes",
	"Matchers",
	"Configuration",
	"Configuration schema",
	"Hook configuration fields",
	"Environment variables",
	"Security and risks",
	"Managing hooks",
	// hooks.3 (reference.md)
	"Hooks reference",
	"Global hook mechanics",
	"Hook definition",
	"Hook configuration",
	"Base input schema",
	"Common output fields",
	"Tool hooks",
	"Matchers and tool names",
	"BeforeTool",
	"AfterTool",
	"Agent hooks",
	"BeforeAgent",
	"AfterAgent",
	"Model hooks",
	"BeforeModel",
	"BeforeToolSelection",
	"AfterModel",
	"Lifecycle & system hooks",
	"SessionStart",
	"SessionEnd",
	"Notification",
	"PreCompress",
	"Stable Model API",
}

func TestRecognizeGeminiCli_RealRulesLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider:  "gemini-cli",
		Format:    "markdown",
		Landmarks: realGeminiCliRulesLandmarks,
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
		"file_imports",
		"auto_memory",
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
	// Per seeder spec, gemini-cli does NOT document AGENTS.md or other
	// foreign-format recognition.
	if _, has := caps["rules.capabilities.cross_provider_recognition.agents_md.supported"]; has {
		t.Error("cross_provider_recognition.agents_md should NOT be present for gemini-cli")
	}
}

func TestRecognizeGeminiCli_AnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realGeminiCliRulesLandmarks))
	for _, lm := range realGeminiCliRulesLandmarks {
		if lm == "Understand the context hierarchy" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider:  "gemini-cli",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
}

func TestRecognizeGeminiCli_NoLandmarks(t *testing.T) {
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider: "gemini-cli",
		Format:   "markdown",
	})
	if result.Status != capmon.StatusAnchorsMissing {
		t.Errorf("status = %q, want %q", result.Status, capmon.StatusAnchorsMissing)
	}
}

// TestRecognizeGeminiCli_RealHooksLandmarks proves hooks recognition on the
// merged rules+hooks landmarks. Per the curated format YAML, only 3 of the 9
// canonical hooks keys are supported in gemini-cli: matcher_patterns,
// decision_control, json_io_protocol. handler_types is intentionally absent
// because gemini-cli only supports shell handlers.
func TestRecognizeGeminiCli_RealHooksLandmarks(t *testing.T) {
	merged := append([]string{}, realGeminiCliRulesLandmarks...)
	merged = append(merged, realGeminiCliHooksLandmarks...)
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider:  "gemini-cli",
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
		"matcher_patterns",
		"decision_control",
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
		"hooks.capabilities.handler_types.supported",
		"hooks.capabilities.async_execution.supported",
		"hooks.capabilities.hook_scopes.supported",
		"hooks.capabilities.context_injection.supported",
		"hooks.capabilities.permission_control.supported",
		"hooks.capabilities.input_modification.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present for gemini-cli (no heading evidence or curated as unsupported)", absent)
		}
	}
}

// realGeminiCliMcpLandmarks is a snapshot of the headings from gemini-cli's
// MCP docs (.capmon-cache/gemini-cli/mcp.{1,2}/extracted.json — mcp-server.md
// and mcp-setup.md) as of 2026-04-16. mcp.0 (settings.schema.json) is JSON
// Schema and contributes Fields not Landmarks; the heading evidence comes
// from the two markdown sources merged.
//
// Gemini CLI's mcp-server.md is the deepest single MCP doc in the cache (89
// landmarks) — it backs 6 of 8 canonical MCP keys including OAuth, transport
// types, env var expansion, tool filtering, auto-approve (trust-based bypass),
// and resource referencing.
var realGeminiCliMcpLandmarks = []string{
	// mcp.1 — mcp-server.md (89 headings)
	"MCP servers with Gemini CLI",
	"What is an MCP server?",
	"Core integration architecture",
	"Discovery Layer (mcp-client.ts)",
	"Execution layer (mcp-tool.ts)",
	"Transport mechanisms",
	"Working with MCP resources",
	"Discovery and listing",
	"Referencing resources in a conversation",
	"How to set up your MCP server",
	"Configure the MCP server in settings.json",
	"Global MCP settings (mcp)",
	"Server-specific configuration (mcpServers)",
	"Configuration structure",
	"Configuration properties",
	"Required (one of the following)",
	"Optional",
	"Environment variable expansion",
	"Security and environment sanitization",
	"Automatic redaction",
	"Explicit overrides",
	"OAuth support for remote MCP servers",
	"Automatic OAuth discovery",
	"Authentication flow",
	"Browser redirect requirements",
	"Managing OAuth authentication",
	"OAuth configuration properties",
	"Token management",
	"Authentication provider type",
	"Google credentials",
	"Service account impersonation",
	"Setup instructions",
	"Example configurations",
	"Python MCP server (stdio)",
	"Node.js MCP server (stdio)",
	"Docker-based MCP server",
	"HTTP-based MCP server",
	"HTTP-based MCP Server with custom headers",
	"MCP server with tool filtering",
	"SSE MCP server with SA impersonation",
	"Discovery process deep dive",
	"1. Server iteration and connection",
	"2. Tool discovery",
	"3. Tool naming and namespaces",
	"4. Schema processing",
	"5. Connection management",
	"Tool execution flow",
	"1. Tool invocation",
	"2. Confirmation process",
	"Trust-based bypass",
	"Dynamic allow-listing",
	"User choice handling",
	"3. Execution",
	"4. Response handling",
	"How to interact with your MCP server",
	"Using the /mcp command",
	"Example /mcp output",
	"Tool usage",
	"Status monitoring and troubleshooting",
	"Connection states",
	"Overriding extension configurations",
	"Server status (MCPServerStatus)",
	"Discovery state (MCPDiscoveryState)",
	"Common issues and solutions",
	"Server won't connect",
	"No tools discovered",
	"Tools not executing",
	"Sandbox compatibility",
	"Debugging tips",
	"Important notes",
	"Security considerations",
	"Performance and resource management",
	"Schema compatibility",
	"Returning rich content from tools",
	"How it works",
	"Example: Returning text and an image",
	"MCP prompts as slash commands",
	"Defining prompts on the server",
	"Invoking prompts",
	"Managing MCP servers with gemini mcp",
	"Adding a server (gemini mcp add)",
	"Adding an stdio server",
	"Adding an HTTP server",
	"Adding an SSE server",
	"Listing servers (gemini mcp list)",
	"Troubleshooting and Diagnostics",
	"Removing a server (gemini mcp remove)",
	"Enabling/disabling a server (gemini mcp enable, gemini mcp disable)",
	"Instructions",
	// mcp.2 — mcp-setup.md (10 headings)
	"Set up an MCP server",
	"Prerequisites",
	"How to prepare your credentials",
	"How to configure Gemini CLI",
	"How to verify the connection",
	"How to use the new tools",
	"Scenario: Listing pull requests",
	"Scenario: Creating an issue",
	"Troubleshooting",
	"Next steps",
}

// TestRecognizeGeminiCli_RealMcpLandmarks proves MCP recognition emits 6
// canonical MCP keys at "inferred" confidence: transport_types, oauth_support,
// env_var_expansion, tool_filtering, auto_approve, resource_referencing.
// marketplace and enterprise_management must NOT be emitted (no heading
// evidence — gemini-cli has no in-IDE marketplace and no documented org-level
// MCP management).
//
// Test merges all four content type fixtures to verify cross-content-type
// robustness and exercise the required-anchor uniqueness gate.
func TestRecognizeGeminiCli_RealMcpLandmarks(t *testing.T) {
	merged := append([]string{}, realGeminiCliRulesLandmarks...)
	merged = append(merged, realGeminiCliHooksLandmarks...)
	merged = append(merged, realGeminiCliMcpLandmarks...)
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider:  "gemini-cli",
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
		"oauth_support",
		"env_var_expansion",
		"tool_filtering",
		"auto_approve",
		"resource_referencing",
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
		"mcp.capabilities.marketplace.supported",
		"mcp.capabilities.enterprise_management.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be present for gemini-cli (no heading evidence)", absent)
		}
	}
}

// TestRecognizeGeminiCli_McpAnchorsMissing proves the required-anchor guard
// suppresses MCP emission when "MCP servers with Gemini CLI" is absent —
// preventing MCP patterns from firing on contexts that include
// "Transport mechanisms" or "OAuth support for remote MCP servers" landmarks
// but lack the MCP doc anchor.
func TestRecognizeGeminiCli_McpAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realGeminiCliMcpLandmarks))
	for _, lm := range realGeminiCliMcpLandmarks {
		if lm == "MCP servers with Gemini CLI" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider:  "gemini-cli",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["mcp.supported"]; has {
		t.Error("mcp.supported should NOT be present when 'MCP servers with Gemini CLI' anchor is missing")
	}
}

// realGeminiCliCommandsLandmarks is a snapshot of the headings extracted from
// Gemini CLI's custom-commands doc (.capmon-cache/gemini-cli/commands.0/
// extracted.json — docs/cli/custom-commands.md) as of 2026-04-17. The
// "Handling arguments" subsection enumerates four substitution mechanisms
// ({{args}}, default arg handling, !{...} shell injection, @{...} file
// injection) — strong heading-level evidence for argument_substitution.
var realGeminiCliCommandsLandmarks = []string{
	"Custom commands",
	"File locations and precedence",
	"Naming and namespacing",
	"TOML file format (v1)",
	"Required fields",
	"Optional fields",
	"Handling arguments",
	"1. Context-aware injection with {{args}}",
	"2. Default argument handling",
	"3. Executing shell commands with !{...}",
	"4. Injecting file content with @{...}",
	`Example: A "Pure Function" refactoring command`,
}

// TestRecognizeGeminiCli_RealCommandsLandmarks proves commands recognition
// fires on the merged rules+hooks+mcp+commands fixture, emits
// argument_substitution at "inferred" confidence, and does NOT emit
// builtin_commands (built-in slash commands are documented in the CLI binary
// commands page — a different cache source not pulled in by this fixture).
func TestRecognizeGeminiCli_RealCommandsLandmarks(t *testing.T) {
	merged := append([]string{}, realGeminiCliRulesLandmarks...)
	merged = append(merged, realGeminiCliHooksLandmarks...)
	merged = append(merged, realGeminiCliMcpLandmarks...)
	merged = append(merged, realGeminiCliCommandsLandmarks...)
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider:  "gemini-cli",
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
		t.Error("commands.capabilities.builtin_commands.supported should NOT be present for gemini-cli (built-ins live in CLI binary docs page, not custom-commands.md)")
	}
}

// TestRecognizeGeminiCli_CommandsAnchorsMissing proves the required-anchor
// guard suppresses commands emission when the unique "Handling arguments"
// anchor is absent — preventing commands patterns from firing on contexts
// where only the generic "Custom commands" or "Required fields" headings
// appear.
func TestRecognizeGeminiCli_CommandsAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realGeminiCliCommandsLandmarks))
	for _, lm := range realGeminiCliCommandsLandmarks {
		if lm == "Handling arguments" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider:  "gemini-cli",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["commands.supported"]; has {
		t.Error("commands.supported should NOT be present when 'Handling arguments' anchor is missing")
	}
}

// realGeminiCliAgentsLandmarks is a snapshot of the merged headings from
// .capmon-cache/gemini-cli/agents.{0,1}/extracted.json (docs/core/subagents.md
// and docs/core/remote-agents.md) as of 2026-04-28. agents.0 is the primary
// source with 41 headings covering automatic delegation, @-mention invocation,
// agent definition files, tool wildcards, recursion protection, and
// /agents management. agents.1 adds remote-subagent specific headings (Agent
// Card, auth schemes, Agent2Agent transport).
var realGeminiCliAgentsLandmarks = []string{
	// agents.0 — subagents.md (41 headings)
	"Subagents",
	"What are subagents?",
	"How to use subagents",
	"Automatic delegation",
	"Forcing a subagent (@ syntax)",
	"Built-in subagents",
	"Codebase Investigator",
	"CLI Help Agent",
	"Generalist Agent",
	"Browser Agent (experimental)",
	"Creating custom subagents",
	"Agent definition files",
	"File format",
	"Tool wildcards",
	"Isolation and recursion protection",
	"Subagent tool isolation",
	"Configuring isolated tools and servers",
	"Subagent-specific policies",
	"Managing subagents",
	"Interactive management (/agents)",
	"Persistent configuration (settings.json)",
	"Optimizing your subagent",
	"Remote subagents (Agent2Agent)",
	"Extension subagents",
	"Disabling subagents",
	// agents.1 — remote-agents.md (29 headings)
	"Remote Subagents",
	"Defining remote subagents",
	"Single-subagent example",
	"Multi-subagent example",
	"Inline Agent Card JSON",
}

// TestRecognizeGeminiCli_RealAgentsLandmarks proves agents recognition fires
// on the merged rules+hooks+mcp+commands+agents fixture. Per the curated
// format doc (docs/provider-formats/gemini-cli.yaml), 4 of the 7 canonical
// agents keys have direct heading-level evidence in the docs cache:
// definition_format, invocation_patterns, tool_restrictions, per_agent_mcp.
// The remaining keys (agent_scopes, model_selection, subagent_spawning) are
// confirmed in the format doc but lack their own heading anchors —
// agent_scopes and model_selection are described inside config tables, and
// subagent_spawning is a *negative* feature (recursion explicitly prevented).
// Recognizer stays silent on those rather than emit false-positive heading
// claims.
func TestRecognizeGeminiCli_RealAgentsLandmarks(t *testing.T) {
	merged := append([]string{}, realGeminiCliRulesLandmarks...)
	merged = append(merged, realGeminiCliHooksLandmarks...)
	merged = append(merged, realGeminiCliMcpLandmarks...)
	merged = append(merged, realGeminiCliCommandsLandmarks...)
	merged = append(merged, realGeminiCliAgentsLandmarks...)
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider:  "gemini-cli",
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
		"invocation_patterns",
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
	// agent_scopes, model_selection have no heading-level evidence (config
	// table fields). subagent_spawning is a negative feature (recursion
	// prevented) and recognizers do not emit "supported: true" for absences.
	for _, absent := range []string{
		"agents.capabilities.agent_scopes.supported",
		"agents.capabilities.model_selection.supported",
		"agents.capabilities.subagent_spawning.supported",
	} {
		if _, has := caps[absent]; has {
			t.Errorf("%s should NOT be emitted by landmark recognizer (no heading evidence; curated only in format doc)", absent)
		}
	}
}

// TestRecognizeGeminiCli_AgentsAnchorsMissing proves the required-anchor
// guard suppresses agents emission when the unique "Creating custom
// subagents" anchor is absent — preventing agents patterns from firing on
// contexts that include "Tool wildcards" or "Automatic delegation" alone but
// lack the agents-doc anchor.
func TestRecognizeGeminiCli_AgentsAnchorsMissing(t *testing.T) {
	mutated := make([]string, 0, len(realGeminiCliAgentsLandmarks))
	for _, lm := range realGeminiCliAgentsLandmarks {
		if lm == "Creating custom subagents" {
			continue
		}
		mutated = append(mutated, lm)
	}
	result := capmon.RecognizeWithContext("gemini-cli", capmon.RecognitionContext{
		Provider:  "gemini-cli",
		Format:    "markdown",
		Landmarks: mutated,
	})
	if _, has := result.Capabilities["agents.supported"]; has {
		t.Error("agents.supported should NOT be present when 'Creating custom subagents' anchor is missing")
	}
}
