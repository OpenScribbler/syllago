package capmon

func init() {
	RegisterRecognizer("factory-droid", RecognizerKindDoc, recognizeFactoryDroid)
}

// factoryDroidLandmarkOptions returns the landmark patterns for Factory Droid's
// skills doc. Anchors derived from
// .capmon-cache/factory-droid/skills.0/extracted.json
// (https://docs.factory.ai/cli/configuration/skills — Mintlify Next.js SPA
// fetched via chromedp).
//
// Mintlify landmarks have a leading zero-width space prefix (e.g.
// "​Skill file format"). Substring matchers handle this transparently — the
// matcher value "Skill file format" matches the landmark "​Skill file format".
//
// Required anchors are unique to the skills doc:
//   - "Skill file format" — H2; not present in any other factory-droid doc
//   - "Where skills live"  — H2; not present in any other factory-droid doc
func factoryDroidLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Skill file format", CaseInsensitive: true},
		{Kind: "substring", Value: "Where skills live", CaseInsensitive: true},
	}
	pattern := func(cap, anchor, mechanism string) LandmarkPattern {
		return LandmarkPattern{
			Capability: cap,
			Required:   required,
			Matchers:   []StringMatcher{{Kind: "substring", Value: anchor, CaseInsensitive: true}},
			Mechanism:  mechanism,
		}
	}
	return LandmarkOptions{
		ContentType: "skills",
		Patterns: []LandmarkPattern{
			pattern("frontmatter", "Frontmatter reference", "documented under 'Frontmatter reference' heading"),
			pattern("creation_workflow", "Quickstart", "documented under 'Quickstart' heading"),
			pattern("directory_structure", "Where skills live", "documented under 'Where skills live' heading"),
			pattern("invocation", "Control who invokes a skill", "documented under 'Control who invokes a skill' heading"),
			// Bare anchor-only pattern guarantees skills.supported when only the
			// required anchors are present (no specific-capability anchor).
			{
				Required: required,
				Matchers: required,
			},
		},
	}
}

// factoryDroidHooksLandmarkOptions returns the landmark patterns for Factory
// Droid's hooks reference doc. Anchors derived from
// .capmon-cache/factory-droid/hooks.1/extracted.json
// (https://docs.factory.ai/reference/hooks-reference — Mintlify SPA fetched
// via chromedp).
//
// Per the curated format YAML (docs/provider-formats/factory-droid.yaml), only
// decision_control is supported among the 9 canonical hooks keys — Factory
// Droid hooks signal block (non-zero) or allow (zero) via exit codes (the
// hook_exit_code_behavior provider extension). The other 8 canonical keys are
// curated as unsupported and intentionally NOT mapped here.
//
// Required anchors are unique to the hooks reference doc — they distinguish
// hooks evidence from skills/rules/mcp/agents/commands evidence in the merged
// landmarks context:
//   - "Hooks reference" — H1 of hooks.1; absent everywhere else
//   - "Hook Events"      — H2 of hooks.1; substring also matches hooks.0's
//     "Hook Events Overview", but both belong to hooks evidence so the guard
//     still scopes correctly
func factoryDroidHooksLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Hooks reference", CaseInsensitive: true},
		{Kind: "substring", Value: "Hook Events", CaseInsensitive: true},
	}
	return HooksLandmarkOptions(
		HooksLandmarkPattern("decision_control", "Decision Control",
			"hook_exit_code_behavior: Factory Droid hooks use exit codes to signal block (non-zero) or allow (zero) decisions on the triggering action; documented under per-event 'Decision Control' headings", required),
		// Bare anchor-only pattern guarantees hooks.supported when only the
		// required anchors are present (no specific-capability anchor).
		LandmarkPattern{
			Required: required,
			Matchers: required,
		},
	)
}

// factoryDroidAgentsLandmarkOptions returns the landmark patterns for Factory
// Droid's "Custom Droids (Subagents)" doc. Anchors derived from
// .capmon-cache/factory-droid/agents.0/extracted.json
// (https://docs.factory.ai/cli/configuration/custom-droids — Mintlify SPA).
//
// Maps 2 of 7 canonical agents keys at heading-level evidence:
//   - definition_format → "Configuration" heading; .md files with system
//     prompt, model preference, and tooling policy.
//   - tool_restrictions → "Tool categories → concrete tools" heading; named
//     tool categories (filesystem, shell, search, browser, web_fetch) rather
//     than per-tool allowlists.
//
// Five keys are intentionally unmapped despite the curator (factory-droid.yaml)
// marking them supported. The recognizer requires heading-level evidence; the
// curator may mark capabilities supported from broader knowledge of the source:
//   - invocation_patterns: documented in body text under "Using custom droids
//     effectively" but not as discrete invocation-mode headings.
//   - agent_scopes: only example titles surface "(project scope)" and
//     "(personal scope)" — these are example names, not scope-section
//     headings, so the evidence is too weak for nested emission.
//   - model_selection: no Model heading; per-droid model preference lives in
//     YAML body of example configs, not as a section heading.
//   - per_agent_mcp: no heading evidence; curator marks unsupported.
//   - subagent_spawning: parent heading "Custom Droids (Subagents)" implies
//     subagent semantics, and "Importing Claude Code subagents" describes
//     interop, but no chain/spawn/delegate heading exists.
//
// Required anchors are unique to agents.0:
//   - "Custom Droids"          — H1, agents-specific
//   - "Tool categories"        — H3 ("Tool categories → concrete tools"),
//     agents-specific (no other factory-droid doc uses this phrase).
func factoryDroidAgentsLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Custom Droids", CaseInsensitive: true},
		{Kind: "substring", Value: "Tool categories", CaseInsensitive: true},
	}
	return AgentsLandmarkOptions(
		AgentsLandmarkPattern("definition_format", "Configuration",
			"single-file .md droids with system prompt, model preference, and tooling policy documented under 'Configuration' heading", required),
		AgentsLandmarkPattern("tool_restrictions", "Tool categories",
			"categorical tool policy using named categories (filesystem, shell, search, browser, web_fetch) documented under 'Tool categories → concrete tools' heading", required),
	)
}

