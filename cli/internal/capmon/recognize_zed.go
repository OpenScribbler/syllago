package capmon

func init() {
	RegisterRecognizer("zed", RecognizerKindDoc, recognizeZed)
}

// zedRulesLandmarkOptions returns the landmark patterns for Zed's rules
// documentation. Anchors derived from .capmon-cache/zed/rules.1/extracted.json
// (the HTML doc at zed.dev/docs/ai/rules). rules.0 is zed's own .rules
// instance file (their internal Rust coding guidelines) and intentionally NOT
// used as evidence — instance content is not capability vocabulary.
//
// NOTE: The seeder spec drafted cross_provider_recognition as unsupported, but
// the live cache landmarks include AGENTS.md, AGENT.md, CLAUDE.md, GEMINI.md,
// .cursorrules, .windsurfrules, .clinerules, and .github/copilot-instructions.md
// listed under the ".rules files" section. Zed explicitly recognizes all of
// these as fallback rule-file names. The recognizer trusts the live cache
// over the draft spec.
//
// Required anchors are unique to the rules doc:
//   - "Rules Library" (in-app library — distinctive zed feature)
//   - "Migrating from Prompt Library" (zed-specific migration heading)
//
// Per the spec notes, zed's distinctive activation_mode is slash_command
// (Library rules invoked via slash command). Project-root .rules + Library
// Default Rules cover always_on. No frontmatter glob, manual, or
// model_decision modes documented. No file_imports or auto_memory. No
// hierarchical loading documented (project-root only, first-match wins among
// the recognized filenames).
func zedRulesLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Rules Library", CaseInsensitive: true},
		{Kind: "substring", Value: "Migrating from Prompt Library", CaseInsensitive: true},
	}
	return RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode.always_on", ".rules files",
			"project-root .rules file (or first-match fallback name) auto-included in every Agent Panel interaction; Library entries marked as Default Rules also load always_on (documented under '.rules files' / 'Default Rules')", required),
		RulesLandmarkPattern("activation_mode.slash_command", "Slash Commands in Rules",
			"Library rules invoked via slash command to inject the rule into the current agent context (documented under 'Slash Commands in Rules')", required),
		RulesLandmarkPattern("cross_provider_recognition.agents_md", "AGENTS.md",
			"AGENTS.md (and AGENT.md) recognized as fallback rule-file names in project root, first match wins (documented under '.rules files')", required),
		RulesLandmarkPattern("cross_provider_recognition.claude_md", "CLAUDE.md",
			"CLAUDE.md recognized as fallback rule-file name in project root, first match wins (documented under '.rules files')", required),
		RulesLandmarkPattern("cross_provider_recognition.gemini_md", "GEMINI.md",
			"GEMINI.md recognized as fallback rule-file name in project root, first match wins (documented under '.rules files')", required),
		RulesLandmarkPattern("cross_provider_recognition.cursorrules", ".cursorrules",
			".cursorrules recognized as fallback rule-file name in project root, first match wins (documented under '.rules files')", required),
		RulesLandmarkPattern("cross_provider_recognition.windsurfrules", ".windsurfrules",
			".windsurfrules recognized as fallback rule-file name in project root, first match wins (documented under '.rules files')", required),
		RulesLandmarkPattern("cross_provider_recognition.clinerules", ".clinerules",
			".clinerules recognized as fallback rule-file name in project root, first match wins (documented under '.rules files')", required),
	)
}

// recognizeZed recognizes rules capabilities for the Zed provider. Zed does
// not support Agent Skills, so skills emission is intentionally a no-op
// (confirmed-negative signal). Rules recognition uses landmark matching from
// zed's HTML docs at zed.dev/docs/ai/rules.
func recognizeZed(ctx RecognitionContext) RecognitionResult {
	return recognizeLandmarks(ctx, zedRulesLandmarkOptions())
}
