package capmon

func init() {
	RegisterRecognizer("crush", recognizeCrush)
}

// recognizeCrush recognizes skills capabilities for the Crush provider.
// Crush implements the Agent Skills open standard (GoStruct pattern).
func recognizeCrush(fields map[string]FieldValue) map[string]string {
	result := recognizeGoStruct(fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return result
	}
	// Scope: crush supports project-local and global skill directories
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "per-project .crush/skills/ directory", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "user-global ~/.crush/skills/ directory", "confirmed"))
	// Filename: crush uses the canonical SKILL.md filename
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return result
}
