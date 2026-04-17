package capmon

func init() {
	RegisterRecognizer("gemini-cli", RecognizerKindGoStruct, recognizeGeminiCli)
}

// geminiCliRulesLandmarkOptions returns the landmark patterns for Gemini CLI's
// rules documentation (GEMINI.md files). Anchors derived from
// .capmon-cache/gemini-cli/rules.0/extracted.json (gemini-md.md).
//
// Required anchors are unique to the GEMINI.md doc:
//   - "Provide context with GEMINI.md files" — the page H1
//   - "Understand the context hierarchy" — the H2 that defines the load order
//
// Per the seeder spec, gemini-cli does NOT document explicit AGENTS.md
// recognition — cross_provider_recognition is intentionally absent.
func geminiCliRulesLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Provide context with GEMINI.md files", CaseInsensitive: true},
		{Kind: "substring", Value: "Understand the context hierarchy", CaseInsensitive: true},
	}
	return RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode.always_on", "Understand the context hierarchy",
			"GEMINI.md files load automatically within scope (global, workspace, or JIT) — no frontmatter activation modes documented", required),
		RulesLandmarkPattern("file_imports", "Modularize context with imports",
			"GEMINI.md supports importing other markdown files (documented under 'Modularize context with imports' heading)", required),
		RulesLandmarkPattern("auto_memory", "Manage context with the /memory command",
			"/memory show / add / reload slash commands manage hierarchical memory (documented under 'Manage context with the /memory command')", required),
		RulesLandmarkPattern("hierarchical_loading", "Understand the context hierarchy",
			"three-tier hierarchy: global ~/.gemini/GEMINI.md + workspace + JIT ancestor scan up to trusted root (documented under 'Understand the context hierarchy')", required),
	)
}

// recognizeGeminiCli recognizes skills + rules capabilities for the Gemini CLI
// provider. Skills use the GoStruct strategy (Agent Skills open standard).
// Rules are landmark-based against the gemini-md.md doc.
func recognizeGeminiCli(ctx RecognitionContext) RecognitionResult {
	skillsCaps := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(skillsCaps) > 0 {
		mergeInto(skillsCaps, capabilityDotPaths("skills", "project_scope", ".gemini/skills/<name>/SKILL.md", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "global_scope", "~/.gemini/skills/<name>/SKILL.md", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	}
	skillsResult := wrapCapabilities(skillsCaps)

	rulesResult := recognizeLandmarks(ctx, geminiCliRulesLandmarkOptions())

	return mergeRecognitionResults(skillsResult, rulesResult)
}
