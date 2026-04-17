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

// piCommandsLandmarkOptions returns the landmark patterns for Pi's
// "Prompt Templates" doc (Pi brands its slash-commands surface as "Prompt
// Templates"). Anchors derived from .capmon-cache/pi/commands.0/extracted.json.
//
// Maps 1 of 2 canonical commands keys at heading-level evidence:
//   - argument_substitution → "Argument Hints" / "Arguments" / "Usage"
//     headings; the doc covers $1, $2, $@, $ARGUMENTS, ${@:N}, and ${@:N:L}
//     positional shell-style substitution syntaxes.
//
// builtin_commands is intentionally NOT mapped — per the curated YAML
// (docs/provider-formats/pi.yaml), pi has no built-in slash commands; the
// /-prefix surface is entirely user-authored prompt templates. The doc
// confirms this: every section is about CUSTOM templates (locations,
// format, arguments, usage, loading rules).
//
// Required anchors are unique to commands.0:
//   - "Prompt Templates" — H1 page heading; absent from skills/hooks caches
//     (pi skills.0 uses "Skill Commands" but no "Prompt Templates"; pi
//     extensions doc uses "Writing an Extension" / "ExtensionAPI Methods").
//   - "Argument Hints"   — H2; the section that introduces the positional
//     argument syntaxes; absent from every other pi cache.
func piCommandsLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "Prompt Templates", CaseInsensitive: true},
		{Kind: "substring", Value: "Argument Hints", CaseInsensitive: true},
	}
	return CommandsLandmarkOptions(
		CommandsLandmarkPattern("argument_substitution", "Argument Hints",
			"positional shell-style substitution ($1, $2, $@, $ARGUMENTS, ${@:N}, ${@:N:L}) documented under 'Argument Hints' / 'Arguments' / 'Usage' headings", required),
	)
}

// recognizePi recognizes skills + hooks (extensions) + commands capabilities
// for the Pi provider. Skills use the GoStruct strategy (Agent Skills open
// standard). Hooks are landmark-based against the extensions.md doc; commands
// are landmark-based against the prompt-templates doc.
func recognizePi(ctx RecognitionContext) RecognitionResult {
	skillsCaps := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(skillsCaps) > 0 {
		mergeInto(skillsCaps, capabilityDotPaths("skills", "project_scope", ".pi/skills/<name>/SKILL.md or .agents/skills/<name>/SKILL.md", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "global_scope", "~/.pi/agent/skills/<name>/SKILL.md or ~/.agents/skills/<name>/SKILL.md", "confirmed"))
		mergeInto(skillsCaps, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	}
	skillsResult := wrapCapabilities(skillsCaps)

	hooksResult := recognizeLandmarks(ctx, piHooksLandmarkOptions())
	commandsResult := recognizeLandmarks(ctx, piCommandsLandmarkOptions())

	return mergeRecognitionResults(skillsResult, hooksResult, commandsResult)
}
