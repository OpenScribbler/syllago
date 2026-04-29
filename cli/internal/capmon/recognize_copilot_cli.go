package capmon

func init() {
	RegisterRecognizer("copilot-cli", RecognizerKindDoc, recognizeCopilotCli)
}

// copilotCliLandmarkOptions returns the landmark patterns for Copilot CLI's
// skills doc. Anchors derived from the merged landmarks of skills.0 and
// skills.1 (.capmon-cache/copilot-cli/). The skills doc surface is intentionally
// minimal — Copilot CLI documents that skills exist and how to use them, but
// does not expose granular format-level headings the way claude-code does.
//
// As a result, this recognizer primarily proves "skills.supported = true" via
// the required-anchor guard and emits one inferred capability for the
// CLI-management feature that IS documented.
func copilotCliLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "About agent skills", CaseInsensitive: true},
		{Kind: "substring", Value: "Using agent skills", CaseInsensitive: true},
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
			pattern("cli_management", "Skills commands in the CLI", "skill management commands documented under 'Skills commands in the CLI' heading"),
			// A bare anchor-only pattern (no Capability) ensures skills.supported
			// is emitted even when no capability-specific anchor matches.
			{
				Required: required,
				Matchers: required,
			},
		},
	}
}

// copilotCliRulesLandmarkOptions returns the landmark patterns for Copilot CLI's
// rules (custom instructions) doc. Anchors derived from
// .capmon-cache/copilot-cli/rules.0/extracted.json (add-custom-instructions.md).
//
// Required anchors are unique to the rules doc:
//   - "Repository-wide custom instructions"
//   - "Path-specific custom instructions"
//
// Per the seeder spec, copilot-cli has the most comprehensive cross-provider
// compatibility surface in the cache: AGENTS.md, CLAUDE.md, and GEMINI.md are
// all read at the repository root. All three are emitted as nested sub-keys
// gated on the "Agent instructions" heading where the foreign-format support
// is documented.
func copilotCliRulesLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Repository-wide custom instructions", CaseInsensitive: true},
		{Kind: "substring", Value: "Path-specific custom instructions", CaseInsensitive: true},
	}
	return RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode.always_on", "Repository-wide custom instructions",
			".github/copilot-instructions.md applies to all requests in repository scope (documented under 'Repository-wide custom instructions')", required),
		RulesLandmarkPattern("activation_mode.frontmatter_globs", "Path-specific custom instructions",
			"NAME.instructions.md uses 'applyTo' frontmatter glob to scope to file patterns (documented under 'Path-specific custom instructions')", required),
		RulesLandmarkPattern("cross_provider_recognition.agents_md", "Agent instructions",
			"AGENTS.md at repository root, cwd, or COPILOT_CUSTOM_INSTRUCTIONS_DIRS (documented under 'Agent instructions' section)", required),
		RulesLandmarkPattern("cross_provider_recognition.claude_md", "Agent instructions",
			"CLAUDE.md at repository root recognized alongside AGENTS.md (documented under 'Agent instructions' section)", required),
		RulesLandmarkPattern("cross_provider_recognition.gemini_md", "Agent instructions",
			"GEMINI.md at repository root recognized alongside AGENTS.md (documented under 'Agent instructions' section)", required),
		RulesLandmarkPattern("hierarchical_loading", "Local instructions",
			"four-tier scope: Repository-wide + Path-specific + Agent (AGENTS.md) + Local ($HOME/.copilot/copilot-instructions.md) — files merge across tiers (documented under 'Repository-wide' / 'Path-specific' / 'Agent instructions' / 'Local instructions')", required),
	)
}

// copilotCliHooksLandmarkOptions returns the landmark patterns for Copilot CLI's
// hooks-configuration doc. Anchors derived from
// .capmon-cache/copilot-cli/hooks.0/extracted.json
// (raw.githubusercontent.com/.../hooks-configuration.md).
//
// Required anchors are unique to the hooks-configuration doc — "Hook types"
// and "Reading input" do not appear in the rules, skills, or about-copilot-cli
// docs.
//
// Copilot CLI documents 2 of the 9 canonical hooks keys at the heading level
// (handler_types via "Hook types", json_io_protocol via "Reading input" /
// "Outputting JSON"). The other 7 are not documented as headings and are
// intentionally not mapped.
func copilotCliHooksLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Hook types", CaseInsensitive: true},
		{Kind: "substring", Value: "Reading input", CaseInsensitive: true},
	}
	return HooksLandmarkOptions(
		HooksLandmarkPattern("handler_types", "Hook types",
			"hook handler types documented under 'Hook types' heading (session, prompt, tool, error)", required),
		HooksLandmarkPattern("json_io_protocol", "Outputting JSON",
			"JSON I/O protocol documented under 'Reading input' / 'Outputting JSON' headings", required),
	)
}

