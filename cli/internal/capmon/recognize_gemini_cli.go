package capmon

func init() {
	RegisterRecognizer("gemini-cli", RecognizerKindGoStruct, recognizeGeminiCli)
}

// geminiCliRulesLandmarkOptions returns the landmark patterns for Gemini CLI's
// rules documentation (GEMINI.md files). Anchors derived from
// .capmon-cache/gemini-cli/rules.0/extracted.json (gemini-md.md).
//
// Required anchors are unique to the GEMINI.md doc:
//   - "Provide context with GEMINI.md files" — the page H1
//   - "Understand the context hierarchy" — the H2 that defines the load order
//
// Per the seeder spec, gemini-cli does NOT document explicit AGENTS.md
// recognition — cross_provider_recognition is intentionally absent.
func geminiCliRulesLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Provide context with GEMINI.md files", CaseInsensitive: true},
		{Kind: "substring", Value: "Understand the context hierarchy", CaseInsensitive: true},
	}
	return RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode.always_on", "Understand the context hierarchy",
			"GEMINI.md files load automatically within scope (global, workspace, or JIT) — no frontmatter activation modes documented", required),
		RulesLandmarkPattern("file_imports", "Modularize context with imports",
			"GEMINI.md supports importing other markdown files (documented under 'Modularize context with imports' heading)", required),
		RulesLandmarkPattern("auto_memory", "Manage context with the /memory command",
			"/memory show / add / reload slash commands manage hierarchical memory (documented under 'Manage context with the /memory command')", required),
		RulesLandmarkPattern("hierarchical_loading", "Understand the context hierarchy",
			"three-tier hierarchy: global ~/.gemini/GEMINI.md + workspace + JIT ancestor scan up to trusted root (documented under 'Understand the context hierarchy')", required),
	)
}

// geminiCliHooksLandmarkOptions returns the landmark patterns for Gemini CLI's
// hooks documentation. Anchors derived from the merged headings of
// .capmon-cache/gemini-cli/hooks.{2,3}/extracted.json (docs/hooks/index.md and
// docs/hooks/reference.md).
//
// Required anchors are unique to the hooks docs:
//   - "Hook events" — only appears in hooks.2 (the index doc's H2 listing all
//     11 lifecycle events)
//   - "Configuration schema" — appears in both hooks.2 and hooks.3, never in
//     skills/rules/mcp/commands docs
//
// Per the curated format YAML (docs/provider-formats/gemini-cli.yaml), only
// 3 of the 9 canonical hooks keys are supported in gemini-cli:
// matcher_patterns, decision_control, json_io_protocol. handler_types is
// explicitly false because gemini-cli only supports shell handlers (not
// http/llm-prompt/agent types) — "Hook events" describes lifecycle event
// types, not handler types, and is intentionally not mapped to handler_types.
func geminiCliHooksLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Hook events", CaseInsensitive: true},
		{Kind: "substring", Value: "Configuration schema", CaseInsensitive: true},
	}
	return HooksLandmarkOptions(
		HooksLandmarkPattern("matcher_patterns", "Matchers",
			"event/tool matchers documented under 'Matchers' / 'Matchers and tool names' headings", required),
		HooksLandmarkPattern("decision_control", "Exit codes",
			"decision control via exit codes documented under 'Exit codes' heading (non-zero blocks, zero allows)", required),
		HooksLandmarkPattern("json_io_protocol", "Common output fields",
			"JSON I/O protocol documented under 'Strict JSON requirements' / 'Common output fields' headings", required),
	)
}

