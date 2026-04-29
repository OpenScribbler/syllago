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

// cursorSkillsLandmarkOptions returns the landmark patterns for Cursor's
// Skills documentation. Anchors derived from
// .capmon-cache/cursor/skills.0/extracted.json (cursor.com/docs/skills, HTML).
//
// Cursor implements the Agent Skills open standard. Two canonical skills keys
// have direct heading-level evidence in the cache:
//   - canonical_filename: "SKILL.md file format" H2 — the heading text itself
//     literally names the canonical filename.
//   - disable_model_invocation: "Disabling automatic invocation" H2 — the
//     section that documents the disable-model-invocation frontmatter toggle.
//
// Other canonical skills keys (display_name, description, license,
// compatibility, metadata_map, project_scope, global_scope) live in body text
// or are inferred from the Agent Skills standard. The format YAML
// (docs/provider-formats/cursor.yaml lines 97–177) already covers these at
// confirmed confidence reading the same source plus the standard's
// vocabulary; landmark substring matching cannot anchor on them via heading
// text alone.
//
// Required anchors are unique to the skills doc:
//   - "SKILL.md file format" — H2 unique to skills doc; no other cursor doc
//     names SKILL.md in a heading.
//   - "Disabling automatic invocation" — H2 unique to skills doc.
//
// Neither appears in cursor's rules, hooks, mcp, or agents docs.
func cursorSkillsLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "SKILL.md file format", CaseInsensitive: true},
		{Kind: "substring", Value: "Disabling automatic invocation", CaseInsensitive: true},
	}
	return SkillsLandmarkOptions(
		SkillsLandmarkPattern("canonical_filename", "SKILL.md file format",
			"SKILL.md is the canonical skill filename, documented under 'SKILL.md file format' heading", required),
		SkillsLandmarkPattern("disable_model_invocation", "Disabling automatic invocation",
			"disable-model-invocation frontmatter toggle documented under 'Disabling automatic invocation' heading", required),
	)
}

// cursorHooksLandmarkOptions returns the landmark patterns for Cursor's Hooks
// documentation. Anchors derived from
// .capmon-cache/cursor/hooks.0/extracted.json (cursor.com/docs/hooks, HTML).
//
// Three canonical hooks keys have heading-level evidence:
//   - matcher_patterns: "Matcher Configuration" H2 — direct heading for the
//     matcher mechanism documented in format YAML lines 192–195.
//   - hook_scopes: "Project Hooks (Version Control)" H2 — anchors the section
//     introducing project-scope hooks; user-global scope is documented in body
//     prose alongside it (format YAML lines 208–211).
//   - json_io_protocol: "Input (all hooks)" H2 — anchors the input/output
//     schema reference that documents JSON-on-stdin + JSON-on-stdout (format
//     YAML lines 212–215).
//
// Six other canonical hooks keys are intentionally unmapped:
//   - handler_types: NOT emitted — the format-YAML curator marks
//     handler_types: supported: false confirmed (line 188–191: "Cursor hooks
//     execute shell commands only"). The cache landmarks "Command-Based Hooks"
//     and "Prompt-Based Hooks" suggest the curator's claim may be outdated
//     (Cursor appears to have added prompt-based hooks since the YAML was
//     curated), but the recognizer must not contradict a confirmed curator
//     judgment. If the curator's claim is wrong, that is a format-YAML edit,
//     not a recognizer emission.
//   - decision_control: documented as exit-code semantics in body prose; no
//     dedicated heading.
//   - input_modification, async_execution, context_injection,
//     permission_control: format YAML marks these supported: false (inferred
//     or confirmed). No cache evidence to flip them.
//
// Required anchors are unique to the hooks doc:
//   - "Hook Types" — H2 unique to cursor hooks doc.
//   - "preToolUse" — Cursor-specific hook event name; appears in no other
//     cursor doc.
//
// Neither appears in cursor's rules, skills, mcp, or agents docs.
func cursorHooksLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Hook Types", CaseInsensitive: true},
		{Kind: "substring", Value: "preToolUse", CaseInsensitive: true},
	}
	return HooksLandmarkOptions(
		HooksLandmarkPattern("matcher_patterns", "Matcher Configuration",
			"matcher patterns documented under 'Matcher Configuration' heading", required),
		HooksLandmarkPattern("hook_scopes", "Project Hooks (Version Control)",
			"project + user-global hook scopes documented under 'Project Hooks (Version Control)' / 'Configuration' headings", required),
		HooksLandmarkPattern("json_io_protocol", "Input (all hooks)",
			"JSON I/O protocol documented under 'Common schema' / 'Input (all hooks)' headings (event data on stdin, structured decision JSON on stdout)", required),
	)
}

