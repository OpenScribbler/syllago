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

// claudeCodeHooksLandmarkOptions returns the landmark patterns for Claude Code's
// hooks documentation. Anchors derived from .capmon-cache/claude-code/hooks.0/
// extracted.json (https://code.claude.com/docs/en/hooks.md) — 126 H2/H3 headings
// across handler types, matchers, decisions, scopes, JSON I/O, async, and
// permissions.
//
// Required anchors are unique to the hooks doc: "Hook lifecycle" and "Hook
// handler fields" appear in no other content-type doc, so this guard prevents
// hooks patterns from firing on a context that includes only skills or rules
// landmarks.
//
// Two canonical hooks keys are intentionally NOT mapped here:
//   - input_modification: documented only in body text under "PreToolUse decision
//     control" (no dedicated heading). Landmark recognition cannot confirm.
//   - context_injection: documented as systemMessage / additionalContext field in
//     "JSON output" body — no dedicated heading.
//
// Both are real claude-code capabilities (per docs/provider-formats/claude-code.yaml)
// but their evidence lives below the heading layer the landmark recognizer reads.
// Adding them would require field-level recognition (out of scope for Epic 3b).
func claudeCodeHooksLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Hook lifecycle", CaseInsensitive: true},
		{Kind: "substring", Value: "Hook handler fields", CaseInsensitive: true},
	}
	return HooksLandmarkOptions(
		HooksLandmarkPattern("handler_types", "Hook handler fields",
			"four handler types documented under 'Hook handler fields' (Common fields, Command hook fields, HTTP hook fields, Prompt and agent hook fields)", required),
		HooksLandmarkPattern("matcher_patterns", "Matcher patterns",
			"matcher patterns documented under 'Matcher patterns' heading (exact, pipe-separated, regex)", required),
		HooksLandmarkPattern("decision_control", "Decision control",
			"decision control documented under 'Decision control' heading (exit codes + JSON output fields)", required),
		HooksLandmarkPattern("async_execution", "Run hooks in the background",
			"async execution documented under 'Run hooks in the background' / 'Configure an async hook' / 'How async hooks execute' headings", required),
		HooksLandmarkPattern("hook_scopes", "Hook locations",
			"hook scopes documented under 'Hook locations' heading (user, project, local, managed policy, plugin, component frontmatter)", required),
		HooksLandmarkPattern("json_io_protocol", "JSON output",
			"JSON I/O protocol documented under 'Hook input and output' / 'JSON output' headings (continue, stopReason, suppressOutput, systemMessage, hookSpecificOutput)", required),
		HooksLandmarkPattern("permission_control", "Permission update entries",
			"permission control documented under 'PermissionRequest' / 'Permission update entries' headings (addRules/replaceRules/removeRules/setMode)", required),
	)
}

// recognizeClaudeCode recognizes skills + rules + hooks capabilities for the
// Claude Code provider. Source for all three content types is markdown
// documentation, so recognition uses landmark (heading) matching rather than
// typed-source struct extraction. Capabilities emitted at confidence "inferred"
// — recognizeLandmarks enforces this.
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
	hooksResult := recognizeLandmarks(ctx, claudeCodeHooksLandmarkOptions())

	return mergeRecognitionResults(skillsResult, rulesResult, hooksResult)
}
