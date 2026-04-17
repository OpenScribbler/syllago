package capmon

func init() {
	RegisterRecognizer("claude-code", RecognizerKindDoc, recognizeClaudeCode)
}

// claudeCodeLandmarkOptions returns the landmark patterns for Claude Code's skills
// documentation. Anchors are derived from the live skills doc's H2/H3 headings
// (see .capmon-cache/claude-code/skills.0/extracted.json). Required anchors guard
// against passing mentions of "skill" elsewhere (e.g., a docs index that lists the
// skills page). Both required headings must be present for ANY pattern to fire.
func claudeCodeLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Extend Claude with skills", CaseInsensitive: true},
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
			pattern("live_reload", "Live change detection", "documented under 'Live change detection' heading"),
			pattern("nested_directories", "Automatic discovery from nested directories", "documented under 'Automatic discovery from nested directories' heading"),
			pattern("additional_directories", "Skills from additional directories", "documented under 'Skills from additional directories' heading"),
			pattern("arguments", "Pass arguments to skills", "documented under 'Pass arguments to skills' heading"),
			pattern("tool_preapproval", "Pre-approve tools for a skill", "documented under 'Pre-approve tools for a skill' heading"),
			pattern("subagent_invocation", "Run skills in a subagent", "documented under 'Run skills in a subagent' heading"),
			pattern("dynamic_context", "Inject dynamic context", "documented under 'Inject dynamic context' heading"),
			pattern("invoker_control", "Control who invokes a skill", "documented under 'Control who invokes a skill' heading"),
		},
	}
}

// claudeCodeRulesLandmarkOptions returns the landmark patterns for Claude Code's
// memory/rules documentation. Anchors derived from
// .capmon-cache/claude-code/rules.0/extracted.json — Claude Code's rules format
// is the richest documented case (32 landmarks across CLAUDE.md, .claude/rules/,
// auto-memory, AGENTS.md fallback, hierarchical loading, and import syntax).
//
// Required anchors are unique to the rules doc: "CLAUDE.md files" and "How
// CLAUDE.md files load" do not appear in the skills doc, so this guard prevents
// rules patterns from firing on a merged-landmark context that includes only
// skills landmarks.
func claudeCodeRulesLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "CLAUDE.md files", CaseInsensitive: true},
		{Kind: "substring", Value: "How CLAUDE.md files load", CaseInsensitive: true},
	}
	return RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode.always_on", "CLAUDE.md files",
			"CLAUDE.md auto-loads when present in project root or working tree", required),
		RulesLandmarkPattern("activation_mode.glob", "Path-specific rules",
			"path-specific rules in .claude/rules/<name>.md fire on glob match (documented under 'Path-specific rules')", required),
		RulesLandmarkPattern("file_imports", "Import additional files",
			"@-mention import syntax pulls referenced files into context (documented under 'Import additional files')", required),
		RulesLandmarkPattern("cross_provider_recognition.agents_md", "AGENTS.md",
			"AGENTS.md fallback when CLAUDE.md absent (documented under 'AGENTS.md' heading)", required),
		RulesLandmarkPattern("auto_memory", "Auto memory",
			"agent-managed automatic memory with /memory audit/edit (documented under 'Auto memory' heading)", required),
		RulesLandmarkPattern("hierarchical_loading", "User-level rules",
			"user-level rules from ~/.claude/CLAUDE.md plus project + additional directories (documented under 'User-level rules')", required),
	)
}

// recognizeClaudeCode recognizes skills + rules capabilities for the Claude Code
// provider. Source for both content types is markdown documentation, so
// recognition uses landmark (heading) matching rather than typed-source struct
// extraction. Capabilities emitted at confidence "inferred" — recognizeLandmarks
// enforces this.
//
// Static facts (project_scope, global_scope, canonical_filename) are still emitted
// at "confirmed" confidence because they describe behavior documented in literal
// terms, not inferred from heading presence.
func recognizeClaudeCode(ctx RecognitionContext) RecognitionResult {
	skillsResult := recognizeLandmarks(ctx, claudeCodeLandmarkOptions())
	if len(skillsResult.Capabilities) > 0 {
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "project_scope", ".claude/skills/<skill-name>/SKILL.md committed to version control", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "global_scope", "~/.claude/skills/<skill-name>/SKILL.md in user home directory", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	}

	rulesResult := recognizeLandmarks(ctx, claudeCodeRulesLandmarkOptions())

	return mergeRecognitionResults(skillsResult, rulesResult)
}