// geminiCliMcpLandmarkOptions returns the landmark patterns for Gemini CLI's
// MCP documentation. Anchors derived from the merged headings of
// .capmon-cache/gemini-cli/mcp.{1,2}/extracted.json (mcp-server.md and
// mcp-setup.md). mcp.0 is the settings.schema.json — JSON-schema fields are
// extracted into Fields not Landmarks, but its struct names appear in mcp.1's
// "Configuration properties" body text.
//
// Gemini CLI's mcp-server.md is a deeply structured reference doc with 89
// landmarks. 6 of 8 canonical MCP keys map to heading-level evidence:
// transport_types, oauth_support, env_var_expansion, tool_filtering,
// auto_approve, resource_referencing. marketplace and enterprise_management
// are absent — gemini-cli has no in-IDE MCP marketplace and no documented
// org-level MCP management surface.
//
// Required anchors are unique to the MCP docs:
//   - "MCP servers with Gemini CLI" — H1 of mcp.1, MCP-specific
//   - "Set up an MCP server" — H1 of mcp.2, MCP-specific
//
// Neither appears in gemini-cli's skills, rules, or hooks docs.
//
// Per docs/provider-formats/gemini-cli.yaml, 4 of 8 keys are curated as
// supported at "confirmed" confidence (transport_types, env_var_expansion,
// tool_filtering, resource_referencing). The recognizer additionally emits
// oauth_support and auto_approve as "inferred" — both have direct
// heading-level evidence in mcp.1 ("OAuth support for remote MCP servers"
// and "Trust-based bypass" / "Confirmation process") that the curator
// marked as unsupported. Recognizer emissions land in
// docs/provider-capabilities/gemini-cli.yaml independently of the curated
// docs/provider-formats/gemini-cli.yaml.
func geminiCliMcpLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "MCP servers with Gemini CLI", CaseInsensitive: true},
		{Kind: "substring", Value: "Set up an MCP server", CaseInsensitive: true},
	}
	return McpLandmarkOptions(
		McpLandmarkPattern("transport_types", "Transport mechanisms",
			"transport types (stdio, HTTP, SSE) documented under 'Transport mechanisms' heading; per-transport sub-headings 'Adding an stdio server' / 'Adding an HTTP server' / 'Adding an SSE server' under 'gemini mcp add'", required),
		McpLandmarkPattern("oauth_support", "OAuth support for remote MCP servers",
			"OAuth 2.0 with automatic discovery, browser redirect flow, token management, and Google credentials documented under 'OAuth support for remote MCP servers' / 'Automatic OAuth discovery' / 'Authentication flow' headings", required),
		McpLandmarkPattern("env_var_expansion", "Environment variable expansion",
			"environment variable expansion documented under 'Environment variable expansion' heading with 'Security and environment sanitization' / 'Automatic redaction' / 'Explicit overrides' guidance", required),
		McpLandmarkPattern("tool_filtering", "MCP server with tool filtering",
			"per-server tool allow/deny filtering documented under 'MCP server with tool filtering' (Example configurations) and 'Dynamic allow-listing' headings", required),
		McpLandmarkPattern("auto_approve", "Trust-based bypass",
			"auto-approval via trust-based bypass and dynamic allow-listing documented under 'Trust-based bypass' / 'Dynamic allow-listing' headings (under '2. Confirmation process')", required),
		McpLandmarkPattern("resource_referencing", "Working with MCP resources",
			"MCP resources (discovery, listing, in-conversation referencing) documented under 'Working with MCP resources' / 'Discovery and listing' / 'Referencing resources in a conversation' headings", required),
	)
}

// recognizeGeminiCli recognizes skills + rules + hooks + mcp capabilities for
// the Gemini CLI provider. Skills use the GoStruct strategy (Agent Skills
// open standard). Rules, hooks, and MCP are landmark-based against the
// gemini-md.md, hooks/{index,reference}.md, and mcp-server.md /
// mcp-setup.md docs.
func recognizeGeminiCli(ctx RecognitionContext) RecognitionResult {
	skillsCaps := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(skillsCaps) > 0 {
		mergeInto(skillsCaps, capabilityDotPaths("skills", "project_scope", ".gemini/skills/<name>/SKILL.md", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "global_scope", "~/.gemini/skills/<name>/SKILL.md", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	}
	skillsResult := wrapCapabilities(skillsCaps)

	rulesResult := recognizeLandmarks(ctx, geminiCliRulesLandmarkOptions())
	hooksResult := recognizeLandmarks(ctx, geminiCliHooksLandmarkOptions())
	mcpResult := recognizeLandmarks(ctx, geminiCliMcpLandmarkOptions())

	return mergeRecognitionResults(skillsResult, rulesResult, hooksResult, mcpResult)
}
