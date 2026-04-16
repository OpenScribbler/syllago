package capmon

func init() {
	RegisterRecognizer("pi", recognizePi)
}

// recognizePi recognizes skills capabilities for the Pi provider.
// Pi implements the Agent Skills open standard (GoStruct pattern).
// Source is TypeScript; GoStruct recognition fires only if extractor surfaces Skill.* fields.
func recognizePi(fields map[string]FieldValue) map[string]string {
	result := recognizeGoStruct(fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return result
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", ".pi/skills/<name>/SKILL.md or .agents/skills/<name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "~/.pi/agent/skills/<name>/SKILL.md or ~/.agents/skills/<name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return result
}
