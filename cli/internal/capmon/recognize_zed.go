package capmon

func init() {
	RegisterRecognizer("zed", RecognizerKindDoc, recognizeZed)
}

// zedRulesLandmarkOptions returns the landmark patterns for Zed's rules
// documentation. Anchors derived from .capmon-cache/zed/rules.1/extracted.json
// (the HTML doc at zed.dev/docs/ai/rules). rules.0 is zed's own .rules
// instance file (their internal Rust coding guidelines) and intentionally NOT
// used as evidence — instance content is not capability vocabulary.
//
// NOTE: The seeder spec drafted cross_provider_recognition as unsupported, but
// the live cache landmarks include AGENTS.md, AGENT.md, CLAUDE.md, GEMINI.md,
// .cursorrules, .windsurfrules, .clinerules, and .github/copilot-instructions.md
// listed under the ".rules files" section. Zed explicitly recognizes all of
// these as fallback rule-file names. The recognizer trusts the live cache
// over the draft spec.
//
// Required anchors are unique to the rules doc:
//   - "Rules Library" (in-app library — distinctive zed feature)
//   - "Migrating from Prompt Library" (zed-specific migration heading)
//
// Per the spec notes, zed's distinctive activation_mode is slash_command
// (Library rules invoked via slash command). Project-root .rules + Library
// Default Rules cover always_on. No frontmatter glob, manual, or
// model_decision modes documented. No file_imports or auto_memory. No
// hierarchical loading documented (project-root only, first-match wins among
// the recognized filenames).
func zedRulesLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Rules Library", CaseInsensitive: true},
		{Kind: "substring", Value: "Migrating from Prompt Library", CaseInsensitive: true},
	}
	return RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode.always_on", ".rules files",
			"project-root .rules file (or first-match fallback name) auto-included in every Agent Panel interaction; Library entries marked as Default Rules also load always_on (documented under '.rules files' / 'Default Rules')", required),
		RulesLandmarkPattern("activation_mode.slash_command", "Slash Commands in Rules",
			"Library rules invoked via slash command to inject the rule into the current agent context (documented under 'Slash Commands in Rules')", required),
		RulesLandmarkPattern("cross_provider_recognition.agents_md", "AGENTS.md",
			"AGENTS.md (and AGENT.md) recognized as fallback rule-file names in project root, first match wins (documented under '.rules files')", required),
		RulesLandmarkPattern("cross_provider_recognition.claude_md", "CLAUDE.md",
			"CLAUDE.md recognized as fallback rule-file name in project root, first match wins (documented under '.rules files')", required),
		RulesLandmarkPattern("cross_provider_recognition.gemini_md", "GEMINI.md",
			"GEMINI.md recognized as fallback rule-file name in project root, first match wins (documented under '.rules files')", required),
		RulesLandmarkPattern("cross_provider_recognition.cursorrules", ".cursorrules",
			".cursorrules recognized as fallback rule-file name in project root, first match wins (documented under '.rules files')", required),
		RulesLandmarkPattern("cross_provider_recognition.windsurfrules", ".windsurfrules",
			".windsurfrules recognized as fallback rule-file name in project root, first match wins (documented under '.rules files')", required),
		RulesLandmarkPattern("cross_provider_recognition.clinerules", ".clinerules",
			".clinerules recognized as fallback rule-file name in project root, first match wins (documented under '.rules files')", required),
	)
}

// zedMcpLandmarkOptions returns the landmark patterns for Zed's MCP
// documentation. Anchors derived from .capmon-cache/zed/mcp.1/extracted.json
// (zed.dev/docs/ai/mcp, HTML). mcp.0 is a Rust source file
// (crates/context_server/src/context_server.rs) yielding only 3 struct
// names — typed evidence not aligned to landmark matching.
//
// Zed's MCP doc maps only 2 of 8 canonical MCP keys at the heading level:
// tool_filtering ("Tool Permissions") and marketplace ("As Extensions" —
// Zed's extension catalog is the in-IDE MCP server marketplace).
//
// The other 6 keys are intentionally unmapped here:
//   - transport_types: "As Custom Servers" / "As Extensions" sub-headings
//     describe install methods, not transport types. The Rust struct
//     ContextServerTransport (mcp.0) hints at transport abstraction but the
//     doc heading evidence is too weak.
//   - oauth_support, env_var_expansion, auto_approve, resource_referencing,
//     enterprise_management: no heading evidence in mcp.1.
//
// Required anchors are unique to the MCP doc:
//   - "Model Context Protocol" — H1, MCP-specific
//   - "Installing MCP Servers"  — H2, MCP-specific
//
// Neither appears in zed's rules, commands, or agents docs.
//
// docs/provider-formats/zed.yaml has no curated MCP section — the only
// curated content type is skills (marked unsupported). Recognizer emissions
// land in docs/provider-capabilities/zed.yaml at "inferred" confidence.
func zedMcpLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Model Context Protocol", CaseInsensitive: true},
		{Kind: "substring", Value: "Installing MCP Servers", CaseInsensitive: true},
	}
	return McpLandmarkOptions(
		McpLandmarkPattern("tool_filtering", "Tool Permissions",
			"per-tool permission control documented under 'Tool Permissions' heading", required),
		McpLandmarkPattern("marketplace", "As Extensions",
			"in-IDE MCP server marketplace via Zed's extension catalog documented under 'As Extensions' (vs 'As Custom Servers') sub-heading of 'Installing MCP Servers'", required),
	)
}

// recognizeZed recognizes rules + mcp capabilities for the Zed provider. Zed
// does not support Agent Skills, so skills emission is intentionally a no-op
// (confirmed-negative signal). Rules and MCP recognition use landmark
// matching from zed's HTML docs at zed.dev/docs/ai/{rules,mcp}.
func recognizeZed(ctx RecognitionContext) RecognitionResult {
	rulesResult := recognizeLandmarks(ctx, zedRulesLandmarkOptions())
	mcpResult := recognizeLandmarks(ctx, zedMcpLandmarkOptions())
	return mergeRecognitionResults(rulesResult, mcpResult)
}
