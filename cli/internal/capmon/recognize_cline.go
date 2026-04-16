package capmon

func init() {
	RegisterRecognizer("cline", recognizeCline)
}

// recognizeCline recognizes skills capabilities for the Cline provider.
// Cline implements the Agent Skills open standard (GoStruct pattern).
func recognizeCline(fields map[string]FieldValue) map[string]string {
	result := recognizeGoStruct(fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return result
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "Skills stored in .cline/skills/<name>/SKILL.md (recommended), .clinerules/skills/<name>/SKILL.md, or .claude/skills/<name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "Skills stored in ~/.cline/skills/<name>/SKILL.md", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return result
}
