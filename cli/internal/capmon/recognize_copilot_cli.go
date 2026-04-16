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

// recognizeCopilotCli recognizes skills capabilities for the Copilot CLI provider.
// Source is markdown documentation; recognition uses landmark (heading) matching.
// Static facts (project_scope, global_scope, canonical_filename) merge in at
// "confirmed" confidence after a successful landmark match.
func recognizeCopilotCli(ctx RecognitionContext) RecognitionResult {
	landmarkResult := recognizeLandmarks(ctx, copilotCliLandmarkOptions())
	if len(landmarkResult.Capabilities) == 0 {
		return landmarkResult
	}
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "project_scope", "skill directory under .github/skills/<name>/, .claude/skills/<name>/, or .agents/skills/<name>/ in project repository", "confirmed"))
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "global_scope", "skill directory under ~/.copilot/skills/<name>/, ~/.claude/skills/<name>/, or ~/.agents/skills/<name>/", "confirmed"))
	mergeInto(landmarkResult.Capabilities, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return landmarkResult
}
