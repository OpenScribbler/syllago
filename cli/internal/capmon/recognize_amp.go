package capmon

func init() {
	RegisterRecognizer("amp", recognizeAmp)
}

// recognizeAmp recognizes skills capabilities for the Amp provider.
// Amp implements the Agent Skills open standard (GoStruct pattern).
func recognizeAmp(fields map[string]FieldValue) map[string]string {
	result := recognizeGoStruct(fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return result
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "skill directory placed under .agents/skills/<name>/ or .claude/skills/<name>/ within the project root", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "skill directory placed under ~/.config/agents/skills/<name>/, ~/.config/amp/skills/<name>/, or ~/.claude/skills/<name>/", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return result
}
