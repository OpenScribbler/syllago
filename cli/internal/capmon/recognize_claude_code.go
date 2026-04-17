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

// recognizeClaudeCode recognizes skills capabilities for the Claude Code provider.
// Claude Code's source is markdown documentation, so recognition uses landmark
// (heading) matching rather than typed-source struct extraction. Capabilities
// emitted at confidence "inferred" — recognizeLandmarks enforces this.
//
// Static facts (project_scope, global_scope, canonical_filename) are still emitted
// at "confirmed" confidence because they describe behavior documented in literal
// terms, not inferred from heading presence.
func recognizeClaudeCode(ctx RecognitionContext) RecognitionResult {
	landmarkResult := recognizeLandmarks(ctx, claudeCodeLandmarkOptions())
	if len(landmarkResult.Capabilities) == 0 {
		return landmarkResult
	}
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "project_scope", ".claude/skills/<skill-name>/SKILL.md committed to version control", "confirmed"))
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "global_scope", "~/.claude/skills/<skill-name>/SKILL.md in user home directory", "confirmed"))
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return landmarkResult
}
