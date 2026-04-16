package capmon

func init() {
	RegisterRecognizer("copilot-cli", RecognizerKindDoc, recognizeCopilotCli)
}

// recognizeCopilotCli recognizes skills capabilities for the Copilot CLI provider.
// Copilot CLI source is documentation-only (markdown); current implementation uses
// the GoStruct preset and produces no output until PR7 introduces landmark-based
// recognition. Tagged RecognizerKindDoc to reflect the intended strategy.
func recognizeCopilotCli(ctx RecognitionContext) RecognitionResult {
	result := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return wrapCapabilities(result)
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "skill directory under .github/skills/<name>/, .claude/skills/<name>/, or .agents/skills/<name>/ in project repository", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "skill directory under ~/.copilot/skills/<name>/, ~/.claude/skills/<name>/, or ~/.agents/skills/<name>/", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return wrapCapabilities(result)
}