// copilotCliAgentsLandmarkOptions returns the landmark patterns for Copilot
// CLI's "custom agents configuration" doc. Anchors derived from
// .capmon-cache/copilot-cli/agents.0/extracted.json
// (raw.githubusercontent.com/.../custom-agents-configuration.md).
//
// Maps 3 of 7 canonical agents keys at heading-level evidence:
//   - definition_format → "YAML frontmatter properties" (custom agents are
//     markdown files with YAML frontmatter declaring name/description/tools/
//     mcpServers).
//   - tool_restrictions → "Tool aliases" (per-agent tool allowlist via the
//     tools field, plus tool aliases for renaming).
//   - per_agent_mcp → "MCP server configurations" (per-agent mcpServers
//     section in the YAML frontmatter scopes which MCP servers each agent
//     can access).
//
// The other 4 keys are intentionally unmapped:
//   - invocation_patterns: agents.1 (how-to doc) describes invocation as
//     `/agent <name>` body text, not as a heading. Skip.
//   - agent_scopes: scope locations live in body text of agents.1, not as
//     headings. Skip.
//   - model_selection: no Model heading; the YAML frontmatter does not
//     document a model field per the configuration spec.
//   - subagent_spawning: no chain/spawn/delegate heading; no multi-agent
//     coordination documented.
//
// Required anchors are unique to agents.0:
//   - "YAML frontmatter properties" — H2, agents-specific
//   - "Tool aliases"                — H3 under Tools, agents-specific
//
// Neither appears in copilot-cli's skills, rules, hooks, or mcp landmarks.
// agents.1 landmarks ("Introduction", "Further reading") are too generic to
// drive recognition and are deliberately ignored — agents.0 carries all the
// heading-level capability evidence.
func copilotCliAgentsLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "YAML frontmatter properties", CaseInsensitive: true},
		{Kind: "substring", Value: "Tool aliases", CaseInsensitive: true},
	}
	return AgentsLandmarkOptions(
		AgentsLandmarkPattern("definition_format", "YAML frontmatter properties",
			"markdown files with YAML frontmatter (name, description, tools, mcpServers fields) documented under 'YAML frontmatter properties' heading", required),
		AgentsLandmarkPattern("tool_restrictions", "Tool aliases",
			"per-agent tool allowlist via tools field, with tool aliases for renaming, documented under 'Tools' / 'Tool aliases' / 'Tools processing' headings", required),
		AgentsLandmarkPattern("per_agent_mcp", "MCP server configurations",
			"per-agent mcpServers section in YAML frontmatter scopes which MCP servers each agent can access, documented under 'MCP server configuration details' / 'MCP server configurations' headings", required),
	)
}

// copilotCliMcpLandmarkOptions returns the landmark patterns for Copilot
// CLI's MCP doc. Anchors derived from
// .capmon-cache/copilot-cli/mcp.0/extracted.json (add-mcp-servers.md).
//
// Per docs/provider-formats/copilot-cli.yaml (lines 313-345), the curator
// has individually mapped all 8 canonical MCP keys: tool_filtering is
// supported confirmed via the --allow-tool/--deny-tool CLI flag mechanism,
// and the other 7 keys (transport_types, oauth_support, env_var_expansion,
// auto_approve, marketplace, resource_referencing, enterprise_management)
// are curated as unsupported. The mcp.0 cache contains heading-level
// landmarks ("Adding an MCP server", "Using the /mcp add command",
// "Editing the configuration file", "Managing MCP servers", "Using MCP
// servers"), but these are procedural section headings — none aligns
// cleanly with a canonical-key semantic the curator has not already
// adjudicated.
//
// The mcp.1 cache (concepts/context/mcp.md) has product-overview headings
// ("Remote access", "Toolset customization", "About the GitHub MCP
// Registry") that could superficially anchor canonical keys, but the
// curator has explicitly marked transport_types/marketplace as unsupported
// despite this evidence — the headings describe corporate-network access
// scenarios, built-in tool toggling, and GitHub's server-side registry
// rather than the canonical-key semantics (per-server transport choice,
// per-server tool allowlist, in-CLI marketplace browsing). Emitting
// per-key signals from those headings would contradict curator judgment.
//
// This recognizer therefore emits ONLY mcp.supported=true via an
// empty-Capability pattern — confirming Copilot CLI documents an MCP
// surface without overriding curator-extracted per-key claims. The
// curator's per-key flags in the format YAML stay authoritative.
//
// Required anchors are unique to mcp.0:
//   - "Adding an MCP server"      — H1, MCP-specific
//   - "Using the /mcp add command" — H2, MCP-specific
//
// Verified absent from copilot-cli's skills.{0,1}, rules.0, hooks.0,
// agents.{0,1}, and commands.{0,1,2} caches. Note that agents.0 contains
// "MCP server configurations" (per-agent MCP scoping), but neither of the
// required anchors above appears there — so cross-content-type landmark
// merging cannot trigger a false positive against agents.0.
func copilotCliMcpLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Adding an MCP server", CaseInsensitive: true},
		{Kind: "substring", Value: "Using the /mcp add command", CaseInsensitive: true},
	}
	return McpLandmarkOptions(LandmarkPattern{
		Capability: "",
		Required:   required,
		Matchers:   []StringMatcher{{Kind: "substring", Value: "MCP server", CaseInsensitive: true}},
	})
}