// MCP recognition is intentionally NOT wired for factory-droid.
//
// The cached MCP source (.capmon-cache/factory-droid/mcp.0/extracted.json)
// is the docs.factory.ai/llms.txt index — a flat list of links whose
// landmarks array contains only 4 generic entries: "Factory Documentation",
// "Docs", "OpenAPI Specs", "Optional". None of these can anchor canonical
// MCP keys via substring matching.
//
// Per docs/provider-formats/factory-droid.yaml, all 8 canonical MCP keys
// are curated as unsupported (inferred). The curator's notes confirm the
// gap: "Full MCP config details require the /cli/configuration/mcp.md page
// (not fetched — source here is the llms.txt index only)." The provider
// extensions list two MCP-related features (mcp_manager_ui,
// factory_as_mcp_server) but those are non-portable extensions, not
// canonical MCP-protocol capabilities.
//
// Recognizer silence is the right move — emitting "supported" for any
// canonical key based on llms.txt nav landmarks would be a false positive.
// MCP recognition can be wired once the Mintlify SPA mcp.md page is in
// the cache and yields heading-level evidence.

// factoryDroidCommandsLandmarkOptions returns the landmark patterns for
// Factory Droid's "Custom Slash Commands" doc. Anchors derived from
// .capmon-cache/factory-droid/commands.0/extracted.json
// (https://docs.factory.ai/cli/configuration/custom-slash-commands —
// Mintlify SPA fetched via chromedp).
//
// Mintlify landmarks have a leading zero-width space prefix; substring
// matchers handle this transparently.
//
// Maps 1 of 2 canonical commands keys at heading-level evidence:
//   - argument_substitution → "5 · Usage patterns" / "2 · Markdown commands"
//     headings + body content showing $ARGUMENTS in Markdown commands and
//     positional args in Executable commands. Curator confirms support.
//
// builtin_commands is intentionally NOT mapped — per the curated YAML
// (docs/provider-formats/factory-droid.yaml), Factory Droid has no built-in
// slash commands; commands are entirely user-defined Markdown templates or
// executables. The doc structure confirms: every section is about CUSTOM
// commands (discovery/naming, markdown, executables, managing, usage,
// examples).
//
// Required anchors are unique to commands.0:
//   - "Custom Slash Commands" — H1 page heading; absent from all other
//     factory-droid caches.
//   - "Markdown commands"     — H2 ("2 · Markdown commands"); the doc's
//     unique commands taxonomy phrase.
func factoryDroidCommandsLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Custom Slash Commands", CaseInsensitive: true},
		{Kind: "substring", Value: "Markdown commands", CaseInsensitive: true},
	}
	return CommandsLandmarkOptions(
		CommandsLandmarkPattern("argument_substitution", "Usage patterns",
			"$ARGUMENTS substitution in Markdown commands plus positional shell args ($1, $2, ...) in Executable commands documented under '2 · Markdown commands' / '3 · Executable commands' / '5 · Usage patterns' headings", required),
	)
}

// recognizeFactoryDroid recognizes skills + hooks + agents + commands
// capabilities for the Factory Droid provider. Source for all four is
// markdown documentation (Mintlify SPA); recognition uses landmark matching.
// Static facts merge in at "confirmed" confidence after a successful skills
// landmark match. MCP recognition is intentionally absent — see the comment
// block immediately above this function for rationale.
func recognizeFactoryDroid(ctx RecognitionContext) RecognitionResult {
	skillsResult := recognizeLandmarks(ctx, factoryDroidLandmarkOptions())
	if len(skillsResult.Capabilities) > 0 {
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "project_scope", "<repo>/.factory/skills/<skill-name>/SKILL.md or .agent/skills/<skill-name>/SKILL.md", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "global_scope", "~/.factory/skills/<skill-name>/SKILL.md", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "canonical_filename", "SKILL.md (skill.mdx also accepted)", "confirmed"))
	}

	hooksResult := recognizeLandmarks(ctx, factoryDroidHooksLandmarkOptions())
	agentsResult := recognizeLandmarks(ctx, factoryDroidAgentsLandmarkOptions())
	commandsResult := recognizeLandmarks(ctx, factoryDroidCommandsLandmarkOptions())

	return mergeRecognitionResults(skillsResult, hooksResult, agentsResult, commandsResult)
}
