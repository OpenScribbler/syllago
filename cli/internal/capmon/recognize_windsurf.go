package capmon

func init() {
	RegisterRecognizer("windsurf", RecognizerKindGoStruct, recognizeWindsurf)
}

// windsurfRulesLandmarkOptions returns the landmark patterns for Windsurf's
// rules documentation. Anchors derived from
// .capmon-cache/windsurf/rules.0/extracted.json (Memories & Rules doc) and
// .capmon-cache/windsurf/rules.1/extracted.json (AGENTS.md doc).
//
// Required anchors "Rules Discovery" and "Rules Storage Locations" are unique
// to the rules.0 doc — they prevent rules patterns from firing on sources that
// only mention rules in passing (e.g. the AGENTS.md doc alone, or any other
// content type's landmarks merged into the recognition context).
//
// Activation modes: the rules.0 doc has a single "Activation Modes" heading
// followed by a table whose rows name the four trigger values
// (always_on, manual, model_decision, glob). Table cells are not extracted as
// landmarks, so all four sub-keys gate on the same parent heading. This is the
// strongest available signal that windsurf supports the full activation_mode
// vocabulary — the seeder spec singles out windsurf as the most explicit
// among all 14 providers.
func windsurfRulesLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Rules Discovery", CaseInsensitive: true},
		{Kind: "substring", Value: "Rules Storage Locations", CaseInsensitive: true},
	}
	return RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode.always_on", "Activation Modes",
			"always_on trigger value documented in 'Activation Modes' table — full rule content included in system prompt every message", required),
		RulesLandmarkPattern("activation_mode.manual", "Activation Modes",
			"manual trigger value documented in 'Activation Modes' table — activated via @-mention", required),
		RulesLandmarkPattern("activation_mode.model_decision", "Activation Modes",
			"model_decision trigger value documented in 'Activation Modes' table — Cascade decides based on context", required),
		RulesLandmarkPattern("activation_mode.glob", "Activation Modes",
			"glob trigger value documented in 'Activation Modes' table — activated when files matching glob pattern are in context", required),
		RulesLandmarkPattern("cross_provider_recognition.agents_md", "AGENTS.md",
			"AGENTS.md fallback documented in dedicated rules.1 doc with 'Comparison with Rules' section", required),
		RulesLandmarkPattern("auto_memory", "How to Manage Memories",
			"Cascade-managed Memories layer auto-writes context based on conversation (documented under 'Memories & Rules' / 'How to Manage Memories')", required),
		RulesLandmarkPattern("hierarchical_loading", "Rules Storage Locations",
			".windsurf/rules scanned in current workspace, sub-directories, and parent dirs up to git root (documented under 'Rules Discovery' / 'Rules Storage Locations')", required),
	)
}

// windsurfHooksLandmarkOptions returns the landmark patterns for Windsurf's
// "Cascade Hooks" doc. Anchors derived from
// .capmon-cache/windsurf/hooks.0/extracted.json (cascade/hooks.md).
//
// Required anchors are unique to the hooks doc:
//   - "Cascade Hooks" — H1 of the hooks page, not present in rules/skills/mcp
//   - "Hook Events" — H2 listing the 12 lifecycle events, unique to hooks
//
// Per the curated format YAML (docs/provider-formats/windsurf.yaml), 2 of the
// 9 canonical hooks keys are supported: hook_scopes (System/User/Workspace
// three-tier config scopes) and json_io_protocol (JSON event context via
// stdin). Both are mapped here at "inferred" confidence via heading evidence.
//
// Matchers "Workspace-Level" and "Common Input Structure" are themselves
// unique to the hooks doc — verified absent from rules, skills, mcp, and
// commands landmarks.
func windsurfHooksLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Cascade Hooks", CaseInsensitive: true},
		{Kind: "substring", Value: "Hook Events", CaseInsensitive: true},
	}
	return HooksLandmarkOptions(
		HooksLandmarkPattern("hook_scopes", "Workspace-Level",
			"three-tier config scopes documented under 'System-Level' / 'User-Level' / 'Workspace-Level' headings", required),
		HooksLandmarkPattern("json_io_protocol", "Common Input Structure",
			"JSON event context delivered via stdin documented under 'Common Input Structure' heading", required),
	)
}

