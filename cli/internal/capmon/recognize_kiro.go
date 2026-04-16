package capmon

func init() {
	RegisterRecognizer("kiro", RecognizerKindDoc, recognizeKiro)
}

// recognizeKiro recognizes skills capabilities for the Kiro provider.
// Kiro calls them "Powers" (POWER.md); the canonical mapping still populates
// skills.* dot-paths for cross-provider portability. Source is documentation-only
// (markdown); landmark-based recognition lands in PR9.
func recognizeKiro(ctx RecognitionContext) RecognitionResult {
	result := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return wrapCapabilities(result)
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "Self-contained directory installed via Kiro Powers panel (UI installation, no fixed filesystem path)", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "POWER.md (fixed, all caps)", "confirmed"))
	return wrapCapabilities(result)
}
