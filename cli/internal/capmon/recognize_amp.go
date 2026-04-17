package capmon

func init() {
	RegisterRecognizer("amp", RecognizerKindDoc, recognizeAmp)
}

// ampLandmarkOptions returns the landmark patterns for Amp's Agent Skills doc.
// Anchors derived from .capmon-cache/amp/skills.0/extracted.json. The two
// required anchors guard against passing mentions: "Agent Skills" alone could
// appear in unrelated docs, so we additionally require "Skill Format" to
// confirm the page actually documents the format.
func ampLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Agent Skills", CaseInsensitive: true},
		{Kind: "substring", Value: "Skill Format", CaseInsensitive: true},
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
			pattern("creation_workflow", "Creating Skills", "documented under 'Creating Skills' heading"),
			pattern("installation_workflow", "Installing Skills", "documented under 'Installing Skills' heading"),
			pattern("mcp_integration", "MCP Servers in Skills", "documented under 'MCP Servers in Skills' heading"),
			pattern("executable_tools", "Executable Tools in Skills", "documented under 'Executable Tools in Skills' heading"),
		},
	}
}

// ampRulesLandmarkOptions returns the landmark patterns for Amp's rules
// documentation. Anchors derived from .capmon-cache/amp/rules.2/extracted.json
// (the HTML Owner's Manual at ampcode.com/manual). rules.0 and rules.1 are
// example AGENTS.md *instances* from the amp-examples repo and intentionally
// not used as evidence.
//
// Note: rules.2 landmarks have trailing "##" anchor-link suffixes (e.g.
// "AGENTS.md##"). Substring matchers handle this transparently — the matcher
// value "AGENTS.md" matches the landmark "AGENTS.md##".
//
// Required anchors are unique to the rules doc (skills.0 has no AGENTS.md or
// "Writing AGENTS.md Files" landmarks):
//   - "AGENTS.md" (matches "AGENTS.md##")
//   - "Writing AGENTS.md Files"
func ampRulesLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "AGENTS.md", CaseInsensitive: true},
		{Kind: "substring", Value: "Writing AGENTS.md Files", CaseInsensitive: true},
	}
	return RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode.always_on", "AGENTS.md",
			"AGENTS.md files are always_on within their scope (cwd, parent dirs, subtrees) — documented under 'AGENTS.md' / 'Writing AGENTS.md Files'", required),
		RulesLandmarkPattern("activation_mode.frontmatter_globs", "Granular Guidance",
			"@-mentioned files use 'globs' frontmatter for selective activation (documented under 'Granular Guidance'); globs implicitly prefixed with **/ unless ../ or ./", required),
		RulesLandmarkPattern("file_imports", "Granular Guidance",
			"@-mention syntax includes other files as context — supports relative, absolute, and ~/ paths (documented under 'Granular Guidance')", required),
		RulesLandmarkPattern("cross_provider_recognition.agents_md", "AGENTS.md",
			"native AGENTS.md format; 'Migrating to AGENTS.md' documents one-time mv+symlink from CLAUDE.md and .cursorrules", required),
		RulesLandmarkPattern("hierarchical_loading", "Writing AGENTS.md Files",
			"three-tier scope: AGENTS.md in cwd + parent dirs + subtrees + personal global at $HOME/.config/amp/AGENTS.md (documented under 'Writing AGENTS.md Files'); subtree loading is unique to amp", required),
	)
}

// ampHooksLandmarkOptions returns the landmark patterns for Amp's hooks
// capabilities. Evidence is split across two cache sources:
//   - hooks.0 (hooks.md): only one landmark "Hooks" — too thin to anchor on
//   - hooks.1 (permissions-reference.md): rich landmarks describing match
//     conditions, regex patterns, and rule management
//
// Per the curated format YAML (docs/provider-formats/amp.yaml), 2 of the 9
// canonical hooks keys are supported: matcher_patterns
// (hook_match_input_contains via "Match Conditions" + "Regular Expression
// Patterns") and permission_control (permissions_system via "How Permissions
// Work" + "Add Rules"). Both pieces of evidence live in the permissions
// reference doc — amp's hooks integrate with the permission system rather
// than expose a hook-specific protocol.
//
// Required anchors are unique to the permissions doc:
//   - "Permissions Reference" — H1 of permissions-reference.md
//   - "How Permissions Work" — H2, not present in any other amp content doc
func ampHooksLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Permissions Reference", CaseInsensitive: true},
		{Kind: "substring", Value: "How Permissions Work", CaseInsensitive: true},
	}
	return HooksLandmarkOptions(
		HooksLandmarkPattern("matcher_patterns", "Match Conditions",
			"per-tool input matching documented under 'Match Conditions' / 'Regular Expression Patterns' headings (hook_match_input_contains)", required),
		HooksLandmarkPattern("permission_control", "How Permissions Work",
			"hooks integrate with amp's permission system to control tool availability (documented under 'How Permissions Work' / 'Add Rules' headings)", required),
	)
}

