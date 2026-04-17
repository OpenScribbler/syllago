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

// codexRulesLandmarkOptions returns the landmark patterns for Codex's rules
// (AGENTS.md) documentation. Anchors derived from
// .capmon-cache/codex/rules.0/extracted.json (docs/agents_md.md). The cached
// doc is intentionally short — it redirects to the developers.openai.com
// AGENTS.md spec which was not cached. Recognition is constrained to the two
// landmarks present in the stub spec doc.
//
// rules.1 is codex's OWN AGENTS.md instance file (their internal dev rules)
// and intentionally NOT used as evidence — instance content is not capability
// vocabulary.
//
// Required anchors are unique to the rules doc (skills.* sources have no
// AGENTS.md or "Hierarchical agents message" landmarks):
//   - "AGENTS.md"
//   - "Hierarchical agents message"
//
// Per the seeder spec, codex supports activation_mode.always_on,
// cross_provider_recognition.agents_md, and hierarchical_loading. file_imports
// and auto_memory are intentionally absent from the cached doc surface.
func codexRulesLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "AGENTS.md", CaseInsensitive: true},
		{Kind: "substring", Value: "Hierarchical agents message", CaseInsensitive: true},
	}
	return RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode.always_on", "AGENTS.md",
			"AGENTS.md files are always_on within their scope (project root and child directories) — documented under 'AGENTS.md' (per docs/agents_md.md, redirects to developers.openai.com spec)", required),
		RulesLandmarkPattern("cross_provider_recognition.agents_md", "AGENTS.md",
			"codex is a primary AGENTS.md spec implementer (per github.com/openai/codex docs); AGENTS.md is the cross-provider standard", required),
		RulesLandmarkPattern("hierarchical_loading", "Hierarchical agents message",
			"hierarchical AGENTS.md loading gated by child_agents_md feature flag in config.toml; codex emits a precedence-explanation message to the model when enabled (documented under 'Hierarchical agents message')", required),
	)
}

// recognizeCodex recognizes skills + rules capabilities for the Codex provider.
// Codex implements the Agent Skills open standard. Skills source is Rust;
// rules source is markdown. Recognition fires only if the extractor surfaces
// fields under one of the 5 included struct prefixes (see codexSkillsOptions),
// or landmarks under codexRulesLandmarkOptions.
func recognizeCodex(ctx RecognitionContext) RecognitionResult {
	skillsCaps := recognizeGoStruct(ctx.Fields, codexSkillsOptions())
	if len(skillsCaps) > 0 {
		mergeInto(skillsCaps, capabilityDotPaths("skills", "project_scope", ".agents/skills/<name>/ under project config folder or between project root and cwd (SkillScope::Repo)", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "global_scope", "~/.agents/skills/<name>/ (SkillScope::User)", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "canonical_filename", "SKILL.md (SKILLS_FILENAME constant)", "confirmed"))
	}
	skillsResult := wrapCapabilities(skillsCaps)

	rulesResult := recognizeLandmarks(ctx, codexRulesLandmarkOptions())

	return mergeRecognitionResults(skillsResult, rulesResult)
}
