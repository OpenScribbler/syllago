package capmon

func init() {
	RegisterRecognizer("opencode", recognizeOpencode)
}

// recognizeOpencode recognizes skills capabilities for the OpenCode provider.
// OpenCode is archived; it has no native skill implementation, so this
// recognizer uses the cross-provider SKILL.md convention. GoStruct pattern
// will produce output only if upstream extraction surfaces Skill.* fields
// (unlikely for an archived project).
func recognizeOpencode(fields map[string]FieldValue) map[string]string {
	result := recognizeGoStruct(fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return result
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "cross-provider SKILL.md convention at .opencode/skill/<name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "cross-provider convention at ~/.config/opencode/skills/<name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md (Agent Skills spec)", "confirmed"))
	return result
}
