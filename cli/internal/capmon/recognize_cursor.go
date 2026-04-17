package capmon

func init() {
	RegisterRecognizer("cursor", RecognizerKindDoc, recognizeCursor)
}

// cursorRulesLandmarkOptions returns the landmark patterns for Cursor's rules
// documentation. Anchors derived from .capmon-cache/cursor/rules.0/extracted.json
// (https://cursor.com/docs/rules, HTML).
//
// Required anchors are unique to the rules doc:
//   - "How rules work" — top-level rules-page heading
//   - "Rule anatomy" — frontmatter/schema heading
//
// Note: the seeder spec (drafted earlier) marked file_imports and
// cross_provider_recognition as unsupported, but the live cache landmarks now
// include "Importing Rules" and "AGENTS.md" / "Nested AGENTS.md support". The
// docs evolved post-spec; we trust the cache as the live source of truth.
//
// Activation modes (Always / Auto Attached / Apply Manually / Agent Requested)
// are documented as Rule Type values in a frontmatter table. Like windsurf,
// the table cells are not extracted as standalone landmarks — all four
// sub-keys gate on the parent "Activation and enforcement" heading.
func cursorRulesLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "How rules work", CaseInsensitive: true},
		{Kind: "substring", Value: "Rule anatomy", CaseInsensitive: true},
	}
	return RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode.always_on", "Activation and enforcement",
			"'Always' Rule Type — rule included in every prompt (documented under 'Activation and enforcement')", required),
		RulesLandmarkPattern("activation_mode.frontmatter_globs", "Activation and enforcement",
			"'Auto Attached' Rule Type — frontmatter glob patterns activate when matching files in context (documented under 'Activation and enforcement')", required),
		RulesLandmarkPattern("activation_mode.manual", "Activation and enforcement",
			"'Apply Manually' Rule Type — activated via @-mention in chat (documented under 'Activation and enforcement')", required),
		RulesLandmarkPattern("activation_mode.model_decision", "Activation and enforcement",
			"'Agent Requested' Rule Type — agent decides based on rule description (documented under 'Activation and enforcement')", required),
		RulesLandmarkPattern("file_imports", "Importing Rules",
			"rules can reference / import other rules (documented under 'Importing Rules' heading)", required),
		RulesLandmarkPattern("cross_provider_recognition.agents_md", "AGENTS.md",
			"AGENTS.md fallback with nested directory support (documented under 'AGENTS.md' / 'Nested AGENTS.md support' headings)", required),
		RulesLandmarkPattern("hierarchical_loading", "Team Rules",
			"three-tier scope: Project rules (.cursor/rules) + Team Rules (org-wide) + User Rules (documented under 'Project rules' / 'Team Rules' / 'User Rules')", required),
	)
}

// cursorMcpLandmarkOptions returns the landmark patterns for Cursor's MCP
// documentation. Anchors derived from .capmon-cache/cursor/mcp.0/extracted.json
// (https://cursor.com/docs/context/mcp, HTML).
//
// Cursor's MCP doc maps 6 of 8 canonical MCP keys at the heading level.
// Two are intentionally absent:
//   - resource_referencing: documented in the "Protocol and extension support"
//     capability table as a row labeled "Resources", but table cells are
//     extracted into Fields, not Landmarks. No heading-level evidence exists
//     so the recognizer cannot anchor on it via landmark substring matching.
//   - enterprise_management: cursor's enterprise MCP policy lives in
//     admin-console docs not in this scrape.
//
// Required anchors are unique to the MCP doc:
//   - "What is MCP?" — H1/H2 of MCP discovery section
//   - "Installing MCP servers" — H2 unique to MCP installation flow
//
// Neither appears in cursor's rules, skills, or hooks docs.
//
// Per docs/provider-formats/cursor.yaml, no curated MCP section exists yet
// (only skills is curated, marked unsupported). Recognizer emissions land in
// docs/provider-capabilities/cursor.yaml at "inferred" confidence.
func cursorMcpLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "What is MCP?", CaseInsensitive: true},
		{Kind: "substring", Value: "Installing MCP servers", CaseInsensitive: true},
	}
	return McpLandmarkOptions(
		McpLandmarkPattern("transport_types", "STDIO server configuration",
			"stdio transport documented under 'STDIO server configuration' heading; SSE and Streamable HTTP variants covered in body text and configuration tables", required),
		McpLandmarkPattern("oauth_support", "Static OAuth for remote servers",
			"OAuth 2.0 (with Client ID/Secret + scopes) documented under 'Static OAuth for remote servers' / 'Authentication' headings", required),
		McpLandmarkPattern("env_var_expansion", "Config interpolation",
			"${env:NAME}, ${userHome}, ${workspaceFolder}, ${pathSeparator} variable interpolation documented under 'Config interpolation' / 'Combining with config interpolation' headings", required),
		McpLandmarkPattern("tool_filtering", "Tool approval",
			"per-server enable/disable toggle and per-tool approval documented under 'Tool approval' / 'Using MCP in chat' headings", required),
		McpLandmarkPattern("auto_approve", "Auto-run",
			"auto-approval / auto-run mode for trusted tools documented under 'Auto-run' heading", required),
		McpLandmarkPattern("marketplace", "One-click installation",
			"one-click MCP server installation from a curated catalog documented under 'One-click installation' heading", required),
	)
}

// recognizeCursor recognizes rules + mcp capabilities for the Cursor provider.
// Cursor does not support Agent Skills (FormatDoc status: unsupported), so no
// skills emission. Rules and MCP are landmark-based against cursor.com/docs.
func recognizeCursor(ctx RecognitionContext) RecognitionResult {
	rulesResult := recognizeLandmarks(ctx, cursorRulesLandmarkOptions())
	mcpResult := recognizeLandmarks(ctx, cursorMcpLandmarkOptions())
	return mergeRecognitionResults(rulesResult, mcpResult)
}