// ampMcpLandmarkOptions returns the landmark patterns for Amp's MCP
// documentation. Evidence is split across three cache sources:
//   - mcp.0 (amp-mcp-setup-guide.md): community VS Code setup guide; provides
//     the "3. Configure Tool Access" heading that backs tool_filtering
//     (per-tool enable/disable in the VS Code UI)
//   - mcp.1 (ampcode.com/manual/mcp.md): authoritative Amp manual section
//     covering CLI, config schema, OAuth, and workspace trust
//   - mcp.2 (mcp-registry-allowlist.md): enterprise MCP registry allowlist
//
// Per the curated format YAML (docs/provider-formats/amp.yaml), 3 of the 8
// canonical MCP keys are supported:
//   - oauth_support → "OAuth for Remote MCP Servers" heading (mcp.1)
//   - tool_filtering → "3. Configure Tool Access" heading (mcp.0)
//   - enterprise_management → "MCP Registry Allowlist" heading (mcp.2)
//
// The other 5 canonical keys (transport_types, env_var_expansion, auto_approve,
// marketplace, resource_referencing) are curated as unsupported and therefore
// not mapped — emitting them on heading-presence alone would contradict the
// curated YAML.
//
// Required anchors are unique to the MCP docs:
//   - "Amp MCP Setup Guide" — H1 of mcp.0 (community setup guide)
//   - "MCP Server Loading Order" — H2 of mcp.1 (manual section)
//
// Neither appears in amp's skills, rules, or hooks docs, so this guard
// prevents MCP patterns from firing on non-MCP content-type contexts.
func ampMcpLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Amp MCP Setup Guide", CaseInsensitive: true},
		{Kind: "substring", Value: "MCP Server Loading Order", CaseInsensitive: true},
	}
	return McpLandmarkOptions(
		McpLandmarkPattern("oauth_support", "OAuth for Remote MCP Servers",
			"OAuth 2.0 authentication for remote MCP servers documented under 'OAuth for Remote MCP Servers' heading", required),
		McpLandmarkPattern("tool_filtering", "Configure Tool Access",
			"per-tool enable/disable documented under '3. Configure Tool Access' heading (VS Code UI exposes individual tool toggles within each configured server)", required),
		McpLandmarkPattern("enterprise_management", "MCP Registry Allowlist",
			"workspace-admin-enforced MCP registry with allowlist documented under 'MCP Registry Allowlist' heading (modelcontextprotocol.io schema; unreachable registry blocks all servers)", required),
	)
}

// recognizeAmp recognizes skills + rules + hooks + mcp capabilities for the
// Amp provider. Source for all four content types is markdown/HTML
// documentation; recognition uses landmark (heading) matching. Static facts
// merge in at "confirmed" confidence after a successful skills landmark match.
//
// Amp's hooks doc itself (hooks.0) has only a single H1 landmark, so hooks
// recognition is anchored against the permissions reference doc (hooks.1)
// where the matcher_patterns and permission_control evidence lives.
func recognizeAmp(ctx RecognitionContext) RecognitionResult {
	skillsResult := recognizeLandmarks(ctx, ampLandmarkOptions())
	if len(skillsResult.Capabilities) > 0 {
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "project_scope", "skill directory placed under .agents/skills/<name>/ or .claude/skills/<name>/ within the project root", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "global_scope", "skill directory placed under ~/.config/agents/skills/<name>/, ~/.config/amp/skills/<name>/, or ~/.claude/skills/<name>/", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	}

	rulesResult := recognizeLandmarks(ctx, ampRulesLandmarkOptions())
	hooksResult := recognizeLandmarks(ctx, ampHooksLandmarkOptions())
	mcpResult := recognizeLandmarks(ctx, ampMcpLandmarkOptions())

	return mergeRecognitionResults(skillsResult, rulesResult, hooksResult, mcpResult)
}
