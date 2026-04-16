package capmon

func init() {
	RegisterRecognizer("codex", recognizeCodex)
}

// recognizeCodex recognizes skills capabilities for the Codex provider.
// Codex implements the Agent Skills open standard (GoStruct pattern).
// Source is Rust; GoStruct recognition fires only if extractor surfaces Skill.* fields.
func recognizeCodex(fields map[string]FieldValue) map[string]string {
	result := recognizeGoStruct(fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return result
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", ".agents/skills/<name>/ under project config folder or between project root and cwd (SkillScope::Repo)", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "~/.agents/skills/<name>/ (SkillScope::User)", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md (SKILLS_FILENAME constant)", "confirmed"))
	return result
}
