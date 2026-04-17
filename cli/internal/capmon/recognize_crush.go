package capmon

func init() {
	RegisterRecognizer("crush", RecognizerKindGoStruct, recognizeCrush)
}

// recognizeCrush recognizes skills capabilities for the Crush provider.
// Crush implements the Agent Skills open standard (GoStruct pattern).
//
// Rules recognition is intentionally NOT implemented for crush. The cached
// rules source (rules.0) is crush's OWN AGENTS.md instance file (their
// internal Rust dev guide with landmarks like "Build/Test/Lint Commands",
// "Code Style Guidelines", "Working on the TUI"). These are example content,
// not capability vocabulary — using them as recognition evidence would be the
// same instance-vs-spec mismatch pattern the codex multi-struct allow-list
// fix addressed. No external AGENTS.md format-spec doc is cached for crush.
//
// The seeder spec at .develop/seeder-specs/crush-rules.yaml flagged this for
// reviewer choice: (1) recognize on presence, (2) cross-reference amp/codex
// specs, or (3) leave unrecognized. We chose (3) — evidence-based extraction
// must extract from vocabulary, not from examples. Crush's rules support is
// real but undocumented separately, so its rules.* dot-paths remain
// "not_evaluated" until either a format-spec doc is added or the policy
// changes.
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
