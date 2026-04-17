package capmon

func init() {
	RegisterRecognizer("roo-code", RecognizerKindGoStruct, recognizeRooCode)
}

// MCP recognition is intentionally NOT wired for roo-code.
//
// All three cached MCP sources are TypeScript implementation files, not
// documentation or schema:
//   - mcp.0 (RooVetGit/Roo-Code/main/src/services/mcp/McpHub.ts) — 4
//     connection-state struct names: ConnectedMcpConnection,
//     DisconnectedMcpConnection, McpConnection, DisableReason. These describe
//     internal client state, not capability surface.
//   - mcp.1 (packages/types/src/mcp.ts) — 10 type names mostly mirroring
//     core MCP-protocol types: McpServer, McpTool, McpResource,
//     McpResourceTemplate, McpResourceResponse, McpToolCallResponse,
//     McpExecutionStatus, McpServerUse, McpErrorEntry, EnabledMcpToolsCount.
//     These are protocol primitives — every MCP client has analogous types.
//     Treating "McpResource" as evidence for resource_referencing would be a
//     false positive: it proves the type exists, not that referencing is a
//     user-facing feature.
//   - mcp.2 (src/shared/globalFileNames.ts) — empty (landmarks: null).
//
// docs/provider-formats/roo-code.yaml has no curated MCP section either —
// the format YAML covers skills + agents/modes only.
//
// Recognizer silence is the right move — emitting any MCP key based on
// implementation struct names would conflate "type exists in the codebase"
// with "feature is documented and user-accessible". MCP recognition can be
// wired once a documentation source (e.g., the Roo Code docs site's MCP
// pages) is added to the cache and yields heading-level evidence.

// rooCodeAgentsLandmarkOptions returns the landmark patterns for Roo Code's
// custom modes (their term for agents). Anchors derived from three caches:
//   - agents.0 (TypeScript: packages/types/src/mode.ts) — landmarks include
//     ModeConfig, CustomModesSettings, GroupOptions, GroupEntry,
//     PromptComponent, CustomModePrompts, CustomSupportPrompts. Strong
//     type-vocabulary evidence for the format.
//   - agents.1 (TypeScript: src/core/config/CustomModesManager.ts) —
//     landmarks include ExportedModeConfig, ImportData, RuleFile,
//     ExportResult, ImportResult.
//   - agents.2 (YAML instance: .roomodes) — landmark "customModes". The
//     instance demonstrates the per-mode schema: slug, name, description,
//     roleDefinition, whenToUse, groups (array with optional inline filter
//     {fileRegex, description} for the edit group), source ("project" |
//     "global").
//
// Maps 3 of 7 canonical agents keys at type-name landmark evidence. This is
// lower-quality evidence than HTML doc headings (no prose explaining what
// the types do), but the type names are descriptive enough to anchor
// confidently when paired with the YAML instance shape.
//
//   - definition_format: ModeConfig + CustomModesSettings parent type plus
//     the .roomodes YAML instance file demonstrating slug/name/description/
//     roleDefinition/whenToUse/groups/source field shape.
//   - tool_restrictions: GroupEntry / GroupOptions types and the YAML groups
//     array vocabulary (read | edit | command | mcp) with optional inline
//     filter for the edit group.
//   - agent_scopes: nested .project + .user emission. The YAML "source"
//     field with values "project" (mapped to .project scope) and "global"
//     (mapped to .user scope) per the .roomodes instance.
//
// Four keys are intentionally unmapped:
//   - invocation_patterns: no clear evidence in the cached types or YAML.
//     Mode invocation is presumably via UI mode-picker, not slash-command
//     or @-mention. No invocation type or landmark.
//   - per_agent_mcp: the "mcp" group enables MCP tool access for the mode
//     but does NOT scope to specific MCP servers — it is a mode-level
//     mcp-enabled toggle, not a per-mode server allowlist. Skip.
//   - model_selection: no model field in ModeConfig per the type vocabulary.
//   - subagent_spawning: no chain/spawn/delegate type or landmark. Modes do
//     not coordinate or delegate to other modes per the cached types.
//
// Required anchors are unique to the agents caches:
//   - "ModeConfig"  — TypeScript type name (agents.0); unique vs roo-code's
//     mcp/rules/skills/commands type vocabularies (mcp uses Mcp* prefix,
//     rules/commands have no overlapping type names).
//   - "customModes" — YAML root key (agents.2); unique vs other content type
//     YAML files in the cache.
func rooCodeAgentsLandmarkOptions() LandmarkOptions {
	required := []StringMatcher{
		{Kind: "substring", Value: "ModeConfig", CaseInsensitive: true},
		{Kind: "substring", Value: "customModes", CaseInsensitive: true},
	}
	return AgentsLandmarkOptions(
		AgentsLandmarkPattern("definition_format", "ModeConfig",
			"YAML modes (.roomodes file or settings) with slug, name, description, roleDefinition, whenToUse, groups, source fields per ModeConfig + CustomModesSettings TypeScript types", required),
		AgentsLandmarkPattern("tool_restrictions", "GroupEntry",
			"per-mode tool group allowlist (read | edit | command | mcp) with optional inline filter (e.g. {fileRegex, description}) for the edit group, per GroupEntry / GroupOptions TypeScript types", required),
		AgentsLandmarkPattern("agent_scopes.project", "customModes",
			"project-scoped custom modes via .roomodes file in repo root (mode source field = 'project')", required),
		AgentsLandmarkPattern("agent_scopes.user", "customModes",
			"user-scoped (global) custom modes via roo-code config (mode source field = 'global')", required),
	)
}

