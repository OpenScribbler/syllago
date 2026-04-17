package capmon

func init() {
	RegisterRecognizer("cursor", RecognizerKindDoc, recognizeCursor)
}

// cursorRulesLandmarkOptions returns the landmark patterns for Cursor's rules
// documentation. Anchors derived from .capmon-cache/cursor/rules.0/extracted.json
// (https://cursor.com/docs/rules, HTML).
//
// Required anchors are unique to the rules doc:
//   - "How rules work" — top-level rules-page heading
//   - "Rule anatomy" — frontmatter/schema heading
//
// Note: the seeder spec (drafted earlier) marked file_imports and
// cross_provider_recognition as unsupported, but the live cache landmarks now
// include "Importing Rules" and "AGENTS.md" / "Nested AGENTS.md support". The
// docs evolved post-spec; we trust the cache as the live source of truth.
//
// Activation modes (Always / Auto Attached / Apply Manually / Agent Requested)
// are documented as Rule Type values in a frontmatter table. Like windsurf,
// the table cells are not extracted as standalone landmarks — all four
// sub-keys gate on the parent "Activation and enforcement" heading.
func cursorRulesLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "How rules work", CaseInsensitive: true},
		{Kind: "substring", Value: "Rule anatomy", CaseInsensitive: true},
	}
	return RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode.always_on", "Activation and enforcement",
			"'Always' Rule Type — rule included in every prompt (documented under 'Activation and enforcement')", required),
		RulesLandmarkPattern("activation_mode.frontmatter_globs", "Activation and enforcement",
			"'Auto Attached' Rule Type — frontmatter glob patterns activate when matching files in context (documented under 'Activation and enforcement')", required),
		RulesLandmarkPattern("activation_mode.manual", "Activation and enforcement",
			"'Apply Manually' Rule Type — activated via @-mention in chat (documented under 'Activation and enforcement')", required),
		RulesLandmarkPattern("activation_mode.model_decision", "Activation and enforcement",
			"'Agent Requested' Rule Type — agent decides based on rule description (documented under 'Activation and enforcement')", required),
		RulesLandmarkPattern("file_imports", "Importing Rules",
			"rules can reference / import other rules (documented under 'Importing Rules' heading)", required),
		RulesLandmarkPattern("cross_provider_recognition.agents_md", "AGENTS.md",
			"AGENTS.md fallback with nested directory support (documented under 'AGENTS.md' / 'Nested AGENTS.md support' headings)", required),
		RulesLandmarkPattern("hierarchical_loading", "Team Rules",
			"three-tier scope: Project rules (.cursor/rules) + Team Rules (org-wide) + User Rules (documented under 'Project rules' / 'Team Rules' / 'User Rules')", required),
	)
}

// recognizeCursor recognizes rules capabilities for the Cursor provider.
// Cursor does not support Agent Skills (FormatDoc status: unsupported), so no
// skills emission. Rules are landmark-based against cursor.com/docs/rules.
func recognizeCursor(ctx RecognitionContext) RecognitionResult {
	return recognizeLandmarks(ctx, cursorRulesLandmarkOptions())
}
