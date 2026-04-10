package capmon

func init() {
	RegisterRecognizer("roo-code", recognizeRooCodeSkills)
}

// recognizeRooCodeSkills recognizes skills capabilities for the Roo Code provider.
// Roo Code implements the Agent Skills open standard (GoStruct pattern).
func recognizeRooCodeSkills(fields map[string]FieldValue) map[string]string {
	result := recognizeSkillsGoStruct(fields)
	if len(result) == 0 {
		return result
	}
	// Scope: roo-code supports project-local and global skill directories
	mergeInto(result, capabilityDotPaths("project_scope", "per-project .roo/skills/ directory", "confirmed"))
	mergeInto(result, capabilityDotPaths("global_scope", "user-global ~/.roo/skills/ directory", "confirmed"))
	// Filename: roo-code uses the canonical SKILL.md filename
	mergeInto(result, capabilityDotPaths("canonical_filename", "SKILL.md", "confirmed"))
	return result
}
