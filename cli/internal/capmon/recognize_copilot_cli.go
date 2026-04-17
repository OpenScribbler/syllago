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

// recognizeCopilotCli recognizes skills + rules capabilities for the Copilot CLI
// provider. Source for both content types is markdown documentation; recognition
// uses landmark (heading) matching. Static facts merge in at "confirmed"
// confidence after a successful skills landmark match.
func recognizeCopilotCli(ctx RecognitionContext) RecognitionResult {
	skillsResult := recognizeLandmarks(ctx, copilotCliLandmarkOptions())
	if len(skillsResult.Capabilities) > 0 {
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "project_scope", "skill directory under .github/skills/<name>/, .claude/skills/<name>/, or .agents/skills/<name>/ in project repository", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "global_scope", "skill directory under ~/.copilot/skills/<name>/, ~/.claude/skills/<name>/, or ~/.agents/skills/<name>/", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	}

	rulesResult := recognizeLandmarks(ctx, copilotCliRulesLandmarkOptions())

	return mergeRecognitionResults(skillsResult, rulesResult)
}