// cursorAgentsLandmarkOptions returns the landmark patterns for Cursor's
// Subagents documentation. Anchors derived from
// .capmon-cache/cursor/agents.0/extracted.json (cursor.com/docs/subagents,
// HTML).
//
// Verified live: https://cursor.com/docs/subagents on 2026-04-28. The earlier
// format-YAML source URL (cursor.com/docs/agent/overview) pointed at the
// built-in Agent feature page rather than the file-based custom subagents
// surface; the URL was updated and the cache refetched.
//
// Maps 6 of 7 canonical agents keys at heading-level evidence:
//   - definition_format → "File format" heading documents the .md + YAML
//     frontmatter shape (name, description, model, readonly, is_background).
//   - tool_restrictions → "Configuration fields" heading documents readonly
//     flag and other YAML properties.
//   - invocation_patterns.automatic_delegation → "Automatic delegation"
//     heading documents proactive delegation by Agent based on description,
//     complexity, and tools.
//   - invocation_patterns.explicit → "Explicit invocation" heading documents
//     /name syntax and natural-language mentions.
//   - agent_scopes.project + agent_scopes.user → "File locations" heading
//     documents .cursor/agents/ project + ~/.cursor/agents/ user-global
//     scopes.
//   - model_selection → "Model configuration" heading documents the model
//     frontmatter field with inherit/fast/<model-id> values plus admin
//     overrides.
//   - subagent_spawning → "Can subagents launch other subagents?" FAQ
//     heading anchors the answer: since Cursor 2.5, subagents can launch
//     child subagents.
//
// per_agent_mcp is intentionally unmapped — the live docs say "Subagents
// inherit all tools from the parent, including MCP tools from configured
// servers", which means MCP is shared across subagents rather than scoped
// per-agent. Format YAML correctly marks per_agent_mcp supported: false.
//
// Required anchors are unique to the subagents doc:
//   - "Custom subagents" — H2 unique to subagents page; no other cursor doc
//     uses this heading.
//   - "File format" — H2 unique to subagents page in the cursor docs set
//     (rules uses "Rule anatomy", skills uses "SKILL.md file format", hooks
//     uses "Hook Types").
func cursorAgentsLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Custom subagents", CaseInsensitive: true},
		{Kind: "substring", Value: "File format", CaseInsensitive: true},
	}
	return AgentsLandmarkOptions(
		AgentsLandmarkPattern("definition_format", "File format",
			"Markdown files with YAML frontmatter (name, description, model, readonly, is_background) followed by a prompt body, documented under 'File format' heading", required),
		AgentsLandmarkPattern("tool_restrictions", "Configuration fields",
			"readonly frontmatter flag restricts the subagent to read-only tools (documented in 'Configuration fields')", required),
		AgentsLandmarkPattern("invocation_patterns.automatic_delegation", "Automatic delegation",
			"Cursor's Agent proactively delegates tasks to subagents based on description, complexity, and available tools (documented under 'Automatic delegation' heading)", required),
		AgentsLandmarkPattern("invocation_patterns.explicit", "Explicit invocation",
			"/<name> syntax or natural-language mentions invoke a specific subagent (documented under 'Explicit invocation' heading)", required),
		AgentsLandmarkPattern("agent_scopes.project", "File locations",
			".cursor/agents/ project scope, taking precedence over user scope (documented under 'File locations' heading)", required),
		AgentsLandmarkPattern("agent_scopes.user", "File locations",
			"~/.cursor/agents/ user-global scope (documented under 'File locations' heading)", required),
		AgentsLandmarkPattern("model_selection", "Model configuration",
			"model frontmatter field accepts 'inherit', 'fast', or a specific model id; admin policies and Max Mode requirements may override (documented under 'Model configuration' heading)", required),
		AgentsLandmarkPattern("subagent_spawning", "Can subagents launch other subagents?",
			"Since Cursor 2.5, subagents can launch child subagents to create a tree of coordinated work; nested launches require Task tool access and can be restricted by hooks or tool policies (FAQ heading 'Can subagents launch other subagents?')", required),
	)
}

// recognizeCursor recognizes rules + skills + mcp + hooks + agents
// capabilities for the Cursor provider. All are landmark-based against
// cursor.com/docs.
//
// Skills uses the canonical SkillsLandmarkOptions/SkillsLandmarkPattern
// constructors with canonical-key validation. After a successful skills
// recognition, confirmed scope facts are merged in as static evidence (the
// .cursor/skills/ + .agents/skills/ project paths and the ~/.agents/skills/
// global path are documented in body prose, not headings).
//
// Agents uses the canonical AgentsLandmarkOptions constructors with
// heading-level anchors from cursor.com/docs/subagents, mapping 6 of 7
// canonical agents keys (per_agent_mcp is correctly absent — subagents
// inherit MCP from parent rather than scoping per-agent).
func recognizeCursor(ctx RecognitionContext) RecognitionResult {
	rulesResult := recognizeLandmarks(ctx, cursorRulesLandmarkOptions())
	skillsResult := recognizeLandmarks(ctx, cursorSkillsLandmarkOptions())
	if len(skillsResult.Capabilities) > 0 {
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "project_scope", "Skills stored in .cursor/skills/<name>/SKILL.md (cursor-native) or .agents/skills/<name>/SKILL.md (cross-provider Agent Skills convention)", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "global_scope", "Skills stored in ~/.agents/skills/<name>/SKILL.md per the cross-provider Agent Skills convention", "confirmed"))
	}
	mcpResult := recognizeLandmarks(ctx, cursorMcpLandmarkOptions())
	hooksResult := recognizeLandmarks(ctx, cursorHooksLandmarkOptions())
	agentsResult := recognizeLandmarks(ctx, cursorAgentsLandmarkOptions())
	return mergeRecognitionResults(rulesResult, skillsResult, mcpResult, hooksResult, agentsResult)
}