// copilotCliCommandsLandmarkOptions returns the landmark patterns for Copilot
// CLI's CLI command reference. Anchors derived from
// .capmon-cache/copilot-cli/commands.0/extracted.json
// (https://raw.githubusercontent.com/github/docs/main/content/copilot/reference/copilot-cli-reference/cli-command-reference.md
// — the unified reference for the interactive interface, ~50 built-in slash
// commands plus command-line flags, environment variables, and configuration).
//
// The source URL switched 2026-04-28 from three plugin pages
// (about-cli-plugins.md, cli-plugin-reference.md, plugins-creating.md) — which
// described Copilot CLI's plugin packaging surface, not slash commands — to
// the cli-command-reference.md page that explicitly documents Copilot CLI's
// built-in slash-command surface. The plugin pages remain referenced from the
// format YAML's provider_extensions block (the plugin system is a real but
// non-canonical extension surface that bundles agents/skills/hooks/MCP).
//
// Maps 1 of 2 canonical commands keys at heading-level evidence:
//   - builtin_commands → "Slash commands in the interactive interface" + the
//     "Command-line commands" H2; together these document ~50 built-in slash
//     commands (/help, /mcp, /init, /clear, /agent, /delegate, /review,
//     /add-dir, /fleet, /research, /plugin) plus the top-level `copilot`
//     subcommands (login, completion).
//
// argument_substitution is intentionally NOT mapped. Copilot CLI's built-in
// slash commands accept literal positional arguments (e.g. /add-dir PATH,
// /init suppress) but the reference does not document a user-authored
// custom-command authoring mechanism with template-substitution syntax — no
// $ARGUMENTS, no $1/$2 positional substitution, no {{args}} interpolation.
// The format YAML curator marks argument_substitution unsupported on this
// basis.
//
// Required anchors are unique to commands.0:
//   - "Slash commands in the interactive interface" — H2; the slash-command
//     taxonomy heading, absent from skills.{0,1}, rules.{0,1,2}, hooks.{0,1,2},
//     agents.{0,1}, and mcp.{0,1} caches.
//   - "Command-line commands" — H2; the top-level `copilot` subcommand
//     reference, also unique to commands.0.
//
// Together these gate against false positives both ways: a future drift in
// the cli-command-reference page that drops either heading will block
// commands recognition, and a doc that mentions "slash commands" or
// "command-line commands" only in passing won't carry both anchors.
func copilotCliCommandsLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Slash commands in the interactive interface", CaseInsensitive: true},
		{Kind: "substring", Value: "Command-line commands", CaseInsensitive: true},
	}
	return CommandsLandmarkOptions(
		CommandsLandmarkPattern("builtin_commands", "Slash commands in the interactive interface",
			"~50 built-in slash commands documented under 'Slash commands in the interactive interface' (e.g. /help, /mcp, /init, /clear, /agent, /delegate, /review, /add-dir); 'Command-line commands' covers top-level `copilot` subcommands (login, completion)", required),
	)
}

// recognizeCopilotCli recognizes skills + rules + hooks + agents + mcp +
// commands capabilities for the Copilot CLI provider. Source for all six
// content types is markdown documentation; recognition uses landmark
// (heading) matching. Static facts merge in at "confirmed" confidence after
// a successful skills landmark match. MCP recognition emits only
// mcp.supported=true (empty-Capability pattern) since the curator owns
// all per-key signals.
func recognizeCopilotCli(ctx RecognitionContext) RecognitionResult {
	skillsResult := recognizeLandmarks(ctx, copilotCliLandmarkOptions())
	if len(skillsResult.Capabilities) > 0 {
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "project_scope", "skill directory under .github/skills/<name>/, .claude/skills/<name>/, or .agents/skills/<name>/ in project repository", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "global_scope", "skill directory under ~/.copilot/skills/<name>/, ~/.claude/skills/<name>/, or ~/.agents/skills/<name>/", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	}

	rulesResult := recognizeLandmarks(ctx, copilotCliRulesLandmarkOptions())
	hooksResult := recognizeLandmarks(ctx, copilotCliHooksLandmarkOptions())
	agentsResult := recognizeLandmarks(ctx, copilotCliAgentsLandmarkOptions())
	mcpResult := recognizeLandmarks(ctx, copilotCliMcpLandmarkOptions())
	commandsResult := recognizeLandmarks(ctx, copilotCliCommandsLandmarkOptions())

	return mergeRecognitionResults(skillsResult, rulesResult, hooksResult, agentsResult, mcpResult, commandsResult)
}
