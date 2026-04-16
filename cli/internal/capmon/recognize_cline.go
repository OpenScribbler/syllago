package capmon

func init() {
	RegisterRecognizer("cline", RecognizerKindDoc, recognizeCline)
}

// recognizeCline recognizes skills capabilities for the Cline provider.
// Cline source is documentation-only (markdown); current implementation uses
// the GoStruct preset and produces no output until PR6 introduces landmark-based
// recognition. Tagged RecognizerKindDoc to reflect the intended strategy.
func recognizeCline(ctx RecognitionContext) RecognitionResult {
	result := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return wrapCapabilities(result)
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "Skills stored in .cline/skills/<name>/SKILL.md (recommended), .clinerules/skills/<name>/SKILL.md, or .claude/skills/<name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "Skills stored in ~/.cline/skills/<name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return wrapCapabilities(result)
}