// windsurfMcpLandmarkOptions returns the landmark patterns for Windsurf's
// MCP documentation. Anchors derived from
// .capmon-cache/windsurf/mcp.0/extracted.json (cascade/mcp.md).
//
// Windsurf's MCP doc is the richest of the 14 providers — 19 landmarks
// covering transport, interpolation, marketplace, enterprise admin controls,
// and registry customization. 5 of 8 canonical MCP keys map to heading-level
// evidence: transport_types, env_var_expansion, tool_filtering, marketplace,
// enterprise_management.
//
// Three keys are intentionally absent:
//   - oauth_support: no OAuth heading; curator confirms windsurf does not
//     document OAuth 2.0 for remote servers.
//   - auto_approve: no auto-approve heading; curator confirms windsurf does
//     not support pre-configured auto-approval.
//   - resource_referencing: no @-mention or resources heading; curator
//     confirms windsurf does not document MCP resource access.
//
// Required anchors are unique to the MCP doc:
//   - "Model Context Protocol (MCP)" — H1, MCP-specific
//   - "Adding a new MCP" — H2, MCP-specific
//
// Neither appears in windsurf's skills, rules, hooks, agents, or commands
// docs.
//
// Per docs/provider-formats/windsurf.yaml, the curator marks 4 keys as
// supported confirmed (transport_types, env_var_expansion, marketplace,
// enterprise_management) and tool_filtering as unsupported. The recognizer
// additionally emits tool_filtering as "inferred" because heading-level
// evidence exists ("Configuring MCP tools" and "MCP Whitelist") — the
// curator interprets these headings as the 100-tool cap and admin-side
// regex matching rather than per-server tool allowlist. The two YAML files
// are independent: provider-capabilities/ tracks recognizer emissions,
// provider-formats/ tracks curator judgments.
func windsurfMcpLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Model Context Protocol (MCP)", CaseInsensitive: true},
		{Kind: "substring", Value: "Adding a new MCP", CaseInsensitive: true},
	}
	return McpLandmarkOptions(
		McpLandmarkPattern("transport_types", "Remote HTTP MCPs",
			"transport types (stdio for local, Streamable HTTP and SSE for remote) documented under 'Remote HTTP MCPs' heading; remote servers use serverUrl/url instead of command/args", required),
		McpLandmarkPattern("env_var_expansion", "Config Interpolation",
			"${env:VAR_NAME} and ${file:/path} interpolation documented under 'Config Interpolation' heading", required),
		McpLandmarkPattern("tool_filtering", "MCP Whitelist",
			"per-server tool filtering documented under 'Configuring MCP tools' (UI tool toggles, 100-tool cap) and 'MCP Whitelist' (admin-side regex matching) headings", required),
		McpLandmarkPattern("marketplace", "MCP Registry",
			"in-IDE MCP marketplace documented under 'MCP Registry' / 'Configuring Custom Registries' headings", required),
		McpLandmarkPattern("enterprise_management", "Admin Controls (Teams & Enterprises)",
			"organization-level MCP admin controls (whitelist, custom registries, regex matching) documented under 'Admin Controls (Teams & Enterprises)' / 'Admin Configuration Guidelines' headings", required),
	)
}

// recognizeWindsurf recognizes skills + rules + hooks + mcp capabilities for
// the Windsurf provider. Skills currently use the GoStruct strategy (Agent
// Skills open standard) but the live windsurf docs cache contains no Skill.*
// typed fields — skills emission depends on future typed-source availability.
// Rules, hooks, and MCP are landmark-based against the rules.{0,1} (Memories
// & Rules, AGENTS.md), hooks.0 (Cascade Hooks), and mcp.0 (cascade/mcp.md)
// docs respectively.
func recognizeWindsurf(ctx RecognitionContext) RecognitionResult {
	skillsCaps := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(skillsCaps) > 0 {
		mergeInto(skillsCaps, capabilityDotPaths("skills", "project_scope", "Skill directory at .windsurf/skills/<skill-name>/, also discovered at .agents/skills/<skill-name>/ and .claude/skills/<skill-name>/", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "global_scope", "Skill directory at ~/.codeium/windsurf/skills/<skill-name>/, also discovered at ~/.agents/skills/<skill-name>/ and ~/.claude/skills/<skill-name>/", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "canonical_filename", "SKILL.md (all scopes share the same convention)", "confirmed"))
	}
	skillsResult := wrapCapabilities(skillsCaps)

	rulesResult := recognizeLandmarks(ctx, windsurfRulesLandmarkOptions())
	hooksResult := recognizeLandmarks(ctx, windsurfHooksLandmarkOptions())
	mcpResult := recognizeLandmarks(ctx, windsurfMcpLandmarkOptions())

	return mergeRecognitionResults(skillsResult, rulesResult, hooksResult, mcpResult)
}
