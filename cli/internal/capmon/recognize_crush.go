package capmon

func init() {
	RegisterRecognizer("crush", RecognizerKindGoStruct, recognizeCrush)
}

// recognizeCrush recognizes skills capabilities for the Crush provider.
// Crush implements the Agent Skills open standard (GoStruct pattern).
func recognizeCrush(ctx RecognitionContext) RecognitionResult {
	result := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return wrapCapabilities(result)
	}
	// Scope: crush supports project-local and global skill directories
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "per-project .crush/skills/ directory", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "user-global ~/.crush/skills/ directory", "confirmed"))
	// Filename: crush uses the canonical SKILL.md filename
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return wrapCapabilities(result)
}
