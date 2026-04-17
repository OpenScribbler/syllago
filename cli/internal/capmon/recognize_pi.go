package capmon

func init() {
	RegisterRecognizer("pi", RecognizerKindTSInterface, recognizePi)
}

// piHooksLandmarkOptions returns the landmark patterns for Pi's "Extensions"
// doc (Pi brands its hook system as "Extensions"). Anchors derived from
// .capmon-cache/pi/hooks.1/extracted.json (extensions.md).
//
// Required anchors are unique to the extensions doc:
//   - "Writing an Extension" — H2 in extensions.md, not present elsewhere
//   - "ExtensionAPI Methods" — H2 in extensions.md, not present elsewhere
//
// Per the curated format YAML (docs/provider-formats/pi.yaml), 2 of the 9
// canonical hooks keys are supported: handler_types (TypeScript-native
// handler beyond shell) and input_modification (tool_call event can intercept
// and modify tool call inputs). Both are mapped here at "inferred" confidence
// via heading evidence.
func piHooksLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Writing an Extension", CaseInsensitive: true},
		{Kind: "substring", Value: "ExtensionAPI Methods", CaseInsensitive: true},
	}
	return HooksLandmarkOptions(
		HooksLandmarkPattern("handler_types", "Writing an Extension",
			"TypeScript-native extension handlers documented under 'Writing an Extension' / 'Extension Styles' (beyond shell-only)", required),
		HooksLandmarkPattern("input_modification", "tool_call",
			"tool_call event handlers can intercept and modify tool call inputs before execution (documented under 'tool_call' event)", required),
	)
}

// recognizePi recognizes skills + hooks (extensions) capabilities for the Pi
// provider. Skills use the GoStruct strategy (Agent Skills open standard).
// Hooks are landmark-based against the extensions.md doc.
func recognizePi(ctx RecognitionContext) RecognitionResult {
	skillsCaps := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(skillsCaps) > 0 {
		mergeInto(skillsCaps, capabilityDotPaths("skills", "project_scope", ".pi/skills/<name>/SKILL.md or .agents/skills/<name>/SKILL.md", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "global_scope", "~/.pi/agent/skills/<name>/SKILL.md or ~/.agents/skills/<name>/SKILL.md", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	}
	skillsResult := wrapCapabilities(skillsCaps)

	hooksResult := recognizeLandmarks(ctx, piHooksLandmarkOptions())

	return mergeRecognitionResults(skillsResult, hooksResult)
}
