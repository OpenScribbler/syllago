package capmon

func init() {
	RegisterRecognizer("claude-code", recognizeClaudeCode)
}

// recognizeClaudeCode recognizes skills capabilities for the Claude Code provider.
// Claude Code implements the Agent Skills open standard (GoStruct pattern).
func recognizeClaudeCode(fields map[string]FieldValue) map[string]string {
	result := recognizeGoStruct(fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return result
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", ".claude/skills/<skill-name>/SKILL.md committed to version control", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "~/.claude/skills/<skill-name>/SKILL.md in user home directory", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return result
}
