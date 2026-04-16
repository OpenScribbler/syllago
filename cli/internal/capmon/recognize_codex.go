package capmon

func init() {
	RegisterRecognizer("codex", RecognizerKindRustStruct, recognizeCodex)
}

// recognizeCodex recognizes skills capabilities for the Codex provider.
// Codex implements the Agent Skills open standard.
// Source is Rust; recognition fires only if extractor surfaces Skill.* fields.
func recognizeCodex(ctx RecognitionContext) RecognitionResult {
	result := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return wrapCapabilities(result)
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", ".agents/skills/<name>/ under project config folder or between project root and cwd (SkillScope::Repo)", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "~/.agents/skills/<name>/ (SkillScope::User)", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md (SKILLS_FILENAME constant)", "confirmed"))
	return wrapCapabilities(result)
}
