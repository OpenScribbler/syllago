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
