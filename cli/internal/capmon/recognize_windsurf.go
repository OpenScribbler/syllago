package capmon

func init() {
	RegisterRecognizer("windsurf", recognizeWindsurf)
}

// recognizeWindsurf recognizes skills capabilities for the Windsurf provider.
// Windsurf implements the Agent Skills open standard (GoStruct pattern).
func recognizeWindsurf(fields map[string]FieldValue) map[string]string {
	result := recognizeGoStruct(fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return result
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "Skill directory at .windsurf/skills/<skill-name>/, also discovered at .agents/skills/<skill-name>/ and .claude/skills/<skill-name>/", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "Skill directory at ~/.codeium/windsurf/skills/<skill-name>/, also discovered at ~/.agents/skills/<skill-name>/ and ~/.claude/skills/<skill-name>/", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md (all scopes share the same convention)", "confirmed"))
	return result
}
