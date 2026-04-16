package capmon

func init() {
	RegisterRecognizer("roo-code", recognizeRooCode)
}

// recognizeRooCode recognizes skills capabilities for the Roo Code provider.
// Roo Code implements the Agent Skills open standard (GoStruct pattern).
func recognizeRooCode(fields map[string]FieldValue) map[string]string {
	result := recognizeGoStruct(fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return result
	}
	// Scope: roo-code supports project-local and global skill directories
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "per-project .roo/skills/ directory", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "user-global ~/.roo/skills/ directory", "confirmed"))
	// Filename: roo-code uses the canonical SKILL.md filename
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return result
}
