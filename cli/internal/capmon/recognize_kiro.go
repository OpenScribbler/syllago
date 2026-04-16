package capmon

func init() {
	RegisterRecognizer("kiro", recognizeKiro)
}

// recognizeKiro recognizes skills capabilities for the Kiro provider.
// Kiro calls them "Powers" (POWER.md); the canonical mapping still populates
// skills.* dot-paths for cross-provider portability.
func recognizeKiro(fields map[string]FieldValue) map[string]string {
	result := recognizeGoStruct(fields, SkillsGoStructOptions())
	if len(result) == 0 {
		return result
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", "Self-contained directory installed via Kiro Powers panel (UI installation, no fixed filesystem path)", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "POWER.md (fixed, all caps)", "confirmed"))
	return result
}
