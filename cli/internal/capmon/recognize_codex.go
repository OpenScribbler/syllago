package capmon

func init() {
	RegisterRecognizer("codex", RecognizerKindRustStruct, recognizeCodex)
}

// codexSkillsOptions returns the codex-specific GoStruct options. Codex's Rust
// source defines 8 Skill-prefixed structs but only 5 of them describe the skill
// format itself. The other 3 (SkillError, SkillLoadOutcome, SkillFileSystemsByPath)
// are runtime types that pollute capability output if matched indiscriminately
// by a single "Skill." prefix.
//
// Included (5):
//   - SkillMetadata.       skill identity / frontmatter fields
//   - SkillPolicy.         allow/deny rules
//   - SkillInterface.      tool/capability surface description
//   - SkillDependencies.   declared dependency manifest
//   - SkillToolDependency. per-tool dependency entry
//
// Excluded (deliberately, not in StructPrefixes):
//   - SkillError.            runtime error type
//   - SkillLoadOutcome.      runtime load result
//   - SkillFileSystemsByPath. runtime filesystem state
func codexSkillsOptions() GoStructOptions {
	return GoStructOptions{
		ContentType: "skills",
		StructPrefixes: []string{
			"SkillMetadata.",
			"SkillPolicy.",
			"SkillInterface.",
			"SkillDependencies.",
			"SkillToolDependency.",
		},
		KeyMapper: skillsKeyMapper,
	}
}

// recognizeCodex recognizes skills capabilities for the Codex provider.
// Codex implements the Agent Skills open standard. Source is Rust; recognition
// fires only if the extractor surfaces fields under one of the 5 included
// struct prefixes (see codexSkillsOptions).
func recognizeCodex(ctx RecognitionContext) RecognitionResult {
	result := recognizeGoStruct(ctx.Fields, codexSkillsOptions())
	if len(result) == 0 {
		return wrapCapabilities(result)
	}
	mergeInto(result, capabilityDotPaths("skills", "project_scope", ".agents/skills/<name>/ under project config folder or between project root and cwd (SkillScope::Repo)", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "global_scope", "~/.agents/skills/<name>/ (SkillScope::User)", "confirmed"))
	mergeInto(result, capabilityDotPaths("skills", "canonical_filename", "SKILL.md (SKILLS_FILENAME constant)", "confirmed"))
	return wrapCapabilities(result)
}
