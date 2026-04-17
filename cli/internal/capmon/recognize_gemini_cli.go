package capmon

func init() {
	RegisterRecognizer("gemini-cli", RecognizerKindGoStruct, recognizeGeminiCli)
}

// recognizeGeminiCli recognizes skills capabilities for the Gemini CLI provider.
// Gemini CLI implements the Agent Skills open standard (GoStruct pattern).
func recognizeGeminiCli(ctx RecognitionContext) RecognitionResult {
	result := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return wrapCapabilities(result)
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", ".gemini/skills/<name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "~/.gemini/skills/<name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return wrapCapabilities(result)
}
