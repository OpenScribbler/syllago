package capmon

func init() {
	RegisterRecognizer("amp", RecognizerKindDoc, recognizeAmp)
}

// recognizeAmp recognizes skills capabilities for the Amp provider.
// Amp source is documentation-only (markdown); current implementation uses
// the GoStruct preset and produces no output until PR5 introduces landmark-based
// recognition. Tagged RecognizerKindDoc to reflect the intended strategy.
func recognizeAmp(ctx RecognitionContext) RecognitionResult {
	result := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return wrapCapabilities(result)
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "skill directory placed under .agents/skills/<name>/ or .claude/skills/<name>/ within the project root", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "skill directory placed under ~/.config/agents/skills/<name>/, ~/.config/amp/skills/<name>/, or ~/.claude/skills/<name>/", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return wrapCapabilities(result)
}
