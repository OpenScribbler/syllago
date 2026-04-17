package capmon

func init() {
	RegisterRecognizer("roo-code", RecognizerKindGoStruct, recognizeRooCode)
}

// recognizeRooCode recognizes skills capabilities for the Roo Code provider.
// Roo Code implements the Agent Skills open standard (GoStruct pattern).
//
// Rules recognition is intentionally NOT implemented for roo-code. Both
// cached rules sources (rules.0 = .roo/rules/rules.md and rules.1 =
// .roo/rules-code/use-safeWriteJson.md) are roo-code's OWN instance files —
// the team's internal "Code Quality Rules" and "JSON File Writing Must Be
// Atomic" rules. Landmarks are example content, not capability vocabulary.
// No external rules-format-spec doc is cached for roo-code.
//
// Same instance-vs-spec mismatch as crush — see recognize_crush.go for the
// full rationale. The .roo/rules/ + .roo/rules-code/ directory split observed
// in the cache hints at a category-scoped activation mechanism unique among
// providers, but the syntax/semantics are not documented in any cached
// source. Recognition would be guessing.
//
// Roo-code's rules.* dot-paths remain "not_evaluated" until either a
// format-spec doc is added or the policy on instance-as-evidence changes.
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
