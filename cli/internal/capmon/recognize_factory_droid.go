package capmon

func init() {
	RegisterRecognizer("factory-droid", RecognizerKindDoc, recognizeFactoryDroid)
}

// factoryDroidLandmarkOptions returns the landmark patterns for Factory Droid's
// skills doc. Anchors derived from
// .capmon-cache/factory-droid/skills.0/extracted.json
// (https://docs.factory.ai/cli/configuration/skills — Mintlify Next.js SPA
// fetched via chromedp).
//
// Mintlify landmarks have a leading zero-width space prefix (e.g.
// "​Skill file format"). Substring matchers handle this transparently — the
// matcher value "Skill file format" matches the landmark "​Skill file format".
//
// Required anchors are unique to the skills doc:
//   - "Skill file format" — H2; not present in any other factory-droid doc
//   - "Where skills live"  — H2; not present in any other factory-droid doc
func factoryDroidLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Skill file format", CaseInsensitive: true},
		{Kind: "substring", Value: "Where skills live", CaseInsensitive: true},
	}
	pattern := func(cap, anchor, mechanism string) LandmarkPattern {
		return LandmarkPattern{
			Capability: cap,
			Required:   required,
			Matchers:   []StringMatcher{{Kind: "substring", Value: anchor, CaseInsensitive: true}},
			Mechanism:  mechanism,
		}
	}
	return LandmarkOptions{
		ContentType: "skills",
		Patterns: []LandmarkPattern{
			pattern("frontmatter", "Frontmatter reference", "documented under 'Frontmatter reference' heading"),
			pattern("creation_workflow", "Quickstart", "documented under 'Quickstart' heading"),
			pattern("directory_structure", "Where skills live", "documented under 'Where skills live' heading"),
			pattern("invocation", "Control who invokes a skill", "documented under 'Control who invokes a skill' heading"),
			// Bare anchor-only pattern guarantees skills.supported when only the
			// required anchors are present (no specific-capability anchor).
			{
				Required: required,
				Matchers: required,
			},
		},
	}
}

// factoryDroidHooksLandmarkOptions returns the landmark patterns for Factory
// Droid's hooks reference doc. Anchors derived from
// .capmon-cache/factory-droid/hooks.1/extracted.json
// (https://docs.factory.ai/reference/hooks-reference — Mintlify SPA fetched
// via chromedp).
//
// Per the curated format YAML (docs/provider-formats/factory-droid.yaml), only
// decision_control is supported among the 9 canonical hooks keys — Factory
// Droid hooks signal block (non-zero) or allow (zero) via exit codes (the
// hook_exit_code_behavior provider extension). The other 8 canonical keys are
// curated as unsupported and intentionally NOT mapped here.
//
// Required anchors are unique to the hooks reference doc — they distinguish
// hooks evidence from skills/rules/mcp/agents/commands evidence in the merged
// landmarks context:
//   - "Hooks reference" — H1 of hooks.1; absent everywhere else
//   - "Hook Events"      — H2 of hooks.1; substring also matches hooks.0's
//     "Hook Events Overview", but both belong to hooks evidence so the guard
//     still scopes correctly
func factoryDroidHooksLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Hooks reference", CaseInsensitive: true},
		{Kind: "substring", Value: "Hook Events", CaseInsensitive: true},
	}
	return HooksLandmarkOptions(
		HooksLandmarkPattern("decision_control", "Decision Control",
			"hook_exit_code_behavior: Factory Droid hooks use exit codes to signal block (non-zero) or allow (zero) decisions on the triggering action; documented under per-event 'Decision Control' headings", required),
		// Bare anchor-only pattern guarantees hooks.supported when only the
		// required anchors are present (no specific-capability anchor).
		LandmarkPattern{
			Required: required,
			Matchers: required,
		},
	)
}

// MCP recognition is intentionally NOT wired for factory-droid.
//
// The cached MCP source (.capmon-cache/factory-droid/mcp.0/extracted.json)
// is the docs.factory.ai/llms.txt index — a flat list of links whose
// landmarks array contains only 4 generic entries: "Factory Documentation",
// "Docs", "OpenAPI Specs", "Optional". None of these can anchor canonical
// MCP keys via substring matching.
//
// Per docs/provider-formats/factory-droid.yaml, all 8 canonical MCP keys
// are curated as unsupported (inferred). The curator's notes confirm the
// gap: "Full MCP config details require the /cli/configuration/mcp.md page
// (not fetched — source here is the llms.txt index only)." The provider
// extensions list two MCP-related features (mcp_manager_ui,
// factory_as_mcp_server) but those are non-portable extensions, not
// canonical MCP-protocol capabilities.
//
// Recognizer silence is the right move — emitting "supported" for any
// canonical key based on llms.txt nav landmarks would be a false positive.
// MCP recognition can be wired once the Mintlify SPA mcp.md page is in
// the cache and yields heading-level evidence.

// recognizeFactoryDroid recognizes skills + hooks capabilities for the Factory
// Droid provider. Source for both is markdown documentation (Mintlify SPA);
// recognition uses landmark matching. Static facts merge in at "confirmed"
// confidence after a successful skills landmark match. MCP recognition is
// intentionally absent — see the comment block immediately above this
// function for rationale.
func recognizeFactoryDroid(ctx RecognitionContext) RecognitionResult {
	skillsResult := recognizeLandmarks(ctx, factoryDroidLandmarkOptions())
	if len(skillsResult.Capabilities) > 0 {
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "project_scope", "<repo>/.factory/skills/<skill-name>/SKILL.md or .agent/skills/<skill-name>/SKILL.md", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "global_scope", "~/.factory/skills/<skill-name>/SKILL.md", "confirmed"))
		mergeInto(skillsResult.Capabilities, capabilityDotPaths("skills", "canonical_filename", "SKILL.md (skill.mdx also accepted)", "confirmed"))
	}

	hooksResult := recognizeLandmarks(ctx, factoryDroidHooksLandmarkOptions())

	return mergeRecognitionResults(skillsResult, hooksResult)
}
