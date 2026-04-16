package capmon

func init() {
	RegisterRecognizer("roo-code", RecognizerKindGoStruct, recognizeRooCode)
}

// recognizeRooCode recognizes skills capabilities for the Roo Code provider.
// Roo Code implements the Agent Skills open standard (GoStruct pattern).
func recognizeRooCode(ctx RecognitionContext) RecognitionResult {
	result := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return wrapCapabilities(result)
	}
	// Scope: roo-code supports project-local and global skill directories
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "per-project .roo/skills/ directory", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "user-global ~/.roo/skills/ directory", "confirmed"))
	// Filename: roo-code uses the canonical SKILL.md filename
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	return wrapCapabilities(result)
}
