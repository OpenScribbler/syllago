package capmon

func init() {
	RegisterRecognizer("pi", RecognizerKindTSInterface, recognizePi)
}

// recognizePi recognizes skills capabilities for the Pi provider.
// Pi implements the Agent Skills open standard.
// Source is TypeScript; recognition fires only if extractor surfaces Skill.* fields.
func recognizePi(ctx RecognitionContext) RecognitionResult {
	result := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return wrapCapabilities(result)
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", ".pi/skills/<name>/SKILL.md or .agents/skills/<name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "~/.pi/agent/skills/<name>/SKILL.md or ~/.agents/skills/<name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return wrapCapabilities(result)
}
