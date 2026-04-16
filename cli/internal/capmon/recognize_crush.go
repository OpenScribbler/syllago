package capmon

func init() {
	RegisterRecognizer("crush", recognizeCrush)
}

// recognizeCrush recognizes skills capabilities for the Crush provider.
// Crush implements the Agent Skills open standard (GoStruct pattern).
func recognizeCrush(fields map[string]FieldValue) map[string]string {
	result := recognizeSkillsGoStruct(fields)
	if len(result) == 0 {
		return result
	}
	// Scope: crush supports project-local and global skill directories
	mergeInto(result, capabilityDotPaths("project_scope", "per-project .crush/skills/ directory", "confirmed"))
	mergeInto(result, capabilityDotPaths("global_scope", "user-global ~/.crush/skills/ directory", "confirmed"))
	// Filename: crush uses the canonical SKILL.md filename
	mergeInto(result, capabilityDotPaths("canonical_filename", "SKILL.md", "confirmed"))
	return result
}
