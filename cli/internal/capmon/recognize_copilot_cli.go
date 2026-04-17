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

// MCP recognition is intentionally NOT wired for copilot-cli.
//
// Copilot CLI's MCP evidence (.capmon-cache/copilot-cli/mcp.0 + mcp.1) is
// procedural body text rather than heading-level structure. The mcp.0 doc
// (add-mcp-servers.md) walks through the interactive `/mcp add` form via
// numbered list items: "Next to Server Type, select... Local or STDIO...
// HTTP or SSE..." and "Next to Tools, specify which tools...". Transport
// types and tool filtering are mentioned, but as inline list options inside
// a single procedure — not as discrete H1/H2 anchors that the substring
// matcher can use to distinguish capabilities.
//
// Per docs/provider-formats/copilot-cli.yaml, only 1 of 8 canonical MCP keys
// (tool_filtering, mechanism: --allow-tool/--deny-tool CLI flags) is curated
// as supported, at "confirmed" confidence. The other 7 are curated as
// unsupported. The doc-source landmark approach would either:
//   - Be redundant (recognizer "inferred" loses to curator "confirmed" for
//     tool_filtering, and the doc evidence is the config-form Tools field —
//     a different mechanism than the curated --allow-tool flags).
//   - Contradict the curator (e.g. emitting transport_types as inferred-true
//     against curator's explicit unsupported assertion based on procedural
//     body text rather than heading evidence).
//
// Recognizer silence preserves the curator's higher-confidence data and
// avoids surfacing false-positive heading inferences from list-item body
// text. mcp.1 (concepts/context/mcp.md) is the GitHub-wide MCP discovery
// page covering chat/IDE/cloud-agent variants — its landmarks describe
// extension and integration concepts, not consumer-side capability
// vocabulary specific to the CLI.

// Commands recognition is intentionally NOT wired for copilot-cli.
//
// All three cached commands sources describe the PLUGIN system, not slash
// commands:
//   - commands.0 (about-copilot-cli-plugins.md) — overview of "what is a
//     plugin", landmarks "What is a plugin?" / "What plugins contain" / "Why
//     use plugins?". No slash-command vocabulary.
//   - commands.1 (copilot-cli-plugin-specification.md) — plugin.json schema
//     reference, landmarks "Plugin specification for install command" /
//     "marketplace.json" / "Component path fields". The "CLI commands"
//     landmark refers to plugin-management CLI commands (install/uninstall),
//     not user-invokable slash commands.
//   - commands.2 (creating-copilot-cli-plugin.md) — plugin-author tutorial,
//     landmarks "Plugin structure" / "Creating a plugin" / "Distributing
//     your plugin". Again no slash-command surface.
//
// Per docs/provider-formats/copilot-cli.yaml, copilot-cli has NO user-
// invokable slash commands — the /-prefix surface is reserved for built-in
// CLI features (/help, /reset, /quit) that are not documented as a custom-
// command authoring API. The plugin system is a parallel extension surface
// covered by the existing skills + agents recognizers. Both canonical
// commands keys (argument_substitution, builtin_commands) are curated as
// unsupported.
//
// Recognizer silence preserves the curator's "unsupported" assertion.
// Emitting any commands.* key from plugin landmarks would conflate plugins
// (a packaging mechanism) with slash commands (an invocation mechanism) —
// two unrelated capability surfaces.

// recognizeCopilotCli recognizes skills + rules + hooks + agents capabilities
// for the Copilot CLI provider. Source for all four content types is markdown
// documentation; recognition uses landmark (heading) matching. Static facts
// merge in at "confirmed" confidence after a successful skills landmark match.
// MCP and commands recognition are intentionally absent — see the comment
// blocks above for rationale.
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

	return mergeRecognitionResults(skillsResult, rulesResult, hooksResult, agentsResult)
}
