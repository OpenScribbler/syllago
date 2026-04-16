package capmon

func init() {
	RegisterRecognizer("copilot-cli", recognizeCopilotCli)
}

// recognizeCopilotCli recognizes skills capabilities for the Copilot CLI provider.
// Copilot CLI implements the Agent Skills open standard (GoStruct pattern).
func recognizeCopilotCli(fields map[string]FieldValue) map[string]string {
	result := recognizeGoStruct(fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return result
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "skill directory under .github/skills/<name>/, .claude/skills/<name>/, or .agents/skills/<name>/ in project repository", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "skill directory under ~/.copilot/skills/<name>/, ~/.claude/skills/<name>/, or ~/.agents/skills/<name>/", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return result
}
