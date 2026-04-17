package capmon

func init() {
	RegisterRecognizer("windsurf", RecognizerKindGoStruct, recognizeWindsurf)
}

// windsurfRulesLandmarkOptions returns the landmark patterns for Windsurf's
// rules documentation. Anchors derived from
// .capmon-cache/windsurf/rules.0/extracted.json (Memories & Rules doc) and
// .capmon-cache/windsurf/rules.1/extracted.json (AGENTS.md doc).
//
// Required anchors "Rules Discovery" and "Rules Storage Locations" are unique
// to the rules.0 doc — they prevent rules patterns from firing on sources that
// only mention rules in passing (e.g. the AGENTS.md doc alone, or any other
// content type's landmarks merged into the recognition context).
//
// Activation modes: the rules.0 doc has a single "Activation Modes" heading
// followed by a table whose rows name the four trigger values
// (always_on, manual, model_decision, glob). Table cells are not extracted as
// landmarks, so all four sub-keys gate on the same parent heading. This is the
// strongest available signal that windsurf supports the full activation_mode
// vocabulary — the seeder spec singles out windsurf as the most explicit
// among all 14 providers.
func windsurfRulesLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Rules Discovery", CaseInsensitive: true},
		{Kind: "substring", Value: "Rules Storage Locations", CaseInsensitive: true},
	}
	return RulesLandmarkOptions(
		RulesLandmarkPattern("activation_mode.always_on", "Activation Modes",
			"always_on trigger value documented in 'Activation Modes' table — full rule content included in system prompt every message", required),
		RulesLandmarkPattern("activation_mode.manual", "Activation Modes",
			"manual trigger value documented in 'Activation Modes' table — activated via @-mention", required),
		RulesLandmarkPattern("activation_mode.model_decision", "Activation Modes",
			"model_decision trigger value documented in 'Activation Modes' table — Cascade decides based on context", required),
		RulesLandmarkPattern("activation_mode.glob", "Activation Modes",
			"glob trigger value documented in 'Activation Modes' table — activated when files matching glob pattern are in context", required),
		RulesLandmarkPattern("cross_provider_recognition.agents_md", "AGENTS.md",
			"AGENTS.md fallback documented in dedicated rules.1 doc with 'Comparison with Rules' section", required),
		RulesLandmarkPattern("auto_memory", "How to Manage Memories",
			"Cascade-managed Memories layer auto-writes context based on conversation (documented under 'Memories & Rules' / 'How to Manage Memories')", required),
		RulesLandmarkPattern("hierarchical_loading", "Rules Storage Locations",
			".windsurf/rules scanned in current workspace, sub-directories, and parent dirs up to git root (documented under 'Rules Discovery' / 'Rules Storage Locations')", required),
	)
}

// recognizeWindsurf recognizes skills + rules capabilities for the Windsurf
// provider. Skills currently use the GoStruct strategy (Agent Skills open
// standard) but the live windsurf docs cache contains no Skill.* typed fields
// — skills emission depends on future typed-source availability. Rules are
// landmark-based against the rules.0 (Memories & Rules) + rules.1 (AGENTS.md)
// docs.
func recognizeWindsurf(ctx RecognitionContext) RecognitionResult {
	skillsCaps := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(skillsCaps) > 0 {
		mergeInto(skillsCaps, capabilityDotPaths("skills", "project_scope", "Skill directory at .windsurf/skills/<skill-name>/, also discovered at .agents/skills/<skill-name>/ and .claude/skills/<skill-name>/", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "global_scope", "Skill directory at ~/.codeium/windsurf/skills/<skill-name>/, also discovered at ~/.agents/skills/<skill-name>/ and ~/.claude/skills/<skill-name>/", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "canonical_filename", "SKILL.md (all scopes share the same convention)", "confirmed"))
	}
	skillsResult := wrapCapabilities(skillsCaps)

	rulesResult := recognizeLandmarks(ctx, windsurfRulesLandmarkOptions())

	return mergeRecognitionResults(skillsResult, rulesResult)
}
