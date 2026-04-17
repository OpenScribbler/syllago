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

// geminiCliHooksLandmarkOptions returns the landmark patterns for Gemini CLI's
// hooks documentation. Anchors derived from the merged headings of
// .capmon-cache/gemini-cli/hooks.{2,3}/extracted.json (docs/hooks/index.md and
// docs/hooks/reference.md).
//
// Required anchors are unique to the hooks docs:
//   - "Hook events" — only appears in hooks.2 (the index doc's H2 listing all
//     11 lifecycle events)
//   - "Configuration schema" — appears in both hooks.2 and hooks.3, never in
//     skills/rules/mcp/commands docs
//
// Per the curated format YAML (docs/provider-formats/gemini-cli.yaml), only
// 3 of the 9 canonical hooks keys are supported in gemini-cli:
// matcher_patterns, decision_control, json_io_protocol. handler_types is
// explicitly false because gemini-cli only supports shell handlers (not
// http/llm-prompt/agent types) — "Hook events" describes lifecycle event
// types, not handler types, and is intentionally not mapped to handler_types.
func geminiCliHooksLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Hook events", CaseInsensitive: true},
		{Kind: "substring", Value: "Configuration schema", CaseInsensitive: true},
	}
	return HooksLandmarkOptions(
		HooksLandmarkPattern("matcher_patterns", "Matchers",
			"event/tool matchers documented under 'Matchers' / 'Matchers and tool names' headings", required),
		HooksLandmarkPattern("decision_control", "Exit codes",
			"decision control via exit codes documented under 'Exit codes' heading (non-zero blocks, zero allows)", required),
		HooksLandmarkPattern("json_io_protocol", "Common output fields",
			"JSON I/O protocol documented under 'Strict JSON requirements' / 'Common output fields' headings", required),
	)
}

// recognizeGeminiCli recognizes skills + rules + hooks capabilities for the
// Gemini CLI provider. Skills use the GoStruct strategy (Agent Skills open
// standard). Rules and hooks are landmark-based against the gemini-md.md and
// hooks/{index,reference}.md docs.
func recognizeGeminiCli(ctx RecognitionContext) RecognitionResult {
	skillsCaps := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(skillsCaps) > 0 {
		mergeInto(skillsCaps, capabilityDotPaths("skills", "project_scope", ".gemini/skills/<name>/SKILL.md", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "global_scope", "~/.gemini/skills/<name>/SKILL.md", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	}
	skillsResult := wrapCapabilities(skillsCaps)

	rulesResult := recognizeLandmarks(ctx, geminiCliRulesLandmarkOptions())
	hooksResult := recognizeLandmarks(ctx, geminiCliHooksLandmarkOptions())

	return mergeRecognitionResults(skillsResult, rulesResult, hooksResult)
}
