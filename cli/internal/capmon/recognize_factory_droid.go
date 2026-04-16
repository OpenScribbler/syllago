package capmon

func init() {
	RegisterRecognizer("factory-droid", RecognizerKindDoc, recognizeFactoryDroid)
}

// recognizeFactoryDroid recognizes skills capabilities for the Factory Droid provider.
// Factory Droid source is documentation-only (markdown); current implementation uses
// the GoStruct preset and produces no output until PR8 introduces landmark-based
// recognition. Tagged RecognizerKindDoc to reflect the intended strategy.
func recognizeFactoryDroid(ctx RecognitionContext) RecognitionResult {
	result := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return wrapCapabilities(result)
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "<repo>/.factory/skills/<skill-name>/SKILL.md or .agent/skills/<skill-name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "~/.factory/skills/<skill-name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md (skill.mdx also accepted)", "confirmed"))
	return wrapCapabilities(result)
}