// Commands recognition is intentionally NOT wired for roo-code.
//
// The cached commands source (.capmon-cache/roo-code/commands.0/extracted.json)
// yields one landmark, and it is an instance frontmatter snippet rather than
// a heading: "description: 'Commit and push changes with a descriptive
// message'argument-hint: '[optional-context]'mode: code". This is the YAML
// frontmatter of an example /-command file (commit-and-push), surfaced as a
// landmark by the markdown extractor because the file lacks any H1/H2
// headings above the snippet.
//
// Two problems:
//  1. The required-anchor uniqueness gate cannot fire on a single instance-
//     specific frontmatter blob — there's no second unique substring, and
//     the single landmark contains command-name-specific text that would
//     not generalize to other roo-code commands.
//  2. The frontmatter shape (argument-hint, mode) hints that argument
//     substitution exists, but argument-hint is just a UI hint string for
//     auto-complete — it does NOT prove a substitution syntax (like
//     {{args}} or $ARGUMENTS) is supported. Inferring argument_substitution
//     from a hint field would be a guess, not evidence.
//
// docs/provider-formats/roo-code.yaml has no curated commands section —
// the format YAML covers skills + agents/modes only, with commands left
// "not_evaluated" pending a docs-page or schema source.
//
// Recognizer silence is the right move — emitting any commands key from a
// single example file's frontmatter would conflate "an example exists" with
// "the capability is documented". Commands recognition can be wired once
// the Roo Code docs site adds a /-commands reference page or once the
// cache includes a TypeScript schema for the command-file frontmatter.

// recognizeRooCode recognizes skills + agents capabilities for the Roo Code
// provider. Roo Code implements the Agent Skills open standard (GoStruct
// pattern). Custom modes (their agents primitive) use type-name landmark
// matching against the TypeScript ModeConfig / CustomModesSettings types
// plus the .roomodes YAML instance file.
//
// Rules recognition is intentionally NOT implemented for roo-code. Both
// cached rules sources (rules.0 = .roo/rules/rules.md and rules.1 =
// .roo/rules-code/use-safeWriteJson.md) are roo-code's OWN instance files —
// the team's internal "Code Quality Rules" and "JSON File Writing Must Be
// Atomic" rules. Landmarks are example content, not capability vocabulary.
// No external rules-format-spec doc is cached for roo-code.
//
// Same instance-vs-spec mismatch as crush — see recognize_crush.go for the
// full rationale. The .roo/rules/ + .roo/rules-code/ directory split observed
// in the cache hints at a category-scoped activation mechanism unique among
// providers, but the syntax/semantics are not documented in any cached
// source. Recognition would be guessing.
//
// Roo-code's rules.* dot-paths remain "not_evaluated" until either a
// format-spec doc is added or the policy on instance-as-evidence changes.
//
// MCP and commands recognition are also intentionally absent — see the
// comment blocks immediately above this function for rationale.
func recognizeRooCode(ctx RecognitionContext) RecognitionResult {
	skills := recognizeGoStruct(ctx.Fields, SkillsGoStructOptions())
	if len(skills) > 0 {
		// Scope: roo-code supports project-local and global skill directories
		mergeInto(skills, capabilityDotPaths("skills", "project_scope", "per-project .roo/skills/ directory", "confirmed"))
		mergeInto(skills, capabilityDotPaths("skills", "global_scope", "user-global ~/.roo/skills/ directory", "confirmed"))
		// Filename: roo-code uses the canonical SKILL.md filename
		mergeInto(skills, capabilityDotPaths("skills", "canonical_filename", "SKILL.md", "confirmed"))
	}
	skillsResult := wrapCapabilities(skills)

	agentsResult := recognizeLandmarks(ctx, rooCodeAgentsLandmarkOptions())

	return mergeRecognitionResults(skillsResult, agentsResult)
}
